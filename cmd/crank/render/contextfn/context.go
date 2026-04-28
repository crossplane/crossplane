/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package contextfn implements an in-process composition function that the
// CLI hosts to inject and capture pipeline context for `crossplane render`.
// It replaces the previous function-go-templating based approach so the CLI
// no longer depends on an external function image for context handling.
package contextfn

import (
	"context"
	"encoding/json"
	"maps"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// FunctionName is the pkgv1.Function.Name used for the in-process context
// function. It is exported because callers must key the FunctionInput
// address map with it.
const FunctionName = "crossplane-render-context"

const (
	stepSeed    = "crossplane-render-inject-context"
	stepCapture = "crossplane-render-extract-context"

	modeSeed    = "Seed"
	modeCapture = "Capture"
)

// input is the pipeline step input for the context function. The only field
// it carries is the mode: because the function runs in-process with the CLI,
// context data does not need to round-trip through the wire.
type input struct {
	Mode string `json:"mode"`
}

// server implements the v1 FunctionRunnerService in-process. It holds the
// CLI-parsed context data and records the end-of-pipeline context.
type server struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	contextData map[string]any

	mu       sync.Mutex
	captured *structpb.Struct
}

func newServer(contextData map[string]any) *server {
	return &server{contextData: contextData}
}

// capturedContext returns the context observed by the most recent capture
// invocation, or nil if capture has not run.
func (s *server) capturedContext() *structpb.Struct {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.captured
}

// RunFunction handles both seed and capture modes based on the input.
// Exported because it implements fnv1.FunctionRunnerServiceServer.
func (s *server) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	in := input{}
	if raw := req.GetInput(); raw != nil {
		b, err := raw.MarshalJSON()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "cannot marshal input: %v", err)
		}
		if err := json.Unmarshal(b, &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "cannot unmarshal input: %v", err)
		}
	}

	switch in.Mode {
	case modeSeed:
		return s.seed(req)
	case modeCapture:
		return s.capture(req)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported mode %q", in.Mode)
	}
}

func (s *server) seed(req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	merged := map[string]any{}
	if c := req.GetContext(); c != nil {
		merged = c.AsMap()
	}
	maps.Copy(merged, s.contextData)

	sp, err := structpb.NewStruct(merged)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot build context struct: %v", err)
	}

	return &fnv1.RunFunctionResponse{
		Meta:    &fnv1.ResponseMeta{Tag: req.GetMeta().GetTag()},
		Desired: req.GetDesired(),
		Context: sp,
	}, nil
}

func (s *server) capture(req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	var captured *structpb.Struct
	if c := req.GetContext(); c != nil {
		clone, ok := proto.Clone(c).(*structpb.Struct)
		if !ok {
			return nil, status.Errorf(codes.Internal, "unexpected type from proto.Clone")
		}
		captured = clone
	}

	s.mu.Lock()
	s.captured = captured
	s.mu.Unlock()

	return &fnv1.RunFunctionResponse{
		Meta:    &fnv1.ResponseMeta{Tag: req.GetMeta().GetTag()},
		Desired: req.GetDesired(),
	}, nil
}
