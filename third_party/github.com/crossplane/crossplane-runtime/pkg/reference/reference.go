/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package reference contains utilities for working with cross-resource
// references.
package reference

import (
	"context"
	"strconv"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errGetManaged  = "cannot get referenced resource"
	errListManaged = "cannot list resources that match selector"
	errNoMatches   = "no resources matched selector"
	errNoValue     = "referenced field was empty (referenced resource may not yet be ready)"
)

// NOTE(negz): There are many equivalents of FromPtrValue and ToPtrValue
// throughout the Crossplane codebase. We duplicate them here to reduce the
// number of packages our API types have to import to support references.

// FromPtrValue adapts a string pointer field for use as a CurrentValue.
func FromPtrValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// FromFloatPtrValue adapts a float pointer field for use as a CurrentValue.
func FromFloatPtrValue(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 0, 64)
}

// ToPtrValue adapts a ResolvedValue for use as a string pointer field.
func ToPtrValue(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// ToFloatPtrValue adapts a ResolvedValue for use as a float64 pointer field.
func ToFloatPtrValue(v string) *float64 {
	if v == "" {
		return nil
	}
	vParsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &vParsed
}

// FromPtrValues adapts a slice of string pointer fields for use as CurrentValues.
// NOTE: Do not use this utility function unless you have to.
// Using pointer slices does not adhere to our current API practices.
// The current use case is where generated code creates reference-able fields in a provider which are
// string pointers and need to be resolved as part of `ResolveMultiple`
func FromPtrValues(v []*string) []string {
	var res = make([]string, len(v))
	for i := 0; i < len(v); i++ {
		res[i] = FromPtrValue(v[i])
	}
	return res
}

// FromFloatPtrValues adapts a slice of float64 pointer fields for use as CurrentValues.
func FromFloatPtrValues(v []*float64) []string {
	var res = make([]string, len(v))
	for i := 0; i < len(v); i++ {
		res[i] = FromFloatPtrValue(v[i])
	}
	return res
}

// ToPtrValues adapts ResolvedValues for use as a slice of string pointer fields.
// NOTE: Do not use this utility function unless you have to.
// Using pointer slices does not adhere to our current API practices.
// The current use case is where generated code creates reference-able fields in a provider which are
// string pointers and need to be resolved as part of `ResolveMultiple`
func ToPtrValues(v []string) []*string {
	var res = make([]*string, len(v))
	for i := 0; i < len(v); i++ {
		res[i] = ToPtrValue(v[i])
	}
	return res
}

// ToFloatPtrValues adapts ResolvedValues for use as a slice of float64 pointer fields.
func ToFloatPtrValues(v []string) []*float64 {
	var res = make([]*float64, len(v))
	for i := 0; i < len(v); i++ {
		res[i] = ToFloatPtrValue(v[i])
	}
	return res
}

// To indicates the kind of managed resource a reference is to.
type To struct {
	Managed resource.Managed
	List    resource.ManagedList
}

// An ExtractValueFn specifies how to extract a value from the resolved managed
// resource.
type ExtractValueFn func(resource.Managed) string

// ExternalName extracts the resolved managed resource's external name from its
// external name annotation.
func ExternalName() ExtractValueFn {
	return func(mg resource.Managed) string {
		return meta.GetExternalName(mg)
	}
}

// A ResolutionRequest requests that a reference to a particular kind of
// managed resource be resolved.
type ResolutionRequest struct {
	CurrentValue string
	Reference    *xpv1.Reference
	Selector     *xpv1.Selector
	To           To
	Extract      ExtractValueFn
}

// IsNoOp returns true if the supplied ResolutionRequest cannot or should not be
// processed.
func (rr *ResolutionRequest) IsNoOp() bool {
	isAlways := false
	if rr.Selector != nil {
		if rr.Selector.Policy.IsResolvePolicyAlways() {
			rr.Reference = nil
			isAlways = true
		}
	} else if rr.Reference != nil {
		if rr.Reference.Policy.IsResolvePolicyAlways() {
			isAlways = true
		}
	}

	// We don't resolve values that are already set (if reference resolution policy
	// is not set to Always); we effectively cache resolved values. The CR author
	// can invalidate the cache and trigger a new resolution by explicitly clearing
	// the resolved value.
	if rr.CurrentValue != "" && !isAlways {
		return true
	}

	// We can't resolve anything if neither a reference nor a selector were
	// provided.
	return rr.Reference == nil && rr.Selector == nil
}

// A ResolutionResponse returns the result of a reference resolution. The
// returned values are always safe to set if resolution was successful.
type ResolutionResponse struct {
	ResolvedValue     string
	ResolvedReference *xpv1.Reference
}

// Validate this ResolutionResponse.
func (rr ResolutionResponse) Validate() error {
	if rr.ResolvedValue == "" {
		return errors.New(errNoValue)
	}

	return nil
}

// A MultiResolutionRequest requests that several references to a particular
// kind of managed resource be resolved.
type MultiResolutionRequest struct {
	CurrentValues []string
	References    []xpv1.Reference
	Selector      *xpv1.Selector
	To            To
	Extract       ExtractValueFn
}

// IsNoOp returns true if the supplied MultiResolutionRequest cannot or should
// not be processed.
func (rr *MultiResolutionRequest) IsNoOp() bool {
	isAlways := false
	if rr.Selector != nil {
		if rr.Selector.Policy.IsResolvePolicyAlways() {
			rr.References = nil
			isAlways = true
		}
	} else {
		for _, r := range rr.References {
			if r.Policy.IsResolvePolicyAlways() {
				isAlways = true
				break
			}
		}
	}

	// We don't resolve values that are already set (if reference resolution policy
	// is not set to Always); we effectively cache resolved values. The CR author
	// can invalidate the cache and trigger a new resolution by explicitly clearing
	// the resolved values. This is a little unintuitive for the APIMultiResolver
	// but mimics the UX of the APIResolver and simplifies the overall mental model.
	if len(rr.CurrentValues) > 0 && !isAlways {
		return true
	}

	// We can't resolve anything if neither a reference nor a selector were
	// provided.
	return len(rr.References) == 0 && rr.Selector == nil
}

// A MultiResolutionResponse returns the result of several reference
// resolutions. The returned values are always safe to set if resolution was
// successful.
type MultiResolutionResponse struct {
	ResolvedValues     []string
	ResolvedReferences []xpv1.Reference
}

// Validate this MultiResolutionResponse.
func (rr MultiResolutionResponse) Validate() error {
	if len(rr.ResolvedValues) == 0 {
		return errors.New(errNoMatches)
	}

	for i, v := range rr.ResolvedValues {
		if v == "" {
			return getResolutionError(rr.ResolvedReferences[i].Policy, errors.New(errNoValue))
		}
	}

	return nil
}

// An APIResolver selects and resolves references to managed resources in the
// Kubernetes API server.
type APIResolver struct {
	client client.Reader
	from   resource.Managed
}

// NewAPIResolver returns a Resolver that selects and resolves references from
// the supplied managed resource to other managed resources in the Kubernetes
// API server.
func NewAPIResolver(c client.Reader, from resource.Managed) *APIResolver {
	return &APIResolver{client: c, from: from}
}

// Resolve the supplied ResolutionRequest. The returned ResolutionResponse
// always contains valid values unless an error was returned.
func (r *APIResolver) Resolve(ctx context.Context, req ResolutionRequest) (ResolutionResponse, error) {
	// Return early if from is being deleted, or the request is a no-op.
	if meta.WasDeleted(r.from) || req.IsNoOp() {
		return ResolutionResponse{ResolvedValue: req.CurrentValue, ResolvedReference: req.Reference}, nil
	}

	// The reference is already set - resolve it.
	if req.Reference != nil {
		if err := r.client.Get(ctx, types.NamespacedName{Name: req.Reference.Name}, req.To.Managed); err != nil {
			if kerrors.IsNotFound(err) {
				return ResolutionResponse{}, getResolutionError(req.Reference.Policy, errors.Wrap(err, errGetManaged))
			}
			return ResolutionResponse{}, errors.Wrap(err, errGetManaged)
		}

		rsp := ResolutionResponse{ResolvedValue: req.Extract(req.To.Managed), ResolvedReference: req.Reference}
		return rsp, getResolutionError(req.Reference.Policy, rsp.Validate())
	}

	// The reference was not set, but a selector was. Select a reference.
	if err := r.client.List(ctx, req.To.List, client.MatchingLabels(req.Selector.MatchLabels)); err != nil {
		return ResolutionResponse{}, errors.Wrap(err, errListManaged)
	}

	for _, to := range req.To.List.GetItems() {
		if ControllersMustMatch(req.Selector) && !meta.HaveSameController(r.from, to) {
			continue
		}

		rsp := ResolutionResponse{ResolvedValue: req.Extract(to), ResolvedReference: &xpv1.Reference{Name: to.GetName()}}
		return rsp, getResolutionError(req.Selector.Policy, rsp.Validate())
	}

	// We couldn't resolve anything.
	return ResolutionResponse{}, getResolutionError(req.Selector.Policy, errors.New(errNoMatches))

}

// ResolveMultiple resolves the supplied MultiResolutionRequest. The returned
// MultiResolutionResponse always contains valid values unless an error was
// returned.
func (r *APIResolver) ResolveMultiple(ctx context.Context, req MultiResolutionRequest) (MultiResolutionResponse, error) { //nolint: gocyclo // Only at 11.
	// Return early if from is being deleted, or the request is a no-op.
	if meta.WasDeleted(r.from) || req.IsNoOp() {
		return MultiResolutionResponse{ResolvedValues: req.CurrentValues, ResolvedReferences: req.References}, nil
	}

	// The references are already set - resolve them.
	if len(req.References) > 0 {
		vals := make([]string, len(req.References))
		for i := range req.References {
			if err := r.client.Get(ctx, types.NamespacedName{Name: req.References[i].Name}, req.To.Managed); err != nil {
				if kerrors.IsNotFound(err) {
					return MultiResolutionResponse{}, getResolutionError(req.References[i].Policy, errors.Wrap(err, errGetManaged))
				}
				return MultiResolutionResponse{}, errors.Wrap(err, errGetManaged)
			}
			vals[i] = req.Extract(req.To.Managed)
		}

		rsp := MultiResolutionResponse{ResolvedValues: vals, ResolvedReferences: req.References}
		return rsp, rsp.Validate()
	}

	// No references were set, but a selector was. Select and resolve references.
	if err := r.client.List(ctx, req.To.List, client.MatchingLabels(req.Selector.MatchLabels)); err != nil {
		return MultiResolutionResponse{}, errors.Wrap(err, errListManaged)
	}

	items := req.To.List.GetItems()
	refs := make([]xpv1.Reference, 0, len(items))
	vals := make([]string, 0, len(items))
	for _, to := range req.To.List.GetItems() {
		if ControllersMustMatch(req.Selector) && !meta.HaveSameController(r.from, to) {
			continue
		}

		vals = append(vals, req.Extract(to))
		refs = append(refs, xpv1.Reference{Name: to.GetName()})
	}

	rsp := MultiResolutionResponse{ResolvedValues: vals, ResolvedReferences: refs}
	return rsp, getResolutionError(req.Selector.Policy, rsp.Validate())
}

func getResolutionError(p *xpv1.Policy, err error) error {
	if !p.IsResolutionPolicyOptional() {
		return err
	}
	return nil
}

// ControllersMustMatch returns true if the supplied Selector requires that a
// reference be to a managed resource whose controller reference matches the
// referencing resource.
func ControllersMustMatch(s *xpv1.Selector) bool {
	if s == nil {
		return false
	}
	return s.MatchControllerRef != nil && *s.MatchControllerRef
}
