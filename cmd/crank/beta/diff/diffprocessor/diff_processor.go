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
	Initialize(writer io.Writer, ctx context.Context) error
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
	envConfigProvider := NewEnvironmentConfigProvider([]*unstructured.Unstructured{})

	// Create the composite provider with all our extra resource providers
	processor.extraResourceProvider = NewCompositeExtraResourceProvider(
		envConfigProvider,
		NewSelectorExtraResourceProvider(client),
		NewReferenceExtraResourceProvider(client),
		NewTemplatedExtraResourceProvider(client, config.RenderFunc, config.Logger),
	)

	return processor, nil
}

// Initialize loads required resources like CRDs and environment configs
func (p *DefaultDiffProcessor) Initialize(writer io.Writer, ctx context.Context) error {
	xrds, err := p.client.GetXRDs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get XRDs")
	}

	// Use the helper function to convert XRDs to CRDs
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

	return nil
}

// ProcessAll handles all resources stored in the processor.  each resource is a separate XR which will render a separate diff.
func (p *DefaultDiffProcessor) ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error {
	var errs []error
	for _, res := range resources {
		if err := p.ProcessResource(stdout, ctx, res); err != nil {
			errs = append(errs, errors.Wrapf(err, "unable to process resource %s", res.GetName()))
		}
	}

	return errors.Join(errs...)
}

// ProcessResource handles one resource at a time with better separation of concerns
func (p *DefaultDiffProcessor) ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error {
	// Convert the unstructured resource to a composite unstructured for rendering
	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		return errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	// Find the matching composition
	comp, err := p.client.FindMatchingComposition(res)
	if err != nil {
		return errors.Wrap(err, "cannot find matching composition")
	}

	// Get functions for rendering
	fns, err := p.client.GetFunctionsFromPipeline(comp)
	if err != nil {
		return errors.Wrap(err, "cannot get functions from pipeline")
	}

	// Get all extra resources using our extraResourceProvider
	extraResources, err := p.extraResourceProvider.GetExtraResources(ctx, comp, res, []*unstructured.Unstructured{})
	if err != nil {
		return errors.Wrap(err, "cannot get extra resources")
	}

	// Convert the extra resources to the format expected by render.Inputs
	extraResourcesForRender := make([]unstructured.Unstructured, 0, len(extraResources))
	for _, er := range extraResources {
		extraResourcesForRender = append(extraResourcesForRender, *er)
	}

	// Render the resources
	desired, err := p.config.RenderFunc(ctx, p.config.Logger, render.Inputs{
		CompositeResource: xr,
		Composition:       comp,
		Functions:         fns,
		ExtraResources:    extraResourcesForRender,
	})
	if err != nil {
		return errors.Wrap(err, "cannot render resources")
	}

	// Validate the resources
	if err := p.ValidateResources(stdout, &unstructured.Unstructured{Object: desired.CompositeResource.UnstructuredContent()}, desired.ComposedResources); err != nil {
		return errors.Wrap(err, "cannot validate resources")
	}

	// Calculate all diffs
	diffs, err := p.CalculateDiffs(ctx, xr, desired)
	if err != nil {
		// We don't fail completely if some diffs couldn't be calculated
		p.config.Logger.Debug("Error calculating some diffs", "error", err)
	}

	// Render and print the diffs
	return p.RenderDiffs(stdout, diffs)
}

