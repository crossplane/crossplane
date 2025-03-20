package diffprocessor

import (
	"context"
	"dario.cat/mergo"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource"
	"github.com/crossplane/crossplane/cmd/crank/beta/validate"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sort"
	"strings"
)

// RenderFunc defines the signature of a function that can render resources
type RenderFunc func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error)

// DiffProcessor interface for processing resources
type DiffProcessor interface {
	ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error
	ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error
	Initialize(ctx context.Context) error
}

// DefaultDiffProcessor handles the processing of resources for diffing.
type DefaultDiffProcessor struct {
	client                cc.ClusterClient
	config                ProcessorConfig
	crds                  []*extv1.CustomResourceDefinition
	extraResourceProvider ExtraResourceProvider
}

// NewDiffProcessor creates a new DefaultDiffProcessor with the provided options
func NewDiffProcessor(client cc.ClusterClient, options ...DiffProcessorOption) (DiffProcessor, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	// Create default configuration
	config := ProcessorConfig{
		Namespace:  "default",
		Colorize:   true,
		Compact:    false,
		Logger:     logging.NewNopLogger(),
		RenderFunc: render.Render,
	}

	// Apply all provided options
	for _, option := range options {
		option(&config)
	}

	// Validate required fields
	if config.RestConfig == nil {
		return nil, errors.New("REST config cannot be nil")
	}

	processor := &DefaultDiffProcessor{
		client: client,
		config: config,
		crds:   []*extv1.CustomResourceDefinition{},
	}

	// Create environment config provider with empty configs (will be populated in Initialize)
	envConfigProvider := NewEnvironmentConfigProvider([]*unstructured.Unstructured{}, config.Logger)

	// Create the composite provider with all our extra resource providers
	processor.extraResourceProvider = NewCompositeExtraResourceProvider(
		config.Logger,
		envConfigProvider,
		NewSelectorExtraResourceProvider(client, config.Logger),
		NewReferenceExtraResourceProvider(client, config.Logger),
		NewTemplatedExtraResourceProvider(client, config.RenderFunc, config.Logger),
	)

	return processor, nil
}

// Initialize loads required resources like CRDs and environment configs
func (p *DefaultDiffProcessor) Initialize(ctx context.Context) error {
	p.config.Logger.Debug("Initializing diff processor")

	p.config.Logger.Debug("Fetching XRDs from cluster")
	xrds, err := p.client.GetXRDs(ctx)
	if err != nil {
		p.config.Logger.Debug("Failed to get XRDs", "error", err)
		return errors.Wrap(err, "cannot get XRDs")
	}
	p.config.Logger.Debug("Retrieved XRDs", "count", len(xrds))

	// Use the helper function to convert XRDs to CRDs
	p.config.Logger.Debug("Converting XRDs to CRDs")
	crds, err := internal.ConvertToCRDs(xrds)
	if err != nil {
		p.config.Logger.Debug("Failed to convert XRDs to CRDs", "error", err)
		return errors.Wrap(err, "cannot convert XRDs to CRDs")
	}
	p.config.Logger.Debug("Converted XRDs to CRDs", "count", len(crds))

	p.crds = crds

	// Get and cache environment configs
	p.config.Logger.Debug("Fetching environment configs")
	environmentConfigs, err := p.client.GetEnvironmentConfigs(ctx)
	if err != nil {
		p.config.Logger.Debug("Failed to get environment configs", "error", err)
		return errors.Wrap(err, "cannot get environment configs")
	}
	p.config.Logger.Debug("Retrieved environment configs", "count", len(environmentConfigs))

	// Update the EnvironmentConfigProvider with the fetched configs
	// Find the EnvironmentConfigProvider in our composite provider
	if compositeProvider, ok := p.extraResourceProvider.(*CompositeExtraResourceProvider); ok {
		for _, provider := range compositeProvider.providers {
			if envProvider, ok := provider.(*EnvironmentConfigProvider); ok {
				p.config.Logger.Debug("Updating environment config provider with configs")
				envProvider.configs = environmentConfigs
				break
			}
		}
	}

	p.config.Logger.Debug("Diff processor initialization complete")
	return nil
}

