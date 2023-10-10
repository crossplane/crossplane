package main

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
		in  RenderInputs
	}
	type want struct {
		out RenderOutputs
		err error
	}

	cases := map[string]struct {
		reason string
		rsp    *fnv1beta1.RunFunctionResponse
		args   args
		want   want
	}{
		"UnknownRuntime": {
			args: args{
				in: RenderInputs{
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
				in: RenderInputs{
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
				in: RenderInputs{
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
				in: RenderInputs{
					CompositeResource: &composite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-xrender"
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
										"cool-resource": {
											Resource: MustStructJSON(`{
												"apiVersion": "test.crossplane.io/v1",
												"kind": "Composed",
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
				},
			},
			want: want{
				out: RenderOutputs{
					CompositeResource: &composite.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: MustLoadJSON(`{
								"apiVersion": "nop.example.org/v1alpha1",
								"kind": "XNopResource",
								"metadata": {
									"name": "test-xrender"
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
									"apiVersion": "test.crossplane.io/v1",
									"metadata": {
										"generateName": "test-xrender-",
										"labels": {
											"crossplane.io/composite": "test-xrender"
										},
										"annotations": {
											"crossplane.io/composition-resource-name": "cool-resource"
										},
										"ownerReferences": [{
											"apiVersion": "nop.example.org/v1alpha1",
											"kind": "XNopResource",
											"name": "test-xrender",
											"blockOwnerDeletion": true,
											"controller": true,
											"uid": ""
										}]
									},
									"kind": "Composed",
									"spec": {
										"widgets": 9002
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

type MockFunctionRunner struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	Response *fnv1beta1.RunFunctionResponse
	Error    error
}

func (r *MockFunctionRunner) RunFunction(context.Context, *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	return r.Response, r.Error
}

func MustStructJSON(j string) *structpb.Struct {
	s := &structpb.Struct{}
	if err := protojson.Unmarshal([]byte(j), s); err != nil {
		panic(err)
	}
	return s
}
