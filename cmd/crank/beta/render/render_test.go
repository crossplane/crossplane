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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	fnv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
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
		rsp    *fnv1beta1.RunFunctionResponse
		args   args
		want   want
	}{
		"InvalidContextValue": {
			args: args{
				in: Inputs{
					CompositeResource: composite.New(),
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
					CompositeResource: composite.New(),
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
					Functions: []pkgv1beta1.Function{{
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
					CompositeResource: composite.New(),
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
					CompositeResource: composite.New(),
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
					Functions: []pkgv1beta1.Function{
						func() pkgv1beta1.Function {

							lis := NewFunction(t, &fnv1beta1.RunFunctionResponse{
								Results: []*fnv1beta1.Result{
									{
										Severity: fnv1beta1.Severity_SEVERITY_FATAL,
									},
								},
							})
							listeners = append(listeners, lis)

							return pkgv1beta1.Function{
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
					CompositeResource: &composite.Unstructured{
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
					Functions: []pkgv1beta1.Function{
						func() pkgv1beta1.Function {

							lis := NewFunction(t, &fnv1beta1.RunFunctionResponse{
								Desired: &fnv1beta1.State{
									Composite: &fnv1beta1.Resource{
										Resource: MustStructJSON(`{
											"status": {
												"widgets": 9001
											}
										}`),
									},
									Resources: map[string]*fnv1beta1.Resource{
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
							})
							listeners = append(listeners, lis)

							return pkgv1beta1.Function{
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
					CompositeResource: &composite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								},
								"status": {
									"widgets": 9001
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
					CompositeResource: &composite.Unstructured{
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
					Functions: []pkgv1beta1.Function{
						func() pkgv1beta1.Function {
							i := 0
							lis := NewFunctionWithRunFunc(t, func(ctx context.Context, request *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
								defer func() { i++ }()
								switch i {
								case 0:
									return &fnv1beta1.RunFunctionResponse{
										Requirements: &fnv1beta1.Requirements{
											ExtraResources: map[string]*fnv1beta1.ResourceSelector{
												"extra-resource-by-name": {
													ApiVersion: "test.crossplane.io/v1",
													Kind:       "Foo",
													Match: &fnv1beta1.ResourceSelector_MatchName{
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
									return &fnv1beta1.RunFunctionResponse{
										Requirements: &fnv1beta1.Requirements{
											ExtraResources: map[string]*fnv1beta1.ResourceSelector{
												"extra-resource-by-name": {
													ApiVersion: "test.crossplane.io/v1",
													Kind:       "Foo",
													Match: &fnv1beta1.ResourceSelector_MatchName{
														MatchName: "extra-resource",
													},
												},
											},
										},
										Desired: &fnv1beta1.State{
											Composite: &fnv1beta1.Resource{
												Resource: MustStructJSON(`{
											"status": {
												"widgets": "` + foo + `"
											}
										}`),
											},
											Resources: map[string]*fnv1beta1.Resource{
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

							return pkgv1beta1.Function{
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
					CompositeResource: &composite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-render"
								},
								"status": {
									"widgets": "bar"
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

			out, err := Render(tc.args.ctx, tc.args.in)

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

func NewFunction(t *testing.T, rsp *fnv1beta1.RunFunctionResponse) net.Listener {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	fnv1beta1.RegisterFunctionRunnerServiceServer(srv, &MockFunctionRunner{Response: rsp})
	go srv.Serve(lis) // This will stop when lis is closed.

	return lis
}

func NewFunctionWithRunFunc(t *testing.T, runFunc func(context.Context, *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error)) net.Listener {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	fnv1beta1.RegisterFunctionRunnerServiceServer(srv, &MockFunctionRunner{RunFunc: runFunc})
	go srv.Serve(lis) // This will stop when lis is closed.

	return lis
}

type MockFunctionRunner struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	Response *fnv1beta1.RunFunctionResponse
	RunFunc  func(context.Context, *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error)
	Error    error
}

func (r *MockFunctionRunner) RunFunction(ctx context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	if r.Response != nil {
		return r.Response, r.Error
	}
	return r.RunFunc(ctx, req)
}

func TestFilterExtraResources(t *testing.T) {
	type args struct {
		ers      []unstructured.Unstructured
		selector *fnv1beta1.ResourceSelector
	}
	type want struct {
		out *fnv1beta1.Resources
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilResources": {
			reason: "Should return empty slice if no extra resources are passed",
			args: args{
				ers: []unstructured.Unstructured{},
				selector: &fnv1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					Match: &fnv1beta1.ResourceSelector_MatchName{
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
			args: args{
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
				selector: nil,
			},
			want: want{
				out: nil,
				err: nil,
			},
		},
		"MatchName": {
			reason: "Should return slice with matching resource for name selector",
			args: args{
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
				selector: &fnv1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Bar",
					Match: &fnv1beta1.ResourceSelector_MatchName{
						MatchName: "extra-resource-right",
					},
				},
			},
			want: want{
				out: &fnv1beta1.Resources{
					Items: []*fnv1beta1.Resource{
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
			args: args{
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
				selector: &fnv1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Bar",
					Match: &fnv1beta1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1beta1.MatchLabels{
							Labels: map[string]string{
								"right": "true",
							},
						},
					},
				},
			},
			want: want{
				out: &fnv1beta1.Resources{
					Items: []*fnv1beta1.Resource{
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
			out, err := filterExtraResources(tc.args.ers, tc.args.selector)
			if diff := cmp.Diff(tc.want.out, out, cmpopts.EquateEmpty(), cmpopts.IgnoreUnexported(fnv1beta1.Resources{}, fnv1beta1.Resource{}, structpb.Struct{}, structpb.Value{})); diff != "" {
				t.Errorf("%s\nfilterExtraResources(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nfilterExtraResources(...): -want error, +got error:\n%s", tc.reason, diff)
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