// ValidateResources validates the resources using schema validation
func (p *DefaultDiffProcessor) ValidateResources(writer io.Writer, xr *unstructured.Unstructured, composed []composed.Unstructured) error {
	// Make sure we have CRDs before validation
	if len(p.crds) == 0 {
		return errors.New("no CRDs available for validation")
	}

	// Convert XR and composed resources to unstructured
	resources := make([]*unstructured.Unstructured, 0, len(composed)+1)

	// Add the XR to the validation list; we've already taken care of merging it together with desired
	resources = append(resources, xr)

	// Add composed resources to validation list
	for i := range composed {
		// Use the correct index (i) for accessing composed resources
		composedUnstr := &unstructured.Unstructured{Object: composed[i].UnstructuredContent()}
		resources = append(resources, composedUnstr)
	}

	loggerWriter := internal.NewLoggerWriter(p.config.Logger)

	// Validate using the converted CRD schema
	// Do not actually write to stdout (as it can interfere with diffs) -- just use the logger
	if err := validate.SchemaValidation(resources, p.crds, true, loggerWriter); err != nil {
		return errors.Wrap(err, "schema validation failed")
	}

	return nil
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
	diffs := make(map[string]*ResourceDiff)
	var errs []error

	// Create a map to track resources that were rendered
	renderedResources := make(map[string]bool)

	// First, calculate diff for the XR itself
	xrUnstructured, err := mergeUnstructured(&unstructured.Unstructured{Object: desired.CompositeResource.UnstructuredContent()}, &unstructured.Unstructured{Object: xr.UnstructuredContent()})
	if err != nil {
		return nil, errors.Wrap(err, "cannot merge input XR with result of rendered XR")
	}

	xrDiff, err := p.CalculateDiff(ctx, "", xrUnstructured)
	if err != nil {
		errs = append(errs, errors.Wrap(err, "cannot calculate diff for XR"))
	} else if xrDiff != nil {
		key := fmt.Sprintf("%s/%s", xrDiff.ResourceKind, xrDiff.ResourceName)
		diffs[key] = xrDiff
		p.config.Logger.Debug("Added XR diff", "key", key)
	}

	// Then calculate diffs for all composed resources
	for _, d := range desired.ComposedResources {
		un := &unstructured.Unstructured{Object: d.UnstructuredContent()}

		// Generate a key to identify this resource
		apiVersion := un.GetAPIVersion()
		kind := un.GetKind()
		name := un.GetName()

		// Track this resource as rendered (for detecting removals)
		key := fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
		renderedResources[key] = true

		diff, err := p.CalculateDiff(ctx, xr.GetName(), un)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "cannot calculate diff for %s", key))
			continue
		}

		if diff != nil {
			diffKey := fmt.Sprintf("%s/%s", diff.ResourceKind, diff.ResourceName)
			diffs[diffKey] = diff
			p.config.Logger.Debug("Added composed resource diff", "key", diffKey)
		}
	}

	// Find resources that would be removed - but don't block the diff process if this fails
	removedDiffs, err := p.CalculateRemovedResourceDiffs(ctx, xr, renderedResources)
	if err != nil {
		p.config.Logger.Debug("Warning: Error calculating removed resources", "error", err)
	}

	// Add removed resources to the diffs map
	for key, diff := range removedDiffs {
		diffs[key] = diff
		p.config.Logger.Debug("Added removed resource diff", "key", key)
	}

	if len(errs) > 0 {
		return diffs, errors.Join(errs...)
	}

	return diffs, nil
}

// CalculateRemovedResourceDiffs identifies resources that would be removed and calculates their diffs
func (p *DefaultDiffProcessor) CalculateRemovedResourceDiffs(ctx context.Context, xr *ucomposite.Unstructured, renderedResources map[string]bool) (map[string]*ResourceDiff, error) {
	removedDiffs := make(map[string]*ResourceDiff)

	// Try to find the XR and get its resource tree, but don't fail the entire diff if we can't
	gvk := xr.GroupVersionKind()
	xrRes, err := p.client.GetResource(ctx, gvk, "", xr.GetName())
	if err != nil {
		// Log the error but continue - we just won't detect removed resources
		p.config.Logger.Debug("Cannot find composite resource to check for removed resources", "error", err)
		return removedDiffs, nil
	}

	// Try to get the resource tree
	resourceTree, err := p.client.GetResourceTree(ctx, xrRes)
	if err != nil {
		// Log the error but continue - we just won't detect removed resources
		p.config.Logger.Debug("Cannot get resource tree to check for removed resources", "error", err)
		return removedDiffs, nil
	}

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
			p.config.Logger.Debug("Checking if resource will be removed", "resource", key, "rendered", renderedResources[key])

			if !renderedResources[key] {
				// This resource exists but wasn't rendered - it will be removed
				diffOpts := p.config.GetDiffOptions()
				diff, err := GenerateDiffWithOptions(&node.Unstructured, nil, diffOpts)
				if err != nil {
					p.config.Logger.Debug("Cannot calculate removal diff", "resource", key, "error", err)
					return
				}

				if diff != nil {
					diffKey := fmt.Sprintf("%s/%s", diff.ResourceKind, diff.ResourceName)
					removedDiffs[diffKey] = diff
					p.config.Logger.Debug("Found resource to be removed", "resource", key)
				}
			}
		}

		for _, child := range node.Children {
			findRemovedResources(child)
		}
	}

	// Start the traversal from the root's children to skip the XR itself
	for _, child := range resourceTree.Children {
		findRemovedResources(child)
	}

	return removedDiffs, nil
}

