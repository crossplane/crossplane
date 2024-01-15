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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestFunctionCompose(t *testing.T) {
	errBoom := errors.New("boom")

	errProtoSyntax := protojson.Unmarshal([]byte("hi"), &structpb.Struct{})

	type params struct {
		kube client.Client
		r    FunctionRunner
		o    []FunctionComposerOption
	}
	type args struct {
		ctx context.Context
		xr  *composite.Unstructured
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
		"FetchConnectionError": {
			reason: "We should return any error encountered while fetching the XR's connection details.",
			params: params{
				o: []FunctionComposerOption{
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return ComposedResourceStates{}, nil
					})),
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, errBoom
					})),
				},
			},
			args: args{
				xr:  composite.New(),
				req: CompositionRequest{Revision: &v1.CompositionRevision{}},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchXRConnectionDetails),
			},
		},
		"GetComposedResourcesError": {
			reason: "We should return any error encountered while getting the XR's existing composed resources.",
			params: params{
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, errBoom
					})),
				},
			},
			args: args{
				xr:  composite.New(),
				req: CompositionRequest{Revision: &v1.CompositionRevision{}},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetExistingCDs),
			},
		},
		"UnmarshalFunctionInputError": {
			reason: "We should return any error encountered while unmarshalling a Composition Function input",
			params: params{
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				xr: composite.New(),
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
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					return nil, errBoom
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				xr: composite.New(),
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
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					r := &v1beta1.Result{
						Severity: v1beta1.Severity_SEVERITY_FATAL,
						Message:  "oh no",
					}
					return &v1beta1.RunFunctionResponse{Results: []*v1beta1.Result{r}}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				xr: composite.New(),
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
				err: errors.Errorf(errFmtFatalResult, "run-cool-function", "oh no"),
			},
		},
		"RenderComposedResourceMetadataError": {
			reason: "We should return any error we encounter when rendering composed resource metadata",
			params: params{
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					d := &v1beta1.State{
						Resources: map[string]*v1beta1.Resource{
							"cool-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "CoolComposed",
								}),
							},
						},
					}
					return &v1beta1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				// Missing labels required by RenderComposedResourceMetadata.
				xr: composite.New(),
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
				err: errors.Wrapf(RenderComposedResourceMetadata(nil, composite.New(), ""), errFmtRenderMetadata, "cool-resource"),
			},
		},
		"GenerateNameCreateComposedResourceError": {
			reason: "We should return any error we encounter when naming a composed resource",
			params: params{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					d := &v1beta1.State{
						Resources: map[string]*v1beta1.Resource{
							"cool-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "CoolComposed",

									// No name means we'll dry-run apply.
								}),
							},
						},
					}
					return &v1beta1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				xr: WithParentLabel(),
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
				err: errors.Wrapf(errBoom, errFmtGenerateName, "cool-resource"),
			},
		},
		"GarbageCollectComposedResourcesError": {
			reason: "We should return any error we encounter when garbage collecting composed resources",
			params: params{
				kube: &test.MockClient{
					MockPatch: test.NewMockPatchFn(nil),
				},
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					return &v1beta1.RunFunctionResponse{}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return errBoom
					})),
				},
			},
			args: args{
				xr: WithParentLabel(),
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
		"ApplyXRResourceReferencesError": {
			reason: "We should return any error we encounter when applying the composite resource's resource references",
			params: params{
				kube: &test.MockClient{
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// We only want to return an error for the XR.
						u := obj.(*kunstructured.Unstructured)
						if u.GetKind() == "CoolComposed" {
							return nil
						}
						return errBoom
					}),
				},
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					return &v1beta1.RunFunctionResponse{}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: composite.New(),
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
				err: errors.Wrap(errBoom, errApplyXRRefs),
			},
		},
		"ApplyXRStatusError": {
			reason: "We should return any error we encounter when applying the composite resource status",
			params: params{
				kube: &test.MockClient{
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(errBoom),
				},
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					d := &v1beta1.State{
						Composite: &v1beta1.Resource{
							Resource: MustStruct(map[string]any{
								"status": map[string]any{
									"widgets": 42,
								},
							})},
					}
					return &v1beta1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: composite.New(),
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
				err: errors.Wrap(errBoom, errApplyXRStatus),
			},
		},
		"ApplyComposedResourceError": {
			reason: "We should return any error we encounter when applying a composed resource",
			params: params{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "UncoolComposed"}, "")), // all names are available
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// We only want to return an error if we're patching a
						// composed resource.
						u := obj.(*kunstructured.Unstructured)
						if u.GetKind() == "UncoolComposed" {
							return errBoom
						}
						return nil
					}),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (rsp *v1beta1.RunFunctionResponse, err error) {
					d := &v1beta1.State{
						Resources: map[string]*v1beta1.Resource{
							"uncool-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "UncoolComposed",
								}),
							},
						},
					}
					return &v1beta1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: WithParentLabel(),
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
				err: errors.Wrapf(errBoom, errFmtApplyCD, "uncool-resource"),
			},
		},
		"Successful": {
			reason: "We should return a valid CompositionResult when a 'pure Function' (i.e. patch-and-transform-less) reconcile succeeds",
			params: params{
				kube: &test.MockClient{
					MockGet:         test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "UncoolComposed"}, "")), // all names are available
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (*v1beta1.RunFunctionResponse, error) {
					rsp := &v1beta1.RunFunctionResponse{
						Desired: &v1beta1.State{
							Composite: &v1beta1.Resource{
								Resource: MustStruct(map[string]any{
									"status": map[string]any{
										"widgets": 42,
									},
								}),
								ConnectionDetails: map[string][]byte{"from": []byte("function-pipeline")},
							},
							Resources: map[string]*v1beta1.Resource{
								"observed-resource-a": {
									Resource: MustStruct(map[string]any{
										"apiVersion": "test.crossplane.io/v1",
										"kind":       "CoolComposed",
									}),
									Ready: v1beta1.Ready_READY_TRUE,
								},
								"desired-resource-a": {
									Resource: MustStruct(map[string]any{
										"apiVersion": "test.crossplane.io/v1",
										"kind":       "CoolComposed",
									}),
								},
							},
						},
						Results: []*v1beta1.Result{
							{
								Severity: v1beta1.Severity_SEVERITY_NORMAL,
								Message:  "A normal result",
							},
							{
								Severity: v1beta1.Severity_SEVERITY_WARNING,
								Message:  "A warning result",
							},
							{
								Severity: v1beta1.Severity_SEVERITY_UNSPECIFIED,
								Message:  "A result of unspecified severity",
							},
						},
					}
					return rsp, nil
				}),
				o: []FunctionComposerOption{
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
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: func() *composite.Unstructured {
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
							Message: "Pipeline step \"run-cool-function\": A normal result",
						},
						{
							Type:    "Warning",
							Reason:  "ComposeResources",
							Message: "Pipeline step \"run-cool-function\": A warning result",
						},
						{
							Type:    "Warning",
							Reason:  "ComposeResources",
							Message: "Pipeline step \"run-cool-function\" returned a result of unknown severity (assuming warning): A result of unspecified severity",
						},
					},
				},
				err: nil,
			},
		},
		"SuccessfulWithExtraResources": {
			reason: "We should return a valid CompositionResult when a 'pure Function' (i.e. patch-and-transform-less) reconcile succeeds after having requested some extra resource",
			params: params{
				kube: &test.MockClient{
					MockGet:         test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "UncoolComposed"}, "")), // all names are available
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: func() FunctionRunner {
					var nrCalls int
					return FunctionRunnerFn(func(ctx context.Context, name string, req *v1beta1.RunFunctionRequest) (*v1beta1.RunFunctionResponse, error) {
						defer func() { nrCalls++ }()
						requirements := &v1beta1.Requirements{
							ExtraResources: map[string]*v1beta1.ResourceSelector{
								"existing": {
									ApiVersion: "test.crossplane.io/v1",
									Kind:       "Foo",
									MatchName:  ptr.To("existing"),
								},
								"missing": {
									ApiVersion: "test.crossplane.io/v1",
									Kind:       "Bar",
									MatchName:  ptr.To("missing"),
								},
							},
						}
						rsp := &v1beta1.RunFunctionResponse{
							Desired: &v1beta1.State{
								Composite: &v1beta1.Resource{
									Resource: MustStruct(map[string]any{
										"status": map[string]any{
											"widgets": 42,
										},
									}),
									ConnectionDetails: map[string][]byte{"from": []byte("function-pipeline")},
								},
								Resources: map[string]*v1beta1.Resource{
									"observed-resource-a": {
										Resource: MustStruct(map[string]any{
											"apiVersion": "test.crossplane.io/v1",
											"kind":       "CoolComposed",
											"spec": map[string]any{
												"someKey": req.GetInput().AsMap()["someKey"].(string),
											},
										}),
										Ready: v1beta1.Ready_READY_TRUE,
									},
									"desired-resource-a": {
										Resource: MustStruct(map[string]any{
											"apiVersion": "test.crossplane.io/v1",
											"kind":       "CoolComposed",
										}),
									},
								},
							},
							Results: []*v1beta1.Result{
								{
									Severity: v1beta1.Severity_SEVERITY_NORMAL,
									Message:  "A normal result",
								},
								{
									Severity: v1beta1.Severity_SEVERITY_WARNING,
									Message:  "A warning result",
								},
								{
									Severity: v1beta1.Severity_SEVERITY_UNSPECIFIED,
									Message:  "A result of unspecified severity",
								},
							},
							Requirements: requirements,
						}

						if nrCalls > 1 {
							t.Fatalf("unexpected number of calls to FunctionRunner.RunFunction, should have been exactly 2: %d", nrCalls+1)
							return nil, errBoom
						}

						if nrCalls == 1 {
							if len(req.GetExtraResources()) != 2 {
								t.Fatalf("unexpected number of extra resources: %d", len(requirements.GetExtraResources()))
							}
							if rs := req.GetExtraResources()["missing"]; rs != nil && len(rs.GetItems()) != 0 {
								t.Fatalf("unexpected extra resource, expected none, got: %v", rs)
							}
							if rs := req.GetExtraResources()["existing"]; rs == nil || len(rs.GetItems()) != 1 {
								t.Fatalf("unexpected extra resource, expected one, got: %v", rs)
							}
						}

						return rsp, nil
					})
				}(),
				o: []FunctionComposerOption{
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
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(ctx context.Context, owner metav1.Object, observed, desired ComposedResourceStates) error {
						return nil
					})),
					WithExtraResourcesGetter(ExtraResourcesGetterFn(func(ctx context.Context, selector *v1beta1.ResourceSelector) (*v1beta1.Resources, error) {
						if selector.GetMatchName() == "existing" {
							return &v1beta1.Resources{
								Items: []*v1beta1.Resource{
									{
										Resource: MustStruct(map[string]any{
											"apiVersion": "test.crossplane.io/v1",
											"kind":       "Foo",
											"metadata": map[string]any{
												"name": "existing",
											},
											"spec": map[string]any{
												"someField": "someValue",
											},
										}),
									},
								},
							}, nil
						}
						return nil, nil
					})),
				},
			},
			args: args{
				xr: func() *composite.Unstructured {
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
									Input: &runtime.RawExtension{Raw: []byte(`{
										"apiVersion": "test.crossplane.io/v1",
										"kind": "Input",
										"someKey": "someValue"
									}`)},
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
							Message: "Pipeline step \"run-cool-function\": A normal result",
						},
						{
							Type:    "Warning",
							Reason:  "ComposeResources",
							Message: "Pipeline step \"run-cool-function\": A warning result",
						},
						{
							Type:    "Warning",
							Reason:  "ComposeResources",
							Message: "Pipeline step \"run-cool-function\" returned a result of unknown severity (assuming warning): A result of unspecified severity",
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := NewFunctionComposer(tc.params.kube, tc.params.r, tc.params.o...)
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

func MustStruct(v map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(v)
	if err != nil {
		panic(err)
	}
	return s
}

func WithParentLabel() *composite.Unstructured {
	xr := composite.New()
	xr.SetLabels(map[string]string{xcrd.LabelKeyNamePrefixForComposed: "parent-xr"})
	return xr
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
							Controller: ptr.To(true),
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
		d   *v1beta1.State
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
				d: &v1beta1.State{
					Composite: &v1beta1.Resource{
						Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
							"apiVersion": structpb.NewStringValue("example.org/v1"),
							"kind":       structpb.NewStringValue("Composite"),
						}},
						ConnectionDetails: map[string][]byte{"a": []byte("b")},
					},
					Resources: map[string]*v1beta1.Resource{
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
									Controller: ptr.To(true),
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
									Controller: ptr.To(true),
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
									Controller: ptr.To(true),
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

func TestExistingExtraResourcesGetterGet(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		selector *v1beta1.ResourceSelector
		c        client.Reader
	}
	type want struct {
		res *v1beta1.Resources
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessMatchName": {
			reason: "We should return a valid Resources when a resource is found by name",
			args: args{
				selector: &v1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					MatchName:  ptr.To("cool-resource"),
				},
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource")
						return nil
					}),
				},
			},
			want: want{
				res: &v1beta1.Resources{
					Items: []*v1beta1.Resource{
						{
							Resource: MustStruct(map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "Foo",
								"metadata": map[string]any{
									"name": "cool-resource",
								},
							}),
						},
					},
				},
			},
		},
		"SuccessMatchLabels": {
			reason: "We should return a valid Resources when a resource is found by labels",
			args: args{
				selector: &v1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					MatchLabels: map[string]string{
						"cool": "resource",
					},
				},
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "Foo",
									"metadata": map[string]interface{}{
										"name": "cool-resource",
										"labels": map[string]interface{}{
											"cool": "resource",
										},
									},
								},
							},
							{
								Object: map[string]interface{}{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "Foo",
									"metadata": map[string]interface{}{
										"name": "cooler-resource",
										"labels": map[string]interface{}{
											"cool": "resource",
										},
									},
								},
							},
						}
						return nil
					}),
				},
			},
			want: want{
				res: &v1beta1.Resources{
					Items: []*v1beta1.Resource{
						{
							Resource: MustStruct(map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "Foo",
								"metadata": map[string]any{
									"name": "cool-resource",
									"labels": map[string]any{
										"cool": "resource",
									},
								},
							}),
						},
						{
							Resource: MustStruct(map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "Foo",
								"metadata": map[string]any{
									"name": "cooler-resource",
									"labels": map[string]any{
										"cool": "resource",
									},
								},
							}),
						},
					},
				},
			},
		},
		"NotFoundMatchName": {
			reason: "We should return no error when a resource is not found by name",
			args: args{
				selector: &v1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					MatchName:  ptr.To("cool-resource"),
				},
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "Foo"}, "cool-resource")),
				},
			},
			want: want{
				res: nil,
				err: nil,
			},
		},
		// NOTE(phisco): No NotFound error is returned when listing resources by labels, so there is no NotFoundMatchLabels test case.
		"ErrorMatchName": {
			reason: "We should return any other error encountered when getting a resource by name",
			args: args{
				selector: &v1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					MatchName:  ptr.To("cool-resource"),
				},
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				res: nil,
				err: errBoom,
			},
		},
		"ErrorMatchLabels": {
			reason: "We should return any other error encountered when listing resources by labels",
			args: args{
				selector: &v1beta1.ResourceSelector{
					ApiVersion: "test.crossplane.io/v1",
					Kind:       "Foo",
					MatchLabels: map[string]string{
						"cool": "resource",
					},
				},
				c: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
			},
			want: want{
				res: nil,
				err: errBoom,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewExistingExtraResourcesGetter(tc.args.c)
			res, err := g.Get(context.Background(), tc.args.selector)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.res, res, cmpopts.IgnoreUnexported(v1beta1.Resources{}, v1beta1.Resource{}, structpb.Struct{}, structpb.Value{})); diff != "" {
				t.Errorf("\n%s\nGet(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
