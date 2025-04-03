package diffprocessor

import (
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	xp "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/crossplane"
	k8 "github.com/crossplane/crossplane/cmd/crank/beta/diff/client/kubernetes"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer"
)

// ProcessorConfig contains configuration for the DiffProcessor
type ProcessorConfig struct {
	// Namespace is the namespace to use for resources
	Namespace string

	// Colorize determines whether to use colors in the diff output
	Colorize bool

	// Compact determines whether to show a compact diff format
	Compact bool

	// Logger is the logger to use
	Logger logging.Logger

	// RenderFunc is the function to use for rendering resources
	RenderFunc RenderFunc

	// Factories provide factory functions for creating components
	Factories ComponentFactories
}

// ComponentFactories contains factory functions for creating processor components
type ComponentFactories struct {
	// ResourceManager creates a ResourceManager
	ResourceManager func(client k8.ResourceClient, logger logging.Logger) ResourceManager

	// SchemaValidator creates a SchemaValidator
	SchemaValidator func(schema k8.SchemaClient, def xp.DefinitionClient, logger logging.Logger) SchemaValidator

	// DiffCalculator creates a DiffCalculator
	DiffCalculator func(apply k8.ApplyClient, tree xp.ResourceTreeClient, resourceManager ResourceManager, logger logging.Logger, diffOptions renderer.DiffOptions) DiffCalculator

	// DiffRenderer creates a DiffRenderer
	DiffRenderer func(logger logging.Logger, diffOptions renderer.DiffOptions) renderer.DiffRenderer

	// RequirementsProvider creates an ExtraResourceProvider
	RequirementsProvider func(res k8.ResourceClient, def xp.EnvironmentClient, renderFunc RenderFunc, logger logging.Logger) *RequirementsProvider
}

// ProcessorOption defines a function that can modify a ProcessorConfig
type ProcessorOption func(*ProcessorConfig)

// WithNamespace sets the namespace for the processor
func WithNamespace(namespace string) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Namespace = namespace
	}
}

// WithColorize sets whether to use colors in diff output
func WithColorize(colorize bool) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Colorize = colorize
	}
}

// WithCompact sets whether to use compact diff format
func WithCompact(compact bool) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Compact = compact
	}
}

// WithLogger sets the logger for the processor
func WithLogger(logger logging.Logger) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Logger = logger
	}
}

// WithRenderFunc sets the render function for the processor
func WithRenderFunc(renderFn RenderFunc) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.RenderFunc = renderFn
	}
}

// WithResourceManagerFactory sets the ResourceManager factory function
func WithResourceManagerFactory(factory func(k8.ResourceClient, logging.Logger) ResourceManager) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Factories.ResourceManager = factory
	}
}

// WithSchemaValidatorFactory sets the SchemaValidator factory function
func WithSchemaValidatorFactory(factory func(k8.SchemaClient, xp.DefinitionClient, logging.Logger) SchemaValidator) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Factories.SchemaValidator = factory
	}
}

// WithDiffCalculatorFactory sets the DiffCalculator factory function
func WithDiffCalculatorFactory(factory func(k8.ApplyClient, xp.ResourceTreeClient, ResourceManager, logging.Logger, renderer.DiffOptions) DiffCalculator) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Factories.DiffCalculator = factory
	}
}

// WithDiffRendererFactory sets the DiffRenderer factory function
func WithDiffRendererFactory(factory func(logging.Logger, renderer.DiffOptions) renderer.DiffRenderer) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Factories.DiffRenderer = factory
	}
}

// WithRequirementsProviderFactory sets the RequirementsProvider factory function
func WithRequirementsProviderFactory(factory func(k8.ResourceClient, xp.EnvironmentClient, RenderFunc, logging.Logger) *RequirementsProvider) ProcessorOption {
	return func(config *ProcessorConfig) {
		config.Factories.RequirementsProvider = factory
	}
}

// GetDiffOptions returns DiffOptions based on the ProcessorConfig
func (c *ProcessorConfig) GetDiffOptions() renderer.DiffOptions {
	opts := renderer.DefaultDiffOptions()
	opts.UseColors = c.Colorize
	opts.Compact = c.Compact

	return opts
}

// SetDefaultFactories sets default component factory functions if not already set
func (c *ProcessorConfig) SetDefaultFactories() {
	if c.Factories.ResourceManager == nil {
		c.Factories.ResourceManager = NewResourceManager
	}

	if c.Factories.SchemaValidator == nil {
		c.Factories.SchemaValidator = NewSchemaValidator
	}

	if c.Factories.DiffCalculator == nil {
		c.Factories.DiffCalculator = NewDiffCalculator
	}

	if c.Factories.DiffRenderer == nil {
		c.Factories.DiffRenderer = renderer.NewDiffRenderer
	}

	if c.Factories.RequirementsProvider == nil {
		c.Factories.RequirementsProvider = NewRequirementsProvider
	}
}
