/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

// Package inspected contains a FunctionRunner that emits request and response
// data to a PipelineInspector for inspection.
package inspected

import (
	"context"

	pipelinev1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/pipelineinspector/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/step"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// Metrics for the pipeline inspector.
type Metrics interface {
	// ErrorOnRequest errors encountered emitting RunFunctionRequest data.
	ErrorOnRequest(name string)

	// ErrorOnResponse records errors encountered emitting RunFunctionResponse data.
	ErrorOnResponse(name string)
}

// A FunctionRunner runs a composition function.
type FunctionRunner interface {
	// RunFunction runs the named composition function with the given request.
	RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)
}

// A PipelineInspector inspects function pipeline execution data.
type PipelineInspector interface {
	// EmitRequest emits the given request and metadata.
	EmitRequest(ctx context.Context, req *fnv1.RunFunctionRequest, meta *pipelinev1alpha1.StepMeta) error

	// EmitResponse emits the given response, error, and metadata.
	EmitResponse(ctx context.Context, rsp *fnv1.RunFunctionResponse, err error, meta *pipelinev1alpha1.StepMeta) error
}

// A Runner is a FunctionRunner that wraps another
// FunctionRunner and emits request and response data to a PipelineInspector.
type Runner struct {
	wrapped   FunctionRunner
	inspector PipelineInspector
	metrics   Metrics
	log       logging.Logger
}

// An RunnerOption configures an InspectedRunner.
type RunnerOption func(*Runner)

// WithMetrics sets the metrics used by the InspectedRunner.
func WithMetrics(m Metrics) RunnerOption {
	return func(r *Runner) {
		r.metrics = m
	}
}

// WithLogger sets the logger used by the InspectedRunner.
func WithLogger(l logging.Logger) RunnerOption {
	return func(r *Runner) {
		r.log = l
	}
}

// NewInspectedRunner creates a new TeeFunctionRunner that wraps the given
// FunctionRunner and emits data to the given PipelineInspector.
func NewInspectedRunner(wrapped FunctionRunner, inspector PipelineInspector, opts ...RunnerOption) *Runner {
	r := &Runner{
		wrapped:   wrapped,
		inspector: inspector,
		metrics:   &NopMetrics{},
		log:       logging.NewNopLogger(),
	}

	for _, fn := range opts {
		fn(r)
	}

	return r
}

// RunFunction runs the named composition function, emitting the request before
// execution and the response after execution.
func (r *Runner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	meta, err := step.BuildMetadata(ctx, name, req)
	if err != nil {
		r.log.Info("failed to extract step metadata, skipping inspection", "function", name, "error", err)
		// Skip inspection if we can't build metadata, but proceed with function execution.
		return r.wrapped.RunFunction(ctx, name, req)
	}

	if err := r.inspector.EmitRequest(ctx, req, meta); err != nil {
		r.metrics.ErrorOnRequest(name)
		r.log.Info("failed to inspect request for function", "function", name, "error", err)
	}

	// Run the wrapped function.
	rsp, err := r.wrapped.RunFunction(ctx, name, req)

	if err := r.inspector.EmitResponse(ctx, rsp, err, meta); err != nil {
		r.metrics.ErrorOnResponse(name)
		r.log.Info("failed to inspect response for function", "function", name, "error", err)
	}

	return rsp, err
}
