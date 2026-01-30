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
	"google.golang.org/protobuf/types/known/timestamppb"

	pipelinev1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/pipelineinspector/proto/v1alpha1"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// Context keys for pipeline step metadata.
type contextKey string

const (
	// ContextKeyTraceID is the context key for the trace ID (shared across all steps in a reconciliation).
	ContextKeyTraceID contextKey = "trace-id"

	// ContextKeyStepIndex is the context key for the current pipeline step index.
	ContextKeyStepIndex contextKey = "pipeline-step-index"

	// ContextKeyStepName is the context key for the current pipeline step name.
	ContextKeyStepName contextKey = "pipeline-step-name"

	// ContextKeyCompositionName is the context key for the composition name.
	ContextKeyCompositionName contextKey = "composition-name"

	// ContextKeyIteration is the context key for the iteration counter (shared across all steps in a reconciliation).
	ContextKeyIteration contextKey = "iteration"
)

// ContextWithStepMeta returns a new context with pipeline step metadata.
func ContextWithStepMeta(ctx context.Context, traceID string, compositionName string, stepName string, stepIndex, iteration int32) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, ContextKeyTraceID, traceID)
	ctx = context.WithValue(ctx, ContextKeyStepIndex, stepIndex)
	ctx = context.WithValue(ctx, ContextKeyStepName, stepName)
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
func BuildMetadata(ctx context.Context, functionName string, req *fnv1.RunFunctionRequest) (*pipelinev1alpha1.StepMeta, error) {
	if req == nil {
		return nil, fmt.Errorf("function request should not be nil")
	}

	meta := pipelinev1alpha1.StepMeta{
		FunctionName: functionName,
		Timestamp:    timestamppb.New(time.Now()),
	}

	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	// Extract trace_id, step index, composition name, and iteration from context.
	if v, ok := ctx.Value(ContextKeyTraceID).(string); ok {
		meta.TraceId = v
	} else {
		return nil, fmt.Errorf("could not extract trace ID from context")
	}
	if v, ok := ctx.Value(ContextKeyStepIndex).(int32); ok {
		meta.StepIndex = v
	} else {
		return nil, fmt.Errorf("could not extract step index from context")
	}
	if v, ok := ctx.Value(ContextKeyStepName).(string); ok {
		meta.StepName = v
	} else {
		return nil, fmt.Errorf("could not extract step name from context")
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
	meta.SpanId = uuid.NewString()

	// Extract composite resource metadata from the request.
	xr := req.GetObserved().GetComposite().GetResource()
	if xr != nil {
		meta.CompositeResourceApiVersion = getStringField(xr, "apiVersion")
		meta.CompositeResourceKind = getStringField(xr, "kind")

		if metadata := getStructField(xr, "metadata"); metadata != nil {
			meta.CompositeResourceName = getStringField(metadata, "name")
			meta.CompositeResourceNamespace = getStringField(metadata, "namespace")
			meta.CompositeResourceUid = getStringField(metadata, "uid")
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
