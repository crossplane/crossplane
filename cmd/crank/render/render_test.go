/*
Copyright 2023 The Crossplane Authors.

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

package render

import (
	"context"
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
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
)

var (
	_ composite.FunctionRunner        = &RuntimeFunctionRunner{}
	_ composite.ExtraResourcesFetcher = &FilteringFetcher{}
)

func TestRender(t *testing.T) {
	pipeline := apiextensionsv1.CompositionModePipeline

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
		rsp    *fnv1.RunFunctionResponse
		args   args
		want   want
	}{
		"InvalidContextValue": {
			args: args{
				in: Inputs{
					CompositeResource: ucomposite.New(),
					Context: map[string][]byte{
						"not-valid-json": []byte(`{`),
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidInput": {
			args: args{
				in: Inputs{
					CompositeResource: ucomposite.New(),
					Composition: &apiextensionsv1.Composition{
						Spec: apiextensionsv1.CompositionSpec{
							Pipeline: []apiextensionsv1.PipelineStep{
								{
									// Not valid JSON.
									Input: &runtime.RawExtension{Raw: []byte(`{`)},
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
		"UnknownRuntime": {
			args: args{
				in: Inputs{
					Functions: []pkgv1.Function{{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								AnnotationKeyRuntime: "wat",
							},
						},
					}},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"UnknownFunction": {
			args: args{
				ctx: context.Background(),
				in: Inputs{
					CompositeResource: ucomposite.New(),
					Composition: &apiextensionsv1.Composition{
						Spec: apiextensionsv1.CompositionSpec{
							Mode: &pipeline,
							Pipeline: []apiextensionsv1.PipelineStep{
								{
									Step:        "test",
									FunctionRef: apiextensionsv1.FunctionReference{Name: "function-test"},
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
		"FatalResult": {
			args: args{
				ctx: context.Background(),
				in: Inputs{
					CompositeResource: ucomposite.New(),
					Composition: &apiextensionsv1.Composition{
						Spec: apiextensionsv1.CompositionSpec{
							Mode: &pipeline,
							Pipeline: []apiextensionsv1.PipelineStep{
								{
									Step:        "test",
									FunctionRef: apiextensionsv1.FunctionReference{Name: "function-test"},
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
									},
								},
							})
							listeners = append(listeners, lis)

							return pkgv1.Function{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-test",
									Annotations: map[string]string{
										AnnotationKeyRuntime:                  string(AnnotationValueRuntimeDevelopment),
										AnnotationKeyRuntimeDevelopmentTarget: lis.Addr().String(),
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
			args: args{
				ctx: context.Background(),
				in: Inputs{
					CompositeResource: &ucomposite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								}
							}`),
						},
					},
					Composition: &apiextensionsv1.Composition{
						Spec: apiextensionsv1.CompositionSpec{
							Mode: &pipeline,
							Pipeline: []apiextensionsv1.PipelineStep{
								{
									Step:        "test",
									FunctionRef: apiextensionsv1.FunctionReference{Name: "function-test"},
								},
							},
						},
					},
					Functions: []pkgv1.Function{
						func() pkgv1.Function {
							lis := NewFunction(t, &fnv1.RunFunctionResponse{
								Desired: &fnv1.State{
									Composite: &fnv1.Resource{
										Resource: MustStructJSON(`{
											"status": {
												"widgets": 9001,
												"conditions": [{
													"lastTransitionTime": "2024-01-01T00:00:00Z",
													"type": "Ready",
													"status": "False",
													"reason": "Creating",
													"message": "Unready resources: a-cool-resource, b-cool-resource"
												}]
											}
										}`),
									},
									Resources: map[string]*fnv1.Resource{
										"b-cool-resource": {
											Resource: MustStructJSON(`{
												"apiVersion": "atest.crossplane.io/v1",
												"kind": "AComposed",
												"spec": {
													"widgets": 9003
												}
											}`),
										},
										"a-cool-resource": {
											Resource: MustStructJSON(`{
												"apiVersion": "btest.crossplane.io/v1",
												"kind": "BComposed",
												"spec": {
													"widgets": 9002
												}
											}`),
										},
									},
								},
								Conditions: []*fnv1.Condition{
									{
										Type:    "ProvisioningSuccess",
										Status:  fnv1.Status_STATUS_CONDITION_TRUE,
										Reason:  "Provisioned",
										Message: ptr.To("Provisioned successfully"),
										Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
									},
								},
							})
							listeners = append(listeners, lis)

							return pkgv1.Function{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-test",
									Annotations: map[string]string{
										AnnotationKeyRuntime:                  string(AnnotationValueRuntimeDevelopment),
										AnnotationKeyRuntimeDevelopmentTarget: lis.Addr().String(),
									},
								},
							}
						}(),
					},
					Context: map[string][]byte{
						"crossplane.io/context-key": []byte(`{}`),
					},
				},
			},
			want: want{
				out: Outputs{
					CompositeResource: &ucomposite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								},
								"status": {
									"widgets": 9001,
									"conditions": [
										{
											"lastTransitionTime": "2024-01-01T00:00:00Z",
											"type": "Ready",
											"status": "False",
											"reason": "Creating",
											"message": "Unready resources: a-cool-resource, b-cool-resource"
										},
										{
											"lastTransitionTime": "2024-01-01T00:00:00Z",
											"type": "ProvisioningSuccess",
											"status": "True",
											"reason": "Provisioned",
											"message": "Provisioned successfully"
										}
									]
								}
							}`),
						},
					},
					ComposedResources: []composed.Unstructured{
						{
							Unstructured: unstructured.Unstructured{
								Object: MustLoadJSON(`{
									"apiVersion": "btest.crossplane.io/v1",
									"metadata": {
										"generateName": "test-render-",
										"labels": {
											"crossplane.io/composite": "test-render"
										},
										"annotations": {
											"crossplane.io/composition-resource-name": "a-cool-resource"
										},
										"ownerReferences": [{
											"apiVersion": "nop.example.org/v1alpha1",
											"kind": "XNopResource",
											"name": "test-render",
											"blockOwnerDeletion": true,
											"controller": true,
											"uid": ""
										}]
									},
									"kind": "BComposed",
									"spec": {
										"widgets": 9002
									}
								}`),
							},
						},
						{
							Unstructured: unstructured.Unstructured{
								Object: MustLoadJSON(`{
									"apiVersion": "atest.crossplane.io/v1",
									"metadata": {
										"generateName": "test-render-",
										"labels": {
											"crossplane.io/composite": "test-render"
										},
										"annotations": {
											"crossplane.io/composition-resource-name": "b-cool-resource"
										},
										"ownerReferences": [{
											"apiVersion": "nop.example.org/v1alpha1",
											"kind": "XNopResource",
											"name": "test-render",
											"blockOwnerDeletion": true,
											"controller": true,
											"uid": ""
										}]
									},
									"kind": "AComposed",
									"spec": {
										"widgets": 9003
									}
								}`),
							},
						},
					},
				},
			},
		},
		"SuccessReady": {
			args: args{
				ctx: context.Background(),
				in: Inputs{
					CompositeResource: &ucomposite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								}
							}`),
						},
					},
					Composition: &apiextensionsv1.Composition{
						Spec: apiextensionsv1.CompositionSpec{
							Mode: &pipeline,
							Pipeline: []apiextensionsv1.PipelineStep{
								{
									Step:        "test",
									FunctionRef: apiextensionsv1.FunctionReference{Name: "function-test"},
								},
							},
						},
					},
					Functions: []pkgv1.Function{
						func() pkgv1.Function {
							lis := NewFunction(t, &fnv1.RunFunctionResponse{
								Desired: &fnv1.State{
									Composite: &fnv1.Resource{
										Resource: MustStructJSON(`{
											"status": {
												"widgets": 9001,
												"conditions": [{
													"lastTransitionTime": "2024-01-01T00:00:00Z",
													"type": "Ready",
													"status": "False",
													"reason": "Creating",
													"message": "Unready resources: a-cool-resource, b-cool-resource"
												}]
											}
										}`),
									},
									Resources: map[string]*fnv1.Resource{
										"b-cool-resource": {
											Resource: MustStructJSON(`{
												"apiVersion": "atest.crossplane.io/v1",
												"kind": "AComposed",
												"spec": {
													"widgets": 9003
												}
											}`),
											Ready: fnv1.Ready_READY_TRUE,
										},
										"a-cool-resource": {
											Resource: MustStructJSON(`{
												"apiVersion": "btest.crossplane.io/v1",
												"kind": "BComposed",
												"spec": {
													"widgets": 9002
												}
											}`),
											Ready: fnv1.Ready_READY_TRUE,
										},
									},
								},
							})
							listeners = append(listeners, lis)

							return pkgv1.Function{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-test",
									Annotations: map[string]string{
										AnnotationKeyRuntime:                  string(AnnotationValueRuntimeDevelopment),
										AnnotationKeyRuntimeDevelopmentTarget: lis.Addr().String(),
									},
								},
							}
						}(),
					},
					Context: map[string][]byte{
						"crossplane.io/context-key": []byte(`{}`),
					},
				},
			},
			want: want{
				out: Outputs{
					CompositeResource: &ucomposite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								},
								"status": {
									"widgets": 9001,
									"conditions": [{
										"lastTransitionTime": "2024-01-01T00:00:00Z",
										"type": "Ready",
										"status": "True",
										"reason": "Available"
									}]
								}
							}`),
						},
					},
					ComposedResources: []composed.Unstructured{
						{
							Unstructured: unstructured.Unstructured{
								Object: MustLoadJSON(`{
									"apiVersion": "btest.crossplane.io/v1",
									"metadata": {
										"generateName": "test-render-",
										"labels": {
											"crossplane.io/composite": "test-render"
										},
										"annotations": {
											"crossplane.io/composition-resource-name": "a-cool-resource"
										},
										"ownerReferences": [{
											"apiVersion": "nop.example.org/v1alpha1",
											"kind": "XNopResource",
											"name": "test-render",
											"blockOwnerDeletion": true,
											"controller": true,
											"uid": ""
										}]
									},
									"kind": "BComposed",
									"spec": {
										"widgets": 9002
									}
								}`),
							},
						},
						{
							Unstructured: unstructured.Unstructured{
								Object: MustLoadJSON(`{
									"apiVersion": "atest.crossplane.io/v1",
									"metadata": {
										"generateName": "test-render-",
										"labels": {
											"crossplane.io/composite": "test-render"
										},
										"annotations": {
											"crossplane.io/composition-resource-name": "b-cool-resource"
										},
										"ownerReferences": [{
											"apiVersion": "nop.example.org/v1alpha1",
											"kind": "XNopResource",
											"name": "test-render",
											"blockOwnerDeletion": true,
											"controller": true,
											"uid": ""
										}]
									},
									"kind": "AComposed",
									"spec": {
										"widgets": 9003
									}
								}`),
							},
						},
					},
				},
			},
		},
		"SuccessWithExtraResources": {
			args: args{
				ctx: context.Background(),
				in: Inputs{
					CompositeResource: &ucomposite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								}
							}`),
						},
					},
					Composition: &apiextensionsv1.Composition{
						Spec: apiextensionsv1.CompositionSpec{
							Mode: &pipeline,
							Pipeline: []apiextensionsv1.PipelineStep{
								{
									Step:        "test",
									FunctionRef: apiextensionsv1.FunctionReference{Name: "function-test"},
								},
							},
						},
					},
					Functions: []pkgv1.Function{
						func() pkgv1.Function {
							i := 0
							lis := NewFunctionWithRunFunc(t, func(_ context.Context, request *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
								defer func() { i++ }()
								switch i {
								case 0:
									return &fnv1.RunFunctionResponse{
										Requirements: &fnv1.Requirements{
											ExtraResources: map[string]*fnv1.ResourceSelector{
												"extra-resource-by-name": {
													ApiVersion: "test.crossplane.io/v1",
													Kind:       "Foo",
													Match: &fnv1.ResourceSelector_MatchName{
														MatchName: "extra-resource",
													},
												},
											},
										},
									}, nil
								case 1:
									if len(request.GetExtraResources()) == 0 {
										t.Fatalf("expected extra resources to be passed to function on second call")
									}
									res := request.GetExtraResources()["extra-resource-by-name"]
									if res == nil || len(res.GetItems()) == 0 {
										t.Fatalf("expected extra resource to be passed to function on second call")
									}
									foo := (res.GetItems()[0].GetResource().AsMap()["spec"].(map[string]interface{}))["foo"].(string)
									return &fnv1.RunFunctionResponse{
										Requirements: &fnv1.Requirements{
											ExtraResources: map[string]*fnv1.ResourceSelector{
												"extra-resource-by-name": {
													ApiVersion: "test.crossplane.io/v1",
													Kind:       "Foo",
													Match: &fnv1.ResourceSelector_MatchName{
														MatchName: "extra-resource",
													},
												},
											},
										},
										Desired: &fnv1.State{
											Composite: &fnv1.Resource{
												Resource: MustStructJSON(`{
											"status": {
												"widgets": "` + foo + `"
											}
										}`),
											},
											Resources: map[string]*fnv1.Resource{
												"b-cool-resource": {
													Resource: MustStructJSON(`{
												"apiVersion": "atest.crossplane.io/v1",
												"kind": "AComposed",
												"spec": {
													"widgets": 9003
												}
											}`),
												},
												"a-cool-resource": {
													Resource: MustStructJSON(`{
												"apiVersion": "btest.crossplane.io/v1",
												"kind": "BComposed",
												"spec": {
													"widgets": 9002
												}
											}`),
												},
											},
										},
									}, nil
								default:
									t.Fatalf("expected function to be called only twice")
									return nil, nil
								}
							})
							listeners = append(listeners, lis)

							return pkgv1.Function{
								ObjectMeta: metav1.ObjectMeta{
									Name: "function-test",
									Annotations: map[string]string{
										AnnotationKeyRuntime:                  string(AnnotationValueRuntimeDevelopment),
										AnnotationKeyRuntimeDevelopmentTarget: lis.Addr().String(),
									},
								},
							}
						}(),
					},
					ExtraResources: []unstructured.Unstructured{
						{
							Object: MustLoadJSON(`{
								"apiVersion": "test.crossplane.io/v1",
								"kind": "Foo",
								"metadata": {
									"name": "extra-resource"
								},
								"spec": {
									"foo": "bar"
								}
							}`),
						},
					},
					Context: map[string][]byte{
						"crossplane.io/context-key": []byte(`{}`),
					},
				},
			},
			want: want{
				out: Outputs{
					CompositeResource: &ucomposite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								},
								"status": {
									"widgets": "bar",
									"conditions": [{
										"lastTransitionTime": "2024-01-01T00:00:00Z",
										"type": "Ready",
										"status": "False",
										"reason": "Creating",
										"message": "Unready resources: a-cool-resource, b-cool-resource"
									}]
								}
							}`),
						},
					},
					ComposedResources: []composed.Unstructured{
						{
							Unstructured: unstructured.Unstructured{
								Object: MustLoadJSON(`{
									"apiVersion": "btest.crossplane.io/v1",
									"metadata": {
										"generateName": "test-render-",
										"labels": {
											"crossplane.io/composite": "test-render"
										},
										"annotations": {
											"crossplane.io/composition-resource-name": "a-cool-resource"
										},
										"ownerReferences": [{
											"apiVersion": "nop.example.org/v1alpha1",
											"kind": "XNopResource",
											"name": "test-render",
											"blockOwnerDeletion": true,
											"controller": true,
											"uid": ""
										}]
									},
									"kind": "BComposed",
									"spec": {
										"widgets": 9002
									}
								}`),
							},
						},
						{
							Unstructured: unstructured.Unstructured{
								Object: MustLoadJSON(`{
									"apiVersion": "atest.crossplane.io/v1",
									"metadata": {
										"generateName": "test-render-",
										"labels": {
											"crossplane.io/composite": "test-render"
										},
										"annotations": {
											"crossplane.io/composition-resource-name": "b-cool-resource"
										},
										"ownerReferences": [{
											"apiVersion": "nop.example.org/v1alpha1",
											"kind": "XNopResource",
											"name": "test-render",
											"blockOwnerDeletion": true,
											"controller": true,
											"uid": ""
										}]
									},
									"kind": "AComposed",
									"spec": {
										"widgets": 9003
									}
								}`),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := Render(tc.args.ctx, logging.NewNopLogger(), tc.args.in)

			if diff := cmp.Diff(tc.want.out, out, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nRender(...): -want error, +got error:\n%s", tc.reason, diff)
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

func TestFilterExtraResources(t *testing.T) {
	type params struct {
		ers []unstructured.Unstructured
	}
	type args struct {
		ctx      context.Context
		selector *fnv1.ResourceSelector
	}
	type want struct {
		out *fnv1.Resources
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"NilResources": {
			reason: "Should return empty slice if no extra resources are passed",
			params: params{
				ers: []unstructured.Unstructured{},
			},
			args: args{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "extra-resource",
					},
				},
			},
			want: want{
				out: nil,
				err: nil,
			},
		},
		"NilSelector": {
			reason: "Should return empty slice if no selector is passed",
			params: params{
				ers: []unstructured.Unstructured{
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1",
							"kind": "Foo",
							"metadata": {
								"name": "extra-resource"
							}
						}`),
					},
				},
			},
			args: args{
				selector: nil,
			},
			want: want{
				out: nil,
				err: nil,
			},
		},
		"MatchName": {
			reason: "Should return slice with matching resource for name selector",
			params: params{
				ers: []unstructured.Unstructured{
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1",
							"kind": "Foo",
							"metadata": {
								"name": "extra-resource-wrong-kind"
							}
						}`),
					},
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1beta1",
							"kind": "Bar",
							"metadata": {
								"name": "extra-resource-wrong-apiVersion"
							}
						}`),
					},
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1",
							"kind": "Bar",
							"metadata": {
								"name": "extra-resource-right"
							}
						}`),
					},
				},
			},
			args: args{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Bar",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "extra-resource-right",
					},
				},
			},
			want: want{
				out: &fnv1.Resources{
					Items: []*fnv1.Resource{
						{
							Resource: MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1",
								"kind": "Bar",
								"metadata": {
									"name": "extra-resource-right"
								}
							}`),
						},
					},
				},
				err: nil,
			},
		},
		"MatchLabels": {
			reason: "Should return slice with matching resources for matching selector",
			params: params{
				ers: []unstructured.Unstructured{
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1",
							"kind": "Foo",
							"metadata": {
								"name": "extra-resource-wrong-kind"
							}
						}`),
					},
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1beta1",
							"kind": "Bar",
							"metadata": {
								"name": "extra-resource-wrong-apiVersion",
								"labels": {
									"right": "false"
								}
							}
						}`),
					},
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1",
							"kind": "Bar",
							"metadata": {
								"name": "extra-resource-right-1",
								"labels": {
									"right": "true"
								}
							}
						}`),
					},
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1",
							"kind": "Bar",
							"metadata": {
								"name": "extra-resource-right-2",
								"labels": {
									"right": "true"
								}
							}
						}`),
					},
					{
						Object: MustLoadJSON(`{
							"apiVersion": "test.crossplane.io/v1",
							"kind": "Bar",
							"metadata": {
								"name": "extra-resource-wrong-label value",
								"labels": {
									"right": "false"
								}
							}
						}`),
					},
				},
			},
			args: args{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Bar",
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{
								"right": "true",
							},
						},
					},
				},
			},
			want: want{
				out: &fnv1.Resources{
					Items: []*fnv1.Resource{
						{
							Resource: MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1",
								"kind": "Bar",
								"metadata": {
									"name": "extra-resource-right-1",
									"labels": {
										"right": "true"
									}
								}
							}`),
						},
						{
							Resource: MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1",
								"kind": "Bar",
								"metadata": {
									"name": "extra-resource-right-2",
									"labels": {
										"right": "true"
									}
								}
							}`),
						},
					},
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &FilteringFetcher{extra: tc.params.ers}
			out, err := f.Fetch(tc.args.ctx, tc.args.selector)
			if diff := cmp.Diff(tc.want.out, out, cmpopts.EquateEmpty(), cmpopts.IgnoreUnexported(fnv1.Resources{}, fnv1.Resource{}, structpb.Struct{}, structpb.Value{})); diff != "" {
				t.Errorf("%s\nfilterExtraResources(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nfilterExtraResources(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetSecret(t *testing.T) {
	secrets := []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: "namespace1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: "namespace2",
			},
		},
	}

	tests := map[string]struct {
		name      string
		namespace string
		secrets   []corev1.Secret
		wantErr   bool
	}{
		"SecretFound": {
			name:      "secret1",
			namespace: "namespace1",
			secrets:   secrets,
			wantErr:   false,
		},
		"SecretNotFound": {
			name:      "secret3",
			namespace: "namespace3",
			secrets:   secrets,
			wantErr:   true,
		},
		"SecretWrongNamespace": {
			name:      "secret1",
			namespace: "namespace2",
			secrets:   secrets,
			wantErr:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := getSecret(tc.name, tc.namespace, tc.secrets)
			if (err != nil) != tc.wantErr {
				t.Errorf("getSecret() error = %v, wantErr %v", err, tc.wantErr)
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
