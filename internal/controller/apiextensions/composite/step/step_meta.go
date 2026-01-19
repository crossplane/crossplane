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

// Package step contains types for managing function pipeline steps metadata.
package step

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// Metadata contains metadata for correlating and identifying a function
// invocation within a pipeline execution.
type Metadata struct {
	// TraceID is a UUID identifying the entire pipeline execution.
	// All function invocations within a single reconciliation share the same TraceID.
	TraceID string

	// SpanID is a UUID identifying this specific function invocation.
	SpanID string

	// Iteration counts how many times this step has been called, starting from 0.
	// Useful for tracking retries or repeated calls to the same step.
	Iteration int32

	// StepIndex is the zero-based index of this step in the function pipeline.
	StepIndex int32

	// FunctionName is the name of the function being invoked.
	FunctionName string

	// CompositionName is the name of the Composition defining this pipeline.
	CompositionName string

	// CompositeResourceUID is the UID of the composite resource being reconciled.
	CompositeResourceUID string

	// CompositeResourceName is the name of the composite resource being reconciled.
	CompositeResourceName string

	// CompositeResourceNamespace is the namespace of the composite resource
	// (empty for cluster-scoped resources).
	CompositeResourceNamespace string

	// CompositeResourceAPIVersion is the API version of the composite resource.
	CompositeResourceAPIVersion string

	// CompositeResourceKind is the kind of the composite resource.
	CompositeResourceKind string

	// Timestamp is when this step was executed.
	Timestamp time.Time
}

// Context keys for pipeline step metadata.
type contextKey string

const (
	// ContextKeyTraceID is the context key for the trace ID (shared across all steps in a reconciliation).
	ContextKeyTraceID contextKey = "trace-id"

	// ContextKeyStepIndex is the context key for the current pipeline step index.
	ContextKeyStepIndex contextKey = "pipeline-step-index"

	// ContextKeyCompositionName is the context key for the composition name.
	ContextKeyCompositionName contextKey = "composition-name"

	// ContextKeyIteration is the context key for the iteration counter (shared across all steps in a reconciliation).
	ContextKeyIteration contextKey = "iteration"
)

// ContextWithStepMeta returns a new context with pipeline step metadata.
func ContextWithStepMeta(ctx context.Context, traceID string, compositionName string, stepIndex, iteration int32) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, ContextKeyTraceID, traceID)
	ctx = context.WithValue(ctx, ContextKeyStepIndex, stepIndex)
	ctx = context.WithValue(ctx, ContextKeyCompositionName, compositionName)
	ctx = context.WithValue(ctx, ContextKeyIteration, iteration)
	return ctx
}

// ContextWithStepIteration returns a new context with an updated iteration counter.
// This is used by FetchingFunctionRunner to track how many times a step has been called.
func ContextWithStepIteration(ctx context.Context, iteration int32) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ContextKeyIteration, iteration)
}

// BuildMetadata builds Metadata from the given context and function request.
func BuildMetadata(ctx context.Context, functionName string, req *fnv1.RunFunctionRequest) (*Metadata, error) {
	meta := Metadata{
		FunctionName: functionName,
		Timestamp:    time.Now(),
	}

	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	// Extract trace_id, step index, composition name, and iteration from context.
	if v, ok := ctx.Value(ContextKeyTraceID).(string); ok {
		meta.TraceID = v
	} else {
		return nil, fmt.Errorf("could not extract trace ID from context")
	}
	if v, ok := ctx.Value(ContextKeyStepIndex).(int32); ok {
		meta.StepIndex = v
	} else {
		return nil, fmt.Errorf("could not extract step index from context")
	}
	if v, ok := ctx.Value(ContextKeyCompositionName).(string); ok {
		meta.CompositionName = v
	} else {
		return nil, fmt.Errorf("could not extract composition name from context")
	}
	if v, ok := ctx.Value(ContextKeyIteration).(int32); ok {
		meta.Iteration = v
		// This is optional; we can default to 0 if not found.
	}

	// Generate a unique span_id for this function invocation.
	meta.SpanID = uuid.NewString()

	// Extract composite resource metadata from the request.
	xr := req.GetObserved().GetComposite().GetResource()
	if xr != nil {
		meta.CompositeResourceAPIVersion = getStringField(xr, "apiVersion")
		meta.CompositeResourceKind = getStringField(xr, "kind")

		if metadata := getStructField(xr, "metadata"); metadata != nil {
			meta.CompositeResourceName = getStringField(metadata, "name")
			meta.CompositeResourceNamespace = getStringField(metadata, "namespace")
			meta.CompositeResourceUID = getStringField(metadata, "uid")
		}
	}

	return &meta, nil
}

// getStringField extracts a string value from a protobuf Struct.
func getStringField(s *structpb.Struct, key string) string {
	if s == nil {
		return ""
	}
	if v, ok := s.GetFields()[key]; ok {
		return v.GetStringValue()
	}
	return ""
}

// getStructField extracts a nested Struct value from a protobuf Struct.
func getStructField(s *structpb.Struct, key string) *structpb.Struct {
	if s == nil {
		return nil
	}
	if v, ok := s.GetFields()[key]; ok {
		return v.GetStructValue()
	}
	return nil
}
