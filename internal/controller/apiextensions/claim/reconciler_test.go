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

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	name := "coolclaim"

	type args struct {
		mgr  manager.Manager
		of   resource.CompositeClaimKind
		with resource.CompositeKind
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
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								switch o := obj.(type) {
								case *claim.Unstructured:
									o.SetResourceReference(&corev1.ObjectReference{})
									return nil
								case *composite.Unstructured:
									return errBoom
								}
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetComposite),
			},
		},
		"CompositeAlreadyDeleted": {
			reason: "We should not try to delete if the resource is already gone.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								switch o := obj.(type) {
								case *claim.Unstructured:
									o.SetResourceReference(&corev1.ObjectReference{})
									now := metav1.Now()
									o.SetDeletionTimestamp(&now)
									o.SetFinalizers([]string{finalizer})
									return nil
								case *composite.Unstructured:
									return kerrors.NewNotFound(schema.GroupResource{}, "")
								}
								return nil
							}),
						},
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
				},
			},
			want: want{
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
								switch o := obj.(type) {
								case *claim.Unstructured:
									now := metav1.Now()
									o.SetName(name)
									o.SetDeletionTimestamp(&now)
									o.SetResourceReference(&corev1.ObjectReference{})
								case *composite.Unstructured:
									o.SetCreationTimestamp(metav1.Now())
									o.SetClaimReference(&corev1.ObjectReference{Name: "some-other-claim"})
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(errBoom),
						},
					}),
				},
			},
			want: want{
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
								switch o := obj.(type) {
								case *claim.Unstructured:
									now := metav1.Now()
									o.SetName(name)
									o.SetDeletionTimestamp(&now)
									o.SetResourceReference(&corev1.ObjectReference{})
								case *composite.Unstructured:
									o.SetCreationTimestamp(metav1.Now())
									o.SetClaimReference(&corev1.ObjectReference{Name: name})
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteComposite),
			},
		},
		"RemoveFinalizerError": {
			reason: "We should return any error we encounter while removing the claim's finalizer",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									now := metav1.Now()
									o.SetDeletionTimestamp(&now)
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return errBoom },
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRemoveFinalizer),
			},
		},
		"SuccessfulDelete": {
			reason: "We should not requeue if we successfully delete the bound composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									now := metav1.Now()
									o.SetDeletionTimestamp(&now)
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"AddFinalizerError": {
			reason: "We should return any error we encounter while adding the claim's finalizer",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
						},
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return errBoom },
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddFinalizer),
			},
		},
		"ConfigureError": {
			reason: "We should return any error we encounter configuring the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
						},
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return errBoom })),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errConfigureComposite),
			},
		},
		"BindError": {
			reason: "We should return any error we encounter binding the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(ConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return errBoom })),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errBindComposite),
			},
		},
		"ApplyError": {
			reason: "We should return any error we encounter applying the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
						},
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
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyComposite),
			},
		},
		"ClaimConfigureError": {
			reason: "We should return any error we encounter configuring the claim resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
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
			},
			want: want{
				err: errors.Wrap(errBoom, errConfigureClaim),
			},
		},
		"CompositeNotReady": {
			reason: "We should return early if the bound composite resource is not yet ready",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
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
			},
			want: want{
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
								switch o := obj.(type) {
								case *claim.Unstructured:
									o.SetResourceReference(&corev1.ObjectReference{})
								case *composite.Unstructured:
									o.SetConditions(xpv1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
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
			},
			want: want{
				err: errors.Wrap(errBoom, errPropagateCDs),
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
								switch o := obj.(type) {
								case *claim.Unstructured:
									o.SetResourceReference(&corev1.ObjectReference{})
								case *composite.Unstructured:
									o.SetConditions(xpv1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
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
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, tc.args.of, tc.args.with, tc.args.opts...)
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
