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

package render

import (
	"bytes"
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// fatalFunctionServer announces a required resource on its first call and
// returns SEVERITY_FATAL on its second, mirroring the
// function-extra-resources / function-environment-configs scenarios that
// motivated issue #7446.
type fatalFunctionServer struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	requirementName string
	selector        *fnv1.ResourceSelector
	fatalMessage    string

	mu    sync.Mutex
	calls int
}

func (s *fatalFunctionServer) RunFunction(_ context.Context, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()

	rsp := &fnv1.RunFunctionResponse{
		Requirements: &fnv1.Requirements{
			Resources: map[string]*fnv1.ResourceSelector{
				s.requirementName: s.selector,
			},
		},
	}
	if call > 1 {
		rsp.Results = []*fnv1.Result{{Severity: fnv1.Severity_SEVERITY_FATAL, Message: s.fatalMessage}}
	}
	return rsp, nil
}

func startTestFunctionServer(t *testing.T, ss fnv1.FunctionRunnerServiceServer) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen for test gRPC server: %v", err)
	}
	s := grpc.NewServer()
	fnv1.RegisterFunctionRunnerServiceServer(s, ss)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)
	return lis.Addr().String()
}

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}
	return s
}

// mustMarshalRequest serializes a RenderRequest to a buffer that the CLI can
// read from stdin.
func mustMarshalRequest(t *testing.T, req *renderv1alpha1.RenderRequest) *bytes.Buffer {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("proto.Marshal request: %v", err)
	}
	return bytes.NewBuffer(data)
}

func TestRunWritesPartialResponseOnPipelineFatal(t *testing.T) {
	const stepName = "fetch-extras"
	const fatalMsg = "Required extra resource \"namedClusterRole\" not found"
	const requirementName = "namedClusterRole"

	wantSelector := &fnv1.ResourceSelector{
		ApiVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Match: &fnv1.ResourceSelector_MatchName{
			MatchName: "some-cluster-role",
		},
	}

	addr := startTestFunctionServer(t, &fatalFunctionServer{
		requirementName: requirementName,
		selector:        wantSelector,
		fatalMessage:    fatalMsg,
	})

	req := &renderv1alpha1.RenderRequest{
		Input: &renderv1alpha1.RenderRequest_Composite{
			Composite: &renderv1alpha1.CompositeInput{
				CompositeResource: mustStruct(t, map[string]any{
					"apiVersion": "example.org/v1alpha1",
					"kind":       "XExample",
					"metadata":   map[string]any{"name": "my-example"},
				}),
				Composition: mustStruct(t, map[string]any{
					"metadata": map[string]any{"name": "example-composition"},
					"spec": map[string]any{
						"compositeTypeRef": map[string]any{
							"apiVersion": "example.org/v1alpha1",
							"kind":       "XExample",
						},
						"mode": "Pipeline",
						"pipeline": []any{
							map[string]any{
								"step":        stepName,
								"functionRef": map[string]any{"name": "function-extra-resources"},
							},
						},
					},
				}),
				Functions: []*renderv1alpha1.FunctionInput{
					{Name: "function-extra-resources", Address: addr},
				},
			},
		},
	}

	stdin := mustMarshalRequest(t, req)
	stdout := &bytes.Buffer{}

	cmd := &Command{
		Timeout: 30 * time.Second,
		stdin:   stdin,
		stdout:  stdout,
	}
	err := cmd.Run(logging.NewNopLogger())

	// AC5.1: the returned error must signal a non-zero exit code of 3 to Kong
	// so wrappers can branch on the FATAL pipeline result without parsing
	// stderr.
	if err == nil {
		t.Fatalf("Run(...) expected error, got nil")
	}
	var ec kong.ExitCoder
	if !errors.As(err, &ec) {
		t.Fatalf("returned error does not implement kong.ExitCoder: %T: %v", err, err)
	}
	if got, want := ec.ExitCode(), 3; got != want {
		t.Errorf("ExitCode() = %d, want %d", got, want)
	}

	// AC5.1: the returned error chain must still surface the typed
	// PipelineFatalError so callers using the library directly can match it.
	// Assert the typed-error properties (Step, Message) — not the error
	// string — per the contribution guide's "Test Error Properties, not
	// Error Strings" guidance.
	var pfe *xcomposite.PipelineFatalError
	if !errors.As(err, &pfe) {
		t.Fatalf("error chain missing *PipelineFatalError: %v", err)
	}
	if pfe.Step != stepName {
		t.Errorf("PipelineFatalError.Step = %q, want %q", pfe.Step, stepName)
	}
	if pfe.Message != fatalMsg {
		t.Errorf("PipelineFatalError.Message = %q, want %q", pfe.Message, fatalMsg)
	}

	// AC5.1: stdout must contain a parseable RenderResponse with the
	// recorded RequiredResources populated.
	rsp := &renderv1alpha1.RenderResponse{}
	if err := proto.Unmarshal(stdout.Bytes(), rsp); err != nil {
		t.Fatalf("cannot unmarshal stdout RenderResponse: %v (bytes=%d)", err, stdout.Len())
	}
	composite := rsp.GetComposite()
	if composite == nil {
		t.Fatalf("RenderResponse.Composite is nil; expected partial output")
	}
	if got := len(composite.GetRequiredResources()); got != 1 {
		t.Fatalf("len(RequiredResources) = %d, want 1; rsp=%v", got, rsp)
	}
}

func TestRunWrapsNonFatalRenderErrorAsBefore(t *testing.T) {
	// AC5.2: A non-fatal render error must still be returned wrapped with
	// "cannot render composite resource" and stdout must be empty (no
	// partial response) so existing wrappers don't see ambiguous output.
	req := &renderv1alpha1.RenderRequest{
		Input: &renderv1alpha1.RenderRequest_Composite{
			Composite: &renderv1alpha1.CompositeInput{
				// Missing CompositeResource fields cause Render to fail before
				// Reconcile.
				CompositeResource: mustStruct(t, map[string]any{}),
				Composition: mustStruct(t, map[string]any{
					"metadata": map[string]any{"name": "broken"},
					"spec": map[string]any{
						"compositeTypeRef": map[string]any{
							"apiVersion": "example.org/v1alpha1",
							"kind":       "XBroken",
						},
						"pipeline": []any{},
					},
				}),
			},
		},
	}

	stdin := mustMarshalRequest(t, req)
	stdout := &bytes.Buffer{}

	cmd := &Command{
		Timeout: 30 * time.Second,
		stdin:   stdin,
		stdout:  stdout,
	}
	err := cmd.Run(logging.NewNopLogger())
	if err == nil {
		t.Fatalf("Run(...) expected error, got nil")
	}
	var pfe *xcomposite.PipelineFatalError
	if errors.As(err, &pfe) {
		t.Errorf("non-fatal error unexpectedly classified as PipelineFatalError: %v", err)
	}
	var ec kong.ExitCoder
	if errors.As(err, &ec) {
		t.Errorf("non-fatal error unexpectedly implements kong.ExitCoder with code %d", ec.ExitCode())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on non-fatal failure; got %d bytes", stdout.Len())
	}
}
