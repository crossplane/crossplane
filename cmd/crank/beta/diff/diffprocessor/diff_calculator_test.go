package diffprocessor

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	cpd "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	cmp "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	xp "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/crossplane"
	k8 "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/kubernetes"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer"
	dt "github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer/types"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/render"
)

// Ensure MockDiffCalculator implements the DiffCalculator interface.
var _ DiffCalculator = &tu.MockDiffCalculator{}

func TestDefaultDiffCalculator_CalculateDiff(t *testing.T) {
	ctx := context.Background()

	// Create test resources
	existingResource := tu.NewResource("example.org/v1", "TestResource", "existing-resource").
		WithSpecField("field", "old-value").
		Build()

	modifiedResource := tu.NewResource("example.org/v1", "TestResource", "existing-resource").
		WithSpecField("field", "new-value").
		Build()

	newResource := tu.NewResource("example.org/v1", "TestResource", "new-resource").
		WithSpecField("field", "value").
		Build()

	const ParentXRName = "parent-xr"
	composedResource := tu.NewResource("example.org/v1", "ComposedResource", "cpd-resource").
		WithSpecField("field", "old-value").
		WithLabels(map[string]string{
			"crossplane.io/composite": ParentXRName,
		}).
		WithAnnotations(map[string]string{
			"crossplane.io/composition-resource-name": "resource-a",
		}).
		Build()

	// Parent XR
	parentXR := tu.NewResource("example.org/v1", "XR", ParentXRName).
		WithSpecField("field", "value").
		Build()

	tests := map[string]struct {
		setupMocks func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager)
		composite  *un.Unstructured
		desired    *un.Unstructured
		wantDiff   *dt.ResourceDiff
		wantNil    bool
		wantErr    bool
	}{
		"ExistingResourceModified": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client (not used in this test)
				resourceTreeClient := tu.NewMockResourceTreeClient().Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingResource).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			composite: nil,
			desired:   modifiedResource,
			wantDiff: &dt.ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "TestResource", Group: "example.org", Version: "v1"},
				ResourceName: "existing-resource",
				DiffType:     dt.DiffTypeModified,
			},
		},
		"NewResource": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client (not used in this test)
				resourceTreeClient := tu.NewMockResourceTreeClient().Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourceNotFound().
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			composite: nil,
			desired:   newResource,
			wantDiff: &dt.ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "TestResource", Group: "example.org", Version: "v1"},
				ResourceName: "new-resource",
				DiffType:     dt.DiffTypeAdded,
			},
		},
		"ComposedResource": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client (not used in this test)
				resourceTreeClient := tu.NewMockResourceTreeClient().Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(composedResource).
					WithResourcesFoundByLabel([]*un.Unstructured{composedResource}, "crossplane.io/composite", ParentXRName).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			composite: parentXR,
			desired: tu.NewResource("example.org/v1", "ComposedResource", "cpd-resource").
				WithSpecField("field", "new-value").
				WithLabels(map[string]string{
					"crossplane.io/composite": ParentXRName,
				}).
				WithAnnotations(map[string]string{
					"crossplane.io/composition-resource-name": "resource-a",
				}).
				Build(),
			wantDiff: &dt.ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "ComposedResource", Group: "example.org", Version: "v1"},
				ResourceName: "cpd-resource",
				DiffType:     dt.DiffTypeModified,
			},
		},
		"NoChanges": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client (not used in this test)
				resourceTreeClient := tu.NewMockResourceTreeClient().Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingResource).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			composite: nil,
			desired:   existingResource.DeepCopy(),
			wantDiff: &dt.ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "TestResource", Group: "example.org", Version: "v1"},
				ResourceName: "existing-resource",
				DiffType:     dt.DiffTypeEqual,
			},
		},
		"ErrorGettingCurrentObject": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client (not used because test fails earlier)
				applyClient := tu.NewMockApplyClient().Build()

				// Create mock resource tree client (not used in this test)
				resourceTreeClient := tu.NewMockResourceTreeClient().Build()

				// Create mock resource client for resource manager that returns an error
				resourceClient := tu.NewMockResourceClient().
					WithGetResource(func(context.Context, schema.GroupVersionKind, string, string) (*un.Unstructured, error) {
						return nil, errors.New("resource not found")
					}).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			composite: nil,
			desired:   existingResource,
			wantErr:   true,
		},
		"DryRunError": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client that returns an error
				applyClient := tu.NewMockApplyClient().
					WithFailedDryRun("apply error").
					Build()

				// Create mock resource tree client (not used in this test)
				resourceTreeClient := tu.NewMockResourceTreeClient().Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingResource).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			composite: nil,
			desired:   modifiedResource,
			wantErr:   true,
		},
		"Successfully find and diff resource with generateName": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// The composed resource with generateName
				composedWithGenName := tu.NewResource("example.org/v1", "ComposedResource", "").
					WithLabels(map[string]string{
						"crossplane.io/composite": ParentXRName,
					}).
					WithAnnotations(map[string]string{
						"crossplane.io/composition-resource-name": "resource-a",
					}).
					Build()

				// Set generateName instead of name
				composedWithGenName.SetGenerateName("test-resource-")

				// The existing resource on the cluster with a generated name
				existingComposed := tu.NewResource("example.org/v1", "ComposedResource", "test-resource-abc123").
					WithLabels(map[string]string{
						"crossplane.io/composite": ParentXRName,
					}).
					WithAnnotations(map[string]string{
						"crossplane.io/composition-resource-name": "resource-a",
					}).
					WithSpecField("field", "old-value").
					Build()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client (not used in this test)
				resourceTreeClient := tu.NewMockResourceTreeClient().Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					// Return "not found" for direct name lookup
					WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
						// This should fail as the resource has generateName, not name
						if name == "test-resource-abc123" {
							return existingComposed, nil
						}
						return nil, apierrors.NewNotFound(
							schema.GroupResource{
								Group:    gvk.Group,
								Resource: strings.ToLower(gvk.Kind) + "s",
							},
							name,
						)
					}).
					// Return our existing resource when looking up by label
					WithGetResourcesByLabel(func(_ context.Context, _ string, _ schema.GroupVersionKind, sel metav1.LabelSelector) ([]*un.Unstructured, error) {
						// Verify we're looking up with the right composite owner label
						if owner, exists := sel.MatchLabels["crossplane.io/composite"]; exists && owner == ParentXRName {
							return []*un.Unstructured{existingComposed}, nil
						}
						return []*un.Unstructured{}, nil
					}).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			composite: parentXR,
			desired: tu.NewResource("example.org/v1", "ComposedResource", "").
				WithLabels(map[string]string{
					"crossplane.io/composite": ParentXRName,
				}).
				WithAnnotations(map[string]string{
					"crossplane.io/composition-resource-name": "resource-a",
				}).
				WithSpecField("field", "new-value").
				WithGenerateName("test-resource-").
				Build(),
			wantDiff: &dt.ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "ComposedResource", Group: "example.org", Version: "v1"},
				ResourceName: "test-resource-abc123", // Should have found the existing resource name
				DiffType:     dt.DiffTypeModified,    // Should be modified, not added
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := tu.TestLogger(t, false)

			// Setup mocks
			applyClient, resourceTreeClient, resourceManager := tt.setupMocks(t)

			// Setup the diff calculator with the mocks
			calculator := NewDiffCalculator(
				applyClient,
				resourceTreeClient,
				resourceManager,
				logger,
				renderer.DefaultDiffOptions(),
			)

			// Call the function under test
			diff, err := calculator.CalculateDiff(ctx, tt.composite, tt.desired)

			// Check error condition
			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateDiff() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("CalculateDiff() unexpected error: %v", err)
			}

			// Check nil diff case
			if tt.wantNil {
				if diff != nil {
					t.Errorf("CalculateDiff() expected nil diff but got: %v", diff)
				}
				return
			}

			// Check non-nil case
			if diff == nil {
				t.Fatalf("CalculateDiff() returned nil diff, expected non-nil")
			}

			// Check the basics of the diff
			if diff.Gvk != tt.wantDiff.Gvk {
				t.Errorf("Gvk = %v, want %v", diff.Gvk.String(), tt.wantDiff.Gvk.String())
			}

			if diff.ResourceName != tt.wantDiff.ResourceName {
				t.Errorf("ResourceName = %v, want %v", diff.ResourceName, tt.wantDiff.ResourceName)
			}

			if diff.DiffType != tt.wantDiff.DiffType {
				t.Errorf("DiffType = %v, want %v", diff.DiffType, tt.wantDiff.DiffType)
			}

			// For modified resources, check that LineDiffs is populated
			if diff.DiffType == dt.DiffTypeModified && len(diff.LineDiffs) == 0 {
				t.Errorf("LineDiffs is empty for %s", name)
			}
		})
	}
}