// ProcessAll handles all resources stored in the processor. Each resource is a separate XR which will render a separate diff.
func (p *DefaultDiffProcessor) ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
	p.config.Logger.Debug("Processing all resources", "count", len(resources))

	if len(resources) == 0 {
		p.config.Logger.Debug("No resources to process, returning early")
		return nil
	}

	var errs []error
	for i, res := range resources {
		p.config.Logger.Debug("Processing resource",
			"index", i,
			"kind", res.GetKind(),
			"name", res.GetName())

		if err := p.ProcessResource(stdout, ctx, res); err != nil {
			p.config.Logger.Debug("Failed to process resource",
				"kind", res.GetKind(),
				"name", res.GetName(),
				"error", err)
			errs = append(errs, errors.Wrapf(err, "unable to process resource %s", res.GetName()))
		} else {
			p.config.Logger.Debug("Successfully processed resource",
				"kind", res.GetKind(),
				"name", res.GetName())
		}
	}

	if len(errs) > 0 {
		p.config.Logger.Debug("Completed processing all resources with errors",
			"totalResources", len(resources),
			"errorCount", len(errs))
		return errors.Join(errs...)
	}

	p.config.Logger.Debug("Successfully completed processing all resources",
		"totalResources", len(resources))
	return nil
}

// ProcessResource handles one resource at a time with better separation of concerns
func (p *DefaultDiffProcessor) ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
	p.config.Logger.Debug("Processing resource",
		"kind", res.GetKind(),
		"name", res.GetName())

	// Convert the unstructured resource to a composite unstructured for rendering
	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		p.config.Logger.Debug("Failed to convert XR to composite unstructured", "error", err)
		return errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	// Find the matching composition
	comp, err := p.client.FindMatchingComposition(res)
	if err != nil {
		p.config.Logger.Debug("Failed to find matching composition", "error", err)
		return errors.Wrap(err, "cannot find matching composition")
	}
	p.config.Logger.Debug("Found matching composition", "composition", comp.GetName())

	// Get functions for rendering
	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		p.config.Logger.Debug("Failed to get functions from pipeline", "error", err)
		return errors.Wrap(err, "cannot get functions from pipeline")
	}
	p.config.Logger.Debug("Got functions from pipeline", "count", len(fns))

	// Get all extra resources using our extraResourceProvider
	extraResources, err := p.extraResourceProvider.GetExtraResources(ctx, comp, res, []*unstructured.Unstructured{})
	if err != nil {
		p.config.Logger.Debug("Failed to get extra resources", "error", err)
		return errors.Wrap(err, "cannot get extra resources")
	}
	p.config.Logger.Debug("Got extra resources", "count", len(extraResources))

	// Convert the extra resources to the format expected by render.Inputs
	extraResourcesForRender := make([]unstructured.Unstructured, 0, len(extraResources))
	for _, er := range extraResources {
		extraResourcesForRender = append(extraResourcesForRender, *er)
	}

	// Render the resources
	p.config.Logger.Debug("Rendering resources")
	desired, err := p.config.RenderFunc(ctx, p.config.Logger, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    extraResourcesForRender,
	})
	if err != nil {
		p.config.Logger.Debug("Failed to render resources", "error", err)
		return errors.Wrap(err, "cannot render resources")
	}
	p.config.Logger.Debug("Successfully rendered resources",
		"composedResourceCount", len(desired.ComposedResources))

	// Merge the result of the render together with the input XR
	p.config.Logger.Debug("Merging rendered XR with input XR")
	xrUnstructured, err := mergeUnstructured(
		&unstructured.Unstructured{Object: desired.CompositeResource.UnstructuredContent()},
		&unstructured.Unstructured{Object: xr.UnstructuredContent()})
	if err != nil {
		p.config.Logger.Debug("Failed to merge input XR with rendered XR", "error", err)
		return errors.Wrap(err, "cannot merge input XR with result of rendered XR")
	}

	// Validate the resources
	p.config.Logger.Debug("Validating resources")
	if err := p.ValidateResources(ctx, xrUnstructured, desired.ComposedResources); err != nil {
		p.config.Logger.Debug("Resource validation failed", "error", err)
		return errors.Wrap(err, "cannot validate resources")
	}
	p.config.Logger.Debug("Resources validated successfully")

	// Calculate all diffs
	p.config.Logger.Debug("Calculating diffs")
	diffs, err := p.CalculateDiffs(ctx, xr, desired)
	if err != nil {
		// We don't fail completely if some diffs couldn't be calculated
		p.config.Logger.Debug("Error calculating some diffs", "error", err)
	}
	p.config.Logger.Debug("Diffs calculated", "diffCount", len(diffs))

	// Render and print the diffs
	p.config.Logger.Debug("Rendering diffs to output")
	err = p.RenderDiffs(stdout, diffs)
	if err != nil {
		p.config.Logger.Debug("Failed to render diffs", "error", err)
	} else {
		p.config.Logger.Debug("Successfully rendered diffs")
	}

	return err
}

