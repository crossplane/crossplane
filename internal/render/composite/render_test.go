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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/features"
	"github.com/crossplane/crossplane/v2/internal/render/rendertest"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
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
	// Steps may reference function packages by digest. Rendering doesn't
	// install functions, so the function must be supplied using its package as
	// its name.
	const pkg = "xpkg.crossplane.io/crossplane-contrib/function-cool@sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	addr := rendertest.StartFunctionServer(t, &rendertest.StaticFunctionServer{
		Response: &fnv1.RunFunctionResponse{
			Results: []*fnv1.Result{{Severity: fnv1.Severity_SEVERITY_NORMAL, Message: "cool function ran"}},
		},
	})

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
				CompositeResourceDefinition: mustStruct(map[string]any{
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
				CompositeResourceDefinition: mustStruct(map[string]any{
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
		"LegacyXRWithV1XRDExplicitLegacyClusterScope": {
			reason: "When a v1 XRD with explicit Spec.Scope=LegacyCluster is supplied, the rendered XR should use the legacy field paths. Mirrors the LegacyXRWithV1XRDDefaultScope case end-to-end through Render() to catch any pointer-default handling regression.",
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
				CompositeResourceDefinition: mustStruct(map[string]any{
					"apiVersion": "apiextensions.crossplane.io/v1",
					"kind":       "CompositeResourceDefinition",
					"metadata":   map[string]any{"name": "xlegacyresources.example.org"},
					"spec": map[string]any{
						"group": "example.org",
						"names": map[string]any{
							"kind":   "XLegacyResource",
							"plural": "xlegacyresources",
						},
						"scope": "LegacyCluster",
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
		"LegacyXRWithObservedResources": {
			reason: "When a v1 XRD is supplied and observed resources are passed, both InjectResourceRefs (pre-reconcile) and the reconciler must agree on the legacy field path. With an empty pipeline the observed resource is garbage-collected — and that GC only happens if the reconciler successfully READ the ref InjectResourceRefs wrote. If Schema were modern at either site, the paths would diverge and deleted_resources would be empty.",
			input: &renderv1alpha1.CompositeInput{
				CompositeResource: mustStruct(map[string]any{
					"apiVersion": "example.org/v1alpha1",
					"kind":       "XLegacyResource",
					"metadata":   map[string]any{"name": "my-xr", "uid": "xr-uid"},
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
				ObservedResources: []*structpb.Struct{
					mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XComposed",
						"metadata": map[string]any{
							"name": "obs-1",
							"annotations": map[string]any{
								"crossplane.io/composition-resource-name": "obs-1",
							},
							"ownerReferences": []any{
								map[string]any{
									"apiVersion":         "example.org/v1alpha1",
									"kind":               "XLegacyResource",
									"name":               "my-xr",
									"uid":                "xr-uid",
									"controller":         true,
									"blockOwnerDeletion": true,
								},
							},
						},
					}),
				},
				CompositeResourceDefinition: mustStruct(map[string]any{
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
					DeletedResources: []*structpb.Struct{
						mustStruct(map[string]any{
							"apiVersion": "example.org/v1alpha1",
							"kind":       "XComposed",
							"metadata": map[string]any{
								"name": "obs-1",
								"annotations": map[string]any{
									"crossplane.io/composition-resource-name": "obs-1",
								},
								"ownerReferences": []any{
									map[string]any{
										"apiVersion":         "example.org/v1alpha1",
										"kind":               "XLegacyResource",
										"name":               "my-xr",
										"uid":                "xr-uid",
										"controller":         true,
										"blockOwnerDeletion": true,
									},
								},
							},
						}),
					},
					Events: []*renderv1alpha1.Event{
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: legacy-composition"},
					},
				},
			},
		},
		"FunctionByPackage": {
			reason: "A pipeline step that references its function by package OCI reference should be routed to the function supplied with that package as its name.",
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
						"mode": "Pipeline",
						"pipeline": []any{
							map[string]any{
								"step":     "cool",
								"function": pkg,
							},
						},
					},
				}),
				Functions: []*renderv1alpha1.FunctionInput{
					{Name: pkg, Address: addr},
				},
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
					// Our function's result surfaces as an event. Seeing it
					// means we routed to the function by package.
					Events: []*renderv1alpha1.Event{
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: bucket-composition"},
						{Type: "Normal", Reason: "ComposeResources", Message: "Pipeline step \"cool\": cool function ran"},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := Render(t.Context(), logging.NewNopLogger(), &feature.Flags{}, tc.input)

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
		def             *structpb.Struct
		wantSchema      ucomposite.Schema
		wantErr         bool
		wantErrContains []string
	}{
		"NoXRDBackCompat": {
			reason:     "No XRD supplied preserves the historical default of SchemaModern.",
			gvk:        xrGVK,
			def:        nil,
			wantSchema: ucomposite.SchemaModern,
		},
		"V1XRDDefaultScope": {
			reason:     "A v1 XRD with default (LegacyCluster) scope yields SchemaLegacy.",
			gvk:        xrGVK,
			def:        v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", nil),
			wantSchema: ucomposite.SchemaLegacy,
		},
		"V1XRDExplicitLegacyClusterScope": {
			reason:     "A v1 XRD with explicit Spec.Scope=LegacyCluster yields SchemaLegacy.",
			gvk:        xrGVK,
			def:        v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", new("LegacyCluster")),
			wantSchema: ucomposite.SchemaLegacy,
		},
		"V1XRDExplicitClusterScope": {
			reason:     "A v1 XRD with explicit Spec.Scope=Cluster (theoretical edge case) yields SchemaModern. Mirrors the production reconciler's rule.",
			gvk:        xrGVK,
			def:        v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", new("Cluster")),
			wantSchema: ucomposite.SchemaModern,
		},
		"V2XRD": {
			reason:     "A v2 XRD with no scope (or any non-LegacyCluster scope) yields SchemaModern.",
			gvk:        xrGVK,
			def:        v2XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", nil),
			wantSchema: ucomposite.SchemaModern,
		},
		"V2FormPreservingLegacyClusterScope": {
			reason:     "A v2-form XRD with Spec.Scope=LegacyCluster (e.g. a v1-posted XRD round-tripped through the storage version) yields SchemaLegacy. The v2 Go type's Spec.Scope is a string alias with no runtime enum validation, so 'LegacyCluster' survives the round-trip and must be honored to avoid forcing consumers to specifically fetch v1-form.",
			gvk:        xrGVK,
			def:        v2XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", new("LegacyCluster")),
			wantSchema: ucomposite.SchemaLegacy,
		},
		"XRDDoesNotMatchXR": {
			reason:          "When the supplied XRD's Group+Kind does not match the input XR, return a clear error naming both.",
			gvk:             xrGVK,
			def:             v1XRD("xother.example.org", "example.org", "XOther", nil),
			wantErr:         true,
			wantErrContains: []string{"does not match the input XR", "XLegacyResource", "XOther"},
		},
		"XRMatchesByGroupAndKindOnDifferentVersion": {
			reason:     "An XR submitted at a served-but-not-referenceable version of the XRD's Group+Kind still selects schema correctly. GetCompositeGroupVersionKind returns only the referenceable version, so we match on Group+Kind rather than full GVK.",
			gvk:        schema.GroupVersionKind{Group: "example.org", Version: "v1beta1", Kind: "XLegacyResource"},
			def:        v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", nil),
			wantSchema: ucomposite.SchemaLegacy,
		},
		"UnrecognizedXRDAPIVersion": {
			reason: "An XRD with an apiVersion that isn't apiextensions.crossplane.io/{v1,v2} returns a clear error.",
			gvk:    xrGVK,
			def: mustStruct(map[string]any{
				"apiVersion": "apiextensions.crossplane.io/v3",
				"kind":       "CompositeResourceDefinition",
				"metadata":   map[string]any{"name": "weird.example.org"},
				"spec":       map[string]any{},
			}),
			wantErr:         true,
			wantErrContains: []string{"unrecognized apiVersion", "apiextensions.crossplane.io/v3"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := selectSchema(tc.gvk, tc.def)

			// Error assertions use substring matching rather than the
			// repo convention of cmp.Diff(want, got, cmpopts.EquateErrors()):
			// the substrings in wantErrContains assert that user-facing
			// error messages name the relevant XRD/GVK/apiVersion, which
			// EquateErrors cannot check.
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

func TestRenderErrors(t *testing.T) {
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
		// pipelineFatal asserts whether *PipelineFatalError is
		// expected in the returned error chain (errors.As).
		pipelineFatal bool
		// fatalStep / fatalMessage are checked only when pipelineFatal is
		// true.
		fatalStep, fatalMessage string
		// hasOutput indicates that Render must return non-nil output (the
		// partial-output contract on FATAL); when false, output must be nil.
		hasOutput bool
		// requiredResources is the expected count of recorded resource
		// selectors in the partial output. Checked only when hasOutput.
		requiredResources int
		// wantSelector is the expected first recorded selector. Checked
		// only when hasOutput && requiredResources > 0.
		wantSelector *fnv1.ResourceSelector
	}

	cases := map[string]struct {
		reason string
		// input returns the CompositeInput. It's a closure so the FATAL
		// case can stand up its own gRPC fixture and bake the address in.
		input func(t *testing.T) *renderv1alpha1.CompositeInput
		want  want
	}{
		"PipelineFatalReturnsRequirements": {
			reason: "When a pipeline step returns SEVERITY_FATAL, Render must return the partial CompositeOutput (with recorded RequiredResources) and an error chain containing *PipelineFatalError reachable via errors.As.",
			input: func(t *testing.T) *renderv1alpha1.CompositeInput {
				t.Helper()
				addr := rendertest.StartFunctionServer(t, &rendertest.FatalFunctionServer{
					RequirementName: requirementName,
					Selector:        wantSelector,
					FatalMessage:    fatalMsg,
				})
				return &renderv1alpha1.CompositeInput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XExample",
						"metadata":   map[string]any{"name": "my-example"},
					}),
					Composition: mustStruct(map[string]any{
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
		"NonFatalReconcileErrorWrapsAsBefore": {
			reason: "A render request whose CompositeResource cannot be decoded fails before Reconcile runs. The error must NOT be a *PipelineFatalError, and no partial output must be returned.",
			input: func(t *testing.T) *renderv1alpha1.CompositeInput {
				t.Helper()
				return &renderv1alpha1.CompositeInput{
					// Empty CompositeResource fails before Reconcile.
					CompositeResource: mustStruct(map[string]any{}),
					Composition: mustStruct(map[string]any{
						"metadata": map[string]any{"name": "broken"},
						"spec": map[string]any{
							"compositeTypeRef": map[string]any{
								"apiVersion": "example.org/v1alpha1",
								"kind":       "XBroken",
							},
							"pipeline": []any{},
						},
					}),
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
			out, err := Render(t.Context(), logging.NewNopLogger(), &feature.Flags{}, tc.input(t))

			if err == nil {
				t.Fatalf("%s\nRender(...) expected error, got nil", tc.reason)
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

			// Output presence + recorded selectors.
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

func TestCheckObservedResources(t *testing.T) {
	const xrUID = types.UID("xr-uid")

	observed := func(name, namespace string, owners ...metav1.OwnerReference) kunstructured.Unstructured {
		u := kunstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.org", Version: "v1alpha1", Kind: "XComposed"})
		u.SetName(name)
		u.SetNamespace(namespace)
		u.SetOwnerReferences(owners)

		return u
	}
	controllerRef := func(uid types.UID) metav1.OwnerReference {
		return metav1.OwnerReference{
			APIVersion: "example.org/v1alpha1",
			Kind:       "XLegacyResource",
			Name:       "my-xr",
			UID:        uid,
			Controller: new(true),
		}
	}
	nonControllerRef := func(uid types.UID) metav1.OwnerReference {
		return metav1.OwnerReference{
			APIVersion: "example.org/v1alpha1",
			Kind:       "XLegacyResource",
			Name:       "my-xr",
			UID:        uid,
		}
	}

	cases := map[string]struct {
		reason      string
		xrNamespace string
		observed    []kunstructured.Unstructured
		// wantErrContains lists substrings that must all appear in the error
		// message. An empty/nil slice means no error is expected.
		wantErrContains []string
	}{
		"NoObservedResources": {
			reason:   "An XR with no observed resources should pass validation.",
			observed: nil,
		},
		"NoControllerRefClusterScoped": {
			reason: "An observed resource with no owner refs should pass validation.",
			observed: []kunstructured.Unstructured{
				observed("obs-1", ""),
			},
		},
		"MatchingControllerRef": {
			reason: "An observed resource controlled by the XR (matching UID) should pass validation.",
			observed: []kunstructured.Unstructured{
				observed("obs-1", "", controllerRef(xrUID)),
			},
		},
		"NonControllerOwnerRefIgnored": {
			reason: "An observed resource with a non-controller owner ref (Controller != true) should pass validation even if its UID differs, because only controller refs are checked.",
			observed: []kunstructured.Unstructured{
				observed("obs-1", "", nonControllerRef("other-uid")),
			},
		},
		"MismatchedControllerRef": {
			reason: "An observed resource with a controller ref whose UID does not match the XR should fail validation.",
			observed: []kunstructured.Unstructured{
				observed("obs-bad", "", controllerRef("other-uid")),
			},
			wantErrContains: []string{"has a controller ref but is not controlled by the XR", "obs-bad"},
		},
		"NamespacedXRSameNamespace": {
			reason:      "A namespaced XR with an observed resource in the same namespace should pass validation.",
			xrNamespace: "ns-a",
			observed: []kunstructured.Unstructured{
				observed("obs-1", "ns-a"),
			},
		},
		"NamespacedXRDifferentNamespace": {
			reason:      "A namespaced XR with an observed resource in a different namespace should fail validation.",
			xrNamespace: "ns-a",
			observed: []kunstructured.Unstructured{
				observed("obs-bad", "ns-b"),
			},
			wantErrContains: []string{"is not in the same namespace as the XR", "obs-bad"},
		},
		"ClusterScopedXRIgnoresNamespace": {
			reason: "A cluster-scoped XR (empty namespace) should not enforce the namespace check, so an observed resource with any namespace passes validation.",
			observed: []kunstructured.Unstructured{
				observed("obs-1", "ns-b"),
			},
		},
		"MultipleViolationsJoined": {
			reason:      "A single observed resource that both has a mismatched controller ref and is in a different namespace should produce a joined error naming both violations.",
			xrNamespace: "ns-a",
			observed: []kunstructured.Unstructured{
				observed("obs-bad", "ns-b", controllerRef("other-uid")),
			},
			wantErrContains: []string{
				"has a controller ref but is not controlled by the XR",
				"is not in the same namespace as the XR",
				"obs-bad",
			},
		},
		"MultipleResourcesOneBad": {
			reason: "When multiple observed resources are supplied, only the offending one should be reported.",
			observed: []kunstructured.Unstructured{
				observed("obs-good", "", controllerRef(xrUID)),
				observed("obs-bad", "", controllerRef("other-uid")),
			},
			wantErrContains: []string{"has a controller ref but is not controlled by the XR", "obs-bad"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr := ucomposite.New()
			xr.SetUID(xrUID)
			if tc.xrNamespace != "" {
				xr.SetNamespace(tc.xrNamespace)
			}

			err := CheckObservedResources(xr, tc.observed)

			// Error assertions use substring matching rather than cmp.Diff(...,
			// cmpopts.EquateErrors()): the substrings assert that the
			// user-facing message names the offending resource and the specific
			// violation, which EquateErrors cannot check.
			if len(tc.wantErrContains) == 0 {
				if err != nil {
					t.Fatalf("\n%s\nCheckObservedResources(...): unexpected error: %v", tc.reason, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("\n%s\nCheckObservedResources(...): expected error, got nil", tc.reason)
			}
			for _, sub := range tc.wantErrContains {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("\n%s\nCheckObservedResources(...): error %q does not contain %q", tc.reason, err.Error(), sub)
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

func TestRenderDeletion(t *testing.T) {
	// A function that only desires first, e.g. to delete second before it.
	first := rendertest.StartFunctionServer(t, &rendertest.StaticFunctionServer{
		Response: &fnv1.RunFunctionResponse{
			Desired: &fnv1.State{
				Resources: map[string]*fnv1.Resource{
					"first": {Resource: mustStruct(map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata":   map[string]any{"name": "first", "namespace": "default"},
					})},
				},
			},
		},
	})

	// A function that desires nothing.
	nothing := rendertest.StartFunctionServer(t, &rendertest.StaticFunctionServer{
		Response: &fnv1.RunFunctionResponse{},
	})

	// The XR, Composition, and composed resources the cases share.
	xr := mustStruct(map[string]any{
		"apiVersion": "example.org/v1alpha1",
		"kind":       "XThing",
		"metadata": map[string]any{
			"name":              "my-xr",
			"namespace":         "default",
			"uid":               "xr-uid",
			"deletionTimestamp": "2026-11-02T00:00:00Z",
		},
	})
	composition := func(policy string) *structpb.Struct {
		return mustStruct(map[string]any{
			"metadata": map[string]any{"name": "things"},
			"spec": map[string]any{
				"compositeTypeRef": map[string]any{"apiVersion": "example.org/v1alpha1", "kind": "XThing"},
				"mode":             "Pipeline",
				"deletionPolicy":   policy,
				"pipeline": []any{map[string]any{
					"step":        "teardown",
					"functionRef": map[string]any{"name": "function-teardown"},
				}},
			},
		})
	}
	composed := func(name string) *structpb.Struct {
		return mustStruct(map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":        name,
				"namespace":   "default",
				"annotations": map[string]any{"crossplane.io/composition-resource-name": name},
				"ownerReferences": []any{map[string]any{
					"apiVersion":         "example.org/v1alpha1",
					"kind":               "XThing",
					"name":               "my-xr",
					"uid":                "xr-uid",
					"controller":         true,
					"blockOwnerDeletion": true,
				}},
			},
		})
	}
	unowned := func(name string) *structpb.Struct {
		return mustStruct(map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":        name,
				"namespace":   "default",
				"annotations": map[string]any{"crossplane.io/composition-resource-name": name},
			},
		})
	}
	ref := func(name string) map[string]any {
		return map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "name": name}
	}

	// Flags with function-controlled deletion enabled.
	fcd := &feature.Flags{}
	fcd.Enable(features.EnableAlphaFunctionControlledDeletion)

	type args struct {
		ff    *feature.Flags
		input *renderv1alpha1.CompositeInput
	}

	type want struct {
		err error
		out *renderv1alpha1.CompositeOutput
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FeatureOff": {
			reason: "Without function-controlled deletion a deleted XR should be done deleting whatever its Composition's policy: Crossplane removes its finalizer, and leaves its composed resources to Kubernetes.",
			args: args{
				ff: &feature.Flags{},
				input: &renderv1alpha1.CompositeInput{
					CompositeResource: xr,
					Composition:       composition("Pipeline"),
					ObservedResources: []*structpb.Struct{composed("first")},
					Functions:         []*renderv1alpha1.FunctionInput{{Name: "function-teardown", Address: first}},
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"uid":               "xr-uid",
							"resourceVersion":   "999",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{},
						},
						"spec": map[string]any{
							"crossplane": map[string]any{"resourceRefs": []any{ref("first")}},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Ready", "status": "False", "reason": "Deleting"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
						},
					}),
				},
			},
		},
		"Background": {
			reason: "A deleted XR whose Composition has the Background policy should be done deleting: Crossplane removes its finalizer, and leaves its composed resources to Kubernetes.",
			args: args{
				ff: fcd,
				input: &renderv1alpha1.CompositeInput{
					CompositeResource: xr,
					Composition:       composition("Background"),
					ObservedResources: []*structpb.Struct{composed("first")},
					Functions:         []*renderv1alpha1.FunctionInput{{Name: "function-teardown", Address: first}},
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"uid":               "xr-uid",
							"resourceVersion":   "999",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{},
						},
						"spec": map[string]any{
							"crossplane": map[string]any{"resourceRefs": []any{ref("first")}},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Ready", "status": "False", "reason": "Deleting"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
							"crossplane": map[string]any{"deletionPolicy": "Background"},
						},
					}),
				},
			},
		},
		"Foreground": {
			reason: "A deleted XR whose Composition has the Foreground policy should delete its composed resources without running its pipeline, and keep Crossplane's finalizer until they're gone.",
			args: args{
				ff: fcd,
				input: &renderv1alpha1.CompositeInput{
					CompositeResource: xr,
					Composition:       composition("Foreground"),
					ObservedResources: []*structpb.Struct{composed("first"), composed("second")},
					Functions:         []*renderv1alpha1.FunctionInput{{Name: "function-teardown", Address: first}},
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"uid":               "xr-uid",
							"resourceVersion":   "999",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{"composite.apiextensions.crossplane.io"},
						},
						"spec": map[string]any{
							"crossplane": map[string]any{"resourceRefs": []any{ref("first"), ref("second")}},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Ready", "status": "False", "reason": "Deleting", "message": "Waiting for 2 composed resource(s) to be deleted"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
							"crossplane": map[string]any{"deletionPolicy": "Foreground"},
						},
					}),
					DeletedResources: []*structpb.Struct{composed("first"), composed("second")},
				},
			},
		},
		"ForegroundWithoutControllerRefs": {
			reason: "A deleted XR should control observed resources that have no controller, so Crossplane deletes and waits for them.",
			args: args{
				ff: fcd,
				input: &renderv1alpha1.CompositeInput{
					CompositeResource: xr,
					Composition:       composition("Foreground"),
					ObservedResources: []*structpb.Struct{unowned("first"), unowned("second")},
					Functions:         []*renderv1alpha1.FunctionInput{{Name: "function-teardown", Address: first}},
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"uid":               "xr-uid",
							"resourceVersion":   "999",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{"composite.apiextensions.crossplane.io"},
						},
						"spec": map[string]any{
							"crossplane": map[string]any{"resourceRefs": []any{ref("first"), ref("second")}},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Ready", "status": "False", "reason": "Deleting", "message": "Waiting for 2 composed resource(s) to be deleted"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
							"crossplane": map[string]any{"deletionPolicy": "Foreground"},
						},
					}),
					DeletedResources: []*structpb.Struct{composed("first"), composed("second")},
				},
			},
		},
		"Pipeline": {
			reason: "A deleted XR whose Composition has the Pipeline policy should run its pipeline, delete the composed resources it no longer desires, and keep Crossplane's finalizer while some remain.",
			args: args{
				ff: fcd,
				input: &renderv1alpha1.CompositeInput{
					CompositeResource: xr,
					Composition:       composition("Pipeline"),
					ObservedResources: []*structpb.Struct{composed("first"), composed("second")},
					Functions:         []*renderv1alpha1.FunctionInput{{Name: "function-teardown", Address: first}},
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{"composite.apiextensions.crossplane.io"},
						},
						"spec": map[string]any{
							// Crossplane keeps referencing second until it's gone.
							"crossplane": map[string]any{"resourceRefs": []any{ref("first"), ref("second")}},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Ready", "status": "False", "reason": "Deleting", "message": "Waiting for the function pipeline to delete composed resources"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
							"crossplane": map[string]any{"deletionPolicy": "Pipeline"},
						},
					}),
					ComposedResources: []*structpb.Struct{mustStruct(map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":        "first",
							"namespace":   "default",
							"annotations": map[string]any{"crossplane.io/composition-resource-name": "first"},
							"labels":      map[string]any{"crossplane.io/composite": "my-xr"},
							"ownerReferences": []any{map[string]any{
								"apiVersion":         "example.org/v1alpha1",
								"kind":               "XThing",
								"name":               "my-xr",
								"uid":                "xr-uid",
								"controller":         true,
								"blockOwnerDeletion": true,
							}},
						},
					})},
					DeletedResources: []*structpb.Struct{composed("second")},
					Events: []*renderv1alpha1.Event{
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: things"},
						{Type: "Normal", Reason: "ComposeResources", Message: "Composed resource \"first\" is not yet ready"},
					},
				},
			},
		},
		"PipelineDone": {
			reason: "A deleted Pipeline XR whose pipeline desires nothing, with no composed resources left, should be done deleting: Crossplane removes its finalizer and keeps the others.",
			args: args{
				ff: fcd,
				input: &renderv1alpha1.CompositeInput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"uid":               "xr-uid",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{"example.org/other"},
						},
					}),
					Composition: composition("Pipeline"),
					Functions:   []*renderv1alpha1.FunctionInput{{Name: "function-teardown", Address: nothing}},
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{"example.org/other"},
						},
						"spec": map[string]any{
							"crossplane": map[string]any{"resourceRefs": []any{}},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Ready", "status": "False", "reason": "Deleting"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
							"crossplane": map[string]any{"deletionPolicy": "Pipeline"},
						},
					}),
					Events: []*renderv1alpha1.Event{
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: things"},
					},
				},
			},
		},
		"PipelineWaitsForDeletingResource": {
			reason: "A deleted Pipeline XR whose pipeline desires nothing should keep Crossplane's finalizer while a composed resource it controls is still being deleted.",
			args: args{
				ff: fcd,
				input: &renderv1alpha1.CompositeInput{
					CompositeResource: xr,
					Composition:       composition("Pipeline"),
					ObservedResources: []*structpb.Struct{mustStruct(map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":              "first",
							"namespace":         "default",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{"example.org/hold"},
							"annotations":       map[string]any{"crossplane.io/composition-resource-name": "first"},
							"ownerReferences": []any{map[string]any{
								"apiVersion":         "example.org/v1alpha1",
								"kind":               "XThing",
								"name":               "my-xr",
								"uid":                "xr-uid",
								"controller":         true,
								"blockOwnerDeletion": true,
							}},
						},
					})},
					Functions: []*renderv1alpha1.FunctionInput{{Name: "function-teardown", Address: nothing}},
				},
			},
			want: want{
				out: &renderv1alpha1.CompositeOutput{
					CompositeResource: mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XThing",
						"metadata": map[string]any{
							"name":              "my-xr",
							"namespace":         "default",
							"deletionTimestamp": "2026-11-02T00:00:00Z",
							"finalizers":        []any{"composite.apiextensions.crossplane.io"},
						},
						"spec": map[string]any{
							// Crossplane keeps referencing first until it's gone.
							"crossplane": map[string]any{"resourceRefs": []any{ref("first")}},
						},
						"status": map[string]any{
							"conditions": []any{
								map[string]any{"type": "Responsive", "status": "True", "reason": "WatchCircuitClosed"},
								map[string]any{"type": "Ready", "status": "False", "reason": "Deleting", "message": "Waiting for the function pipeline to delete composed resources"},
								map[string]any{"type": "Synced", "status": "True", "reason": "ReconcileSuccess"},
							},
							"crossplane": map[string]any{"deletionPolicy": "Pipeline"},
						},
					}),
					Events: []*renderv1alpha1.Event{
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: things"},
					},
				},
			},
		},
	}

	// Crossplane deletes composed resources in no particular order.
	byName := protocmp.SortRepeated(func(a, b *structpb.Struct) bool {
		return a.GetFields()["metadata"].GetStructValue().GetFields()["name"].GetStringValue() <
			b.GetFields()["metadata"].GetStructValue().GetFields()["name"].GetStringValue()
	})

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := Render(t.Context(), logging.NewNopLogger(), tc.args.ff, tc.args.input)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.out, out, cmpopts.EquateEmpty(), protocmp.Transform(), byName, ignoreTimestamps); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPrepareDeletion(t *testing.T) {
	deleted := metav1.Now()

	controller := metav1.OwnerReference{
		APIVersion:         "example.org/v1alpha1",
		Kind:               "XThing",
		Name:               "my-xr",
		UID:                "xr-uid",
		Controller:         new(true),
		BlockOwnerDeletion: new(true),
	}
	owner := metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "owner", UID: "owner-uid"}

	composed := func(name string, refs ...metav1.OwnerReference) kunstructured.Unstructured {
		u := kunstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("ConfigMap")
		u.SetName(name)
		u.SetOwnerReferences(refs)
		return u
	}

	type args struct {
		xr                         *ucomposite.Unstructured
		comp                       *apiextensionsv1.Composition
		observed                   []kunstructured.Unstructured
		functionControlledDeletion bool
	}

	type want struct {
		xr       *ucomposite.Unstructured
		observed []kunstructured.Unstructured
		deleting bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotDeleted": {
			reason: "We shouldn't touch an XR that isn't deleted.",
			args: args{
				xr:                         ucomposite.New(),
				comp:                       &apiextensionsv1.Composition{},
				functionControlledDeletion: true,
			},
			want: want{
				xr: ucomposite.New(),
			},
		},
		"Paused": {
			reason: "We shouldn't touch a paused XR, because Crossplane doesn't delete it.",
			args: args{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					xr.SetAnnotations(map[string]string{"crossplane.io/paused": "true"})
					return xr
				}(),
				comp:                       &apiextensionsv1.Composition{},
				observed:                   []kunstructured.Unstructured{composed("first")},
				functionControlledDeletion: true,
			},
			want: want{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					xr.SetAnnotations(map[string]string{"crossplane.io/paused": "true"})
					return xr
				}(),
				observed: []kunstructured.Unstructured{composed("first")},
			},
		},
		"FunctionControlledDeletionDisabled": {
			reason: "Without function-controlled deletion we should only add Crossplane's finalizer.",
			args: args{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					return xr
				}(),
				comp: &apiextensionsv1.Composition{Spec: apiextensionsv1.CompositionSpec{DeletionPolicy: apiextensionsv1.CompositionDeletionPolicyPipeline}},
			},
			want: want{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					xr.SetFinalizers([]string{xcomposite.Finalizer})
					return xr
				}(),
				deleting: true,
			},
		},
		"RecordCompositionPolicy": {
			reason: "We should add Crossplane's finalizer, and record the Composition's policy if the XR has none.",
			args: args{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					return xr
				}(),
				comp:                       &apiextensionsv1.Composition{Spec: apiextensionsv1.CompositionSpec{DeletionPolicy: apiextensionsv1.CompositionDeletionPolicyPipeline}},
				functionControlledDeletion: true,
			},
			want: want{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					xr.SetFinalizers([]string{xcomposite.Finalizer})
					xr.SetDeletionPolicy(apiextensionsv1.CompositionDeletionPolicyPipeline)
					return xr
				}(),
				deleting: true,
			},
		},
		"KeepRecordedPolicy": {
			reason: "We should keep the policy the XR recorded, even if its Composition's differs, because Crossplane deletes an XR using the policy it recorded.",
			args: args{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					xr.SetDeletionPolicy(apiextensionsv1.CompositionDeletionPolicyForeground)
					return xr
				}(),
				comp:                       &apiextensionsv1.Composition{Spec: apiextensionsv1.CompositionSpec{DeletionPolicy: apiextensionsv1.CompositionDeletionPolicyPipeline}},
				functionControlledDeletion: true,
			},
			want: want{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetDeletionTimestamp(&deleted)
					xr.SetFinalizers([]string{xcomposite.Finalizer})
					xr.SetDeletionPolicy(apiextensionsv1.CompositionDeletionPolicyForeground)
					return xr
				}(),
				deleting: true,
			},
		},
		"ControlObservedResources": {
			reason: "We should make the XR the controller of observed resources that have no controller, keeping their other owners, because Crossplane only deletes resources the XR controls.",
			args: args{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetAPIVersion("example.org/v1alpha1")
					xr.SetKind("XThing")
					xr.SetName("my-xr")
					xr.SetUID("xr-uid")
					xr.SetDeletionTimestamp(&deleted)
					return xr
				}(),
				comp:     &apiextensionsv1.Composition{},
				observed: []kunstructured.Unstructured{composed("first", owner), composed("second", controller)},
			},
			want: want{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New()
					xr.SetAPIVersion("example.org/v1alpha1")
					xr.SetKind("XThing")
					xr.SetName("my-xr")
					xr.SetUID("xr-uid")
					xr.SetDeletionTimestamp(&deleted)
					xr.SetFinalizers([]string{xcomposite.Finalizer})
					return xr
				}(),
				observed: []kunstructured.Unstructured{composed("first", owner, controller), composed("second", controller)},
				deleting: true,
			},
		},
		"LegacyXR": {
			reason: "We should only add Crossplane's finalizer to a legacy XR, because legacy XRs don't support deletion policies.",
			args: args{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New(ucomposite.WithSchema(ucomposite.SchemaLegacy))
					xr.SetDeletionTimestamp(&deleted)
					return xr
				}(),
				comp:                       &apiextensionsv1.Composition{Spec: apiextensionsv1.CompositionSpec{DeletionPolicy: apiextensionsv1.CompositionDeletionPolicyPipeline}},
				functionControlledDeletion: true,
			},
			want: want{
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New(ucomposite.WithSchema(ucomposite.SchemaLegacy))
					xr.SetDeletionTimestamp(&deleted)
					xr.SetFinalizers([]string{xcomposite.Finalizer})
					return xr
				}(),
				deleting: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			deleting := prepareDeletion(tc.args.xr, tc.args.comp, tc.args.observed, tc.args.functionControlledDeletion)

			if diff := cmp.Diff(tc.want.deleting, deleting); diff != "" {
				t.Errorf("\n%s\nprepareDeletion(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xr.Object, tc.args.xr.Object); diff != "" {
				t.Errorf("\n%s\nprepareDeletion(...): -want XR, +got XR:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.observed, tc.args.observed); diff != "" {
				t.Errorf("\n%s\nprepareDeletion(...): -want observed, +got observed:\n%s", tc.reason, diff)
			}
		})
	}
}
