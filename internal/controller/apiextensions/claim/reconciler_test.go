/*
Copyright 2020 The Crossplane Authors.

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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/reference"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xresource"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()

	type args struct {
		client client.Client
		of     resource.CompositeClaimKind
		with   resource.CompositeKind
		opts   []ReconcilerOption
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
		"ClaimNotFound": {
			reason: "We should not return an error if the composite resource was not found.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetClaimError": {
			reason: "We should return any error we encounter getting the claim.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errGetClaim),
			},
		},
		"ReconciliationPaused": {
			reason: `If a claim has the pause annotation with value "true" we should stop reconciling and not requeue.`,
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.(*claim.Unstructured).SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
						cm.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ReconciliationUnpaused": {
			reason: "If a claim has the ReconcilePaused status condition but no paused annotation, the condition should change to ReconcileSuccess.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						// This claim was paused.
						obj.(*claim.Unstructured).SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that our synced status condition changed
						// from Paused to ReconcileSuccess.
						cm.SetConditions(xpv1.ReconcileSuccess())
						cm.SetConditions(Waiting())
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
					WithCompositeSyncer(CompositeSyncerFn(func(_ context.Context, _ *claim.Unstructured, _ *composite.Unstructured) error { return nil })),
					WithConnectionPropagator(ConnectionPropagatorFn(func(_ context.Context, _ xresource.LocalConnectionSecretOwner, _ resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetCompositeError": {
			reason: "The reconcile should fail if we can't get the XR, unless it wasn't found.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						case *composite.Unstructured:
							// Return an error getting the XR.
							return errBoom
						}
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						cm.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errGetComposite)))
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositeAlreadyBoundError": {
			reason: "The reconcile should fail if the referenced XR is bound to another claim",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						case *composite.Unstructured:
							// This XR was created, and references another
							//  claim.
							o.SetCreationTimestamp(now)
							o.SetClaimReference(&reference.Claim{Name: "some-other-claim"})
						}
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						cm.SetConditions(xpv1.ReconcileError(errors.Errorf(errFmtUnbound, "", "some-other-claim")))
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"DeleteCompositeError": {
			reason: "We should not try to delete if the resource is already gone.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							o.SetDeletionTimestamp(&now)
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						case *composite.Unstructured:
							// Pretend the XR exists.
							o.SetCreationTimestamp(now)
						}
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(errBoom),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetDeletionTimestamp(&now)
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						cm.SetConditions(xpv1.Deleting())
						cm.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errDeleteComposite)))
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"UnpublishConnectionDetailsError": {
			reason: "The reconcile should fail if we can't unpublish the claim's connection details.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.(*claim.Unstructured).SetDeletionTimestamp(&now)
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(nil),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetDeletionTimestamp(&now)
						cm.SetConditions(xpv1.Deleting())
						cm.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errDeleteCDs)))
					})),
				},
				opts: []ReconcilerOption{
					WithConnectionUnpublisher(ConnectionUnpublisherFn(func(_ context.Context, _ xresource.LocalConnectionSecretOwner, _ managed.ConnectionDetails) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"RemoveFinalizerError": {
			reason: "The reconcile should fail if we can't remove the claim's finalizer.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.(*claim.Unstructured).SetDeletionTimestamp(&now)
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(nil),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetDeletionTimestamp(&now)
						cm.SetConditions(xpv1.Deleting())
						cm.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errRemoveFinalizer)))
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"SuccessfulDelete": {
			reason: "We should not requeue if we successfully delete the resource.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.(*claim.Unstructured).SetDeletionTimestamp(&now)
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(nil),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetDeletionTimestamp(&now)
						cm.SetConditions(xpv1.Deleting())
						cm.SetConditions(xpv1.ReconcileSuccess())
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulForegroundDelete": {
			reason: "We should requeue if we successfully delete the bound composite resource using Foreground deletion",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							o.SetDeletionTimestamp(&now)
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
							// We want to foreground delete.
							fg := xpv1.CompositeDeleteForeground
							o.SetCompositeDeletePolicy(&fg)
						case *composite.Unstructured:
							// Pretend the XR exists and is bound.
							o.SetCreationTimestamp(now)
							o.SetClaimReference(&reference.Claim{})
						}
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(nil),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"ForegroundDeleteWaitForCompositeDeletion": {
			reason: "We should requeue if we successfully deleted the bound composite resource using Foreground deletion and it has not yet been deleted",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							o.SetDeletionTimestamp(&now)
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
							// We want to foreground delete.
							fg := xpv1.CompositeDeleteForeground
							o.SetCompositeDeletePolicy(&fg)
						case *composite.Unstructured:
							// Pretend the XR exists and is bound, but is
							// being deleted.
							o.SetCreationTimestamp(now)
							o.SetDeletionTimestamp(&now)
							o.SetClaimReference(&reference.Claim{})
						}
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						// We want to foreground delete.
						fg := xpv1.CompositeDeleteForeground
						cm.SetCompositeDeletePolicy(&fg)

						// Check that we set our status condition.
						cm.SetDeletionTimestamp(&now)
						cm.SetConditions(xpv1.Deleting())
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"AddFinalizerError": {
			reason: "We should fail the reconcile if we can't add the claim's finalizer",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"SyncCompositeError": {
			reason: "We should fail the reconcile if we can't bind and sync the claim with a composite resource",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errSync)))
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
					WithCompositeSyncer(CompositeSyncerFn(func(_ context.Context, _ *claim.Unstructured, _ *composite.Unstructured) error { return errBoom })),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositeNotReady": {
			reason: "We should return early if the bound composite resource is not yet ready",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						case *composite.Unstructured:
							// Pretend the XR exists and is bound, but is
							// still being created.
							o.SetCreationTimestamp(now)
							o.SetClaimReference(&reference.Claim{})
							o.SetConditions(xpv1.Creating())
						}
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						cm.SetConditions(xpv1.ReconcileSuccess())
						cm.SetConditions(Waiting())
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
					WithCompositeSyncer(CompositeSyncerFn(func(_ context.Context, _ *claim.Unstructured, _ *composite.Unstructured) error { return nil })),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"PropagateConnectionError": {
			reason: "We should fail the reconcile if we can't propagate the bound XR's connection details",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						case *composite.Unstructured:
							// Pretend the XR exists and is available.
							o.SetCreationTimestamp(now)
							o.SetClaimReference(&reference.Claim{})
							o.SetConditions(xpv1.Available())
						}
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						cm.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errPropagateCDs)))
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
					WithCompositeSyncer(CompositeSyncerFn(func(_ context.Context, _ *claim.Unstructured, _ *composite.Unstructured) error { return nil })),
					WithConnectionPropagator(ConnectionPropagatorFn(func(_ context.Context, _ xresource.LocalConnectionSecretOwner, _ resource.ConnectionSecretOwner) (propagated bool, err error) {
						return false, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"SuccessfulReconcile": {
			reason: "We should not requeue if we successfully synced the composite resource and propagated its connection details",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						case *composite.Unstructured:
							// Pretend the XR exists and is available.
							o.SetCreationTimestamp(now)
							o.SetClaimReference(&reference.Claim{})
							o.SetConditions(xpv1.Available())
						}
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						cm.SetConnectionDetailsLastPublishedTime(&now)
						cm.SetConditions(xpv1.ReconcileSuccess())
						cm.SetConditions(xpv1.Available())
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
					WithCompositeSyncer(CompositeSyncerFn(func(_ context.Context, _ *claim.Unstructured, _ *composite.Unstructured) error { return nil })),
					WithConnectionPropagator(ConnectionPropagatorFn(func(_ context.Context, _ xresource.LocalConnectionSecretOwner, _ resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ClaimConditions": {
			reason: "We should copy custom conditions from the XR if seen in the claimConditions array.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						switch o := obj.(type) {
						case *claim.Unstructured:
							// We won't try to get an XR unless the claim
							// references one.
							o.SetResourceReference(&reference.Composite{Name: "cool-composite"})
							// The system conditions are already set.
							o.SetConditions(xpv1.ReconcileSuccess())
							o.SetConditions(xpv1.Available())
							// Database was marked as creating in a prior reconciliation.
							o.SetConditions(xpv1.Condition{
								Type:   "DatabaseReady",
								Status: corev1.ConditionFalse,
								Reason: "Creating",
							})
						case *composite.Unstructured:
							// Pretend the XR exists and is available.
							o.SetCreationTimestamp(now)
							o.SetClaimReference(&reference.Claim{})
							o.SetConditions(xpv1.Available())
							o.SetConditions(
								// Database has become ready.
								xpv1.Condition{
									Type:   "DatabaseReady",
									Status: corev1.ConditionTrue,
									Reason: "Available",
								},
								// Bucket is a new condition.
								xpv1.Condition{
									Type:   "BucketReady",
									Status: corev1.ConditionFalse,
									Reason: "Creating",
								},
								// Internal condition should not be copied over as it is not in
								// claimConditions.
								xpv1.Condition{
									Type:   "InternalSync",
									Status: corev1.ConditionFalse,
									Reason: "Syncing",
								},
							)
							// Database and Bucket are claim conditions so they should be
							// copied over.
							o.SetClaimConditionTypes("DatabaseReady", "BucketReady")
						}
						return nil
					}),
					MockStatusUpdate: WantClaim(t, NewClaim(func(cm *claim.Unstructured) {
						// Check that we set our status condition.
						cm.SetResourceReference(&reference.Composite{Name: "cool-composite"})
						cm.SetConnectionDetailsLastPublishedTime(&now)
						cm.SetConditions(xpv1.ReconcileSuccess())
						cm.SetConditions(xpv1.Available())
						cm.SetConditions(
							// Database condition should have been updated to show ready.
							xpv1.Condition{
								Type:   "DatabaseReady",
								Status: corev1.ConditionTrue,
								Reason: "Available",
							},
							// Bucket condition should have been created.
							xpv1.Condition{
								Type:   "BucketReady",
								Status: corev1.ConditionFalse,
								Reason: "Creating",
							},
						)
					})),
				},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil },
					}),
					WithCompositeSyncer(CompositeSyncerFn(func(_ context.Context, _ *claim.Unstructured, _ *composite.Unstructured) error { return nil })),
					WithConnectionPropagator(ConnectionPropagatorFn(func(_ context.Context, _ xresource.LocalConnectionSecretOwner, _ resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.client, tc.args.of, tc.args.with, tc.args.opts...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{})
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type ClaimModifier func(cm *claim.Unstructured)

func NewClaim(m ...ClaimModifier) *claim.Unstructured {
	cm := claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{}))
	for _, fn := range m {
		fn(cm)
	}
	return cm
}

// A status update function that ensures the supplied object is the claim we want.
func WantClaim(t *testing.T, want *claim.Unstructured) func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	t.Helper()
	return func(_ context.Context, got client.Object, _ ...client.SubResourceUpdateOption) error {
		t.Helper()
		// Normally we use a custom Equal method on conditions to ignore the
		// lastTransitionTime, but we're using unstructured types here where
		// the conditions are just a map[string]any.
		diff := cmp.Diff(want, got, cmpopts.AcyclicTransformer("StringToTime", func(s string) any {
			ts, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return s
			}
			return ts
		}), cmpopts.EquateApproxTime(3*time.Second))
		if diff != "" {
			t.Errorf("WantClaim(...): -want, +got: %s", diff)
		}
		return nil
	}
}
