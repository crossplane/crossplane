package diffprocessor

import (
	"bytes"
	"context"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// Ensure MockDiffProcessor implements the DiffProcessor interface
var _ DiffProcessor = &tu.MockDiffProcessor{}

func TestDiffProcessor_ProcessResource(t *testing.T) {
	pipelineMode := apiextensionsv1.CompositionModePipeline

	// Create mocks with proper behavior
	mockCompositionNotFound := &tu.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("composition not found")
		},
		GetEnvironmentConfigsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{}, nil
		},
	}

	mockExtraResourcesError := &tu.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: &pipelineMode,
				},
			}, nil
		},
		GetEnvironmentConfigsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{}, nil
		},
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
			return nil, errors.New("failed to get extra resources")
		},
	}

	mockGetFunctionsError := &tu.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: &pipelineMode,
				},
			}, nil
		},
		GetEnvironmentConfigsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{}, nil
		},
		GetExtraResourcesFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{}, nil
		},
		GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
			return nil, errors.New("function not found")
		},
	}

	type args struct {
		ctx context.Context
		res *unstructured.Unstructured
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		mock   *tu.MockClusterClient
		args   args
		want   want
	}{
		"CompositionNotFound": {
			reason: "Should return error when matching composition is not found",
			mock:   mockCompositionNotFound,
			args: args{
				ctx: context.Background(),
				res: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "XR1",
						"metadata": map[string]interface{}{
							"name": "my-xr",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.New("composition not found"), "cannot find matching composition"),
			},
		},
		"ExtraResourcesError": {
			reason: "Should return error when identifying needed extra resources fails",
			mock:   mockExtraResourcesError,
			args: args{
				ctx: context.Background(),
				res: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "XR1",
						"metadata": map[string]interface{}{
							"name": "my-xr",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.New("failed to get extra resources"), "cannot get extra resources"),
			},
		},
		"GetFunctionsError": {
			reason: "Should return error when getting functions from pipeline fails",
			mock:   mockGetFunctionsError,
			args: args{
				ctx: context.Background(),
				res: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "XR1",
						"metadata": map[string]interface{}{
							"name": "my-xr",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.New("function not found"), "cannot get functions from pipeline"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor that uses our mock client
			p, _ := NewDiffProcessor(tc.mock, WithRestConfig(&rest.Config{}))

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.ProcessResource(&stdout, tc.args.ctx, tc.args.res)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nProcessResource(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nProcessResource(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nProcessResource(...): unexpected error: %v", tc.reason, err)
			}
		})
	}
}

func TestDiffProcessor_ProcessAll(t *testing.T) {
	// Create mock clients for testing
	mockCompositionNotFound := &tu.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("composition not found")
		},
	}

	mockMultipleErrors := &tu.MockClusterClient{
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.Errorf("composition not found for %s", res.GetName())
		},
	}

	mockNoErrors := &tu.MockClusterClient{
		// Since this test has no resources, these functions shouldn't be called
		FindMatchingCompositionFn: func(res *unstructured.Unstructured) (*apiextensionsv1.Composition, error) {
			return nil, errors.New("should not be called")
		},
	}

	type args struct {
		ctx       context.Context
		resources []*unstructured.Unstructured
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		mock   *tu.MockClusterClient
		args   args
		want   want
	}{
		"NoResources": {
			reason: "Should not return error when no resources are provided",
			mock:   mockNoErrors,
			args: args{
				ctx:       context.Background(),
				resources: []*unstructured.Unstructured{},
			},
			want: want{
				err: nil,
			},
		},
		"ProcessResourceError": {
			reason: "Should return error when processing a resource fails",
			mock:   mockCompositionNotFound,
			args: args{
				ctx: context.Background(),
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
							"metadata": map[string]interface{}{
								"name": "my-xr",
							},
						},
					},
				},
			},
			want: want{
				err: errors.New("unable to process resource my-xr: cannot find matching composition: composition not found"),
			},
		},
		"MultipleResourceErrors": {
			reason: "Should return all errors when multiple resources fail processing",
			mock:   mockMultipleErrors,
			args: args{
				ctx: context.Background(),
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
							"metadata": map[string]interface{}{
								"name": "my-xr-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "XR1",
							"metadata": map[string]interface{}{
								"name": "my-xr-2",
							},
						},
					},
				},
			},
			want: want{
				err: errors.New("[unable to process resource my-xr-1: cannot find matching composition: composition not found for my-xr-1, unable to process resource my-xr-2: cannot find matching composition: composition not found for my-xr-2]"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor with our mock client
			p, _ := NewDiffProcessor(tc.mock, WithRestConfig(&rest.Config{}))

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.ProcessAll(&stdout, tc.args.ctx, tc.args.resources)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nProcessAll(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nProcessAll(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nProcessAll(...): unexpected error: %v", tc.reason, err)
			}
		})
	}
}

