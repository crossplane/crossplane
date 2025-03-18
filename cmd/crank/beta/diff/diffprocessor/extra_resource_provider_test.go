package diffprocessor

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Helper function to create a pointer to a CompositionMode
func composePtr(mode apiextensionsv1.CompositionMode) *apiextensionsv1.CompositionMode {
	return &mode
}

func TestSelectorExtraResourceProvider_GetExtraResources(t *testing.T) {
	tests := []struct {
		name           string
		composition    *apiextensionsv1.Composition
		xr             *unstructured.Unstructured
		mockClient     *tu.MockClusterClient
		expectResCount int
		expectResNames []string
		expectError    bool
	}{
		{
			name: "Success with field path values",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "fetch-external-resources",
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
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
					"spec": map[string]interface{}{
						"environment": "production",
					},
				},
			},
			mockClient: &tu.MockClusterClient{
				// Using GetAllResourcesByLabels instead of GetResourcesByLabel
				GetAllResourcesByLabelsFn: func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					// Verify correct GVR
					if len(gvrs) == 0 || len(selectors) == 0 {
						return nil, errors.New("no GVRs or selectors provided")
					}

					expectedGVR := schema.GroupVersionResource{
						Group:    "example.org",
						Version:  "v1",
						Resource: "tests",
					}

					if gvrs[0] != expectedGVR {
						return nil, errors.Errorf("unexpected GVR: %v", gvrs[0])
					}

					// Verify correct selector
					expectedSelector := metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
							"env": "production",
						},
					}

					if !cmp.Equal(selectors[0].MatchLabels, expectedSelector.MatchLabels) {
						return nil, errors.New("unexpected selector")
					}

					// Return mock resources
					return []*unstructured.Unstructured{
						{
							Object: map[string]interface{}{
								"apiVersion": "example.org/v1",
								"kind":       "Test",
								"metadata": map[string]interface{}{
									"name": "test-resource",
									"labels": map[string]interface{}{
										"app": "test-app",
										"env": "production",
									},
								},
							},
						},
					}, nil
				},
			},
			expectResCount: 1,
			expectResNames: []string{"test-resource"},
			expectError:    false,
		},
		{
			name: "Non-pipeline composition returns no resources",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionMode("NonPipeline")),
				},
			},
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient:     &tu.MockClusterClient{},
			expectResCount: 0,
			expectError:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewSelectorExtraResourceProvider(tc.mockClient)
			resources, err := provider.GetExtraResources(context.Background(), tc.composition, tc.xr, nil)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetExtraResources() error = %v", err)
			}

			if len(resources) != tc.expectResCount {
				t.Fatalf("Expected %d resource(s), got %d", tc.expectResCount, len(resources))
			}

			// Check resource names if expected
			for i, name := range tc.expectResNames {
				if i >= len(resources) {
					break
				}
				if resources[i].GetName() != name {
					t.Errorf("Resource at index %d: expected name '%s', got '%s'", i, name, resources[i].GetName())
				}
			}
		})
	}
}

