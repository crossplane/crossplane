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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")

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
				r: reconcile.Result{},
			},
		},
		"GetCompositeError": {
			reason: "We should requeue after a short wait if we encounter an error while getting the referenced composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
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
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"DeleteCompositeError": {
			reason: "We should requeue after a short wait if we encounter an error while deleting the referenced composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									now := metav1.Now()
									o.SetDeletionTimestamp(&now)
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(errBoom),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"RemoveFinalizerError": {
			reason: "We should requeue after a short wait if we encounter an error while removing the claim's finalizer",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
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
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"SuccessfulDelete": {
			reason: "We should not requeue if we successfully delete the bound composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
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
			reason: "We should requeue after a short wait if we encounter an error while adding the claim's finalizer",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
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
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"ConfigureError": {
			reason: "We should requeue after a short wait if we encounter an error configuring the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
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
					WithCompositeConfigurator(CompositeConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return errBoom })),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"ApplyError": {
			reason: "We should requeue after a short wait if we encounter an error applying the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(CompositeConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"BindError": {
			reason: "We should requeue after a short wait if we encounter an error binding the composite resource",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(CompositeConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return errBoom })),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"CompositeNotReady": {
			reason: "We should return early if the bound composite resource is not yet ready",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if o, ok := obj.(*claim.Unstructured); ok {
									o.SetResourceReference(&corev1.ObjectReference{})
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(CompositeConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"PropagateConnectionError": {
			reason: "We should requeue after a short wait if an error is encountered while propagating the bound composite's connection details",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								switch o := obj.(type) {
								case *claim.Unstructured:
									o.SetResourceReference(&corev1.ObjectReference{})
								case *composite.Unstructured:
									o.SetConditions(v1alpha1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(CompositeConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithConnectionPropagator(ConnectionPropagatorFn(func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
						return false, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"SuccessfulPropagate": {
			reason: "We should not requeue if we successfully applied the composite resource and propagated its connection details",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								switch o := obj.(type) {
								case *claim.Unstructured:
									o.SetResourceReference(&corev1.ObjectReference{})
								case *composite.Unstructured:
									o.SetConditions(v1alpha1.Available())
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error { return nil },
					}),
					WithCompositeConfigurator(CompositeConfiguratorFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
					WithBinder(BinderFn(func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error { return nil })),
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
			got, err := r.Reconcile(reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
