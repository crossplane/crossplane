package diffprocessor

import (
	"bytes"
	"context"
	"fmt"
	cpd "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	cmp "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xp "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/crossplane"
	k8 "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/kubernetes"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer"
	dt "github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer/types"
	"github.com/sergi/go-diff/diffmatchpatch"
	"io"
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
	composedResource := tu.NewResource("cpd.org/v1", "ComposedResource", "resource1").
		WithCompositeOwner("my-xr-1").
		WithCompositionResourceName("resA").
		WithSpecField("param", "value").
		Build()

	// Test cases
	tests := map[string]struct {
		setupMocks      func() (k8.Clients, xp.Clients)
		resources       []*un.Unstructured
		processorOpts   []ProcessorOption
		verifyOutput    func(t *testing.T, output string)
		want            error
		validationError bool
	}{
		"NoResources": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type:     tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().Build(),
					Definition:  tu.NewMockDefinitionClient().Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function:     tu.NewMockFunctionClient().Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			resources: []*un.Unstructured{},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: nil,
		},
		"DiffSingleResourceError": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type:     tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().
						WithNoMatchingComposition().
						Build(),
					Definition: tu.NewMockDefinitionClient().Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function:     tu.NewMockFunctionClient().Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			resources: []*un.Unstructured{resource1},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: errors.New("unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found"),
		},
		"MultipleResourceErrors": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type:     tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().
						WithNoMatchingComposition().
						Build(),
					Definition: tu.NewMockDefinitionClient().Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function:     tu.NewMockFunctionClient().Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			resources: []*un.Unstructured{resource1, resource2},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: errors.New("[unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found, " +
				"unable to process resource XR1/my-xr-2: cannot find matching composition: composition not found]"),
		},
		"CompositionNotFound": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type:     tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().
						WithNoMatchingComposition().
						Build(),
					Definition: tu.NewMockDefinitionClient().Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function:     tu.NewMockFunctionClient().Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			resources: []*un.Unstructured{resource1},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: errors.New("unable to process resource XR1/my-xr-1: cannot find matching composition: composition not found"),
		},
		"GetFunctionsError": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type:     tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().
						WithSuccessfulCompositionMatch(composition).
						Build(),
					Definition: tu.NewMockDefinitionClient().Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function: tu.NewMockFunctionClient().
						WithFailedFunctionsFetch("function not found").
						Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			resources: []*un.Unstructured{resource1},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: errors.New("unable to process resource XR1/my-xr-1: cannot get functions from pipeline: function not found"),
		},
		"SuccessfulDiff": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create mock functions that render will call successfully
				functions := []pkgv1.Function{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-test",
						},
					},
				}

				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply: tu.NewMockApplyClient().
						WithSuccessfulDryRun().
						Build(),
					Resource: tu.NewMockResourceClient().
						WithResourcesExist(resource1, composedResource). // Add resources to existing resources
						WithResourcesFoundByLabel([]*un.Unstructured{composedResource}, "crossplane.io/composite", "test-xr").
						Build(),
					Schema: tu.NewMockSchemaClient().
						WithNoResourcesRequiringCRDs().
						Build(),
					Type: tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().
						WithSuccessfulCompositionMatch(composition).
						Build(),
					Definition: tu.NewMockDefinitionClient().
						WithSuccessfulXRDsFetch([]*un.Unstructured{}).
						Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function: tu.NewMockFunctionClient().
						WithSuccessfulFunctionsFetch(functions).
						Build(),
					ResourceTree: tu.NewMockResourceTreeClient().
						WithEmptyResourceTree().
						Build(),
				}

				return k8sClients, xpClients
			},
			resources: []*un.Unstructured{resource1},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
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
				// Override the schema validator factory to use a simple validator
				WithSchemaValidatorFactory(func(k8.SchemaClient, xp.DefinitionClient, logging.Logger) SchemaValidator {
					return &tu.MockSchemaValidator{
						ValidateResourcesFn: func(context.Context, *un.Unstructured, []cpd.Unstructured) error {
							return nil
						},
					}
				}),
				// Override the diff calculator factory to return actual diffs
				WithDiffCalculatorFactory(func(k8.ApplyClient, xp.ResourceTreeClient, ResourceManager, logging.Logger, renderer.DiffOptions) DiffCalculator {
					return &tu.MockDiffCalculator{
						CalculateDiffsFn: func(context.Context, *cmp.Unstructured, render.Outputs) (map[string]*dt.ResourceDiff, error) {
							diffs := make(map[string]*dt.ResourceDiff)

							// Add a modified diff (not just equal)
							lineDiffs := []diffmatchpatch.Diff{
								{Type: diffmatchpatch.DiffDelete, Text: "  field: old-value"},
								{Type: diffmatchpatch.DiffInsert, Text: "  field: new-value"},
							}

							diffs["example.org/v1/XR1/test-xr"] = &dt.ResourceDiff{
								Gvk:          schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XR1"},
								ResourceName: "test-xr",
								DiffType:     dt.DiffTypeModified,
								LineDiffs:    lineDiffs, // Add line diffs
								Current:      resource1, // Set current for completeness
								Desired:      resource1, // Set desired for completeness
							}

							// Add a composed resource diff that's also modified
							diffs["example.org/v1/ComposedResource/resource-a"] = &dt.ResourceDiff{
								Gvk:          schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "ComposedResource"},
								ResourceName: "resource-a",
								DiffType:     dt.DiffTypeModified,
								LineDiffs:    lineDiffs,
								Current:      composedResource,
								Desired:      composedResource,
							}

							return diffs, nil
						},
					}
				}),
				// Override the diff renderer factory to produce actual output
				WithDiffRendererFactory(func(logging.Logger, renderer.DiffOptions) renderer.DiffRenderer {
					return &tu.MockDiffRenderer{
						RenderDiffsFn: func(w io.Writer, _ map[string]*dt.ResourceDiff) error {
							// Write a simple summary to the output
							_, err := fmt.Fprintln(w, "Changes will be applied to 2 resources:")
							if err != nil {
								return err
							}
							_, err = fmt.Fprintln(w, "- example.org/v1/XR1/test-xr will be modified")
							if err != nil {
								return err
							}
							_, err = fmt.Fprintln(w, "- example.org/v1/ComposedResource/resource-a will be modified")
							if err != nil {
								return err
							}
							_, err = fmt.Fprintln(w, "\nSummary: 0 to create, 2 to modify, 0 to delete")

							return err
						},
					}
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
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create mock functions that render will call successfully
				functions := []pkgv1.Function{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-test",
						},
					},
				}

				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply: tu.NewMockApplyClient().
						WithSuccessfulDryRun().
						Build(),
					Resource: tu.NewMockResourceClient().
						WithResourcesExist(resource1).
						Build(),
					Schema: tu.NewMockSchemaClient().
						WithNoResourcesRequiringCRDs().
						Build(),
					Type: tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().
						WithSuccessfulCompositionMatch(composition).
						Build(),
					Definition: tu.NewMockDefinitionClient().Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function: tu.NewMockFunctionClient().
						WithSuccessfulFunctionsFetch(functions).
						Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			resources: []*un.Unstructured{resource1},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
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
				// Override with a validator that fails
				WithSchemaValidatorFactory(func(_ k8.SchemaClient, _ xp.DefinitionClient, _ logging.Logger) SchemaValidator {
					return &tu.MockSchemaValidator{
						ValidateResourcesFn: func(context.Context, *un.Unstructured, []cpd.Unstructured) error {
							return errors.New("validation error")
						},
					}
				}),
			},
			want:            errors.New("unable to process resource XR1/my-xr-1: cannot validate resources: validation error"),
			validationError: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Create components for testing
			k8sClients, xpClients := tt.setupMocks()

			// Create the diff processor
			processor := NewDiffProcessor(k8sClients, xpClients, tt.processorOpts...)

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

			// Check output if verification function is provided
			if tt.verifyOutput != nil {
				tt.verifyOutput(t, stdout.String())
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
		setupMocks    func() (k8.Clients, xp.Clients)
		processorOpts []ProcessorOption
		want          error
	}{
		"XRDsError": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type:     tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks with a failing Definition client
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().Build(),
					Definition: tu.NewMockDefinitionClient().
						WithFailedXRDsFetch("XRD not found").
						Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function:     tu.NewMockFunctionClient().Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: errors.Wrap(errors.Wrap(errors.New("XRD not found"), "cannot get XRDs"), "cannot load CRDs"),
		},
		"EnvConfigsError": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type:     tu.NewMockTypeConverter().Build(),
				}

				// Create Crossplane client mocks with a failing Environment client
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().Build(),
					Definition: tu.NewMockDefinitionClient().
						WithSuccessfulXRDsFetch([]*un.Unstructured{}).
						Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithGetEnvironmentConfigs(func(_ context.Context) ([]*un.Unstructured, error) {
							return nil, errors.New("env configs not found")
						}).
						Build(),
					Function:     tu.NewMockFunctionClient().Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: errors.Wrap(errors.New("env configs not found"), "cannot get environment configs"),
		},
		"Success": {
			setupMocks: func() (k8.Clients, xp.Clients) {
				// Create Kubernetes client mocks
				k8sClients := k8.Clients{
					Apply:    tu.NewMockApplyClient().Build(),
					Resource: tu.NewMockResourceClient().Build(),
					Schema:   tu.NewMockSchemaClient().Build(),
					Type: tu.NewMockTypeConverter().
						WithDefaultGVKToGVR().
						Build(),
				}

				// Create Crossplane client mocks with successful initialization
				xpClients := xp.Clients{
					Composition: tu.NewMockCompositionClient().Build(),
					Definition: tu.NewMockDefinitionClient().
						WithSuccessfulXRDsFetch([]*un.Unstructured{xrd1}).
						Build(),
					Environment: tu.NewMockEnvironmentClient().
						WithSuccessfulEnvironmentConfigsFetch([]*un.Unstructured{}).
						Build(),
					Function:     tu.NewMockFunctionClient().Build(),
					ResourceTree: tu.NewMockResourceTreeClient().Build(),
				}

				return k8sClients, xpClients
			},
			processorOpts: []ProcessorOption{
				WithLogger(tu.TestLogger(t, false)),
			},
			want: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Get the clients for this test
			k8sClients, xpClients := tc.setupMocks()

			// Build processor options
			options := tc.processorOpts

			// Create the processor
			processor := NewDiffProcessor(k8sClients, xpClients, options...)

			// Call the Initialize method
			err := processor.Initialize(ctx)

			// Verify error expectations
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
	const ConfigMap = "ConfigMap"
	const ConfigMapName = "config1"
	configMap := tu.NewResource("v1", ConfigMap, ConfigMapName).Build()
	secret := tu.NewResource("v1", "Secret", "secret1").Build()

	tests := map[string]struct {
		xr                     *cmp.Unstructured
		composition            *apiextensionsv1.Composition
		functions              []pkgv1.Function
		resourceID             string
		setupResourceClient    func() *tu.MockResourceClient
		setupEnvironmentClient func() *tu.MockEnvironmentClient
		setupRenderFunc        func() RenderFunc
		wantComposedCount      int
		wantRenderIterations   int
		wantErr                bool
	}{
		"NoRequirements": {
			xr:          xr,
			composition: composition,
			functions:   functions,
			resourceID:  "XR/test-xr",
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
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
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == ConfigMap && name == ConfigMapName {
							return configMap, nil
						}
						return nil, errors.New("resource not found")
					}).
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
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
										Kind:       ConfigMap,
										Match: &v1.ResourceSelector_MatchName{
											MatchName: ConfigMapName,
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
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == ConfigMap && name == ConfigMapName {
							return configMap, nil
						}
						if gvk.Kind == "Secret" && name == "secret1" {
							return secret, nil
						}
						return nil, errors.New("resource not found")
					}).
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
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
						if res.GetKind() == ConfigMap && res.GetName() == ConfigMapName {
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
								Kind:       ConfigMap,
								Match: &v1.ResourceSelector_MatchName{
									MatchName: ConfigMapName,
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
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
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
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						if gvk.Kind == ConfigMap && name == ConfigMapName {
							return configMap, nil
						}
						return nil, errors.New("resource not found")
					}).
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
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
										Kind:       ConfigMap,
										Match: &v1.ResourceSelector_MatchName{
											MatchName: ConfigMapName,
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
			setupResourceClient: func() *tu.MockResourceClient {
				return tu.NewMockResourceClient().
					WithResourceNotFound().
					Build()
			},
			setupEnvironmentClient: func() *tu.MockEnvironmentClient {
				return tu.NewMockEnvironmentClient().
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
									Kind:       ConfigMap,
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
			// Set up mock clients
			resourceClient := tt.setupResourceClient()
			environmentClient := tt.setupEnvironmentClient()

			// Create a logger
			logger := tu.TestLogger(t, false)
			renderFunc := tt.setupRenderFunc()

			// Create a render iteration counter to verify
			renderCount := 0
			countingRenderFunc := func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error) {
				renderCount++
				return renderFunc(ctx, log, in)
			}

			// Create the requirements provider
			requirementsProvider := NewRequirementsProvider(
				resourceClient,
				environmentClient,
				countingRenderFunc,
				logger,
			)

			// Build processor options
			processor := NewDiffProcessor(k8.Clients{}, xp.Clients{},
				WithLogger(logger),
				WithRenderFunc(countingRenderFunc),
				WithRequirementsProviderFactory(func(k8.ResourceClient, xp.EnvironmentClient, RenderFunc, logging.Logger) *RequirementsProvider {
					return requirementsProvider
				}),
			)

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

			// Check composed resource count
			if len(output.ComposedResources) != tt.wantComposedCount {
				t.Errorf("RenderWithRequirements() returned %d composed resources, want %d",
					len(output.ComposedResources), tt.wantComposedCount)
			}
		})
	}
}
