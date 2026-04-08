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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/render"
)

// ignoreConditionTimestamps filters out lastTransitionTime from condition maps
// inside unstructured objects.
var ignoreConditionTimestamps = cmpopts.IgnoreMapEntries(func(k string, _ any) bool {
	return k == "lastTransitionTime"
})

func TestRender(t *testing.T) {
	type want struct {
		err error
		out *Output
	}

	cases := map[string]struct {
		reason string
		input  *Input
		want   want
	}{
		"EmptyPipeline": {
			reason: "An Operation with an empty pipeline should reconcile successfully and be marked Complete.",
			input: &Input{
				APIVersion: APIVersion,
				Kind:       KindInput,
				Operation: opsv1alpha1.Operation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-operation",
						Namespace: "default",
					},
					Spec: opsv1alpha1.OperationSpec{
						Pipeline: []opsv1alpha1.PipelineStep{},
					},
				},
				Functions: []render.FunctionInput{},
			},
			want: want{
				out: &Output{
					APIVersion: APIVersion,
					Kind:       KindOutput,
					Operation: unstructured.Unstructured{Object: map[string]any{
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
								map[string]any{
									"type":   "Succeeded",
									"status": "True",
									"reason": "PipelineSuccess",
								},
								map[string]any{
									"type":   "ValidPipeline",
									"status": "True",
									"reason": "ValidPipeline",
								},
								map[string]any{
									"type":   "Synced",
									"status": "True",
									"reason": "ReconcileSuccess",
								},
							},
						},
					}},
					AppliedResources: []unstructured.Unstructured{},
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
			if diff := cmp.Diff(tc.want.out, out, ignoreConditionTimestamps, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