// ValidateResources validates the resources using schema validation
// Assumes that XRD-derived CRDs are already cached in p.crds
func (p *DefaultDiffProcessor) ValidateResources(ctx context.Context, xr *unstructured.Unstructured, composed []composed.Unstructured) error {
	p.config.Logger.Debug("Starting resource validation",
		"composedCount", len(composed))

	// Collect all resources that need to be validated
	resources := make([]*unstructured.Unstructured, 0, len(composed)+1)

	// Add the XR to the validation list
	resources = append(resources, xr)
	p.config.Logger.Debug("Added XR to validation list",
		"kind", xr.GetKind(),
		"name", xr.GetName())

	// Add composed resources to validation list
	for i := range composed {
		composedUnstr := &unstructured.Unstructured{Object: composed[i].UnstructuredContent()}
		resources = append(resources, composedUnstr)
		p.config.Logger.Debug("Added composed resource to validation list",
			"kind", composedUnstr.GetKind(),
			"name", composedUnstr.GetName())
	}

	// Check if we have CRDs cached and fetch any that we're missing for composed resources
	if len(p.crds) > 0 {
		p.config.Logger.Debug("Using cached CRDs for validation", "count", len(p.crds))
		// We have some CRDs cached, but we need to ensure we have all required ones
		// for the composed resources
		p.ensureComposedResourceCRDs(ctx, resources)
	} else {
		// No CRDs cached, we need to fetch them
		p.config.Logger.Debug("No cached CRDs found, fetching from cluster")
		xrds, err := p.client.GetXRDs(ctx)
		if err != nil {
			p.config.Logger.Debug("Failed to get XRDs from cluster", "error", err)
			return errors.Wrap(err, "cannot get XRDs from cluster")
		}
		p.config.Logger.Debug("Retrieved XRDs from cluster", "count", len(xrds))

		crds, err := internal.ConvertToCRDs(xrds)
		if err != nil {
			p.config.Logger.Debug("Failed to convert XRDs to CRDs", "error", err)
			return errors.Wrap(err, "cannot convert XRDs to CRDs")
		}
		p.config.Logger.Debug("Converted XRDs to CRDs", "count", len(crds))

		p.crds = crds

		// Now also ensure we have CRDs for the composed resources
		p.ensureComposedResourceCRDs(ctx, resources)
	}

	// Create a logger writer to capture output
	loggerWriter := internal.NewLoggerWriter(p.config.Logger)

	// Validate using the CRD schemas
	// Use skipSuccessLogs=true to avoid cluttering the output with success messages
	p.config.Logger.Debug("Performing schema validation",
		"resourceCount", len(resources),
		"crdCount", len(p.crds))
	if err := validate.SchemaValidation(resources, p.crds, true, loggerWriter); err != nil {
		p.config.Logger.Debug("Schema validation failed", "error", err)
		return errors.Wrap(err, "schema validation failed")
	}
	p.config.Logger.Debug("Schema validation succeeded")

	return nil
}

