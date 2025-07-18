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
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
				opts: []ReconcilerOption{
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return nil
					})),
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
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return nil
					})),
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
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return nil
					})),
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
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return nil
					})),
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
		"CapabilityCheckError": {
			reason: "We should increment failures and return an error if a function doesn't have the required operation capability",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "check-caps",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "function-missing-caps",
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
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return errors.New("boom")
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"BootstrapRequirementsFetchError": {
			reason: "We should return an error if we can't fetch bootstrap requirements",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							op := &v1alpha1.Operation{
								Spec: v1alpha1.OperationSpec{
									Pipeline: []v1alpha1.PipelineStep{
										{
											Step: "requires-resources",
											FunctionRef: v1alpha1.FunctionReference{
												Name: "function-cool",
											},
											Requirements: &v1alpha1.FunctionRequirements{
												RequiredResources: []v1alpha1.RequiredResourceSelector{
													{
														RequirementName: "test-resources",
														APIVersion:      "v1",
														Kind:            "ConfigMap",
														Name:            ptr.To("missing-configmap"),
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
				opts: []ReconcilerOption{
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return nil
					})),
					WithRequiredResourcesFetcher(xfn.RequiredResourcesFetcherFn(func(_ context.Context, _ *fnv1.ResourceSelector) (*fnv1.Resources, error) {
						return nil, errors.New("boom")
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
					WithCapabilityChecker(xfn.CapabilityCheckerFn(func(_ context.Context, _ []string, _ ...string) error {
						return nil
					})),
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

func MustUnstructJSON(j string) *kunstructured.Unstructured {
	u := &kunstructured.Unstructured{}
	if err := json.Unmarshal([]byte(j), u); err != nil {
		panic(err)
	}
	return u
}

func TestAddPipelineStepOutput(t *testing.T) {
	type args struct {
		pipeline []v1alpha1.PipelineStepStatus
		step     string
		output   *runtime.RawExtension
	}

	type want struct {
		pipeline []v1alpha1.PipelineStepStatus
	}

	output1 := &runtime.RawExtension{Raw: []byte(`{"key": "value1"}`)}
	output2 := &runtime.RawExtension{Raw: []byte(`{"key": "value2"}`)}
	outputUpdated := &runtime.RawExtension{Raw: []byte(`{"key": "updated"}`)}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AddToEmptyPipeline": {
			reason: "Should add step to empty pipeline",
			args: args{
				pipeline: []v1alpha1.PipelineStepStatus{},
				step:     "step1",
				output:   output1,
			},
			want: want{
				pipeline: []v1alpha1.PipelineStepStatus{
					{Step: "step1", Output: output1},
				},
			},
		},
		"AddNewStep": {
			reason: "Should add new step to existing pipeline",
			args: args{
				pipeline: []v1alpha1.PipelineStepStatus{
					{Step: "step1", Output: output1},
				},
				step:   "step2",
				output: output2,
			},
			want: want{
				pipeline: []v1alpha1.PipelineStepStatus{
					{Step: "step1", Output: output1},
					{Step: "step2", Output: output2},
				},
			},
		},
		"UpdateExistingStep": {
			reason: "Should update existing step output in place",
			args: args{
				pipeline: []v1alpha1.PipelineStepStatus{
					{Step: "step1", Output: output1},
					{Step: "step2", Output: output2},
				},
				step:   "step1",
				output: outputUpdated,
			},
			want: want{
				pipeline: []v1alpha1.PipelineStepStatus{
					{Step: "step1", Output: outputUpdated},
					{Step: "step2", Output: output2},
				},
			},
		},
		"UpdateMiddleStep": {
			reason: "Should update step in middle of pipeline",
			args: args{
				pipeline: []v1alpha1.PipelineStepStatus{
					{Step: "step1", Output: output1},
					{Step: "step2", Output: output2},
					{Step: "step3", Output: output1},
				},
				step:   "step2",
				output: outputUpdated,
			},
			want: want{
				pipeline: []v1alpha1.PipelineStepStatus{
					{Step: "step1", Output: output1},
					{Step: "step2", Output: outputUpdated},
					{Step: "step3", Output: output1},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := AddPipelineStepOutput(tc.args.pipeline, tc.args.step, tc.args.output)
			if diff := cmp.Diff(tc.want.pipeline, got); diff != "" {
				t.Errorf("\n%s\nAddPipelineStepOutput(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestToProtobufResourceSelector(t *testing.T) {
	type args struct {
		selector v1alpha1.RequiredResourceSelector
	}

	type want struct {
		selector *fnv1.ResourceSelector
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BasicSelector": {
			reason: "Should convert basic API version and kind",
			args: args{
				selector: v1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Pod",
				},
			},
			want: want{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
				},
			},
		},
		"SelectorWithName": {
			reason: "Should convert selector with specific name",
			args: args{
				selector: v1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Secret",
					Name:            ptr.To("test-secret"),
				},
			},
			want: want{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Secret",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "test-secret",
					},
				},
			},
		},
		"SelectorWithLabels": {
			reason: "Should convert selector with match labels",
			args: args{
				selector: v1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Pod",
					MatchLabels: map[string]string{
						"app": "test",
						"env": "prod",
					},
				},
			},
			want: want{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{
								"app": "test",
								"env": "prod",
							},
						},
					},
				},
			},
		},
		"SelectorWithNamespace": {
			reason: "Should convert namespaced selector",
			args: args{
				selector: v1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Namespace:       ptr.To("default"),
				},
			},
			want: want{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  ptr.To("default"),
				},
			},
		},
		"SelectorWithNameAndNamespace": {
			reason: "Should convert selector with both name and namespace",
			args: args{
				selector: v1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Secret",
					Name:            ptr.To("my-secret"),
					Namespace:       ptr.To("kube-system"),
				},
			},
			want: want{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Secret",
					Namespace:  ptr.To("kube-system"),
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "my-secret",
					},
				},
			},
		},
		"SelectorWithLabelsAndNamespace": {
			reason: "Should convert selector with labels and namespace",
			args: args{
				selector: v1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "apps/v1",
					Kind:            "Deployment",
					MatchLabels: map[string]string{
						"tier": "frontend",
					},
					Namespace: ptr.To("production"),
				},
			},
			want: want{
				selector: &fnv1.ResourceSelector{
					ApiVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  ptr.To("production"),
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{
								"tier": "frontend",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ToProtobufResourceSelector(tc.args.selector)
			if diff := cmp.Diff(tc.want.selector, got, cmpopts.IgnoreUnexported(
				fnv1.ResourceSelector{},
				fnv1.ResourceSelector_MatchName{},
				fnv1.ResourceSelector_MatchLabels{},
				fnv1.MatchLabels{},
			)); diff != "" {
				t.Errorf("\n%s\nToProtobufResourceSelector(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAddResourceRef(t *testing.T) {
	type args struct {
		refs []v1alpha1.AppliedResourceRef
		u    *kunstructured.Unstructured
	}

	type want struct {
		refs []v1alpha1.AppliedResourceRef
	}

	// Test resources
	clusterResource := &kunstructured.Unstructured{}
	clusterResource.SetAPIVersion("example.org/v1")
	clusterResource.SetKind("ClusterResource")
	clusterResource.SetName("cluster-resource")

	namespacedResource := &kunstructured.Unstructured{}
	namespacedResource.SetAPIVersion("example.org/v1")
	namespacedResource.SetKind("NamespacedResource")
	namespacedResource.SetName("namespaced-resource")
	namespacedResource.SetNamespace("default")

	anotherNamespacedResource := &kunstructured.Unstructured{}
	anotherNamespacedResource.SetAPIVersion("example.org/v1")
	anotherNamespacedResource.SetKind("NamespacedResource")
	anotherNamespacedResource.SetName("another-resource")
	anotherNamespacedResource.SetNamespace("other")

	// Expected refs
	clusterRef := v1alpha1.AppliedResourceRef{
		APIVersion: "example.org/v1",
		Kind:       "ClusterResource",
		Name:       "cluster-resource",
	}

	namespacedRef := v1alpha1.AppliedResourceRef{
		APIVersion: "example.org/v1",
		Kind:       "NamespacedResource",
		Name:       "namespaced-resource",
		Namespace:  ptr.To("default"),
	}

	anotherNamespacedRef := v1alpha1.AppliedResourceRef{
		APIVersion: "example.org/v1",
		Kind:       "NamespacedResource",
		Name:       "another-resource",
		Namespace:  ptr.To("other"),
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AddToEmptySlice": {
			reason: "Should add resource to empty slice",
			args: args{
				refs: []v1alpha1.AppliedResourceRef{},
				u:    clusterResource,
			},
			want: want{
				refs: []v1alpha1.AppliedResourceRef{clusterRef},
			},
		},
		"AddClusterScopedResource": {
			reason: "Should add cluster-scoped resource without namespace",
			args: args{
				refs: []v1alpha1.AppliedResourceRef{},
				u:    clusterResource,
			},
			want: want{
				refs: []v1alpha1.AppliedResourceRef{clusterRef},
			},
		},
		"AddNamespacedResource": {
			reason: "Should add namespaced resource with namespace",
			args: args{
				refs: []v1alpha1.AppliedResourceRef{},
				u:    namespacedResource,
			},
			want: want{
				refs: []v1alpha1.AppliedResourceRef{namespacedRef},
			},
		},
		"AddMultipleResourcesSorted": {
			reason: "Should add multiple resources and keep them sorted",
			args: args{
				refs: []v1alpha1.AppliedResourceRef{namespacedRef},
				u:    clusterResource,
			},
			want: want{
				refs: []v1alpha1.AppliedResourceRef{clusterRef, namespacedRef},
			},
		},
		"AddDuplicateResource": {
			reason: "Should not add duplicate resource",
			args: args{
				refs: []v1alpha1.AppliedResourceRef{clusterRef},
				u:    clusterResource,
			},
			want: want{
				refs: []v1alpha1.AppliedResourceRef{clusterRef},
			},
		},
		"AddResourceToSortedSlice": {
			reason: "Should add resource to sorted slice and maintain order",
			args: args{
				refs: []v1alpha1.AppliedResourceRef{clusterRef, namespacedRef},
				u:    anotherNamespacedResource,
			},
			want: want{
				refs: []v1alpha1.AppliedResourceRef{clusterRef, namespacedRef, anotherNamespacedRef},
			},
		},
		"AddDuplicateNamespacedResource": {
			reason: "Should not add duplicate namespaced resource",
			args: args{
				refs: []v1alpha1.AppliedResourceRef{namespacedRef},
				u:    namespacedResource,
			},
			want: want{
				refs: []v1alpha1.AppliedResourceRef{namespacedRef},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := AddResourceRef(tc.args.refs, tc.args.u)
			if diff := cmp.Diff(tc.want.refs, got); diff != "" {
				t.Errorf("\n%s\nAddResourceRef(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