func TestDefaultDiffCalculator_CalculateDiffs(t *testing.T) {
	ctx := context.Background()

	// Create test XR
	modifiedXr := tu.NewResource("example.org/v1", "XR", "test-xr").
		WithSpecField("field", "new-value").
		BuildUComposite()

	// Create test rendered resources
	renderedXR := tu.NewResource("example.org/v1", "XR", "test-xr").
		BuildUComposite()

	// Create rendered composed resources
	composedResource1 := tu.NewResource("example.org/v1", "Composed", "cpd-1").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-1").
		WithSpecField("field", "new-value").
		BuildUComposed()

	// Create existing resources for the client to find
	existingXRBuilder := tu.NewResource("example.org/v1", "XR", "test-xr").
		WithSpecField("field", "old-value")
	existingXR := existingXRBuilder.Build()
	existingXrUComp := existingXRBuilder.BuildUComposite()

	existingComposed := tu.NewResource("example.org/v1", "Composed", "cpd-1").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-1").
		WithSpecField("field", "old-value").
		Build()

	tests := map[string]struct {
		setupMocks    func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager)
		inputXR       *cmp.Unstructured
		renderedOut   render.Outputs
		expectedDiffs map[string]dt.DiffType // Map of expected keys and their diff types
		wantErr       bool
	}{
		"XR and composed resource modifications": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client
				resourceTreeClient := tu.NewMockResourceTreeClient().
					WithEmptyResourceTree().
					Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingXR, existingComposed).
					WithResourcesFoundByLabel([]*un.Unstructured{existingComposed}, "crossplane.io/composite", "test-xr").
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			inputXR: modifiedXr,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				ComposedResources: []cpd.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]dt.DiffType{
				"example.org/v1/XR/test-xr":     dt.DiffTypeModified,
				"example.org/v1/Composed/cpd-1": dt.DiffTypeModified,
			},
			wantErr: false,
		},
		"XR not modified, composed resource modified": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client
				resourceTreeClient := tu.NewMockResourceTreeClient().
					WithEmptyResourceTree().
					Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingXR, existingComposed).
					WithResourcesFoundByLabel([]*un.Unstructured{existingComposed}, "crossplane.io/composite", "test-xr").
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			inputXR: existingXrUComp,
			renderedOut: render.Outputs{
				CompositeResource: func() *cmp.Unstructured {
					// Create XR with same values (no changes)
					sameXR := &cmp.Unstructured{}
					sameXR.SetUnstructuredContent(existingXR.UnstructuredContent())
					return sameXR
				}(),
				ComposedResources: []cpd.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]dt.DiffType{
				"example.org/v1/Composed/cpd-1": dt.DiffTypeModified,
			},
			wantErr: false,
		},
		"Error calculating diff": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create mock apply client that returns an error
				applyClient := tu.NewMockApplyClient().
					WithFailedDryRun("dry run error").
					Build()

				// Create mock resource tree client
				resourceTreeClient := tu.NewMockResourceTreeClient().
					Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingXR, existingComposed).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			inputXR: existingXrUComp,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				ComposedResources: []cpd.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]dt.DiffType{},
			wantErr:       true,
		},
		"Resource tree with potential resources to remove": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create a resource that isn't in the rendered output
				extraComposedResource := tu.NewResource("example.org/v1", "Composed", "cpd-2").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-to-be-removed").
					WithSpecField("field", "value").
					Build()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client with the XR as root and some composed resources as children
				resourceTreeClient := tu.NewMockResourceTreeClient().
					WithResourceTreeFromXRAndComposed(existingXR, []*un.Unstructured{
						existingComposed,
						extraComposedResource,
					}).
					Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingXR, existingComposed, extraComposedResource).
					WithResourcesFoundByLabel([]*un.Unstructured{existingComposed}, "crossplane.io/composite", "test-xr").
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			inputXR: modifiedXr,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				ComposedResources: []cpd.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]dt.DiffType{
				"example.org/v1/XR/test-xr":     dt.DiffTypeModified,
				"example.org/v1/Composed/cpd-1": dt.DiffTypeModified,
				"example.org/v1/Composed/cpd-2": dt.DiffTypeRemoved,
			},
			wantErr: false,
		},
		"Resource removal detection": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create existing version of the resource
				existingComposedWithOldValue := tu.NewResource("example.org/v1", "Composed", "cpd-1").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-1").
					WithSpecField("field", "old-value").
					Build()

				// Create an extra resource that should be removed
				extraResource := tu.NewResource("example.org/v1", "Composed", "resource-to-remove").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-to-remove").
					Build()

				// Create mock apply client
				applyClient := tu.NewMockApplyClient().
					WithSuccessfulDryRun().
					Build()

				// Create mock resource tree client
				resourceTreeClient := tu.NewMockResourceTreeClient().
					WithResourceTreeFromXRAndComposed(
						existingXR,
						[]*un.Unstructured{existingComposedWithOldValue, extraResource},
					).
					Build()

				// Create mock resource client for resource manager
				resourceClient := tu.NewMockResourceClient().
					WithResourcesExist(existingXR, existingComposedWithOldValue, extraResource).
					WithResourcesFoundByLabel(
						[]*un.Unstructured{existingComposedWithOldValue, extraResource},
						"crossplane.io/composite",
						"test-xr",
					).
					Build()

				// Create resource manager
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			inputXR: modifiedXr,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				// Include a modified version of composedResource1 with new value
				ComposedResources: []cpd.Unstructured{*tu.NewResource("example.org/v1", "Composed", "cpd-1").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-1").
					WithSpecField("field", "new-value"). // Different value than existing
					BuildUComposed()},
			},
			expectedDiffs: map[string]dt.DiffType{
				"example.org/v1/XR/test-xr":                  dt.DiffTypeModified,
				"example.org/v1/Composed/cpd-1":              dt.DiffTypeModified,
				"example.org/v1/Composed/resource-to-remove": dt.DiffTypeRemoved,
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := tu.TestLogger(t, false)

			// Setup mocks
			applyClient, resourceTreeClient, resourceManager := tt.setupMocks(t)

			// Create a diff calculator with default options
			calculator := NewDiffCalculator(
				applyClient,
				resourceTreeClient,
				resourceManager,
				logger,
				renderer.DefaultDiffOptions(),
			)

			// Call the function under test
			diffs, err := calculator.CalculateDiffs(ctx, tt.inputXR, tt.renderedOut)

			// Check error condition
			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateDiffs() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("CalculateDiffs() unexpected error: %v", err)
			}

			// Check that we have the expected number of diffs
			if len(diffs) != len(tt.expectedDiffs) {
				t.Errorf("CalculateDiffs() returned %d diffs, want %d", len(diffs), len(tt.expectedDiffs))

				// Print what diffs we actually got to help debug
				for key, diff := range diffs {
					t.Logf("Found diff: %s of type %s", key, diff.DiffType)
				}
			}

			// Check each expected diff
			for expectedKey, expectedType := range tt.expectedDiffs {
				diff, found := diffs[expectedKey]
				if !found {
					t.Errorf("CalculateDiffs() missing expected diff for key %s", expectedKey)
					continue
				}

				if diff.DiffType != expectedType {
					t.Errorf("CalculateDiffs() diff for key %s has type %s, want %s",
						expectedKey, diff.DiffType, expectedType)
				}

				// Check that LineDiffs is not empty for non-nil diffs
				if len(diff.LineDiffs) == 0 {
					t.Errorf("CalculateDiffs() returned diff with empty LineDiffs for key %s", expectedKey)
				}
			}

			// Check for unexpected diffs
			for key := range diffs {
				if _, expected := tt.expectedDiffs[key]; !expected {
					t.Errorf("CalculateDiffs() returned unexpected diff for key %s", key)
				}
			}
		})
	}
}

