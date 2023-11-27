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

package claim

import (
	"context"
	"fmt"
	"strings"

	"dario.cat/mergo"
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	errUnsupportedClaimSpec = "composite resource claim spec was not an object"
	errUnsupportedDstObject = "destination object was not valid object"
	errUnsupportedSrcObject = "source object was not valid object"

	errBindCompositeConflict = "cannot bind composite resource that references a different claim"

	errMergeClaimSpec   = "unable to merge claim spec"
	errMergeClaimStatus = "unable to merge claim status"
)

type apiCompositeConfigurator struct {
	names.NameGenerator
}

// NewAPICompositeConfigurator returns a Configurator of composite
// resources that may perform a dry-run create against an API server in order to
// name and validate the configured resource.
func NewAPICompositeConfigurator(ng names.NameGenerator) Configurator {
	return &apiCompositeConfigurator{NameGenerator: ng}
}

// Configure the supplied composite resource by propagating configuration from
// the supplied claim. Both create and update scenarios are supported; i.e. the
// composite may or may not have been created in the API server when passed to
// this method. The configured composite may be submitted to an API server via a
// dry run create in order to name and validate it.
func (c *apiCompositeConfigurator) Configure(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { //nolint:gocyclo // Only slightly over (12).
	ucm, ok := cm.(*claim.Unstructured)
	if !ok {
		return nil
	}
	ucp, ok := cp.(*composite.Unstructured)
	if !ok {
		return nil
	}

	icmSpec := ucm.Object["spec"]
	spec, ok := icmSpec.(map[string]any)
	if !ok {
		return errors.New(errUnsupportedClaimSpec)
	}

	existing := ucp.GetClaimReference()
	proposed := ucm.GetReference()
	if existing != nil && !cmp.Equal(existing, proposed) {
		return errors.New(errBindCompositeConflict)
	}

	// It's possible we're being asked to configure a statically provisioned
	// composite resource in which case we should respect its existing name and
	// external name.
	en := meta.GetExternalName(ucp)

	// Do not propagate *.kubernetes.io annotations/labels down to the composite
	// For example: when a claim gets deployed using kubectl,
	// its kubectl.kubernetes.io/last-applied-configuration annotation
	// should not be propagated to the corresponding composite resource,
	// because:
	// * XR was not created using kubectl
	// * The content of the annotaton refers to the claim, not XR
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/
	// for all annotations and their semantic
	meta.AddAnnotations(ucp, withoutReservedK8sEntries(ucm.GetAnnotations()))
	meta.AddLabels(ucp, withoutReservedK8sEntries(cm.GetLabels()))
	meta.AddLabels(ucp, map[string]string{
		xcrd.LabelKeyClaimName:      ucm.GetName(),
		xcrd.LabelKeyClaimNamespace: ucm.GetNamespace(),
	})

	// If our composite resource already exists we want to restore its
	// original external name (if set) in order to ensure we don't try to
	// rename anything after the fact.
	if meta.WasCreated(ucp) && en != "" {
		meta.SetExternalName(ucp, en)
	}

	// We want to propagate the claim's spec to the composite's spec, but
	// first we must filter out any well-known fields that are unique to
	// claims. We do this by:
	// 1. Grabbing a map whose keys represent all well-known claim fields.
	// 2. Deleting any well-known fields that we want to propagate.
	// 3. Using the resulting map keys to filter the claim's spec.
	wellKnownClaimFields := xcrd.CompositeResourceClaimSpecProps()
	for _, field := range xcrd.PropagateSpecProps {
		delete(wellKnownClaimFields, field)
	}

	// CompositionRevision is a special field which needs to be propagated
	// based on the Update policy. If the policy is `Manual`, we need to
	// remove CompositionRevisionRef from wellKnownClaimFields, so it
	// does not get filtered out and is set correctly in composite
	if cp.GetCompositionUpdatePolicy() != nil && *cp.GetCompositionUpdatePolicy() == xpv1.UpdateManual {
		delete(wellKnownClaimFields, xcrd.CompositionRevisionRef)
	}

	claimSpecFilter := xcrd.GetPropFields(wellKnownClaimFields)
	ucp.Object["spec"] = filter(spec, claimSpecFilter...)

	// Note that we overwrite the entire composite spec above, so we wait
	// until this point to set the claim reference. We compute the reference
	// earlier so we can return early if it would not be allowed.
	ucp.SetClaimReference(proposed)

	if !meta.WasCreated(cp) {
		cp.SetGenerateName(fmt.Sprintf("%s-", cm.GetName()))

		// Generated name is likely (but not guaranteed) to be available
		// when we create the composite resource. If taken,
		// then we are going to update an existing composite,
		// hijacking it from another claim. Depending on context/environment
		// the consequences could be more or less serious.
		// TODO: decide if we must prevent it.
		return c.GenerateName(ctx, cp)
	}

	return nil
}

func withoutReservedK8sEntries(a map[string]string) map[string]string {
	for k := range a {
		s := strings.Split(k, "/")
		if strings.HasSuffix(s[0], "kubernetes.io") || strings.HasSuffix(s[0], "k8s.io") {
			delete(a, k)
		}
	}
	return a
}

func filter(in map[string]any, keys ...string) map[string]any {
	filter := map[string]bool{}
	for _, k := range keys {
		filter[k] = true
	}

	out := map[string]any{}
	for k, v := range in {
		if filter[k] {
			continue
		}

		out[k] = v
	}
	return out
}

// APIClaimConfigurator configures the supplied claims with fields
// from the composite. This includes late-initializing spec values
// and updating status fields in claim.
type APIClaimConfigurator struct {
	client client.Client
}

// NewAPIClaimConfigurator returns a APIClaimConfigurator.
func NewAPIClaimConfigurator(client client.Client) *APIClaimConfigurator {
	return &APIClaimConfigurator{client: client}
}

// Configure the supplied claims with fields from the composite.
// This includes late-initializing spec values and updating status fields in claim.
func (c *APIClaimConfigurator) Configure(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	ucm, ok := cm.(*claim.Unstructured)
	if !ok {
		return nil
	}
	ucp, ok := cp.(*composite.Unstructured)
	if !ok {
		return nil
	}

	if err := merge(ucm.Object["status"], ucp.Object["status"],
		// Status fields from composite overwrite non-empty fields in claim
		withMergeOptions(mergo.WithOverride),
		withSrcFilter(xcrd.GetPropFields(xcrd.CompositeResourceStatusProps())...)); err != nil {
		return errors.Wrap(err, errMergeClaimStatus)
	}

	if err := c.client.Status().Update(ctx, cm); err != nil {
		return errors.Wrap(err, errUpdateClaimStatus)
	}

	// Propagate the actual external name back from the composite to the
	// claim if it's set. The name we're propagating here will may be a name
	// the XR must enforce (i.e. overriding any requested by the claim) but
	// will often actually just be propagating back a name that was already
	// propagated forward from the claim to the XR during the
	// preceding configure phase.
	if en := meta.GetExternalName(cp); en != "" {
		meta.SetExternalName(cm, en)
	}

	// We want to propagate the composite's spec to the claim's spec, but
	// first we must filter out any well-known fields that are unique to
	// composites. We do this by:
	// 1. Grabbing a map whose keys represent all well-known composite fields.
	// 2. Deleting any well-known fields that we want to propagate.
	// 3. Filtering OUT the remaining map keys from the composite's spec so
	// that we end up adding only the well-known fields to the claim's spec.
	wellKnownCompositeFields := xcrd.CompositeResourceSpecProps()
	for _, field := range xcrd.PropagateSpecProps {
		delete(wellKnownCompositeFields, field)
	}

	// CompositionRevision is a special field which needs to be propagated
	// based on the Update policy. If the policy is `Automatic`, we need to
	// overwrite the claim's value with the composite's which should be the
	// `currentRevision`
	if cp.GetCompositionUpdatePolicy() != nil && *cp.GetCompositionUpdatePolicy() == xpv1.UpdateAutomatic {
		cm.SetCompositionRevisionReference(cp.GetCompositionRevisionReference())
	}

	compositeSpecFilter := xcrd.GetPropFields(wellKnownCompositeFields)
	if err := merge(ucm.Object["spec"], ucp.Object["spec"],
		withSrcFilter(compositeSpecFilter...)); err != nil {
		return errors.Wrap(err, errMergeClaimSpec)
	}
	return errors.Wrap(c.client.Update(ctx, cm), errUpdateClaim)
}

type mergeConfig struct {
	mergeOptions []func(*mergo.Config)
	srcfilter    []string
}

// withMergeOptions allows custom mergo.Config options
func withMergeOptions(opts ...func(*mergo.Config)) func(*mergeConfig) {
	return func(config *mergeConfig) {
		config.mergeOptions = opts
	}
}

// withSrcFilter filters supplied keys from src map before merging
func withSrcFilter(keys ...string) func(*mergeConfig) {
	return func(config *mergeConfig) {
		config.srcfilter = keys
	}
}

// merge a src map into dst map
func merge(dst, src any, opts ...func(*mergeConfig)) error {
	if dst == nil || src == nil {
		// Nothing available to merge if dst or src are nil.
		// This can occur early on in reconciliation when the
		// status subresource has not been set yet.
		return nil
	}

	config := &mergeConfig{}

	for _, opt := range opts {
		opt(config)
	}

	dstMap, ok := dst.(map[string]any)
	if !ok {
		return errors.New(errUnsupportedDstObject)
	}

	srcMap, ok := src.(map[string]any)
	if !ok {
		return errors.New(errUnsupportedSrcObject)
	}

	return mergo.Merge(&dstMap, filter(srcMap, config.srcfilter...), config.mergeOptions...)
}