func TestReferenceExtraResourceProvider_GetExtraResources(t *testing.T) {
	tests := []struct {
		name           string
		composition    *apiextensionsv1.Composition
		xr             *unstructured.Unstructured
		mockClient     *tu.MockClusterClient
		expectResCount int
		expectResName  string
		expectError    bool
	}{
		{
			name: "Success fetching reference resource",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "fetch-external-resources",
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
												"type": "Reference",
												"ref": {
													"name": "test-reference"
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
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient: &tu.MockClusterClient{
				GetResourceFn: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					// Verify correct GVK and name
					expectedGVK := schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "Test",
					}

					if gvk != expectedGVK {
						return nil, errors.Errorf("unexpected GVK: %v", gvk)
					}

					if name != "test-reference" {
						return nil, errors.Errorf("unexpected name: %s", name)
					}

					// Return mock resource
					return &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test-reference",
							},
						},
					}, nil
				},
			},
			expectResCount: 1,
			expectResName:  "test-reference",
			expectError:    false,
		},
		{
			name: "Non-pipeline composition returns no resources",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionMode("NonPipeline")),
				},
			},
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient:     &tu.MockClusterClient{},
			expectResCount: 0,
			expectError:    false,
		},
		{
			name: "Error fetching reference resource",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "fetch-external-resources",
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
												"type": "Reference",
												"ref": {
													"name": "test-reference"
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
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient: &tu.MockClusterClient{
				GetResourceFn: func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, errors.New("resource not found")
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewReferenceExtraResourceProvider(tc.mockClient)
			resources, err := provider.GetExtraResources(context.Background(), tc.composition, tc.xr, nil)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetExtraResources() error = %v", err)
			}

			if len(resources) != tc.expectResCount {
				t.Fatalf("Expected %d resource(s), got %d", tc.expectResCount, len(resources))
			}

			if tc.expectResCount > 0 && resources[0].GetName() != tc.expectResName {
				t.Errorf("Expected resource name '%s', got '%s'", tc.expectResName, resources[0].GetName())
			}
		})
	}
}

