// Package diffprocessor contains the logic to calculate and render diffs.
package diffprocessor

import (
	"context"
	"dario.cat/mergo"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	cmp "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xp "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/crossplane"
	k8 "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/kubernetes"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer"
	dt "github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer/types"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
)

// RenderFunc defines the signature of a function that can render resources
type RenderFunc func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error)

// DiffProcessor interface for processing resources
type DiffProcessor interface {
	// PerformDiff processes all resources and produces a diff output
	PerformDiff(ctx context.Context, stdout io.Writer, resources []*un.Unstructured) error

	// Initialize loads required resources like CRDs and environment configs
	Initialize(ctx context.Context) error
}

// DefaultDiffProcessor implements DiffProcessor with modular components
type DefaultDiffProcessor struct {
	fnClient             xp.FunctionClient
	compClient           xp.CompositionClient
	config               ProcessorConfig
	schemaValidator      SchemaValidator
	diffCalculator       DiffCalculator
	diffRenderer         renderer.DiffRenderer
	requirementsProvider *RequirementsProvider
}

// NewDiffProcessor creates a new DefaultDiffProcessor with the provided options
func NewDiffProcessor(k8cs k8.Clients, xpcs xp.Clients, opts ...ProcessorOption) DiffProcessor {
	// Create default configuration
	config := ProcessorConfig{
		Namespace:  "default",
		Colorize:   true,
		Compact:    false,
		Logger:     logging.NewNopLogger(),
		RenderFunc: render.Render,
	}

	// Apply all provided options
	for _, option := range opts {
		option(&config)
	}

	// Set default factory functions if not provided
	config.SetDefaultFactories()

	// Create the diff options based on configuration
	diffOpts := config.GetDiffOptions()

	// Create components using factories
	resourceManager := config.Factories.ResourceManager(k8cs.Resource, config.Logger)
	schemaValidator := config.Factories.SchemaValidator(k8cs.Schema, xpcs.Definition, config.Logger)
	requirementsProvider := config.Factories.RequirementsProvider(k8cs.Resource, xpcs.Environment, config.RenderFunc, config.Logger)
	diffCalculator := config.Factories.DiffCalculator(k8cs.Apply, xpcs.ResourceTree, resourceManager, config.Logger, diffOpts)
	diffRenderer := config.Factories.DiffRenderer(config.Logger, diffOpts)

	processor := &DefaultDiffProcessor{
		fnClient:             xpcs.Function,
		compClient:           xpcs.Composition,
		config:               config,
		schemaValidator:      schemaValidator,
		diffCalculator:       diffCalculator,
		diffRenderer:         diffRenderer,
		requirementsProvider: requirementsProvider,
	}

	return processor
}

// Initialize loads required resources like CRDs and environment configs
func (p *DefaultDiffProcessor) Initialize(ctx context.Context) error {
	p.config.Logger.Debug("Initializing diff processor")

	// Load CRDs (handled by the schema validator)
	if err := p.initializeSchemaValidator(ctx); err != nil {
		return err
	}

	// Init requirements provider
	if err := p.requirementsProvider.Initialize(ctx); err != nil {
		return err
	}

	p.config.Logger.Debug("Diff processor initialized")
	return nil
}

// initializeSchemaValidator initializes the schema validator with CRDs
func (p *DefaultDiffProcessor) initializeSchemaValidator(ctx context.Context) error {
	// If the schema validator implements our interface with LoadCRDs, use it
	if validator, ok := p.schemaValidator.(*DefaultSchemaValidator); ok {
		if err := validator.LoadCRDs(ctx); err != nil {
			return errors.Wrap(err, "cannot load CRDs")
		}
		p.config.Logger.Debug("Schema validator initialized with CRDs",
			"crdCount", len(validator.GetCRDs()))
	}
	return nil
}

