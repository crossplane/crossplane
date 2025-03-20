package diffprocessor

import (
	"github.com/crossplane/crossplane-runtime/pkg/logging"
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

// GetDiffOptions returns DiffOptions based on the ProcessorConfig
func (c *ProcessorConfig) GetDiffOptions() DiffOptions {
	opts := DefaultDiffOptions()
	opts.UseColors = c.Colorize
	opts.Compact = c.Compact

	return opts
}
