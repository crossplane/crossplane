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

// Package rendertest contains shared test fixtures for the render packages
// (internal/render, cmd/crossplane/render). They are exported so that tests
// across packages can share them — they are not intended for production use.
package rendertest

import (
	"context"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// FatalFunctionServer is an in-process gRPC FunctionRunnerService server that
// announces a required resource on its first call and returns
// SEVERITY_FATAL on its second. It mirrors the function-extra-resources /
// function-environment-configs scenarios that motivated issue #7446: a
// function declares a requirement on the first call, then FATALs on the
// second call when the requirement still isn't satisfied.
type FatalFunctionServer struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	// RequirementName is the key of the required-resource selector returned
	// in the response's Requirements.Resources map.
	RequirementName string
	// Selector is the resource selector returned for RequirementName.
	Selector *fnv1.ResourceSelector
	// FatalMessage is the message attached to the SEVERITY_FATAL result on
	// the second call.
	FatalMessage string

	mu    sync.Mutex
	calls int
}

// RunFunction implements fnv1.FunctionRunnerServiceServer.
func (s *FatalFunctionServer) RunFunction(_ context.Context, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()

	if call == 1 {
		return &fnv1.RunFunctionResponse{
			Requirements: &fnv1.Requirements{
				Resources: map[string]*fnv1.ResourceSelector{
					s.RequirementName: s.Selector,
				},
			},
		}, nil
	}
	return &fnv1.RunFunctionResponse{
		Results: []*fnv1.Result{{Severity: fnv1.Severity_SEVERITY_FATAL, Message: s.FatalMessage}},
	}, nil
}

// StartFunctionServer starts an in-process gRPC server registered with the
// supplied FunctionRunnerServiceServer and returns its TCP address. The
// server is stopped automatically when the test ends.
func StartFunctionServer(t *testing.T, ss fnv1.FunctionRunnerServiceServer) string {
	t.Helper()

	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen for test gRPC server: %v", err)
	}

	s := grpc.NewServer()
	fnv1.RegisterFunctionRunnerServiceServer(s, ss)
	go func() { _ = s.Serve(lis) }()

	t.Cleanup(s.Stop)
	return lis.Addr().String()
}
