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
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/render/rendertest"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// orderingFunctionServer composes two ConfigMaps, and declares that b depends
// on a. It records whether it was told dependencies are supported.
type orderingFunctionServer struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	mu        sync.Mutex
	supported bool
}

func (s *orderingFunctionServer) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	s.mu.Lock()
	s.supported = slices.Contains(req.GetMeta().GetCapabilities(), fnv1.Capability_CAPABILITY_DEPENDENCIES)
	s.mu.Unlock()

	cm := func(name string) *fnv1.Resource {
		return &fnv1.Resource{Resource: mustStruct(map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": name, "namespace": "default"},
			"data":       map[string]any{"name": name},
		})}
	}

	return &fnv1.RunFunctionResponse{
		Desired: &fnv1.State{
			Composite: req.GetDesired().GetComposite(),
			Resources: map[string]*fnv1.Resource{"a": cm("a"), "b": cm("b")},
		},
		Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{{
			Resource:  "b",
			DependsOn: &fnv1.Dependency_ComposedResource{ComposedResource: "a"},
		}}},
	}, nil
}

func (s *orderingFunctionServer) wasSupported() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.supported
}

// orderingInput renders a namespaced XR with a v2 XRD, so it reports what
// ordering holds back under status.crossplane.
func orderingInput(addr string) *renderv1alpha1.CompositeInput {
	return &renderv1alpha1.CompositeInput{
		CompositeResource: mustStruct(map[string]any{
			"apiVersion": "example.org/v1alpha1",
			"kind":       "XOrdered",
			"metadata":   map[string]any{"name": "ordered", "namespace": "default"},
		}),
		Composition: mustStruct(map[string]any{
			"metadata": map[string]any{"name": "ordered"},
			"spec": map[string]any{
				"compositeTypeRef": map[string]any{"apiVersion": "example.org/v1alpha1", "kind": "XOrdered"},
				"mode":             "Pipeline",
				"pipeline": []any{
					map[string]any{"step": "compose", "functionRef": map[string]any{"name": "function-ordering"}},
				},
			},
		}),
		CompositeResourceDefinition: mustStruct(map[string]any{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata":   map[string]any{"name": "xordereds.example.org"},
			"spec": map[string]any{
				"group":    "example.org",
				"scope":    "Namespaced",
				"names":    map[string]any{"kind": "XOrdered", "plural": "xordereds"},
				"versions": []any{map[string]any{"name": "v1alpha1", "served": true, "referenceable": true}},
			},
		}),
		Functions: []*renderv1alpha1.FunctionInput{{Name: "function-ordering", Address: addr}},
	}
}

func TestRenderComposedResourceOrdering(t *testing.T) {
	type want struct {
		supported bool
		composed  []string
		pending   []string
	}

	cases := map[string]struct {
		reason string
		opts   []Option
		want   want
	}{
		"Enabled": {
			reason: "With ordering enabled, render should tell the function dependencies are supported, render only what the graph lets through, and report what it holds back, as a Crossplane with the feature on would.",
			opts:   []Option{WithComposedResourceOrdering(true)},
			want:   want{supported: true, composed: []string{"a"}, pending: []string{"b"}},
		},
		"Disabled": {
			reason: "With ordering off, render should neither advertise dependencies nor honor them, as a Crossplane with the feature off doesn't.",
			want:   want{supported: false, composed: []string{"a", "b"}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := &orderingFunctionServer{}
			addr := rendertest.StartFunctionServer(t, fn)

			out, err := Render(t.Context(), logging.NewNopLogger(), orderingInput(addr), tc.opts...)
			if err != nil {
				t.Fatalf("\n%s\nRender(...): %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want.supported, fn.wasSupported()); diff != "" {
				t.Errorf("\n%s\nCAPABILITY_DEPENDENCIES advertised: -want, +got:\n%s", tc.reason, diff)
			}

			composed := make([]string, 0, len(out.GetComposedResources()))
			for _, r := range out.GetComposedResources() {
				composed = append(composed, r.GetFields()["metadata"].GetStructValue().GetFields()["name"].GetStringValue())
			}
			sort.Strings(composed)
			if diff := cmp.Diff(tc.want.composed, composed); diff != "" {
				t.Errorf("\n%s\nRender(...) composed resources: -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.pending, pendingNames(out.GetCompositeResource())); diff != "" {
				t.Errorf("\n%s\nRender(...) status.crossplane.pendingResources: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// pendingNames returns the resource names in an XR's
// status.crossplane.pendingResources, or nil if it has none.
func pendingNames(xr *structpb.Struct) []string {
	pending := xr.GetFields()["status"].GetStructValue().GetFields()["crossplane"].GetStructValue().GetFields()["pendingResources"].GetListValue().GetValues()
	if len(pending) == 0 {
		return nil
	}
	names := make([]string, 0, len(pending))
	for _, p := range pending {
		names = append(names, p.GetStructValue().GetFields()["resourceName"].GetStringValue())
	}
	return names
}
