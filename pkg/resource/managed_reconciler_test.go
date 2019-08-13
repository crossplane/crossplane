/*
Copyright 2019 The Crossplane Authors.

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

package resource

import (
	"context"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var _ reconcile.Reconciler = &ManagedReconciler{}

func TestManagedReconciler(t *testing.T) {
	type args struct {
		m  manager.Manager
		mg ManagedKind
		e  ExternalClient
		o  []ManagedReconcilerOption
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	now := metav1.Now()
	testFinalizers := []string{"finalizer.crossplane.io"}
	testConnectionDetails := ConnectionDetails{
		"username": []byte("crossplane.io"),
		"password": []byte("open-cloud"),
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"GetManagedError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetManaged)},
		},
		"ExternalConnectError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ExternalObserveError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{}, errBoom
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"DeletedButResourceExistsAndReclaimDelete": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							mg.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							want.SetConditions(v1alpha1.Deleting())
							want.SetConditions(v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
					DeleteFn: func(_ context.Context, _ Managed) error {
						return nil
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithShortWait(2 * time.Minute),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: 2 * time.Minute}},
		},
		"DeletedButResourceExistsAndReclaimDeleteError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							mg.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							want.SetConditions(v1alpha1.Deleting())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
					DeleteFn: func(_ context.Context, _ Managed) error {
						return errBoom
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"DeletedButResourceExistsAndReclaimRetain": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							mg.SetReclaimPolicy(v1alpha1.ReclaimRetain)
							mg.SetFinalizers([]string{"test"})
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetReclaimPolicy(v1alpha1.ReclaimRetain)
							want.SetConditions(v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							UnpublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error {
								return nil
							},
						}
					},
					func(r *ManagedReconciler) {
						r.managed.ManagedFinalizer = ManagedFinalizerFn(func(_ context.Context, mg Managed) error {
							mg.SetFinalizers(nil)
							return nil
						})
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"DeletedAndResourceDoesNotExist": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.ReconcileSuccess())
							want.SetFinalizers(nil)
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    false,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							UnpublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error {
								return nil
							},
						}
					},
					func(r *ManagedReconciler) {
						r.managed.ManagedFinalizer = ManagedFinalizerFn(func(_ context.Context, mg Managed) error {
							mg.SetFinalizers(nil)
							return nil
						})
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"UnpublishDeletedAndResourceDoesNotExistError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							want.SetFinalizers(nil)
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    false,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedConnectionPublishers(ManagedConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error {
							return errBoom
						},
					}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"FinalizeDeletedAndResourceDoesNotExistError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							want.SetFinalizers(nil)
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    false,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedConnectionPublishers(ManagedConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error {
							return nil
						},
					}),
					func(r *ManagedReconciler) {
						r.managed.ManagedFinalizer = ManagedFinalizerFn(func(_ context.Context, _ Managed) error {
							return errBoom
						})
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ResourceDoesNotExist": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileSuccess())
							want.SetFinalizers(testFinalizers)
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists: false,
						}, nil
					},
					CreateFn: func(_ context.Context, _ Managed) (ExternalCreation, error) {
						return ExternalCreation{
							ConnectionDetails: testConnectionDetails,
						}, nil
					},
				},
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							PublishConnectionFn: func(_ context.Context, _ Managed, c ConnectionDetails) error {
								if len(c) != 0 {
									if diff := cmp.Diff(testConnectionDetails, c); diff != "" {
										t.Errorf("-want, +got:\n%s", diff)
									}
								}
								return nil
							},
						}
					},
					func(r *ManagedReconciler) {
						r.managed.ManagedEstablisher = ManagedEstablisherFn(func(_ context.Context, mg Managed) error {
							mg.SetFinalizers(testFinalizers)
							return nil
						})
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"CreateResourceDoesNotExistError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							want.SetFinalizers(testFinalizers)
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists: false,
						}, nil
					},
					CreateFn: func(_ context.Context, _ Managed) (ExternalCreation, error) {
						return ExternalCreation{}, errBoom
					},
				},
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							PublishConnectionFn: func(_ context.Context, _ Managed, c ConnectionDetails) error {
								if diff := cmp.Diff(0, len(c)); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							},
						}
					},
					func(r *ManagedReconciler) {
						r.managed.ManagedEstablisher = ManagedEstablisherFn(func(_ context.Context, mg Managed) error {
							mg.SetFinalizers(testFinalizers)
							return nil
						})
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"EstablishResourceDoesNotExistError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists: false,
						}, nil
					},
				},
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							PublishConnectionFn: func(_ context.Context, _ Managed, c ConnectionDetails) error {
								if diff := cmp.Diff(0, len(c)); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							},
						}
					},
					func(r *ManagedReconciler) {
						r.managed.ManagedEstablisher = ManagedEstablisherFn(func(_ context.Context, _ Managed) error {
							return errBoom
						})
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ResourceExists": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: testConnectionDetails,
						}, nil
					},
					UpdateFn: func(_ context.Context, _ Managed) (ExternalUpdate, error) {
						return ExternalUpdate{}, nil
					},
				},
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							PublishConnectionFn: func(_ context.Context, _ Managed, c ConnectionDetails) error {
								if len(c) != 0 {
									if diff := cmp.Diff(testConnectionDetails, c); diff != "" {
										t.Errorf("-want, +got:\n%s", diff)
									}
								}
								return nil
							},
						}
					},
					WithLongWait(5 * time.Minute),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: 5 * time.Minute}},
		},
		"PublishResourceExistsError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: testConnectionDetails,
						}, nil
					},
					UpdateFn: func(_ context.Context, _ Managed) (ExternalUpdate, error) {
						return ExternalUpdate{}, nil
					},
				},
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							PublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error {
								return errBoom
							},
						}
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"UpdateResourceExistsError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				e: &ExternalClientFns{
					ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: testConnectionDetails,
						}, nil
					},
					UpdateFn: func(_ context.Context, _ Managed) (ExternalUpdate, error) {
						return ExternalUpdate{}, errBoom
					},
				},
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFns{
							PublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error {
								return nil
							},
						}
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			defaultOptions := []ManagedReconcilerOption{
				func(r *ManagedReconciler) {
					r.external = mrExternal{
						ExternalConnecter: ExternalConnectorFn(func(_ context.Context, _ Managed) (ExternalClient, error) {
							return tc.args.e, nil
						}),
					}
				},
			}
			tc.args.o = append(defaultOptions, tc.args.o...)

			r := NewManagedReconciler(tc.args.m, tc.args.mg, tc.args.o...)
			got, err := r.Reconcile(reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r.Reconcile(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("r.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}