func TestDiffProcessor_IdentifyNeededExtraResources(t *testing.T) {
	pipelineMode := apiextensionsv1.CompositionModePipeline
	nonPipelineMode := apiextensionsv1.CompositionMode("NonPipeline")

	// Create a sample XR for field path resolution tests
	sampleXR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "XTest",
			"metadata": map[string]interface{}{
				"name": "test-xr",
			},
			"spec": map[string]interface{}{
				"environment": "production",
				"region":      "us-west-2",
				"nested": map[string]interface{}{
					"value": "nested-value",
				},
			},
		},
	}

	type args struct {
		comp *apiextensionsv1.Composition
		xr   *unstructured.Unstructured
	}

	type want struct {
		gvrs      []schema.GroupVersionResource
		selectors []metav1.LabelSelector
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NonPipelineMode": {
			reason: "Should return empty slices for non-pipeline mode",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &nonPipelineMode,
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
			},
		},
		"NoExtraResourcesFunction": {
			reason: "Should return empty slices when no function-extra-resources exists",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-other"},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
			},
		},
		"NoExtraResourcesInInput": {
			reason: "Should return empty slices when function exists but no extraResources in input",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{"apiVersion":"extra-resources.fn.crossplane.io/v1beta1","kind":"Input","spec":{"otherField":"value"}}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
			},
		},
		"InvalidInput": {
			reason: "Should handle invalid input JSON",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{invalid-json`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
				err:       errors.New("cannot unmarshal function-extra-resources input"),
			},
		},
		"ReferenceTypeSkipped": {
			reason: "Should skip Reference type resources (default)",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testRef",
													"ref": {
														"name": "test-name"
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs:      nil,
				selectors: nil,
			},
		},
		"SelectorTypeWithSimpleValue": {
			reason: "Should process Selector type with simple static value in matchLabels",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "app",
																"type": "Value",
																"value": "test-app"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"app": "test-app"}},
				},
			},
		},
		"SelectorTypeWithFieldPath": {
			reason: "Should process Selector type with field path value in matchLabels",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "env",
																"type": "FromCompositeFieldPath",
																"valueFromFieldPath": "spec.environment"
															},
															{
																"key": "region",
																"type": "FromCompositeFieldPath",
																"valueFromFieldPath": "spec.region"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{
						"env":    "production",
						"region": "us-west-2",
					}},
				},
			},
		},
		"SelectorTypeWithMixedValues": {
			reason: "Should process Selector type with a mix of static and field path values",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "app",
																"type": "Value",
																"value": "test-app"
															},
															{
																"key": "env",
																"type": "FromCompositeFieldPath",
																"valueFromFieldPath": "spec.environment"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{
						"app": "test-app",
						"env": "production",
					}},
				},
			},
		},
		"SelectorTypeWithNestedFieldPath": {
			reason: "Should process Selector type with a nested field path value",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "nested-value",
																"type": "FromCompositeFieldPath",
																"valueFromFieldPath": "spec.nested.value"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{
						"nested-value": "nested-value",
					}},
				},
			},
		},
		"MultipleSelectorTypes": {
			reason: "Should process multiple resources with Selector type",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test1",
													"into": "test1Selector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "app",
																"type": "Value",
																"value": "test-app-1"
															}
														]
													}
												},
												{
													"apiVersion": "example.org/v1",
													"kind": "Test2",
													"into": "test2Selector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "app",
																"type": "Value",
																"value": "test-app-2"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "test1s"},
					{Group: "example.org", Version: "v1", Resource: "test2s"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"app": "test-app-1"}},
					{MatchLabels: map[string]string{"app": "test-app-2"}},
				},
			},
		},
		"MissingLabelKey": {
			reason: "Should skip label with missing key",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"type": "Value",
																"value": "test-app"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{}},
				},
			},
		},
		"MissingValueInStaticValue": {
			reason: "Should skip label with missing value in Value type",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "app",
																"type": "Value"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{}},
				},
			},
		},
		"MissingValueFromFieldPath": {
			reason: "Should skip label with missing valueFromFieldPath in FromCompositeFieldPath type",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "env",
																"type": "FromCompositeFieldPath"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{}},
				},
			},
		},
		"NonExistentFieldPath": {
			reason: "Should skip label when field path doesn't exist in XR",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-extra-resources"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "extra-resources.fn.crossplane.io/v1beta1",
										"kind": "Input",
										"spec": {
											"extraResources": [
												{
													"apiVersion": "example.org/v1",
													"kind": "Test",
													"into": "testSelector",
													"type": "Selector",
													"selector": {
														"matchLabels": [
															{
																"key": "nonexistent",
																"type": "FromCompositeFieldPath",
																"valueFromFieldPath": "spec.nonexistent"
															}
														]
													}
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
				xr: sampleXR,
			},
			want: want{
				gvrs: []schema.GroupVersionResource{
					{Group: "example.org", Version: "v1", Resource: "tests"},
				},
				selectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &DefaultDiffProcessor{}
			gvrs, selectors, err := p.IdentifyNeededExtraResources(tc.args.comp, tc.args.xr)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nIdentifyNeededExtraResources(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nIdentifyNeededExtraResources(...): expected error containing %q, got %q", tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nIdentifyNeededExtraResources(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.gvrs), len(gvrs)); diff != "" {
				t.Errorf("\n%s\nIdentifyNeededExtraResources(...): -want GVR count, +got GVR count:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(len(tc.want.selectors), len(selectors)); diff != "" {
				t.Errorf("\n%s\nIdentifyNeededExtraResources(...): -want selector count, +got selector count:\n%s", tc.reason, diff)
			}

			for i, wantGVR := range tc.want.gvrs {
				if i >= len(gvrs) {
					break
				}
				if diff := cmp.Diff(wantGVR.String(), gvrs[i].String()); diff != "" {
					t.Errorf("\n%s\nIdentifyNeededExtraResources(...): -want GVR, +got GVR at index %d:\n%s", tc.reason, i, diff)
				}
			}

			for i, wantSelector := range tc.want.selectors {
				if i >= len(selectors) {
					break
				}
				if diff := cmp.Diff(wantSelector.MatchLabels, selectors[i].MatchLabels); diff != "" {
					t.Errorf("\n%s\nIdentifyNeededExtraResources(...): -want selector, +got selector at index %d:\n%s", tc.reason, i, diff)
				}
			}
		})
	}
}

func TestScanForTemplatedExtraResources(t *testing.T) {
	pipelineMode := apiextensionsv1.CompositionModePipeline
	nonPipelineMode := apiextensionsv1.CompositionMode("NonPipeline")

	type args struct {
		comp *apiextensionsv1.Composition
	}

	type want struct {
		hasTemplated bool
		err          error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NonPipelineMode": {
			reason: "Should return false for non-pipeline mode compositions",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &nonPipelineMode,
					},
				},
			},
			want: want{
				hasTemplated: false,
			},
		},
		"NoGoTemplatingFunction": {
			reason: "Should return false when no go-templating function exists",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-other"},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: false,
			},
		},
		"TemplateWithoutExtraResources": {
			reason: "Should return false when template doesn't have ExtraResources",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "crossplane.io/v1alpha1",
										"kind": "GoTemplatingInput",
										"spec": {
											"inline": {
												"template": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test"
											}
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: false,
			},
		},
		"TemplateWithExtraResources": {
			reason: "Should return true when template has ExtraResources",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "crossplane.io/v1alpha1",
										"kind": "GoTemplatingInput",
										"spec": {
											"inline": {
												"template": "apiVersion: render.crossplane.io/v1\nkind: ExtraResources\nspec:\n  resources:\n  - apiVersion: v1\n    kind: ConfigMap"
											}
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: true,
			},
		},
		"InvalidTemplate": {
			reason: "Should return error for invalid template",
			args: args{
				comp: &apiextensionsv1.Composition{
					Spec: apiextensionsv1.CompositionSpec{
						Mode: &pipelineMode,
						Pipeline: []apiextensionsv1.PipelineStep{
							{
								Step:        "step-1",
								FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
								Input: &runtime.RawExtension{
									Raw: []byte(`{
										"apiVersion": "crossplane.io/v1alpha1",
										"kind": "GoTemplatingInput",
										"spec": {
											"inline": {
												"template": "{{"
											}
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				hasTemplated: false,
				err:          errors.New("cannot decode template YAML"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ScanForTemplatedExtraResources(tc.args.comp)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nscanForTemplatedExtraResources(...): expected error but got none", tc.reason)
					return
				}

				if !strings.Contains(err.Error(), tc.want.err.Error()) {
					t.Errorf("\n%s\nscanForTemplatedExtraResources(...): expected error containing %q, got %q", tc.reason, tc.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nscanForTemplatedExtraResources(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(tc.want.hasTemplated, got); diff != "" {
				t.Errorf("\n%s\nscanForTemplatedExtraResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetExtraResourcesFromResult(t *testing.T) {
	type args struct {
		result *unstructured.Unstructured
	}

	type want struct {
		resources []unstructured.Unstructured
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoSpec": {
			reason: "Should return error when result has no spec",
			args: args{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "render.crossplane.io/v1beta1",
						"kind":       "ExtraResources",
						"metadata": map[string]interface{}{
							"name": "result",
						},
					},
				},
			},
			want: want{
				err: errors.New("no spec found in ExtraResources result"),
			},
		},
		"NoResources": {
			reason: "Should return error when spec has no resources",
			args: args{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "render.crossplane.io/v1beta1",
						"kind":       "ExtraResources",
						"metadata": map[string]interface{}{
							"name": "result",
						},
						"spec": map[string]interface{}{},
					},
				},
			},
			want: want{
				err: errors.New("no resources found in ExtraResources spec"),
			},
		},
		"WithResources": {
			reason: "Should return resources when they exist",
			args: args{
				result: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "render.crossplane.io/v1beta1",
						"kind":       "ExtraResources",
						"metadata": map[string]interface{}{
							"name": "result",
						},
						"spec": map[string]interface{}{
							"resources": []interface{}{
								map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "ConfigMap",
									"metadata": map[string]interface{}{
										"name": "resource-1",
									},
								},
								map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Secret",
									"metadata": map[string]interface{}{
										"name": "resource-2",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				resources: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name": "resource-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
							"metadata": map[string]interface{}{
								"name": "resource-2",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := GetExtraResourcesFromResult(tc.args.result)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\ngetExtraResourcesFromResult(...): unexpected error: %v", tc.reason, err)
				return
			}

			if diff := cmp.Diff(len(tc.want.resources), len(got)); diff != "" {
				t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want resource count, +got resource count:\n%s", tc.reason, diff)
			}

			for i, wantRes := range tc.want.resources {
				if i >= len(got) {
					break
				}

				if diff := cmp.Diff(wantRes.GetKind(), got[i].GetKind()); diff != "" {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want kind, +got kind at index %d:\n%s", tc.reason, i, diff)
				}

				if diff := cmp.Diff(wantRes.GetName(), got[i].GetName()); diff != "" {
					t.Errorf("\n%s\ngetExtraResourcesFromResult(...): -want name, +got name at index %d:\n%s", tc.reason, i, diff)
				}
			}
		})
	}
}

func TestDefaultDiffProcessor_CalculateDiff(t *testing.T) {
	ctx := context.Background()

	type fields struct {
		mockGetResource         func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error)
		mockDryRunApply         func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
		mockGetResourcesByLabel func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error)
	}

	type args struct {
		ctx       context.Context
		composite string
		desired   runtime.Object
	}

	type want struct {
		diff string
		err  error
	}

	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Non-Unstructured Object": {
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					return nil, nil
				},
				mockGetResourcesByLabel: func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					return nil, nil
				},
			},
			args: args{
				ctx:       ctx,
				composite: "",
				desired:   &corev1.Pod{}, // Using a typed object to test error handling
			},
			want: want{
				err: errors.New("desired object is not unstructured"),
			},
		},
		"New Resource": {
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, name)
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Should not be called for new resources
					return nil, errors.New("should not be called")
				},
				mockGetResourcesByLabel: func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					return nil, nil
				},
			},
			args: args{
				ctx:       ctx,
				composite: "",
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "new-resource",
						},
						"spec": map[string]interface{}{
							"param": "value",
						},
					},
				},
			},
			want: want{
				diff: `+++ ExampleResource/new-resource
` + tu.Green(`+ apiVersion: example.org/v1
+ kind: ExampleResource
+ metadata:
+   name: new-resource
+ spec:
+   param: value`),
			},
		},
		"Error Getting Current Resource": {
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, errors.New("get resource error")
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					return nil, nil
				},
				mockGetResourcesByLabel: func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					return nil, nil
				},
			},
			args: args{
				ctx:       ctx,
				composite: "",
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "error-resource",
						},
					},
				},
			},
			want: want{
				err: errors.New("cannot get current object: get resource error"),
			},
		},
		"Dry Run Apply Error": {
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name": "existing-resource",
							},
							"spec": map[string]interface{}{
								"param": "old-value",
							},
						},
					}, nil
				},
				// This is returning DryRunApply error
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					return nil, errors.New("dry run apply error")
				},
			},
			args: args{
				ctx:       ctx,
				composite: "",
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "existing-resource",
						},
						"spec": map[string]interface{}{
							"param": "new-value",
						},
					},
				},
			},
			want: want{
				err: errors.New("cannot dry-run apply desired object: dry run apply error"),
			},
		},
		"Modified Simple Fields": {
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name": "modified-resource",
							},
							"spec": map[string]interface{}{
								"param1": "old-value1",
								"param2": "old-value2",
								"param3": "unchanged",
							},
						},
					}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return a version that would result from applying the changes
					return obj, nil
				},
				mockGetResourcesByLabel: func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					return nil, nil
				},
			},
			args: args{
				ctx:       ctx,
				composite: "",
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "modified-resource",
						},
						"spec": map[string]interface{}{
							"param1": "new-value1",
							"param2": "new-value2",
							"param3": "unchanged",
							"param4": "added",
						},
					},
				},
			},
			want: want{
				diff: `~~~ ExampleResource/modified-resource
  apiVersion: example.org/v1
  kind: ExampleResource
  metadata:
    name: modified-resource
  spec:
` + tu.Red(`-   param1: old-value1
-   param2: old-value2
-   param3: unchanged`) + `
` + tu.Green(`+   param1: new-value1
+   param2: new-value2
+   param3: unchanged
+   param4: added`),
			},
		},
		"Nested Fields": {
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name": "nested-resource",
							},
							"spec": map[string]interface{}{
								"simple": "unchanged",
								"nested": map[string]interface{}{
									"field1": "old-nested-value",
									"field2": map[string]interface{}{
										"deepField": "old-deep-value",
									},
								},
							},
						},
					}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return a version that would result from applying the changes
					return obj, nil
				},
				mockGetResourcesByLabel: func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					return nil, nil
				},
			},
			args: args{
				ctx:       ctx,
				composite: "",
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "nested-resource",
						},
						"spec": map[string]interface{}{
							"simple": "unchanged",
							"nested": map[string]interface{}{
								"field1": "new-nested-value",
								"field2": map[string]interface{}{
									"deepField":      "new-deep-value",
									"addedDeepField": "added-deep-value",
								},
								"field3": "added-field",
							},
						},
					},
				},
			},
			want: want{
				diff: `~~~ ExampleResource/nested-resource
  apiVersion: example.org/v1
  kind: ExampleResource
  metadata:
    name: nested-resource
  spec:
    nested:
` + tu.Red(`-     field1: old-nested-value
-     field2:
-       deepField: old-deep-value`) + `
` + tu.Green(`+     field1: new-nested-value
+     field2:
+       addedDeepField: added-deep-value
+       deepField: new-deep-value
+     field3: added-field`) + `
    simple: unchanged`,
			},
		},
		"Array Fields": {
			fields: fields{
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "ExampleResource",
							"metadata": map[string]interface{}{
								"name": "array-resource",
							},
							"spec": map[string]interface{}{
								"items": []interface{}{
									map[string]interface{}{
										"name":  "item1",
										"value": "value1",
									},
									map[string]interface{}{
										"name":  "item2",
										"value": "value2",
									},
								},
							},
						},
					}, nil
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					// Return a version that would result from applying the changes
					return obj, nil
				},
				mockGetResourcesByLabel: func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					return nil, nil
				},
			},
			args: args{
				ctx:       ctx,
				composite: "",
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ExampleResource",
						"metadata": map[string]interface{}{
							"name": "array-resource",
						},
						"spec": map[string]interface{}{
							"items": []interface{}{
								map[string]interface{}{
									"name":  "item1",
									"value": "modified1",
								},
								map[string]interface{}{
									"name":  "item3",
									"value": "value3",
								},
							},
						},
					},
				},
			},
			want: want{
				diff: `~~~ ExampleResource/array-resource
  apiVersion: example.org/v1
  kind: ExampleResource
  metadata:
    name: array-resource
  spec:
    items:
    - name: item1
` + tu.Red(`-     value: value1
-   - name: item2
-     value: value2`) + `
` + tu.Green(`+     value: modified1
+   - name: item3
+     value: value3`),
			},
		},
		"Composed Resource": {
			fields: fields{
				// Fix the mock for composed resources
				mockGetResource: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					// For composed resources, we simulate a resource that exists but has different values
					if name == "composed-resource" {
						return &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "example.org/v1",
								"kind":       "ComposedResource",
								"metadata": map[string]interface{}{
									"name": "composed-resource",
									"labels": map[string]interface{}{
										"crossplane.io/composite": "parent-xr",
									},
									"annotations": map[string]interface{}{
										"crossplane.io/composition-resource-name": "resource-a",
									},
								},
								"spec": map[string]interface{}{
									"param": "old-value",
								},
							},
						}, nil
					}
					// For GetResourcesByLabel fallback (if implemented in DefaultDiffProcessor)
					return nil, apierrors.NewNotFound(
						schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, name)
				},
				mockDryRunApply: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
					return obj, nil
				},
				mockGetResourcesByLabel: func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					// For composed resources with composite reference
					compositeLabel, hasCompositeLabel := selector.MatchLabels["crossplane.io/composite"]
					if hasCompositeLabel && compositeLabel == "parent-xr" {
						// Mock data for testing composed resources by label
						return []*unstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": "example.org/v1",
									"kind":       "ComposedResource",
									"metadata": map[string]interface{}{
										"name": "composed-resource",
										"labels": map[string]interface{}{
											"crossplane.io/composite": compositeLabel,
										},
										"annotations": map[string]interface{}{
											"crossplane.io/composition-resource-name": "resource-a",
										},
									},
									"spec": map[string]interface{}{
										"param": "old-value",
									},
								},
							},
						}, nil
					}
					return nil, nil
				},
			},
			args: args{
				ctx:       ctx,
				composite: "parent-xr", // This indicates it's a composed resource
				desired: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "example.org/v1",
						"kind":       "ComposedResource",
						"metadata": map[string]interface{}{
							"name": "composed-resource",
							"labels": map[string]interface{}{
								"crossplane.io/composite": "parent-xr",
							},
							"annotations": map[string]interface{}{
								"crossplane.io/composition-resource-name": "resource-a",
							},
						},
						"spec": map[string]interface{}{
							"param": "new-value",
						},
					},
				},
			},
			want: want{
				diff: `~~~ ComposedResource/composed-resource
  apiVersion: example.org/v1
  kind: ComposedResource
  metadata:
    annotations:
      crossplane.io/composition-resource-name: resource-a
    labels:
      crossplane.io/composite: parent-xr
    name: composed-resource
  spec:
` + tu.Red(`-   param: old-value`) + `
` + tu.Green(`+   param: new-value`),
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a mock client
			mockClient := &tu.MockClusterClient{
				GetResourceFn:         tt.fields.mockGetResource,
				DryRunApplyFn:         tt.fields.mockDryRunApply,
				GetResourcesByLabelFn: tt.fields.mockGetResourcesByLabel,
			}

			config := ProcessorConfig{
				RestConfig: &rest.Config{},
				Colorize:   true,
			}

			p := &DefaultDiffProcessor{
				client: mockClient,
				config: config,
			}

			diff, err := p.CalculateDiff(tt.args.ctx, tt.args.composite, tt.args.desired)

			// Check error expectations
			if tt.want.err != nil {
				if err == nil {
					t.Errorf("CalculateDiff() expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.want.err.Error()) {
					t.Errorf("CalculateDiff() error = %v, wantErr %v", err, tt.want.err)
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateDiff() unexpected error: %v", err)
				return
			}

			// For empty expected diff, verify the result is also empty
			if tt.want.diff == "" && diff != "" {
				t.Errorf("CalculateDiff() expected empty diff, got: %s", diff)
				return
			}

			// Check if we need to compare expected diff with actual diff
			if tt.want.diff != "" && err == nil {
				// Normalize both strings by trimming trailing whitespace from each line
				normalizedExpected := normalizeTrailingWhitespace(tt.want.diff)
				normalizedActual := normalizeTrailingWhitespace(diff)

				// Direct string comparison with normalized strings
				if normalizedExpected != normalizedActual {
					// If they're equal when ignoring ANSI, show escaped ANSI for debugging
					if tu.CompareIgnoringAnsi(tt.want.diff, diff) {
						t.Errorf("CalculateDiff() diff matches content but ANSI codes differ.\nWant (escaped):\n%q\n\nGot (escaped):\n%q",
							tt.want.diff, diff)
					} else {
						t.Errorf("CalculateDiff() diff does not match expected.\nWant:\n%s\n\nGot:\n%s",
							tt.want.diff, diff)
					}
				}
			}
		})
	}
}

