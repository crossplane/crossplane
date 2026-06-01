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

package operation

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// ignoreTimestamps ignores any map entry keyed "lastTransitionTime" at any
// nesting depth.
var ignoreTimestamps = cmp.FilterPath(func(p cmp.Path) bool {
	for _, s := range p {
		mi, ok := s.(cmp.MapIndex)
		if !ok {
			continue
		}
		k, ok := mi.Key().Interface().(string)
		if ok && k == "lastTransitionTime" {
			return true
		}
	}
	return false
}, cmp.Ignore())

func TestRender(t *testing.T) {
	type want struct {
		err error
		out *renderv1alpha1.OperationOutput
	}

	cases := map[string]struct {
		reason string
		input  *renderv1alpha1.OperationInput
		want   want
	}{
		"EmptyPipeline": {
			reason: "An Operation with an empty pipeline should reconcile successfully and be marked Complete.",
			input: &renderv1alpha1.OperationInput{
				Operation: mustStruct(map[string]any{
					"apiVersion": "ops.crossplane.io/v1alpha1",
					"kind":       "Operation",
					"metadata": map[string]any{
						"name":      "my-operation",
						"namespace": "default",
					},
					"spec": map[string]any{
						"mode":     "",
						"pipeline": []any{},
					},
				}),
			},
			want: want{
				out: &renderv1alpha1.OperationOutput{
					Operation: mustStruct(map[string]any{
						"apiVersion": "ops.crossplane.io/v1alpha1",
						"kind":       "Operation",
						"metadata": map[string]any{
							"name":            "my-operation",
							"namespace":       "default",
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"mode":     "",
							"pipeline": []any{},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Succeeded", "status": "True", "reason": "PipelineSuccess"},
								map[string]any{"type": "ValidPipeline", "status": "True", "reason": "ValidPipeline"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
						},
					}),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := Render(context.Background(), logging.NewNopLogger(), tc.input)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.out, out, cmpopts.EquateEmpty(), protocmp.Transform(), ignoreTimestamps); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func mustStruct(m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}

// fatalFunctionServer simulates a real function-extra-resources style pipeline
// step. On its first call it announces its required resources via
// Requirements.Resources (no FATAL), letting the FetchingFunctionRunner record
// the selectors via the recording fetcher. On its second call (when the
// fetched resources came back empty because the caller did not pre-populate
// them) it returns SEVERITY_FATAL.
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

	if call == 1 {
		return &fnv1.RunFunctionResponse{
			Requirements: &fnv1.Requirements{
				Resources: map[string]*fnv1.ResourceSelector{
					s.requirementName: s.selector,
				},
			},
		}, nil
	}

	return &fnv1.RunFunctionResponse{
		Requirements: &fnv1.Requirements{
			Resources: map[string]*fnv1.ResourceSelector{
				s.requirementName: s.selector,
			},
		},
		Results: []*fnv1.Result{
			{Severity: fnv1.Severity_SEVERITY_FATAL, Message: s.fatalMessage},
		},
	}, nil
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

	t.Cleanup(func() {
		s.Stop()
	})

	return lis.Addr().String()
}

func TestRenderPipelineFatalReturnsRequirements(t *testing.T) {
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

	server := &fatalFunctionServer{
		requirementName: requirementName,
		selector:        wantSelector,
		fatalMessage:    fatalMsg,
	}

	addr := startTestFunctionServer(t, server)

	in := &renderv1alpha1.OperationInput{
		Operation: mustStruct(map[string]any{
			"apiVersion": "ops.crossplane.io/v1alpha1",
			"kind":       "Operation",
			"metadata": map[string]any{
				"name":      "my-operation",
				"namespace": "default",
			},
			"spec": map[string]any{
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
	}

	out, err := Render(context.Background(), logging.NewNopLogger(), in)

	var pfe *xcomposite.PipelineFatalError
	if !errors.As(err, &pfe) {
		t.Fatalf("Render(...) error: want *PipelineFatalError in chain, got %T: %v", err, err)
	}
	if pfe.Step != stepName {
		t.Errorf("PipelineFatalError.Step = %q, want %q", pfe.Step, stepName)
	}
	if pfe.Message != fatalMsg {
		t.Errorf("PipelineFatalError.Message = %q, want %q", pfe.Message, fatalMsg)
	}

	if out == nil {
		t.Fatalf("Render(...) returned nil output on PipelineFatalError; want non-nil with RequiredResources populated")
	}
	if got := len(out.GetRequiredResources()); got != 1 {
		t.Fatalf("len(out.RequiredResources) = %d, want 1; out=%v", got, out)
	}

	gotSelector := &fnv1.ResourceSelector{}
	bs, err := out.GetRequiredResources()[0].MarshalJSON()
	if err != nil {
		t.Fatalf("cannot marshal recorded selector to JSON: %v", err)
	}
	if err := protojson.Unmarshal(bs, gotSelector); err != nil {
		t.Fatalf("cannot decode recorded ResourceSelector: %v", err)
	}
	if diff := cmp.Diff(wantSelector, gotSelector, protocmp.Transform()); diff != "" {
		t.Errorf("recorded ResourceSelector: -want, +got:\n%s", diff)
	}
}

func TestRenderNonFatalReconcileErrorWraps(t *testing.T) {
	// A pipeline step whose function name is not registered in
	// FunctionInput causes the reconciler to fail with an "unknown function"
	// error — a non-fatal failure mode. The returned error must NOT be a
	// *PipelineFatalError, and no partial output must be returned. This
	// guards the non-fatal path against regressions that would accidentally
	// surface partial output for any reconcile failure.
	in := &renderv1alpha1.OperationInput{
		Operation: mustStruct(map[string]any{
			"apiVersion": "ops.crossplane.io/v1alpha1",
			"kind":       "Operation",
			"metadata": map[string]any{
				"name":      "my-operation",
				"namespace": "default",
			},
			"spec": map[string]any{
				"mode": "Pipeline",
				"pipeline": []any{
					map[string]any{
						"step":        "missing",
						"functionRef": map[string]any{"name": "function-does-not-exist"},
					},
				},
			},
		}),
		// Functions list intentionally empty so the runner has no
		// connection for "function-does-not-exist".
	}

	out, err := Render(context.Background(), logging.NewNopLogger(), in)
	if err == nil {
		t.Fatalf("Render(...) expected error, got nil; out=%v", out)
	}
	var pfe *PipelineFatalError
	if errors.As(err, &pfe) {
		t.Errorf("Render(...) error unexpectedly classified as *PipelineFatalError: %v", err)
	}
	if out != nil {
		t.Errorf("Render(...) returned out=%v on non-fatal error; want nil", out)
	}
}
