package diffprocessor

import (
	"bytes"
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

// Ensure MockDiffProcessor implements the DiffProcessor interface
var _ DiffProcessor = &tu.MockDiffProcessor{}

func TestDefaultDiffProcessor_PerformDiff(t *testing.T) {
	// Setup test context
	ctx := context.Background()

	// Create test resources
	resource1 := tu.NewResource("example.org/v1", "XR1", "my-xr-1").
		WithSpecField("coolField", "test-value-1").
		Build()

	resource2 := tu.NewResource("example.org/v1", "XR1", "my-xr-2").
		WithSpecField("coolField", "test-value-2").
		Build()

	// Create a composition for testing
	composition := tu.NewComposition("test-comp").
		WithCompositeTypeRef("example.org/v1", "XR1").
		WithPipelineMode().
		WithPipelineStep("step1", "function-test", nil).
		Build()

	// Create a composed resource for testing
	composedResource := tu.NewResource("composed.org/v1", "ComposedResource", "resource1").
		WithCompositeOwner("my-xr-1").
		WithCompositionResourceName("resA").
		WithSpecField("param", "value").
		Build()

	// Test cases
	tests := map[string]struct {
		client       func() *tu.MockClusterClient
		resources    []*unstructured.Unstructured
		mockRender   func(context.Context, logging.Logger, render.Inputs) (render.Outputs, error)
		verifyOutput func(t *testing.T, output string)
		want         error
	}{
		"NoResources": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			resources: []*unstructured.Unstructured{},
			want:      nil,
		},
		"DiffSingleResourceError": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithNoMatchingComposition().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			resources: []*unstructured.Unstructured{resource1},
			want:      errors.New("unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found"),
		},
		"MultipleResourceErrors": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithNoMatchingComposition().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			resources: []*unstructured.Unstructured{resource1, resource2},
			want: errors.New("[unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found, " +
				"unable to process resource XR1/my-xr-2: cannot find matching composition: composition not found]"),
		},
		"CompositionNotFound": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithNoMatchingComposition().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			resources: []*unstructured.Unstructured{resource1},
			want:      errors.New("unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found"),
		},
		"GetFunctionsError": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(composition).
					WithFailedFunctionsFetch("function not found").
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			resources: []*unstructured.Unstructured{resource1},
			want:      errors.New("unable to process resource XR1/my-xr-1: cannot get functions from pipeline: function not found"),
		},
		"SuccessfulDiff": {
			client: func() *tu.MockClusterClient {
				// Create mock functions that render will call successfully
				functions := []pkgv1.Function{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-test",
						},
					},
				}

				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(composition).
					WithSuccessfulFunctionsFetch(functions).
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					WithResourcesExist(resource1, composedResource). // Add resources to existing resources
					WithComposedResourcesByOwner(composedResource). // Add composed resource lookup by owner
					WithSuccessfulDryRun().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{}).
					// Add this line to make test resources not require CRDs:
					WithNoResourcesRequiringCRDs().
					Build()
			},
			resources: []*unstructured.Unstructured{resource1},
			mockRender: func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				return render.Outputs{
					CompositeResource: in.CompositeResource,
					ComposedResources: []composed.Unstructured{
						{
							Unstructured: unstructured.Unstructured{
								Object: composedResource.Object,
							},
						},
					},
				}, nil
			},
			verifyOutput: func(t *testing.T, output string) {
				// We should have some output from the diff
				if output == "" {
					t.Errorf("Expected non-empty diff output")
				}

				// Simple check for expected output format
				if !strings.Contains(output, "Summary:") {
					t.Errorf("Expected diff output to contain a Summary section")
				}
			},
			want: nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Create components for testing
			mockClient := tt.client()
			logger := tu.TestLogger(t)

			// Create mock components for the processor
			resourceManager := NewResourceManager(mockClient, logger)
			schemaValidator := NewSchemaValidator(mockClient, logger)
			diffOptions := DefaultDiffOptions()
			diffCalculator := NewDiffCalculator(mockClient, resourceManager, logger, diffOptions)
			diffRenderer := NewDiffRenderer(logger, diffOptions)

			// Create the processor with test components
			processor, err := CreateTestProcessor(
				mockClient,
				resourceManager,
				schemaValidator,
				diffCalculator,
				diffRenderer,
				tt.mockRender,
				logger,
			)
			if err != nil {
				t.Fatalf("Failed to create processor: %v", err)
			}

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err = processor.PerformDiff(&stdout, ctx, tt.resources)

			if tt.want != nil {
				if err == nil {
					t.Errorf("PerformDiff(...): expected error but got none")
					return
				}

				if diff := cmp.Diff(tt.want.Error(), err.Error()); diff != "" {
					t.Errorf("PerformDiff(...): -want error, +got error:\n%s", diff)
				}
				return
			}

			if err != nil {
				t.Errorf("PerformDiff(...): unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultDiffProcessor_Initialize(t *testing.T) {
	// Setup test context
	ctx := context.Background()

	// Create test resources
	xrd1 := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd1").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XExampleResource",
			"plural":   "xexampleresources",
			"singular": "xexampleresource",
		}).
		Build()

	// Test cases
	tests := map[string]struct {
		client func() *tu.MockClusterClient
		want   error
	}{
		"XRDsError": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithFailedXRDsFetch("XRD not found").
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			want: errors.Wrap(errors.Wrap(errors.New("XRD not found"), "cannot get XRDs"), "cannot load CRDs"),
		},
		"EnvConfigsError": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{}).
					WithEnvironmentConfigs(func(ctx context.Context) ([]*unstructured.Unstructured, error) {
						return nil, errors.New("env configs not found")
					}).
					Build()
			},
			want: errors.Wrap(errors.New("env configs not found"), "cannot get environment configs"),
		},
		"Success": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{xrd1}).
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			want: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a DiffProcessor with our components
			mockClient := tc.client()
			logger := tu.TestLogger(t)

			// Create mock components for the processor
			resourceManager := NewResourceManager(mockClient, logger)
			schemaValidator := NewSchemaValidator(mockClient, logger)
			diffOptions := DefaultDiffOptions()
			diffCalculator := NewDiffCalculator(mockClient, resourceManager, logger, diffOptions)
			diffRenderer := NewDiffRenderer(logger, diffOptions)

			// Create the processor with our mock components
			processor, err := CreateTestProcessor(
				mockClient,
				resourceManager,
				schemaValidator,
				diffCalculator,
				diffRenderer,
				render.Render,
				logger,
			)
			if err != nil {
				t.Fatalf("Failed to create processor: %v", err)
			}

			err = processor.Initialize(ctx)

			if tc.want != nil {
				if err == nil {
					t.Errorf("Initialize(...): expected error but got none")
					return
				}

				if diff := cmp.Diff(tc.want.Error(), err.Error()); diff != "" {
					t.Errorf("Initialize(...): -want error, +got error:\n%s", diff)
				}
				return
			}

			if err != nil {
				t.Errorf("Initialize(...): unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultDiffProcessor_RenderWithRequirements(t *testing.T) {
	ctx := context.Background()

	// Create test resources
	xr := tu.NewResource("example.org/v1", "XR", "test-xr").BuildUComposite()

	// Create a composition with pipeline mode
	pipelineMode := apiextensionsv1.CompositionModePipeline
	composition := &apiextensionsv1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-composition",
		},
		Spec: apiextensionsv1.CompositionSpec{
			Mode: &pipelineMode,
		},
	}

	// Create test functions
	functions := []pkgv1.Function{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-function",
			},
		},
	}

	// Create test resources for requirements
	configMap := tu.NewResource("v1", "ConfigMap", "config1").Build()
	secret := tu.NewResource("v1", "Secret", "secret1").Build()

	tests := map[string]struct {
		xr                   *ucomposite.Unstructured
		composition          *apiextensionsv1.Composition
		functions            []pkgv1.Function
		resourceID           string
		setupClient          func() *tu.MockClusterClient
		setupRenderFunc      func() RenderFunc
		wantComposedCount    int
		wantRenderIterations int
		wantErr              bool
	}{
		"NoRequirements": {
			xr:          xr,
			composition: composition,
			functions:   functions,
			resourceID:  "XR/test-xr",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
					iteration++
					// Return a simple output with no requirements
					return render.Outputs{
						CompositeResource: in.CompositeResource,
						ComposedResources: []composed.Unstructured{
							{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
								"apiVersion": "example.org/v1",
								"kind":       "ComposedResource",
								"metadata": map[string]interface{}{
									"name": "composed1",
								},
							}}},
						},
						Requirements: map[string]v1.Requirements{},
					}, nil
				}
			},
			wantComposedCount:    1,
			wantRenderIterations: 1, // Only renders once when no requirements
			wantErr:              false,
		},
		"SingleIterationWithRequirements": {
			xr:          xr,
			composition: composition,
			functions:   functions,
			resourceID:  "XR/test-xr",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
					iteration++

					// First render includes requirements, second should have no requirements
					var reqs map[string]v1.Requirements
					if iteration == 1 {
						reqs = map[string]v1.Requirements{
							"step1": {
								ExtraResources: map[string]*v1.ResourceSelector{
									"config": {
										ApiVersion: "v1",
										Kind:       "ConfigMap",
										Match: &v1.ResourceSelector_MatchName{
											MatchName: "config1",
										},
									},
								},
							},
						}
					} else {
						reqs = map[string]v1.Requirements{}
					}

					// Return a simple output
					return render.Outputs{
						CompositeResource: in.CompositeResource,
						ComposedResources: []composed.Unstructured{
							{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
								"apiVersion": "example.org/v1",
								"kind":       "ComposedResource",
								"metadata": map[string]interface{}{
									"name": "composed1",
								},
							}}},
						},
						Requirements: reqs,
					}, nil
				}
			},
			wantComposedCount:    1,
			wantRenderIterations: 2, // Renders once with requirements, then once more to confirm no new requirements
			wantErr:              false,
		},
		"MultipleIterationsWithRequirements": {
			xr:          xr,
			composition: composition,
			functions:   functions,
			resourceID:  "XR/test-xr",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						if gvk.Kind == "Secret" && name == "secret1" {
							return secret, nil
						}
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
					iteration++

					// Track existing resources to simulate dependencies
					hasConfig := false
					hasSecret := false

					for _, res := range in.ExtraResources {
						if res.GetKind() == "ConfigMap" && res.GetName() == "config1" {
							hasConfig = true
						}
						if res.GetKind() == "Secret" && res.GetName() == "secret1" {
							hasSecret = true
						}
					}

					// Build requirements based on what we already have
					var reqs map[string]*v1.ResourceSelector

					if !hasConfig {
						// First iteration - request ConfigMap
						reqs = map[string]*v1.ResourceSelector{
							"config": {
								ApiVersion: "v1",
								Kind:       "ConfigMap",
								Match: &v1.ResourceSelector_MatchName{
									MatchName: "config1",
								},
							},
						}
					} else if !hasSecret {
						// Second iteration - request Secret
						reqs = map[string]*v1.ResourceSelector{
							"secret": {
								ApiVersion: "v1",
								Kind:       "Secret",
								Match: &v1.ResourceSelector_MatchName{
									MatchName: "secret1",
								},
							},
						}
					}

					requirements := map[string]v1.Requirements{}
					if len(reqs) > 0 {
						requirements["step1"] = v1.Requirements{
							ExtraResources: reqs,
						}
					}

					// Return a simple output
					return render.Outputs{
						CompositeResource: in.CompositeResource,
						ComposedResources: []composed.Unstructured{
							{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
								"apiVersion": "example.org/v1",
								"kind":       "ComposedResource",
								"metadata": map[string]interface{}{
									"name": "composed1",
								},
							}}},
						},
						Requirements: requirements,
					}, nil
				}
			},
			wantComposedCount:    1,
			wantRenderIterations: 3, // Iterations: 1. Request ConfigMap, 2. Request Secret, 3. No more requirements
			wantErr:              false,
		},
		"RenderError": {
			xr:          xr,
			composition: composition,
			functions:   functions,
			resourceID:  "XR/test-xr",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				return func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
					return render.Outputs{}, errors.New("render error")
				}
			},
			wantComposedCount:    0,
			wantRenderIterations: 1,
			wantErr:              true,
		},
		"RenderErrorWithRequirements": {
			xr:          xr,
			composition: composition,
			functions:   functions,
			resourceID:  "XR/test-xr",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
					iteration++

					// First render has requirements but errors
					if iteration == 1 {
						reqs := map[string]v1.Requirements{
							"step1": {
								ExtraResources: map[string]*v1.ResourceSelector{
									"config": {
										ApiVersion: "v1",
										Kind:       "ConfigMap",
										Match: &v1.ResourceSelector_MatchName{
											MatchName: "config1",
										},
									},
								},
							},
						}

						return render.Outputs{
							Requirements: reqs,
						}, errors.New("render error with requirements")
					}

					// Second render succeeds
					return render.Outputs{
						CompositeResource: in.CompositeResource,
						ComposedResources: []composed.Unstructured{
							{Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
								"apiVersion": "example.org/v1",
								"kind":       "ComposedResource",
								"metadata": map[string]interface{}{
									"name": "composed1",
								},
							}}},
						},
					}, nil
				}
			},
			wantComposedCount:    1,
			wantRenderIterations: 2,     // Renders once with error but requirements, then once more successfully
			wantErr:              false, // Should not error as the second render succeeds
		},
		"RequirementsProcessingError": {
			xr:          xr,
			composition: composition,
			functions:   functions,
			resourceID:  "XR/test-xr",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				return func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
					reqs := map[string]v1.Requirements{
						"step1": {
							ExtraResources: map[string]*v1.ResourceSelector{
								"config": {
									ApiVersion: "v1",
									Kind:       "ConfigMap",
									Match: &v1.ResourceSelector_MatchName{
										MatchName: "missing-config",
									},
								},
							},
						},
					}

					return render.Outputs{
						CompositeResource: in.CompositeResource,
						Requirements:      reqs,
					}, nil
				}
			},
			wantComposedCount:    0,
			wantRenderIterations: 1,
			wantErr:              true, // Should error because requirements processing fails
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up mock client and renderFunc
			mockClient := tt.setupClient()
			renderFunc := tt.setupRenderFunc()

			// Create a render iteration counter to verify
			renderCount := 0
			countingRenderFunc := func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				renderCount++
				return renderFunc(ctx, log, in)
			}

			// Create a logger
			logger := tu.TestLogger(t)

			// Create a requirements provider
			requirementsProvider := NewRequirementsProvider(mockClient, render.Render, logger)

			// Create the processor components
			resourceManager := NewResourceManager(mockClient, logger)
			diffOptions := DefaultDiffOptions()

			// Create the processor
			processor := &DefaultDiffProcessor{
				client: mockClient,
				config: ProcessorConfig{
					Logger:     logger,
					RenderFunc: countingRenderFunc,
				},
				resourceManager:      resourceManager,
				requirementsProvider: requirementsProvider,
				diffCalculator:       NewDiffCalculator(mockClient, resourceManager, logger, diffOptions),
				diffRenderer:         NewDiffRenderer(logger, diffOptions),
			}

			// Call the method under test
			output, err := processor.RenderWithRequirements(ctx, tt.xr, tt.composition, tt.functions, tt.resourceID)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("RenderWithRequirements() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("RenderWithRequirements() unexpected error: %v", err)
				return
			}

			// Check render iterations
			if renderCount != tt.wantRenderIterations {
				t.Errorf("RenderWithRequirements() called render func %d times, want %d",
					renderCount, tt.wantRenderIterations)
			}

			// Check composed resource count
			if len(output.ComposedResources) != tt.wantComposedCount {
				t.Errorf("RenderWithRequirements() returned %d composed resources, want %d",
					len(output.ComposedResources), tt.wantComposedCount)
			}
		})
	}
}

// Helper functions for processor testing

// CreateTestProcessor creates a processor with the provided components for testing
func CreateTestProcessor(
	client cc.ClusterClient,
	resourceManager ResourceManager,
	schemaValidator SchemaValidator,
	diffCalculator DiffCalculator,
	diffRenderer DiffRenderer,
	renderFunc RenderFunc,
	logger logging.Logger,
) (DiffProcessor, error) {
	// Create processor with custom components
	processor := &DefaultDiffProcessor{
		client:               client,
		resourceManager:      resourceManager,
		schemaValidator:      schemaValidator,
		diffCalculator:       diffCalculator,
		diffRenderer:         diffRenderer,
		requirementsProvider: NewRequirementsProvider(client, renderFunc, logger),
		config: ProcessorConfig{
			Namespace:  "default",
			Colorize:   true,
			Compact:    false,
			Logger:     logger,
			RenderFunc: renderFunc,
			RestConfig: &rest.Config{},
		},
	}

	return processor, nil
}
