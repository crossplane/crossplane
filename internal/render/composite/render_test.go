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
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
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

func strPtr(s string) *string { return &s }

func mustStruct(m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}

// fatalFunctionServer simulates a real function-extra-resources style pipeline
// step. On its first call it announces its required resources via
// Requirements.Resources (no FATAL), letting the FetchingFunctionRunner record
// the selectors via the recording fetcher. On its second call (when the
// fetched resources came back empty because the caller did not pre-populate
// them) it returns SEVERITY_FATAL.
type fatalFunctionServer struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	requirementName string
	selector        *fnv1.ResourceSelector
	fatalMessage    string

	mu    sync.Mutex
	calls int
}

func (s *fatalFunctionServer) RunFunction(_ context.Context, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()

	if call == 1 {
		// First iteration: announce the required resource. The
		// FetchingFunctionRunner will then fetch it, which is when the
		// RecordingRequiredResourcesFetcher records the selector.
		return &fnv1.RunFunctionResponse{
			Requirements: &fnv1.Requirements{
				Resources: map[string]*fnv1.ResourceSelector{
					s.requirementName: s.selector,
				},
			},
		}, nil
	}

	// Second iteration: the requirement still isn't satisfied; return FATAL.
	return &fnv1.RunFunctionResponse{
		Requirements: &fnv1.Requirements{
			Resources: map[string]*fnv1.ResourceSelector{
				s.requirementName: s.selector,
			},
		},
		Results: []*fnv1.Result{
			{Severity: fnv1.Severity_SEVERITY_FATAL, Message: s.fatalMessage},
		},
	}, nil
}

// startTestFunctionServer starts an in-process gRPC server registered with the
// supplied FunctionRunnerServiceServer and returns its address. The server is
// stopped automatically when the test ends.
func startTestFunctionServer(t *testing.T, ss fnv1.FunctionRunnerServiceServer) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen for test gRPC server: %v", err)
	}

	s := grpc.NewServer()
	fnv1.RegisterFunctionRunnerServiceServer(s, ss)
	go func() { _ = s.Serve(lis) }()

	t.Cleanup(func() {
		s.Stop()
	})

	return lis.Addr().String()
}

func TestRenderPipelineFatalReturnsRequirements(t *testing.T) {
	// Function step name and the FATAL message we expect to see propagated.
	const stepName = "fetch-extras"
	const fatalMsg = "Required extra resource \"namedClusterRole\" not found"
	const requirementName = "namedClusterRole"

	// Selector the function records as a requirement before fataling. After
	// the FATAL, this selector must still surface in CompositeOutput.
	wantSelector := &fnv1.ResourceSelector{
		ApiVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Match: &fnv1.ResourceSelector_MatchName{
			MatchName: "some-cluster-role",
		},
	}

	server := &fatalFunctionServer{
		requirementName: requirementName,
		selector:        wantSelector,
		fatalMessage:    fatalMsg,
	}

	addr := startTestFunctionServer(t, server)

	in := &renderv1alpha1.CompositeInput{
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

	out, err := Render(context.Background(), logging.NewNopLogger(), in)

	// the error must be a typed PipelineFatalError with the right step
	// and message, even when wrapped by the reconciler chain.
	var pfe *xcomposite.PipelineFatalError
	if !errors.As(err, &pfe) {
		t.Fatalf("Render(...) error: want *PipelineFatalError in chain, got %T: %v", err, err)
	}
	if pfe.Step != stepName {
		t.Errorf("PipelineFatalError.Step = %q, want %q", pfe.Step, stepName)
	}
	if pfe.Message != fatalMsg {
		t.Errorf("PipelineFatalError.Message = %q, want %q", pfe.Message, fatalMsg)
	}

	// the partial output must be returned and must contain the
	// recorded resource selector, so callers can iterate on requirements.
	if out == nil {
		t.Fatalf("Render(...) returned nil output on PipelineFatalError; want non-nil with RequiredResources populated")
	}
	if got := len(out.GetRequiredResources()); got != 1 {
		t.Fatalf("len(out.RequiredResources) = %d, want 1; out=%v", got, out)
	}

	// Decode the recorded selector and compare to the one the function
	// returned. Render encodes selectors via protojson; reverse it.
	gotSelector := &fnv1.ResourceSelector{}
	bs, err := out.GetRequiredResources()[0].MarshalJSON()
	if err != nil {
		t.Fatalf("cannot marshal recorded selector to JSON: %v", err)
	}
	if err := protojson.Unmarshal(bs, gotSelector); err != nil {
		t.Fatalf("cannot decode recorded ResourceSelector: %v", err)
	}
	if diff := cmp.Diff(wantSelector, gotSelector, protocmp.Transform()); diff != "" {
		t.Errorf("recorded ResourceSelector: -want, +got:\n%s", diff)
	}
}

func TestRenderNonFatalReconcileErrorWraps(t *testing.T) {
	// A render request whose CompositeResource cannot be decoded
	// fails before Reconcile even runs. The error must NOT be a
	// PipelineFatalError, and the existing wrapping must be preserved.
	in := &renderv1alpha1.CompositeInput{
		// Missing CompositeResource — Render's protobuf decode of the XR
		// returns an error before reaching the reconciler.
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

	out, err := Render(context.Background(), logging.NewNopLogger(), in)
	if err == nil {
		t.Fatalf("Render(...) expected error, got nil; out=%v", out)
	}
	var pfe *xcomposite.PipelineFatalError
	if errors.As(err, &pfe) {
		t.Errorf("Render(...) error unexpectedly classified as PipelineFatalError: %v", err)
	}
	if out != nil {
		t.Errorf("Render(...) returned out=%v on non-fatal error; want nil", out)
	}
}
