package diffprocessor

import (
	"context"
	"dario.cat/mergo"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"io"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// RenderFunc defines the signature of a function that can render resources
type RenderFunc func(ctx context.Context, log logging.Logger, in render.Inputs) (render.Outputs, error)

// DiffProcessor interface for processing resources
type DiffProcessor interface {
	// ProcessAll handles all resources stored in the processor
	ProcessAll(stdout io.Writer, ctx context.Context, resources []*unstructured.Unstructured) error

	// ProcessResource handles one resource at a time
	ProcessResource(stdout io.Writer, ctx context.Context, res *unstructured.Unstructured) error

	// Initialize loads required resources like CRDs and environment configs
	Initialize(ctx context.Context) error
}

// DefaultDiffProcessor implements DiffProcessor with modular components
type DefaultDiffProcessor struct {
	client                cc.ClusterClient
	config                ProcessorConfig
	resourceManager       ResourceManager
	schemaValidator       SchemaValidator
	diffCalculator        DiffCalculator
	diffRenderer          DiffRenderer
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

	// Set default factory functions if not provided
	config.SetDefaultFactories()

	// Create the diff options based on configuration
	diffOpts := config.GetDiffOptions()

	// Create components using factories
	resourceManager := config.ComponentFactories.ResourceManagerFactory(client, config.Logger)
	schemaValidator := config.ComponentFactories.SchemaValidatorFactory(client, config.Logger)
	extraResourceProvider := config.ComponentFactories.ExtraResourceProviderFactory(client, config.RenderFunc, config.Logger)
	diffCalculator := config.ComponentFactories.DiffCalculatorFactory(client, resourceManager, config.Logger, diffOpts)
	diffRenderer := config.ComponentFactories.DiffRendererFactory(config.Logger, diffOpts)

	processor := &DefaultDiffProcessor{
		client:                client,
		config:                config,
		resourceManager:       resourceManager,
		schemaValidator:       schemaValidator,
		diffCalculator:        diffCalculator,
		diffRenderer:          diffRenderer,
		extraResourceProvider: extraResourceProvider,
	}

	return processor, nil
}

// Initialize loads required resources like CRDs and environment configs
func (p *DefaultDiffProcessor) Initialize(ctx context.Context) error {
	p.config.Logger.Debug("Initializing diff processor")

	// Load CRDs (handled by the schema validator)
	if err := p.initializeSchemaValidator(ctx); err != nil {
		return err
	}

	// Get and cache environment configs
	if err := p.initializeEnvironmentConfigs(ctx); err != nil {
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

// initializeEnvironmentConfigs initializes environment configs
func (p *DefaultDiffProcessor) initializeEnvironmentConfigs(ctx context.Context) error {
	environmentConfigs, err := p.client.GetEnvironmentConfigs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get environment configs")
	}

	// Update the EnvironmentConfigProvider with the fetched configs
	// This is specific to our CompositeExtraResourceProvider implementation
	if compositeProvider, ok := p.extraResourceProvider.(*CompositeExtraResourceProvider); ok {
		for _, provider := range compositeProvider.providers {
			if envProvider, ok := provider.(*EnvironmentConfigProvider); ok {
				envProvider.configs = environmentConfigs
				p.config.Logger.Debug("Environment config provider initialized",
					"configCount", len(environmentConfigs))
				break
			}
		}
	}

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

	xr, err, done := p.SanitizeXR(res, resourceID)
	if done {
		return err
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
		desired.CompositeResource.GetUnstructured(),
		xr.GetUnstructured(),
	)

	if err != nil {
		p.config.Logger.Debug("Failed to merge XR", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot merge input XR with result of rendered XR")
	}

	// Validate the resources
	if err := p.schemaValidator.ValidateResources(ctx, xrUnstructured, desired.ComposedResources); err != nil {
		p.config.Logger.Debug("Resource validation failed", "resource", resourceID, "error", err)
		return errors.Wrap(err, "cannot validate resources")
	}

	// Calculate all diffs
	p.config.Logger.Debug("Calculating diffs", "resource", resourceID)
	diffs, err := p.diffCalculator.CalculateDiffs(ctx, xr, desired)
	if err != nil {
		// We don't fail completely if some diffs couldn't be calculated
		p.config.Logger.Debug("Partial error calculating diffs", "resource", resourceID, "error", err)
	}

	// Render and print the diffs
	diffErr := p.diffRenderer.RenderDiffs(stdout, diffs)
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

func (p *DefaultDiffProcessor) SanitizeXR(res *unstructured.Unstructured, resourceID string) (*ucomposite.Unstructured, error, bool) {
	// Convert the unstructured resource to a composite unstructured for rendering
	xr := ucomposite.New()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), xr); err != nil {
		p.config.Logger.Debug("Failed to convert resource", "resource", resourceID, "error", err)
		return nil, errors.Wrap(err, "cannot convert XR to composite unstructured"), true
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
	return xr, nil, false
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
