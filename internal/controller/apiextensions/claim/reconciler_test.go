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
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	name := "coolclaim"

	type args struct {
		mgr   manager.Manager
		of    resource.CompositeClaimKind
		with  resource.CompositeKind
		opts  []ReconcilerOption
		claim *claim.Unstructured
	}
	type want struct {
		r     reconcile.Result
		claim *claim.Unstructured
		err   error
	}

	type claimModifier func(o *claim.Unstructured)
	withClaim := func(mods ...claimModifier) *claim.Unstructured {
		cm := claim.New()
		for _, m := range mods {
			m(cm)
		}
		return cm
	}

	now := metav1.Now()

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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(errBoom),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetCreationTimestamp(metav1.Now())
									o.SetClaimReference(&claim.Reference{Name: "some-other-claim"})
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(errBoom),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetCreationTimestamp(metav1.Now())
									o.SetClaimReference(&claim.Reference{Name: name})
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(errBoom),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil),
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetCreationTimestamp(metav1.Now())
									o.SetClaimReference(&claim.Reference{Name: name})
								}
								return nil
							}),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil),
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetCreationTimestamp(metav1.Now())
									o.SetClaimReference(&claim.Reference{Name: name})
								}
								return nil
							}),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil),
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetCreationTimestamp(now)
									o.SetDeletionTimestamp(&now)
									o.SetClaimReference(&claim.Reference{Name: name})
								}
								return nil
							}),
						},
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
		"AddFinalizerError": {
			reason: "We should return any error we encounter while adding the claim's finalizer",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return errBoom },
					}),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},

		"ConfigureError": {
			reason: "We should return any error we encounter configuring the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return errBoom })),
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
		"BindError": {
			reason: "We should return any error we encounter binding the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return errBoom })),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errBindComposite)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ApplyError": {
			reason: "We should return any error we encounter applying the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errApplyComposite)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ClaimConfigureError": {
			reason: "We should return any error we encounter configuring the claim resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithClaimConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return errBoom })),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithClaimConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
				},
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
				}),
			},
			want: want{
				claim: withClaim(func(o *claim.Unstructured) {
					o.SetResourceReference(&corev1.ObjectReference{})
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetConditions(xpv1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithClaimConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetConditions(xpv1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithClaimConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
						},
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetConditions(xpv1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithClaimConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
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
					o.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: ""})
					o.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConnectionDetailsLastPublishedTime(&now)
				}),
			},
		},
		"ReconciliationResumesAfterAnnotationRemoval": {
			reason: `If a composite resource claim has the pause annotation removed and the Synced=False/ReconcilePaused status condition, claim should acquire Synced=True/ReconcileSuccess.`,
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									o.SetConditions(xpv1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithClaimConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
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
					o.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
					o.SetResourceReference(&corev1.ObjectReference{})
					o.SetConnectionDetailsLastPublishedTime(&now)
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
					mockGet := func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if o, ok := obj.(*claim.Unstructured); ok {
							tc.args.claim.DeepCopyInto(&o.Unstructured)
							return nil
						}
						if customGet != nil {
							return customGet(ctx, key, obj)
						}
						return nil
					}

					mockStatusUpdate := func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						if o, ok := obj.(*claim.Unstructured); ok {
							o.DeepCopyInto(&tc.args.claim.Unstructured)
						}
						if customStatusUpdate != nil {
							return customStatusUpdate(ctx, obj, opts...)
						}
						return nil
					}

					if mockClient, ok := r.client.Client.(*test.MockClient); ok {
						customGet = mockClient.MockGet
						customStatusUpdate = mockClient.MockStatusUpdate
						mockClient.MockGet = mockGet
						mockClient.MockStatusUpdate = mockStatusUpdate
					} else {
						r.client.Client = &test.MockClient{
							MockGet:          mockGet,
							MockStatusUpdate: mockStatusUpdate,
						}
					}
				})
			}

			r := NewReconciler(tc.args.mgr, tc.args.of, tc.args.with, append(tc.args.opts, WithLogger(testLog))...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.claim, tc.args.claim, cmpopts.AcyclicTransformer("StringToTime", func(s string) any {
				ts, err := time.Parse(time.RFC3339, s)
				if err != nil {
					return s
				}
				return ts
			}), cmpopts.EquateApproxTime(3*time.Second)); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
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
