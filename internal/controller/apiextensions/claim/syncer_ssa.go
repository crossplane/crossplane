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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/shared"
	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings.
const (
	errCreatePatch                = "cannot create patch"
	errPatchFieldManagers         = "cannot patch field managers"
	errUnsupportedCompositeStatus = "composite resource status was not an object"
)

// Server-side-apply field owners.
const (
	// FieldOwnerXR owns the fields this controller mutates on composite
	// resources (XRs).
	FieldOwnerXR = "apiextensions.crossplane.io/claim"
)

// A NopManagedFieldsUpgrader does nothing.
type NopManagedFieldsUpgrader struct{}

// Upgrade does nothing.
func (u *NopManagedFieldsUpgrader) Upgrade(_ context.Context, _ client.Object, _ string) error {
	return nil
}

// A PatchingManagedFieldsUpgrader uses a JSON patch to upgrade an object's
// managed fields from client-side to server-side apply. The upgrade is a no-op
// if the object does not need upgrading.
type PatchingManagedFieldsUpgrader struct {
	client client.Writer
}

// NewPatchingManagedFieldsUpgrader returns a ManagedFieldsUpgrader that uses a
// JSON patch to upgrade and object's managed fields from client-side to
// server-side apply.
func NewPatchingManagedFieldsUpgrader(w client.Writer) *PatchingManagedFieldsUpgrader {
	return &PatchingManagedFieldsUpgrader{client: w}
}

// Upgrade the supplied object's field managers from client-side to server-side
// apply.
//
// This is a multi-step process.
//
// Step 1: All fields are owned by either manager 'crossplane', operation
// 'Update' or manager 'apiextensions.crossplane.io/composite', operation
// 'Apply'. This represents all fields set by the claim or XR controller up to
// this point.
//
// Step 2: Upgrade is called for the first time. We delete all field managers.
//
// Step 3: The claim controller server-side applies its fully specified intent
// as field manager 'apiextensions.crossplane.io/claim'. This becomes the
// manager of all the fields that are part of the claim controller's fully
// specified intent. All existing fields the claim controller didn't specify
// become owned by a special manager - 'before-first-apply', operation 'Update'.
//
// Step 4: Upgrade is called for the second time. It deletes the
// 'before-first-apply' field manager entry. Only the claim field manager
// remains.
//
// Step 5: Eventually the XR reconciler updates a field (e.g. spec.resourceRefs)
// and becomes owner of that field.
func (u *PatchingManagedFieldsUpgrader) Upgrade(ctx context.Context, obj client.Object, ssaManager string) error {
	// The XR doesn't exist, nothing to upgrade.
	if !meta.WasCreated(obj) {
		return nil
	}

	foundSSA := false
	foundBFA := false
	idxBFA := -1

	for i, e := range obj.GetManagedFields() {
		if e.Manager == ssaManager {
			foundSSA = true
		}

		if e.Manager == "before-first-apply" {
			foundBFA = true
			idxBFA = i
		}
	}

	switch {
	// If our SSA field manager exists and the before-first-apply field manager
	// doesn't, we've already done the upgrade. Don't do it again.
	case foundSSA && !foundBFA:
		return nil

	// We found our SSA field manager but also before-first-apply. It should now
	// be safe to delete before-first-apply.
	case foundSSA && foundBFA:
		p := []byte(fmt.Sprintf(`[
			{"op":"remove","path":"/metadata/managedFields/%d"},
			{"op":"replace","path":"/metadata/resourceVersion","value":"%s"}
		]`, idxBFA, obj.GetResourceVersion()))

		return errors.Wrap(resource.IgnoreNotFound(u.client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, p))), "cannot remove before-first-apply from field managers")

	// We didn't find our SSA field manager or the before-first-apply field
	// manager. This means we haven't started the upgrade. The first thing we
	// want to do is clear all managed fields. After we do this we'll let our
	// SSA field manager apply the fields it cares about. The result will be
	// that our SSA field manager shares ownership with a new manager named
	// 'before-first-apply'.
	default:
		p := []byte(fmt.Sprintf(`[
			{"op":"replace","path": "/metadata/managedFields","value": [{}]},
			{"op":"replace","path":"/metadata/resourceVersion","value":"%s"}
		]`, obj.GetResourceVersion()))

		return errors.Wrap(resource.IgnoreNotFound(u.client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, p))), "cannot clear field managers")
	}
}

