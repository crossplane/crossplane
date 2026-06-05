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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/render/rendertest"
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
			out, err := Render(t.Context(), logging.NewNopLogger(), tc.input)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.out, out, cmpopts.EquateEmpty(), protocmp.Transform(), ignoreTimestamps); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRenderErrors(t *testing.T) {
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
		pipelineFatal           bool
		fatalStep, fatalMessage string
		hasOutput               bool
		requiredResources       int
		wantSelector            *fnv1.ResourceSelector
	}

	cases := map[string]struct {
		reason string
		input  func(t *testing.T) *renderv1alpha1.OperationInput
		want   want
	}{
		"PipelineFatalReturnsRequirements": {
			reason: "When a pipeline step returns SEVERITY_FATAL, Render must return the partial OperationOutput (with recorded RequiredResources) and an error chain containing *PipelineFatalError reachable via errors.As.",
			input: func(t *testing.T) *renderv1alpha1.OperationInput {
				t.Helper()
				addr := rendertest.StartFunctionServer(t, &rendertest.FatalFunctionServer{
					RequirementName: requirementName,
					Selector:        wantSelector,
					FatalMessage:    fatalMsg,
				})
				return &renderv1alpha1.OperationInput{
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
			},
			want: want{
				pipelineFatal:     true,
				fatalStep:         stepName,
				fatalMessage:      fatalMsg,
				hasOutput:         true,
				requiredResources: 1,
				wantSelector:      wantSelector,
			},
		},
		"NonFatalReconcileErrorWraps": {
			reason: "A pipeline step whose function name is not registered in FunctionInput causes the reconciler to fail with an 'unknown function' error — a non-fatal failure mode. The error must NOT be a *PipelineFatalError, and no partial output must be returned.",
			input: func(t *testing.T) *renderv1alpha1.OperationInput {
				t.Helper()
				return &renderv1alpha1.OperationInput{
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
					// Functions list intentionally empty.
				}
			},
			want: want{
				pipelineFatal: false,
				hasOutput:     false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := Render(t.Context(), logging.NewNopLogger(), tc.input(t))

			if err == nil {
				t.Fatalf("%s\nRender(...) expected error, got nil", tc.reason)
			}

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

			if !tc.want.hasOutput {
				if out != nil {
					t.Errorf("%s\nRender(...) returned out=%v on non-fatal error; want nil", tc.reason, out)
				}
				return
			}
			if out == nil {
				t.Fatalf("%s\nRender(...) returned nil output; want non-nil with RequiredResources populated", tc.reason)
			}
			if got := len(out.GetRequiredResources()); got != tc.want.requiredResources {
				t.Fatalf("%s\nlen(out.RequiredResources) = %d, want %d; out=%v", tc.reason, got, tc.want.requiredResources, out)
			}
			if tc.want.requiredResources > 0 && tc.want.wantSelector != nil {
				gotSelector := &fnv1.ResourceSelector{}
				bs, err := out.GetRequiredResources()[0].MarshalJSON()
				if err != nil {
					t.Fatalf("%s\ncannot marshal recorded selector to JSON: %v", tc.reason, err)
				}
				if err := protojson.Unmarshal(bs, gotSelector); err != nil {
					t.Fatalf("%s\ncannot decode recorded ResourceSelector: %v", tc.reason, err)
				}
				if diff := cmp.Diff(tc.want.wantSelector, gotSelector, protocmp.Transform()); diff != "" {
					t.Errorf("%s\nrecorded ResourceSelector: -want, +got:\n%s", tc.reason, diff)
				}
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
