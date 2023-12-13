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
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	errUnsupportedClaimSpec = "composite resource claim spec was not an object"
	errUnsupportedDstObject = "destination object was not valid object"
	errUnsupportedSrcObject = "source object was not valid object"

	errMergeClaimSpec   = "unable to merge claim spec"
	errMergeClaimStatus = "unable to merge claim status"
)

var (
	// ErrBindCompositeConflict can occur if the composite refers a different claim
	ErrBindCompositeConflict = errors.New("cannot bind composite resource that references a different claim")
)

type apiCompositeConfigurator struct {
	names.NameGenerator
}

// Configure configures the supplied composite patch
// by propagating configuration from the supplied claim.
// Both create and update scenarios are supported; i.e. the
// composite may or may not have been created in the API server
// when passed to this method.
func (c *apiCompositeConfigurator) Configure(ctx context.Context, cmObserved *claim.Unstructured, cpObserved, cpPatch *composite.Unstructured) error { //nolint:gocyclo // Only slightly over (12).

	icmSpec := cmObserved.Object["spec"]
	spec, ok := icmSpec.(map[string]any)
	if !ok {
		return errors.New(errUnsupportedClaimSpec)
	}

	existing := cpObserved.GetClaimReference()
	proposed := cmObserved.GetReference()
	if existing != nil && !cmp.Equal(existing, proposed) {
		return ErrBindCompositeConflict
	}

	// It's possible we're being asked to configure a statically provisioned
	// composite resource in which case we should respect its existing name and
	// external name.
	en := meta.GetExternalName(cpObserved)

	// Do not propagate *.kubernetes.io annotations/labels down to the composite
	// For example: when a claim gets deployed using kubectl,
	// its kubectl.kubernetes.io/last-applied-configuration annotation
	// should not be propagated to the corresponding composite resource,
	// because:
	// * XR was not created using kubectl
	// * The content of the annotaton refers to the claim, not XR
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/
	// for all annotations and their semantic
	if ann := withoutReservedK8sEntries(cmObserved.GetAnnotations()); len(ann) > 0 {
		meta.AddAnnotations(cpPatch, withoutReservedK8sEntries(cmObserved.GetAnnotations()))
	}
	meta.AddLabels(cpPatch, withoutReservedK8sEntries(cmObserved.GetLabels()))
	meta.AddLabels(cpPatch, map[string]string{
		xcrd.LabelKeyClaimName:      cmObserved.GetName(),
		xcrd.LabelKeyClaimNamespace: cmObserved.GetNamespace(),
	})

	// If our composite resource already exists we want to restore its
	// original external name (if set) in order to ensure we don't try to
	// rename anything after the fact.
	if meta.WasCreated(cpObserved) && en != "" {
		meta.SetExternalName(cpPatch, en)
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
	if cpObserved.GetCompositionUpdatePolicy() != nil && *cpObserved.GetCompositionUpdatePolicy() == xpv1.UpdateManual {
		delete(wellKnownClaimFields, xcrd.CompositionRevisionRef)
	}

	claimSpecFilter := xcrd.GetPropFields(wellKnownClaimFields)
	cpPatch.Object["spec"] = withoutKeys(spec, claimSpecFilter...)

	// Note that we overwrite the entire composite spec above, so we wait
	// until this point to set the claim reference. We compute the reference
	// earlier so we can return early if it would not be allowed.
	cpPatch.SetClaimReference(proposed)

	if meta.WasCreated(cpObserved) {
		cpPatch.SetName(cpObserved.GetName())
		return nil
	}

	// The composite was not found in the informer cache,
	// or in the apiserver watch cache,
	// or really does not exist.
	// If the claim contains the composite reference,
	// try to use it to set the composite name.
	// This protects us against stale caches:
	// 1. If the composite exists, but the cache was not up-to-date,
	//    then its creation is going to fail, and after requeue,
	//    the cache eventually gets up-to-date and everything is good.
	// 2. If the composite really does not exist, it means that
	//    the claim got bound in one of previous loop,
	//    but something went wrong at composite creation and we requeued.
	//    It is alright to try to use the very same name again.
	if ref := cmObserved.GetResourceReference(); ref != nil &&
		ref.APIVersion == cpObserved.GetAPIVersion() && ref.Kind == cpObserved.GetKind() {
		cpPatch.SetName(ref.Name)
		return nil
	}
	// Otherwise, generate name with a random suffix, hoping it is not already taken
	cpObserved.SetGenerateName(fmt.Sprintf("%s-", cmObserved.GetName()))
	// Generated name is likely (but not guaranteed) to be available
	// when we create the composite resource. If taken,
	// then we are going to update an existing composite,
	// hijacking it from another claim. Depending on context/environment
	// the consequences could be more or less serious.
	// TODO: decide if we must prevent it.
	if err := c.GenerateName(ctx, cpObserved); err != nil {
		return err
	}
	cpPatch.SetName(cpObserved.GetName())
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

func withoutKeys(in map[string]any, keys ...string) map[string]any {
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

func onlyKeys(in map[string]any, keys ...string) map[string]any {
	filter := map[string]bool{}
	for _, k := range keys {
		filter[k] = true
	}

	out := map[string]any{}
	for k, v := range in {
		if filter[k] {
			out[k] = v
		}
	}
	return out
}

// configure the supplied claim patch with fields from the composite.
// This includes late-initializing spec values and updating status fields in claim.
func configureClaim(_ context.Context, cmObserved *claim.Unstructured, cmPatch *claim.Unstructured, cpObserved *composite.Unstructured, cpPatch *composite.Unstructured) error { //nolint:gocyclo // Only slightly over (10)
	existing := cmObserved.GetResourceReference()
	proposed := meta.ReferenceTo(cpPatch, cpPatch.GetObjectKind().GroupVersionKind())
	equal := cmp.Equal(existing, proposed, cmpopts.IgnoreFields(corev1.ObjectReference{}, "UID"))

	// We refuse to 're-bind' a claim that is already bound to a different
	// composite resource.
	if existing != nil && !equal {
		return errors.New(errBindClaimConflict)
	}

	cmPatch.SetResourceReference(proposed)

	cmPatch.Object["status"] = map[string]any{}
	// If existing claim has the status set,
	// copy the conditions to the patch
	if s, ok := cmObserved.Object["status"]; ok {
		fs, ok := s.(map[string]any)
		if !ok {
			return errors.Wrap(errors.New(errUnsupportedSrcObject), errMergeClaimStatus)
		}
		cmPatch.Object["status"] = onlyKeys(fs, "conditions")
	}

	// merge from the composite status everything
	// except conditions and connectionDetails
	if v := cpObserved.Object["status"]; v != nil {
		cpObservedStatus, ok := v.(map[string]any)
		if !ok {
			return errors.Wrap(errors.New(errUnsupportedSrcObject), errMergeClaimStatus)
		}
		if err := merge(cmPatch.Object["status"], withoutKeys(cpObservedStatus, xcrd.GetPropFields(xcrd.CompositeResourceStatusProps())...),
			// Status fields from composite overwrite non-empty fields in claim
			mergo.WithOverride); err != nil {
			return errors.Wrap(err, errMergeClaimStatus)
		}
	}
	// Propagate the actual external name back from the composite to the
	// claim if it's set. The name we're propagating here will may be a name
	// the XR must enforce (i.e. overriding any requested by the claim) but
	// will often actually just be propagating back a name that was already
	// propagated forward from the claim to the XR during the
	// preceding configure phase.
	if en := meta.GetExternalName(cpObserved); en != "" {
		meta.SetExternalName(cmPatch, en)
	}

	// CompositionRevision is a special field which needs to be propagated
	// based on the Update policy. If the policy is `Automatic`, we need to
	// overwrite the claim's value with the composite's which should be the
	// `currentRevision`
	if cpObserved.GetCompositionUpdatePolicy() != nil &&
		*cpObserved.GetCompositionUpdatePolicy() == xpv1.UpdateAutomatic && cpObserved.GetCompositionRevisionReference() != nil {
		cmPatch.SetCompositionRevisionReference(cpObserved.GetCompositionRevisionReference())
	}

	// propagate to the claim only the following fields:
	// - "compositionRef"
	// - "compositionSelector"
	// - "compositionUpdatePolicy"
	// - "compositionRevisionSelector"
	if v := cpObserved.Object["spec"]; v != nil {
		cpObservedSpec, ok := v.(map[string]any)
		if !ok {
			return errors.Wrap(errors.New(errUnsupportedSrcObject), errMergeClaimSpec)
		}
		if err := merge(cmPatch.Object["spec"], onlyKeys(cpObservedSpec, xcrd.PropagateSpecProps...)); err != nil {
			return errors.Wrap(err, errMergeClaimSpec)
		}
	}
	return nil
}

// merge a src map into dst map
func merge(dst, src any, opts ...func(*mergo.Config)) error {
	if dst == nil || src == nil {
		// Nothing available to merge if dst or src are nil.
		// This can occur early on in reconciliation when the
		// status subresource has not been set yet.
		return nil
	}

	dstMap, ok := dst.(map[string]any)
	if !ok {
		return errors.New(errUnsupportedDstObject)
	}

	srcMap, ok := src.(map[string]any)
	if !ok {
		return errors.New(errUnsupportedSrcObject)
	}

	return mergo.Merge(&dstMap, srcMap, opts...)
}
