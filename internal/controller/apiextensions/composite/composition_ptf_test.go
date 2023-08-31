/*
Copyright 2022 The Crossplane Authors.

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

package composite

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	fnv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestPTFCompose(t *testing.T) {
	errBoom := errors.New("boom")

	var cd composed.Unstructured
	errMissingKind := json.Unmarshal([]byte("{}"), &cd)
	errProtoSyntax := protojson.Unmarshal([]byte("hi"), &structpb.Struct{})

	type params struct {
		kube client.Client
		o    []PTFComposerOption
	}
	type args struct {
		ctx context.Context
		xr  resource.Composite
		req CompositionRequest
	}
	type want struct {
		res CompositionResult
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"ComposedTemplatesError": {
			reason: "We should return any error encountered while inlining a composition's patchsets.",
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{{
								Patches: []v1.Patch{{
									// This reference to a non-existent patchset
									// triggers the error.
									Type:         v1.PatchTypePatchSet,
									PatchSetName: pointer.String("nonexistent-patchset"),
								}},
							}},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtUndefinedPatchSet, "nonexistent-patchset"), errInline),
			},
		},
		"FetchConnectionError": {
			reason: "We should return any error encountered while fetching the XR's connection details.",
			params: params{
				o: []PTFComposerOption{
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return ComposedResourceStates{}, nil
					})),
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, errBoom
					})),
				},
			},
			args: args{
				req: CompositionRequest{Revision: &v1.CompositionRevision{}},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchXRConnectionDetails),
			},
		},
		"GetComposedResourcesError": {
			reason: "We should return any error encountered while getting the XR's existing composed resources.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, errBoom
					})),
				},
			},
			args: args{
				req: CompositionRequest{Revision: &v1.CompositionRevision{}},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetExistingCDs),
			},
		},
		"ParseComposedResourceBaseError": {
			reason: "We should return any error encountered while parsing a composed resource base template",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								{
									Name: pointer.String("uncool-resource"),
									Base: runtime.RawExtension{Raw: []byte("{}")}, // An invalid, empty base resource template.
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errMissingKind, errUnmarshalJSON), errFmtParseBase, "uncool-resource"),
			},
		},
		"UnmarshalFunctionInputError": {
			reason: "We should return any error encountered while unmarshalling a Composition Function input",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
									Input:       &runtime.RawExtension{Raw: []byte("hi")}, // This is invalid - it must be a JSON object.
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errProtoSyntax, errFmtUnmarshalPipelineStepInput, "run-cool-function"),
			},
		},
		"RunFunctionError": {
			reason: "We should return any error encountered while running a Composition Function",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						return nil, errBoom
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtRunPipelineStep, "run-cool-function"),
			},
		},
		"FatalFunctionResultError": {
			reason: "We should return any fatal function results as an error",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						r := &fnv1beta1.Result{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
							Message:  "oh no",
						}
						return &fnv1beta1.RunFunctionResponse{Results: []*fnv1beta1.Result{r}}, nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.New("oh no"), errFatalResult),
			},
		},
		"RenderXRFromStructError": {
			reason: "We should return any error we encounter when rendering our XR from the struct returned in the FunFunctionResponse",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						d := &fnv1beta1.State{
							Composite: &fnv1beta1.Resource{
								Resource: &structpb.Struct{}, // Missing APIVersion and Kind.
							},
						}
						return &fnv1beta1.RunFunctionResponse{Desired: d}, nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errMissingKind, errUnmarshalJSON), errUnmarshalDesiredXR),
			},
		},
		"RenderComposedResourceFromStructError": {
			reason: "We should return any error we encounter when rendering a composed resource from the struct returned in the FunFunctionResponse",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						d := &fnv1beta1.State{
							Composite: &fnv1beta1.Resource{
								Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
									"kind":       structpb.NewStringValue("CoolComposite"),
								}},
							},
							Resources: map[string]*fnv1beta1.Resource{
								"cool-resource": {
									// Missing APIVersion and Kind.
									Resource: &structpb.Struct{},
								},
							},
						}
						return &fnv1beta1.RunFunctionResponse{Desired: d}, nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errMissingKind, errUnmarshalJSON), errFmtUnmarshalDesiredCD, "cool-resource"),
			},
		},
		"RenderComposedResourceMetadataError": {
			reason: "We should return any error we encounter when rendering composed resource metadata",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						d := &fnv1beta1.State{
							Composite: &fnv1beta1.Resource{
								Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
									"kind":       structpb.NewStringValue("CoolComposite"),

									// Missing labels required by RenderComposedResourceMetadata.
								}},
							},
							Resources: map[string]*fnv1beta1.Resource{
								"cool-resource": {
									Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
										"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
										"kind":       structpb.NewStringValue("CoolComposite"),
									}},
								},
							},
						}
						return &fnv1beta1.RunFunctionResponse{Desired: d}, nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(RenderComposedResourceMetadata(nil, &fake.Composite{}, ""), errFmtRenderMetadata, "cool-resource"),
			},
		},
		"DryRunRenderComposedResourceError": {
			reason: "We should return any error we encounter when dry-run rendering a composed resource",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						d := &fnv1beta1.State{
							Composite: &fnv1beta1.Resource{
								Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
									"kind":       structpb.NewStringValue("CoolComposite"),
									"metadata": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
										"labels": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
											xcrd.LabelKeyNamePrefixForComposed: structpb.NewStringValue("parent-xr"),
										}}),
									}}),
								}},
							},
							Resources: map[string]*fnv1beta1.Resource{
								"cool-resource": {
									Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
										"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
										"kind":       structpb.NewStringValue("CoolComposite"),
									}},
								},
							},
						}
						return &fnv1beta1.RunFunctionResponse{Desired: d}, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return errBoom })),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtDryRunApply, "cool-resource"),
			},
		},
		"GarbageCollectComposedResourcesError": {
			reason: "We should return any error we encounter when garbage collecting composed resources",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						d := &fnv1beta1.State{
							Composite: &fnv1beta1.Resource{
								Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
									"kind":       structpb.NewStringValue("CoolComposite"),
									"metadata": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
										"labels": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
											xcrd.LabelKeyNamePrefixForComposed: structpb.NewStringValue("parent-xr"),
										}}),
									}}),
								}},
							},
							Resources: map[string]*fnv1beta1.Resource{
								"cool-resource": {
									Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
										"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
										"kind":       structpb.NewStringValue("CoolComposed"),
									}},
								},
							},
						}
						return &fnv1beta1.RunFunctionResponse{Desired: d}, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return nil })),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return errBoom
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGarbageCollectCDs),
			},
		},
		"ApplyXRError": {
			reason: "We should return any error we encounter when applying the composite resource",
			params: params{
				kube: &test.MockClient{
					// Apply calls Get.
					MockGet: test.NewMockGetFn(errBoom),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						d := &fnv1beta1.State{
							Composite: &fnv1beta1.Resource{
								Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
									"kind":       structpb.NewStringValue("CoolComposite"),
								}},
							},
						}
						return &fnv1beta1.RunFunctionResponse{Desired: d}, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return nil })),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyXR),
			},
		},
		"ApplyComposedResourceError": {
			reason: "We should return any error we encounter when applying a composed resource",
			params: params{
				kube: &test.MockClient{
					// Apply calls Get and Patch for the XR.
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),

					// Apply calls Create (immediately) for the composed
					// resource because it has a GenerateName set.
					MockCreate: test.NewMockCreateFn(errBoom),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (rsp *fnv1beta1.RunFunctionResponse, err error) {
						d := &fnv1beta1.State{
							Composite: &fnv1beta1.Resource{
								Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
									"kind":       structpb.NewStringValue("CoolComposite"),
									"metadata": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
										"labels": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
											xcrd.LabelKeyNamePrefixForComposed: structpb.NewStringValue("parent-xr"),
										}}),
									}}),
								}},
							},
							Resources: map[string]*fnv1beta1.Resource{
								"uncool-resource": {
									Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
										"apiVersion": structpb.NewStringValue("test.crossplane.io/v1"),
										"kind":       structpb.NewStringValue("UncoolComposed"),
									}},
								},
							},
						}
						return &fnv1beta1.RunFunctionResponse{Desired: d}, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return nil })),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errBoom, "cannot create object"), errFmtApplyCD, "uncool-resource"),
			},
		},
		"ExtractConnectionDetailsError": {
			reason: "We should return any error we encounter when extracting XR connection details from a composed resource",
			params: params{
				kube: &test.MockClient{
					// Apply calls Get and Patch for the XR.
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),

					// Apply calls Create (immediately) for the composed
					// resource because it has a GenerateName set.
					MockCreate: test.NewMockCreateFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						// We only try to extract connection details for
						// observed resources.
						return ComposedResourceStates{"uncool-resource": ComposedResourceState{Resource: &fake.Composed{}}}, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return nil })),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
					WithConnectionDetailsExtractor(ConnectionDetailsExtractorFn(func(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error) {
						return nil, errBoom
					})),
				},
			},
			args: args{
				xr: func() resource.Composite {
					// Our XR needs a GVK to survive round-tripping through a
					// protobuf struct (which involves using the Kubernetes-aware
					// JSON unmarshaller that requires a GVK).
					xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "test.crossplane.io",
						Version: "v1",
						Kind:    "CoolComposite",
					}))
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "parent-xr",
					})
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								{
									Name: pointer.String("uncool-resource"),

									// Our composed resources need a GVK too -
									// same reason as the XR above.
									Base: runtime.RawExtension{Raw: []byte(`{"apiversion":"test.crossplane.io/v1","kind":"UncoolComposed"}`)},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtExtractConnectionDetails, "uncool-resource", "", ""),
			},
		},
		"CheckComposedResourceReadinessError": {
			reason: "We should return any error we encounter when checking the readiness of a composed resource",
			params: params{
				kube: &test.MockClient{
					// Apply calls Get and Patch for the XR.
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),

					// Apply calls Create (immediately) for the composed
					// resource because it has a GenerateName set.
					MockCreate: test.NewMockCreateFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						// We only try to extract connection details for
						// observed resources.
						return ComposedResourceStates{"uncool-resource": ComposedResourceState{Resource: &fake.Composed{}}}, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return nil })),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
					WithConnectionDetailsExtractor(ConnectionDetailsExtractorFn(func(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, o ConditionedObject, rc ...ReadinessCheck) (ready bool, err error) {
						return false, errBoom
					})),
				},
			},
			args: args{
				xr: func() resource.Composite {
					// Our XR needs a GVK to survive round-tripping through a
					// protobuf struct (which involves using the Kubernetes-aware
					// JSON unmarshaller that requires a GVK).
					xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "test.crossplane.io",
						Version: "v1",
						Kind:    "CoolComposite",
					}))
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "parent-xr",
					})
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								{
									Name: pointer.String("uncool-resource"),

									// Our composed resources need a GVK too -
									// same reason as the XR above.
									Base: runtime.RawExtension{Raw: []byte(`{"apiversion":"test.crossplane.io/v1","kind":"UncoolComposed"}`)},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtReadiness, "uncool-resource", "", ""),
			},
		},
		"SuccessfulPatchAndTransformOnly": {
			reason: "We should return a valid CompositionResult when a 'pure patch and transform' (i.e. Function-less) reconcile succeeds",
			params: params{
				kube: &test.MockClient{
					// Apply calls Get and Patch for the XR.
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),

					// Apply calls Create (immediately) for the composed
					// resource because it has a GenerateName set.
					MockCreate: test.NewMockCreateFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						// We only try to extract connection details for
						// observed resources.
						r := ComposedResourceStates{
							"observed-resource-a": ComposedResourceState{
								Resource: &fake.Composed{
									ObjectMeta: metav1.ObjectMeta{Name: "observed-resource-a"},
								},
							},
							"observed-resource-b": ComposedResourceState{
								Resource: &fake.Composed{
									ObjectMeta: metav1.ObjectMeta{Name: "observed-resource-b"},
								},
							},
						}
						return r, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return nil })),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
					WithConnectionDetailsExtractor(ConnectionDetailsExtractorFn(func(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error) {
						return managed.ConnectionDetails{cd.GetName(): []byte("secret")}, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, o ConditionedObject, rc ...ReadinessCheck) (ready bool, err error) {
						return true, nil
					})),
				},
			},
			args: args{
				xr: func() resource.Composite {
					// Our XR needs a GVK to survive round-tripping through a
					// protobuf struct (which involves using the Kubernetes-aware
					// JSON unmarshaller that requires a GVK).
					xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "test.crossplane.io",
						Version: "v1",
						Kind:    "CoolComposite",
					}))
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "parent-xr",
					})
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								// Our composed resources need a GVK too -
								// same reason as the XR above.
								{
									Name: pointer.String("observed-resource-a"),
									Base: runtime.RawExtension{Raw: []byte(`{"apiversion":"test.crossplane.io/v1","kind":"CoolComposed"}`)},
								},
								{
									Name: pointer.String("observed-resource-b"),
									Base: runtime.RawExtension{Raw: []byte(`{"apiversion":"test.crossplane.io/v1","kind":"CoolComposed"}`)},
								},
								{
									// We have a template for this resource, but
									// we didn't observe it.
									Name: pointer.String("desired-resource"),
									Base: runtime.RawExtension{Raw: []byte(`{"apiversion":"test.crossplane.io/v1","kind":"CoolComposed"}`)},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{
						{ResourceName: "observed-resource-a", Ready: true},
						{ResourceName: "observed-resource-b", Ready: true},
						{ResourceName: "desired-resource"},
					},
					ConnectionDetails: managed.ConnectionDetails{
						"observed-resource-a": []byte("secret"),
						"observed-resource-b": []byte("secret"),
					},
				},
				err: nil,
			},
		},
		"SuccessfulFunctionsOnly": {
			reason: "We should return a valid CompositionResult when a 'pure Function' (i.e. patch-and-transform-less) reconcile succeeds",
			params: params{
				kube: &test.MockClient{
					// Apply calls Get and Patch for the XR.
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),

					// Apply calls Create (immediately) for the composed
					// resource because it has a GenerateName set.
					MockCreate: test.NewMockCreateFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						// We only try to extract connection details for
						// observed resources.
						r := ComposedResourceStates{
							"observed-resource-a": ComposedResourceState{
								Resource: &fake.Composed{
									ObjectMeta: metav1.ObjectMeta{Name: "observed-resource-a"},
								},
							},
						}
						return r, nil
					})),
					WithDryRunRenderer(DryRunRendererFn(func(ctx context.Context, cd resource.Object) error { return nil })),
					WithFunctionRunner(FunctionRunnerFn(func(ctx context.Context, name string, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
						rsp := &fnv1beta1.RunFunctionResponse{
							Desired: &fnv1beta1.State{
								Composite: &fnv1beta1.Resource{
									Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
										"apiVersion": structpb.NewStringValue("example.org/v1"),
										"kind":       structpb.NewStringValue("Composite"),
										"metadata": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
											"name": structpb.NewStringValue("cool-xr"),
											"labels": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
												xcrd.LabelKeyNamePrefixForComposed: structpb.NewStringValue("cool-xr"),
											}}),
										}}),
									}},
									ConnectionDetails: map[string][]byte{"from": []byte("function-pipeline")},
								},
								Resources: map[string]*fnv1beta1.Resource{
									"observed-resource-a": {
										Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
											"apiVersion": structpb.NewStringValue("example.org/v2"),
											"kind":       structpb.NewStringValue("Composed"),
										}},
									},
									"desired-resource-a": {
										Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
											"apiVersion": structpb.NewStringValue("example.org/v2"),
											"kind":       structpb.NewStringValue("Composed"),
										}},
									},
								},
							},
							Results: []*fnv1beta1.Result{
								{
									Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
									Message:  "A normal result",
								},
								{
									Severity: fnv1beta1.Severity_SEVERITY_WARNING,
									Message:  "A warning result",
								},
								{
									Severity: fnv1beta1.Severity_SEVERITY_UNSPECIFIED,
									Message:  "A result of unspecified severity",
								},
							},
						}
						return rsp, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: func() resource.Composite {
					// Our XR needs a GVK to survive round-tripping through a
					// protobuf struct (which involves using the Kubernetes-aware
					// JSON unmarshaller that requires a GVK).
					xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "test.crossplane.io",
						Version: "v1",
						Kind:    "CoolComposite",
					}))
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "parent-xr",
					})
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{
						{ResourceName: "desired-resource-a"},
						{ResourceName: "observed-resource-a", Ready: true},
					},
					ConnectionDetails: managed.ConnectionDetails{
						"from": []byte("function-pipeline"),
					},
					Events: []event.Event{
						{
							Type:    "Normal",
							Reason:  "ComposeResources",
							Message: "A normal result",
						},
						{
							Type:    "Warning",
							Reason:  "ComposeResources",
							Message: "A warning result",
						},
						{
							Type:    "Warning",
							Reason:  "ComposeResources",
							Message: "Composition Function pipeline returned a result of unknown severity (assuming warning): A result of unspecified severity",
						},
					},
				},
				err: nil,
			},
		},
		// TODO(negz): Test interactions between P&T and Functions? They're not
		// super interesting/different from the above tests.
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := NewPTFComposer(tc.params.kube, tc.params.o...)
			res, err := c.Compose(tc.args.ctx, tc.args.xr, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}

			// We iterate over a map to produce ComposedResources, so they're
			// returned in random order.
			if diff := cmp.Diff(tc.want.res, res, cmpopts.EquateEmpty(), cmpopts.SortSlices(func(i, j ComposedResource) bool { return i.ResourceName < j.ResourceName })); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetComposedResources(t *testing.T) {
	errBoom := errors.New("boom")
	details := managed.ConnectionDetails{"a": []byte("b")}

	type params struct {
		c client.Reader
		f managed.ConnectionDetailsFetcher
	}

	type args struct {
		ctx context.Context
		xr  resource.Composite
	}

	type want struct {
		ors ComposedResourceStates
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"UnnamedRef": {
			reason: "We should skip any resource references without names.",
			params: params{
				c: &test.MockClient{
					// We should continue past the unnamed reference and not hit
					// this error.
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{
								APIVersion: "example.org/v1",
								Kind:       "Unnamed",
							},
						},
					},
				},
			},
		},
		"ComposedResourceNotFound": {
			reason: "We should skip any resources that are not found.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
		},
		"GetComposedResourceError": {
			reason: "We should return any error we encounter while getting a composed resource.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetComposed),
			},
		},
		"UncontrolledComposedResource": {
			reason: "We should skip any composed resources our XR doesn't control.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						_ = meta.AddControllerReference(obj, metav1.OwnerReference{
							UID:        types.UID("someone-else"),
							Controller: pointer.Bool(true),
						})

						return nil
					}),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"AnonymousComposedResourceError": {
			reason: "We should return an error if we encounter an (unsupported) anonymous composed resource.",
			params: params{
				c: &test.MockClient{
					// We 'return' an empty resource with no annotations.
					MockGet: test.NewMockGetFn(nil),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
			want: want{
				err: errors.New(errAnonymousCD),
			},
		},
		"FetchConnectionDetailsError": {
			reason: "We should return an error if we can't fetch composed resource connection details.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource-42")
						SetCompositionResourceName(obj, "cool-resource")
						return nil
					}),
				},
				f: ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return nil, errBoom
				}),
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{
								Kind: "Broken",
								Name: "cool-resource-42",
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtFetchCDConnectionDetails, "cool-resource", "Broken", "cool-resource-42"),
			},
		},
		"Success": {
			reason: "We should return any composed resources and their connection details.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource-42")
						SetCompositionResourceName(obj, "cool-resource")
						return nil
					}),
				},
				f: ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return details, nil
				}),
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{
								APIVersion: "example.org/v1",
								Kind:       "Composed",
								Name:       "cool-resource-42",
							},
						},
					},
				},
			},
			want: want{
				ors: ComposedResourceStates{"cool-resource": ComposedResourceState{
					ConnectionDetails: details,
					Resource: func() resource.Composed {
						cd := composed.New()
						cd.SetAPIVersion("example.org/v1")
						cd.SetKind("Composed")
						cd.SetName("cool-resource-42")
						SetCompositionResourceName(cd, "cool-resource")
						return cd
					}(),
				},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			g := NewExistingComposedResourceObserver(tc.params.c, tc.params.f)
			ors, err := g.ObserveComposedResources(tc.args.ctx, tc.args.xr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nObserveComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.ors, ors, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nObserveComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAsState(t *testing.T) {
	type args struct {
		xr resource.Composite
		xc managed.ConnectionDetails
		rs ComposedResourceStates
	}
	type want struct {
		d   *fnv1beta1.State
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "We should successfully build RunFunctionRequest State.",
			args: args{
				xr: composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "Composite",
				})),
				xc: managed.ConnectionDetails{"a": []byte("b")},
				rs: ComposedResourceStates{
					"cool-resource": ComposedResourceState{
						Resource: composed.New(composed.FromReference(corev1.ObjectReference{
							APIVersion: "example.org/v2",
							Kind:       "Composed",
							Name:       "cool-resource-42",
						})),
					},
				},
			},
			want: want{
				d: &fnv1beta1.State{
					Composite: &fnv1beta1.Resource{
						Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
							"apiVersion": structpb.NewStringValue("example.org/v1"),
							"kind":       structpb.NewStringValue("Composite"),
						}},
						ConnectionDetails: map[string][]byte{"a": []byte("b")},
					},
					Resources: map[string]*fnv1beta1.Resource{
						"cool-resource": {
							Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
								"apiVersion": structpb.NewStringValue("example.org/v2"),
								"kind":       structpb.NewStringValue("Composed"),
								"metadata": structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
									"name": structpb.NewStringValue("cool-resource-42"),
								}}),
							}},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := AsState(tc.args.xr, tc.args.xc, tc.args.rs)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nState(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.d, d, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nState(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunFunction(t *testing.T) {
	errBoom := errors.New("boom")

	// An error indicating that no security credentials were passed.
	_, errTransportSecurity := grpc.DialContext(context.Background(), "fake.test.crossplane.io")

	// Make sure to add listeners here.
	listeners := make([]net.Listener, 0)

	type params struct {
		c  client.Client
		tc credentials.TransportCredentials
	}
	type args struct {
		ctx  context.Context
		name string
		req  *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1beta1.RunFunctionResponse
		err error
	}
	cases := map[string]struct {
		reason string
		server fnv1beta1.FunctionRunnerServiceServer
		params params
		args   args
		want   want
	}{
		"GetFunctionError": {
			reason: "We should return any error encountered trying to get the named Function package",
			params: params{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return errBoom
					},
				},
			},
			args: args{
				name: "cool-fn",
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtGetFunction, "cool-fn"),
			},
		},
		"MissingEndpointError": {
			reason: "We should return an error if the named Function has an empty endpoint",
			params: params{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return nil // Return an empty Function
					},
				},
			},
			args: args{
				name: "cool-fn",
			},
			want: want{
				err: errors.Errorf(errFmtEmptyFunctionStatus, "cool-fn"),
			},
		},
		"DialFunctionError": {
			reason: "We should return an error if dialing the named Function returns an error",
			params: params{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						obj.(*pkgv1beta1.Function).Status.Endpoint = "fake.test.crossplane.io"
						return nil
					},
				},
				tc: nil, // Supplying no credentials triggers a dial error.
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
				req:  &fnv1beta1.RunFunctionRequest{},
			},
			want: want{
				err: errors.Wrapf(errTransportSecurity, errFmtDialFunction, "cool-fn"),
			},
		},
		"RunFunctionError": {
			reason: "We should return an error if running the named Function returns an error",
			params: params{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						lis := NewGRPCServer(t, &MockFunctionServer{err: errBoom})
						listeners = append(listeners, lis)

						obj.(*pkgv1beta1.Function).Status.Endpoint = lis.Addr().String()
						return nil
					},
				},
				tc: insecure.NewCredentials(),
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
				req:  &fnv1beta1.RunFunctionRequest{},
			},
			want: want{
				err: errors.Wrapf(status.Errorf(codes.Unknown, errBoom.Error()), errFmtRunFunction, "cool-fn"),
			},
		},
		"RunFunctionSuccess": {
			reason: "We should return the RunFunctionResponse from the Function",
			params: params{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						lis := NewGRPCServer(t, &MockFunctionServer{
							rsp: &fnv1beta1.RunFunctionResponse{
								Results: []*fnv1beta1.Result{
									{
										Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
										Message:  "Successfully ran Function!",
									},
								},
							},
						})
						listeners = append(listeners, lis)

						obj.(*pkgv1beta1.Function).Status.Endpoint = lis.Addr().String()
						return nil
					},
				},
				tc: insecure.NewCredentials(),
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
				req:  &fnv1beta1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
							Message:  "Successfully ran Function!",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewPackagedFunctionRunner(tc.params.c, tc.params.tc)
			rsp, err := r.RunFunction(tc.args.ctx, tc.args.name, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}

	for _, lis := range listeners {
		if err := lis.Close(); err != nil {
			t.Logf("lis.Close: %s", err)
		}
	}
}

func NewGRPCServer(t *testing.T, ss fnv1beta1.FunctionRunnerServiceServer) net.Listener {
	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	// TODO(negz): Is it worth using a WaitGroup for these?
	go func() {
		s := grpc.NewServer()
		fnv1beta1.RegisterFunctionRunnerServiceServer(s, ss)
		_ = s.Serve(lis)
	}()

	// The caller must close this listener to terminate the server.
	return lis
}

type MockFunctionServer struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	rsp *fnv1beta1.RunFunctionResponse
	err error
}

func (s *MockFunctionServer) RunFunction(context.Context, *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	return s.rsp, s.err
}

func TestGarbageCollectComposedResources(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		client client.Writer
	}

	type args struct {
		ctx      context.Context
		owner    metav1.Object
		observed ComposedResourceStates
		desired  ComposedResourceStates
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"UncontrolledResource": {
			reason: "Resources the XR doesn't control should not be deleted.",
			params: params{
				client: &test.MockClient{
					// We know Delete wasn't called because it's a nil function
					// and would thus panic if it was.
				},
			},
			args: args{
				owner: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cool-xr",
					},
				},
				observed: ComposedResourceStates{
					"undesired-resource": ComposedResourceState{Resource: &fake.Composed{}},
				},
			},
			want: want{
				err: nil,
			},
		},
		"DeleteError": {
			reason: "We should return any error encountered deleting the resource.",
			params: params{
				client: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(errBoom),
				},
			},
			args: args{
				owner: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cool-xr",
					},
				},
				observed: ComposedResourceStates{
					"undesired-resource": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								// This resource is controlled by the XR.
								OwnerReferences: []metav1.OwnerReference{{
									Controller: pointer.Bool(true),
									UID:        "cool-xr",
								}},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtDeleteCD, "undesired-resource", "", ""),
			},
		},
		"SuccessfulDelete": {
			reason: "We should successfully delete an observed resource from the API server if it is not desired.",
			params: params{
				client: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				},
			},
			args: args{
				owner: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cool-xr",
					},
				},
				observed: ComposedResourceStates{
					"undesired-resource": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								// This resource is controlled by the XR.
								OwnerReferences: []metav1.OwnerReference{{
									Controller: pointer.Bool(true),
									UID:        "cool-xr",
								}},
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulNoop": {
			reason: "We should not delete an observed resource from the API server if it is desired.",
			params: params{
				client: &test.MockClient{
					// We know Delete wasn't called because it's nil and would
					// panic if it was.
				},
			},
			args: args{
				owner: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						UID: "cool-xr",
					},
				},
				observed: ComposedResourceStates{
					"desired-resource": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								// This resource is controlled by the XR.
								OwnerReferences: []metav1.OwnerReference{{
									Controller: pointer.Bool(true),
									UID:        "cool-xr",
								}},
							},
						},
					},
				},
				desired: ComposedResourceStates{
					// The observed resource above is also desired.
					"desired-resource": ComposedResourceState{},
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			d := NewDeletingComposedResourceGarbageCollector(tc.params.client)
			err := d.GarbageCollectComposedResources(tc.args.ctx, tc.args.owner, tc.args.observed, tc.args.desired)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGarbageCollectComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestUpdateResourceRefs(t *testing.T) {
	type args struct {
		xr  resource.ComposedResourcesReferencer
		drs ComposedResourceStates
	}

	type want struct {
		xr resource.ComposedResourcesReferencer
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "We should return a consistently ordered set of references.",
			args: args{
				xr: &fake.Composite{},
				drs: ComposedResourceStates{
					"never-created-c": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Name: "never-created-c-42",
							},
						},
					},
					"never-created-b": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Name: "never-created-b-42",
							},
						},
					},
					"never-created-a": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Name: "never-created-a-42",
							},
						},
					},
				},
			},
			want: want{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "never-created-a-42"},
							{Name: "never-created-b-42"},
							{Name: "never-created-c-42"},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			UpdateResourceRefs(tc.args.xr, tc.args.drs)

			if diff := cmp.Diff(tc.want.xr, tc.args.xr); diff != "" {
				t.Errorf("\n%s\nUpdateResourceRefs(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}
