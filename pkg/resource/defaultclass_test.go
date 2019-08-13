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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var _ reconcile.Reconciler = &DefaultClassReconciler{}

type MockObjectConvertor struct {
	runtime.ObjectConvertor
}

func (m *MockObjectConvertor) Convert(in, out, context interface{}) error {
	i, inok := in.(*unstructured.Unstructured)
	if !inok {
		return errors.Errorf("expected conversion input to be of type %s", reflect.TypeOf(unstructured.Unstructured{}).String())
	}
	_, outok := out.(*MockPolicy)
	if !outok {
		return errors.Errorf("expected conversion input to be of type %s", reflect.TypeOf(MockPolicy{}).String())
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(i.Object, out); err != nil {
		return err
	}
	return nil
}

func TestDefaultClassReconcile(t *testing.T) {
	type args struct {
		m  manager.Manager
		of ClaimKind
		by PolicyKind
		o  []DefaultClassReconcilerOption
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
	convPolicy, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&policy)
	unPolicy := unstructured.Unstructured{Object: convPolicy}

	cases := map[string]struct {
		args args
		want want
	}{
		"GetClaimError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PolicyKind{Singular: MockGVK(&MockPolicy{}), Plural: MockGVK(&MockPolicyList{})},
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
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PolicyKind{Singular: MockGVK(&MockPolicy{}), Plural: MockGVK(&MockPolicyList{})},
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
							case *unstructured.UnstructuredList:
								*o = unstructured.UnstructuredList{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errNoPolicies)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PolicyKind{Singular: MockGVK(&MockPolicy{}), Plural: MockGVK(&MockPolicyList{})},
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
							case *unstructured.UnstructuredList:
								cm := &unstructured.UnstructuredList{}
								cm.Items = []unstructured.Unstructured{
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
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errMultiplePolicies)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PolicyKind{Singular: MockGVK(&MockPolicy{}), Plural: MockGVK(&MockPolicyList{})},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultClassWait}},
		},
		"Successful": {
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
							case *unstructured.UnstructuredList:
								cm := &unstructured.UnstructuredList{}
								cm.Items = []unstructured.Unstructured{
									unPolicy,
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
					s: MockSchemeWith(&MockClaim{}, &MockPolicy{}, &MockPolicyList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PolicyKind{Singular: MockGVK(&MockPolicy{}), Plural: MockGVK(&MockPolicyList{})},
				o:  []DefaultClassReconcilerOption{WithObjectConverter(&MockObjectConvertor{})},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewDefaultClassReconciler(tc.args.m, tc.args.of, tc.args.by, tc.args.o...)
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
