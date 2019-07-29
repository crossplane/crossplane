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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var _ reconcile.Reconciler = &DefaultClassReconciler{}

func TestDefaultClassReconcile(t *testing.T) {
	type args struct {
		m      manager.Manager
		of     ClaimKind
		by     PolicyKind
		byList PolicyListKind
		or     ClusterPolicyKind
		orList ClusterPolicyListKind
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	errUnexpected := errors.New("unexpected object type")

	defClassRef := &corev1.ObjectReference{
		Name:      "default-class",
		Namespace: "default-namespace",
	}
	policy := MockPolicy{}
	policy.SetDefaultClassReference(defClassRef)

	defClassRefCluster := &corev1.ObjectReference{
		Name:      "another-default-class",
		Namespace: "another-default-namespace",
	}
	clusterPolicy := MockClusterPolicy{}
	clusterPolicy.SetDefaultClassReference(defClassRefCluster)

	cases := map[string]struct {
		args args
		want want
	}{
		"GetClaimError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}, &MockClusterPolicy{}, &MockClusterPolicyList{}),
				},
				of:     ClaimKind(MockGVK(&MockClaim{})),
				by:     PolicyKind(MockGVK(&MockPolicy{})),
				byList: PolicyListKind(MockGVK(&MockPolicyList{})),
				or:     ClusterPolicyKind(MockGVK(&MockClusterPolicy{})),
				orList: ClusterPolicyListKind(MockGVK(&MockClusterPolicyList{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetClaim)},
		},
		"ListPoliciesError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(errBoom),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errFailedList)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}, &MockClusterPolicy{}, &MockClusterPolicyList{}),
				},
				of:     ClaimKind(MockGVK(&MockClaim{})),
				by:     PolicyKind(MockGVK(&MockPolicy{})),
				byList: PolicyListKind(MockGVK(&MockPolicyList{})),
				or:     ClusterPolicyKind(MockGVK(&MockClusterPolicy{})),
				orList: ClusterPolicyListKind(MockGVK(&MockClusterPolicyList{})),
			},
			want: want{result: reconcile.Result{}},
		},
		"NoDefaultClass": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPolicyList:
								*o = MockPolicyList{}
								return nil
							case *MockClusterPolicyList:
								*o = MockClusterPolicyList{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errNoDefaultClass)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}, &MockClusterPolicy{}, &MockClusterPolicyList{}),
				},
				of:     ClaimKind(MockGVK(&MockClaim{})),
				by:     PolicyKind(MockGVK(&MockPolicy{})),
				byList: PolicyListKind(MockGVK(&MockPolicyList{})),
				or:     ClusterPolicyKind(MockGVK(&MockClusterPolicy{})),
				orList: ClusterPolicyListKind(MockGVK(&MockClusterPolicyList{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultClassWait}},
		},
		"MultipleDefaultClassPolicies": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPolicyList:
								cm := &MockPolicyList{}
								cm.Items = []MockPolicy{
									{},
									{},
								}
								*o = *cm
								return nil
							case *MockClusterPolicyList:
								*o = MockClusterPolicyList{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errMultiplePolicies)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}, &MockClusterPolicy{}, &MockClusterPolicyList{}),
				},
				of:     ClaimKind(MockGVK(&MockClaim{})),
				by:     PolicyKind(MockGVK(&MockPolicy{})),
				byList: PolicyListKind(MockGVK(&MockPolicyList{})),
				or:     ClusterPolicyKind(MockGVK(&MockClusterPolicy{})),
				orList: ClusterPolicyListKind(MockGVK(&MockClusterPolicyList{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultClassWait}},
		},
		"MultipleDefaultClassClusterPolicies": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPolicyList:
								*o = MockPolicyList{}
								return nil
							case *MockClusterPolicyList:
								cm := &MockClusterPolicyList{}
								cm.Items = []MockClusterPolicy{
									{},
									{},
								}
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errMultipleClusterPolicies)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}, &MockClusterPolicy{}, &MockClusterPolicyList{}),
				},
				of:     ClaimKind(MockGVK(&MockClaim{})),
				by:     PolicyKind(MockGVK(&MockPolicy{})),
				byList: PolicyListKind(MockGVK(&MockPolicyList{})),
				or:     ClusterPolicyKind(MockGVK(&MockClusterPolicy{})),
				orList: ClusterPolicyListKind(MockGVK(&MockClusterPolicyList{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultClassWait}},
		},
		"NamespaceScopedSuccessful": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPolicyList:
								cm := &MockPolicyList{}
								cm.Items = []MockPolicy{
									policy,
								}
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetClassReference(policy.GetDefaultClassReference())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}, &MockClusterPolicy{}, &MockClusterPolicyList{}),
				},
				of:     ClaimKind(MockGVK(&MockClaim{})),
				by:     PolicyKind(MockGVK(&MockPolicy{})),
				byList: PolicyListKind(MockGVK(&MockPolicyList{})),
				or:     ClusterPolicyKind(MockGVK(&MockClusterPolicy{})),
				orList: ClusterPolicyListKind(MockGVK(&MockClusterPolicyList{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ClusterScopedSuccessful": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPolicyList:
								cm := &MockPolicyList{}
								cm.Items = []MockPolicy{}
								*o = *cm
								return nil
							case *MockClusterPolicyList:
								cm := &MockClusterPolicyList{}
								cm.Items = []MockClusterPolicy{
									clusterPolicy,
								}
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetClassReference(clusterPolicy.GetDefaultClassReference())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}, &MockClusterPolicy{}, &MockClusterPolicyList{}),
				},
				of:     ClaimKind(MockGVK(&MockClaim{})),
				by:     PolicyKind(MockGVK(&MockPolicy{})),
				byList: PolicyListKind(MockGVK(&MockPolicyList{})),
				or:     ClusterPolicyKind(MockGVK(&MockClusterPolicy{})),
				orList: ClusterPolicyListKind(MockGVK(&MockClusterPolicyList{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewDefaultClassReconciler(tc.args.m, tc.args.of, tc.args.by, tc.args.byList, tc.args.or, tc.args.orList)
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