// TDOO evaluate whether this function or the existing one is better for testing
func TestScanForTemplatedExtraResources2(t *testing.T) {
	tests := []struct {
		name        string
		composition *apiextensionsv1.Composition
		expectFound bool
		expectError bool
	}{
		{
			name: "No templated extra resources",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "generate-resources",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "template.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
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
			expectFound: false,
			expectError: false,
		},
		{
			name: "With templated extra resources",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "generate-resources",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "template.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
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
			expectFound: true,
			expectError: false,
		},
		{
			name: "Non-pipeline composition",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionMode("NonPipeline")),
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name: "Invalid template YAML",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "generate-resources",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "template.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
									"spec": {
										"inline": {
											"template": "{{ invalid template"
										}
									}
								}`),
							},
						},
					},
				},
			},
			expectFound: false,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			found, err := ScanForTemplatedExtraResources(tc.composition)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ScanForTemplatedExtraResources() error = %v", err)
			}

			if found != tc.expectFound {
				t.Errorf("Expected found=%v, got %v", tc.expectFound, found)
			}
		})
	}
}

func TestTemplatedExtraResourceProvider_GetExtraResources(t *testing.T) {
	tests := []struct {
		name           string
		composition    *apiextensionsv1.Composition
		xr             *unstructured.Unstructured
		mockClient     *tu.MockClusterClient
		mockRenderFn   RenderFunc
		expectResCount int
		expectResName  string
		expectError    bool
	}{
		{
			name: "Success fetching templated resources",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "generate-resources",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "template.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
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
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient: &tu.MockClusterClient{
				GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return []pkgv1.Function{}, nil
				},
			},
			mockRenderFn: func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				return render.Outputs{
					Results: []unstructured.Unstructured{
						{
							Object: map[string]interface{}{
								"apiVersion": "render.crossplane.io/v1beta1",
								"kind":       "ExtraResources",
								"spec": map[string]interface{}{
									"resources": []interface{}{
										map[string]interface{}{
											"apiVersion": "v1",
											"kind":       "ConfigMap",
											"metadata": map[string]interface{}{
												"name": "test-configmap",
											},
										},
									},
								},
							},
						},
					},
				}, nil
			},
			expectResCount: 1,
			expectResName:  "test-configmap",
			expectError:    false,
		},
		{
			name: "No templated resources in composition",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "generate-resources",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "template.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
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
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient: &tu.MockClusterClient{
				GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return []pkgv1.Function{}, nil
				},
			},
			mockRenderFn: func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				return render.Outputs{}, nil
			},
			expectResCount: 0,
			expectError:    false,
		},
		{
			name: "Error fetching functions",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "generate-resources",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "template.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
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
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient: &tu.MockClusterClient{
				GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return nil, errors.New("failed to fetch functions")
				},
			},
			mockRenderFn: func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				return render.Outputs{}, nil
			},
			expectError: true,
		},
		{
			name: "Error in render function",
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
					Pipeline: []apiextensionsv1.PipelineStep{
						{
							Step:        "generate-resources",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "template.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
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
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			mockClient: &tu.MockClusterClient{
				GetFunctionsFromPipelineFn: func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return []pkgv1.Function{}, nil
				},
			},
			mockRenderFn: func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				return render.Outputs{}, errors.New("render error")
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewTemplatedExtraResourceProvider(tc.mockClient, tc.mockRenderFn, logging.NewNopLogger())
			resources, err := provider.GetExtraResources(context.Background(), tc.composition, tc.xr, nil)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetExtraResources() error = %v", err)
			}

			if len(resources) != tc.expectResCount {
				t.Fatalf("Expected %d resource(s), got %d", tc.expectResCount, len(resources))
			}

			if tc.expectResCount > 0 && resources[0].GetName() != tc.expectResName {
				t.Errorf("Expected resource name '%s', got '%s'", tc.expectResName, resources[0].GetName())
			}
		})
	}
}

func TestCompositeExtraResourceProvider_GetExtraResources(t *testing.T) {
	tests := []struct {
		name           string
		providers      []ExtraResourceProvider
		composition    *apiextensionsv1.Composition
		xr             *unstructured.Unstructured
		expectResCount int
		expectResNames []string
		expectError    bool
	}{
		{
			name: "Success with multiple providers",
			providers: []ExtraResourceProvider{
				&tu.MockExtraResourceProvider{
					GetExtraResourcesFn: func(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
						return []*unstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": "example.org/v1",
									"kind":       "Test1",
									"metadata": map[string]interface{}{
										"name": "resource-1",
									},
								},
							},
						}, nil
					},
				},
				&tu.MockExtraResourceProvider{
					GetExtraResourcesFn: func(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
						// Verify that resources from provider1 are included
						if len(resources) != 1 || resources[0].GetName() != "resource-1" {
							return nil, errors.New("expected resources from provider1")
						}

						return []*unstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": "example.org/v1",
									"kind":       "Test2",
									"metadata": map[string]interface{}{
										"name": "resource-2",
									},
								},
							},
						}, nil
					},
				},
			},
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
				},
			},
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			expectResCount: 2,
			expectResNames: []string{"resource-1", "resource-2"},
			expectError:    false,
		},
		{
			name: "Error in second provider",
			providers: []ExtraResourceProvider{
				&tu.MockExtraResourceProvider{
					GetExtraResourcesFn: func(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
						return []*unstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": "example.org/v1",
									"kind":       "Test1",
									"metadata": map[string]interface{}{
										"name": "resource-1",
									},
								},
							},
						}, nil
					},
				},
				&tu.MockExtraResourceProvider{
					GetExtraResourcesFn: func(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
						return nil, errors.New("failed to get resources")
					},
				},
			},
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
				},
			},
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			expectError: true,
		},
		{
			name:      "Empty provider list returns no resources",
			providers: []ExtraResourceProvider{},
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
				},
			},
			xr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.org/v1",
					"kind":       "XExampleResource",
					"metadata": map[string]interface{}{
						"name": "test-xr",
					},
				},
			},
			expectResCount: 0,
			expectError:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewCompositeExtraResourceProvider(tc.providers...)
			resources, err := provider.GetExtraResources(context.Background(), tc.composition, tc.xr, nil)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetExtraResources() error = %v", err)
			}

			if len(resources) != tc.expectResCount {
				t.Fatalf("Expected %d resource(s), got %d", tc.expectResCount, len(resources))
			}

			// Check resource names if expected
			for i, name := range tc.expectResNames {
				if i >= len(resources) {
					break
				}
				if resources[i].GetName() != name {
					t.Errorf("Resource at index %d: expected name '%s', got '%s'", i, name, resources[i].GetName())
				}
			}
		})
	}
}
