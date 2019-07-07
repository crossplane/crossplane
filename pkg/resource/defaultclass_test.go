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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var _ reconcile.Reconciler = &DefaultClassReconciler{}

func TestDefaultClassReconcile(t *testing.T) {
	type args struct {
		m  manager.Manager
		of ClaimKind
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	errUnexpected := errors.New("unexpected object type")
	class := corev1alpha1.ResourceClass{}
	class.SetName("default-class")
	class.SetNamespace("default-namespace")

	cases := map[string]struct {
		args args
		want want
	}{
		"GetClaimError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					s: MockSchemeWith(&MockClaim{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetClaim)},
		},
		"ListDefaultClassesError": {
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
					s: MockSchemeWith(&MockClaim{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
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
							case *corev1alpha1.ResourceClassList:
								cm := &corev1alpha1.ResourceClassList{}
								cm.Items = []corev1alpha1.ResourceClass{}
								*o = *cm
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
					s: MockSchemeWith(&MockClaim{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultClassWait}},
		},
		"MultipleDefaultClasses": {
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
							case *corev1alpha1.ResourceClassList:
								cm := &corev1alpha1.ResourceClassList{}
								cm.Items = []corev1alpha1.ResourceClass{
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
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errMultipleDefaultClasses)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
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
							case *corev1alpha1.ResourceClassList:
								cm := &corev1alpha1.ResourceClassList{}
								cm.Items = []corev1alpha1.ResourceClass{
									class,
								}
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetClassReference(meta.ReferenceTo(class.GetObjectMeta(), corev1alpha1.ResourceClassGroupVersionKind))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewDefaultClassReconciler(tc.args.m, tc.args.of)
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
