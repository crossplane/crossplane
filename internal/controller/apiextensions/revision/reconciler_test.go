/*
Copyright 2025 The Crossplane Authors.

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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/xfn"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")

	// listControlledRevision lists an active FunctionRevision controlled by the
	// test-function Function, so that name-based steps resolve.
	listControlledRevision := test.NewMockListFn(nil, func(obj client.ObjectList) error {
		obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-function-abc123",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: pkgv1.FunctionGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.FunctionKind,
					Name:       "test-function",
					UID:        "test-function-uid",
					Controller: new(true),
				}},
			},
			Spec: pkgv1.FunctionRevisionSpec{
				PackageRevisionSpec: pkgv1.PackageRevisionSpec{
					DesiredState: pkgv1.PackageRevisionActive,
					Package:      "example.org/test-function:v1",
				},
			},
		}}
		return nil
	})

	type params struct {
		mgr  manager.Manager
		opts []ReconcilerOption
	}

	type args struct {
		ctx context.Context
		req reconcile.Request
	}

	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"CompositionRevisionNotFound": {
			reason: "We should not return an error if the CompositionRevision was not found.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(_ client.Object) error {
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}),
					},
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ValidPipeline": {
			reason: "We should mark the CompositionRevision as having a valid pipeline when all functions have the composition capability.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							switch o := obj.(type) {
							case *v1.CompositionRevision:
								*o = v1.CompositionRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: "test-revision",
									},
									Spec: v1.CompositionRevisionSpec{
										Pipeline: []v1.PipelineStep{
											{
												Step: "test-step",
												FunctionRef: v1.FunctionReference{
													Name: "test-function",
												},
											},
										},
									},
								}
							case *pkgv1.Function:
								o.SetName("test-function")
								o.SetUID("test-function-uid")
								o.Spec.Package = "example.org/test-function:v1"
							}
							return nil
						}),
						MockList:         listControlledRevision,
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLogger(logging.NewNopLogger()),
					WithRecorder(event.NewNopRecorder()),
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, revs ...string) error {
						if diff := cmp.Diff([]string{"test-function-abc123"}, revs); diff != "" {
							t.Errorf("CheckCapabilities(): -want, +got:\n%s", diff)
						}
						return nil
					})),
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-revision"}},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"InvalidPipeline": {
			reason: "We should mark the CompositionRevision as having an invalid pipeline when functions lack the composition capability.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							switch o := obj.(type) {
							case *v1.CompositionRevision:
								*o = v1.CompositionRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: "test-revision",
									},
									Spec: v1.CompositionRevisionSpec{
										Pipeline: []v1.PipelineStep{
											{
												Step: "test-step",
												FunctionRef: v1.FunctionReference{
													Name: "test-function",
												},
											},
										},
									},
								}
							case *pkgv1.Function:
								o.SetName("test-function")
								o.SetUID("test-function-uid")
								o.Spec.Package = "example.org/test-function:v1"
							}
							return nil
						}),
						MockList:         listControlledRevision,
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLogger(logging.NewNopLogger()),
					WithRecorder(event.NewNopRecorder()),
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return errBoom
					})),
				},
			},
			args: args{
				ctx: context.Background(),
				req: reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-revision"}},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.params.mgr, tc.params.opts...)
			got, err := r.Reconcile(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