// A ServerSideCompositeSyncer binds and syncs a claim with a composite resource
// (XR). It uses server-side apply to update the XR.
type ServerSideCompositeSyncer struct {
	client client.Client
	names  names.NameGenerator
}

// NewServerSideCompositeSyncer returns a CompositeSyncer that uses server-side
// apply to sync a claim with a composite resource.
func NewServerSideCompositeSyncer(c client.Client, ng names.NameGenerator) *ServerSideCompositeSyncer {
	return &ServerSideCompositeSyncer{client: c, names: ng}
}

// Sync the supplied claim with the supplied composite resource (XR). Syncing
// may involve creating and binding the XR.
func (s *ServerSideCompositeSyncer) Sync(ctx context.Context, cm *claim.Unstructured, xr *composite.Unstructured) error {
	// First we sync claim -> XR.

	// Create an empty XR patch object. We'll use this object to ensure we only
	// SSA our desired state, not the state we previously read from the API
	// server.
	xrPatch := composite.New(composite.WithGroupVersionKind(xr.GroupVersionKind()), composite.WithSchema(composite.SchemaLegacy))

	// If the claim references an XR, make sure we're going to apply that XR. We
	// do this instead of using the supplied XR's name just in case the XR
	// exists, but we couldn't get it due to a stale cache.
	if ref := cm.GetResourceReference(); ref != nil {
		xrPatch.SetName(ref.Name)
	}

	// If the XR doesn't have a name (i.e. doesn't exist), derive a name from
	// the claim. The generated name is likely (but not guaranteed) to be
	// available when we create the XR. If taken, then we are going to update an
	// existing XR, probably hijacking it from another claim.
	if xrPatch.GetName() == "" {
		xrPatch.SetGenerateName(fmt.Sprintf("%s-", cm.GetName()))

		if err := s.names.GenerateName(ctx, xrPatch); err != nil {
			return errors.Wrap(err, errGenerateName)
		}
	}

	// It's possible we're being asked to configure a statically provisioned XR.
	// We should respect its existing external name annotation.
	en := meta.GetExternalName(xr)

	// Do not propagate *.kubernetes.io annotations/labels from claim to XR. For
	// example kubectl.kubernetes.io/last-applied-configuration should not be
	// propagated.
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/
	// for all annotations and their semantic
	if ann := withoutReservedK8sEntries(cm.GetAnnotations()); len(ann) > 0 {
		meta.AddAnnotations(xrPatch, withoutReservedK8sEntries(cm.GetAnnotations()))
	}

	meta.AddLabels(xrPatch, withoutReservedK8sEntries(cm.GetLabels()))
	meta.AddLabels(xrPatch, map[string]string{
		xcrd.LabelKeyClaimName:      cm.GetName(),
		xcrd.LabelKeyClaimNamespace: cm.GetNamespace(),
	})

	// Restore the XR's original external name annotation in order to ensure we
	// don't try to rename anything after the fact.
	if en != "" {
		meta.SetExternalName(xrPatch, en)
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

	// Propagate composition revision ref from the claim if the update policy is
	// manual. When the update policy is manual the claim controller is
	// authoritative for this field. See below for the automatic case.
	if xr.GetCompositionUpdatePolicy() != nil && *xr.GetCompositionUpdatePolicy() == xpv1.UpdateManual {
		delete(wellKnownClaimFields, xcrd.CompositionRevisionRef)
	}

	cmSpec, ok := cm.Object["spec"].(map[string]any)
	if !ok {
		return errors.New(errUnsupportedClaimSpec)
	}

	// Propagate the claim's spec (minus well known fields) to the XR's spec.
	xrPatch.Object["spec"] = withoutKeys(cmSpec, xcrd.GetPropFields(wellKnownClaimFields)...)

	// We overwrite the entire XR spec above, so we wait until this point to set
	// the claim reference.
	xrPatch.SetClaimReference(cm.GetReference())

	// Below this point we're syncing XR -> claim.

	// Bind the claim to the XR. If this is a new XR it's important that we
	// apply the claim before we create it. This ensures we don't leak an XR. We
	// could leak an XR if we created an XR then crashed before saving a
	// reference to it. We'd create another XR on the next reconcile.
	cm.SetResourceReference(xrPatch.GetReference())

	// Propagate the actual external name back from the composite to the
	// claim if it's set. The name we're propagating here will may be a name
	// the XR must enforce (i.e. overriding any requested by the claim) but
	// will often actually just be propagating back a name that was already
	// propagated forward from the claim to the XR during the
	// preceding configure phase.
	if en := meta.GetExternalName(xr); en != "" {
		meta.SetExternalName(cm, en)
	}

	// Propagate composition ref from the XR if the claim doesn't have an
	// opinion. Composition and revision selectors only propagate from claim ->
	// XR. When a claim has selectors **and no reference** the flow should be:
	//
	// 1. Claim controller propagates selectors claim -> XR.
	// 2. XR controller uses selectors to set XR's composition ref.
	// 3. Claim controller propagates ref XR -> claim.
	//
	// When a claim sets a composition ref, it supercedes selectors. It should
	// only be propagated claim -> XR.
	if ref := xr.GetCompositionReference(); ref != nil && cm.GetCompositionReference() == nil {
		cm.SetCompositionReference(ref)
	}

	// Propagate composition revision ref from the XR if the update policy is
	// automatic. When the update policy is automatic the XR controller is
	// authoritative for this field. It will update the XR's ref as new
	// revisions become available, and we want to propgate the ref XR -> claim.
	if p := xr.GetCompositionUpdatePolicy(); p != nil && *p == xpv1.UpdateAutomatic && xr.GetCompositionRevisionReference() != nil {
		cm.SetCompositionRevisionReference(xr.GetCompositionRevisionReference())
	}

	// It's important that we update the claim before we apply the XR, to make
	// sure the claim's resourceRef points to the XR before we create the XR.
	// Otherwise we risk leaking an XR.
	//
	// It's also important that the API server will reject this update if we're
	// reconciling an old claim, e.g. due to a stale cache. It's possible that
	// we're seeing an old version without a resourceRef set, but in reality an
	// XR has already been created. We don't want to leak it and create another.
	if err := s.client.Update(ctx, cm); err != nil {
		return errors.Wrap(err, errUpdateClaim)
	}

	if err := s.client.Patch(ctx, xrPatch, client.Apply, client.ForceOwnership, client.FieldOwner(FieldOwnerXR)); err != nil {
		return errors.Wrap(err, errApplyComposite)
	}

	// Update the XR passed to this method to reflect the state returned by the
	// API server when we patched it.
	xr.Object = xrPatch.Object

	m, ok := xr.Object["status"]
	if !ok {
		// If the XR doesn't have a status yet there's nothing else to sync.
		// Just update the claim passed to this method to reflect the state
		// returned by the API server when we patched it.
		return nil
	}

	xrStatus, ok := m.(map[string]any)
	if !ok {
		return errors.New(errUnsupportedCompositeStatus)
	}

	// Preserve Crossplane machinery, like status conditions.
	cmcs := xpv1.ConditionedStatus{}
	_ = fieldpath.Pave(cm.Object).GetValueInto("status", &cmcs)
	pub := cm.GetConnectionDetailsLastPublishedTime()

	// Update the claim's user-defined status fields to match the XRs.
	cm.Object["status"] = withoutKeys(xrStatus, xcrd.GetPropFields(xcrd.CompositeResourceStatusProps(shared.CompositeResourceScopeLegacyCluster))...) //nolint:staticcheck // we are sll supporting v1 XRD

	if cmcs.Conditions != nil {
		cm.SetConditions(cmcs.Conditions...)
	}

	if pub != nil {
		cm.SetConnectionDetailsLastPublishedTime(pub)
	}

	return errors.Wrap(s.client.Status().Update(ctx, cm), errUpdateClaimStatus)
}
