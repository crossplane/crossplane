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

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var _ reconcile.Reconciler = &ManagedReconciler{}

// Scenarios: (TODO:delete this block after impl is done)
// create external from managed
// update external with change from managed
// delete managed and then purge external with ReclaimDelete: + check finalizer scenario, deletion should be done after a few calls
// delete managed and see if external is touched with ReclaimRetain

func TestManagedReconciler_Reconcile(t *testing.T) {
	//nopClient := NopClient{}
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
	//errUnexpected := errors.New("unexpected object type")
	now := metav1.Now()

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
						MockStatusUpdate: test.MockStatusUpdateFn(func(ctx context.Context, obj runtime.Object) error {
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
					func(r *ManagedReconciler) {
						r.external = mrExternal{
							ExternalConnecter: ExternalConnectorFn(func(ctx context.Context, mg Managed) (ExternalClient, error) {
								return &NopClient{}, errBoom
							}),
						}
					},
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ExternalObserveError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(ctx context.Context, obj runtime.Object) error {
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
				e: &ExternalClientFn{
					ObserveFn: func(ctx context.Context, mg Managed) (observation ExternalObservation, e error) {
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
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(ctx context.Context, obj runtime.Object) error {
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
				e: &ExternalClientFn{
					ObserveFn: func(ctx context.Context, mg Managed) (observation ExternalObservation, e error) {
						mg.SetDeletionTimestamp(&now)
						mg.SetReclaimPolicy(v1alpha1.ReclaimDelete)
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
					DeleteFn: func(ctx context.Context, mg Managed) error {
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
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(ctx context.Context, obj runtime.Object) error {
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
				e: &ExternalClientFn{
					ObserveFn: func(ctx context.Context, mg Managed) (observation ExternalObservation, e error) {
						mg.SetDeletionTimestamp(&now)
						mg.SetReclaimPolicy(v1alpha1.ReclaimRetain)
						mg.SetFinalizers([]string{"test"})
						return ExternalObservation{
							ResourceExists:    true,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFn{
							UnpublishConnectionFn: func(ctx context.Context, mg Managed, c ConnectionDetails) error {
								return nil
							},
						}
					},
					func(r *ManagedReconciler) {
						r.managed.ManagedFinalizer = ManagedFinalizerFn(func(ctx context.Context, mg Managed) error {
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
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(ctx context.Context, obj runtime.Object) error {
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
				e: &ExternalClientFn{
					ObserveFn: func(ctx context.Context, mg Managed) (observation ExternalObservation, e error) {
						mg.SetDeletionTimestamp(&now)
						return ExternalObservation{
							ResourceExists:    false,
							ConnectionDetails: map[string][]byte{},
						}, nil
					},
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					func(r *ManagedReconciler) {
						r.managed.ManagedConnectionPublisher = ManagedConnectionPublisherFn{
							UnpublishConnectionFn: func(ctx context.Context, mg Managed, c ConnectionDetails) error {
								return nil
							},
						}
					},
					func(r *ManagedReconciler) {
						r.managed.ManagedFinalizer = ManagedFinalizerFn(func(ctx context.Context, mg Managed) error {
							mg.SetFinalizers(nil)
							return nil
						})
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
						ExternalConnecter: ExternalConnectorFn(func(ctx context.Context, mg Managed) (ExternalClient, error) {
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