// ensureComposedResourceCRDs checks if we have all the CRDs needed for the composed resources
// and fetches any missing ones from the cluster
func (p *DefaultDiffProcessor) ensureComposedResourceCRDs(ctx context.Context, resources []*unstructured.Unstructured) {
	p.config.Logger.Debug("Ensuring CRDs for composed resources",
		"resourceCount", len(resources),
		"existingCRDCount", len(p.crds))

	// Create a map of existing CRDs by GVK for quick lookup
	existingCRDs := make(map[schema.GroupVersionKind]bool)
	for _, crd := range p.crds {
		for _, version := range crd.Spec.Versions {
			gvk := schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: version.Name,
				Kind:    crd.Spec.Names.Kind,
			}
			existingCRDs[gvk] = true
			p.config.Logger.Debug("Existing CRD found", "gvk", gvk.String())
		}
	}

	// Collect GVKs from resources that aren't already covered
	missingGVKs := make(map[schema.GroupVersionKind]bool)
	for _, res := range resources {
		gvk := res.GroupVersionKind()
		if !existingCRDs[gvk] {
			missingGVKs[gvk] = true
			p.config.Logger.Debug("Missing CRD identified for resource",
				"gvk", gvk.String(),
				"name", res.GetName())
		}
	}

	// If we have all the CRDs already, we're done
	if len(missingGVKs) == 0 {
		p.config.Logger.Debug("All required CRDs are already cached")
		return
	}

	p.config.Logger.Debug("Fetching additional CRDs for composed resources",
		"missing", len(missingGVKs))

	// Fetch missing CRDs
	for gvk := range missingGVKs {
		// Try to get the CRD by its conventional name pattern (plural.group)
		// This is a naive approach to pluralization, might need improvement
		// for irregular plurals
		crdName := guessCRDName(gvk)

		p.config.Logger.Debug("Fetching CRD for resource",
			"gvk", gvk.String(),
			"crdName", crdName)

		crdObj, err := p.client.GetResource(
			ctx,
			schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			},
			"",
			crdName,
		)

		if err != nil {
			// Log but don't fail - we might not need all CRDs or it could
			// be a built-in resource type without a CRD
			p.config.Logger.Debug("Could not find CRD for resource",
				"name", crdName,
				"gvk", gvk.String(),
				"error", err)
			continue
		}

		// Convert to CRD
		crd := &extv1.CustomResourceDefinition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(crdObj.Object, crd); err != nil {
			p.config.Logger.Debug("Error converting CRD", "error", err)
			continue
		}

		// Add to our cache
		p.crds = append(p.crds, crd)
		p.config.Logger.Debug("Added CRD to cache", "name", crd.Name, "gvk", gvk.String())
	}

	p.config.Logger.Debug("Finished ensuring CRDs for composed resources",
		"totalCRDsNow", len(p.crds))
}

// guessCRDName attempts to create the CRD name for a given GVK
// using the conventional pattern (plural.group)
func guessCRDName(gvk schema.GroupVersionKind) string {
	// This is a naïve pluralization - in a real implementation
	// we might want to handle irregular plurals or use a library
	plural := strings.ToLower(gvk.Kind) + "s"

	// Handle some common special cases
	switch strings.ToLower(gvk.Kind) {
	case "policy":
		plural = "policies"
	case "gateway":
		plural = "gateways"
	case "proxy":
		plural = "proxies"
	case "index":
		plural = "indices"
	case "matrix":
		plural = "matrices"
	}

	return fmt.Sprintf("%s.%s", plural, gvk.Group)
}

// resourceKey generates a unique key for a resource based on GVK and name
func resourceKey(res *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s",
		res.GetAPIVersion(),
		res.GetKind(),
		res.GetName())
}

// findResourcesToBeRemoved identifies resources that exist in the current state but are not in the processed list
func (p *DefaultDiffProcessor) findResourcesToBeRemoved(ctx context.Context, composite string, processedResources map[string]bool) ([]*unstructured.Unstructured, error) {

	// Find the XR
	xrRes, err := p.client.GetResource(ctx, schema.GroupVersionKind{
		Group:   "example.org", // This needs to be determined dynamically based on the XR
		Version: "v1alpha1",    // This needs to be determined dynamically
		Kind:    "XRKind",      // This needs to be determined dynamically
	}, "", composite)
	if err != nil {
		return nil, errors.Wrap(err, "cannot find composite resource")
	}

	// Get the resource tree
	resourceTree, err := p.client.GetResourceTree(ctx, xrRes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get resource tree")
	}

	// Find resources that weren't processed (meaning they would be removed)
	var toBeRemoved []*unstructured.Unstructured

	// Function to recursively traverse the tree and find composed resources
	var findComposedResources func(node *resource.Resource)
	findComposedResources = func(node *resource.Resource) {
		// Skip the root (XR) node
		if node.Unstructured.GetAnnotations()["crossplane.io/composition-resource-name"] != "" {
			key := resourceKey(&node.Unstructured)
			if !processedResources[key] {
				// This resource exists but wasn't in our desired resources - it will be removed
				toBeRemoved = append(toBeRemoved, &node.Unstructured)
			}
		}

		for _, child := range node.Children {
			findComposedResources(child)
		}
	}

	// Start the traversal from the root's children to skip the XR itself
	for _, child := range resourceTree.Children {
		findComposedResources(child)
	}

	return toBeRemoved, nil
}

