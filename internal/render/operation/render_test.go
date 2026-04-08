/*
Copyright 2025 The Crossplane Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

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
			stripTimestamps(out.GetOperation())
			if diff := cmp.Diff(tc.want.out, out, cmpopts.EquateEmpty(), protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// stripTimestamps recursively removes lastTransitionTime from a protobuf
// Struct. Timestamps are non-deterministic and should not be compared.
func stripTimestamps(s *structpb.Struct) {
	if s == nil {
		return
	}
	delete(s.GetFields(), "lastTransitionTime")
	for _, v := range s.GetFields() {
		if sv := v.GetStructValue(); sv != nil {
			stripTimestamps(sv)
		}
		if lv := v.GetListValue(); lv != nil {
			for _, item := range lv.GetValues() {
				if sv := item.GetStructValue(); sv != nil {
					stripTimestamps(sv)
				}
			}
		}
	}
}

func mustStruct(m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}
