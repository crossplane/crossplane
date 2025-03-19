package diffprocessor

import (
	"bytes"
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/render"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// Ensure MockDiffProcessor implements the DiffProcessor interface
var _ DiffProcessor = &tu.MockDiffProcessor{}

func TestDiffProcessor_ProcessResource(t *testing.T) {
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
					WithResourcesExist(xr, composedResource). // Add the XR to existing resources
					WithComposedResourcesByOwner(composedResource). // Add composed resource lookup by owner
					WithSuccessfulDryRun().
					WithSuccessfulXRDsFetch([]*unstructured.Unstructured{composedXrd}).
					//WithSuccessfulXRDsFetch([]*unstructured.Unstructured{sampleCRD, composedCRD}).
					//WithSuccessfulXRDsToCRDs(sampleCRD, composedCRD).
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

			// Create the processor with options
			processor, err := NewDiffProcessor(mockClient,
				WithRestConfig(&rest.Config{}),
				WithRenderFunc(mockRenderFn))
			if err != nil {
				t.Fatalf("Failed to create processor: %v", err)
			}

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			// Initialize the processor for the Success case only
			if tc.name == "Success" {
				if err := processor.Initialize(&stdout, ctx); err != nil {
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

func TestDiffProcessor_ProcessAll(t *testing.T) {
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
			want:      errors.New("unable to process resource my-xr-1: cannot find matching composition: composition not found"),
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
			want: errors.New("[unable to process resource my-xr-1: cannot find matching composition: composition not found, " +
				"unable to process resource my-xr-2: cannot find matching composition: composition not found]"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a DiffProcessor with our mock client
			p, _ := NewDiffProcessor(tc.client(), WithRestConfig(&rest.Config{}))

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.ProcessAll(&stdout, ctx, tc.resources)

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

func TestDefaultDiffProcessor_CalculateDiff(t *testing.T) {
	ctx := context.Background()

	// Create some reusable test resources
	existingResource := tu.NewResource("example.org/v1", "ExampleResource", "existing-resource").
		WithSpecField("param", "old-value").
		Build()

	modifiedResource := tu.NewResource("example.org/v1", "ExampleResource", "modified-resource").
		WithSpecField("param1", "old-value1").
		WithSpecField("param2", "old-value2").
		WithSpecField("param3", "unchanged").
		Build()

	nestedResource := tu.NewResource("example.org/v1", "ExampleResource", "nested-resource").
		WithSpecField("simple", "unchanged").
		WithSpecField("nested", map[string]interface{}{
			"field1": "old-nested-value",
			"field2": map[string]interface{}{
				"deepField": "old-deep-value",
			},
		}).
		Build()

	// Resource with array fields
	arrayItems := []interface{}{
		map[string]interface{}{
			"name":  "item1",
			"value": "value1",
		},
		map[string]interface{}{
			"name":  "item2",
			"value": "value2",
		},
	}
	arrayResource := tu.NewResource("example.org/v1", "ExampleResource", "array-resource").
		WithSpecField("items", arrayItems).
		Build()

	// Composed resource
	composedResource := tu.NewResource("example.org/v1", "ComposedResource", "composed-resource").
		WithSpecField("param", "old-value").
		WithLabels(map[string]string{
			"crossplane.io/composite": "parent-xr",
		}).
		WithAnnotations(map[string]string{
			"crossplane.io/composition-resource-name": "resource-a",
		}).
		Build()

	noColorResource := tu.NewResource("example.org/v1", "ExampleResource", "nocolor-resource").
		WithSpecField("param", "old-value").
		Build()

	tests := map[string]struct {
		reason      string
		setupClient func() *tu.MockClusterClient
		ctx         context.Context
		composite   string
		desired     runtime.Object
		expectDiff  string
		expectError error
		noColor     bool
	}{
		"Non-Unstructured Object": {
			reason: "Should return error when desired object is not an unstructured object",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().Build()
			},
			ctx:         ctx,
			composite:   "",
			desired:     &corev1.Pod{}, // Using a typed object to test error handling
			expectError: errors.New("desired object is not unstructured"),
		},
		// Fixed test case for New Resource
		"New Resource": {
			reason: "Should generate a diff for a new resource that doesn't exist in the cluster",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
						// Explicitly return NotFound error for apierrors.IsNotFound to work
						return nil, apierrors.NewNotFound(
							schema.GroupResource{Group: gvk.Group, Resource: strings.ToLower(gvk.Kind)},
							name)
					}).
					Build()
			},
			ctx:       ctx,
			composite: "",
			desired: tu.NewResource("example.org/v1", "ExampleResource", "new-resource").
				WithSpecField("param", "value").
				Build(),
			expectDiff: `+++ ExampleResource/new-resource
` + tu.Green(`+ apiVersion: example.org/v1
+ kind: ExampleResource
+ metadata:
+   name: new-resource
+ spec:
+   param: value`),
		},
		"Error Getting Current Resource": {
			reason: "Should return error when getting the current resource fails",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) (*unstructured.Unstructured, error) {
						return nil, errors.New("get resource error")
					}).
					Build()
			},
			ctx:         ctx,
			composite:   "",
			desired:     tu.NewResource("example.org/v1", "ExampleResource", "error-resource").Build(),
			expectError: errors.New("cannot get current object: get resource error"),
		},
		"Dry Run Apply Error": {
			reason: "Should return error when the dry run apply fails",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(existingResource).
					WithFailedDryRun("dry run apply error").
					Build()
			},
			ctx:       ctx,
			composite: "",
			desired: tu.NewResource("example.org/v1", "ExampleResource", "existing-resource").
				WithSpecField("param", "new-value").
				Build(),
			expectError: errors.New("cannot dry-run apply desired object: dry run apply error"),
		},
		"Modified Simple Fields": {
			reason: "Should generate a diff for a resource with modified simple fields",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(modifiedResource).
					WithSuccessfulDryRun().
					Build()
			},
			ctx:       ctx,
			composite: "",
			desired: tu.NewResource("example.org/v1", "ExampleResource", "modified-resource").
				WithSpecField("param1", "new-value1").
				WithSpecField("param2", "new-value2").
				WithSpecField("param3", "unchanged").
				WithSpecField("param4", "added").
				Build(),
			expectDiff: `~~~ ExampleResource/modified-resource
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
		"Nested Fields": {
			reason: "Should generate a diff for a resource with modified nested fields",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(nestedResource).
					WithSuccessfulDryRun().
					Build()
			},
			ctx:       ctx,
			composite: "",
			desired: tu.NewResource("example.org/v1", "ExampleResource", "nested-resource").
				WithSpecField("simple", "unchanged").
				WithSpecField("nested", map[string]interface{}{
					"field1": "new-nested-value",
					"field2": map[string]interface{}{
						"deepField":      "new-deep-value",
						"addedDeepField": "added-deep-value",
					},
					"field3": "added-field",
				}).
				Build(),
			expectDiff: `~~~ ExampleResource/nested-resource
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
		"Array Fields": {
			reason: "Should generate a diff for a resource with modified array fields",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(arrayResource).
					WithSuccessfulDryRun().
					Build()
			},
			ctx:       ctx,
			composite: "",
			desired: tu.NewResource("example.org/v1", "ExampleResource", "array-resource").
				WithSpecField("items", []interface{}{
					map[string]interface{}{
						"name":  "item1",
						"value": "modified1",
					},
					map[string]interface{}{
						"name":  "item3",
						"value": "value3",
					},
				}).
				Build(),
			expectDiff: `~~~ ExampleResource/array-resource
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
		"Composed Resource": {
			reason: "Should generate a diff for a composed resource using labels to identify it",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(composedResource).
					WithResourcesFoundByLabel([]*unstructured.Unstructured{composedResource}, "crossplane.io/composite", "parent-xr").
					WithSuccessfulDryRun().
					Build()
			},
			ctx:       ctx,
			composite: "parent-xr", // This indicates it's a composed resource
			desired: tu.NewResource("example.org/v1", "ComposedResource", "composed-resource").
				WithSpecField("param", "new-value").
				WithLabels(map[string]string{
					"crossplane.io/composite": "parent-xr",
				}).
				WithAnnotations(map[string]string{
					"crossplane.io/composition-resource-name": "resource-a",
				}).
				Build(),
			expectDiff: `~~~ ComposedResource/composed-resource
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
		"No Color Output": {
			reason: "Should generate a diff without ANSI color codes when colorize is disabled",
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(noColorResource).
					WithSuccessfulDryRun().
					Build()
			},
			ctx:       ctx,
			composite: "",
			desired: tu.NewResource("example.org/v1", "ExampleResource", "nocolor-resource").
				WithSpecField("param", "new-value").
				Build(),
			expectDiff: `~~~ ExampleResource/nocolor-resource
  apiVersion: example.org/v1
  kind: ExampleResource
  metadata:
    name: nocolor-resource
  spec:
-   param: old-value
+   param: new-value`,
			noColor: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a mock client using the mock builder pattern
			mockClient := tt.setupClient()

			// Create processor configuration
			config := ProcessorConfig{
				RestConfig: &rest.Config{},
				Colorize:   !tt.noColor,
			}

			// Create the processor
			p := &DefaultDiffProcessor{
				client: mockClient,
				config: config,
			}

			// Call the function under test
			diff, err := p.CalculateDiff(tt.ctx, tt.composite, tt.desired)

			// Check error expectations
			if tt.expectError != nil {
				if err == nil {
					t.Errorf("%s: CalculateDiff() expected error but got none", tt.reason)
					return
				}
				if !strings.Contains(err.Error(), tt.expectError.Error()) {
					t.Errorf("%s: CalculateDiff() error = %v, want error containing %v", tt.reason, err, tt.expectError)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: CalculateDiff() unexpected error: %v", tt.reason, err)
				return
			}

			// For empty expected diff, verify the result is also empty
			if tt.expectDiff == "" && diff != "" {
				t.Errorf("%s: CalculateDiff() expected empty diff, got: %s", tt.reason, diff)
				return
			}

			// Check if we need to compare expected diff with actual diff
			if tt.expectDiff != "" && err == nil {
				// Normalize both strings by trimming trailing whitespace from each line
				normalizedExpected := normalizeTrailingWhitespace(tt.expectDiff)
				normalizedActual := normalizeTrailingWhitespace(diff)

				// Direct string comparison with normalized strings
				if normalizedExpected != normalizedActual {
					// If they're equal when ignoring ANSI, show escaped ANSI for debugging
					if tu.CompareIgnoringAnsi(tt.expectDiff, diff) {
						t.Errorf("%s: CalculateDiff() diff matches content but ANSI codes differ.\nWant (escaped):\n%q\n\nGot (escaped):\n%q",
							tt.reason, tt.expectDiff, diff)
					} else {
						t.Errorf("%s: CalculateDiff() diff does not match expected.\nWant:\n%s\n\nGot:\n%s",
							tt.reason, tt.expectDiff, diff)
					}
				}
			}
		})
	}
}

func TestDiffProcessor_Initialize(t *testing.T) {
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
			want: errors.Wrap(errors.New("XRD not found"), "cannot get XRDs"),
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
			// Create a DiffProcessor that uses our mock client
			p, _ := NewDiffProcessor(tc.client(), WithRestConfig(&rest.Config{}))

			// Create a dummy writer for stdout
			var stdout bytes.Buffer

			err := p.Initialize(&stdout, ctx)

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
