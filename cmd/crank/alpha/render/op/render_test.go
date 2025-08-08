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

package op

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	opsv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/cmd/crank/render"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func TestRender(t *testing.T) {
	// Add all listeners here so we can close them to shutdown our gRPC servers.
	listeners := make([]io.Closer, 0)

	type args struct {
		ctx context.Context
		in  Inputs
	}

	type want struct {
		out Outputs
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessEmptyPipeline": {
			reason: "Should successfully render an operation with no pipeline steps",
			args: args{
				ctx: context.Background(),
				in: Inputs{
					Operation: &opsv1alpha1.Operation{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "ops.crossplane.io/v1alpha1",
							Kind:       "Operation",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-operation",
							Namespace: "default",
						},
						Spec: opsv1alpha1.OperationSpec{
							Pipeline: []opsv1alpha1.PipelineStep{},
						},
					},
					Functions: []pkgv1.Function{},
				},
			},
			want: want{
				out: Outputs{
					Operation: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "ops.crossplane.io/v1alpha1",
							"kind":       "Operation",
							"metadata": &metav1.ObjectMeta{
								Name:      "test-operation",
								Namespace: "default",
							},
							"status": map[string]any{
								"appliedResourceRefs": []any{},
								"pipeline":            []any{},
							},
						},
					},
					Resources: []unstructured.Unstructured{},
					Results:   []unstructured.Unstructured{},
				},
			},
		},
		"ErrorInvalidInput": {
			reason: "Should return an error if function input is invalid JSON",
			args: args{
				in: Inputs{
					Operation: &opsv1alpha1.Operation{
						Spec: opsv1alpha1.OperationSpec{
							Pipeline: []opsv1alpha1.PipelineStep{
								{
									Step: "invalid-input",
									FunctionRef: opsv1alpha1.FunctionReference{
										Name: "function-test",
									},
									Input: &runtime.RawExtension{
										Raw: []byte("invalid json"),
									},
								},
							},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorMissingSecret": {
			reason: "Should return an error if credentials secret is missing",
			args: args{
				ctx: context.Background(),
				in: Inputs{
					Operation: &opsv1alpha1.Operation{
						Spec: opsv1alpha1.OperationSpec{
							Pipeline: []opsv1alpha1.PipelineStep{
								{
									Step: "needs-secret",
									FunctionRef: opsv1alpha1.FunctionReference{
										Name: "function-test",
									},
									Credentials: []opsv1alpha1.FunctionCredentials{
										{
											Name:   "secret-creds",
											Source: opsv1alpha1.FunctionCredentialsSourceSecret,
											SecretRef: &xpv1.SecretReference{
												Name:      "missing-secret",
												Namespace: "default",
											},
										},
									},
								},
							},
						},
					},
					Functions: []pkgv1.Function{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "function-test",
							},
						},
					},
					FunctionCredentials: []corev1.Secret{},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorMissingFunction": {
			reason: "Should return an error if function is not found",
			args: args{
				in: Inputs{
					Operation: &opsv1alpha1.Operation{
						Spec: opsv1alpha1.OperationSpec{
							Pipeline: []opsv1alpha1.PipelineStep{
								{
									Step: "missing-function",
									FunctionRef: opsv1alpha1.FunctionReference{
										Name: "missing-function",
									},
								},
							},
						},
					},
					Functions: []pkgv1.Function{},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorInvalidContextData": {
			reason: "Should return an error if context data is invalid JSON",
			args: args{
				in: Inputs{
					Operation: &opsv1alpha1.Operation{
						Spec: opsv1alpha1.OperationSpec{
							Pipeline: []opsv1alpha1.PipelineStep{},
						},
					},
					Functions: []pkgv1.Function{},
					Context: map[string][]byte{
						"invalid": []byte("invalid json"),
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"FatalResult": {
			reason: "Should return an error if function returns fatal result",
			args: args{
				ctx: context.Background(),
				in: Inputs{
					Operation: &opsv1alpha1.Operation{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "ops.crossplane.io/v1alpha1",
							Kind:       "Operation",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-operation",
						},
						Spec: opsv1alpha1.OperationSpec{
							Pipeline: []opsv1alpha1.PipelineStep{
								{
									Step: "fatal-step",
									FunctionRef: opsv1alpha1.FunctionReference{
										Name: "function-fatal",
									},
								},
							},
						},
					},
					Functions: []pkgv1.Function{
						func() pkgv1.Function {
							lis := NewFunction(t, &fnv1.RunFunctionResponse{
								Results: []*fnv1.Result{
									{
										Severity: fnv1.Severity_SEVERITY_FATAL,
										Message:  "Something went wrong",
									},
								},
							})
							listeners = append(listeners, lis)

							return pkgv1.Function{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-fatal",
									Annotations: map[string]string{
										render.AnnotationKeyRuntime:                  string(render.AnnotationValueRuntimeDevelopment),
										render.AnnotationKeyRuntimeDevelopmentTarget: lis.Addr().String(),
									},
								},
							}
						}(),
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "Should successfully render operation with function creating resources",
			args: args{
				ctx: context.Background(),
				in: Inputs{
					Operation: &opsv1alpha1.Operation{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "ops.crossplane.io/v1alpha1",
							Kind:       "Operation",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-operation",
						},
						Spec: opsv1alpha1.OperationSpec{
							Pipeline: []opsv1alpha1.PipelineStep{
								{
									Step: "create-configmap",
									FunctionRef: opsv1alpha1.FunctionReference{
										Name: "function-test",
									},
								},
							},
						},
					},
					Functions: []pkgv1.Function{
						func() pkgv1.Function {
							lis := NewFunction(t, &fnv1.RunFunctionResponse{
								Desired: &fnv1.State{
									Resources: map[string]*fnv1.Resource{
										"cool-configmap": {
											Resource: MustStructJSON(`{
												"apiVersion": "v1",
												"kind": "ConfigMap",
												"metadata": {
													"name": "cool-map",
													"namespace": "default"
												},
												"data": {
													"coolData": "I'm cool!"
												}
											}`),
										},
									},
								},
								Results: []*fnv1.Result{
									{
										Severity: fnv1.Severity_SEVERITY_NORMAL,
										Message:  "Created a ConfigMap!",
									},
								},
							})
							listeners = append(listeners, lis)

							return pkgv1.Function{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-test",
									Annotations: map[string]string{
										render.AnnotationKeyRuntime:                  string(render.AnnotationValueRuntimeDevelopment),
										render.AnnotationKeyRuntimeDevelopmentTarget: lis.Addr().String(),
									},
								},
							}
						}(),
					},
				},
			},
			want: want{
				out: Outputs{
					Operation: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "ops.crossplane.io/v1alpha1",
							"kind":       "Operation",
							"metadata": &metav1.ObjectMeta{
								Name: "test-operation",
							},
							"status": map[string]any{
								"appliedResourceRefs": []any{
									map[string]any{
										"apiVersion": "v1",
										"kind":       "ConfigMap",
										"name":       "cool-map",
										"namespace":  "default",
									},
								},
								"pipeline": []any{},
							},
						},
					},
					Resources: []unstructured.Unstructured{
						{
							Object: MustLoadJSON(`{
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"metadata": {
									"name": "cool-map",
									"namespace": "default"
								},
								"data": {
									"coolData": "I'm cool!"
								}
							}`),
						},
					},
					Results: []unstructured.Unstructured{
						{
							Object: map[string]any{
								"apiVersion": "render.crossplane.io/v1beta1",
								"kind":       "Result",
								"step":       "create-configmap",
								"severity":   "SEVERITY_NORMAL",
								"message":    "Created a ConfigMap!",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Render(tc.args.ctx, logging.NewNopLogger(), tc.args.in)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.out, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

	for _, l := range listeners {
		l.Close()
	}
}

func NewFunction(t *testing.T, rsp *fnv1.RunFunctionResponse) net.Listener {
	t.Helper()

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	fnv1.RegisterFunctionRunnerServiceServer(srv, &MockFunctionRunner{Response: rsp})

	go srv.Serve(lis) // This will stop when lis is closed.

	return lis
}

func NewFunctionWithRunFunc(t *testing.T, runFunc func(context.Context, *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)) net.Listener {
	t.Helper()

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	fnv1.RegisterFunctionRunnerServiceServer(srv, &MockFunctionRunner{RunFunc: runFunc})

	go srv.Serve(lis) // This will stop when lis is closed.

	return lis
}

type MockFunctionRunner struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	Response *fnv1.RunFunctionResponse
	RunFunc  func(context.Context, *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)
	Error    error
}

func (r *MockFunctionRunner) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	if r.Response != nil {
		return r.Response, r.Error
	}

	return r.RunFunc(ctx, req)
}

func MustStructJSON(j string) *structpb.Struct {
	s := &structpb.Struct{}
	if err := protojson.Unmarshal([]byte(j), s); err != nil {
		panic(err)
	}

	return s
}

func MustLoadJSON(j string) map[string]any {
	out := make(map[string]any)
	if err := json.Unmarshal([]byte(j), &out); err != nil {
		panic(err)
	}

	return out
}
