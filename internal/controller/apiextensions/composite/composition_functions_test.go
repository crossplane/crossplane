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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
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
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return ComposedResourceStates{}, nil
					})),
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
		"GetCredentialsSecretError": {
			reason: "We should return any error encountered while getting the credentials secret for a Composition Function",
			params: params{
				kube: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
				},
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
									Credentials: []v1.FunctionCredentials{
										{
											Name:   "cool-secret",
											Source: v1.FunctionCredentialsSourceSecret,
											SecretRef: &xpv1.SecretReference{
												Namespace: "default",
												Name:      "cool-secret",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtGetCredentialsFromSecret, "run-cool-function", "cool-secret"),
			},
		},
		"RunFunctionError": {
			reason: "We should return any error encountered while running a Composition Function",
			params: params{
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					return nil, errBoom
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
			reason: "We should return any fatal function results as an error. Any conditions returned by the function should be passed up. Any results returned by the function prior to the fatal result should be passed up.",
			params: params{
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					return &fnv1.RunFunctionResponse{
						Results: []*fnv1.Result{
							// This result should be passed up as it was sent before the fatal
							// result. The reason should be defaulted. The target should be
							// defaulted.
							{
								Severity: fnv1.Severity_SEVERITY_NORMAL,
								Message:  "A result before the fatal result with the default Reason.",
							},
							// This result should be passed up as it was sent before the fatal
							// result. The reason should be kept. The target should be kept.
							{
								Severity: fnv1.Severity_SEVERITY_NORMAL,
								Reason:   ptr.To("SomeReason"),
								Message:  "A result before the fatal result with a specific Reason.",
								Target:   fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
							},
							// The fatal result
							{
								Severity: fnv1.Severity_SEVERITY_FATAL,
								Message:  "oh no",
							},
							// This result should not be passed up as it was sent after the
							// fatal result.
							{
								Severity: fnv1.Severity_SEVERITY_NORMAL,
								Message:  "a result after the fatal result",
							},
						},
						Conditions: []*fnv1.Condition{
							// A condition returned by the function with only the minimum
							// necessary values.
							{
								Type:   "DatabaseReady",
								Status: fnv1.Status_STATUS_CONDITION_FALSE,
								Reason: "Creating",
							},
							// A condition returned by the function with all optional values
							// given.
							{
								Type:    "DeploymentReady",
								Status:  fnv1.Status_STATUS_CONDITION_TRUE,
								Reason:  "Available",
								Message: ptr.To("The deployment is ready."),
								Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
							},
						},
					}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
				res: CompositionResult{
					Events: []TargetedEvent{
						// The event with minimum values.
						{
							Event: event.Event{
								Type:    "Normal",
								Reason:  "ComposeResources",
								Message: "A result before the fatal result with the default Reason.",
							},
							Detail: "Pipeline step \"run-cool-function\"",
							Target: CompositionTargetComposite,
						},
						// The event that provides all possible values.
						{
							Event: event.Event{
								Type:    "Normal",
								Reason:  "SomeReason",
								Message: "A result before the fatal result with a specific Reason.",
							},
							Detail: "Pipeline step \"run-cool-function\"",
							Target: CompositionTargetCompositeAndClaim,
						},
					},
					Conditions: []TargetedCondition{
						// The condition with minimum values.
						{
							Condition: xpv1.Condition{
								Type:   "DatabaseReady",
								Status: "False",
								Reason: "Creating",
							},
							Target: CompositionTargetComposite,
						},
						// The condition that provides all possible values.
						{
							Condition: xpv1.Condition{
								Type:    "DeploymentReady",
								Status:  "True",
								Reason:  "Available",
								Message: "The deployment is ready.",
							},
							Target: CompositionTargetCompositeAndClaim,
						},
					},
				},
			},
		},
		"RenderComposedResourceMetadataError": {
			reason: "We should return any error we encounter when rendering composed resource metadata",
			params: params{
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"cool-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "CoolComposed",
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"cool-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "CoolComposed",

									// No name means we'll dry-run apply.
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					return &fnv1.RunFunctionResponse{}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
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
						switch obj.(type) {
						case *composite.Unstructured:
							return errBoom
						default:
						}
						return nil
					}),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					return &fnv1.RunFunctionResponse{}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
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
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: MustStruct(map[string]any{
								"status": map[string]any{
									"widgets": 42,
								},
							}),
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
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
						switch obj.(type) {
						case *composed.Unstructured:
							return errBoom
						default:
						}
						return nil
					}),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"uncool-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "UncoolComposed",
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
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
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							s.Data = map[string][]byte{
								"secret": []byte("password"),
							}
							return nil
						}

						// If this isn't a secret, it's a composed resource.
						// Return not found to indicate its name is available.
						// TODO(negz): This is "testing through" to the
						// names.NameGenerator implementation. Mock it out.
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					}),
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Desired: &fnv1.State{
							Composite: &fnv1.Resource{
								Resource: MustStruct(map[string]any{
									"status": map[string]any{
										"widgets": 42,
									},
								}),
								ConnectionDetails: map[string][]byte{"from": []byte("function-pipeline")},
							},
							Resources: map[string]*fnv1.Resource{
								"observed-resource-a": {
									Resource: MustStruct(map[string]any{
										"apiVersion": "test.crossplane.io/v1",
										"kind":       "CoolComposed",
									}),
									Ready: fnv1.Ready_READY_TRUE,
								},
								"desired-resource-a": {
									Resource: MustStruct(map[string]any{
										"apiVersion": "test.crossplane.io/v1",
										"kind":       "CoolComposed",
									}),
								},
							},
						},
						Results: []*fnv1.Result{
							{
								Severity: fnv1.Severity_SEVERITY_NORMAL,
								Message:  "A normal result",
							},
							{
								Severity: fnv1.Severity_SEVERITY_WARNING,
								Message:  "A warning result",
							},
							{
								Severity: fnv1.Severity_SEVERITY_UNSPECIFIED,
								Message:  "A result of unspecified severity",
							},
							{
								Severity: fnv1.Severity_SEVERITY_NORMAL,
								Reason:   ptr.To("SomeReason"),
								Message:  "A result with all values explicitly set.",
								Target:   fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
							},
						},
						Conditions: []*fnv1.Condition{
							// A condition returned by the function with only the minimum
							// necessary values.
							{
								Type:   "DatabaseReady",
								Status: fnv1.Status_STATUS_CONDITION_FALSE,
								Reason: "Creating",
							},
							// A condition returned by the function with all optional values
							// given.
							{
								Type:    "DeploymentReady",
								Status:  fnv1.Status_STATUS_CONDITION_TRUE,
								Reason:  "Available",
								Message: ptr.To("The deployment is ready."),
								Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
							},
						},
					}
					return rsp, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
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
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
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
									Credentials: []v1.FunctionCredentials{
										{
											Name:   "cool-secret",
											Source: v1.FunctionCredentialsSourceSecret,
											SecretRef: &xpv1.SecretReference{
												Namespace: "default",
												Name:      "cool-secret",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{
						{ResourceName: "desired-resource-a", Synced: true},
						{ResourceName: "observed-resource-a", Ready: true, Synced: true},
					},
					ConnectionDetails: managed.ConnectionDetails{
						"from": []byte("function-pipeline"),
					},
					Events: []TargetedEvent{
						{
							Event: event.Event{
								Type:    "Normal",
								Reason:  "ComposeResources",
								Message: "A normal result",
							},
							Detail: "Pipeline step \"run-cool-function\"",
							Target: CompositionTargetComposite,
						},
						{
							Event: event.Event{
								Type:    "Warning",
								Reason:  "ComposeResources",
								Message: "A warning result",
							},
							Detail: "Pipeline step \"run-cool-function\"",
							Target: CompositionTargetComposite,
						},
						{
							Event: event.Event{
								Type:    "Warning",
								Reason:  "ComposeResources",
								Message: "Pipeline step \"run-cool-function\" returned a result of unknown severity (assuming warning): A result of unspecified severity",
							},
							Target: CompositionTargetComposite,
						},
						{
							Event: event.Event{
								Type:    "Normal",
								Reason:  "SomeReason",
								Message: "A result with all values explicitly set.",
							},
							Detail: "Pipeline step \"run-cool-function\"",
							Target: CompositionTargetCompositeAndClaim,
						},
					},
					Conditions: []TargetedCondition{
						// The condition with minimum values.
						{
							Condition: xpv1.Condition{
								Type:   "DatabaseReady",
								Status: "False",
								Reason: "Creating",
							},
							Target: CompositionTargetComposite,
						},
						// The condition that provides all possible values.
						{
							Condition: xpv1.Condition{
								Type:    "DeploymentReady",
								Status:  "True",
								Reason:  "Available",
								Message: "The deployment is ready.",
							},
							Target: CompositionTargetCompositeAndClaim,
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
				f: ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
				f: ConnectionDetailsFetcherFn(func(_ context.Context, _ resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
				ors: ComposedResourceStates{
					"cool-resource": ComposedResourceState{
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
		d   *fnv1.State
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
				d: &fnv1.State{
					Composite: &fnv1.Resource{
						Resource: &structpb.Struct{Fields: map[string]*structpb.Value{
							"apiVersion": structpb.NewStringValue("example.org/v1"),
							"kind":       structpb.NewStringValue("Composite"),
						}},
						ConnectionDetails: map[string][]byte{"a": []byte("b")},
					},
					Resources: map[string]*fnv1.Resource{
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
					"undesired-resource": ComposedResourceState{Resource: &fake.Composed{
						ObjectMeta: metav1.ObjectMeta{
							// This resource isn't controlled by the XR.
							OwnerReferences: []metav1.OwnerReference{{
								Controller: ptr.To(true),
								UID:        "a-different-xr",
								Kind:       "XR",
								Name:       "different",
							}},
						},
					}},
				},
			},
			want: want{
				err: errors.New(`refusing to delete composed resource "undesired-resource" that is controlled by XR "different"`),
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
