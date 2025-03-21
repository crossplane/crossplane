package diffprocessor

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DiffCalculator calculates differences between resources
type DiffCalculator interface {
	// CalculateDiff computes the diff for a single resource
	CalculateDiff(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*ResourceDiff, error)

	// CalculateDiffs computes all diffs for the rendered resources and identifies resources to be removed
	CalculateDiffs(ctx context.Context, xr *ucomposite.Unstructured, desired render.Outputs) (map[string]*ResourceDiff, error)

	// CalculateRemovedResourceDiffs identifies resources that would be removed and calculates their diffs
	CalculateRemovedResourceDiffs(ctx context.Context, xr *unstructured.Unstructured, renderedResources map[string]bool) (map[string]*ResourceDiff, error)
}

// DefaultDiffCalculator implements the DiffCalculator interface
type DefaultDiffCalculator struct {
	client          cc.ClusterClient
	resourceManager ResourceManager
	logger          logging.Logger
	diffOptions     DiffOptions
}

// SetDiffOptions updates the diff options used by the calculator
func (c *DefaultDiffCalculator) SetDiffOptions(options DiffOptions) {
	c.diffOptions = options
}

// NewDiffCalculator creates a new DefaultDiffCalculator
func NewDiffCalculator(client cc.ClusterClient, resourceManager ResourceManager, logger logging.Logger, diffOptions DiffOptions) DiffCalculator {
	return &DefaultDiffCalculator{
		client:          client,
		resourceManager: resourceManager,
		logger:          logger,
		diffOptions:     diffOptions,
	}
}

