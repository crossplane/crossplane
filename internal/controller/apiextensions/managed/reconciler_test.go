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
	"k8s.io/utils/ptr"
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
		"StaleCRDControllerRefDemoteError": {
			reason: "We should return an error and set BlockedManaged when demoting stale controller ownerReferences fails",
			args: args{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							return withMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
								mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
							}))(ctx, key, o)
						case *extv1.CustomResourceDefinition:
							o.CreationTimestamp = metav1.Now()
							o.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: "pkg.crossplane.io/v1",
								Kind:       "ProviderRevision",
								Controller: ptr.To(true),
							}}
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
					MockUpdate: test.NewMockUpdateFn(errBoom),
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
						mrd.SetConditions(v1alpha1.BlockedManaged().WithMessage("unable to fix CRD controller references, see events"))
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
		"StaleCRDControllerRefDemoted": {
			reason: "We should demote stale controller ownerReferences, re-read the CRD, and continue to reconcile successfully",
			args: args{
				c: &test.MockClient{
					MockGet: func() func(context.Context, client.ObjectKey, client.Object) error {
						crdGetCall := 0
						return func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
										Scope:    extv1.ClusterScoped,
										Versions: []v1alpha1.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true}},
									}
								}))(ctx, key, o)
							case *extv1.CustomResourceDefinition:
								crdGetCall++
								if crdGetCall == 1 {
									// First Get: CRD exists with a stale ProviderRevision controller ownerRef.
									o.CreationTimestamp = metav1.Now()
									o.OwnerReferences = []metav1.OwnerReference{{
										APIVersion: "pkg.crossplane.io/v1",
										Kind:       "ProviderRevision",
										Controller: ptr.To(true),
									}}
								}
								// Second Get (after Update): clean CRD, no stale refs.
								return nil
							default:
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							}
						}
					}(),
					MockUpdate: test.NewMockUpdateFn(nil),
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						u, ok := obj.(*unstructured.Unstructured)
						if !ok {
							return nil
						}
						u.Object["status"] = map[string]any{
							"conditions": []any{
								map[string]any{
									"type":   string(extv1.Established),
									"status": string(extv1.ConditionTrue),
								},
							},
						}
						return nil
					},
					MockStatusUpdate: wantMRD(t, newMRD(func(mrd *v1alpha1.ManagedResourceDefinition) {
						mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
						mrd.Spec.CustomResourceDefinitionSpec = v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope:    extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true}},
						}
						mrd.SetConditions(v1alpha1.EstablishedManaged())
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

func TestDemoteStaleControllerRefs(t *testing.T) {
	cases := map[string]struct {
		reason  string
		crd     *extv1.CustomResourceDefinition
		want    []metav1.OwnerReference
		changed bool
	}{
		"NoOwnerRefs": {
			reason:  "Should return false when there are no ownerReferences",
			crd:     &extv1.CustomResourceDefinition{},
			want:    nil,
			changed: false,
		},
		"MRDControllerOnly": {
			reason: "Should return false when the only controller is the ManagedResourceDefinition",
			crd: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "apiextensions.crossplane.io/v1alpha1", Kind: "ManagedResourceDefinition", Controller: ptr.To(true)},
					},
				},
			},
			want:    []metav1.OwnerReference{{APIVersion: "apiextensions.crossplane.io/v1alpha1", Kind: "ManagedResourceDefinition", Controller: ptr.To(true)}},
			changed: false,
		},
		"SameKindDifferentGroup": {
			reason: "Should demote an ownerRef with Kind=ManagedResourceDefinition but a different group",
			crd: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "other.io/v1", Kind: "ManagedResourceDefinition", Controller: ptr.To(true)},
					},
				},
			},
			want:    []metav1.OwnerReference{{APIVersion: "other.io/v1", Kind: "ManagedResourceDefinition", Controller: ptr.To(false)}},
			changed: true,
		},
		"ProviderRevisionController": {
			reason: "Should demote a ProviderRevision with controller:true",
			crd: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Controller: ptr.To(true)},
						{APIVersion: "apiextensions.crossplane.io/v1alpha1", Kind: "ManagedResourceDefinition", Controller: ptr.To(false)},
					},
				},
			},
			want: []metav1.OwnerReference{
				{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Controller: ptr.To(false)},
				{APIVersion: "apiextensions.crossplane.io/v1alpha1", Kind: "ManagedResourceDefinition", Controller: ptr.To(false)},
			},
			changed: true,
		},
		"MultipleStaleControllers": {
			reason: "Should demote all non-MRD controller refs",
			crd: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Name: "old", Controller: ptr.To(true)},
						{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Name: "older", Controller: ptr.To(true)},
						{APIVersion: "apiextensions.crossplane.io/v1alpha1", Kind: "ManagedResourceDefinition", Controller: ptr.To(true)},
					},
				},
			},
			want: []metav1.OwnerReference{
				{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Name: "old", Controller: ptr.To(false)},
				{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Name: "older", Controller: ptr.To(false)},
				{APIVersion: "apiextensions.crossplane.io/v1alpha1", Kind: "ManagedResourceDefinition", Controller: ptr.To(true)},
			},
			changed: true,
		},
		"NilControllerPointer": {
			reason: "Should treat Controller==nil as non-controller and leave the ref unchanged",
			crd: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Controller: nil},
					},
				},
			},
			want:    []metav1.OwnerReference{{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Controller: nil}},
			changed: false,
		},
		"NonControllerProviderRevision": {
			reason: "Should not modify a ProviderRevision that is already controller:false",
			crd: &extv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Controller: ptr.To(false)},
					},
				},
			},
			want:    []metav1.OwnerReference{{APIVersion: "pkg.crossplane.io/v1", Kind: "ProviderRevision", Controller: ptr.To(false)}},
			changed: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := demoteStaleControllerRefs(tc.crd)
			if diff := cmp.Diff(tc.changed, got); diff != "" {
				t.Errorf("\n%s\ndemoteStaleControllerRefs(...): -want changed, +got changed:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, tc.crd.OwnerReferences); diff != "" {
				t.Errorf("\n%s\ndemoteStaleControllerRefs(...): -want refs, +got refs:\n%s", tc.reason, diff)
			}
		})
	}
}
