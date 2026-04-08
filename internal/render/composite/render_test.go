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
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
)

var (
	_ composite.FunctionRunner = &FunctionRunner{}
	_ event.Recorder           = &EventRecorder{}
)

// ignoreConditionTimestamps filters out lastTransitionTime from condition maps
// inside unstructured objects. Timestamps are non-deterministic and should not
// be compared.
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
			reason: "An XR with an empty pipeline should reconcile successfully, set conditions, and produce no composed resources.",
			input: &Input{
				APIVersion: APIVersion,
				Kind:       KindInput,
				CompositeResource: unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "example.org/v1alpha1",
					"kind":       "XBucket",
					"metadata":   map[string]any{"name": "my-bucket"},
				}},
				Composition: apiextensionsv1.Composition{
					ObjectMeta: metav1.ObjectMeta{Name: "bucket-composition"},
					Spec: apiextensionsv1.CompositionSpec{
						CompositeTypeRef: apiextensionsv1.TypeReference{
							APIVersion: "example.org/v1alpha1",
							Kind:       "XBucket",
						},
						Pipeline: []apiextensionsv1.PipelineStep{},
					},
				},
				Functions: []FunctionInput{},
			},
			want: want{
				out: &Output{
					APIVersion: APIVersion,
					Kind:       KindOutput,
					CompositeResource: unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XBucket",
						"metadata": map[string]any{
							"name": "my-bucket",
						},
						"spec": map[string]any{
							"crossplane": map[string]any{
								"resourceRefs": []any{},
							},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{
									"type":   "Responsive",
									"status": "True",
									"reason": "WatchCircuitClosed",
								},
								map[string]any{
									"type":   "Synced",
									"status": "True",
									"reason": "ReconcileSuccess",
								},
								map[string]any{
									"type":   "Ready",
									"status": "True",
									"reason": "Available",
								},
							},
						},
					}},
					ComposedResources: []unstructured.Unstructured{},
					Events: []OutputEvent{
						{
							Type:    "Normal",
							Reason:  "SelectComposition",
							Message: "Successfully selected composition: bucket-composition",
						},
					},
				},
			},
		},
		"EmptyPipelineGarbageCollectsObservedResources": {
			reason: "An XR with observed resources and an empty pipeline should garbage collect the undesired observed resources.",
			input: &Input{
				APIVersion: APIVersion,
				Kind:       KindInput,
				CompositeResource: unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "example.org/v1alpha1",
					"kind":       "XBucket",
					"metadata":   map[string]any{"name": "my-bucket"},
				}},
				Composition: apiextensionsv1.Composition{
					ObjectMeta: metav1.ObjectMeta{Name: "bucket-composition"},
					Spec: apiextensionsv1.CompositionSpec{
						CompositeTypeRef: apiextensionsv1.TypeReference{
							APIVersion: "example.org/v1alpha1",
							Kind:       "XBucket",
						},
						Pipeline: []apiextensionsv1.PipelineStep{},
					},
				},
				Functions: []FunctionInput{},
				ObservedResources: []unstructured.Unstructured{{Object: map[string]any{
					"apiVersion": "s3.aws.upbound.io/v1beta1",
					"kind":       "Bucket",
					"metadata": map[string]any{
						"name": "my-bucket-abcde",
						"annotations": map[string]any{
							"crossplane.io/composition-resource-name": "bucket",
						},
					},
				}}},
			},
			want: want{
				out: &Output{
					APIVersion: APIVersion,
					Kind:       KindOutput,
					CompositeResource: unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XBucket",
						"metadata": map[string]any{
							"name": "my-bucket",
						},
						"spec": map[string]any{
							"crossplane": map[string]any{
								"resourceRefs": []any{},
							},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{
									"type":   "Responsive",
									"status": "True",
									"reason": "WatchCircuitClosed",
								},
								map[string]any{
									"type":   "Synced",
									"status": "True",
									"reason": "ReconcileSuccess",
								},
								map[string]any{
									"type":   "Ready",
									"status": "True",
									"reason": "Available",
								},
							},
						},
					}},
					ComposedResources: []unstructured.Unstructured{},
					DeletedResources: []unstructured.Unstructured{{Object: map[string]any{
						"apiVersion": "s3.aws.upbound.io/v1beta1",
						"kind":       "Bucket",
						"metadata": map[string]any{
							"name": "my-bucket-abcde",
							"annotations": map[string]any{
								"crossplane.io/composition-resource-name": "bucket",
							},
						},
					}}},
					Events: []OutputEvent{
						{
							Type:    "Normal",
							Reason:  "SelectComposition",
							Message: "Successfully selected composition: bucket-composition",
						},
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
			if diff := cmp.Diff(tc.want.out, out, ignoreConditionTimestamps, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInputUnmarshal(t *testing.T) {
	type want struct {
		err error
		in  *Input
	}

	cases := map[string]struct {
		reason string
		json   string
		want   want
	}{
		"ValidMinimalInput": {
			reason: "A minimal valid Input should unmarshal correctly with all fields preserved.",
			json: `{
				"apiVersion": "render.crossplane.io/v1alpha1",
				"kind": "Input",
				"compositeResource": {
					"apiVersion": "example.org/v1alpha1",
					"kind": "XBucket",
					"metadata": {"name": "my-bucket"}
				},
				"composition": {
					"metadata": {"name": "bucket-comp"},
					"spec": {
						"compositeTypeRef": {
							"apiVersion": "example.org/v1alpha1",
							"kind": "XBucket"
						},
						"pipeline": []
					}
				},
				"functions": []
			}`,
			want: want{
				in: &Input{
					APIVersion: "render.crossplane.io/v1alpha1",
					Kind:       "Input",
					CompositeResource: unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XBucket",
						"metadata":   map[string]any{"name": "my-bucket"},
					}},
					Composition: apiextensionsv1.Composition{
						ObjectMeta: metav1.ObjectMeta{Name: "bucket-comp"},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v1alpha1",
								Kind:       "XBucket",
							},
							Pipeline: []apiextensionsv1.PipelineStep{},
						},
					},
					Functions: []FunctionInput{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := &Input{}
			err := json.Unmarshal([]byte(tc.json), got)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\njson.Unmarshal(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.in, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\njson.Unmarshal(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
