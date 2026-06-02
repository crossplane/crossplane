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
	"testing"

	"github.com/alecthomas/kong"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/render/rendertest"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

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

func TestRun(t *testing.T) {
	// Constants for the FATAL case shared between request construction and
	// expected-result assertions.
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

	type want struct {
		// pipelineFatal asserts whether *xcomposite.PipelineFatalError is
		// expected in the returned error's chain (errors.As).
		pipelineFatal bool
		// fatalStep / fatalMessage are checked only when pipelineFatal is
		// true.
		fatalStep, fatalMessage string
		// exitCode is the expected kong.ExitCoder code; zero means the
		// returned error must NOT implement kong.ExitCoder.
		exitCode int
		// compositeOutput indicates we expect stdout to contain a parseable
		// RenderResponse with Composite set; otherwise stdout must be empty.
		compositeOutput bool
		// requiredResources is the expected
		// CompositeOutput.RequiredResources count when compositeOutput is
		// true.
		requiredResources int
	}

	cases := map[string]struct {
		reason string
		// req returns the RenderRequest. It's a closure so each test case
		// can stand up its own gRPC fixture and bake the address into the
		// request.
		req  func(t *testing.T) *renderv1alpha1.RenderRequest
		want want
	}{
		"PipelineFatalReturnsPartialOutputWithExitCode3": {
			reason: "When a pipeline step returns SEVERITY_FATAL, Run must marshal the partial RenderResponse (with recorded RequiredResources) to stdout and return an error with kong.ExitCoder code 3 and *PipelineFatalError reachable via errors.As.",
			req: func(t *testing.T) *renderv1alpha1.RenderRequest {
				t.Helper()
				addr := rendertest.StartFunctionServer(t, &rendertest.FatalFunctionServer{
					RequirementName: requirementName,
					Selector:        wantSelector,
					FatalMessage:    fatalMsg,
				})
				return &renderv1alpha1.RenderRequest{
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
			},
			want: want{
				pipelineFatal:     true,
				fatalStep:         stepName,
				fatalMessage:      fatalMsg,
				exitCode:          ExitCodePipelineFatal,
				compositeOutput:   true,
				requiredResources: 1,
			},
		},
		"NonFatalRenderErrorWrapsAsBefore": {
			reason: "A non-fatal render error must be returned wrapped with the existing 'cannot render composite resource' context; stdout must be empty and the error must NOT implement kong.ExitCoder.",
			req: func(t *testing.T) *renderv1alpha1.RenderRequest {
				t.Helper()
				return &renderv1alpha1.RenderRequest{
					Input: &renderv1alpha1.RenderRequest_Composite{
						Composite: &renderv1alpha1.CompositeInput{
							// Missing CompositeResource fields cause Render
							// to fail before Reconcile.
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
			},
			want: want{
				pipelineFatal: false,
				exitCode:      0,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stdin := mustMarshalRequest(t, tc.req(t))
			stdout := &bytes.Buffer{}

			// Minimal Kong tree mirroring how the render subcommand is
			// mounted in main.go's cli struct. We pre-set the unexported
			// stdin/stdout fields; Kong's reflection ignores untagged
			// unexported fields and doesn't clobber them, so the values
			// survive parsing.
			var cli struct {
				Render Command `cmd:""`
			}
			cli.Render.stdin = stdin
			cli.Render.stdout = stdout

			// Bind the logging.Logger interface the same way main.go does
			// (kong.BindTo with the interface pointer); without this Kong
			// can't satisfy Run's logging.Logger parameter.
			parser, err := kong.New(&cli,
				kong.BindTo(logging.NewNopLogger(), (*logging.Logger)(nil)),
			)
			if err != nil {
				t.Fatalf("%s\nkong.New: %v", tc.reason, err)
			}
			ktx, err := parser.Parse([]string{"render", "--timeout=30s"})
			if err != nil {
				t.Fatalf("%s\nkong.Parse: %v", tc.reason, err)
			}
			err = ktx.Run()

			if err == nil {
				t.Fatalf("%s\nRun(...) expected error, got nil", tc.reason)
			}

			// PipelineFatalError property check (errors.As + Step/Message),
			// per the contribution guide's "Test Error Properties, not
			// Error Strings" guidance.
			var pfe *xcomposite.PipelineFatalError
			gotFatal := errors.As(err, &pfe)
			if gotFatal != tc.want.pipelineFatal {
				t.Errorf("%s\nerrors.As(*PipelineFatalError) = %v, want %v; err=%v", tc.reason, gotFatal, tc.want.pipelineFatal, err)
			}
			if tc.want.pipelineFatal && gotFatal {
				if pfe.Step != tc.want.fatalStep {
					t.Errorf("%s\nPipelineFatalError.Step = %q, want %q", tc.reason, pfe.Step, tc.want.fatalStep)
				}
				if pfe.Message != tc.want.fatalMessage {
					t.Errorf("%s\nPipelineFatalError.Message = %q, want %q", tc.reason, pfe.Message, tc.want.fatalMessage)
				}
			}

			// Kong exit-code property check.
			var ec kong.ExitCoder
			gotEC := errors.As(err, &ec)
			wantEC := tc.want.exitCode != 0
			if gotEC != wantEC {
				t.Errorf("%s\nerrors.As(kong.ExitCoder) = %v, want %v; err=%v", tc.reason, gotEC, wantEC, err)
			}
			if wantEC && gotEC {
				if got := ec.ExitCode(); got != tc.want.exitCode {
					t.Errorf("%s\nExitCode() = %d, want %d", tc.reason, got, tc.want.exitCode)
				}
			}

			// stdout shape.
			if !tc.want.compositeOutput {
				if stdout.Len() != 0 {
					t.Errorf("%s\nstdout should be empty; got %d bytes", tc.reason, stdout.Len())
				}
				return
			}

			rsp := &renderv1alpha1.RenderResponse{}
			if err := proto.Unmarshal(stdout.Bytes(), rsp); err != nil {
				t.Fatalf("%s\ncannot unmarshal stdout RenderResponse: %v (bytes=%d)", tc.reason, err, stdout.Len())
			}
			composite := rsp.GetComposite()
			if composite == nil {
				t.Fatalf("%s\nRenderResponse.Composite is nil; expected partial output", tc.reason)
			}
			if got := len(composite.GetRequiredResources()); got != tc.want.requiredResources {
				t.Errorf("%s\nlen(RequiredResources) = %d, want %d; rsp=%v", tc.reason, got, tc.want.requiredResources, rsp)
			}
		})
	}
}
