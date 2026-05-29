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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

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
		"ModernXRWithV2XRD": {
			reason: "When a v2 XRD is supplied, the rendered XR should use the modern field paths (spec.crossplane.resourceRefs).",
			input: &renderv1alpha1.CompositeInput{
				CompositeResource: mustStruct(map[string]any{
					"apiVersion": "example.org/v1alpha1",
					"kind":       "XModernResource",
					"metadata":   map[string]any{"name": "my-xr", "namespace": "default"},
				}),
				Composition: mustStruct(map[string]any{
					"metadata": map[string]any{"name": "modern-composition"},
					"spec": map[string]any{
						"compositeTypeRef": map[string]any{
							"apiVersion": "example.org/v1alpha1",
							"kind":       "XModernResource",
						},
						"pipeline": []any{},
					},
				}),
				CompositeResourceDefinitions: []*structpb.Struct{
					mustStruct(map[string]any{
						"apiVersion": "apiextensions.crossplane.io/v2",
						"kind":       "CompositeResourceDefinition",
						"metadata":   map[string]any{"name": "xmodernresources.example.org"},
						"spec": map[string]any{
							"group": "example.org",
							"names": map[string]any{
								"kind":   "XModernResource",
								"plural": "xmodernresources",
							},
							"versions": []any{
								map[string]any{
									"name":          "v1alpha1",
									"served":        true,
									"referenceable": true,
								},
							},
						},
					}),
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XModernResource",
						"metadata":   map[string]any{"name": "my-xr", "namespace": "default"},
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
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: modern-composition"},
					},
				},
			},
		},
		"LegacyXRWithV1XRDDefaultScope": {
			reason: "When a v1 XRD with default (LegacyCluster) scope is supplied, the rendered XR should use the legacy field paths (spec.resourceRefs), not the modern paths (spec.crossplane.resourceRefs).",
			input: &renderv1alpha1.CompositeInput{
				CompositeResource: mustStruct(map[string]any{
					"apiVersion": "example.org/v1alpha1",
					"kind":       "XLegacyResource",
					"metadata":   map[string]any{"name": "my-xr"},
				}),
				Composition: mustStruct(map[string]any{
					"metadata": map[string]any{"name": "legacy-composition"},
					"spec": map[string]any{
						"compositeTypeRef": map[string]any{
							"apiVersion": "example.org/v1alpha1",
							"kind":       "XLegacyResource",
						},
						"pipeline": []any{},
					},
				}),
				CompositeResourceDefinitions: []*structpb.Struct{
					mustStruct(map[string]any{
						"apiVersion": "apiextensions.crossplane.io/v1",
						"kind":       "CompositeResourceDefinition",
						"metadata":   map[string]any{"name": "xlegacyresources.example.org"},
						"spec": map[string]any{
							"group": "example.org",
							"names": map[string]any{
								"kind":   "XLegacyResource",
								"plural": "xlegacyresources",
							},
							"versions": []any{
								map[string]any{
									"name":          "v1alpha1",
									"served":        true,
									"referenceable": true,
								},
							},
						},
					}),
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XLegacyResource",
						"metadata":   map[string]any{"name": "my-xr"},
						"spec": map[string]any{
							"resourceRefs": []any{},
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
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: legacy-composition"},
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

func TestSelectSchema(t *testing.T) {
	xrGVK := schema.GroupVersionKind{Group: "example.org", Version: "v1alpha1", Kind: "XLegacyResource"}

	v1XRD := func(name, group, kind string, scope *string) *structpb.Struct {
		spec := map[string]any{
			"group": group,
			"names": map[string]any{
				"kind":   kind,
				"plural": strings.ToLower(kind) + "s",
			},
			"versions": []any{
				map[string]any{"name": "v1alpha1", "served": true, "referenceable": true},
			},
		}
		if scope != nil {
			spec["scope"] = *scope
		}
		return mustStruct(map[string]any{
			"apiVersion": "apiextensions.crossplane.io/v1",
			"kind":       "CompositeResourceDefinition",
			"metadata":   map[string]any{"name": name},
			"spec":       spec,
		})
	}
	v2XRD := func(name, group, kind string, scope *string) *structpb.Struct {
		spec := map[string]any{
			"group": group,
			"names": map[string]any{
				"kind":   kind,
				"plural": strings.ToLower(kind) + "s",
			},
			"versions": []any{
				map[string]any{"name": "v1alpha1", "served": true, "referenceable": true},
			},
		}
		if scope != nil {
			spec["scope"] = *scope
		}
		return mustStruct(map[string]any{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata":   map[string]any{"name": name},
			"spec":       spec,
		})
	}

	cases := map[string]struct {
		reason          string
		gvk             schema.GroupVersionKind
		defs            []*structpb.Struct
		wantSchema      ucomposite.Schema
		wantErr         bool
		wantErrContains []string
	}{
		"NoXRDsBackCompat": {
			reason:     "No XRDs supplied preserves the historical default of SchemaModern.",
			gvk:        xrGVK,
			defs:       nil,
			wantSchema: ucomposite.SchemaModern,
		},
		"V1XRDDefaultScope": {
			reason:     "A v1 XRD with default (LegacyCluster) scope yields SchemaLegacy.",
			gvk:        xrGVK,
			defs:       []*structpb.Struct{v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", nil)},
			wantSchema: ucomposite.SchemaLegacy,
		},
		"V1XRDExplicitLegacyClusterScope": {
			reason:     "A v1 XRD with explicit Spec.Scope=LegacyCluster yields SchemaLegacy.",
			gvk:        xrGVK,
			defs:       []*structpb.Struct{v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", strPtr("LegacyCluster"))},
			wantSchema: ucomposite.SchemaLegacy,
		},
		"V1XRDExplicitClusterScope": {
			reason:     "A v1 XRD with explicit Spec.Scope=Cluster (theoretical edge case) yields SchemaModern. Mirrors the production reconciler's rule.",
			gvk:        xrGVK,
			defs:       []*structpb.Struct{v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", strPtr("Cluster"))},
			wantSchema: ucomposite.SchemaModern,
		},
		"V2XRD": {
			reason:     "A v2 XRD with no scope (or any non-LegacyCluster scope) yields SchemaModern.",
			gvk:        xrGVK,
			defs:       []*structpb.Struct{v2XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", nil)},
			wantSchema: ucomposite.SchemaModern,
		},
		"V2FormPreservingLegacyClusterScope": {
			reason: "A v2-form XRD with Spec.Scope=LegacyCluster (e.g. a v1-posted XRD round-tripped through the storage version) yields SchemaLegacy. The v2 Go type's Spec.Scope is a string alias with no runtime enum validation, so 'LegacyCluster' survives the round-trip and must be honored to avoid forcing consumers to specifically fetch v1-form.",
			gvk:        xrGVK,
			defs:       []*structpb.Struct{v2XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", strPtr("LegacyCluster"))},
			wantSchema: ucomposite.SchemaLegacy,
		},
		"NoMatchingXRD": {
			reason: "When no XRD's composite GVK matches the input XR, return a clear error mentioning the XR GVK.",
			gvk:    xrGVK,
			defs: []*structpb.Struct{
				v1XRD("xother.example.org", "example.org", "XOther", nil),
			},
			wantErr:         true,
			wantErrContains: []string{"no CompositeResourceDefinition matches", "XLegacyResource"},
		},
		"MultipleMatchingXRDs": {
			reason: "When multiple XRDs match the input XR's GVK, return a clear error mentioning the conflicting names.",
			gvk:    xrGVK,
			defs: []*structpb.Struct{
				v1XRD("first.example.org", "example.org", "XLegacyResource", nil),
				v2XRD("second.example.org", "example.org", "XLegacyResource", nil),
			},
			wantErr:         true,
			wantErrContains: []string{"multiple CompositeResourceDefinitions match", "first.example.org", "second.example.org"},
		},
		"UnrecognizedXRDAPIVersion": {
			reason: "An XRD with an apiVersion that isn't apiextensions.crossplane.io/{v1,v2} returns a clear error.",
			gvk:    xrGVK,
			defs: []*structpb.Struct{
				mustStruct(map[string]any{
					"apiVersion": "apiextensions.crossplane.io/v3",
					"kind":       "CompositeResourceDefinition",
					"metadata":   map[string]any{"name": "weird.example.org"},
					"spec":       map[string]any{},
				}),
			},
			wantErr:         true,
			wantErrContains: []string{"unrecognized apiVersion", "apiextensions.crossplane.io/v3"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := selectSchema(tc.gvk, tc.defs)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("\n%s\nselectSchema(...): expected error, got nil", tc.reason)
				}
				for _, sub := range tc.wantErrContains {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("\n%s\nselectSchema(...): error %q does not contain %q", tc.reason, err.Error(), sub)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("\n%s\nselectSchema(...): unexpected error: %v", tc.reason, err)
			}
			if got != tc.wantSchema {
				t.Errorf("\n%s\nselectSchema(...): want %v, got %v", tc.reason, tc.wantSchema, got)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func mustStruct(m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}
