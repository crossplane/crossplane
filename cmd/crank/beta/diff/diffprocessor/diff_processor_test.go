package diffprocessor

import (
	"bytes"
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestDefaultDiffProcessor_ProcessResource(t *testing.T) {
	ctx := context.Background()

	// Create test resources
	xr := tu.NewResource("example.org/v1", "XR1", "my-xr").
		WithSpecField("coolField", "test-value").
		Build()

	composition := tu.NewComposition("test-comp").
		WithCompositeTypeRef("example.org/v1", "XR1").
		WithPipelineMode().
		WithPipelineStep("step1", "function-test", nil).
		Build()

	composedResource := tu.NewResource("composed.org/v1", "ComposedResource", "resource1").
		WithCompositeOwner("my-xr").
		WithCompositionResourceName("resA").
		WithSpecField("param", "value").
		Build()

	composedXrd := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd1").
		WithSpecField("group", "composed.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "ComposedResource",
			"plural":   "composedresources",
			"singular": "composedresource",
		}).
		Build()

	// Test cases
	tests := []struct {
		name      string
		mockSetup func() *tu.MockClusterClient
		want      error
	}{
		{
			name: "CompositionNotFound",
			mockSetup: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithNoMatchingComposition().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			want: errors.Wrap(errors.New("composition not found"), "cannot find matching composition"),
		},
		{
			name: "GetFunctionsError",
			mockSetup: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulCompositionMatch(composition).
					WithFailedFunctionsFetch("function not found").
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			want: errors.Wrap(errors.New("function not found"), "cannot get functions from pipeline"),
		},
		{
			name: "Success",
			mockSetup: func() *tu.MockClusterClient {
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
					WithResourcesExist(xr, composedResource).       // Add the XR to existing resources
					WithComposedResourcesByOwner(composedResource). // Add composed resource lookup by owner
					WithSuccessfulDryRun().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{composedXrd}).
					Build()
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a DiffProcessor with our mock client
			mockClient := tc.mockSetup()

			// Create a mock render function
			mockRenderFn := func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
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
			}

			// Create a logger
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
				NewEnvironmentConfigProvider([]*unstructured.Unstructured{}, logger),
				logger,
			)
			if err != nil {
				t.Fatalf("Failed to create processor: %v", err)
			}

			// Override the render function
			processor.(*DefaultDiffProcessor).config.RenderFunc = mockRenderFn

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			// Initialize the processor for the Success case only
			if tc.name == "Success" {
				if err := processor.Initialize(ctx); err != nil {
					t.Fatalf("Failed to initialize processor: %v", err)
				}
			}

			err = processor.ProcessResource(&stdout, ctx, xr)

			if tc.want != nil {
				if err == nil {
					t.Errorf("ProcessResource(...): expected error but got none")
					return
				}

				if diff := cmp.Diff(tc.want.Error(), err.Error()); diff != "" {
					t.Errorf("ProcessResource(...): -want error, +got error:\n%s", diff)
				}
				return
			}

			if err != nil {
				t.Errorf("ProcessResource(...): unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultDiffProcessor_ProcessAll(t *testing.T) {
	// Setup test context
	ctx := context.Background()

	// Create test resources
	resource1 := tu.NewResource("example.org/v1", "XR1", "my-xr-1").
		WithSpecField("coolField", "test-value-1").
		Build()

	resource2 := tu.NewResource("example.org/v1", "XR1", "my-xr-2").
		WithSpecField("coolField", "test-value-2").
		Build()

	// Test cases
	tests := []struct {
		name      string
		client    func() *tu.MockClusterClient
		resources []*unstructured.Unstructured
		want      error
	}{
		{
			name: "NoResources",
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulInitialize().
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			resources: []*unstructured.Unstructured{},
			want:      nil,
		},
		{
			name: "ProcessResourceError",
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
		{
			name: "MultipleResourceErrors",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create components for testing
			mockClient := tc.client()
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
				NewEnvironmentConfigProvider([]*unstructured.Unstructured{}, logger),
				logger,
			)
			if err != nil {
				t.Fatalf("Failed to create processor: %v", err)
			}

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err = processor.ProcessAll(&stdout, ctx, tc.resources)

			if tc.want != nil {
				if err == nil {
					t.Errorf("ProcessAll(...): expected error but got none")
					return
				}

				if diff := cmp.Diff(tc.want.Error(), err.Error()); diff != "" {
					t.Errorf("ProcessResource(...): -want error, +got error:\n%s", diff)
				}
				return
			}

			if err != nil {
				t.Errorf("ProcessResource(...): unexpected error: %v", err)
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
	tests := []struct {
		name   string
		client func() *tu.MockClusterClient
		want   error
	}{
		{
			name: "XRDsError",
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithFailedXRDsFetch("XRD not found").
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			want: errors.Wrap(errors.Wrap(errors.New("XRD not found"), "cannot get XRDs"), "cannot load CRDs"),
		},
		{
			name: "EnvConfigsError",
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
		{
			name: "Success",
			client: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{xrd1}).
					WithSuccessfulEnvironmentConfigsFetch([]*unstructured.Unstructured{}).
					Build()
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
				NewEnvironmentConfigProvider([]*unstructured.Unstructured{}, logger),
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

// Helper functions for processor testing

// CreateTestProcessor creates a processor with the provided components for testing
func CreateTestProcessor(
	client cc.ClusterClient,
	resourceManager ResourceManager,
	schemaValidator SchemaValidator,
	diffCalculator DiffCalculator,
	diffRenderer DiffRenderer,
	extraResourceProvider ExtraResourceProvider,
	logger logging.Logger,
) (DiffProcessor, error) {
	// Create processor with custom components
	processor := &DefaultDiffProcessor{
		client:                client,
		resourceManager:       resourceManager,
		schemaValidator:       schemaValidator,
		diffCalculator:        diffCalculator,
		diffRenderer:          diffRenderer,
		extraResourceProvider: extraResourceProvider,
		config: ProcessorConfig{
			Namespace:  "default",
			Colorize:   true,
			Compact:    false,
			Logger:     logger,
			RenderFunc: render.Render,
			RestConfig: &rest.Config{},
		},
	}

	return processor, nil
}
