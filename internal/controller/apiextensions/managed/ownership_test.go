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

package managed

import (
	"context"
	"testing"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/ssa"
)

func TestReconcileCRDOwnership(t *testing.T) {
	mrd := newMRD()
	installer := meta.AsController(meta.TypedReferenceTo(&pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "provider", UID: "provider-uid"}}, pkgv1.ProviderRevisionGroupVersionKind))
	provider := meta.AsOwner(meta.TypedReferenceTo(&pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "provider", UID: "package-uid"}}, pkgv1.ProviderGroupVersionKind))
	mrd.OwnerReferences = []metav1.OwnerReference{installer, provider}
	mrdOwner := meta.AsOwner(meta.TypedReferenceTo(mrd, v1alpha1.ManagedResourceDefinitionGroupVersionKind))
	wrongUID := mrdOwner
	wrongUID.UID = "old-mrd-uid"
	wrongName := mrdOwner
	wrongName.Name = "another-mrd"
	wrongKind := mrdOwner
	wrongKind.Kind = "CompositeResourceDefinition"
	oldInstaller := installer
	oldInstaller.Name = "old-revision"
	oldInstaller.UID = "old-revision-uid"
	foreign := installer
	foreign.Kind = "UnrelatedController"
	for name, tc := range map[string]struct {
		owners                []metav1.OwnerReference
		installer             *metav1.OwnerReference
		absent, allowed, race bool
	}{
		"Create":                 {absent: true, allowed: true},
		"CreateRace":             {absent: true, allowed: true, race: true},
		"Unowned":                {},
		"OwnedByMRD":             {owners: []metav1.OwnerReference{mrdOwner}, allowed: true},
		"RevisionHandoff":        {owners: []metav1.OwnerReference{mrdOwner, foreign}, allowed: true},
		"MigrateDirectCRD":       {owners: []metav1.OwnerReference{installer}, allowed: true},
		"MigrateAcrossRevisions": {owners: []metav1.OwnerReference{provider, oldInstaller}, allowed: true},
		"WrongUID":               {owners: []metav1.OwnerReference{wrongUID}},
		"WrongName":              {owners: []metav1.OwnerReference{wrongName}},
		"WrongKind":              {owners: []metav1.OwnerReference{wrongKind}},
		"ForeignInstaller":       {owners: []metav1.OwnerReference{foreign}, installer: &foreign},
		"UpdateRace":             {owners: []metav1.OwnerReference{mrdOwner}, allowed: true, race: true},
	} {
		t.Run(name, func(t *testing.T) {
			parent := mrd.DeepCopy()
			if tc.installer != nil {
				parent.OwnerReferences = []metav1.OwnerReference{*tc.installer}
			}
			writes := 0
			c := &test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ManagedResourceDefinition:
						*o = *parent.DeepCopy()
					case *extv1.CustomResourceDefinition:
						if tc.absent {
							return kerrors.NewNotFound(schema.GroupResource{}, parent.Name)
						}
						o.ObjectMeta = metav1.ObjectMeta{Name: parent.Name, UID: "crd-uid", ResourceVersion: "123", OwnerReferences: tc.owners}
					}
					return nil
				},
				MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
					writes++
					if !tc.absent {
						t.Error("must update an existing CRD")
					}
					if tc.race {
						return kerrors.NewAlreadyExists(schema.GroupResource{}, parent.Name)
					}
					return nil
				},
				MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
					writes++
					if tc.absent {
						t.Error("must not upsert a CRD that was absent")
					}
					if obj.GetResourceVersion() != "123" {
						t.Error("apply must preserve the checked resource version")
					}
					if tc.race {
						return kerrors.NewConflict(schema.GroupResource{}, parent.Name, nil)
					}
					return nil
				},
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
			}
			r := &Reconciler{client: c, managedFields: &ssa.NopManagedFieldsUpgrader{}, log: logging.NewNopLogger(), record: event.NewNopRecorder(), conditions: conditions.ObservedGenerationPropagationManager{}}
			_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(parent)})
			if got, want := err != nil, !tc.allowed || tc.race; got != want {
				t.Errorf("error = %v, want error %v", err, want)
			}
			wantWrites := 0
			if tc.allowed {
				wantWrites = 1
			}
			if writes != wantWrites {
				t.Errorf("CRD writes = %d, want %d", writes, wantWrites)
			}
		})
	}
}
