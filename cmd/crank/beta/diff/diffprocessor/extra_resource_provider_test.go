package diffprocessor

import (
	"context"
	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
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
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").
				WithSpecField("environment", "production").
				Build(),
			mockClient: tu.NewMockClusterClient().
				WithGetAllResourcesByLabels(func(ctx context.Context, gvrs []schema.GroupVersionResource, selectors []metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
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
						tu.NewResource("example.org/v1", "Test", "test-resource").
							WithLabels(map[string]string{
								"app": "test-app",
								"env": "production",
							}).
							Build(),
					}, nil
				}).
				Build(),
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
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient: tu.NewMockClusterClient().
				Build(),
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
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient: tu.NewMockClusterClient().
				WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
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
					return tu.NewResource("example.org/v1", "Test", "test-reference").Build(), nil
				}).
				Build(),
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
			xr:             tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient:     tu.NewMockClusterClient().Build(),
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
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient: tu.NewMockClusterClient().
				WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
					return nil, errors.New("resource not found")
				}).
				Build(),
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
							Step:        "render-templates",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "gotemplating.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
									"source": "Inline",
									"inline": {
										"template": "apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1\nkind: ExtraResources\nrequirements:\n  configmaps:\n    apiVersion: v1\n    kind: ConfigMap\n    matchLabels:\n      app: test-app"
									}
								}`),
							},
						},
					},
				},
			},
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient: tu.NewMockClusterClient().
				WithGetFunctionsFromPipeline(func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return []pkgv1.Function{}, nil
				}).
				WithGetResourcesByLabel(func(ctx context.Context, ns string, gvr schema.GroupVersionResource, selector metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
					// Verify the GVR and selector match our expectations
					expectedGVR := schema.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "configmaps",
					}
					if gvr != expectedGVR {
						return nil, errors.Errorf("unexpected GVR: %v", gvr)
					}

					// Verify the selector matches our expectation
					if selector.MatchLabels["app"] != "test-app" {
						return nil, errors.New("unexpected selector")
					}

					return []*unstructured.Unstructured{
						tu.NewResource("v1", "ConfigMap", "test-configmap").
							WithLabels(map[string]string{"app": "test-app"}).
							Build(),
					}, nil
				}).
				Build(),
			mockRenderFn: func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				// Create the match labels for the resource selector
				matchLabels := &fnv1.MatchLabels{
					Labels: map[string]string{
						"app": "test-app",
					},
				}

				// Create the resource selector with the match labels
				resourceSelector := &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "ConfigMap",
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: matchLabels,
					},
				}

				// Create the requirements with the resource selector
				requirements := map[string]fnv1.Requirements{
					"render-templates": {
						ExtraResources: map[string]*fnv1.ResourceSelector{
							"configmaps": resourceSelector,
						},
					},
				}

				return render.Outputs{
					Requirements: requirements,
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
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient: tu.NewMockClusterClient().
				WithGetFunctionsFromPipeline(func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return []pkgv1.Function{}, nil
				}).
				Build(),
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
							Step:        "render-templates",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "gotemplating.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
									"source": "Inline", 
									"inline": {
										"template": "apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1\nkind: ExtraResources\nrequirements:\n  configmaps:\n    apiVersion: v1\n    kind: ConfigMap\n    matchLabels:\n      app: test-app"
									}
								}`),
							},
						},
					},
				},
			},
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient: tu.NewMockClusterClient().
				WithGetFunctionsFromPipeline(func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return nil, errors.New("failed to fetch functions")
				}).
				Build(),
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
							Step:        "render-templates",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-go-templating"},
							Input: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion": "gotemplating.fn.crossplane.io/v1beta1",
									"kind": "GoTemplate",
									"source": "Inline",
									"inline": {
										"template": "apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1\nkind: ExtraResources\nrequirements:\n  configmaps:\n    apiVersion: v1\n    kind: ConfigMap\n    matchLabels:\n      app: test-app"
									}
								}`),
							},
						},
					},
				},
			},
			xr: tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			mockClient: tu.NewMockClusterClient().
				WithGetFunctionsFromPipeline(func(comp *apiextensionsv1.Composition) ([]pkgv1.Function, error) {
					return []pkgv1.Function{}, nil
				}).
				Build(),
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
				tu.NewMockExtraResourceProvider().
					WithSuccessfulExtraResourcesFetch([]*unstructured.Unstructured{
						tu.NewResource("example.org/v1", "Test1", "resource-1").Build(),
					}).
					Build(),
				tu.NewMockExtraResourceProvider().
					WithGetExtraResources(func(ctx context.Context, comp *apiextensionsv1.Composition, xr *unstructured.Unstructured, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
						// Verify that resources from provider1 are included
						if len(resources) != 1 || resources[0].GetName() != "resource-1" {
							return nil, errors.New("expected resources from provider1")
						}

						return []*unstructured.Unstructured{
							tu.NewResource("example.org/v1", "Test2", "resource-2").Build(),
						}, nil
					}).
					Build(),
			},
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
				},
			},
			xr:             tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
			expectResCount: 2,
			expectResNames: []string{"resource-1", "resource-2"},
			expectError:    false,
		},
		{
			name: "Error in second provider",
			providers: []ExtraResourceProvider{
				tu.NewMockExtraResourceProvider().
					WithSuccessfulExtraResourcesFetch([]*unstructured.Unstructured{
						tu.NewResource("example.org/v1", "Test1", "resource-1").Build(),
					}).
					Build(),
				tu.NewMockExtraResourceProvider().
					WithFailedExtraResourcesFetch("failed to get resources").
					Build(),
			},
			composition: &apiextensionsv1.Composition{
				Spec: apiextensionsv1.CompositionSpec{
					Mode: composePtr(apiextensionsv1.CompositionModePipeline),
				},
			},
			xr:          tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
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
			xr:             tu.NewResource("example.org/v1", "XExampleResource", "test-xr").Build(),
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

func TestScanForTemplatedExtraResources(t *testing.T) {
	tests := []struct {
		name        string
		composition *apiextensionsv1.Composition
		expectFound bool
		expectError bool
	}{
		{
			name: "No templated extra resources",
			composition: tu.NewComposition("test-comp").
				WithPipelineMode().
				WithPipelineStep("generate-resources", "function-go-templating", map[string]interface{}{
					"apiVersion": "template.fn.crossplane.io/v1beta1",
					"kind":       "GoTemplate",
					"spec": map[string]interface{}{
						"inline": map[string]interface{}{
							"template": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test",
						},
					},
				}).
				Build(),
			expectFound: false,
			expectError: false,
		},
		{
			name: "With templated extra resources",
			composition: tu.NewComposition("test-comp").
				WithPipelineMode().
				WithPipelineStep("generate-resources", "function-go-templating", map[string]interface{}{
					"apiVersion": "template.fn.crossplane.io/v1beta1",
					"kind":       "GoTemplate",
					"spec": map[string]interface{}{
						"inline": map[string]interface{}{
							"template": "apiVersion: render.crossplane.io/v1\nkind: ExtraResources\nspec:\n  resources:\n  - apiVersion: v1\n    kind: ConfigMap",
						},
					},
				}).
				Build(),
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
			composition: tu.NewComposition("test-comp").
				WithPipelineMode().
				WithPipelineStep("generate-resources", "function-go-templating", map[string]interface{}{
					"apiVersion": "template.fn.crossplane.io/v1beta1",
					"kind":       "GoTemplate",
					"spec": map[string]interface{}{
						"inline": map[string]interface{}{
							"template": "{{ invalid template",
						},
					},
				}).
				Build(),
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

func TestGetExtraResourcesFromResult(t *testing.T) {
	tests := map[string]struct {
		reason string
		setup  func() *unstructured.Unstructured
		want   struct {
			resources []*unstructured.Unstructured
			err       error
		}
	}{
		"NoSpec": {
			reason: "Should return error when result has no spec",
			setup: func() *unstructured.Unstructured {
				return tu.NewResource("render.crossplane.io/v1beta1", "ExtraResources", "result").
					Build()
			},
			want: struct {
				resources []*unstructured.Unstructured
				err       error
			}{
				resources: nil,
				err:       errors.New("no spec found in ExtraResources result"),
			},
		},
		"NoResources": {
			reason: "Should return error when spec has no resources",
			setup: func() *unstructured.Unstructured {
				return tu.NewResource("render.crossplane.io/v1beta1", "ExtraResources", "result").
					WithSpecField("otherField", "value").
					Build()
			},
			want: struct {
				resources []*unstructured.Unstructured
				err       error
			}{
				resources: nil,
				err:       errors.New("no resources found in ExtraResources spec"),
			},
		},
		"WithResources": {
			reason: "Should return resources when they exist",
			setup: func() *unstructured.Unstructured {
				return tu.NewResource("render.crossplane.io/v1beta1", "ExtraResources", "result").
					WithSpecField("resources", []interface{}{
						tu.NewResource("v1", "ConfigMap", "resource-1").Build().Object,
						tu.NewResource("v1", "Secret", "resource-2").Build().Object,
					}).
					Build()
			},
			want: struct {
				resources []*unstructured.Unstructured
				err       error
			}{
				resources: []*unstructured.Unstructured{
					tu.NewResource("v1", "ConfigMap", "resource-1").Build(),
					tu.NewResource("v1", "Secret", "resource-2").Build(),
				},
				err: nil,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup the input for this test case
			result := tc.setup()

			// Call the function under test
			got, err := GetExtraResourcesFromResult(result)

			// Check error expectations
			if tc.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nGetExtraResourcesFromResult(...): expected error but got none", tc.reason)
					return
				}

				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("\n%s\nGetExtraResourcesFromResult(...): -want error, +got error:\n%s", tc.reason, diff)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetExtraResourcesFromResult(...): unexpected error: %v", tc.reason, err)
				return
			}

			// Compare resource count
			if diff := cmp.Diff(len(tc.want.resources), len(got)); diff != "" {
				t.Errorf("\n%s\nGetExtraResourcesFromResult(...): -want resource count, +got resource count:\n%s", tc.reason, diff)
			}

			// Check each resource
			for i, wantRes := range tc.want.resources {
				if i >= len(got) {
					break
				}

				if diff := cmp.Diff(wantRes.GetKind(), got[i].GetKind()); diff != "" {
					t.Errorf("\n%s\nGetExtraResourcesFromResult(...): -want kind, +got kind at index %d:\n%s", tc.reason, i, diff)
				}

				if diff := cmp.Diff(wantRes.GetName(), got[i].GetName()); diff != "" {
					t.Errorf("\n%s\nGetExtraResourcesFromResult(...): -want name, +got name at index %d:\n%s", tc.reason, i, diff)
				}
			}
		})
	}
}
