package diffprocessor

import (
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"k8s.io/client-go/rest"
)

// ProcessorConfig contains configuration for the DiffProcessor
type ProcessorConfig struct {
	// RestConfig is the Kubernetes REST configuration
	RestConfig *rest.Config

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

	// ComponentFactories provide factory functions for creating components
	ComponentFactories ComponentFactories
}

// ComponentFactories contains factory functions for creating processor components
type ComponentFactories struct {
	// ResourceManagerFactory creates a ResourceManager
	ResourceManagerFactory func(client cc.ClusterClient, logger logging.Logger) ResourceManager

	// SchemaValidatorFactory creates a SchemaValidator
	SchemaValidatorFactory func(client cc.ClusterClient, logger logging.Logger) SchemaValidator

	// DiffCalculatorFactory creates a DiffCalculator
	DiffCalculatorFactory func(client cc.ClusterClient, resourceManager ResourceManager, logger logging.Logger, diffOptions DiffOptions) DiffCalculator

	// DiffRendererFactory creates a DiffRenderer
	DiffRendererFactory func(logger logging.Logger, diffOptions DiffOptions) DiffRenderer

	// RequirementsProviderFactory creates an ExtraResourceProvider
	RequirementsProviderFactory func(client cc.ClusterClient, renderFunc RenderFunc, logger logging.Logger) *RequirementsProvider
}

// DiffProcessorOption defines a function that can modify a ProcessorConfig
type DiffProcessorOption func(*ProcessorConfig)

// WithNamespace sets the namespace for the processor
func WithNamespace(namespace string) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.Namespace = namespace
	}
}

// WithColorize sets whether to use colors in diff output
func WithColorize(colorize bool) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.Colorize = colorize
	}
}

// WithCompact sets whether to use compact diff format
func WithCompact(compact bool) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.Compact = compact
	}
}

// WithLogger sets the logger for the processor
func WithLogger(logger logging.Logger) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.Logger = logger
	}
}

// WithRenderFunc sets the render function for the processor
func WithRenderFunc(renderFn RenderFunc) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.RenderFunc = renderFn
	}
}

// WithRestConfig sets the REST config for the processor
func WithRestConfig(restConfig *rest.Config) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.RestConfig = restConfig
	}
}

// WithResourceManagerFactory sets the ResourceManager factory function
func WithResourceManagerFactory(factory func(client cc.ClusterClient, logger logging.Logger) ResourceManager) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.ComponentFactories.ResourceManagerFactory = factory
	}
}

// WithSchemaValidatorFactory sets the SchemaValidator factory function
func WithSchemaValidatorFactory(factory func(client cc.ClusterClient, logger logging.Logger) SchemaValidator) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.ComponentFactories.SchemaValidatorFactory = factory
	}
}

// WithDiffCalculatorFactory sets the DiffCalculator factory function
func WithDiffCalculatorFactory(factory func(client cc.ClusterClient, resourceManager ResourceManager, logger logging.Logger, diffOptions DiffOptions) DiffCalculator) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.ComponentFactories.DiffCalculatorFactory = factory
	}
}

// WithDiffRendererFactory sets the DiffRenderer factory function
func WithDiffRendererFactory(factory func(logger logging.Logger, diffOptions DiffOptions) DiffRenderer) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.ComponentFactories.DiffRendererFactory = factory
	}
}

// WithRequirementsProviderFactory sets the RequirementsProvider factory function
func WithRequirementsProviderFactory(factory func(client cc.ClusterClient, renderFunc RenderFunc, logger logging.Logger) *RequirementsProvider) DiffProcessorOption {
	return func(config *ProcessorConfig) {
		config.ComponentFactories.RequirementsProviderFactory = factory
	}
}

// GetDiffOptions returns DiffOptions based on the ProcessorConfig
func (c *ProcessorConfig) GetDiffOptions() DiffOptions {
	opts := DefaultDiffOptions()
	opts.UseColors = c.Colorize
	opts.Compact = c.Compact

	return opts
}

// SetDefaultFactories sets default component factory functions if not already set
func (c *ProcessorConfig) SetDefaultFactories() {
	if c.ComponentFactories.ResourceManagerFactory == nil {
		c.ComponentFactories.ResourceManagerFactory = NewResourceManager
	}

	if c.ComponentFactories.SchemaValidatorFactory == nil {
		c.ComponentFactories.SchemaValidatorFactory = NewSchemaValidator
	}

	if c.ComponentFactories.DiffCalculatorFactory == nil {
		c.ComponentFactories.DiffCalculatorFactory = NewDiffCalculator
	}

	if c.ComponentFactories.DiffRendererFactory == nil {
		c.ComponentFactories.DiffRendererFactory = NewDiffRenderer
	}

	if c.ComponentFactories.RequirementsProviderFactory == nil {
		c.ComponentFactories.RequirementsProviderFactory = func(client cc.ClusterClient, renderFunc RenderFunc, logger logging.Logger) *RequirementsProvider {
			// Create a new unified provider with empty environment configs (will be populated in Initialize)
			return NewRequirementsProvider(client, renderFunc, logger)
		}
	}
}
