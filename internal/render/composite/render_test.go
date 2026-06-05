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
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
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
				ObservedResources: []*structpb.Struct{
					mustStruct(map[string]any{
						"apiVersion": "example.org/v1alpha1",
						"kind":       "XComposed",
						"metadata": map[string]any{
							"name": "obs-1",
							"annotations": map[string]any{
								"crossplane.io/composition-resource-name": "obs-1",
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
							},
						}),
					},
					Events: []*renderv1alpha1.Event{
						{Type: "Normal", Reason: "SelectComposition", Message: "Successfully selected composition: legacy-composition"},
					},
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
			def:        v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", strPtr("LegacyCluster")),
			wantSchema: ucomposite.SchemaLegacy,
		},
		"V1XRDExplicitClusterScope": {
			reason:     "A v1 XRD with explicit Spec.Scope=Cluster (theoretical edge case) yields SchemaModern. Mirrors the production reconciler's rule.",
			gvk:        xrGVK,
			def:        v1XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", strPtr("Cluster")),
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
			def:        v2XRD("xlegacyresources.example.org", "example.org", "XLegacyResource", strPtr("LegacyCluster")),
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
			out, err := Render(t.Context(), logging.NewNopLogger(), tc.input(t))

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

func strPtr(s string) *string { return &s }

func mustStruct(m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}
