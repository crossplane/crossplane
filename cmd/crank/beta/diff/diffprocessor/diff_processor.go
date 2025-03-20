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

	// Fetch XRDs from cluster
	xrds, err := p.client.GetXRDs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get XRDs")
	}

	// Convert XRDs to CRDs
	crds, err := internal.ConvertToCRDs(xrds)
	if err != nil {
		return errors.Wrap(err, "cannot convert XRDs to CRDs")
	}
	p.crds = crds

	// Get and cache environment configs
	environmentConfigs, err := p.client.GetEnvironmentConfigs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get environment configs")
	}

	// Update the EnvironmentConfigProvider with the fetched configs
	// Find the EnvironmentConfigProvider in our composite provider
	if compositeProvider, ok := p.extraResourceProvider.(*CompositeExtraResourceProvider); ok {
		for _, provider := range compositeProvider.providers {
			if envProvider, ok := provider.(*EnvironmentConfigProvider); ok {
				envProvider.configs = environmentConfigs
				break
			}
		}
	}

	p.config.Logger.Debug("Diff processor initialized",
		"crdCount", len(crds),
		"environmentConfigCount", len(environmentConfigs))
	return nil
}

// ProcessAll handles all resources stored in the processor. Each resource is a separate XR which will render a separate diff.
func (p *DefaultDiffProcessor) ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
	p.config.Logger.Debug("Processing resources", "count", len(resources))

	if len(resources) == 0 {
		p.config.Logger.Debug("No resources to process")
		return nil
	}

	var errs []error
	var processedCount, errorCount int

	for _, res := range resources {
		resourceID := fmt.Sprintf("%s/%s", res.GetKind(), res.GetName())

		if err := p.ProcessResource(stdout, ctx, res); err != nil {
			p.config.Logger.Debug("Failed to process resource", "resource", resourceID, "error", err)
			errs = append(errs, errors.Wrapf(err, "unable to process resource %s", resourceID))
			errorCount++
		} else {
			processedCount++
		}
	}

	if len(errs) > 0 {
		p.config.Logger.Debug("Completed processing with errors",
			"totalResources", len(resources),
			"successful", processedCount,
			"failed", errorCount)
		return errors.Join(errs...)
	}

	p.config.Logger.Debug("Successfully processed all resources", "count", processedCount)
	return nil
}