// CalculateDiffs collects all diffs for the desired resources and identifies resources to be removed
func (p *DefaultDiffProcessor) CalculateDiffs(ctx context.Context, xr *ucomposite.Unstructured, desired render.Outputs) (map[string]*ResourceDiff, error) {
	p.config.Logger.Debug("Starting diff calculation",
		"xrName", xr.GetName(),
		"composedCount", len(desired.ComposedResources))

	diffs := make(map[string]*ResourceDiff)
	var errs []error

	// Create a map to track resources that were rendered
	renderedResources := make(map[string]bool)

	// First, calculate diff for the XR itself
	p.config.Logger.Debug("Calculating diff for XR itself")
	xrDiff, err := p.CalculateDiff(ctx, nil, xr.GetUnstructured())
	if err != nil || xrDiff == nil {
		p.config.Logger.Debug("Failed to calculate diff for XR", "error", err)
		return nil, errors.Wrap(err, "cannot calculate diff for XR")
	} else if xrDiff.DiffType != DiffTypeEqual {
		key := fmt.Sprintf("%s/%s", xrDiff.ResourceKind, xrDiff.ResourceName)
		diffs[key] = xrDiff
		p.config.Logger.Debug("Added XR diff", "key", key, "diffType", xrDiff.DiffType)
	} else {
		p.config.Logger.Debug("XR is unchanged, no diff added")
	}

	// Then calculate diffs for all composed resources
	p.config.Logger.Debug("Calculating diffs for composed resources",
		"count", len(desired.ComposedResources))

	for i, d := range desired.ComposedResources {
		un := &unstructured.Unstructured{Object: d.UnstructuredContent()}

		// Generate a key to identify this resource
		apiVersion := un.GetAPIVersion()
		kind := un.GetKind()
		name := un.GetName()

		p.config.Logger.Debug("Processing composed resource",
			"index", i,
			"kind", kind,
			"name", name)

		// Track this resource as rendered (for detecting removals)
		key := fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
		renderedResources[key] = true

		diff, err := p.CalculateDiff(ctx, xrDiff.Current, un)
		if err != nil {
			p.config.Logger.Debug("Error calculating diff for composed resource",
				"key", key,
				"error", err)
			errs = append(errs, errors.Wrapf(err, "cannot calculate diff for %s", key))
			continue
		}

		if diff != nil {
			diffKey := fmt.Sprintf("%s/%s", diff.ResourceKind, diff.ResourceName)
			diffs[diffKey] = diff
			p.config.Logger.Debug("Added composed resource diff",
				"key", diffKey,
				"diffType", diff.DiffType)
		} else {
			p.config.Logger.Debug("No diff for composed resource", "key", key)
		}
	}

	// Find resources that would be removed - but don't block the diff process if this fails
	p.config.Logger.Debug("Finding resources that would be removed")
	removedDiffs, err := p.CalculateRemovedResourceDiffs(ctx, xr, renderedResources)
	if err != nil {
		p.config.Logger.Debug("Warning: Error calculating removed resources", "error", err)
	} else {
		p.config.Logger.Debug("Found resources to be removed", "count", len(removedDiffs))
	}

	// Add removed resources to the diffs map
	for key, diff := range removedDiffs {
		diffs[key] = diff
		p.config.Logger.Debug("Added removed resource diff", "key", key)
	}

	p.config.Logger.Debug("Completed diff calculation",
		"totalDiffs", len(diffs),
		"errorCount", len(errs))

	if len(errs) > 0 {
		return diffs, errors.Join(errs...)
	}

	return diffs, nil
}

