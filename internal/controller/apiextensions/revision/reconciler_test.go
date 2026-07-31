/*
Copyright 2025 The Crossplane Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/name"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/features"
	"github.com/crossplane/crossplane/v2/internal/xfn"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")

	const pkg = "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	enabled := &feature.Flags{}
	enabled.Enable(features.EnableAlphaPipelineOCIReferences)

	// listControlledRevision lists an active FunctionRevision controlled by the
	// test-function Function, so that name-based steps resolve.
	listControlledRevision := test.NewMockListFn(nil, func(obj client.ObjectList) error {
		obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-function-abc123",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: pkgv1.FunctionGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.FunctionKind,
					Name:       "test-function",
					UID:        "test-function-uid",
					Controller: new(true),
				}},
			},
			Spec: pkgv1.FunctionRevisionSpec{
				PackageRevisionSpec: pkgv1.PackageRevisionSpec{
					DesiredState: pkgv1.PackageRevisionActive,
					Package:      "example.org/test-function:v1",
				},
			},
		}}
		return nil
	})

	type params struct {
		mgr  manager.Manager
		opts []ReconcilerOption
	}

	type args struct {
		ctx context.Context
		req reconcile.Request
	}

	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"CompositionRevisionNotFound": {
			reason: "We should not return an error if the CompositionRevision was not found.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(_ client.Object) error {
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}),
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ValidPipeline": {
			reason: "We should mark the CompositionRevision as having a valid pipeline when all functions have the composition capability.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							switch o := obj.(type) {
							case *v1.CompositionRevision:
								*o = v1.CompositionRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: "test-revision",
									},
									Spec: v1.CompositionRevisionSpec{
										Pipeline: []v1.PipelineStep{
											{
												Step: "test-step",
												FunctionRef: &v1.FunctionReference{
													Name: "test-function",
												},
											},
										},
									},
								}
							case *pkgv1.Function:
								o.SetName("test-function")
								o.SetUID("test-function-uid")
								o.Spec.Package = "example.org/test-function:v1"
							}
							return nil
						}),
						MockList:         listControlledRevision,
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLogger(logging.NewNopLogger()),
					WithRecorder(event.NewNopRecorder()),
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, revs ...string) error {
						if diff := cmp.Diff([]string{"test-function-abc123"}, revs); diff != "" {
							t.Errorf("CheckCapabilities(): -want, +got:\n%s", diff)
						}
						return nil
					})),
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-revision"}},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"InvalidPipeline": {
			reason: "We should mark the CompositionRevision as having an invalid pipeline when functions lack the composition capability.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							switch o := obj.(type) {
							case *v1.CompositionRevision:
								*o = v1.CompositionRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: "test-revision",
									},
									Spec: v1.CompositionRevisionSpec{
										Pipeline: []v1.PipelineStep{
											{
												Step: "test-step",
												FunctionRef: &v1.FunctionReference{
													Name: "test-function",
												},
											},
										},
									},
								}
							case *pkgv1.Function:
								o.SetName("test-function")
								o.SetUID("test-function-uid")
								o.Spec.Package = "example.org/test-function:v1"
							}
							return nil
						}),
						MockList:         listControlledRevision,
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLogger(logging.NewNopLogger()),
					WithRecorder(event.NewNopRecorder()),
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return errBoom
					})),
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-revision"}},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MissingFunctionRefFeatureDisabled": {
			reason: "We should mark the CompositionRevision as having an invalid pipeline, without installing functions, when a step has no functionRef and pipeline OCI references are disabled.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*v1.CompositionRevision); ok {
								*o = v1.CompositionRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: "test-revision",
									},
									Spec: v1.CompositionRevisionSpec{
										Pipeline: []v1.PipelineStep{
											{
												Step:     "test-step",
												Function: pkg,
											},
										},
									},
								}
							}
							return nil
						}),
						MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
							t.Errorf("Create(): unexpected call for %T", obj)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
							t.Errorf("Update(): unexpected call for %T", obj)
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
							got := obj.(*v1.CompositionRevision).Status.GetCondition(v1.TypeValidPipeline)
							if got.Reason != v1.ReasonInvalidPipeline {
								t.Errorf("Status().Update(): want %s condition reason %q, got %q", v1.TypeValidPipeline, v1.ReasonInvalidPipeline, got.Reason)
							}
							return nil
						}),
					},
				},
				opts: []ReconcilerOption{
					WithLogger(logging.NewNopLogger()),
					WithRecorder(event.NewNopRecorder()),
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						t.Errorf("CheckCapabilities(): unexpected call")
						return nil
					})),
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-revision"}},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ValidPipelineOCIRefFeatureEnabled": {
			reason: "We should install functions and mark the CompositionRevision as having a valid pipeline when a step references a function by OCI reference and pipeline OCI references are enabled.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							switch o := obj.(type) {
							case *v1.CompositionRevision:
								*o = v1.CompositionRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: "test-revision",
									},
									Spec: v1.CompositionRevisionSpec{
										Pipeline: []v1.PipelineStep{
											{
												Step:     "test-step",
												Function: pkg,
											},
										},
									},
								}

							case *pkgv1.FunctionRevision:
								*o = pkgv1.FunctionRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: "function-patch-and-transform-external",
									},
									Spec: pkgv1.FunctionRevisionSpec{
										PackageRevisionSpec: pkgv1.PackageRevisionSpec{
											DesiredState: pkgv1.PackageRevisionActive,
											Package:      pkg,
										},
									},
								}

							case *pkgv1.Function:
								*o = pkgv1.Function{
									ObjectMeta: metav1.ObjectMeta{
										Name: "function-patch-and-transform",
									},
									Spec: pkgv1.FunctionSpec{
										PackageSpec: pkgv1.PackageSpec{
											Package: pkg,
										},
									},
								}
							}

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
							got := obj.(*v1.CompositionRevision).Status.GetCondition(v1.TypeValidPipeline)
							if got.Reason != v1.ReasonValidPipeline {
								t.Errorf("Status().Update(): want %s condition reason %q, got %q", v1.TypeValidPipeline, v1.ReasonValidPipeline, got.Reason)
							}
							return nil
						}),
					},
				},
				opts: []ReconcilerOption{
					WithLogger(logging.NewNopLogger()),
					WithRecorder(event.NewNopRecorder()),
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, revs ...string) error {
						if diff := cmp.Diff([]string{"function-patch-and-transform-external"}, revs); diff != "" {
							t.Errorf("CheckCapabilities(): -want, +got:\n%s", diff)
						}
						return nil
					})),
					WithFeatures(enabled),
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-revision"}},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.params.mgr, tc.params.opts...)
			got, err := r.Reconcile(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestEnsureFunctions(t *testing.T) {
	const pkg = "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	rev := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{Name: "example-rev", UID: "rev-uid"},
		Spec: v1.CompositionRevisionSpec{
			Pipeline: []v1.PipelineStep{
				{Step: "pnt", Function: pkg},
			},
		},
	}

	t.Run("CreatesFunctionRevisionAndFunction", func(t *testing.T) {
		var createdRev *pkgv1.FunctionRevision
		var createdFn *pkgv1.Function

		c := &test.MockClient{
			MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
			MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
				switch o := obj.(type) {
				case *pkgv1.FunctionRevision:
					createdRev = o
				case *pkgv1.Function:
					createdFn = o
				}
				return nil
			}),
		}

		r := NewReconciler(&fake.Manager{Client: c})
		if err := r.ensureFunctions(context.Background(), rev); err != nil {
			t.Fatalf("ensureFunctions(...): unexpected error: %v", err)
		}

		if createdRev == nil {
			t.Fatal("ensureFunctions(...): expected a FunctionRevision to be created")
		}
		if diff := cmp.Diff(pkg, createdRev.Spec.Package); diff != "" {
			t.Errorf("created FunctionRevision package: -want, +got:\n%s", diff)
		}
		if diff := cmp.Diff(pkgv1.PackageRevisionActive, createdRev.Spec.DesiredState); diff != "" {
			t.Errorf("created FunctionRevision desired state: -want, +got:\n%s", diff)
		}
		if createdRev.Spec.TLSServerSecretName == nil {
			t.Error("created FunctionRevision should have a TLS server secret name")
		}
		if !hasOwner(createdRev.GetOwnerReferences(), rev.GetUID()) {
			t.Error("created FunctionRevision should be owned by the CompositionRevision")
		}

		if createdFn == nil {
			t.Fatal("ensureFunctions(...): expected a Function to be created")
		}
		if createdFn.Spec.Package != "" {
			t.Errorf("created Function should have an empty package, got %q", createdFn.Spec.Package)
		}
		if !hasOwner(createdFn.GetOwnerReferences(), rev.GetUID()) {
			t.Error("created Function should be owned by the CompositionRevision")
		}
	})

	otherRevOwner := metav1.OwnerReference{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       v1.CompositionRevisionKind,
		Name:       "other-rev",
		UID:        "other-rev",
	}

	t.Run("AdoptsExistingFunctionRevision", func(t *testing.T) {
		var updated []client.Object

		c := &test.MockClient{
			// Both the FunctionRevision and the Function already exist, owned
			// by a different CompositionRevision. The FunctionRevision has
			// somehow been deactivated.
			MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				switch o := obj.(type) {
				case *pkgv1.FunctionRevision:
					o.SetOwnerReferences([]metav1.OwnerReference{otherRevOwner})
					o.Spec.Package = pkg
					o.Spec.DesiredState = pkgv1.PackageRevisionInactive
				case *pkgv1.Function:
					o.SetOwnerReferences([]metav1.OwnerReference{otherRevOwner})
				}
				return nil
			}),
			MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
				updated = append(updated, obj.DeepCopyObject().(client.Object))
				return nil
			}),
		}

		r := NewReconciler(&fake.Manager{Client: c})
		if err := r.ensureFunctions(context.Background(), rev); err != nil {
			t.Fatalf("ensureFunctions(...): unexpected error: %v", err)
		}

		// Both the FunctionRevision and the Function should be updated to add
		// our CompositionRevision as an additional owner.
		if len(updated) != 2 {
			t.Fatalf("expected 2 updates (FunctionRevision and Function), got %d", len(updated))
		}
		for _, o := range updated {
			if !hasOwner(o.GetOwnerReferences(), rev.GetUID()) {
				t.Errorf("%T should have been updated with our owner reference", o)
			}
			if !hasOwner(o.GetOwnerReferences(), otherRevOwner.UID) {
				t.Errorf("%T should have kept its existing owner reference", o)
			}
			if fr, ok := o.(*pkgv1.FunctionRevision); ok && fr.Spec.DesiredState != pkgv1.PackageRevisionActive {
				t.Errorf("FunctionRevision should have been updated to be active, got %q", fr.Spec.DesiredState)
			}
		}
	})

	t.Run("DoesNotOwnUnownedFunctions", func(t *testing.T) {
		var updatedRev *pkgv1.FunctionRevision
		var updatedFn *pkgv1.Function

		c := &test.MockClient{
			MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				switch o := obj.(type) {
				case *pkgv1.FunctionRevision:
					o.Spec.Package = pkg
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

		r := NewReconciler(&fake.Manager{Client: c})
		if err := r.ensureFunctions(context.Background(), rev); err != nil {
			t.Fatalf("ensureFunctions(...): unexpected error: %v", err)
		}

		if updatedRev == nil {
			t.Fatal("expected the FunctionRevision to be updated")
		}
		if hasOwner(updatedRev.GetOwnerReferences(), rev.GetUID()) {
			t.Error("FunctionRevision should not have been updated with our owner reference")
		}
		if updatedFn != nil {
			t.Error("Function should not have been updated")
		}
	})

	t.Run("RejectsFunctionRevisionForDifferentPackage", func(t *testing.T) {
		c := &test.MockClient{
			MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
				if o, ok := obj.(*pkgv1.FunctionRevision); ok {
					o.SetOwnerReferences([]metav1.OwnerReference{otherRevOwner})
					o.Spec.Package = "xpkg.crossplane.io/someone-else/function-patch-and-transform@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
				}
				return nil
			}),
			MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
				t.Errorf("Update(): unexpected call for %T", obj)
				return nil
			}),
		}

		r := NewReconciler(&fake.Manager{Client: c})
		if err := r.ensureFunctions(context.Background(), rev); err == nil {
			t.Error("ensureFunctions(...): expected an error for a FunctionRevision with a different package")
		}
	})

	t.Run("NormalizesPackage", func(t *testing.T) {
		// A step that references the package with a tag should create the
		// same FunctionRevision, with the same normalized package, as one that
		// doesn't.
		tagged := "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

		created := map[string]*pkgv1.FunctionRevision{}
		for _, p := range []string{pkg, tagged} {
			rev := &v1.CompositionRevision{
				ObjectMeta: metav1.ObjectMeta{Name: "example-rev", UID: "rev-uid"},
				Spec: v1.CompositionRevisionSpec{
					Pipeline: []v1.PipelineStep{{Step: "pnt", Function: p}},
				},
			}

			c := &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
					if o, ok := obj.(*pkgv1.FunctionRevision); ok {
						created[p] = o
					}
					return nil
				}),
			}

			r := NewReconciler(&fake.Manager{Client: c})
			if err := r.ensureFunctions(context.Background(), rev); err != nil {
				t.Fatalf("ensureFunctions(...): unexpected error: %v", err)
			}
		}

		if diff := cmp.Diff(created[pkg].GetName(), created[tagged].GetName()); diff != "" {
			t.Errorf("FunctionRevision names: -untagged, +tagged:\n%s", diff)
		}
		if diff := cmp.Diff(pkg, created[tagged].Spec.Package); diff != "" {
			t.Errorf("FunctionRevision package: -want, +got:\n%s", diff)
		}
	})

	t.Run("NamesIncludeRegistry", func(t *testing.T) {
		// The same repository and digest from different registries should
		// get different FunctionRevisions.
		a, err := name.NewDigest(pkg, name.StrictValidation)
		if err != nil {
			t.Fatal(err)
		}
		b, err := name.NewDigest("registry.example.org/crossplane-contrib/function-patch-and-transform@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890", name.StrictValidation)
		if err != nil {
			t.Fatal(err)
		}

		if composite.FunctionRevisionName(a) == composite.FunctionRevisionName(b) {
			t.Errorf("functionRevisionName(...): want different names for different registries, got %q for both", composite.FunctionRevisionName(a))
		}
	})

	t.Run("RejectsReferenceWithoutDigest", func(t *testing.T) {
		// The API server enforces this too, but a step could reference a
		// package by tag if the CEL validation is bypassed, e.g. by a
		// CompositionRevision created before we required digests.
		for reason, pkg := range map[string]string{
			"Tag":          "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2",
			"NoIdentifier": "xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform",
		} {
			t.Run(reason, func(t *testing.T) {
				rev := &v1.CompositionRevision{
					ObjectMeta: metav1.ObjectMeta{Name: "example-rev", UID: "rev-uid"},
					Spec: v1.CompositionRevisionSpec{
						Pipeline: []v1.PipelineStep{{Step: "pnt", Function: pkg}},
					},
				}

				c := &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(_ client.Object) error { return nil }),
				}

				r := NewReconciler(&fake.Manager{Client: c})
				if err := r.ensureFunctions(context.Background(), rev); err == nil {
					t.Errorf("ensureFunctions(...): expected an error for package %q", pkg)
				}
			})
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