// CalculateDiff calculates the diff for a single resource
func (p *DefaultDiffProcessor) CalculateDiff(ctx context.Context, composite string, desired *unstructured.Unstructured) (*ResourceDiff, error) {
	// Fetch current object from cluster
	current, _, err := p.fetchCurrentObject(ctx, composite, desired)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch current object")
	}

	p.makeObjectValid(desired)

	wouldBeResult := desired
	if current != nil {
		// Perform a dry-run apply to get the result after we'd apply
		wouldBeResult, err = p.client.DryRunApply(ctx, desired)
		if err != nil {
			return nil, errors.Wrap(err, "cannot dry-run apply desired object")
		}
	}

	// Get diff options from the processor configuration
	diffOpts := p.config.GetDiffOptions()

	// Generate diff with the configured options
	return GenerateDiffWithOptions(current, wouldBeResult, diffOpts)
}

// makeObjectValid makes sure all OwnerReferences have a valid UID
func (p *DefaultDiffProcessor) makeObjectValid(obj *unstructured.Unstructured) {
	// Get the current owner references
	refs := obj.GetOwnerReferences()

	// Create new slice to hold the updated references
	updatedRefs := make([]metav1.OwnerReference, 0, len(refs))

	// Set a valid UID for each reference
	for _, ref := range refs {
		if ref.UID == "" {
			ref.UID = uuid.NewUUID()
		}
		updatedRefs = append(updatedRefs, ref)
	}

	// Update the object with the modified owner references
	obj.SetOwnerReferences(updatedRefs)
}

// fetchCurrentObject retrieves the current state of the object from the cluster
// It returns the current object, a boolean indicating if it's a new object, and any error
func (p *DefaultDiffProcessor) fetchCurrentObject(ctx context.Context, composite string, desired *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	// Get the GroupVersionKind and name/namespace for lookup
	gvk := desired.GroupVersionKind()
	name := desired.GetName()
	namespace := desired.GetNamespace()

	var current *unstructured.Unstructured
	var err error
	isNewObject := false

	if composite != "" {
		// For composed resources, use the label selector approach
		sel := metav1.LabelSelector{
			MatchLabels: map[string]string{
				"crossplane.io/composite": composite,
			},
		}
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: fmt.Sprintf("%ss", strings.ToLower(gvk.Kind)), // naive pluralization
		}

		// Get the current object from the cluster using ClusterClient
		currents, err := p.client.GetResourcesByLabel(ctx, namespace, gvr, sel)
		if err != nil {
			return nil, false, errors.Wrap(err, "cannot get current object")
		}
		if len(currents) > 1 {
			return nil, false, errors.New(fmt.Sprintf("more than one matching resource found for %s/%s", gvk.Kind, name))
		}

		if len(currents) == 1 {
			current = currents[0]
		} else {
			isNewObject = true
		}
	} else {
		// For XRs, use direct lookup by name
		current, err = p.client.GetResource(ctx, gvk, namespace, name)
		if apierrors.IsNotFound(err) {
			isNewObject = true
		} else if err != nil {
			return nil, false, errors.Wrap(err, "cannot get current object")
		}
	}

	return current, isNewObject, nil
}

// RenderDiffs formats and prints the diffs to the provided writer
func (p *DefaultDiffProcessor) RenderDiffs(stdout io.Writer, diffs map[string]*ResourceDiff) error {
	// Get diff options from the processor configuration
	diffOpts := p.config.GetDiffOptions()

	// Sort the keys to ensure a consistent output order
	keys := make([]string, 0, len(diffs))
	for key := range diffs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		diff := diffs[key]

		// Format the diff header based on the diff type
		var header string
		switch diff.DiffType {
		case DiffTypeAdded:
			header = fmt.Sprintf("+++ %s/%s", diff.ResourceKind, diff.ResourceName)
		case DiffTypeRemoved:
			header = fmt.Sprintf("--- %s/%s", diff.ResourceKind, diff.ResourceName)
		case DiffTypeModified:
			header = fmt.Sprintf("~~~ %s/%s", diff.ResourceKind, diff.ResourceName)
		}

		// Format the diff content
		content := FormatDiff(diff.LineDiffs, diffOpts)

		if content != "" {
			_, _ = fmt.Fprintf(stdout, "%s\n%s\n---\n", header, content)
		}
	}

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
