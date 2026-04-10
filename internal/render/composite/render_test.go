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

package composite

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

// ignoreTimestamps ignores any map entry keyed "lastTransitionTime" at any
// nesting depth. This works with protocmp.Transform() because it matches on
// the path structure, not value types.
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
		out *renderv1alpha1.CompositeOutput
	}

	cases := map[string]struct {
		reason string
		input  *renderv1alpha1.CompositeInput
		want   want
	}{
		"EmptyPipeline": {
			reason: "An XR with an empty pipeline should reconcile successfully and set Ready/Synced conditions.",
			input: &renderv1alpha1.CompositeInput{
				CompositeResource: mustStruct(map[string]any{
					"apiVersion": "example.org/v1alpha1",
					"kind":       "XBucket",
					"metadata":   map[string]any{"name": "my-bucket"},
				}),
				Composition: mustStruct(map[string]any{
					"metadata": map[string]any{"name": "bucket-composition"},
					"spec": map[string]any{
						"compositeTypeRef": map[string]any{
							"apiVersion": "example.org/v1alpha1",
							"kind":       "XBucket",
						},
						"pipeline": []any{},
					},
				}),
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XBucket",
						"metadata":   map[string]any{"name": "my-bucket"},
						"spec": map[string]any{
							"crossplane": map[string]any{
								"resourceRefs": []any{},
							},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
								map[string]any{"type": "Ready", "status": "True", "reason": "Available"},
							},
						},
					}),
					Events: []*renderv1alpha1.Event{
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: bucket-composition"},
					},
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
