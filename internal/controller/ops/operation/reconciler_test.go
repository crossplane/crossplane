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

package operation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	"github.com/crossplane/crossplane/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/proto/fn/v1"
)

func TestReconcile(t *testing.T) {
	type params struct {
		mgr  manager.Manager
		opts []ReconcilerOption
	}

	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		want   want
	}{
		"NotFound": {
			reason: "We should return early if the Operation was not found.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetError": {
			reason: "We should return an error if we can't get the Operation",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errors.New("boom")),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"Deleted": {
			reason: "We should return early if the Operation was deleted.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							op := &v1alpha1.Operation{
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: ptr.To(metav1.Now()),
								},
							}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"Complete": {
			reason: "We should return early if the Operation is complete.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							op := &v1alpha1.Operation{}
							op.SetConditions(v1alpha1.Complete())
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"RetryLimitReached": {
			reason: "We should return early if the Operation retry limit was reached.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									RetryLimit: ptr.To[int64](3),
								},
								Status: v1alpha1.OperationStatus{
									Failures: 3,
								},
							}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"UpdateStatusToRunningError": {
			reason: "We should return an error if we can't update the Operation's status to indicate it's running.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							op := &v1alpha1.Operation{}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errors.New("boom")),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"GetCredentialSecretError": {
			reason: "We should return an error if we can't get function credentials from a Secret",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if _, ok := obj.(*corev1.Secret); ok {
								return errors.New("boom")
							}

							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "get-creds",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "function-cool",
											},
											Credentials: []v1alpha1.FunctionCredentials{
												{
													Name:   "doesnt-exist",
													Source: v1alpha1.FunctionCredentialsSourceSecret,
													SecretRef: &v1.SecretReference{
														Namespace: "default",
														Name:      "creds",
													},
												},
											},
										},
									},
								},
							}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"RunFunctionError": {
			reason: "We should return an error if we can't run a function",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if _, ok := obj.(*corev1.Secret); ok {
								return errors.New("boom")
							}

							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "get-creds",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "function-cool",
											},
										},
									},
								},
							}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithFunctionRunner(xfn.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
						return nil, errors.New("boom")
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"FatalResultError": {
			reason: "We should return an error if a function returns a fatal result.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if _, ok := obj.(*corev1.Secret); ok {
								return errors.New("boom")
							}

							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "get-creds",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "function-cool",
											},
										},
									},
								},
							}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithFunctionRunner(xfn.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
						rsp := &fnv1.RunFunctionResponse{
							Results: []*fnv1.Result{
								{
									Severity: fnv1.Severity_SEVERITY_FATAL,
									Message:  "bad stuff!",
								},
							},
						}
						return rsp, nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"PatchResourceError": {
			reason: "We should return an error if we can't patch a desired resource",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if _, ok := obj.(*corev1.Secret); ok {
								return errors.New("boom")
							}

							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "get-creds",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "function-cool",
											},
										},
									},
								},
							}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockPatch:        test.NewMockPatchFn(errors.New("boom")),
					},
				},
				opts: []ReconcilerOption{
					WithFunctionRunner(xfn.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
						rsp := &fnv1.RunFunctionResponse{
							Desired: &fnv1.State{
								Resources: map[string]*fnv1.Resource{
									"patch-me": {
										Resource: MustStructJSON(`{
											"apiVersion": "example.org/v1",
											"kind": "Test",
											"metadata": {
												"name": "patch-me"
											},
											"spec": {
												"widgets": 42
											}
										}`),
									},
								},
							},
						}
						return rsp, nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "We shouldn't return an error if we successfully run the Operation",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							if _, ok := obj.(*corev1.Secret); ok {
								return errors.New("boom")
							}

							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "get-creds",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "function-cool",
											},
										},
									},
								},
							}
							op.DeepCopyInto(obj.(*v1alpha1.Operation))

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
							want := MustUnstructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "Test",
								"metadata": {
									"name": "patch-me"
								},
								"spec": {
									"cool": true
								}
							}`)
							if diff := cmp.Diff(want, obj); diff != "" {
								t.Errorf("Patch(...): -want object, +got object:\n%s", diff)
							}

							return nil
						}),
					},
				},
				opts: []ReconcilerOption{
					WithFunctionRunner(xfn.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
						rsp := &fnv1.RunFunctionResponse{
							Desired: &fnv1.State{
								Resources: map[string]*fnv1.Resource{
									"patch-me": {
										Resource: MustStructJSON(`{
											"apiVersion": "example.org/v1",
											"kind": "Test",
											"metadata": {
												"name": "patch-me"
											},
											"spec": {
												"cool": true
											}
										}`),
									},
								},
							},
						}
						return rsp, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.params.mgr, tc.params.opts...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{})
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want result, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func MustStructJSON(j string) *structpb.Struct {
	s := &structpb.Struct{}
	if err := protojson.Unmarshal([]byte(j), s); err != nil {
		panic(err)
	}
	return s
}

func MustUnstructJSON(j string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(j), u); err != nil {
		panic(err)
	}
	return u
}
