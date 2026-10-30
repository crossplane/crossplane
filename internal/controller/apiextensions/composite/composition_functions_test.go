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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xcrd"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// mockGetFunction returns a MockGetFn that populates *pkgv1.Function requests
// with a package reference (so a name-based pipeline step resolves), delegating
// any other object type to next (which may be nil).
func mockGetFunction(next test.MockGetFn) test.MockGetFn {
	return func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if f, ok := obj.(*pkgv1.Function); ok {
			f.SetName("cool-function")
			f.SetUID("cool-function-uid")
			f.Spec.Package = "xpkg.crossplane.io/example/cool-function:v1.0.0"
			return nil
		}
		if next != nil {
			return next(ctx, key, obj)
		}
		return nil
	}
}

// mockListFunctionRevisions returns a MockListFn that populates
// *pkgv1.FunctionRevisionList requests with an active revision controlled by
// the Function returned by mockGetFunction, so a name-based pipeline step
// resolves.
func mockListFunctionRevisions() test.MockListFn {
	return func(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
		l, ok := obj.(*pkgv1.FunctionRevisionList)
		if !ok {
			return nil
		}

		l.Items = []pkgv1.FunctionRevision{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cool-function-abc123",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: pkgv1.FunctionGroupVersionKind.GroupVersion().String(),
					Kind:       pkgv1.FunctionKind,
					Name:       "cool-function",
					UID:        "cool-function-uid",
					Controller: new(true),
				}},
			},
			Spec: pkgv1.FunctionRevisionSpec{
				PackageRevisionSpec: pkgv1.PackageRevisionSpec{
					DesiredState: pkgv1.PackageRevisionActive,
					Package:      "xpkg.crossplane.io/example/cool-function:v1.0.0",
				},
			},
		}}

		return nil
	}
}

