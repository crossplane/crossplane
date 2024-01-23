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
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	name := "coolclaim"

	type args struct {
		mgr        manager.Manager
		of         resource.CompositeClaimKind
		with       resource.CompositeKind
		opts       []ReconcilerOption
		claim      *claim.Unstructured
		claimPatch *claim.Unstructured
	}
	type want struct {
		r           reconcile.Result
		claim       *claim.Unstructured
		claimPatch  *claim.Unstructured
		err         error
		claimAssert func(args args, want want) error
	}

	type claimModifier func(o *claim.Unstructured)
	withClaim := func(mods ...claimModifier) *claim.Unstructured {
		cm := claim.New()
		cm.SetAPIVersion("")
		cm.SetKind("")
		for _, m := range mods {
			m(cm)
		}
		return cm
	}

	now := metav1.Now()

	noOpConfigureComposite := func(ctx context.Context, cm *claim.Unstructured, cp, cpPatch *composite.Unstructured) error {
		return nil
	}
	noOpConfigureClaim := func(ctx context.Context, cm, cmPatch *claim.Unstructured, cp, cpPatch *composite.Unstructured) error {
		return nil
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ClaimNotFound": {
			reason: "We should not return an error if the composite resource was not found.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"GetCompositeError": {
			reason: "We should return any error we encounter while getting the referenced composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet:          test.NewMockGetFn(errBoom),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errGetComposite)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositeAlreadyDeleted": {
			reason: "We should not try to delete if the resource is already gone.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetDeletionTimestamp(&now)
					o.SetFinalizers([]string{finalizer})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetDeletionTimestamp(&now)
					o.SetFinalizers([]string{finalizer})
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileSuccess())
				}),
				r: reconcile.Result{Requeue: false},
			},
		},
		"DeleteUnboundCompositeError": {
			reason: "We should return without requeuing if we try to delete a composite resource that does not reference us",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(metav1.Now())
								o.SetClaimReference(&claim.Reference{Name: "some-other-claim"})
							}
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.New(errDeleteUnbound)))
				}),
				r: reconcile.Result{Requeue: false},
			},
		},
		"DeleteCompositeError": {
			reason: "We should return any error we encounter while deleting the referenced composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(metav1.Now())
								o.SetClaimReference(&claim.Reference{Name: name})
							}
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					now := metav1.Now()
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
					bg := xpv1.CompositeDeleteBackground
					o.SetCompositeDeletePolicy(&bg)
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					now := metav1.Now()
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errDeleteComposite)))
					bg := xpv1.CompositeDeleteBackground
					o.SetCompositeDeletePolicy(&bg)
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"RemoveFinalizerError": {
			reason: "We should return any error we encounter while removing the claim's finalizer",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockDelete: test.NewMockDeleteFn(nil),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return errBoom },
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errRemoveFinalizer)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"SuccessfulDelete": {
			reason: "We should not requeue if we successfully delete the bound composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockDelete: test.NewMockDeleteFn(nil),
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(metav1.Now())
								o.SetClaimReference(&claim.Reference{Name: name})
							}
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					bg := xpv1.CompositeDeleteBackground
					o.SetCompositeDeletePolicy(&bg)
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
					bg := xpv1.CompositeDeleteBackground
					o.SetCompositeDeletePolicy(&bg)
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileSuccess())
				}),
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulForegroundDelete": {
			reason: "We should requeue if we successfully delete the bound composite resource using Foreground deletion",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockDelete: test.NewMockDeleteFn(nil),
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(metav1.Now())
								o.SetClaimReference(&claim.Reference{Name: name})
							}
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					fg := xpv1.CompositeDeleteForeground
					o.SetCompositeDeletePolicy(&fg)
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
					fg := xpv1.CompositeDeleteForeground
					o.SetCompositeDeletePolicy(&fg)
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ForegroundDeleteWaitForCompositeDeletion": {
			reason: "We should requeue if we successfully deleted the bound composite resource using Foreground deletion and it has not yet been deleted",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockDelete: test.NewMockDeleteFn(nil),
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(now)
								o.SetDeletionTimestamp(&now)
								o.SetClaimReference(&claim.Reference{Name: name})
							}
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					fg := xpv1.CompositeDeleteForeground
					o.SetCompositeDeletePolicy(&fg)
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetName(name)
					o.SetDeletionTimestamp(&now)
					o.SetResourceReference(&corev1.ObjectReference{})
					fg := xpv1.CompositeDeleteForeground
					o.SetCompositeDeletePolicy(&fg)
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ConfigureError": {
			reason: "We should return any error we encounter configuring the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					withCompositeConfigurator(func(ctx context.Context, cm *claim.Unstructured, cp, cpPatch *composite.Unstructured) error {
						return errBoom
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errConfigureComposite)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"PatchError": {
			reason: "We should return any error we encounter patching the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
							obj.SetCreationTimestamp(metav1.NewTime(time.Now()))
							return nil
						},
						MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
							if patch != client.Apply {
								return fmt.Errorf("Should be ssa patch")
							}
							if _, ok := obj.(*composite.Unstructured); ok {
								return errBoom
							}
							return nil
						},
					}),
					withCompositeConfigurator(noOpConfigureComposite),
				},
				claim: withClaim(),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetFinalizers([]string{finalizer})
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errPatchComposite)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ClaimConfigureError": {
			reason: "We should return any error we encounter configuring the claim resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockCreate:       test.NewMockCreateFn(nil),
					}),
					withCompositeConfigurator(noOpConfigureComposite),
					withClaimConfigurator(func(ctx context.Context, cm, cmPatch *claim.Unstructured, cp, cpPatch *composite.Unstructured) error {
						return errBoom
					}),
				},
				claim: withClaim(),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errConfigureClaim)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositeNotReady": {
			reason: "We should return early if the bound composite resource is not yet ready",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(metav1.NewTime(time.Now()))
							}
							return nil
						}),
						MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetConditions(xpv1.Unavailable())
							}
							return nil
						}),
					}),
					withCompositeConfigurator(noOpConfigureComposite),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetFinalizers([]string{finalizer})
					o.SetConditions(xpv1.ReconcileSuccess(), Waiting())
				}),
				r: reconcile.Result{Requeue: false},
			},
		},
		"PropagateConnectionError": {
			reason: "We should return any error we encounter while propagating the bound composite's connection details",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetConditions(xpv1.Available())
								o.SetCreationTimestamp(metav1.NewTime(time.Now()))
							}
							return nil
						}),
						MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetConditions(xpv1.Available())
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					}),
					withCompositeConfigurator(noOpConfigureComposite),
					WithConnectionPropagator(ConnectionPropagatorFn(func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
						return false, errBoom
					})),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetFinalizers([]string{finalizer})
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errPropagateCDs)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"SuccessfulPropagate": {
			reason: "We should not requeue if we successfully applied the composite resource and propagated its connection details",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetConditions(xpv1.Available())
								o.SetCreationTimestamp(metav1.NewTime(time.Now()))
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetConditions(xpv1.Available())
								o.SetCreationTimestamp(metav1.NewTime(time.Now()))
							}
							return nil

						}),
					}),
					withCompositeConfigurator(noOpConfigureComposite),
					WithConnectionPropagator(ConnectionPropagatorFn(func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetFinalizers([]string{finalizer})
					o.SetConnectionDetailsLastPublishedTime(&now)
					o.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
				}),
				r: reconcile.Result{Requeue: false},
			},
		},
		"ReconciliationPausedSuccessful": {
			reason: `If a composite resource claim has the pause annotation with value "true", there should be no further requeue requests.`,
			args: args{
				mgr: &fake.Manager{},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
					o.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
				}),
			},
		},
		"ReconciliationPausedError": {
			reason: `If a composite resource claim has the pause annotation with value "true" and the status update due to reconciliation being paused fails, error should be reported causing an exponentially backed-off requeue.`,
			args: args{
				mgr: &fake.Manager{},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
				}),
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaimStatus),
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
					o.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
				}),
			},
		},
		"ReconciliationResumes": {
			reason: `If a composite resource claim has the pause annotation with some value other than "true" and the Synced=False/ReconcilePaused status condition, claim should acquire Synced=True/ReconcileSuccess.`,
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(metav1.NewTime(time.Now()))
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetConditions(xpv1.Available())
							}
							return nil
						}),
					}),
					withCompositeConfigurator(noOpConfigureComposite),
					WithConnectionPropagator(ConnectionPropagatorFn(func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: ""})
					o.SetConditions(xpv1.ReconcilePaused())
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetFinalizers([]string{finalizer})
					o.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
					o.SetConnectionDetailsLastPublishedTime(&now)
				}),
			},
		},
		"ReconciliationResumesAfterAnnotationRemoval": {
			reason: `If a composite resource claim has the pause annotation removed and the Synced=False/ReconcilePaused status condition, claim should acquire Synced=True/ReconcileSuccess.`,
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetCreationTimestamp(metav1.NewTime(time.Now()))
							}
							return nil
						}),
						MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
							if o, ok := obj.(*composite.Unstructured); ok {
								o.SetConditions(xpv1.Available())
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					}),
					withCompositeConfigurator(noOpConfigureComposite),
					WithConnectionPropagator(ConnectionPropagatorFn(func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					// no annotation atm
					// (but reconciliations were already paused)
					o.SetConditions(xpv1.ReconcilePaused())
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetFinalizers([]string{finalizer})
					o.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
					o.SetConnectionDetailsLastPublishedTime(&now)
				}),
			},
		},
		"CreateComposite": {
			reason: "create composite with a generated name and refer it in the claim",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Group: "foo.com", Resource: "composite"}, "foo")),
					},
				},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet:          test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
							want := &composite.Unstructured{
								Unstructured: unstructured.Unstructured{
									Object: map[string]any{
										"spec": map[string]any{
											"foo": "bar",
											"claimRef": map[string]any{
												"name":       "c",
												"namespace":  "ns",
												"apiVersion": "foo.com/v1",
												"kind":       "Claim",
											},
										},
									},
								},
							}
							want.SetName(obj.GetName())
							want.SetGroupVersionKind(schema.GroupVersionKind{Group: "foo.com", Version: "v1", Kind: "Composite"})
							want.SetLabels(map[string]string{
								xcrd.LabelKeyClaimName:      "c",
								xcrd.LabelKeyClaimNamespace: "ns",
							})
							cp, ok := obj.(*composite.Unstructured)
							if !ok {
								return nil
							}
							if diff := cmp.Diff(want, cp); diff != "" {
								return errors.New(diff)
							}
							return nil
						}),
					}),
					WithConnectionPropagator(ConnectionPropagatorFn(func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
				with: resource.CompositeKind{Group: "foo.com", Version: "v1", Kind: "Composite"},
				of:   resource.CompositeClaimKind{Group: "foo.com", Version: "v1", Kind: "Claim"},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetGroupVersionKind(schema.GroupVersionKind{Group: "foo.com", Version: "v1", Kind: "Claim"})
					o.Object["spec"] = map[string]interface{}{"foo": "bar"}
					o.SetName("c")
					o.SetNamespace("ns")
				}),
			},
			want: want{
				claimAssert: func(args args, want want) error {
					ref := args.claimPatch.GetResourceReference()
					if ref.Namespace != "" || !strings.HasPrefix(ref.Name, "c-") ||
						ref.Kind != "Composite" || ref.APIVersion != "foo.com/v1" {
						return fmt.Errorf("Claim has no valid composite ref %v", ref)
					}
					return nil
				},
			},
		},
		"CompositeCreatedButNotInCacheByNextReconcile": {
			reason: "previous loop created composite, bound it to claim, but in the next loop still not present in the cache. We should try to create composite again under the same name, but we are going to fails and requeu",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet:          test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Group: "foo.com", Resource: "composite"}, "composite")),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
							cp, ok := obj.(*composite.Unstructured)
							if !ok {
								return nil
							}
							if cp.GetName() != "composite" {
								return errors.Errorf("Unexpected composite name: %s", obj.GetName())
							}
							return nil
						},
					}),
					withClaimConfigurator(noOpConfigureClaim),
					WithConnectionPropagator(ConnectionPropagatorFn(func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
						return true, nil
					})),
				},
				with: resource.CompositeKind{Group: "foo.com", Version: "v1", Kind: "Composite"},
				of:   resource.CompositeClaimKind{Group: "foo.com", Version: "v1", Kind: "Claim"},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetGroupVersionKind(schema.GroupVersionKind{Group: "foo.com", Version: "v1", Kind: "Claim"})
					o.Object["spec"] = map[string]interface{}{"foo": "bar"}
					o.SetName("c")
					o.SetNamespace("ns")
					o.SetResourceReference(&corev1.ObjectReference{
						Name:       "composite",
						APIVersion: "foo.com/v1",
						Kind:       "Composite",
					})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetGroupVersionKind(schema.GroupVersionKind{Group: "foo.com", Version: "v1", Kind: "Claim"})
					o.SetName("c")
					o.SetNamespace("ns")
					o.SetFinalizers([]string{finalizer})
					o.Object["status"] = nil
				}),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create wrapper around the Get and Status().Update funcs of the
			// client mock.
			if tc.args.claim != nil {
				tc.args.opts = append(tc.args.opts, func(r *Reconciler) {
					var customGet test.MockGetFn
					var customStatusUpdate test.MockSubResourceUpdateFn
					var customUpdate test.MockUpdateFn
					var customPatch test.MockPatchFn
					mockGet := func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if o, ok := obj.(*claim.Unstructured); ok {
							tc.args.claim.DeepCopyInto(o)
							return nil
						}
						if customGet != nil {
							return customGet(ctx, key, obj)
						}
						return nil
					}

					mockStatusUpdate := func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						if o, ok := obj.(*claim.Unstructured); ok {
							o.DeepCopyInto(tc.args.claim)
						}
						if customStatusUpdate != nil {
							return customStatusUpdate(ctx, obj, opts...)
						}
						return nil
					}

					mockUpdate := func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						if o, ok := obj.(*claim.Unstructured); ok {
							o.DeepCopyInto(tc.args.claim)
						}
						if customStatusUpdate != nil {
							return customUpdate(ctx, obj, opts...)
						}
						return nil
					}

					mockPatch := func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if o, ok := obj.(*claim.Unstructured); ok {
							tc.args.claimPatch = o
						}
						if customPatch != nil {
							return customPatch(ctx, obj, patch, opts...)
						}
						return nil
					}

					if mockClient, ok := r.client.(*test.MockClient); ok {
						customGet = mockClient.MockGet
						customStatusUpdate = mockClient.MockStatusUpdate
						customUpdate = mockClient.MockUpdate
						customPatch = mockClient.MockPatch
						mockClient.MockGet = mockGet
						mockClient.MockStatusUpdate = mockStatusUpdate
						mockClient.MockUpdate = mockUpdate
						mockClient.MockPatch = mockPatch
					} else {
						r.client = &test.MockClient{
							MockGet:          mockGet,
							MockStatusUpdate: mockStatusUpdate,
							MockUpdate:       mockUpdate,
							MockPatch:        mockPatch,
						}
						// r.composite = defaultCRComposite(&test.MockClient{
						//	MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Group: "g", Resource: "r"}, "foo")),
						// })
					}
				})
			}

			r := NewReconciler(tc.args.mgr, tc.args.of, tc.args.with, append(tc.args.opts, WithLogger(testLog))...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if tc.want.claim != nil {
				if diff := cmp.Diff(tc.want.claim, tc.args.claim, cmpopts.AcyclicTransformer("StringToTime", func(s string) any {
					ts, err := time.Parse(time.RFC3339, s)
					if err != nil {
						return s
					}
					return ts
				}), cmpopts.EquateApproxTime(3*time.Second)); diff != "" {
					t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if tc.want.claimPatch != nil {
				if diff := cmp.Diff(tc.want.claimPatch, tc.args.claimPatch, cmpopts.AcyclicTransformer("StringToTime", func(s string) any {
					ts, err := time.Parse(time.RFC3339, s)
					if err != nil {
						return s
					}
					return ts
				}), cmpopts.EquateApproxTime(3*time.Second)); diff != "" {
					t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if tc.want.claimAssert != nil {
				if err := tc.want.claimAssert(tc.args, tc.want); err != nil {
					t.Error(err)
				}
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
