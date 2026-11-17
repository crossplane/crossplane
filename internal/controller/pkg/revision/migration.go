/*
Copyright 2026 The Crossplane Authors.

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

package revision

import (
	"context"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

// establishMRDCRDOwnership links an established package MRD to an existing CRD.
// This runs after writing the MRD from package content. It skips a CRD it
// can't link, leaving the MRD controller to block that MRD rather than
// failing the rest of the package.
func (e *APIEstablisher) establishMRDCRDOwnership(ctx context.Context, obj, parent client.Object) error {
	mrd, ok := obj.(*v1alpha1.ManagedResourceDefinition)
	if !ok {
		return nil
	}
	pr, ok := parent.(*v1.ProviderRevision)
	if !ok {
		return nil
	}

	// Only ownership matters here, so avoid reading the CRD's schema.
	crd := &metav1.PartialObjectMetadata{}
	crd.SetGroupVersionKind(extv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	if err := e.client.Get(ctx, client.ObjectKey{Name: mrd.GetName()}, crd); err != nil {
		return errors.Wrapf(resource.IgnoreNotFound(err), "cannot get CustomResourceDefinition %q for ManagedResourceDefinition", mrd.GetName())
	}
	if mrd.GetUID() == "" || pr.GetUID() == "" {
		return errors.Errorf("cannot link CustomResourceDefinition %q to ManagedResourceDefinition and ProviderRevision %q before both have UIDs", mrd.GetName(), pr.GetName())
	}
	mrdRef := meta.AsOwner(meta.TypedReferenceTo(mrd, v1alpha1.ManagedResourceDefinitionGroupVersionKind))
	prRef := meta.AsOwner(meta.TypedReferenceTo(pr, v1.ProviderRevisionGroupVersionKind))
	// The Provider path lets revisions that replaced the CRD's original one
	// adopt it.
	provider := metav1.GetControllerOf(pr)
	ownedByProvider := provider != nil &&
		schema.FromAPIVersionAndKind(provider.APIVersion, provider.Kind).GroupKind() == v1.ProviderGroupVersionKind.GroupKind() &&
		hasOwner(crd, *provider)
	if !hasOwner(crd, mrdRef) && !hasOwner(crd, prRef) && !ownedByProvider {
		return nil
	}

	before := crd.DeepCopy()
	changed := false
	for _, ref := range []metav1.OwnerReference{mrdRef, prRef} {
		if !hasOwner(crd, ref) {
			meta.AddOwnerReference(crd, ref)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	// The revision retains grants even while the MRD is inactive. Leave the
	// CRD's current controller and schema to the MRD controller, and reject
	// concurrent changes to the ownership we checked.
	return errors.Wrapf(e.client.Patch(ctx, crd, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})), "cannot add ManagedResourceDefinition and ProviderRevision %q as owners of CustomResourceDefinition %q", pr.GetName(), crd.GetName())
}

func hasOwner(obj metav1.Object, want metav1.OwnerReference) bool {
	if want.UID == "" {
		return false
	}
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == want.UID && ref.Name == want.Name &&
			schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind).GroupKind() == schema.FromAPIVersionAndKind(want.APIVersion, want.Kind).GroupKind() {
			return true
		}
	}
	return false
}
