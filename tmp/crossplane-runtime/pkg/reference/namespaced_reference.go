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

package reference

import (
	"context"
	"maps"
	"slices"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// A NamespacedResolutionRequest requests that a reference to a particular kind of
// managed resource be resolved.
type NamespacedResolutionRequest struct {
	CurrentValue string
	Reference    *xpv1.NamespacedReference
	Selector     *xpv1.NamespacedSelector
	To           To
	Extract      ExtractValueFn
	Namespace    string
}

// IsNoOp returns true if the supplied NamespacedResolutionRequest cannot or should not be
// processed.
func (rr *NamespacedResolutionRequest) IsNoOp() bool {
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

// A NamespacedResolutionResponse returns the result of a reference resolution. The
// returned values are always safe to set if resolution was successful.
type NamespacedResolutionResponse struct {
	ResolvedValue     string
	ResolvedReference *xpv1.NamespacedReference
}

// Validate this NamespacedResolutionResponse.
func (rr NamespacedResolutionResponse) Validate() error {
	if rr.ResolvedValue == "" {
		return errors.New(errNoValue)
	}

	return nil
}

// A MultiNamespacedResolutionRequest requests that several references to a particular
// kind of managed resource be resolved.
type MultiNamespacedResolutionRequest struct {
	CurrentValues []string
	References    []xpv1.NamespacedReference
	Selector      *xpv1.NamespacedSelector
	To            To
	Extract       ExtractValueFn
	Namespace     string
}

// IsNoOp returns true if the supplied MultiNamespacedResolutionRequest cannot or should
// not be processed.
func (rr *MultiNamespacedResolutionRequest) IsNoOp() bool {
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
	// but mimics the UX of the MultiNamespacedResolutionRequest and simplifies the overall mental model.
	if len(rr.CurrentValues) > 0 && !isAlways {
		return true
	}

	// We can't resolve anything if neither a reference nor a selector were
	// provided.
	return len(rr.References) == 0 && rr.Selector == nil
}

// A MultiNamespacedResolutionResponse returns the result of several reference
// resolutions. The returned values are always safe to set if resolution was
// successful.
type MultiNamespacedResolutionResponse struct {
	ResolvedValues     []string
	ResolvedReferences []xpv1.NamespacedReference
}

// Validate this MultiNamespacedResolutionResponse.
func (rr MultiNamespacedResolutionResponse) Validate() error {
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

// An APINamespacedResolver selects and resolves references to managed resources in the
// Kubernetes API server.
type APINamespacedResolver struct {
	client client.Reader
	from   resource.Managed
}

// NewAPINamespacedResolver returns a Resolver that selects and resolves references from
// the supplied managed resource to other managed resources in the Kubernetes
// API server.
func NewAPINamespacedResolver(c client.Reader, from resource.Managed) *APINamespacedResolver {
	return &APINamespacedResolver{client: c, from: from}
}

// Resolve the supplied NamespacedResolutionRequest. The returned NamespacedResolutionResponse
// always contains valid values unless an error was returned.
func (r *APINamespacedResolver) Resolve(ctx context.Context, req NamespacedResolutionRequest) (NamespacedResolutionResponse, error) {
	// Return early if from is being deleted, or the request is a no-op.
	if meta.WasDeleted(r.from) || req.IsNoOp() {
		return NamespacedResolutionResponse{ResolvedValue: req.CurrentValue, ResolvedReference: req.Reference}, nil
	}

	// The reference is already set - resolve it.
	if req.Reference != nil {
		// default to same namespace
		ns := req.Reference.Namespace
		if ns == "" {
			ns = r.from.GetNamespace()
		}

		if err := r.client.Get(ctx, types.NamespacedName{Name: req.Reference.Name, Namespace: ns}, req.To.Managed); err != nil {
			if kerrors.IsNotFound(err) {
				return NamespacedResolutionResponse{}, getResolutionError(req.Reference.Policy, errors.Wrap(err, errGetManaged))
			}

			return NamespacedResolutionResponse{}, errors.Wrap(err, errGetManaged)
		}

		rsp := NamespacedResolutionResponse{ResolvedValue: req.Extract(req.To.Managed), ResolvedReference: req.Reference}

		return rsp, getResolutionError(req.Reference.Policy, rsp.Validate())
	}

	// The reference was not set, but a selector was. Select a reference. If the
	// request has no namespace, then InNamespace is a no-op.
	ns := req.Selector.Namespace
	if ns == "" {
		ns = r.from.GetNamespace()
	}

	if err := r.client.List(ctx, req.To.List, client.MatchingLabels(req.Selector.MatchLabels), client.InNamespace(ns)); err != nil {
		return NamespacedResolutionResponse{}, errors.Wrap(err, errListManaged)
	}

	for _, to := range req.To.List.GetItems() {
		if ControllersMustMatchNamespaced(req.Selector) && !meta.HaveSameController(r.from, to) {
			continue
		}

		rsp := NamespacedResolutionResponse{ResolvedValue: req.Extract(to), ResolvedReference: &xpv1.NamespacedReference{Name: to.GetName(), Namespace: ns}}

		return rsp, getResolutionError(req.Selector.Policy, rsp.Validate())
	}

	// We couldn't resolve anything.
	return NamespacedResolutionResponse{}, getResolutionError(req.Selector.Policy, errors.New(errNoMatches))
}

// ResolveMultiple resolves the supplied MultiNamespacedResolutionRequest. The returned
// MultiNamespacedResolutionResponse always contains valid values unless an error was
// returned.
func (r *APINamespacedResolver) ResolveMultiple(ctx context.Context, req MultiNamespacedResolutionRequest) (MultiNamespacedResolutionResponse, error) { //nolint: gocyclo // Only at 11.
	// Return early if from is being deleted, or the request is a no-op.
	if meta.WasDeleted(r.from) || req.IsNoOp() {
		return MultiNamespacedResolutionResponse{ResolvedValues: req.CurrentValues, ResolvedReferences: req.References}, nil
	}

	// The references are already set - resolve them.
	if len(req.References) > 0 {
		resolvedVals := make([]string, len(req.References))
		for i := range req.References {
			ns := req.References[i].Namespace
			if ns == "" {
				ns = r.from.GetNamespace()
			}

			if err := r.client.Get(ctx, types.NamespacedName{Name: req.References[i].Name, Namespace: ns}, req.To.Managed); err != nil {
				if kerrors.IsNotFound(err) {
					return MultiNamespacedResolutionResponse{}, getResolutionError(req.References[i].Policy, errors.Wrap(err, errGetManaged))
				}

				return MultiNamespacedResolutionResponse{}, errors.Wrap(err, errGetManaged)
			}

			resolvedVals[i] = req.Extract(req.To.Managed)
		}

		rsp := MultiNamespacedResolutionResponse{ResolvedValues: resolvedVals, ResolvedReferences: req.References}

		return rsp, rsp.Validate()
	}

	// No references were set, but a selector was. Select and resolve
	// references. If the request has no namespace, then InNamespace is a no-op.
	ns := req.Selector.Namespace
	if ns == "" {
		ns = r.from.GetNamespace()
	}

	if err := r.client.List(ctx, req.To.List, client.MatchingLabels(req.Selector.MatchLabels), client.InNamespace(ns)); err != nil {
		return MultiNamespacedResolutionResponse{}, errors.Wrap(err, errListManaged)
	}

	valueMap := make(map[string]xpv1.NamespacedReference)
	for _, to := range req.To.List.GetItems() {
		if ControllersMustMatchNamespaced(req.Selector) && !meta.HaveSameController(r.from, to) {
			continue
		}

		valueMap[req.Extract(to)] = xpv1.NamespacedReference{Name: to.GetName(), Namespace: ns}
	}

	sortedKeys, sortedRefs := sortGenericMapByKeys(valueMap)

	rsp := MultiNamespacedResolutionResponse{ResolvedValues: sortedKeys, ResolvedReferences: sortedRefs}

	return rsp, getResolutionError(req.Selector.Policy, rsp.Validate())
}

func sortGenericMapByKeys[T any](m map[string]T) ([]string, []T) {
	keys := slices.Sorted(maps.Keys(m))

	values := make([]T, 0, len(keys))
	for _, k := range keys {
		values = append(values, m[k])
	}

	return keys, values
}

// ControllersMustMatchNamespaced returns true if the supplied Selector requires that a
// reference be to a managed resource whose controller reference matches the
// referencing resource.
func ControllersMustMatchNamespaced(s *xpv1.NamespacedSelector) bool {
	if s == nil {
		return false
	}

	return s.MatchControllerRef != nil && *s.MatchControllerRef
}
