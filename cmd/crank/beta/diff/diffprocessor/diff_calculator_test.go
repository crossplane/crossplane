package diffprocessor

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/crossplane/crossplane/cmd/crank/render"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

	composedResource := tu.NewResource("example.org/v1", "ComposedResource", "composed-resource").
		WithSpecField("field", "old-value").
		WithLabels(map[string]string{
			"crossplane.io/composite": "parent-xr",
		}).
		WithAnnotations(map[string]string{
			"crossplane.io/composition-resource-name": "resource-a",
		}).
		Build()

	tests := map[string]struct {
		setupClient func() *tu.MockClusterClient
		composite   *unstructured.Unstructured
		desired     *unstructured.Unstructured
		wantDiff    *ResourceDiff
		wantNil     bool
		wantErr     bool
	}{
		"ExistingResourceModified": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(existingResource).
					WithSuccessfulDryRun().
					Build()
			},
			composite: nil,
			desired:   modifiedResource,
			wantDiff: &ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "TestResource", Group: "example.org", Version: "v1"},
				ResourceName: "existing-resource",
				DiffType:     DiffTypeModified,
			},
		},
		"NewResource": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourceNotFound().
					WithSuccessfulDryRun().
					Build()
			},
			composite: nil,
			desired:   newResource,
			wantDiff: &ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "TestResource", Group: "example.org", Version: "v1"},
				ResourceName: "new-resource",
				DiffType:     DiffTypeAdded,
			},
		},
		"ComposedResource": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesFoundByLabel([]*unstructured.Unstructured{composedResource}, "crossplane.io/composite", "parent-xr").
					// Add this line to mock the GetResource function:
					WithResourcesExist(composedResource).
					WithSuccessfulDryRun().
					Build()
			},
			composite: tu.NewResource("foo", "bar", "parent-xr").Build(),
			desired: tu.NewResource("example.org/v1", "ComposedResource", "composed-resource").
				WithSpecField("field", "new-value").
				WithLabels(map[string]string{
					"crossplane.io/composite": "parent-xr",
				}).
				WithAnnotations(map[string]string{
					"crossplane.io/composition-resource-name": "resource-a",
				}).
				Build(),
			wantDiff: &ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "ComposedResource", Group: "example.org", Version: "v1"},
				ResourceName: "composed-resource",
				DiffType:     DiffTypeModified,
			},
		},
		"NoChanges": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(existingResource).
					WithSuccessfulDryRun().
					Build()
			},
			composite: nil,
			desired:   existingResource.DeepCopy(),
			wantDiff: &ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "TestResource", Group: "example.org", Version: "v1"},
				ResourceName: "existing-resource",
				DiffType:     DiffTypeEqual,
			},
		},
		"ErrorGettingCurrentObject": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
						return nil, errors.New("resource not found")
					}).
					Build()
			},
			composite: nil,
			desired:   existingResource,
			wantErr:   true,
		},
		"DryRunError": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(existingResource).
					WithFailedDryRun("apply error").
					Build()
			},
			composite: nil,
			desired:   modifiedResource,
			wantErr:   true,
		},
		"Successfully find and diff resource with generateName": {
			setupClient: func() *tu.MockClusterClient {
				// The composed resource with generateName
				composedWithGenName := tu.NewResource("example.org/v1", "ComposedResource", "").
					WithLabels(map[string]string{
						"crossplane.io/composite": "parent-xr",
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
						"crossplane.io/composite": "parent-xr",
					}).
					WithAnnotations(map[string]string{
						"crossplane.io/composition-resource-name": "resource-a",
					}).
					WithSpecField("field", "old-value").
					Build()

				// Create a mock client that will return resources by label
				return tu.NewMockClusterClient().
					// Return "not found" for direct name lookup
					WithGetResource(func(ctx context.Context, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
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
					WithGetResourcesByLabel(func(ctx context.Context, ns string, gvr schema.GroupVersionKind, sel metav1.LabelSelector) ([]*unstructured.Unstructured, error) {
						// Verify we're looking up with the right composite owner label
						if owner, exists := sel.MatchLabels["crossplane.io/composite"]; exists && owner == "parent-xr" {
							return []*unstructured.Unstructured{existingComposed}, nil
						}
						return []*unstructured.Unstructured{}, nil
					}).
					WithSuccessfulDryRun().
					Build()
			},
			composite: tu.NewResource("example.org/v1", "XR", "parent-xr").Build(),
			desired: tu.NewResource("example.org/v1", "ComposedResource", "").
				WithLabels(map[string]string{
					"crossplane.io/composite": "parent-xr",
				}).
				WithAnnotations(map[string]string{
					"crossplane.io/composition-resource-name": "resource-a",
				}).
				WithSpecField("field", "new-value").
				WithGenerateName("test-resource-").
				Build(),
			wantDiff: &ResourceDiff{
				Gvk:          schema.GroupVersionKind{Kind: "ComposedResource", Group: "example.org", Version: "v1"},
				ResourceName: "test-resource-abc123", // Should have found the existing resource name
				DiffType:     DiffTypeModified,       // Should be modified, not added
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := tu.TestLogger(t)

			// Setup a resource manager
			resourceManager := NewResourceManager(tt.setupClient(), logger)

			// Setup the diff calculator with the resource manager
			calculator := NewDiffCalculator(
				tt.setupClient(),
				resourceManager,
				logger,
				DefaultDiffOptions(),
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
			if diff.DiffType == DiffTypeModified && len(diff.LineDiffs) == 0 {
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
	composedResource1 := tu.NewResource("example.org/v1", "Composed", "composed-1").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-1").
		WithSpecField("field", "new-value").
		BuildUComposed()

	// Create existing resources for the client to find
	existingXRBuilder := tu.NewResource("example.org/v1", "XR", "test-xr").
		WithSpecField("field", "old-value")
	existingXR := existingXRBuilder.Build()
	existingXrUComp := existingXRBuilder.BuildUComposite()

	existingComposed := tu.NewResource("example.org/v1", "Composed", "composed-1").
		WithCompositeOwner("test-xr").
		WithCompositionResourceName("resource-1").
		WithSpecField("field", "old-value").
		Build()

	tests := map[string]struct {
		setupClient   func() *tu.MockClusterClient
		inputXR       *ucomposite.Unstructured
		renderedOut   render.Outputs
		expectedDiffs map[string]DiffType // Map of expected keys and their diff types
		wantErr       bool
	}{
		"XR and composed resource modifications": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(existingXR, existingComposed).
					WithResourcesFoundByLabel([]*unstructured.Unstructured{existingComposed}, "crossplane.io/composite", "test-xr").
					WithSuccessfulDryRun().
					WithEmptyResourceTree().
					Build()
			},
			inputXR: modifiedXr,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				ComposedResources: []composed.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]DiffType{
				"example.org/v1/XR/test-xr":          DiffTypeModified,
				"example.org/v1/Composed/composed-1": DiffTypeModified,
			},
			wantErr: false,
		},
		"XR not modified, composed resource modified": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(existingXR, existingComposed).
					WithResourcesFoundByLabel([]*unstructured.Unstructured{existingComposed}, "crossplane.io/composite", "test-xr").
					WithSuccessfulDryRun().
					WithEmptyResourceTree().
					Build()
			},
			inputXR: existingXrUComp,
			renderedOut: render.Outputs{
				CompositeResource: func() *ucomposite.Unstructured {
					// Create XR with same values (no changes)
					sameXR := &ucomposite.Unstructured{}
					sameXR.SetUnstructuredContent(existingXR.UnstructuredContent())
					return sameXR
				}(),
				ComposedResources: []composed.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]DiffType{
				"example.org/v1/Composed/composed-1": DiffTypeModified,
			},
			wantErr: false,
		},
		"Error calculating diff": {
			setupClient: func() *tu.MockClusterClient {
				// Return error from dry run
				return tu.NewMockClusterClient().
					WithResourcesExist(existingXR, existingComposed).
					WithFailedDryRun("dry run error").
					Build()
			},
			inputXR: existingXrUComp,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				ComposedResources: []composed.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]DiffType{},
			wantErr:       true,
		},
		"Resource tree with potential resources to remove": {
			setupClient: func() *tu.MockClusterClient {
				// Create a resource tree with resources that aren't in the rendered output
				extraComposedResource := tu.NewResource("example.org/v1", "Composed", "composed-2").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-to-be-removed").
					WithSpecField("field", "value").
					Build()

				// Return a resource tree with the XR as root and some composed resources as children
				return tu.NewMockClusterClient().
					WithResourcesExist(existingXR, existingComposed, extraComposedResource).
					WithResourcesFoundByLabel([]*unstructured.Unstructured{existingComposed}, "crossplane.io/composite", "test-xr").
					WithSuccessfulDryRun().
					WithResourceTreeFromXRAndComposed(existingXR, []*unstructured.Unstructured{
						existingComposed,
						extraComposedResource,
					}).
					Build()
			},
			inputXR: modifiedXr,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				ComposedResources: []composed.Unstructured{*composedResource1},
			},
			expectedDiffs: map[string]DiffType{
				"example.org/v1/XR/test-xr":          DiffTypeModified,
				"example.org/v1/Composed/composed-1": DiffTypeModified,
				"example.org/v1/Composed/composed-2": DiffTypeRemoved,
			},
			wantErr: false,
		},
		"Resource removal detection": {
			setupClient: func() *tu.MockClusterClient {
				// Create existing version of the resource
				existingComposedWithOldValue := tu.NewResource("example.org/v1", "Composed", "composed-1").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-1").
					WithSpecField("field", "old-value").
					Build()

				// Create an extra resource that should be removed
				extraResource := tu.NewResource("example.org/v1", "Composed", "resource-to-remove").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-to-remove").
					Build()

				return tu.NewMockClusterClient().
					// Make existingComposedWithOldValue available via GetResource
					WithResourcesExist(existingXR, existingComposedWithOldValue, extraResource).
					WithResourcesFoundByLabel(
						[]*unstructured.Unstructured{existingComposedWithOldValue, extraResource},
						"crossplane.io/composite",
						"test-xr",
					).
					// Include both resources in the tree
					WithResourceTreeFromXRAndComposed(
						existingXR,
						[]*unstructured.Unstructured{existingComposedWithOldValue, extraResource},
					).
					WithSuccessfulDryRun().
					Build()
			},
			inputXR: modifiedXr,
			renderedOut: render.Outputs{
				CompositeResource: renderedXR,
				// Include a modified version of composedResource1 with new value
				ComposedResources: []composed.Unstructured{*tu.NewResource("example.org/v1", "Composed", "composed-1").
					WithCompositeOwner("test-xr").
					WithCompositionResourceName("resource-1").
					WithSpecField("field", "new-value"). // Different value than existing
					BuildUComposed()},
			},
			expectedDiffs: map[string]DiffType{
				"example.org/v1/XR/test-xr":                  DiffTypeModified,
				"example.org/v1/Composed/composed-1":         DiffTypeModified,
				"example.org/v1/Composed/resource-to-remove": DiffTypeRemoved,
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := tu.TestLogger(t)

			// Setup the mock client
			mockClient := tt.setupClient()

			// Create resource manager
			resourceManager := NewResourceManager(mockClient, logger)

			// Create a diff calculator with default options
			calculator := NewDiffCalculator(
				mockClient,
				resourceManager,
				logger,
				DefaultDiffOptions(),
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
		setupClient       func() *tu.MockClusterClient
		renderedResources map[string]bool
		expectedRemoved   []string
		wantErr           bool
	}{
		"IdentifiesRemovedResources": {
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(xr).
					WithResourceTreeFromXRAndComposed(
						xr,
						[]*unstructured.Unstructured{resourceToKeep, resourceToRemove},
					).
					Build()
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
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(xr).
					WithResourceTreeFromXRAndComposed(
						xr,
						[]*unstructured.Unstructured{resourceToKeep, resourceToRemove},
					).
					Build()
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
			setupClient: func() *tu.MockClusterClient {
				return tu.NewMockClusterClient().
					WithResourcesExist(xr).
					WithFailedResourceTreeFetch("failed to get resource tree").
					Build()
			},
			renderedResources: map[string]bool{},
			expectedRemoved:   []string{},
			wantErr:           true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := tu.TestLogger(t)

			// Setup mock client
			mockClient := tt.setupClient()

			// Create a resource manager (not directly used in this test but required for constructor)
			resourceManager := NewResourceManager(mockClient, logger)

			// Create a diff calculator with default options
			calculator := NewDiffCalculator(
				mockClient,
				resourceManager,
				logger,
				DefaultDiffOptions(),
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
					if strings.Contains(key, name) && diff.DiffType == DiffTypeRemoved {
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