func TestFunctionCompose(t *testing.T) {
	errBoom := errors.New("boom")

	errProtoSyntax := protojson.Unmarshal([]byte("hi"), &structpb.Struct{})
	errFmtFetchBootstrapRequirements := "cannot fetch bootstrap required resources for requirement %q"

	type params struct {
		c  client.Client
		uc client.Client
		r  FunctionRunner
		o  []FunctionComposerOption
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
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
				c: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
				},
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
				},
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
									Credentials: []v1.FunctionCredentials{
										{
											Name:   "cool-secret",
											Source: v1.FunctionCredentialsSourceSecret,
											SecretRef: &xpv2.SecretReference{
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
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(nil),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					return nil, errBoom
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
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
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(nil),
				},
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
								Reason:   new("SomeReason"),
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
								Message: new("The deployment is ready."),
								Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
							},
						},
					}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: &PipelineFatalError{Step: "run-cool-function", Message: "oh no"},
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
							Condition: xpv2.Condition{
								Type:   "DatabaseReady",
								Status: "False",
								Reason: "Creating",
							},
							Target: CompositionTargetComposite,
						},
						// The condition that provides all possible values.
						{
							Condition: xpv2.Condition{
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
				c: &test.MockClient{
					MockList:        mockListFunctionRevisions(),
					MockGet:         mockGetFunction(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
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
		"InvalidNameCreateComposedResourceError": {
			reason: "We should return an error when a resource has an invalid name",
			params: params{
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(nil),
				},
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"cool-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "CoolComposed",
									"metadata": map[string]any{
										"name": "%!(cool)-resource", // This is invalid - it must be a valid DNS subdomain.
									},
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtInvalidName, "cool-resource", "%!(cool)-resource"),
			},
		},
		"GenerateNameCreateComposedResourceError": {
			reason: "We should return any error we encounter when naming a composed resource",
			params: params{
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(test.NewMockGetFn(errBoom)),
				},
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: ComposedResourceError{
					Message: fmt.Sprintf(errFmtGenerateName, "cool-resource"),
					Composed: &composed.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "CoolComposed",
								"metadata": map[string]any{
									"generateName": "parent-xr-",
								},
							},
						},
					},
					Err: errBoom,
				},
			},
		},
		"GarbageCollectComposedResourcesError": {
			reason: "We should return any error we encounter when garbage collecting composed resources",
			params: params{
				c: &test.MockClient{
					MockList:  mockListFunctionRevisions(),
					MockGet:   mockGetFunction(nil),
					MockPatch: test.NewMockPatchFn(nil),
				},
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					return &fnv1.RunFunctionResponse{}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
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
		"NamespacedXRClusterCompositionError": {
			reason: "We should return an error when a namespaced XR tries to compose cluster-scoped resources",
			params: params{
				c: &test.MockClient{
					MockList:               mockListFunctionRevisions(),
					MockGet:                mockGetFunction(nil),
					MockPatch:              test.NewMockPatchFn(nil),
					MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
					MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
				},
				uc: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"cluster-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "ClusterComposed",
									"metadata": map[string]any{
										"name": "cluster-resource",
									},
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
				xr: func() *composite.Unstructured {
					xr := WithParentLabel()
					xr.SetNamespace("test-namespace") // Make the XR namespaced
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtNamespacedXRClusterResource, "cluster-resource", "ClusterComposed", "cluster-resource"),
			},
		},
		"NamespacedXRNamespacedComposition": {
			reason: "We should succeed when a namespaced XR tries to compose namespaced-scoped resources",
			params: params{
				c: &test.MockClient{
					MockList:               mockListFunctionRevisions(),
					MockGet:                mockGetFunction(nil),
					MockPatch:              test.NewMockPatchFn(nil),
					MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
					MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, true),
				},
				uc: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"ns-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "NamespaceComposed",
									"metadata": map[string]any{
										"name": "ns-resource",
									},
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
				xr: func() *composite.Unstructured {
					xr := WithParentLabel()
					xr.SetNamespace("test-namespace") // Make the XR namespaced
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{{ResourceName: "ns-resource", Ready: false, Synced: true}},
				},
			},
		},
		"NamespacedXRNamespaceOverridden": {
			reason: "We should emit a warning when a namespaced XR composes a resource with a different namespace",
			params: params{
				c: &test.MockClient{
					MockList:        mockListFunctionRevisions(),
					MockGet:         mockGetFunction(nil),
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				uc: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"ns-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "NamespaceComposed",
									"metadata": map[string]any{
										"name":      "ns-resource",
										"namespace": "other-namespace",
									},
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
				xr: func() *composite.Unstructured {
					xr := WithParentLabel()
					xr.SetNamespace("test-namespace")
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{{ResourceName: "ns-resource", Ready: false, Synced: true}},
					Events: []TargetedEvent{
						{
							Event: event.Event{
								Type:    "Warning",
								Reason:  reasonNamespaceOverridden,
								Message: "cannot create composed resource \"ns-resource\" in namespace \"other-namespace\", using XR namespace \"test-namespace\" instead",
							},
							Target: CompositionTargetComposite,
						},
					},
				},
			},
		},
		"NamespacedXRNamespaceMatchesXR": {
			reason: "We should not emit a warning when a namespaced XR composes a resource with the same namespace",
			params: params{
				c: &test.MockClient{
					MockList:        mockListFunctionRevisions(),
					MockGet:         mockGetFunction(nil),
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				uc: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"ns-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "NamespaceComposed",
									"metadata": map[string]any{
										"name":      "ns-resource",
										"namespace": "test-namespace", // matches XR namespace
									},
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
				xr: func() *composite.Unstructured {
					xr := WithParentLabel()
					xr.SetNamespace("test-namespace")
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{{ResourceName: "ns-resource", Ready: false, Synced: true}},
				},
			},
		},
		"ClusterXRClusterCompositionSuccess": {
			reason: "Cluster-scoped XRs should be allowed to compose cluster-scoped resources",
			params: params{
				c: &test.MockClient{
					MockList:               mockListFunctionRevisions(),
					MockGet:                mockGetFunction(test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "ClusterComposed"}, ""))), // all names are available
					MockPatch:              test.NewMockPatchFn(nil),
					MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
					MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(errBoom, false),
				},
				uc: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					d := &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"cluster-resource": {
								Resource: MustStruct(map[string]any{
									"apiVersion": "test.crossplane.io/v1",
									"kind":       "ClusterComposed",
									"metadata": map[string]any{
										"name": "cluster-resource",
									},
								}),
							},
						},
					}
					return &fnv1.RunFunctionResponse{Desired: d}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
				xr: WithParentLabel(), // Cluster-scoped XR (no namespace)
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{{ResourceName: "cluster-resource", Ready: false, Synced: true}},
				},
			},
		},
		"ApplyXRResourceReferencesError": {
			reason: "We should return any error we encounter when applying the composite resource's resource references",
			params: params{
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(nil),
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
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
					return &fnv1.RunFunctionResponse{}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
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
				c: &test.MockClient{
					MockList:        mockListFunctionRevisions(),
					MockGet:         mockGetFunction(nil),
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(errBoom),
				},
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
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
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "UncoolComposed"}, ""))), // all names are available
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
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
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
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				err: ComposedResourceError{
					Message: fmt.Sprintf(errFmtApplyCD, "uncool-resource"),
					Composed: &composed.Unstructured{
						Unstructured: unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "test.crossplane.io/v1",
								"kind":       "UncoolComposed",
							},
						},
					},
					Err: errBoom,
				},
			},
		},
		"BootstrapRequirementsError": {
			reason: "We should return an error if we can't fetch bootstrap requirements",
			params: params{
				c: &test.MockClient{},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return &fnv1.RunFunctionResponse{}, nil
				}),
				o: []FunctionComposerOption{
					WithRequiredResourcesFetcher(xfn.RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
						return nil, errBoom
					})),
				},
			},
			args: args{
				ctx: context.Background(),
				xr:  WithParentLabel(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "cool-step",
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
									Requirements: &v1.FunctionRequirements{
										RequiredResources: []v1.RequiredResourceSelector{
											{
												RequirementName: "test-requirement",
												APIVersion:      "v1",
												Kind:            "ConfigMap",
												Name:            new("test-config"),
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
				err: errors.Wrapf(errBoom, errFmtFetchBootstrapRequirements, "test-requirement"),
			},
		},
		"Successful": {
			reason: "We should return a valid CompositionResult when a 'pure Function' (i.e. patch-and-transform-less) reconcile succeeds",
			params: params{
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet: mockGetFunction(test.NewMockGetFn(nil, func(obj client.Object) error {
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
					})),
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				uc: &test.MockClient{
					// Return an error when we try to get the secret.
					MockGet: test.NewMockGetFn(errBoom),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{
							Ttl: durationpb.New(5 * time.Minute),
						},
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
								Reason:   new("SomeReason"),
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
								Message: new("The deployment is ready."),
								Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
							},
						},
					}
					return rsp, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						// We only try to extract connection details for
						// observed resources.
						r := ComposedResourceStates{
							"observed-resource-a": ComposedResourceState{
								Resource: &fake.Composed{
									ObjectMeta: metav1.ObjectMeta{Name: "observed-resource-a", OwnerReferences: []metav1.OwnerReference{{Controller: new(true)}}},
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
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
									Credentials: []v1.FunctionCredentials{
										{
											Name:   "cool-secret",
											Source: v1.FunctionCredentialsSourceSecret,
											SecretRef: &xpv2.SecretReference{
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
							Condition: xpv2.Condition{
								Type:   "DatabaseReady",
								Status: "False",
								Reason: "Creating",
							},
							Target: CompositionTargetComposite,
						},
						// The condition that provides all possible values.
						{
							Condition: xpv2.Condition{
								Type:    "DeploymentReady",
								Status:  "True",
								Reason:  "Available",
								Message: "The deployment is ready.",
							},
							Target: CompositionTargetCompositeAndClaim,
						},
					},
					TTL:      5 * time.Minute,
					Existing: 1,
				},
				err: nil,
			},
		},
		"RetainReferencesToResourcesThatStillExist": {
			reason: "With function-controlled deletion and a Foreground or Pipeline policy, we should keep referencing a composed resource the pipeline no longer desires until it's really gone, so functions can see it's still being deleted.",
			params: params{
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))),
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// We only care about the patch that persists the XR's
						// composed resource references.
						xr, ok := obj.(*composite.Unstructured)
						if !ok {
							return nil
						}

						refs := xr.GetResourceReferences()
						if len(refs) != 1 {
							return errors.Errorf("want 1 resource reference, got %d", len(refs))
						}
						if refs[0].Name != "observed-resource-a" {
							return errors.Errorf("want a reference to observed-resource-a, got %q", refs[0].Name)
						}
						// No resourceRefs schema has a uid field.
						if refs[0].UID != "" {
							return errors.Errorf("want no UID in the resource reference, got %q", refs[0].UID)
						}

						return nil
					}),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					// A function that has stopped desiring the composed resource
					// it used to desire, e.g. to tear it down in order.
					return &fnv1.RunFunctionResponse{Desired: &fnv1.State{}}, nil
				}),
				o: []FunctionComposerOption{
					WithFunctionControlledDeletion(true),
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						// The resource still exists, and Kubernetes has begun
						// deleting it.
						cd := &fake.Composed{ObjectMeta: metav1.ObjectMeta{
							Name:              "observed-resource-a",
							UID:               "some-uid",
							DeletionTimestamp: &metav1.Time{Time: time.Now()},
							OwnerReferences:   []metav1.OwnerReference{{Controller: new(true)}},
						}}
						return ComposedResourceStates{"observed-resource-a": ComposedResourceState{Resource: cd}}, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: func() *composite.Unstructured {
					xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "test.crossplane.io",
						Version: "v1",
						Kind:    "CoolComposite",
					}))
					xr.SetDeletionPolicy(v1.CompositionDeletionPolicyPipeline)

					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{{
								Step:        "run-cool-function",
								FunctionRef: &v1.FunctionReference{Name: "cool-function"},
							}},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{},
					Existing: 1,
				},
			},
		},
		"DontRetainReferencesForBackgroundXRs": {
			reason: "With function-controlled deletion, an XR with the Background policy should stop referencing a composed resource as soon as the pipeline no longer desires it, as it does without the feature.",
			params: params{
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))),
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// We only care about the patch that persists the XR's
						// composed resource references.
						xr, ok := obj.(*composite.Unstructured)
						if !ok {
							return nil
						}

						if refs := xr.GetResourceReferences(); len(refs) != 0 {
							return errors.Errorf("want no resource references, got %d", len(refs))
						}

						return nil
					}),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					// A function that has stopped desiring the composed resource
					// it used to desire, e.g. to tear it down in order.
					return &fnv1.RunFunctionResponse{Desired: &fnv1.State{}}, nil
				}),
				o: []FunctionComposerOption{
					WithFunctionControlledDeletion(true),
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						// The resource still exists, and Kubernetes has begun
						// deleting it.
						cd := &fake.Composed{ObjectMeta: metav1.ObjectMeta{
							Name:              "observed-resource-a",
							UID:               "some-uid",
							DeletionTimestamp: &metav1.Time{Time: time.Now()},
							OwnerReferences:   []metav1.OwnerReference{{Controller: new(true)}},
						}}
						return ComposedResourceStates{"observed-resource-a": ComposedResourceState{Resource: cd}}, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
					Group:   "test.crossplane.io",
					Version: "v1",
					Kind:    "CoolComposite",
				})),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{{
								Step:        "run-cool-function",
								FunctionRef: &v1.FunctionReference{Name: "cool-function"},
							}},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{},
					Existing: 1,
				},
			},
		},
		"PreserveRecordedDeletionPolicy": {
			reason: "We should keep the deletion policy the XR recorded when we apply its status, because functions don't return it and the apply would delete it.",
			params: params{
				c: &test.MockClient{
					MockList:  mockListFunctionRevisions(),
					MockGet:   mockGetFunction(nil),
					MockPatch: test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil, func(obj client.Object) error {
						xr, ok := obj.(*composite.Unstructured)
						if !ok {
							return nil
						}
						if got := xr.GetDeletionPolicy(); got != v1.CompositionDeletionPolicyPipeline {
							return errors.Errorf("want deletion policy %q in the status patch, got %q", v1.CompositionDeletionPolicyPipeline, got)
						}

						return nil
					}),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return &fnv1.RunFunctionResponse{Desired: &fnv1.State{
						Composite: &fnv1.Resource{Resource: MustStruct(map[string]any{
							"apiVersion": "test.crossplane.io/v1",
							"kind":       "CoolComposite",
						})},
					}}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return ComposedResourceStates{}, nil
					})),
				},
			},
			args: args{
				xr: func() *composite.Unstructured {
					xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "test.crossplane.io",
						Version: "v1",
						Kind:    "CoolComposite",
					}))
					xr.SetDeletionPolicy(v1.CompositionDeletionPolicyPipeline)

					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{{
								Step:        "run-cool-function",
								FunctionRef: &v1.FunctionReference{Name: "cool-function"},
							}},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{},
				},
			},
		},
		"DeletingPipelineStopsDesiringResources": {
			reason: "When the XR is being deleted we should garbage collect the composed resources the pipeline no longer desires",
			params: params{
				c: &test.MockClient{
					MockList:        mockListFunctionRevisions(),
					MockGet:         mockGetFunction(test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))),
					MockPatch:       test.NewMockPatchFn(nil),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					// A function that handles deletion. It saw the XR's deletion
					// timestamp, and stopped desiring its composed resource.
					return &fnv1.RunFunctionResponse{
						Desired: &fnv1.State{},
					}, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return ComposedResourceStates{
							"observed-resource-a": ComposedResourceState{
								Resource: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "observed-resource-a", OwnerReferences: []metav1.OwnerReference{{Controller: new(true)}}}},
							},
						}, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, observed, desired ComposedResourceStates) error {
						if _, ok := observed["observed-resource-a"]; !ok {
							return errors.New("observed-resource-a should be observed")
						}
						if len(desired) != 0 {
							return errors.New("nothing should be desired")
						}
						return nil
					})),
				},
			},
			args: args{
				xr: composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
					Group:   "test.crossplane.io",
					Version: "v1",
					Kind:    "CoolComposite",
				})),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{{
								Step:        "run-cool-function",
								FunctionRef: &v1.FunctionReference{Name: "cool-function"},
							}},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{},
					Existing: 1,
				},
			},
		},
		"ResourceReferencesWithoutObservedResources": {
			reason: "When XR has resourceRefs but the actual resources don't exist, the function should use a deterministic name (same as resourceRefs).",
			params: params{
				c: &test.MockClient{
					MockList: mockListFunctionRevisions(),
					MockGet:  mockGetFunction(test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "Deployment"}, ""))), // all names are available
					MockPatch: test.NewMockPatchFn(nil, func(obj client.Object) error {
						// Check if the composed resource uses the expected name from resourceRefs
						if cd, ok := obj.(*composed.Unstructured); ok {
							// This test demonstrates the bug: the composed resource should use "existing-deployment-name" from resourceRefs,
							// but currently it generates a new name instead
							if cd.GetName() != "parent-xr-e5ac98dc40a2" {
								// This is the current buggy behavior - it generates a new name instead of using the existing one
								// Log this for debugging when we run the test
								return errors.Errorf("BUG: Composed resource generated new name %s instead of using existing name from resourceRefs: parent-xr-e5ac98dc40a2", cd.GetName())
							}
						}
						return nil
					}),
					MockStatusPatch: test.NewMockSubResourcePatchFn(nil),
				},
				uc: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "")),
				},
				r: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					// Function returns a desired resource with the same name as referenced
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{Ttl: durationpb.New(5 * time.Minute)},
						Desired: &fnv1.State{
							Resources: map[string]*fnv1.Resource{
								"test-resource-with-a-super-duper-really-long-name-to-test-compaction": {
									Resource: MustStruct(map[string]any{
										"apiVersion": "apps/v1",
										"kind":       "Deployment",
										"metadata":   map[string]any{},
										"spec": map[string]any{
											"replicas": 1,
										},
									}),
								},
							},
						},
					}
					return rsp, nil
				}),
				o: []FunctionComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						// Return empty observed resources - simulating that the resources don't exist in cluster
						return ComposedResourceStates{}, nil
					})),
					WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
						return nil
					})),
				},
			},
			args: args{
				xr: func() *composite.Unstructured {
					xr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "test.crossplane.io",
						Version: "v1",
						Kind:    "CoolComposite",
					}))
					xr.SetLabels(map[string]string{
						xcrd.LabelKeyNamePrefixForComposed: "parent-xr",
					})
					xr.SetUID("75e4a668-035f-4ce8-8c45-f4d3ac850155")
					// Set resource references that exist from a previous reconciliation
					xr.SetResourceReferences([]corev1.ObjectReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "parent-xr-e5ac98dc40a2",
						},
					})
					return xr
				}(),
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{
									Step:        "run-cool-function",
									FunctionRef: &v1.FunctionReference{Name: "cool-function"},
								},
							},
						},
					},
				},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{
						{ResourceName: "test-resource-with-a-super-duper-really-long-name-to-test-compaction", Synced: true},
					},
					TTL: 5 * time.Minute,
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewFunctionComposer(tc.params.c, tc.params.uc, tc.params.r, tc.params.o...)

			res, err := c.Compose(tc.args.ctx, tc.args.xr, tc.args.req)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}
			// Check for our typed errors.
			if tc.want.err != nil {
				if wantErr := new(ComposedResourceError); errors.As(tc.want.err, wantErr) {
					if gotErr := new(ComposedResourceError); errors.As(err, gotErr) {
						if diff := cmp.Diff(wantErr, gotErr, test.EquateErrors()); diff != "" {
							t.Errorf("\n%s\nComposedResourceError: -want, +got:\n%s", tc.reason, diff)
						}
					} else {
						t.Errorf("\n%s\nComposedResourceError: not a typed error:\n%T", tc.reason, err)
					}
				}
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
		c  client.Reader
		uc client.Reader
		f  ConnectionDetailsFetcher
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
				uc: &test.MockClient{
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
				uc: &test.MockClient{
					// We should continue past the unnamed reference and not hit
					// this error.
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
				uc: &test.MockClient{
					// We should continue past the unnamed reference and not hit
					// this error.
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
							Controller: new(true),
						})

						return nil
					}),
				},
				uc: &test.MockClient{
					// We should continue past the unnamed reference and not hit
					// this error.
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
				uc: &test.MockClient{
					// We should continue past the unnamed reference and not hit
					// this error.
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
				err: errors.New(errAnonymousCD),
			},
		},
		"FetchConnectionDetailsError": {
			reason: "We should return an error if we can't fetch composed resource connection details.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource-42")
						xcrd.SetCompositionResourceName(obj, "cool-resource")
						return nil
					}),
				},
				f: ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return nil, errBoom
				}),
				uc: &test.MockClient{
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
		"ComposedResourceNotFoundInCache": {
			reason: "We should use the no-cache client for resources that are not found in the cache.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
				uc: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource-42")
						xcrd.SetCompositionResourceName(obj, "cool-resource")
						return nil
					}),
				},
				f: ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
						Resource: func() *composed.Unstructured {
							cd := composed.New()
							cd.SetAPIVersion("example.org/v1")
							cd.SetKind("Composed")
							cd.SetName("cool-resource-42")
							xcrd.SetCompositionResourceName(cd, "cool-resource")
							return cd
						}(),
					},
				},
			},
		},
		"Success": {
			reason: "We should return any composed resources and their connection details.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource-42")
						xcrd.SetCompositionResourceName(obj, "cool-resource")
						return nil
					}),
				},
				uc: &test.MockClient{
					// We should continue past the unnamed reference and not hit
					// this error.
					MockGet: test.NewMockGetFn(errBoom),
				},
				f: ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
						Resource: func() *composed.Unstructured {
							cd := composed.New()
							cd.SetAPIVersion("example.org/v1")
							cd.SetKind("Composed")
							cd.SetName("cool-resource-42")
							xcrd.SetCompositionResourceName(cd, "cool-resource")
							return cd
						}(),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewExistingComposedResourceObserver(tc.params.c, tc.params.uc, tc.params.f)

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
		client                     client.Writer
		functionControlledDeletion bool
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
								Controller: new(true),
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
		"OrphanedResourceNotDeleted": {
			reason: "A referenced resource with no controller reference is not one we can prove we composed, so we must not delete it - even though it's observed and undesired.",
			params: params{
				client: &test.MockClient{
					// Update and Delete are nil functions and would panic if
					// called, proving we neither cleaned up labels nor deleted.
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
							// This resource has no controller reference. It could
							// be an arbitrary resource smuggled into the XR's
							// spec.resourceRefs, so we leave it alone.
							Name: "orphan",
						},
					}},
				},
			},
			want: want{
				err: nil,
			},
		},
		"ForegroundDeletionByDefault": {
			reason: "We should delete a composed resource with foreground propagation, so that a nested XR's own composed resources are deleted before it is.",
			params: params{
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockDelete: func(_ context.Context, _ client.Object, opts ...client.DeleteOption) error {
						do := &client.DeleteOptions{}
						for _, o := range opts {
							o.ApplyToDelete(do)
						}
						if do.PropagationPolicy == nil || *do.PropagationPolicy != metav1.DeletePropagationForeground {
							return errors.Errorf("want foreground propagation, got %v", do.PropagationPolicy)
						}

						return nil
					},
				},
			},
			args: args{
				owner: &fake.Composite{ObjectMeta: metav1.ObjectMeta{UID: "cool-xr"}},
				observed: ComposedResourceStates{
					"undesired-resource": ComposedResourceState{Resource: &fake.Composed{
						ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
							Controller: new(true),
							UID:        "cool-xr",
						}}},
					}},
				},
			},
			want: want{err: nil},
		},
		"DontDeleteResourcesBeingDeleted": {
			reason: "We shouldn't delete a composed resource that's already being deleted when deletion policies are enabled. We keep referencing it until it's gone, and deleting it again would trigger another reconcile.",
			params: params{
				functionControlledDeletion: true,
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errors.New("shouldn't update a resource that's being deleted")),
					MockDelete: test.NewMockDeleteFn(errors.New("shouldn't delete a resource that's being deleted")),
				},
			},
			args: args{
				owner: &fake.Composite{ObjectMeta: metav1.ObjectMeta{UID: "cool-xr"}},
				observed: ComposedResourceStates{
					"deleting-resource": ComposedResourceState{Resource: &fake.Composed{
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &metav1.Time{},
							OwnerReferences: []metav1.OwnerReference{{
								Controller: new(true),
								UID:        "cool-xr",
							}},
						},
					}},
				},
			},
			want: want{err: nil},
		},
		"BackgroundDeletionForXRsThatDeleteTheirOwnResources": {
			reason: "With function-controlled deletion, we should delete a composed XR that deletes its own composed resources with background propagation, so Kubernetes garbage collection doesn't fight it.",
			params: params{
				functionControlledDeletion: true,
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockDelete: func(_ context.Context, _ client.Object, opts ...client.DeleteOption) error {
						do := &client.DeleteOptions{}
						for _, o := range opts {
							o.ApplyToDelete(do)
						}
						if do.PropagationPolicy == nil || *do.PropagationPolicy != metav1.DeletePropagationBackground {
							return errors.Errorf("want background propagation, got %v", do.PropagationPolicy)
						}

						return nil
					},
				},
			},
			args: args{
				owner: &fake.Composite{ObjectMeta: metav1.ObjectMeta{UID: "cool-xr"}},
				observed: ComposedResourceStates{
					"undesired-resource": ComposedResourceState{Resource: func() *composed.Unstructured {
						cd := composed.New()
						cd.SetOwnerReferences([]metav1.OwnerReference{{
							Controller: new(true),
							UID:        "cool-xr",
						}})
						// This composed resource is an XR whose deletion policy is
						// Pipeline.
						_ = fieldpath.Pave(cd.Object).SetValue("status.crossplane.deletionPolicy", "Pipeline")
						return cd
					}()},
				},
			},
			want: want{err: nil},
		},
		"ForegroundDeletionWhenPoliciesDisabled": {
			reason: "A deletion policy recorded while deletion policies were enabled means nothing once they're disabled, so we should keep foreground propagation.",
			params: params{
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockDelete: func(_ context.Context, _ client.Object, opts ...client.DeleteOption) error {
						do := &client.DeleteOptions{}
						for _, o := range opts {
							o.ApplyToDelete(do)
						}
						if do.PropagationPolicy == nil || *do.PropagationPolicy != metav1.DeletePropagationForeground {
							return errors.Errorf("want foreground propagation, got %v", do.PropagationPolicy)
						}

						return nil
					},
				},
			},
			args: args{
				owner: &fake.Composite{ObjectMeta: metav1.ObjectMeta{UID: "cool-xr"}},
				observed: ComposedResourceStates{
					"undesired-resource": ComposedResourceState{Resource: func() *composed.Unstructured {
						cd := composed.New()
						cd.SetOwnerReferences([]metav1.OwnerReference{{
							Controller: new(true),
							UID:        "cool-xr",
						}})
						// This composed resource is an XR whose deletion policy is
						// Pipeline.
						_ = fieldpath.Pave(cd.Object).SetValue("status.crossplane.deletionPolicy", "Pipeline")
						return cd
					}()},
				},
			},
			want: want{err: nil},
		},
		"ForegroundDeletionForBackgroundXRs": {
			reason: "A composed XR whose deletion policy is Background leaves its composed resources to Kubernetes, so we should keep foreground propagation to delete them bottom up.",
			params: params{
				functionControlledDeletion: true,
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockDelete: func(_ context.Context, _ client.Object, opts ...client.DeleteOption) error {
						do := &client.DeleteOptions{}
						for _, o := range opts {
							o.ApplyToDelete(do)
						}
						if do.PropagationPolicy == nil || *do.PropagationPolicy != metav1.DeletePropagationForeground {
							return errors.Errorf("want foreground propagation, got %v", do.PropagationPolicy)
						}

						return nil
					},
				},
			},
			args: args{
				owner: &fake.Composite{ObjectMeta: metav1.ObjectMeta{UID: "cool-xr"}},
				observed: ComposedResourceStates{
					"undesired-resource": ComposedResourceState{Resource: func() *composed.Unstructured {
						cd := composed.New()
						cd.SetOwnerReferences([]metav1.OwnerReference{{
							Controller: new(true),
							UID:        "cool-xr",
						}})
						// This composed resource is an XR whose deletion policy is
						// Background.
						_ = fieldpath.Pave(cd.Object).SetValue("status.crossplane.deletionPolicy", "Background")
						return cd
					}()},
				},
			},
			want: want{err: nil},
		},
		"UpdateError": {
			reason: "We should return any error encountered updating the resource with removed labels.",
			params: params{
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errBoom),
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
									Controller: new(true),
									UID:        "cool-xr",
								}},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtCleanupLabelsCD, "undesired-resource", "", ""),
			},
		},
		"DeleteError": {
			reason: "We should return any error encountered deleting the resource.",
			params: params{
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
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
									Controller: new(true),
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
					MockUpdate: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
						l := obj.GetLabels()
						if l[xcrd.CategoryComposite] != "" || l[xcrd.LabelKeyClaimName] != "" || l[xcrd.LabelKeyClaimNamespace] != "" {
							return errors.New("resource still has composed resource labels")
						}
						return nil
					},
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
									Controller: new(true),
									UID:        "cool-xr",
								}},
								// With composed resource labels.
								Labels: map[string]string{
									xcrd.LabelKeyNamePrefixForComposed: "cool-xr",
									xcrd.LabelKeyClaimName:             "cool-claim",
									xcrd.LabelKeyClaimNamespace:        "cool-namespace",
								},
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
									Controller: new(true),
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
			d := NewDeletingComposedResourceGarbageCollector(tc.params.client, WithDeletionPolicies(tc.params.functionControlledDeletion))

			err := d.GarbageCollectComposedResources(tc.args.ctx, tc.args.owner, tc.args.observed, tc.args.desired)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGarbageCollectComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestUpdateResourceRefs(t *testing.T) {
	type args struct {
		xr  resource.Composite
		drs ComposedResourceStates
	}

	type want struct {
		xr resource.Composite
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ClusterScopedXR": {
			reason: "We should return a consistently ordered set of references (including namespaces where applicable) suitable for a cluster scoped XR.",
			args: args{
				xr: &fake.Composite{},
				drs: ComposedResourceStates{
					"never-created-c": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "c",
								Name:      "never-created-c-42",
							},
						},
					},
					"never-created-b": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "b",
								Name:      "never-created-b-42",
							},
						},
					},
					"never-created-b-ns-a": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "a",
								Name:      "never-created-b-42",
							},
						},
					},
					"never-created-a": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								// No namespace - cluster scoped.
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
							{Namespace: "a", Name: "never-created-b-42"},
							{Namespace: "b", Name: "never-created-b-42"},
							{Namespace: "c", Name: "never-created-c-42"},
						},
					},
				},
			},
		},
		"NamespacedXR": {
			reason: "We should return a consistently ordered set of references (without namespaces) suitable for a namespaced XR.",
			args: args{
				xr: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						// We take the presence of a namespace to mean the XR
						// is namespaced.
						Namespace: "default",
					},
				},
				drs: ComposedResourceStates{
					"never-created-c": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "default",
								Name:      "never-created-c-42",
							},
						},
					},
					"never-created-b": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "default",
								Name:      "never-created-b-42",
							},
						},
					},
					"never-created-a": ComposedResourceState{
						Resource: &fake.Composed{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "default",
								Name:      "never-created-a-42",
							},
						},
					},
				},
			},
			want: want{
				xr: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							// The refs shouldn't have namespaces. They're
							// assumed to be to resource in the same namespace
							// as the XR.
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

func TestPipelineFatalErrorError(t *testing.T) {
	// PipelineFatalError.Error() must produce the same string as the
	// PipelineFatalErrorFmt format, byte-identical, so reconciler events
	// and condition messages remain unchanged.
	cases := map[string]struct {
		err  *PipelineFatalError
		want string
	}{
		"PopulatedFields": {
			err:  &PipelineFatalError{Step: "fetch-extras", Message: "Required extra resource \"namedClusterRole\" not found"},
			want: fmt.Sprintf(PipelineFatalErrorFmt, "fetch-extras", "Required extra resource \"namedClusterRole\" not found"),
		},
		"EmptyFields": {
			err:  &PipelineFatalError{},
			want: fmt.Sprintf(PipelineFatalErrorFmt, "", ""),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, tc.err.Error()); diff != "" {
				t.Errorf("PipelineFatalError.Error(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestPipelineFatalErrorAs(t *testing.T) {
	// errors.As traverses errors.Wrap chains.
	pfe := &PipelineFatalError{Step: "s", Message: "m"}
	wrapped := errors.Wrap(pfe, "cannot compose resources")

	var got *PipelineFatalError
	if !errors.As(wrapped, &got) {
		t.Fatalf("errors.As did not find *PipelineFatalError in wrapped chain")
	}
	if diff := cmp.Diff(pfe, got); diff != "" {
		t.Errorf("errors.As recovered PipelineFatalError: -want, +got:\n%s", diff)
	}
}

func TestReferenced(t *testing.T) {
	ctrl := []metav1.OwnerReference{{Controller: new(true)}}

	desired := &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "desired"}}
	deleting := &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "deleting", OwnerReferences: ctrl}}
	orphaned := &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "orphaned"}}

	type args struct {
		desired  ComposedResourceStates
		observed ComposedResourceStates
		retain   bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   []ResourceName
	}{
		"DontRetain": {
			reason: "We should only reference desired resources when we're not retaining references.",
			args: args{
				desired:  ComposedResourceStates{"desired": {Resource: desired}},
				observed: ComposedResourceStates{"deleting": {Resource: deleting}},
				retain:   false,
			},
			want: []ResourceName{"desired"},
		},
		"RetainControlled": {
			reason: "We should keep referencing a controlled resource that still exists after we stop desiring it.",
			args: args{
				desired:  ComposedResourceStates{"desired": {Resource: desired}},
				observed: ComposedResourceStates{"deleting": {Resource: deleting}},
				retain:   true,
			},
			want: []ResourceName{"deleting", "desired"},
		},
		"DontRetainUncontrolled": {
			reason: "We should stop referencing a resource without a controller reference, because we'll never delete it.",
			args: args{
				desired:  ComposedResourceStates{"desired": {Resource: desired}},
				observed: ComposedResourceStates{"orphaned": {Resource: orphaned}},
				retain:   true,
			},
			want: []ResourceName{"desired"},
		},
		"KeepDesiredUncontrolled": {
			reason: "We should keep referencing a desired resource, even if it doesn't have a controller reference yet.",
			args: args{
				desired:  ComposedResourceStates{"orphaned": {Resource: orphaned}},
				observed: ComposedResourceStates{"orphaned": {Resource: orphaned}},
				retain:   true,
			},
			want: []ResourceName{"orphaned"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := referenced(tc.args.desired, tc.args.observed, tc.args.retain)

			names := make([]ResourceName, 0, len(got))
			for n := range got {
				names = append(names, n)
			}

			if diff := cmp.Diff(tc.want, names, cmpopts.SortSlices(func(a, b ResourceName) bool { return a < b })); diff != "" {
				t.Errorf("\n%s\nreferenced(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
