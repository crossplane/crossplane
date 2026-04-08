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
			stripTimestamps(out.GetCompositeResource())
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
