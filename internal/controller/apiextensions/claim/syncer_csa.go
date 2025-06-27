/*
Copyright 2024 The Crossplane Authors.

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

	"dario.cat/mergo"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/common"
	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	errUpdateClaim          = "cannot update claim"
	errUnsupportedClaimSpec = "claim spec was not an object"
	errGenerateName         = "cannot generate a name for composite resource"
	errApplyComposite       = "cannot apply composite resource"

	errMergeClaimSpec   = "unable to merge claim spec"
	errMergeClaimStatus = "unable to merge claim status"
)

// A ClientSideCompositeSyncer binds and syncs a claim with a composite resource
// (XR). It uses client-side apply to update the claim and the composite.
type ClientSideCompositeSyncer struct {
	client resource.ClientApplicator
	names  names.NameGenerator
}

// NewClientSideCompositeSyncer returns a CompositeSyncer that uses client-side
// apply to sync a claim with a composite resource.
func NewClientSideCompositeSyncer(c client.Client, ng names.NameGenerator) *ClientSideCompositeSyncer {
	return &ClientSideCompositeSyncer{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
		names: ng,
	}
}

// Sync the supplied claim with the supplied composite resource (XR). Syncing
// may involve creating and binding the XR.
func (s *ClientSideCompositeSyncer) Sync(ctx context.Context, cm *claim.Unstructured, xr *composite.Unstructured) error {
	// First we sync claim -> XR.

	// It's possible we're being asked to configure a statically provisioned XR.
	// We should respect its existing external name annotation.
	en := meta.GetExternalName(xr)

	// Do not propagate *.kubernetes.io annotations/labels from claim to XR. For
	// example kubectl.kubernetes.io/last-applied-configuration should not be
	// propagated.
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/
	// for all annotations and their semantic
	meta.AddAnnotations(xr, withoutReservedK8sEntries(cm.GetAnnotations()))
	meta.AddLabels(xr, withoutReservedK8sEntries(cm.GetLabels()))
	meta.AddLabels(xr, map[string]string{
		xcrd.LabelKeyClaimName:      cm.GetName(),
		xcrd.LabelKeyClaimNamespace: cm.GetNamespace(),
	})

	// If the bound XR already exists we want to restore its original external
	// name annotation in order to ensure we don't try to rename anything after
	// the fact.
	if meta.WasCreated(xr) && en != "" {
		meta.SetExternalName(xr, en)
	}

	// We want to propagate the claim's spec to the composite's spec, but first
	// we must filter out any well-known fields that are unique to claims. We do
	// this by:
	// 1. Grabbing a map whose keys represent all well-known claim fields.
	// 2. Deleting any well-known fields that we want to propagate.
	// 3. Using the resulting map keys to filter the claim's spec.
	wellKnownClaimFields := xcrd.CompositeResourceClaimSpecProps(nil)
	for _, field := range xcrd.PropagateSpecProps {
		delete(wellKnownClaimFields, field)
	}

	// CompositionRevisionRef is a special field which needs to be propagated
	// based on the Update policy. If the policy is `Manual`, we need to remove
	// CompositionRevisionRef from wellKnownClaimFields, so it is propagated
	// from the claim to the XR.
	if xr.GetCompositionUpdatePolicy() != nil && *xr.GetCompositionUpdatePolicy() == xpv1.UpdateManual {
		delete(wellKnownClaimFields, xcrd.CompositionRevisionRef)
	}

	cmSpec, ok := cm.Object["spec"].(map[string]any)
	if !ok {
		return errors.New(errUnsupportedClaimSpec)
	}

	// Propagate the claim's spec (minus well known fields) to the XR's spec.
	xr.Object["spec"] = withoutKeys(cmSpec, xcrd.GetPropFields(wellKnownClaimFields)...)

	// We overwrite the entire XR spec above, so we wait until this point to set
	// the claim reference.
	xr.SetClaimReference(cm.GetReference())

	// If the claim references an XR, make sure we're going to apply that XR. We
	// do this just in case the XR exists, but we couldn't get it due to a stale
	// cache.
	if ref := cm.GetResourceReference(); ref != nil {
		xr.SetName(ref.Name)
	}

	// If the XR doesn't exist, derive a name from the claim. The generated name
	// is likely (but not guaranteed) to be available when we create the XR. If
	// taken, then we are going to update an existing XR, probably hijacking it
	// from another claim.
	if !meta.WasCreated(xr) {
		xr.SetGenerateName(fmt.Sprintf("%s-", cm.GetName()))

		// GenerateName is a no-op if the xr already has a name set.
		if err := s.names.GenerateName(ctx, xr); err != nil {
			return errors.Wrap(err, errGenerateName)
		}
	}

	// We're now done syncing the XR from the claim. If this is a new XR it's
	// important that we update the claim to reference it before we create it.
	// This ensures we don't leak an XR. We could leak an XR if we created an XR
	// then crashed before saving a reference to it. We'd create another XR on
	// the next reconcile.
	existing := cm.GetResourceReference()

	proposed := xr.GetReference()
	if !cmp.Equal(existing, proposed) {
		cm.SetResourceReference(proposed)

		if err := s.client.Update(ctx, cm); err != nil {
			return errors.Wrap(err, errUpdateClaim)
		}
	}

	// Apply the XR, unless it's a no-op change.
	err := s.client.Apply(ctx, xr, resource.AllowUpdateIf(func(old, obj runtime.Object) bool { return !cmp.Equal(old, obj) }))
	if err := resource.Ignore(resource.IsNotAllowed, err); err != nil {
		return errors.Wrap(err, errApplyComposite)
	}

	// Below this point we're syncing XR status -> claim status.

	// Merge the XR's status into the claim's status.
	if err := merge(cm.Object["status"], xr.Object["status"],
		// XR status fields overwrite non-empty claim fields.
		withMergeOptions(mergo.WithOverride),
		// Don't sync XR machinery (i.e. status conditions, connection details).
		withSrcFilter(xcrd.GetPropFields(xcrd.CompositeResourceStatusProps(common.CompositeResourceScopeLegacyCluster))...)); err != nil { //nolint:staticcheck // we are still supporting v1 XRD
		return errors.Wrap(err, errMergeClaimStatus)
	}

	if err := s.client.Status().Update(ctx, cm); err != nil {
		return errors.Wrap(err, errUpdateClaimStatus)
	}

	// Propagate the actual external name back from the XR to the claim if it's
	// set. The name we're propagating here will may be a name the XR must
	// enforce (i.e. overriding any requested by the claim) but will often
	// actually just be propagating back a name that was already propagated
	// forward from the claim to the XR earlier in this method.
	if en := meta.GetExternalName(xr); en != "" {
		meta.SetExternalName(cm, en)
	}

	// We want to propagate the XR's spec to the claim's spec, but first we must
	// filter out any well-known fields that are unique to XR. We do this by:
	// 1. Grabbing a map whose keys represent all well-known XR fields.
	// 2. Deleting any well-known fields that we want to propagate.
	// 3. Filtering OUT the remaining map keys from the XR's spec so that we end
	//    up adding only the well-known fields to the claim's spec.
	wellKnownXRFields := xcrd.CompositeResourceSpecProps(common.CompositeResourceScopeLegacyCluster, nil) //nolint:staticcheck // we are still supporting v1 XRD
	for _, field := range xcrd.PropagateSpecProps {
		delete(wellKnownXRFields, field)
	}

	// CompositionRevisionRef is a special field which needs to be propagated
	// based on the Update policy. If the policy is `Automatic`, we need to
	// overwrite the claim's value with the XR's which should be the
	// `currentRevision`
	if xr.GetCompositionUpdatePolicy() != nil && *xr.GetCompositionUpdatePolicy() == xpv1.UpdateAutomatic {
		cm.SetCompositionRevisionReference(xr.GetCompositionRevisionReference())
	}

	// Propagate the XR's spec (minus well known fields) to the claim's spec.
	if err := merge(cm.Object["spec"], xr.Object["spec"],
		withSrcFilter(xcrd.GetPropFields(wellKnownXRFields)...)); err != nil {
		return errors.Wrap(err, errMergeClaimSpec)
	}

	return errors.Wrap(s.client.Update(ctx, cm), errUpdateClaim)
}