// CalculateRemovedResourceDiffs identifies resources that would be removed and calculates their diffs
func (p *DefaultDiffProcessor) CalculateRemovedResourceDiffs(ctx context.Context, xr *ucomposite.Unstructured, renderedResources map[string]bool) (map[string]*ResourceDiff, error) {
	p.config.Logger.Debug("Calculating removed resource diffs",
		"xrName", xr.GetName(),
		"renderedResourceCount", len(renderedResources))

	removedDiffs := make(map[string]*ResourceDiff)

	// Try to find the XR and get its resource tree, but don't fail the entire diff if we can't
	gvk := xr.GroupVersionKind()
	p.config.Logger.Debug("Looking up XR in cluster",
		"gvk", gvk.String(),
		"name", xr.GetName())

	xrRes, err := p.client.GetResource(ctx, gvk, "", xr.GetName())
	if err != nil {
		// Log the error but continue - we just won't detect removed resources
		p.config.Logger.Debug("Cannot find composite resource to check for removed resources", "error", err)
		return removedDiffs, nil
	}
	p.config.Logger.Debug("Found XR in cluster", "uid", xrRes.GetUID())

	// Try to get the resource tree
	p.config.Logger.Debug("Getting resource tree for XR")
	resourceTree, err := p.client.GetResourceTree(ctx, xrRes)
	if err != nil {
		// Log the error but continue - we just won't detect removed resources
		p.config.Logger.Debug("Cannot get resource tree to check for removed resources", "error", err)
		return removedDiffs, nil
	}
	p.config.Logger.Debug("Got resource tree",
		"childCount", len(resourceTree.Children))

	// Function to recursively traverse the tree and find composed resources
	var findRemovedResources func(node *resource.Resource)
	findRemovedResources = func(node *resource.Resource) {
		// Skip the root (XR) node
		if _, hasAnno := node.Unstructured.GetAnnotations()["crossplane.io/composition-resource-name"]; hasAnno {
			apiVersion := node.Unstructured.GetAPIVersion()
			kind := node.Unstructured.GetKind()
			name := node.Unstructured.GetName()

			// Use the same key format as in CalculateDiffs to check if this resource was rendered
			key := fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
			p.config.Logger.Debug("Checking if resource will be removed",
				"resource", key,
				"rendered", renderedResources[key])

			if !renderedResources[key] {
				// This resource exists but wasn't rendered - it will be removed
				p.config.Logger.Debug("Resource will be removed - generating diff", "resource", key)

				diffOpts := p.config.GetDiffOptions()
				diff, err := GenerateDiffWithOptions(&node.Unstructured, nil, p.config.Logger, diffOpts)
				if err != nil {
					p.config.Logger.Debug("Cannot calculate removal diff",
						"resource", key,
						"error", err)
					return
				}

				if diff != nil {
					diffKey := fmt.Sprintf("%s/%s", diff.ResourceKind, diff.ResourceName)
					removedDiffs[diffKey] = diff
					p.config.Logger.Debug("Found resource to be removed",
						"resource", key,
						"diffKey", diffKey)
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

	p.config.Logger.Debug("Finished calculating removed resource diffs",
		"removedCount", len(removedDiffs))

	return removedDiffs, nil
}

// CalculateDiff calculates the diff for a single resource
func (p *DefaultDiffProcessor) CalculateDiff(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*ResourceDiff, error) {
	p.config.Logger.Debug("Calculating diff for resource",
		"desiredKind", desired.GetKind(),
		"desiredName", desired.GetName())

	// Fetch current object from cluster
	current, isNewObject, err := p.fetchCurrentObject(ctx, composite, desired)
	if err != nil {
		p.config.Logger.Debug("Failed to fetch current object",
			"error", err,
			"kind", desired.GetKind(),
			"name", desired.GetName())
		return nil, errors.Wrap(err, "cannot fetch current object")
	}

	if isNewObject {
		p.config.Logger.Debug("Resource is new (not found in cluster)",
			"kind", desired.GetKind(),
			"name", desired.GetName())
	} else if current != nil {
		p.config.Logger.Debug("Found existing resource in cluster",
			"kind", current.GetKind(),
			"name", current.GetName(),
			"resourceVersion", current.GetResourceVersion())
	}

	p.updateOwnerRefs(composite, desired)

	wouldBeResult := desired
	if current != nil {
		// Perform a dry-run apply to get the result after we'd apply
		p.config.Logger.Debug("Performing dry-run apply",
			"kind", desired.GetKind(),
			"name", desired.GetName())
		wouldBeResult, err = p.client.DryRunApply(ctx, desired)
		if err != nil {
			p.config.Logger.Debug("Dry-run apply failed", "error", err)
			return nil, errors.Wrap(err, "cannot dry-run apply desired object")
		}
		p.config.Logger.Debug("Dry-run apply successful")
	}

	// Get diff options from the processor configuration
	diffOpts := p.config.GetDiffOptions()

	// Generate diff with the configured options
	p.config.Logger.Debug("Generating diff with options",
		"useColors", diffOpts.UseColors,
		"compact", diffOpts.Compact)
	diff, err := GenerateDiffWithOptions(current, wouldBeResult, p.config.Logger, diffOpts)
	if err != nil {
		p.config.Logger.Debug("Failed to generate diff", "error", err)
		return nil, err
	}

	if diff != nil {
		p.config.Logger.Debug("Diff generated successfully",
			"diffType", diff.DiffType,
			"resourceKind", diff.ResourceKind,
			"resourceName", diff.ResourceName)
	} else {
		p.config.Logger.Debug("No diff generated (null result)")
	}

	return diff, nil
}

// updateOwnerRefs makes sure all OwnerReferences have a valid UID
func (p *DefaultDiffProcessor) updateOwnerRefs(parent *unstructured.Unstructured, child *unstructured.Unstructured) {
	// if there's no parent, we are the parent.
	if parent == nil {
		p.config.Logger.Debug("No parent provided for owner references update")
		return
	}

	uid := parent.GetUID()
	p.config.Logger.Debug("Updating owner references",
		"parentKind", parent.GetKind(),
		"parentName", parent.GetName(),
		"parentUID", uid,
		"childKind", child.GetKind(),
		"childName", child.GetName())

	// Get the current owner references
	refs := child.GetOwnerReferences()
	p.config.Logger.Debug("Current owner references", "count", len(refs))

	// Create new slice to hold the updated references
	updatedRefs := make([]metav1.OwnerReference, 0, len(refs))

	// Set a valid UID for each reference
	for _, ref := range refs {
		originalUID := ref.UID

		// if there is an owner ref on the dependent that we are pretty sure comes from us,
		// point the UID to the parent.
		if ref.Name == parent.GetName() &&
			ref.APIVersion == parent.GetAPIVersion() &&
			ref.Kind == parent.GetKind() &&
			ref.UID == "" {
			ref.UID = uid
			p.config.Logger.Debug("Updated matching owner reference with parent UID",
				"refName", ref.Name,
				"oldUID", originalUID,
				"newUID", ref.UID)
		}

		// if we have a non-matching owner ref don't use the parent UID.
		if ref.UID == "" {
			ref.UID = uuid.NewUUID()
			p.config.Logger.Debug("Generated new random UID for owner reference",
				"refName", ref.Name,
				"oldUID", originalUID,
				"newUID", ref.UID)
		}

		updatedRefs = append(updatedRefs, ref)
	}

	// Update the object with the modified owner references
	child.SetOwnerReferences(updatedRefs)
	p.config.Logger.Debug("Updated owner references",
		"newCount", len(updatedRefs))
}

// fetchCurrentObject retrieves the current state of the object from the cluster
// It returns the current object, a boolean indicating if it's a new object, and any error
func (p *DefaultDiffProcessor) fetchCurrentObject(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	// Get the GroupVersionKind and name/namespace for lookup
	gvk := desired.GroupVersionKind()
	name := desired.GetName()
	namespace := desired.GetNamespace()

	p.config.Logger.Debug("Fetching current object from cluster",
		"gvk", gvk.String(),
		"namespace", namespace,
		"name", name)

	var current *unstructured.Unstructured
	var err error
	isNewObject := false

	// For all resources, use direct lookup by GVK and name
	current, err = p.client.GetResource(ctx, gvk, namespace, name)
	if apierrors.IsNotFound(err) {
		// If the resource is not found, it's a new object
		p.config.Logger.Debug("Resource not found in cluster (new object)",
			"gvk", gvk.String(),
			"name", name)
		isNewObject = true
		err = nil // Clear the error since this is an expected condition
	} else if err != nil {
		// For any other error, return it
		p.config.Logger.Debug("Error getting resource from cluster",
			"gvk", gvk.String(),
			"name", name,
			"error", err)
		return nil, false, errors.Wrap(err, "cannot get current object")
	} else {
		p.config.Logger.Debug("Found existing resource in cluster",
			"gvk", gvk.String(),
			"name", name,
			"resourceVersion", current.GetResourceVersion())
	}

	// If the object exists and we have a composite parent to check against...
	if current != nil && composite != nil {
		// Check if this resource is already owned by a different composite
		if labels := current.GetLabels(); labels != nil {
			if owner, exists := labels["crossplane.io/composite"]; exists && owner != composite.GetName() {
				// Log a warning if the resource is owned by a different composite
				p.config.Logger.Info(
					"Warning: Resource already belongs to another composite",
					"resource", fmt.Sprintf("%s/%s", gvk.Kind, name),
					"currentOwner", owner,
					"newOwner", composite.GetName(),
				)
			}
		}
	}

	return current, isNewObject, nil
}

// RenderDiffs formats and prints the diffs to the provided writer
func (p *DefaultDiffProcessor) RenderDiffs(stdout io.Writer, diffs map[string]*ResourceDiff) error {
	p.config.Logger.Debug("Rendering diffs to output", "diffCount", len(diffs))

	// Get diff options from the processor configuration
	diffOpts := p.config.GetDiffOptions()
	p.config.Logger.Debug("Using diff options",
		"useColors", diffOpts.UseColors,
		"compact", diffOpts.Compact)

	// Sort the keys to ensure a consistent output order
	keys := make([]string, 0, len(diffs))
	for key := range diffs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	p.config.Logger.Debug("Sorted diff keys for consistent output", "keyCount", len(keys))

	outputCount := 0
	for _, key := range keys {
		diff := diffs[key]

		// Skip rendering equal resources
		if diff.DiffType == DiffTypeEqual {
			p.config.Logger.Debug("Skipping equal resource (no changes)",
				"resourceKind", diff.ResourceKind,
				"resourceName", diff.ResourceName)
			continue
		}

		// Format the diff header based on the diff type
		var header string
		switch diff.DiffType {
		case DiffTypeAdded:
			header = fmt.Sprintf("+++ %s/%s", diff.ResourceKind, diff.ResourceName)
			p.config.Logger.Debug("Rendering added resource",
				"resourceKind", diff.ResourceKind,
				"resourceName", diff.ResourceName)
		case DiffTypeRemoved:
			header = fmt.Sprintf("--- %s/%s", diff.ResourceKind, diff.ResourceName)
			p.config.Logger.Debug("Rendering removed resource",
				"resourceKind", diff.ResourceKind,
				"resourceName", diff.ResourceName)
		case DiffTypeModified:
			header = fmt.Sprintf("~~~ %s/%s", diff.ResourceKind, diff.ResourceName)
			p.config.Logger.Debug("Rendering modified resource",
				"resourceKind", diff.ResourceKind,
				"resourceName", diff.ResourceName)
		case DiffTypeEqual: // technically a nop, but for completeness
			header = fmt.Sprintf("=== %s/%s", diff.ResourceKind, diff.ResourceName)
			p.config.Logger.Debug("Rendering equal resource",
				"resourceKind", diff.ResourceKind,
				"resourceName", diff.ResourceName)
		}

		// Format the diff content
		content := FormatDiff(diff.LineDiffs, diffOpts)

		if content != "" {
			_, err := fmt.Fprintf(stdout, "%s\n%s\n---\n", header, content)
			if err != nil {
				p.config.Logger.Debug("Error writing diff to output",
					"error", err,
					"resourceKind", diff.ResourceKind,
					"resourceName", diff.ResourceName)
				return errors.Wrap(err, "failed to write diff to output")
			}
			outputCount++
		} else {
			p.config.Logger.Debug("Empty diff content, skipping output",
				"resourceKind", diff.ResourceKind,
				"resourceName", diff.ResourceName)
		}
	}

	p.config.Logger.Debug("Finished rendering diffs",
		"outputCount", outputCount,
		"totalDiffs", len(diffs))

	return nil
}

// mergeUnstructured merges two unstructured objects
func mergeUnstructured(dest *unstructured.Unstructured, src *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Start with a deep copy of the rendered resource
	result := dest.DeepCopy()
	if err := mergo.Merge(&result.Object, src.Object, mergo.WithOverride); err != nil {
		return nil, errors.Wrap(err, "cannot merge unstructured objects")
	}

	return result, nil
}
