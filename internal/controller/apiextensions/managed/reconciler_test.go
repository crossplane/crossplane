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

package managed

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/ssa"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()

	type args struct {
		c    client.Client
		opts []ReconcilerOption
	}
	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ResourceNotFound": {
			reason: "We should not return an error if the ManagedResourceDefinition was not found",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetMRDError": {
			reason: "We should return error encountered while getting the ManagedResourceDefinition",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRDBeingDeleted": {
			reason: "We should handle MRD deletion gracefully",
			args: args{
				c: &test.MockClient{
					MockGet: withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.SetDeletionTimestamp(&now)
					})),
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.SetDeletionTimestamp(&now)
						mrd.SetConditions(v1alpha1.TerminatingManaged())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MRDBeingDeletedStatusUpdateError": {
			reason: "We should handle status update errors during deletion",
			args: args{
				c: &test.MockClient{
					MockGet: withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.SetDeletionTimestamp(&now)
					})),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRDPaused": {
			reason: "We should not reconcile a paused MRD",
			args: args{
				c: &test.MockClient{
					MockGet: withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
					})),
				},
				opts: []ReconcilerOption{
					WithRecorder(&testRecorder{}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MRDInactiveState": {
			reason: "We should mark MRD as inactive when state is not active",
			args: args{
				c: &test.MockClient{
					MockGet: withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionInactive
					})),
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionInactive
						mrd.SetConditions(v1alpha1.InactiveManaged())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MRDActiveCRDGetError": {
			reason: "We should handle errors getting the CRD for managed fields upgrade",
			args: args{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							return withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
								mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
							}))(ctx, key, obj)
						case *extv1.CustomResourceDefinition:
							return errBoom
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
						mrd.SetConditions(v1alpha1.BlockedManaged().WithMessage("unable to get CustomResourceDefinition, see events"))
					})),
				},
				opts: []ReconcilerOption{
					WithRecorder(&testRecorder{}),
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRDActiveCRDPendingNotEstablished": {
			reason: "We should mark MRD as pending when CRD is not yet established",
			args: args{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							return withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
								mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
								mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
									Group: "example.com",
									Names: extv1.CustomResourceDefinitionNames{
										Plural: "databases",
										Kind:   "Database",
									},
									Scope: extv1.ClusterScoped,
									Versions: []v1alpha1.CustomResourceDefinitionVersion{
										{
											Name:    "v1",
											Served:  true,
											Storage: true,
											Schema: &v1alpha1.CustomResourceValidation{
												OpenAPIV3Schema: runtime.RawExtension{
													Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
												},
											},
										},
									},
								}
							}))(ctx, key, o)
						case *extv1.CustomResourceDefinition:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
					MockPatch: test.NewMockPatchFn(nil),
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
						mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema: &v1alpha1.CustomResourceValidation{
										OpenAPIV3Schema: runtime.RawExtension{
											Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
										},
									},
								},
							},
						}
						mrd.SetConditions(v1alpha1.PendingManaged())
					})),
				},
				opts: []ReconcilerOption{
					WithRecorder(&testRecorder{}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MRDActiveCRDApplyError": {
			reason: "We should handle errors applying the CRD",
			args: args{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							return withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
								mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
								mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
									Group: "example.com",
									Names: extv1.CustomResourceDefinitionNames{
										Plural: "databases",
										Kind:   "Database",
									},
									Scope: extv1.ClusterScoped,
									Versions: []v1alpha1.CustomResourceDefinitionVersion{
										{
											Name:    "v1",
											Served:  true,
											Storage: true,
											Schema: &v1alpha1.CustomResourceValidation{
												OpenAPIV3Schema: runtime.RawExtension{
													Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
												},
											},
										},
									},
								}
							}))(ctx, key, o)
						case *extv1.CustomResourceDefinition:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
					MockPatch: test.NewMockPatchFn(errBoom),
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
						mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema: &v1alpha1.CustomResourceValidation{
										OpenAPIV3Schema: runtime.RawExtension{
											Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
										},
									},
								},
							},
						}
						mrd.SetConditions(v1alpha1.BlockedManaged().WithMessage("unable to apply CustomResourceDefinition, see events"))
					})),
				},
				opts: []ReconcilerOption{
					WithRecorder(&testRecorder{}),
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRDActiveCRDEstablished": {
			reason: "We should mark MRD as established when CRD is established",
			args: args{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							return withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
								mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
								mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
									Group: "example.com",
									Names: extv1.CustomResourceDefinitionNames{
										Plural: "databases",
										Kind:   "Database",
									},
									Scope: extv1.ClusterScoped,
									Versions: []v1alpha1.CustomResourceDefinitionVersion{
										{
											Name:    "v1",
											Served:  true,
											Storage: true,
											Schema: &v1alpha1.CustomResourceValidation{
												OpenAPIV3Schema: runtime.RawExtension{
													Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
												},
											},
										},
									},
								}
							}))(ctx, key, o)
						case *extv1.CustomResourceDefinition:
							o.Name = key.Name
							o.Spec = extv1.CustomResourceDefinitionSpec{
								Group: "example.com",
								Names: extv1.CustomResourceDefinitionNames{
									Plural: "databases",
									Kind:   "Database",
								},
								Scope: extv1.ClusterScoped,
								Versions: []extv1.CustomResourceDefinitionVersion{
									{
										Name:    "v1",
										Served:  true,
										Storage: true,
										Schema: &extv1.CustomResourceValidation{
											OpenAPIV3Schema: &extv1.JSONSchemaProps{
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"spec": {
														Type: "object",
													},
												},
											},
										},
									},
								},
							}
							// Set established condition
							o.Status.Conditions = []extv1.CustomResourceDefinitionCondition{
								{
									Type:   extv1.Established,
									Status: extv1.ConditionTrue,
								},
							}
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
						mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema: &v1alpha1.CustomResourceValidation{
										OpenAPIV3Schema: runtime.RawExtension{
											Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
										},
									},
								},
							},
						}
						mrd.SetConditions(v1alpha1.EstablishedManaged())
					})),
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						// Simulate server response populating the unstructured object with CRD status
						u, ok := obj.(*unstructured.Unstructured)
						if !ok {
							return nil
						}
						// Set established status
						status := map[string]any{
							"conditions": []any{
								map[string]any{
									"type":   string(extv1.Established),
									"status": string(extv1.ConditionTrue),
								},
							},
						}
						u.Object["status"] = status
						return nil
					},
				},
				opts: []ReconcilerOption{
					WithRecorder(&testRecorder{}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MRDInactiveStatusUpdateError": {
			reason: "We should handle status update errors when marking MRD as inactive",
			args: args{
				c: &test.MockClient{
					MockGet: withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionInactive
					})),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Reconciler{
				client:        tc.args.c,
				managedFields: &ssa.NopManagedFieldsUpgrader{},
				log:           logging.NewNopLogger(),
				record:        event.NewNopRecorder(),
				conditions:    conditions.ObservedGenerationPropagationManager{},
			}

			for _, opt := range tc.args.opts {
				opt(r)
			}

			got, err := r.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-mrd"},
			})

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestReconcileCRDControllerHandoff covers the hand-off of a CRD between the
// package revisions that control the ManagedResourceDefinition it's derived
// from. The outgoing revision's controlling owner reference may have been
// written by a field manager that isn't ours, in which case server-side apply
// can't prune it and we have to declare it, demoted, to take it over.
func TestReconcileCRDControllerHandoff(t *testing.T) {
	// The revision that controls the MRD, and so should control its CRD.
	incoming := metav1.OwnerReference{
		APIVersion: "pkg.crossplane.io/v1",
		Kind:       "ProviderRevision",
		Name:       "cool-provider-b",
		UID:        types.UID("uid-b"),
		Controller: new(true),
	}

	// The revision it replaced, still controlling the live CRD.
	outgoing := metav1.OwnerReference{
		APIVersion: "pkg.crossplane.io/v1",
		Kind:       "ProviderRevision",
		Name:       "cool-provider-a",
		UID:        types.UID("uid-a"),
		Controller: new(true),
	}

	demoted := outgoing
	demoted.Controller = new(false)

	// The same outgoing revision, referenced through another served version.
	outgoingV1beta1 := outgoing
	outgoingV1beta1.APIVersion = "pkg.crossplane.io/v1beta1"

	demotedV1beta1 := outgoingV1beta1
	demotedV1beta1.Controller = new(false)

	// Something that isn't a package revision at all.
	xrd := metav1.OwnerReference{
		APIVersion: "apiextensions.crossplane.io/v2",
		Kind:       "CompositeResourceDefinition",
		Name:       "databases.example.com",
		UID:        types.UID("uid-xrd"),
		Controller: new(true),
	}

	// The MRD's own reference, which CRDAsUnstructured always adds.
	mrdOwner := metav1.OwnerReference{
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
		Kind:       v1alpha1.ManagedResourceDefinitionKind,
		Name:       "test-mrd",
		UID:        types.UID("test-uid"),
	}

	mrd := func(refs ...metav1.OwnerReference) *v1alpha1.ManagedResourceDefinition {
		return newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
			mrd.SetOwnerReferences(refs)
			mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
				Group: "example.com",
				Names: extv1.CustomResourceDefinitionNames{Plural: "databases", Kind: "Database"},
				Scope: extv1.ClusterScoped,
				Versions: []v1alpha1.CustomResourceDefinitionVersion{{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &v1alpha1.CustomResourceValidation{
						OpenAPIV3Schema: runtime.RawExtension{
							Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
						},
					},
				}},
			}
		})
	}

	crd := func(refs ...metav1.OwnerReference) *extv1.CustomResourceDefinition {
		return &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{OwnerReferences: refs}}
	}

	type args struct {
		mrd *v1alpha1.ManagedResourceDefinition

		// The live CRD, or nil if it doesn't exist yet.
		crd *extv1.CustomResourceDefinition
	}

	cases := map[string]struct {
		reason string
		args   args
		want   []metav1.OwnerReference
	}{
		"DemoteReplacedRevision": {
			reason: "We should declare the outgoing revision's reference demoted alongside our own, so one apply completes the hand-off.",
			args: args{
				mrd: mrd(incoming),
				crd: crd(outgoing),
			},
			want: []metav1.OwnerReference{mrdOwner, incoming, demoted},
		},
		"DemoteAnotherAPIVersionOfSameKind": {
			reason: "We should take the CRD from a revision of the same group and kind whose reference was written through another API version, since the API server may serve the type at more than one.",
			args: args{
				mrd: mrd(incoming),
				crd: crd(outgoingV1beta1),
			},
			want: []metav1.OwnerReference{mrdOwner, incoming, demotedV1beta1},
		},
		"RefuseAnotherKind": {
			reason: "We should not take the CRD from a controller that isn't a package revision of the kind that controls the MRD, so the apply fails loudly instead.",
			args: args{
				mrd: mrd(incoming),
				crd: crd(xrd),
			},
			want: []metav1.OwnerReference{mrdOwner, incoming},
		},
		"AlreadyOurs": {
			reason: "A CRD we already control needs no hand-off, so we should declare only our own references and let SSA prune anything else.",
			args: args{
				mrd: mrd(incoming),
				crd: crd(incoming),
			},
			want: []metav1.OwnerReference{mrdOwner, incoming},
		},
		"AlreadyDemoted": {
			reason: "Once the outgoing reference is a plain owner it's ours to prune, so the second apply should stop declaring it.",
			args: args{
				mrd: mrd(incoming),
				crd: crd(incoming, demoted),
			},
			want: []metav1.OwnerReference{mrdOwner, incoming},
		},
		"CRDDoesNotExist": {
			reason: "There's no incumbent to demote when we're creating the CRD.",
			args: args{
				mrd: mrd(incoming),
			},
			want: []metav1.OwnerReference{mrdOwner, incoming},
		},
		"MRDHasNoController": {
			reason: "We should leave the CRD's controller alone when we're not claiming control of it ourselves.",
			args: args{
				mrd: mrd(),
				crd: crd(outgoing),
			},
			want: []metav1.OwnerReference{mrdOwner},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []metav1.OwnerReference

			c := &test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ManagedResourceDefinition:
						*o = *tc.args.mrd
					case *extv1.CustomResourceDefinition:
						if tc.args.crd == nil {
							return kerrors.NewNotFound(schema.GroupResource{}, "test-mrd")
						}

						*o = *tc.args.crd
					}

					return nil
				},
				MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
					got = obj.GetOwnerReferences()

					return nil
				},
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
			}

			r := &Reconciler{
				client:        c,
				managedFields: &ssa.NopManagedFieldsUpgrader{},
				log:           logging.NewNopLogger(),
				record:        event.NewNopRecorder(),
				conditions:    conditions.ObservedGenerationPropagationManager{},
			}

			if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-mrd"}}); err != nil {
				t.Fatalf("\n%s\nr.Reconcile(...): unexpected error: %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\napplied CustomResourceDefinition owner references: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// Helper functions and types for testing

type mrdModifier func(mrd *v1alpha1.ManagedResourceDefinition)

func newMRD(m ...mrdModifier) *v1alpha1.ManagedResourceDefinition {
	mrd := &v1alpha1.ManagedResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mrd",
			UID:  types.UID("test-uid"),
		},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			State: v1alpha1.ManagedResourceDefinitionActive,
		},
	}
	for _, fn := range m {
		fn(mrd)
	}
	return mrd
}

// withMRD returns a MockGetFn that supplies the input ManagedResourceDefinition.
func withMRD(_ *testing.T, mrd *v1alpha1.ManagedResourceDefinition) func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
	return func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		if o, ok := obj.(*v1alpha1.ManagedResourceDefinition); ok {
			*o = *mrd
		}
		return nil
	}
}

// wantMRD returns a MockStatusUpdateFn that ensures the supplied object is the MRD we want.
func wantMRD(t *testing.T, want *v1alpha1.ManagedResourceDefinition) func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	t.Helper()
	return func(_ context.Context, got client.Object, _ ...client.SubResourceUpdateOption) error {
		t.Helper()
		if diff := cmp.Diff(want, got, cmpopts.EquateApproxTime(3*time.Second)); diff != "" {
			t.Errorf("wantMRD(...): -want, +got: %s", diff)
		}
		return nil
	}
}

// testRecorder allows asserting event creation.
type testRecorder struct {
	events []event.Event
}

func (r *testRecorder) Event(_ runtime.Object, e event.Event) {
	r.events = append(r.events, e)
}

func (r *testRecorder) WithAnnotations(_ ...string) event.Recorder {
	return r
}