// ProcessResource handles one resource at a time with better separation of concerns
func (p *DefaultDiffProcessor) ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
	resourceID := fmt.Sprintf("%s/%s", res.GetKind(), res.GetName())
	p.config.Logger.Debug("Processing resource", "resource", resourceID)

	// Convert the unstructured resource to a composite unstructured for rendering
	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		p.config.Logger.Debug("Failed to convert resource", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	// Find the matching composition
	comp, err := p.client.FindMatchingComposition(res)
	if err != nil {
		p.config.Logger.Debug("No matching composition found", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot find matching composition")
	}

	p.config.Logger.Debug("Resource setup complete",
		"resource", resourceID,
		"composition", comp.GetName())

	// Get functions for rendering
	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		p.config.Logger.Debug("Failed to get functions", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot get functions from pipeline")
	}

	// Get all extra resources using our extraResourceProvider
	extraResources, err := p.extraResourceProvider.GetExtraResources(ctx, comp, res, []*unstructured.Unstructured{})
	if err != nil {
		p.config.Logger.Debug("Failed to get extra resources", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot get extra resources")
	}

	// Convert the extra resources to the format expected by render.Inputs
	extraResourcesForRender := make([]unstructured.Unstructured, 0, len(extraResources))
	for _, er := range extraResources {
		extraResourcesForRender = append(extraResourcesForRender, *er)
	}

	// Render the resources
	p.config.Logger.Debug("Rendering resources",
		"resource", resourceID,
		"extraResourceCount", len(extraResourcesForRender),
		"functionCount", len(fns))

	desired, err := p.config.RenderFunc(ctx, p.config.Logger, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    extraResourcesForRender,
	})
	if err != nil {
		p.config.Logger.Debug("Resource rendering failed", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot render resources")
	}

	// Merge the result of the render together with the input XR
	p.config.Logger.Debug("Merging and validating rendered resources",
		"resource", resourceID,
		"composedCount", len(desired.ComposedResources))

	xrUnstructured, err := mergeUnstructured(
		&unstructured.Unstructured{Object: desired.CompositeResource.UnstructuredContent()},
		&unstructured.Unstructured{Object: xr.UnstructuredContent()})
	if err != nil {
		p.config.Logger.Debug("Failed to merge XR", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot merge input XR with result of rendered XR")
	}

	// Validate the resources
	if err := p.ValidateResources(ctx, xrUnstructured, desired.ComposedResources); err != nil {
		p.config.Logger.Debug("Resource validation failed", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot validate resources")
	}

	// Calculate all diffs
	p.config.Logger.Debug("Calculating diffs", "resource", resourceID)
	diffs, err := p.CalculateDiffs(ctx, xr, desired)
	if err != nil {
		// We don't fail completely if some diffs couldn't be calculated
		p.config.Logger.Debug("Partial error calculating diffs", "resource", resourceID, "error", err)
	}

	// Render and print the diffs
	diffErr := p.RenderDiffs(stdout, diffs)
	if diffErr != nil {
		p.config.Logger.Debug("Failed to render diffs", "resource", resourceID, "error", diffErr)
		return diffErr
	}

	p.config.Logger.Debug("Resource processing complete",
		"resource", resourceID,
		"diffCount", len(diffs),
		"hasErrors", err != nil)

	return err
}

// ValidateResources validates the resources using schema validation
// Assumes that XRD-derived CRDs are already cached in p.crds
func (p *DefaultDiffProcessor) ValidateResources(ctx context.Context, xr *unstructured.Unstructured, composed []composed.Unstructured) error {
	p.config.Logger.Debug("Validating resources",
		"xr", fmt.Sprintf("%s/%s", xr.GetKind(), xr.GetName()),
		"composedCount", len(composed))

	// Collect all resources that need to be validated
	resources := make([]*unstructured.Unstructured, 0, len(composed)+1)

	// Add the XR to the validation list
	resources = append(resources, xr)

	// Add composed resources to validation list
	for i := range composed {
		resources = append(resources, &unstructured.Unstructured{Object: composed[i].UnstructuredContent()})
	}

	// Ensure we have all the required CRDs
	if len(p.crds) > 0 {
		p.config.Logger.Debug("Ensuring required CRDs for validation",
			"cachedCRDs", len(p.crds),
			"resourceCount", len(resources))
		p.ensureComposedResourceCRDs(ctx, resources)
	} else {
		// No CRDs cached, we need to fetch them
		p.config.Logger.Debug("Fetching CRDs for validation")
		xrds, err := p.client.GetXRDs(ctx)
		if err != nil {
			return errors.Wrap(err, "cannot get XRDs from cluster")
		}

		crds, err := internal.ConvertToCRDs(xrds)
		if err != nil {
			return errors.Wrap(err, "cannot convert XRDs to CRDs")
		}

		p.crds = crds
		p.ensureComposedResourceCRDs(ctx, resources)
	}

	// Create a logger writer to capture output
	loggerWriter := internal.NewLoggerWriter(p.config.Logger)

	// Validate using the CRD schemas
	// Use skipSuccessLogs=true to avoid cluttering the output with success messages
	p.config.Logger.Debug("Performing schema validation", "resourceCount", len(resources))
	if err := validate.SchemaValidation(resources, p.crds, true, loggerWriter); err != nil {
		return errors.Wrap(err, "schema validation failed")
	}

	p.config.Logger.Debug("Resources validated successfully")
	return nil
}

// ensureComposedResourceCRDs checks if we have all the CRDs needed for the composed resources
// and fetches any missing ones from the cluster
func (p *DefaultDiffProcessor) ensureComposedResourceCRDs(ctx context.Context, resources []*unstructured.Unstructured) {
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
		}
	}

	// Collect GVKs from resources that aren't already covered
	missingGVKs := make(map[schema.GroupVersionKind]bool)
	for _, res := range resources {
		gvk := res.GroupVersionKind()
		if !existingCRDs[gvk] {
			missingGVKs[gvk] = true
		}
	}

	// If we have all the CRDs already, we're done
	if len(missingGVKs) == 0 {
		p.config.Logger.Debug("All required CRDs are already cached")
		return
	}

	p.config.Logger.Debug("Fetching additional CRDs", "missingCount", len(missingGVKs))

	// Fetch missing CRDs
	for gvk := range missingGVKs {
		// Try to get the CRD by its conventional name pattern (plural.group)
		crdName := guessCRDName(gvk)

		p.config.Logger.Debug("Fetching CRD",
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
			p.config.Logger.Debug("CRD not found (continuing)",
				"gvk", gvk.String(),
				"crdName", crdName)
			continue
		}

		// Convert to CRD
		crd := &extv1.CustomResourceDefinition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(crdObj.Object, crd); err != nil {
			p.config.Logger.Debug("Error converting CRD (continuing)",
				"gvk", gvk.String(),
				"crdName", crdName)
			continue
		}

		// Add to our cache
		p.crds = append(p.crds, crd)
		p.config.Logger.Debug("Added CRD to cache", "crdName", crd.Name)
	}

	p.config.Logger.Debug("Finished ensuring CRDs", "totalCRDs", len(p.crds))
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
	xrName := xr.GetName()
	p.config.Logger.Debug("Calculating diffs",
		"xr", xrName,
		"composedCount", len(desired.ComposedResources))

	diffs := make(map[string]*ResourceDiff)
	var errs []error

	// Create a map to track resources that were rendered
	renderedResources := make(map[string]bool)

	// First, calculate diff for the XR itself
	xrDiff, err := p.CalculateDiff(ctx, nil, xr.GetUnstructured())
	if err != nil || xrDiff == nil {
		return nil, errors.Wrap(err, "cannot calculate diff for XR")
	} else if xrDiff.DiffType != DiffTypeEqual {
		key := fmt.Sprintf("%s/%s", xrDiff.ResourceKind, xrDiff.ResourceName)
		diffs[key] = xrDiff
	}

	// Then calculate diffs for all composed resources
	for _, d := range desired.ComposedResources {
		un := &unstructured.Unstructured{Object: d.UnstructuredContent()}

		// Generate a key to identify this resource
		apiVersion := un.GetAPIVersion()
		kind := un.GetKind()
		name := un.GetName()
		resourceID := fmt.Sprintf("%s/%s", kind, name)

		// Skip resources without names (likely a template issue)
		if name == "" {
			p.config.Logger.Debug("Skipping resource with empty name",
				"kind", kind,
				"apiVersion", apiVersion)
			continue
		}

		// Track this resource as rendered (for detecting removals)
		key := fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
		renderedResources[key] = true

		diff, err := p.CalculateDiff(ctx, xrDiff.Current, un)
		if err != nil {
			p.config.Logger.Debug("Error calculating diff for composed resource", "resource", resourceID, "error", err)
			errs = append(errs, errors.Wrapf(err, "cannot calculate diff for %s", resourceID))
			continue
		}

		if diff != nil && diff.DiffType != DiffTypeEqual {
			diffKey := fmt.Sprintf("%s/%s", diff.ResourceKind, diff.ResourceName)
			diffs[diffKey] = diff
		}
	}

	// Find resources that would be removed
	p.config.Logger.Debug("Finding resources to be removed", "xr", xrName)
	removedDiffs, err := p.CalculateRemovedResourceDiffs(ctx, xr, renderedResources)
	if err != nil {
		p.config.Logger.Debug("Error calculating removed resources (continuing)", "error", err)
	} else if len(removedDiffs) > 0 {
		// Add removed resources to the diffs map
		for key, diff := range removedDiffs {
			diffs[key] = diff
		}
	}

	// Log a summary
	p.config.Logger.Debug("Diff calculation complete",
		"totalDiffs", len(diffs),
		"errors", len(errs),
		"xr", xrName)

	if len(errs) > 0 {
		return diffs, errors.Join(errs...)
	}

	return diffs, nil
}

// CalculateRemovedResourceDiffs identifies resources that would be removed and calculates their diffs
func (p *DefaultDiffProcessor) CalculateRemovedResourceDiffs(ctx context.Context, xr *ucomposite.Unstructured, renderedResources map[string]bool) (map[string]*ResourceDiff, error) {
	xrName := xr.GetName()
	p.config.Logger.Debug("Checking for resources to be removed",
		"xr", xrName,
		"renderedResourceCount", len(renderedResources))

	removedDiffs := make(map[string]*ResourceDiff)

	// Try to find the XR and get its resource tree
	gvk := xr.GroupVersionKind()
	xrRes, err := p.client.GetResource(ctx, gvk, "", xrName)
	if err != nil {
		// Log the error but continue - we just won't detect removed resources
		p.config.Logger.Debug("Cannot find XR to check for removed resources (continuing)", "error", err)
		return removedDiffs, nil
	}

	// Try to get the resource tree
	resourceTree, err := p.client.GetResourceTree(ctx, xrRes)
	if err != nil {
		// Log the error but continue - we just won't detect removed resources
		p.config.Logger.Debug("Cannot get resource tree (continuing)", "error", err)
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
			key := fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)

			if !renderedResources[key] {
				// This resource exists but wasn't rendered - it will be removed
				p.config.Logger.Debug("Resource will be removed", "resource", resourceID)

				diffOpts := p.config.GetDiffOptions()
				diff, err := GenerateDiffWithOptions(&node.Unstructured, nil, p.config.Logger, diffOpts)
				if err != nil {
					p.config.Logger.Debug("Cannot calculate removal diff (continuing)",
						"resource", resourceID,
						"error", err)
					return
				}

				if diff != nil {
					diffKey := fmt.Sprintf("%s/%s", diff.ResourceKind, diff.ResourceName)
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

	p.config.Logger.Debug("Found resources to be removed", "count", len(removedDiffs))
	return removedDiffs, nil
}

// CalculateDiff calculates the diff for a single resource
func (p *DefaultDiffProcessor) CalculateDiff(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*ResourceDiff, error) {
	resourceID := fmt.Sprintf("%s/%s", desired.GetKind(), desired.GetName())
	p.config.Logger.Debug("Calculating diff", "resource", resourceID)

	// Fetch current object from cluster
	current, isNewObject, err := p.fetchCurrentObject(ctx, composite, desired)
	if err != nil {
		p.config.Logger.Debug("Failed to fetch current object", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot fetch current object")
	}

	// Log the resource status
	if isNewObject {
		p.config.Logger.Debug("Resource is new (not found in cluster)", "resource", resourceID)
	} else if current != nil {
		p.config.Logger.Debug("Found existing resource",
			"resource", resourceID,
			"resourceVersion", current.GetResourceVersion())
	}

	// Update owner references if needed
	p.updateOwnerRefs(composite, desired)

	// Determine what the resource would look like after application
	wouldBeResult := desired
	if current != nil {
		// Perform a dry-run apply to get the result after we'd apply
		p.config.Logger.Debug("Performing dry-run apply", "resource", resourceID)
		wouldBeResult, err = p.client.DryRunApply(ctx, desired)
		if err != nil {
			p.config.Logger.Debug("Dry-run apply failed", "resource", resourceID, "error", err)
			return nil, errors.Wrap(err, "cannot dry-run apply desired object")
		}
	}

	// Get diff options from the processor configuration
	diffOpts := p.config.GetDiffOptions()

	// Generate diff with the configured options
	diff, err := GenerateDiffWithOptions(current, wouldBeResult, p.config.Logger, diffOpts)
	if err != nil {
		p.config.Logger.Debug("Failed to generate diff", "resource", resourceID, "error", err)
		return nil, err
	}

	// Log the outcome
	if diff != nil {
		p.config.Logger.Debug("Diff generated",
			"resource", resourceID,
			"diffType", diff.DiffType,
			"hasChanges", diff.DiffType != DiffTypeEqual)
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
	resourceID := fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)

	p.config.Logger.Debug("Fetching current object state", "resource", resourceID)

	var current *unstructured.Unstructured
	var err error
	isNewObject := false

	// First, try direct lookup by GVK and name
	current, err = p.client.GetResource(ctx, gvk, namespace, name)
	if err == nil && current != nil {
		p.config.Logger.Debug("Found resource by direct lookup",
			"resource", resourceID,
			"resourceVersion", current.GetResourceVersion())

		// Check if this resource is already owned by a different composite
		if composite != nil {
			if labels := current.GetLabels(); labels != nil {
				if owner, exists := labels["crossplane.io/composite"]; exists && owner != composite.GetName() {
					// Log a warning if the resource is owned by a different composite
					p.config.Logger.Info(
						"Warning: Resource already belongs to another composite",
						"resource", resourceID,
						"currentOwner", owner,
						"newOwner", composite.GetName(),
					)
				}
			}
		}
		return current, false, nil
	}

	// Handle the resource not found case - this might be a genuinely new resource
	// or it might be a composed resource that we need to look up differently
	if apierrors.IsNotFound(err) {
		// If this is the XR itself (composite is nil), it's genuinely new
		if composite == nil {
			p.config.Logger.Debug("XR not found, creating new", "resource", resourceID)
			return nil, true, nil
		}

		// Check if we have annotations
		annotations := desired.GetAnnotations()
		if annotations == nil {
			p.config.Logger.Debug("Resource not found and has no annotations, creating new",
				"resource", resourceID)
			return nil, true, nil
		}

		// Look for composition resource name annotation
		var compResourceName string
		var hasCompResourceName bool

		// First check standard annotation
		if value, exists := annotations["crossplane.io/composition-resource-name"]; exists {
			compResourceName = value
			hasCompResourceName = true
		}

		// Then check function-specific variations if not found
		if !hasCompResourceName {
			for key, value := range annotations {
				if strings.HasSuffix(key, "/composition-resource-name") {
					compResourceName = value
					hasCompResourceName = true
					break
				}
			}
		}

		// If we don't have a composition resource name, it's a new resource
		if !hasCompResourceName {
			p.config.Logger.Debug("Resource not found and has no composition-resource-name, creating new",
				"resource", resourceID)
			return nil, true, nil
		}

		p.config.Logger.Debug("Resource not found by direct lookup, trying Crossplane labels",
			"resource", resourceID,
			"compositeName", composite.GetName(),
			"compositionResourceName", compResourceName)

		// Only proceed if we have necessary identifiers
		if composite.GetName() != "" {
			// Create a label selector to find resources managed by this composite
			labelSelector := metav1.LabelSelector{
				MatchLabels: map[string]string{
					"crossplane.io/composite": composite.GetName(),
				},
			}

			// Convert the GVK to GVR for the client call
			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: strings.ToLower(gvk.Kind) + "s", // Naive pluralization
			}

			// Handle special cases for some well-known types
			switch gvk.Kind {
			case "Ingress":
				gvr.Resource = "ingresses"
			case "Endpoints":
				gvr.Resource = "endpoints"
			case "ConfigMap":
				gvr.Resource = "configmaps"
				// Add other special cases as needed
			}

			// Look up resources with the composite label
			resources, err := p.client.GetResourcesByLabel(ctx, namespace, gvr, labelSelector)
			if err != nil {
				p.config.Logger.Debug("Error looking up resources by label",
					"resource", resourceID,
					"composite", composite.GetName(),
					"error", err)
			} else if len(resources) > 0 {
				p.config.Logger.Debug("Found potential matches by label",
					"resource", resourceID,
					"matchCount", len(resources))

				// Iterate through results to find one with matching composition-resource-name
				for _, res := range resources {
					resAnnotations := res.GetAnnotations()
					if resAnnotations == nil {
						continue
					}

					// Check both the standard annotation and function-specific variations
					resourceNameMatch := false
					for key, value := range resAnnotations {
						if (key == "crossplane.io/composition-resource-name" ||
							strings.HasSuffix(key, "/composition-resource-name")) &&
							value == compResourceName {
							resourceNameMatch = true
							break
						}
					}

					if resourceNameMatch {
						p.config.Logger.Debug("Found matching resource by composition-resource-name",
							"resource", fmt.Sprintf("%s/%s", res.GetKind(), res.GetName()),
							"annotation", compResourceName)
						return res, false, nil
					}
				}
			}
		}

		// We didn't find a matching resource using any strategy
		p.config.Logger.Debug("No matching resource found by label and annotation",
			"resource", resourceID,
			"compResourceName", compResourceName)
		isNewObject = true
		err = nil // Clear the error since this is an expected condition
	}

	return nil, isNewObject, err
}

// RenderDiffs formats and prints the diffs to the provided writer
func (p *DefaultDiffProcessor) RenderDiffs(stdout io.Writer, diffs map[string]*ResourceDiff) error {
	p.config.Logger.Debug("Rendering diffs to output",
		"diffCount", len(diffs),
		"useColors", p.config.Colorize,
		"compact", p.config.Compact)

	// Get diff options from the processor configuration
	diffOpts := p.config.GetDiffOptions()

	// Sort the keys to ensure a consistent output order
	keys := make([]string, 0, len(diffs))
	for key := range diffs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Track stats for summary logging
	addedCount := 0
	modifiedCount := 0
	removedCount := 0
	equalCount := 0
	outputCount := 0

	for _, key := range keys {
		diff := diffs[key]
		resourceID := fmt.Sprintf("%s/%s", diff.ResourceKind, diff.ResourceName)

		// Count by diff type for summary
		switch diff.DiffType {
		case DiffTypeAdded:
			addedCount++
		case DiffTypeRemoved:
			removedCount++
		case DiffTypeModified:
			modifiedCount++
		case DiffTypeEqual:
			equalCount++
			// Skip rendering equal resources
			continue
		}

		// Format the diff header based on the diff type
		var header string
		switch diff.DiffType {
		case DiffTypeAdded:
			header = fmt.Sprintf("+++ %s", resourceID)
		case DiffTypeRemoved:
			header = fmt.Sprintf("--- %s", resourceID)
		case DiffTypeModified:
			header = fmt.Sprintf("~~~ %s", resourceID)
		}

		// Format the diff content
		content := FormatDiff(diff.LineDiffs, diffOpts)

		if content != "" {
			_, err := fmt.Fprintf(stdout, "%s\n%s\n---\n", header, content)
			if err != nil {
				p.config.Logger.Debug("Error writing diff to output", "resource", resourceID, "error", err)
				return errors.Wrap(err, "failed to write diff to output")
			}
			outputCount++
		} else {
			p.config.Logger.Debug("Empty diff content, skipping output", "resource", resourceID)
		}
	}

	p.config.Logger.Debug("Diff rendering complete",
		"added", addedCount,
		"removed", removedCount,
		"modified", modifiedCount,
		"equal", equalCount,
		"output", outputCount)

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