// CalculateDiff calculates the diff for a single resource
func (c *DefaultDiffCalculator) CalculateDiff(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*ResourceDiff, error) {
	// Get resource identification information
	name := desired.GetName()
	generateName := desired.GetGenerateName()

	// Create a resource ID for logging purposes
	var resourceID string
	if name != "" {
		resourceID = fmt.Sprintf("%s/%s", desired.GetKind(), name)
	} else if generateName != "" {
		resourceID = fmt.Sprintf("%s/%s*", desired.GetKind(), generateName)
	} else {
		resourceID = fmt.Sprintf("%s/<no-name>", desired.GetKind())
	}

	c.logger.Debug("Calculating diff", "resource", resourceID)

	// Fetch current object from cluster
	current, isNewObject, err := c.resourceManager.FetchCurrentObject(ctx, composite, desired)
	if err != nil {
		c.logger.Debug("Failed to fetch current object", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot fetch current object")
	}

	// Log the resource status
	if isNewObject {
		c.logger.Debug("Resource is new (not found in cluster)", "resource", resourceID)
	} else if current != nil {
		c.logger.Debug("Found existing resource",
			"resource", resourceID,
			"existingName", current.GetName(),
			"resourceVersion", current.GetResourceVersion())
	}

	// Update owner references if needed
	c.resourceManager.UpdateOwnerRefs(composite, desired)

	// If this is an existing resource but our desired object uses generateName,
	// set the name from the current resource for the dry-run apply
	if current != nil && name == "" && generateName != "" {
		currentName := current.GetName()
		c.logger.Debug("Using existing resource name for desired object with generateName",
			"resource", resourceID,
			"currentName", currentName)

		// Create a copy of the desired object and set the name from the current resource
		// This is needed for server-side apply which requires a name
		desiredCopy := desired.DeepCopy()
		desiredCopy.SetName(currentName)
		desired = desiredCopy
	}

	// Determine what the resource would look like after application
	wouldBeResult := desired
	if current != nil {
		// Perform a dry-run apply to get the result after we'd apply
		c.logger.Debug("Performing dry-run apply",
			"resource", resourceID,
			"name", desired.GetName())
		wouldBeResult, err = c.client.DryRunApply(ctx, desired)
		if err != nil {
			c.logger.Debug("Dry-run apply failed", "resource", resourceID, "error", err)
			return nil, errors.Wrap(err, "cannot dry-run apply desired object")
		}
	}

	// Generate diff with the configured options
	diff, err := GenerateDiffWithOptions(current, wouldBeResult, c.logger, c.diffOptions)
	if err != nil {
		c.logger.Debug("Failed to generate diff", "resource", resourceID, "error", err)
		return nil, err
	}

	// Log the outcome
	if diff != nil {
		c.logger.Debug("Diff generated",
			"resource", resourceID,
			"diffType", diff.DiffType,
			"hasChanges", diff.DiffType != DiffTypeEqual)
	}

	return diff, nil
}

// CalculateDiffs collects all diffs for the desired resources and identifies resources to be removed
func (c *DefaultDiffCalculator) CalculateDiffs(ctx context.Context, xr *ucomposite.Unstructured, desired render.Outputs) (map[string]*ResourceDiff, error) {
	xrName := xr.GetName()
	c.logger.Debug("Calculating diffs",
		"xr", xrName,
		"composedCount", len(desired.ComposedResources))

	diffs := make(map[string]*ResourceDiff)
	var errs []error

	// Create a map to track resources that were rendered
	renderedResources := make(map[string]bool)

	// First, calculate diff for the XR itself
	xrDiff, err := c.CalculateDiff(ctx, nil, xr.GetUnstructured())
	if err != nil || xrDiff == nil {
		return nil, errors.Wrap(err, "cannot calculate diff for XR")
	} else if xrDiff.DiffType != DiffTypeEqual {
		key := xrDiff.GetDiffKey()
		diffs[key] = xrDiff
	}

	// Then calculate diffs for all composed resources
	for _, d := range desired.ComposedResources {
		un := &unstructured.Unstructured{Object: d.UnstructuredContent()}

		// Generate a key to identify this resource
		apiVersion := un.GetAPIVersion()
		kind := un.GetKind()
		name := un.GetName()
		generateName := un.GetGenerateName()

		// For logging purposes - create a resource ID that might use generateName
		resourceID := fmt.Sprintf("%s/%s", kind, name)
		if name == "" && generateName != "" {
			resourceID = fmt.Sprintf("%s/%s*", kind, generateName)
		}

		// For resources using generateName, the name will be empty but we shouldn't skip them
		// Only skip if both name and generateName are empty (likely a template issue)
		if name == "" && generateName == "" {
			c.logger.Debug("Skipping resource with empty name and generateName",
				"kind", kind,
				"apiVersion", apiVersion)
			continue
		}

		diff, err := c.CalculateDiff(ctx, xrDiff.Current, un)
		if err != nil {
			c.logger.Debug("Error calculating diff for composed resource", "resource", resourceID, "error", err)
			errs = append(errs, errors.Wrapf(err, "cannot calculate diff for %s", resourceID))
			continue
		}

		diffKey := diff.GetDiffKey()
		if diff.DiffType != DiffTypeEqual {
			diffs[diffKey] = diff
		}

		renderedResources[diffKey] = true
	}

	if xrDiff.Current != nil {
		// it only makes sense to calculate removal of things if we have a current XR.
		c.logger.Debug("Finding resources to be removed", "xr", xrName)
		removedDiffs, err := c.CalculateRemovedResourceDiffs(ctx, xrDiff.Current, renderedResources)
		if err != nil {
			c.logger.Debug("Error calculating removed resources (continuing)", "error", err)
		} else if len(removedDiffs) > 0 {
			// Add removed resources to the diffs map
			for key, diff := range removedDiffs {
				diffs[key] = diff
			}
		}
	}

	// Log a summary
	c.logger.Debug("Diff calculation complete",
		"totalDiffs", len(diffs),
		"errors", len(errs),
		"xr", xrName)

	if len(errs) > 0 {
		return diffs, errors.Join(errs...)
	}

	return diffs, nil
}

// CalculateRemovedResourceDiffs identifies resources that would be removed and calculates their diffs
func (c *DefaultDiffCalculator) CalculateRemovedResourceDiffs(ctx context.Context, xr *unstructured.Unstructured, renderedResources map[string]bool) (map[string]*ResourceDiff, error) {
	xrName := xr.GetName()
	c.logger.Debug("Checking for resources to be removed",
		"xr", xrName,
		"renderedResourceCount", len(renderedResources))

	removedDiffs := make(map[string]*ResourceDiff)

	// Try to get the resource tree
	resourceTree, err := c.client.GetResourceTree(ctx, xr)
	if err != nil {
		// Log the error but continue - we just won't detect removed resources
		c.logger.Debug("Cannot get resource tree (continuing)", "error", err)
		return removedDiffs, nil
	}

	// Create a handler function to recursively traverse the tree and find composed resources
	var findRemovedResources func(node *resource.Resource)
	findRemovedResources = func(node *resource.Resource) {
		// Skip the root (XR) node
		if _, hasAnno := node.Unstructured.GetAnnotations()["crossplane.io/composition-resource-name"]; hasAnno {
			apiVersion := node.Unstructured.GetAPIVersion()
			kind := node.Unstructured.GetKind()
			name := node.Unstructured.GetName()
			resourceID := fmt.Sprintf("%s/%s", kind, name)

			// Use the same key format as in CalculateDiffs to check if this resource was rendered
			key := makeDiffKey(apiVersion, kind, name)

			if !renderedResources[key] {
				// This resource exists but wasn't rendered - it will be removed
				c.logger.Debug("Resource will be removed", "resource", resourceID)

				diff, err := GenerateDiffWithOptions(&node.Unstructured, nil, c.logger, c.diffOptions)
				if err != nil {
					c.logger.Debug("Cannot calculate removal diff (continuing)",
						"resource", resourceID,
						"error", err)
					return
				}

				if diff != nil {
					diffKey := diff.GetDiffKey()
					removedDiffs[diffKey] = diff
				}
			}
		}

		// Continue recursively traversing children
		for _, child := range node.Children {
			findRemovedResources(child)
		}
	}

	// Start the traversal from the root's children to skip the XR itself
	for _, child := range resourceTree.Children {
		findRemovedResources(child)
	}

	c.logger.Debug("Found resources to be removed", "count", len(removedDiffs))
	return removedDiffs, nil
}
