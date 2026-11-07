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

package operation

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
)

const (
	testPkg = "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
)

func testRevName(t *testing.T) string {
	t.Helper()

	ref, err := name.NewDigest(testPkg, name.StrictValidation)
	if err != nil {
		t.Fatal(err)
	}

	return xcomposite.FunctionRevisionName(ref)
}

func TestEnsureFunctions(t *testing.T) {
	op := &v1alpha1.Operation{
		ObjectMeta: metav1.ObjectMeta{Name: "example-op", UID: "op-uid"},
		Spec: v1alpha1.OperationSpec{
			Pipeline: []v1alpha1.PipelineStep{{Step: "pnt", Function: testPkg}},
		},
	}

	otherOwner := metav1.OwnerReference{
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
		Kind:       v1alpha1.OperationKind,
		Name:       "other-op",
		UID:        "other-op",
	}

	t.Run("CreatesNormalizedFunctionRevision", func(t *testing.T) {
		var created *pkgv1.FunctionRevision

		tagged := &v1alpha1.Operation{
			ObjectMeta: op.ObjectMeta,
			Spec: v1alpha1.OperationSpec{
				Pipeline: []v1alpha1.PipelineStep{{Step: "pnt", Function: "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"}},
			},
		}

		c := &test.MockClient{
			MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
			MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
				if o, ok := obj.(*pkgv1.FunctionRevision); ok {
					created = o.DeepCopy()
				}
				return nil
			}),
		}

		r := NewReconciler(c)
		if err := r.ensureFunctions(context.Background(), tagged); err != nil {
			t.Fatalf("ensureFunctions(...): unexpected error: %v", err)
		}

		if created == nil {
			t.Fatal("ensureFunctions(...): expected a FunctionRevision to be created")
		}
		if diff := cmp.Diff(testRevName(t), created.GetName()); diff != "" {
			t.Errorf("created FunctionRevision name: -want, +got:\n%s", diff)
		}
		if diff := cmp.Diff(testPkg, created.Spec.Package); diff != "" {
			t.Errorf("created FunctionRevision package: -want, +got:\n%s", diff)
		}
		if !hasOwner(created.GetOwnerReferences(), op.GetUID()) {
			t.Error("created FunctionRevision should be owned by the Operation")
		}
	})

	t.Run("AdoptsExternalFunctionRevision", func(t *testing.T) {
		var updated []client.Object

		c := &test.MockClient{
			MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				switch o := obj.(type) {
				case *pkgv1.FunctionRevision:
					o.SetOwnerReferences([]metav1.OwnerReference{otherOwner})
					o.Spec.Package = testPkg
					o.Spec.DesiredState = pkgv1.PackageRevisionInactive
				case *pkgv1.Function:
					o.SetOwnerReferences([]metav1.OwnerReference{otherOwner})
				}
				return nil
			}),
			MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
				updated = append(updated, obj.DeepCopyObject().(client.Object))
				return nil
			}),
		}

		r := NewReconciler(c)
		if err := r.ensureFunctions(context.Background(), op); err != nil {
			t.Fatalf("ensureFunctions(...): unexpected error: %v", err)
		}

		if len(updated) != 2 {
			t.Fatalf("expected 2 updates (FunctionRevision and Function), got %d", len(updated))
		}
		for _, o := range updated {
			if !hasOwner(o.GetOwnerReferences(), op.GetUID()) {
				t.Errorf("%T should have been updated with our owner reference", o)
			}
			if fr, ok := o.(*pkgv1.FunctionRevision); ok && fr.Spec.DesiredState != pkgv1.PackageRevisionActive {
				t.Errorf("FunctionRevision should have been updated to be active, got %q", fr.Spec.DesiredState)
			}
		}
	})

	t.Run("DoesNotOwnOtherFunctions", func(t *testing.T) {
		var updatedRev *pkgv1.FunctionRevision
		var updatedFn *pkgv1.Function

		c := &test.MockClient{
			MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				switch o := obj.(type) {
				case *pkgv1.FunctionRevision:
					o.Spec.Package = testPkg
				case *pkgv1.Function:
					o.Spec.Package = "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2"
				}
				return nil
			}),
			MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
				switch o := obj.(type) {
				case *pkgv1.FunctionRevision:
					updatedRev = o.DeepCopy()
				case *pkgv1.Function:
					updatedFn = o.DeepCopy()
				}
				return nil
			}),
		}

		r := NewReconciler(c)
		if err := r.ensureFunctions(context.Background(), op); err != nil {
			t.Fatalf("ensureFunctions(...): unexpected error: %v", err)
		}

		if updatedRev == nil {
			t.Fatal("expected the FunctionRevision to be updated")
		}
		if hasOwner(updatedRev.GetOwnerReferences(), op.GetUID()) {
			t.Error("objects not created for a pipeline step should not be updated with our owner reference")
		}
		if updatedFn != nil {
			t.Error("Function should not have been updated")
		}
	})

	t.Run("RejectsFunctionRevisionForDifferentPackage", func(t *testing.T) {
		c := &test.MockClient{
			MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				if o, ok := obj.(*pkgv1.FunctionRevision); ok {
					o.Spec.Package = "registry.example.org/crossplane-contrib/function-patch-and-transform@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
				}
				return nil
			}),
			MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
				t.Errorf("Update(): unexpected call for %T", obj)
				return nil
			}),
		}

		r := NewReconciler(c)
		if err := r.ensureFunctions(context.Background(), op); err == nil {
			t.Error("ensureFunctions(...): expected an error for a FunctionRevision with a different package")
		}
	})
}

func hasOwner(refs []metav1.OwnerReference, uid types.UID) bool {
	for _, r := range refs {
		if r.UID == uid {
			return true
		}
	}
	return false
}
