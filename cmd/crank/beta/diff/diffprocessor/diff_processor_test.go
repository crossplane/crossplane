package diffprocessor

import (
	"bytes"
	"context"
	cpd "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	cmp "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
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
	gcmp "github.com/google/go-cmp/cmp"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	// Create a cpd resource for testing
	composedResource := tu.NewResource("cpd.org/v1", "ComposedResource", "resource1").
		WithCompositeOwner("my-xr-1").
		WithCompositionResourceName("resA").
		WithSpecField("param", "value").
		Build()

	// Test cases
	tests := map[string]struct {
		client    func() *tu.MockClusterClient
		resources []*un.Unstructured
		//mockRender      func(context.Context, logging.Logger, render.Inputs) (render.Outputs, error)
		processorOpts   []ProcessorOption
		verifyOutput    func(t *testing.T, output string)
		want            error
		validationError bool
	}{
		"NoResources": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			resources: []*un.Unstructured{},
			want:      nil,
		},
		"DiffSingleResourceError": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithNoMatchingComposition().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			resources: []*un.Unstructured{resource1},
			want:      errors.New("unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found"),
		},
		"MultipleResourceErrors": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithNoMatchingComposition().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			resources: []*un.Unstructured{resource1, resource2},
			want: errors.New("[unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found, " +
				"unable to process resource XR1/my-xr-2: cannot find matching composition: composition not found]"),
		},
		"CompositionNotFound": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithNoMatchingComposition().
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			resources: []*un.Unstructured{resource1},
			want:      errors.New("unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found"),
		},
		"GetFunctionsError": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(composition).
					WithFailedFunctionsFetch("function not found").
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			resources: []*un.Unstructured{resource1},
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
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					WithResourcesExist(resource1, composedResource). // Add resources to existing resources
					WithComposedResourcesByOwner(composedResource).  // Add cpd resource lookup by owner
					WithSuccessfulDryRun().
					WithSuccessfulXRDsFetch([]*un.Unstructured{}).
					// Add this line to make test resources not require CRDs:
					WithNoResourcesRequiringCRDs().
					Build()
			},
			resources: []*un.Unstructured{resource1},
			processorOpts: []ProcessorOption{
				WithRenderFunc(func(_ context.Context, _ logging.Logger, in render.Inputs) (render.Outputs, error) {
					return render.Outputs{
						CompositeResource: in.CompositeResource,
						ComposedResources: []cpd.Unstructured{
							{
								Unstructured: un.Unstructured{
									Object: composedResource.Object,
								},
							},
						},
					}, nil
				}),
			},
			verifyOutput: func(t *testing.T, output string) {
				t.Helper()
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
		"ValidationError": {
			client: func() *tu.MockClusterClient {
				// Create mock functions that render will call successfully
				functions := []pkgv1.Function{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-test",
						},
					},
				}

				// Setup a client that provides all the necessary data for rendering
				// but validation will fail in a separate mock
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(composition).
					WithSuccessfulFunctionsFetch(functions).
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					WithResourcesExist(resource1).
					WithSuccessfulDryRun().
					WithNoResourcesRequiringCRDs().
					Build()
			},
			resources: []*un.Unstructured{resource1},
			processorOpts: []ProcessorOption{
				WithRenderFunc(func(_ context.Context, _ logging.Logger, in render.Inputs) (render.Outputs, error) {
					// Return valid render outputs
					return render.Outputs{
						CompositeResource: in.CompositeResource,
						ComposedResources: []cpd.Unstructured{
							{
								Unstructured: un.Unstructured{
									Object: composedResource.Object,
								},
							},
						},
					}, nil
				}),
			},
			want:            errors.New("unable to process resource XR1/my-xr-1: cannot validate resources: validation error"),
			validationError: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Create components for testing
			mockClient := tt.client()

			// Add common options
			options := []ProcessorOption{
				WithLogger(tu.TestLogger(t)),
				WithRestConfig(&rest.Config{}),
				WithSchemaValidatorFactory(
					// Create a mock schema validator that succeeds unless we request an error
					func(_ cc.ClusterClient, _ logging.Logger) SchemaValidator {
						return &tu.MockSchemaValidator{
							ValidateResourcesFn: func(_ context.Context, _ *un.Unstructured, _ []cpd.Unstructured) error {
								if tt.validationError {
									return errors.New("validation error")
								}
								return nil // succeed
							},
						}
					},
				),
			}
			options = append(options, tt.processorOpts...)

			processor, _ := NewDiffProcessor(mockClient, options...)

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := processor.PerformDiff(ctx, &stdout, tt.resources)

			if tt.want != nil {
				if err == nil {
					t.Errorf("PerformDiff(...): expected error but got none")
					return
				}

				if diff := gcmp.Diff(tt.want.Error(), err.Error()); diff != "" {
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
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			want: errors.Wrap(errors.Wrap(errors.New("XRD not found"), "cannot get XRDs"), "cannot load CRDs"),
		},
		"EnvConfigsError": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*un.Unstructured{}).
					WithEnvironmentConfigs(func(_ context.Context) ([]*un.Unstructured, error) {
						return nil, errors.New("env configs not found")
					}).
					Build()
			},
			want: errors.Wrap(errors.New("env configs not found"), "cannot get environment configs"),
		},
		"Success": {
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*un.Unstructured{xrd1}).
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
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

			// Build processor options
			options := []ProcessorOption{
				WithLogger(logger),
				WithRestConfig(&rest.Config{}),
			}

			processor, _ := NewDiffProcessor(mockClient, options...)

			err := processor.Initialize(ctx)

			if tc.want != nil {
				if err == nil {
					t.Errorf("Initialize(...): expected error but got none")
					return
				}

				if diff := gcmp.Diff(tc.want.Error(), err.Error()); diff != "" {
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
		xr                   *cmp.Unstructured
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
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(_ context.Context, _ logging.Logger, in render.Inputs) (render.Outputs, error) {
					iteration++
					// Return a simple output with no requirements
					return render.Outputs{
						CompositeResource: in.CompositeResource,
						ComposedResources: []cpd.Unstructured{
							{Unstructured: un.Unstructured{Object: map[string]interface{}{
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
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(_ context.Context, _ logging.Logger, in render.Inputs) (render.Outputs, error) {
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
						ComposedResources: []cpd.Unstructured{
							{Unstructured: un.Unstructured{Object: map[string]interface{}{
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
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						if gvk.Kind == "Secret" && name == "secret1" {
							return secret, nil
						}
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(_ context.Context, _ logging.Logger, in render.Inputs) (render.Outputs, error) {
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
						ComposedResources: []cpd.Unstructured{
							{Unstructured: un.Unstructured{Object: map[string]interface{}{
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
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				return func(context.Context, logging.Logger, render.Inputs) (render.Outputs, error) {
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
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == "ConfigMap" && name == "config1" {
							return configMap, nil
						}
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				iteration := 0
				return func(_ context.Context, _ logging.Logger, in render.Inputs) (render.Outputs, error) {
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
						ComposedResources: []cpd.Unstructured{
							{Unstructured: un.Unstructured{Object: map[string]interface{}{
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
					WithGetResource(func(context.Context, schema.GroupVersionKind, string, string) (*un.Unstructured, error) {
						return nil, errors.New("resource not found")
					}).
					WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
					Build()
			},
			setupRenderFunc: func() RenderFunc {
				return func(_ context.Context, _ logging.Logger, in render.Inputs) (render.Outputs, error) {
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

			// Create a logger
			logger := tu.TestLogger(t)
			renderFunc := tt.setupRenderFunc()

			// Create a render iteration counter to verify
			renderCount := 0
			countingRenderFunc := func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				renderCount++
				return renderFunc(ctx, log, in)
			}

			// Build processor options
			options := []ProcessorOption{
				WithLogger(logger),
				WithRestConfig(&rest.Config{}),
				WithRenderFunc(countingRenderFunc),
			}

			processor, _ := NewDiffProcessor(mockClient, options...)

			// Call the method under test
			output, err := processor.(*DefaultDiffProcessor).RenderWithRequirements(ctx, tt.xr, tt.composition, tt.functions, tt.resourceID)

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

			// Check cpd resource count
			if len(output.ComposedResources) != tt.wantComposedCount {
				t.Errorf("RenderWithRequirements() returned %d cpd resources, want %d",
					len(output.ComposedResources), tt.wantComposedCount)
			}
		})
	}
}