// PerformDiff processes all resources and produces a diff output
func (p *DefaultDiffProcessor) PerformDiff(ctx context.Context, stdout io.Writer, resources []*un.Unstructured) error {
	p.config.Logger.Debug("Processing resources", "count", len(resources))

	if len(resources) == 0 {
		p.config.Logger.Debug("No resources to process")
		return nil
	}

	// Collect all diffs across all resources
	allDiffs := make(map[string]*dt.ResourceDiff)
	var errs []error

	for _, res := range resources {
		resourceID := fmt.Sprintf("%s/%s", res.GetKind(), res.GetName())

		diffs, err := p.DiffSingleResource(ctx, res)
		if err != nil {
			p.config.Logger.Debug("Failed to process resource", "resource", resourceID, "error", err)
			errs = append(errs, errors.Wrapf(err, "unable to process resource %s", resourceID))
		} else {
			// Merge the diffs into our combined map
			for k, v := range diffs {
				allDiffs[k] = v
			}
		}
	}

	// Only render diffs if we found some
	if len(allDiffs) > 0 {
		// Render all diffs in a single pass
		if err := p.diffRenderer.RenderDiffs(stdout, allDiffs); err != nil {
			p.config.Logger.Debug("Failed to render diffs", "error", err)
			errs = append(errs, errors.Wrap(err, "failed to render diffs"))
		}
	}

	p.config.Logger.Debug("Processing complete",
		"resourceCount", len(resources),
		"totalDiffs", len(allDiffs),
		"errorCount", len(errs))

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// DiffSingleResource handles one resource at a time and returns its diffs
func (p *DefaultDiffProcessor) DiffSingleResource(ctx context.Context, res *un.Unstructured) (map[string]*dt.ResourceDiff, error) {
	resourceID := fmt.Sprintf("%s/%s", res.GetKind(), res.GetName())
	p.config.Logger.Debug("Processing resource", "resource", resourceID)

	xr, done, err := p.SanitizeXR(res, resourceID)
	if done {
		return nil, err
	}

	// Find the matching composition
	comp, err := p.compClient.FindMatchingComposition(ctx, res)
	if err != nil {
		p.config.Logger.Debug("No matching composition found", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot find matching composition")
	}

	p.config.Logger.Debug("Resource setup complete",
		"resource", resourceID,
		"composition", comp.GetName())

	// Get functions for rendering
	fns, err := p.fnClient.GetFunctionsFromPipeline(comp)
	if err != nil {
		p.config.Logger.Debug("Failed to get functions", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot get functions from pipeline")
	}

	// Perform iterative rendering and requirements reconciliation
	desired, err := p.RenderWithRequirements(ctx, xr, comp, fns, resourceID)
	if err != nil {
		p.config.Logger.Debug("Resource rendering failed", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot render resources with requirements")
	}

	// Merge the result of the render together with the input XR
	p.config.Logger.Debug("Merging and validating rendered resources",
		"resource", resourceID,
		"composedCount", len(desired.ComposedResources))

	xrUnstructured, err := mergeUnstructured(
		desired.CompositeResource.GetUnstructured(),
		xr.GetUnstructured(),
	)

	if err != nil {
		p.config.Logger.Debug("Failed to merge XR", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot merge input XR with result of rendered XR")
	}

	// Validate the resources
	if err := p.schemaValidator.ValidateResources(ctx, xrUnstructured, desired.ComposedResources); err != nil {
		p.config.Logger.Debug("Resource validation failed", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot validate resources")
	}

	// Calculate all diffs
	p.config.Logger.Debug("Calculating diffs", "resource", resourceID)
	diffs, err := p.diffCalculator.CalculateDiffs(ctx, xr, desired)
	if err != nil {
		// We don't fail completely if some diffs couldn't be calculated
		p.config.Logger.Debug("Partial error calculating diffs", "resource", resourceID, "error", err)
	}

	p.config.Logger.Debug("Resource processing complete",
		"resource", resourceID,
		"diffCount", len(diffs),
		"hasErrors", err != nil)

	return diffs, err
}

// SanitizeXR makes an XR into a valid unstructured object that we can use in a dry-run apply
func (p *DefaultDiffProcessor) SanitizeXR(res *un.Unstructured, resourceID string) (*cmp.Unstructured, bool, error) {
	// Convert the unstructured resource to a composite unstructured for rendering
	xr := cmp.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		p.config.Logger.Debug("Failed to convert resource", "resource", resourceID, "error", err)
		return nil, true, errors.Wrap(err, "cannot convert XR to composite unstructured")
	}

	// Handle XRs with generateName but no name
	if xr.GetName() == "" && xr.GetGenerateName() != "" {
		// Create a display name for the diff
		displayName := xr.GetGenerateName() + "(generated)"
		p.config.Logger.Debug("Setting display name for XR with generateName",
			"generateName", xr.GetGenerateName(),
			"displayName", displayName)

		// Set this display name on the XR for rendering
		xrCopy := xr.DeepCopy()
		xrCopy.SetName(displayName)
		xr = xrCopy
	}
	return xr, false, nil
}

// mergeUnstructured merges two unstructured objects
func mergeUnstructured(dest *un.Unstructured, src *un.Unstructured) (*un.Unstructured, error) {
	// Start with a deep copy of the rendered resource
	result := dest.DeepCopy()
	if err := mergo.Merge(&result.Object, src.Object, mergo.WithOverride); err != nil {
		return nil, errors.Wrap(err, "cannot merge unstructured objects")
	}
	return result, nil
}

// RenderWithRequirements performs an iterative rendering process that discovers and fulfills requirements
func (p *DefaultDiffProcessor) RenderWithRequirements(
	ctx context.Context,
	xr *cmp.Unstructured,
	comp *apiextensionsv1.Composition,
	fns []pkgv1.Function,
	resourceID string,
) (render.Outputs, error) {
	// Start with environment configs as baseline extra resources
	var renderResources []un.Unstructured

	// Track all discovered extra resources to return at the end
	var discoveredResources []*un.Unstructured

	// Track resources we've already discovered to detect when we're done
	discoveredResourcesMap := make(map[string]bool)

	// Set up for iterative discovery
	const maxIterations = 10 // Prevent infinite loops
	var lastOutput render.Outputs
	var lastRenderErr error

	// Track the number of iterations for logging
	iteration := 0

	// Iteratively discover and fetch resources until we have all requirements
	// or until we hit the max iterations limit
	for iteration < maxIterations {
		iteration++
		p.config.Logger.Debug("Performing render iteration to identify requirements",
			"resource", resourceID,
			"iteration", iteration,
			"resourceCount", len(renderResources))

		// Perform render to get requirements
		output, renderErr := p.config.RenderFunc(ctx, p.config.Logger, render.Inputs{
			CompositeResource: xr,
			Composition:       comp,
			Functions:         fns,
			ExtraResources:    renderResources,
		})

		lastOutput = output
		lastRenderErr = renderErr

		// Handle the case where rendering failed but we still have requirements
		var hasRequirements bool

		// Use reflection to safely check if output is non-nil and has Requirements
		if v := reflect.ValueOf(output); v.IsValid() {
			// Check if it has a Requirements field
			if requirements := v.FieldByName("Requirements"); requirements.IsValid() && !requirements.IsNil() {
				hasRequirements = true
			}
		}

		// If we got an error and there are no usable requirements, fail
		if renderErr != nil && !hasRequirements {
			p.config.Logger.Debug("Resource rendering failed completely",
				"resource", resourceID,
				"iteration", iteration,
				"error", renderErr)
			return render.Outputs{}, errors.Wrap(renderErr, "cannot render resources")
		}

		// Log if we're continuing despite render errors
		if renderErr != nil { // && hasRequirements {
			p.config.Logger.Debug("Resource rendering had errors but returned requirements",
				"resource", resourceID,
				"iteration", iteration,
				"error", renderErr,
				"requirementCount", len(output.Requirements))
		}

		// If no requirements, we're done
		if len(output.Requirements) == 0 {
			p.config.Logger.Debug("No more requirements found, discovery complete",
				"iteration", iteration)
			break
		}

		// Process requirements from the render output
		p.config.Logger.Debug("Processing requirements from render output",
			"iteration", iteration,
			"requirementCount", len(output.Requirements))

		additionalResources, err := p.requirementsProvider.ProvideRequirements(ctx, output.Requirements)
		if err != nil {
			return render.Outputs{}, errors.Wrap(err, "failed to process requirements")
		}

		// If no new resources were found, we're done
		if len(additionalResources) == 0 {
			p.config.Logger.Debug("No new resources found from requirements, discovery complete",
				"iteration", iteration)
			break
		}

		// Check if we've already discovered these resources
		newResourcesFound := false
		for _, res := range additionalResources {
			resourceKey := fmt.Sprintf("%s/%s", res.GetAPIVersion(), res.GetName())
			if !discoveredResourcesMap[resourceKey] {
				discoveredResourcesMap[resourceKey] = true
				newResourcesFound = true

				// Add to our collection of extra resources
				discoveredResources = append(discoveredResources, res)

				// Add to render resources for next iteration
				renderResources = append(renderResources, *res)
			}
		}

		// If no new resources were found, we've reached a stable state
		if !newResourcesFound {
			p.config.Logger.Debug("No new unique resources found, discovery complete",
				"iteration", iteration)
			break
		}

		p.config.Logger.Debug("Found additional resources to incorporate",
			"resource", resourceID,
			"iteration", iteration,
			"additionalCount", len(additionalResources),
			"totalResourcesNow", len(discoveredResources))
	}

	// Log if we hit the iteration limit
	if iteration >= maxIterations {
		p.config.Logger.Info("Reached maximum iteration limit for resource discovery",
			"resource", resourceID,
			"maxIterations", maxIterations)
	}

	// If we exited the loop with a render error but still found resources,
	// log it but don't fail the process
	if lastRenderErr != nil {
		p.config.Logger.Debug("Completed resource discovery with render errors",
			"resource", resourceID,
			"iterations", iteration,
			"error", lastRenderErr)
	}

	p.config.Logger.Debug("Finished discovering and rendering resources",
		"totalExtraResources", len(discoveredResources),
		"iterations", iteration)

	return lastOutput, lastRenderErr
}
