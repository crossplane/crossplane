/*
Copyright 2018 The Crossplane Authors.

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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var _ reconcile.Reconciler = &ClaimReconciler{}

type MockManager struct {
	manager.Manager

	c client.Client
	s *runtime.Scheme
}

func (m *MockManager) GetClient() client.Client   { return m.c }
func (m *MockManager) GetScheme() *runtime.Scheme { return m.s }

var MockGV = schema.GroupVersion{Group: "g", Version: "v"}

func MockGVK(o runtime.Object) schema.GroupVersionKind {
	return MockGV.WithKind(reflect.TypeOf(o).Elem().Name())
}

func MockSchemeWith(o ...runtime.Object) *runtime.Scheme {
	s := runtime.NewScheme()
	s.AddKnownTypes(MockGV, o...)
	return s
}

func TestReconcile(t *testing.T) {
	type args struct {
		m    manager.Manager
		of   ClaimKind
		with ManagedKind
		o    []ClaimReconcilerOption
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	errUnexpected := errors.New("unexpected object type")
	now := metav1.Now()

	cases := map[string]struct {
		args args
		want want
	}{
		"GetClaimError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetClaim)},
		},
		"GetManagedError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *MockManaged:
								return errBoom
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"FinalizeManagedError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedFinalizer(ManagedFinalizerFn(func(_ context.Context, _ Managed) error { return errBoom })),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"FinalizeManagedSuccess": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedFinalizer(ManagedFinalizerFn(func(_ context.Context, _ Managed) error { return nil })),
					WithClaimFinalizer(ClaimFinalizerFn(func(_ context.Context, _ Claim) error { return nil })),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"FinalizeClaimError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedFinalizer(ManagedFinalizerFn(func(_ context.Context, _ Managed) error { return nil })),
					WithClaimFinalizer(ClaimFinalizerFn(func(_ context.Context, _ Claim) error { return errBoom })),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"FinalizeClaimSuccess": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedFinalizer(ManagedFinalizerFn(func(_ context.Context, _ Managed) error { return nil })),
					WithClaimFinalizer(ClaimFinalizerFn(func(_ context.Context, _ Claim) error { return nil })),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"GetResourceClassError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *v1alpha1.ResourceClass:
								return errBoom
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ConfigureManagedError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *v1alpha1.ResourceClass:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{WithManagedConfigurators(ManagedConfiguratorFn(
					func(_ context.Context, _ Claim, _ *v1alpha1.ResourceClass, _ Managed) error { return errBoom },
				))},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"CreateManagedError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *v1alpha1.ResourceClass:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedConfigurators(ManagedConfiguratorFn(
						func(_ context.Context, _ Claim, _ *v1alpha1.ResourceClass, _ Managed) error { return nil },
					)),
					WithManagedCreator(ManagedCreatorFn(
						func(_ context.Context, _ Claim, _ *v1alpha1.ResourceClass, _ Managed) error { return errBoom },
					)),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ManagedIsInUnknownBindingPhase": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *MockManaged:
								// We do not explicitly set a BindingPhase here
								// because the zero value of BindingPhase is
								// BindingPhaseUnknown.
								mg := &MockManaged{}
								mg.SetCreationTimestamp(now)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ManagedIsInUnbindableBindingPhase": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *MockManaged:
								mg := &MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbindable)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"PropagateConnectionError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *MockManaged:
								mg := &MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedConnectionPropagator(ManagedConnectionPropagatorFn(
						func(_ context.Context, _ Claim, _ Managed) error { return errBoom },
					)),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"BindError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *MockManaged:
								mg := &MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedConnectionPropagator(ManagedConnectionPropagatorFn(
						func(_ context.Context, _ Claim, _ Managed) error { return nil },
					)),
					WithManagedBinder(ManagedBinderFn(
						func(_ context.Context, _ Claim, _ Managed) error { return errBoom },
					)),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"Successful": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								cm := &MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *MockManaged:
								mg := &MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockManaged{}),
				},
				of:   ClaimKind(MockGVK(&MockClaim{})),
				with: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewClaimReconciler(tc.args.m, tc.args.of, tc.args.with, tc.args.o...)
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