func TestDiffProcessor_Initialize(t *testing.T) {
	// Create a mock client that returns an error for GetXRDs
	mockXRDsError := &tu.MockClusterClient{
		GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return nil, errors.New("XRD not found")
		},
	}

	// Create a mock client that returns success for GetXRDs
	mockXRDsSuccess := &tu.MockClusterClient{
		GetXRDsFn: func(ctx context.Context) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "apiextensions.crossplane.io/v1",
						"kind":       "CompositeResourceDefinition",
						"metadata": map[string]interface{}{
							"name": "xexampleresources.example.org",
						},
						"spec": map[string]interface{}{
							"group": "example.org",
							"names": map[string]interface{}{
								"kind":     "XExampleResource",
								"plural":   "xexampleresources",
								"singular": "xexampleresource",
							},
							"versions": []interface{}{
								map[string]interface{}{
									"name":    "v1",
									"served":  true,
									"storage": true,
									"schema": map[string]interface{}{
										"openAPIV3Schema": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"spec": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"coolParam": map[string]interface{}{
															"type": "string",
														},
													},
												},
												"status": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"coolStatus": map[string]interface{}{
															"type": "string",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	type args struct {
		ctx context.Context
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		mock   *tu.MockClusterClient
		args   args
		want   want
	}{
		"XRDsError": {
			reason: "Should return error when getting XRD schema fails",
			mock:   mockXRDsError,
			args: args{
				ctx: context.Background(),
			},
			want: want{
				err: errors.New("cannot get XRDs: XRD not found"),
			},
		},
		"Success": {
			reason: "Should succeed when XRDs are found and converted",
			mock:   mockXRDsSuccess,
			args: args{
				ctx: context.Background(),
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor that uses our mock client
			p, _ := NewDiffProcessor(tc.mock, WithRestConfig(&rest.Config{}))

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.Initialize(&stdout, tc.args.ctx)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nInitialize(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nInitialize(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nInitialize(...): unexpected error: %v", tc.reason, err)
				return
			}

			// For success case, we assume no error means happy, because we don't want to expose the crd cache field
		})
	}
}

func normalizeTrailingWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	// Remove trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
