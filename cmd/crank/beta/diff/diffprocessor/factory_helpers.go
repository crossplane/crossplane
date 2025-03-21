package diffprocessor

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"k8s.io/client-go/rest"
)

// ProcessorFactory is a helper for creating a DiffProcessor with common options
type ProcessorFactory struct {
	Options []DiffProcessorOption
}

// NewProcessorFactory creates a new ProcessorFactory with default settings
func NewProcessorFactory() *ProcessorFactory {
	return &ProcessorFactory{
		Options: []DiffProcessorOption{},
	}
}

// WithNamespace sets the namespace for the processor
func (f *ProcessorFactory) WithNamespace(namespace string) *ProcessorFactory {
	f.Options = append(f.Options, WithNamespace(namespace))
	return f
}

// WithColorize sets whether to use colors in diff output
func (f *ProcessorFactory) WithColorize(colorize bool) *ProcessorFactory {
	f.Options = append(f.Options, WithColorize(colorize))
	return f
}

// WithCompact sets whether to use compact diff format
func (f *ProcessorFactory) WithCompact(compact bool) *ProcessorFactory {
	f.Options = append(f.Options, WithCompact(compact))
	return f
}

// WithLogger sets the logger for the processor
func (f *ProcessorFactory) WithLogger(logger logging.Logger) *ProcessorFactory {
	f.Options = append(f.Options, WithLogger(logger))
	return f
}

// WithRenderFunc sets the render function for the processor
func (f *ProcessorFactory) WithRenderFunc(renderFn RenderFunc) *ProcessorFactory {
	f.Options = append(f.Options, WithRenderFunc(renderFn))
	return f
}

// WithRestConfig sets the REST config for the processor
func (f *ProcessorFactory) WithRestConfig(restConfig *rest.Config) *ProcessorFactory {
	f.Options = append(f.Options, WithRestConfig(restConfig))
	return f
}

// WithOption adds a custom option to the processor factory
func (f *ProcessorFactory) WithOption(option DiffProcessorOption) *ProcessorFactory {
	f.Options = append(f.Options, option)
	return f
}

// Build creates a new DiffProcessor with the configured options
func (f *ProcessorFactory) Build(client cc.ClusterClient) (DiffProcessor, error) {
	return NewDiffProcessor(client, f.Options...)
}

// BuildAndInitialize creates a new DiffProcessor, initializes it, and returns it
func (f *ProcessorFactory) BuildAndInitialize(ctx context.Context, client cc.ClusterClient) (DiffProcessor, error) {
	processor, err := f.Build(client)
	if err != nil {
		return nil, err
	}

	if err := processor.Initialize(ctx); err != nil {
		return nil, err
	}

	return processor, nil
}