func TestDefaultDiffCalculator_CalculateRemovedResourceDiffs(t *testing.T) {
	ctx := context.Background()

	// Create a test XR
	xr := tu.NewResource("example.org/v1", "XR", "test-xr").
		Build()

	// Create a resource tree with two resources
	resourceToKeep := tu.NewResource("example.org/v1", "Composed", "resource-to-keep").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-to-keep").
		Build()

	resourceToRemove := tu.NewResource("example.org/v1", "Composed", "resource-to-remove").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-to-remove").
		Build()

	tests := map[string]struct {
		setupMocks        func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager)
		renderedResources map[string]bool
		expectedRemoved   []string
		wantErr           bool
	}{
		"IdentifiesRemovedResources": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create a mock apply client (not used in this test)
				applyClient := tu.NewMockApplyClient().Build()

				// Create a resource tree client that returns a tree with both resources
				resourceTreeClient := tu.NewMockResourceTreeClient().
					WithResourceTreeFromXRAndComposed(
						xr,
						[]*un.Unstructured{resourceToKeep, resourceToRemove},
					).
					Build()

				// Create a resource manager (not directly used in this test)
				resourceClient := tu.NewMockResourceClient().Build()
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			// Only include the "resource-to-keep" in rendered resources
			renderedResources: map[string]bool{
				"example.org/v1/Composed/resource-to-keep": true,
				// "example.org/v1/Composed/resource-to-remove" intentionally not included
			},
			expectedRemoved: []string{"resource-to-remove"},
			wantErr:         false,
		},
		"NoRemovedResources": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create a mock apply client (not used in this test)
				applyClient := tu.NewMockApplyClient().Build()

				// Create a resource tree client that returns a tree with both resources
				resourceTreeClient := tu.NewMockResourceTreeClient().
					WithResourceTreeFromXRAndComposed(
						xr,
						[]*un.Unstructured{resourceToKeep, resourceToRemove},
					).
					Build()

				// Create a resource manager (not directly used in this test)
				resourceClient := tu.NewMockResourceClient().Build()
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			// Include all resources in rendered resources (nothing to remove)
			renderedResources: map[string]bool{
				"example.org/v1/Composed/resource-to-keep":   true,
				"example.org/v1/Composed/resource-to-remove": true,
			},
			expectedRemoved: []string{},
			wantErr:         false,
		},
		"ErrorGettingResourceTree": {
			setupMocks: func(t *testing.T) (k8.ApplyClient, xp.ResourceTreeClient, ResourceManager) {
				t.Helper()

				// Create a mock apply client (not used in this test)
				applyClient := tu.NewMockApplyClient().Build()

				// Create a resource tree client that returns an error
				resourceTreeClient := tu.NewMockResourceTreeClient().
					WithFailedResourceTreeFetch("failed to get resource tree").
					Build()

				// Create a resource manager (not directly used in this test)
				resourceClient := tu.NewMockResourceClient().Build()
				resourceManager := NewResourceManager(resourceClient, tu.TestLogger(t, false))

				return applyClient, resourceTreeClient, resourceManager
			},
			renderedResources: map[string]bool{},
			expectedRemoved:   []string{},
			wantErr:           true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := tu.TestLogger(t, false)

			// Setup mocks
			applyClient, resourceTreeClient, resourceManager := tt.setupMocks(t)

			// Create a diff calculator with the mocks
			calculator := NewDiffCalculator(
				applyClient,
				resourceTreeClient,
				resourceManager,
				logger,
				renderer.DefaultDiffOptions(),
			)

			// Call the method under test
			diffs, err := calculator.CalculateRemovedResourceDiffs(ctx, xr, tt.renderedResources)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateRemovedResourceDiffs() expected error but got none")
				}
				return
			}

			// Even if we expect a warning-level error, we shouldn't get an actual error return
			if err != nil {
				t.Errorf("CalculateRemovedResourceDiffs() unexpected error: %v", err)
				return
			}

			// Check that the correct resources were identified for removal
			if len(diffs) != len(tt.expectedRemoved) {
				t.Errorf("CalculateRemovedResourceDiffs() found %d resources to remove, want %d",
					len(diffs), len(tt.expectedRemoved))

				// Log what we found for debugging
				for key := range diffs {
					t.Logf("Found resource to remove: %s", key)
				}
				return
			}

			// Verify each expected removed resource is in the result
			for _, name := range tt.expectedRemoved {
				found := false
				for key, diff := range diffs {
					if strings.Contains(key, name) && diff.DiffType == dt.DiffTypeRemoved {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find %s marked for removal but did not", name)
				}
			}
		})
	}
}
