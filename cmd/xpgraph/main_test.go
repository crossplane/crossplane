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

package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func xr(refs ...any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ordering.example.org/v1alpha1",
		"kind":       "XOrdering",
		"metadata":   map[string]any{"name": "ordered", "namespace": "default"},
		"spec":       map[string]any{"crossplane": map[string]any{"resourceRefs": refs}},
	}}
}

func ref(name, kind string, dependsOn ...string) map[string]any {
	r := map[string]any{
		"apiVersion":   "nop.crossplane.io/v1alpha1",
		"kind":         kind,
		"name":         "ordered-" + name,
		"resourceName": name,
	}

	if len(dependsOn) > 0 {
		d := make([]any, 0, len(dependsOn))
		for _, n := range dependsOn {
			d = append(d, n)
		}

		r["dependsOn"] = d
	}

	return r
}

func TestReadGraph(t *testing.T) {
	type want struct {
		names      map[string][]string
		namespaces map[string]string
		err        bool
	}

	cases := map[string]struct {
		reason string
		xr     *unstructured.Unstructured
		want   want
	}{
		"EdgesAreRead": {
			reason: "Each reference's dependsOn should become that node's edges.",
			xr: xr(
				ref("first", "NopResource"),
				ref("second", "NopResource", "first"),
				ref("third", "NopResource", "second", "first"),
			),
			want: want{names: map[string][]string{
				"first":  nil,
				"second": {"first"},
				"third":  {"second", "first"},
			}},
		},
		"NoReferences": {
			reason: "An XR with no composed resource references has no graph to print.",
			xr: &unstructured.Unstructured{Object: map[string]any{
				"kind":     "XOrdering",
				"metadata": map[string]any{"name": "ordered"},
				"spec":     map[string]any{},
			}},
			want: want{err: true},
		},
		"LegacyReferences": {
			reason: "A legacy XR keeps its references at the top of spec.",
			xr: &unstructured.Unstructured{Object: map[string]any{
				"kind":     "XOrdering",
				"metadata": map[string]any{"name": "ordered"},
				"spec": map[string]any{"resourceRefs": []any{
					ref("only", "NopResource"),
				}},
			}},
			want: want{names: map[string][]string{"only": nil}},
		},
		"ClusterScopedCompositeRecordsNamespaces": {
			reason: "A cluster scoped composite records each composed resource's namespace, since they can differ.",
			xr: xr(map[string]any{
				"apiVersion":   "helm.m.crossplane.io/v1beta1",
				"kind":         "Release",
				"name":         "traefik-abcde",
				"namespace":    "modelplane-system",
				"resourceName": "traefik",
			}),
			want: want{names: map[string][]string{"traefik": nil}, namespaces: map[string]string{"traefik": "modelplane-system"}},
		},
		"ReferenceWithoutResourceName": {
			reason: "A reference predating the composition resource name falls back to the object name, so it still appears.",
			xr: xr(map[string]any{
				"apiVersion": "nop.crossplane.io/v1alpha1",
				"kind":       "NopResource",
				"name":       "ordered-abcde",
			}),
			want: want{names: map[string][]string{"ordered-abcde": nil}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := readGraph(tc.xr)
			if tc.want.err {
				if err == nil {
					t.Errorf("readGraph(...): want error, got none\n%s", tc.reason)
				}

				return
			}

			if err != nil {
				t.Fatalf("readGraph(...): %v\n%s", err, tc.reason)
			}

			names := map[string][]string{}
			for _, n := range got {
				names[n.Name] = n.DependsOn
			}

			if diff := cmp.Diff(tc.want.names, names, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("readGraph(...): -want, +got:\n%s\n%s", diff, tc.reason)
			}

			if tc.want.namespaces != nil {
				ns := map[string]string{}
				for _, n := range got {
					ns[n.Name] = n.Namespace
				}

				if diff := cmp.Diff(tc.want.namespaces, ns, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("readGraph(...) namespaces: -want, +got:\n%s\n%s", diff, tc.reason)
				}
			}
		})
	}
}

func TestWaves(t *testing.T) {
	cases := map[string]struct {
		reason     string
		nodes      []*node
		wantWaves  map[string]int
		wantCyclic []string
	}{
		"Chain": {
			reason: "A chain should put each resource one wave deeper than what it depends on.",
			nodes: []*node{
				{Name: "first"},
				{Name: "second", DependsOn: []string{"first"}},
				{Name: "third", DependsOn: []string{"second"}},
			},
			wantWaves: map[string]int{"first": 0, "second": 1, "third": 2},
		},
		"Diamond": {
			reason: "A resource waits for its deepest dependency, not its first.",
			nodes: []*node{
				{Name: "root"},
				{Name: "left", DependsOn: []string{"root"}},
				{Name: "right", DependsOn: []string{"left"}},
				{Name: "join", DependsOn: []string{"root", "right"}},
			},
			wantWaves: map[string]int{"root": 0, "left": 1, "right": 2, "join": 3},
		},
		"Cycle": {
			reason: "Resources in a cycle can never be ordered, and should be reported rather than placed.",
			nodes: []*node{
				{Name: "alone"},
				{Name: "vpc", DependsOn: []string{"subnet"}},
				{Name: "subnet", DependsOn: []string{"vpc"}},
			},
			wantWaves:  map[string]int{"alone": 0},
			wantCyclic: []string{"subnet", "vpc"},
		},
		"EdgeToUnknown": {
			reason: "An edge naming something the XR doesn't reference shouldn't stall the rest of the graph.",
			nodes: []*node{
				{Name: "orphan", DependsOn: []string{"gone"}},
			},
			wantWaves: map[string]int{"orphan": 0},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotCyclic := waves(tc.nodes)

			gotWaves := map[string]int{}
			for _, n := range tc.nodes {
				if n.wave >= 0 {
					gotWaves[n.Name] = n.wave
				}
			}

			if diff := cmp.Diff(tc.wantWaves, gotWaves); diff != "" {
				t.Errorf("waves(...): -want, +got:\n%s\n%s", diff, tc.reason)
			}

			if diff := cmp.Diff(tc.wantCyclic, gotCyclic, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("waves(...) cyclic: -want, +got:\n%s\n%s", diff, tc.reason)
			}
		})
	}
}

func TestState(t *testing.T) {
	cases := map[string]struct {
		reason string
		node   node
		want   string
	}{
		"Pending":  {reason: "A resource Crossplane hasn't created yet is pending.", node: node{}, want: "pending"},
		"Creating": {reason: "A resource that exists but isn't ready is still creating.", node: node{Exists: true}, want: "creating"},
		"Ready":    {reason: "A resource reporting Ready releases what depends on it.", node: node{Exists: true, Ready: true}, want: "ready"},
		"Deleting": {
			reason: "Deleting wins over ready: a resource on its way out is what teardown is waiting for.",
			node:   node{Exists: true, Ready: true, Deleting: true},
			want:   "deleting",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.node.state(); got != tc.want {
				t.Errorf("state(): want %q, got %q\n%s", tc.want, got, tc.reason)
			}
		})
	}
}

func TestRenderTree(t *testing.T) {
	nodes := []*node{
		{Name: "database", Kind: "NopResource", Exists: true, Ready: true},
		{Name: "app", Kind: "NopResource", DependsOn: []string{"database"}, Exists: true},
		{Name: "ingress", Kind: "NopResource", DependsOn: []string{"app"}},
	}

	got := renderTree(xr(), nodes, style{color: false})

	for _, want := range []string{
		"XOrdering default/ordered",
		"3 composed resources · 2 edges · 3 waves",
		"wave 0 ",
		"✔ ready       database",
		"wave 1 ",
		"◐ creating    app",
		"← database",
		"wave 2 ",
		"○ pending     ingress",
		"1 ready · 1 creating · 1 pending",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderTree(...): missing %q in:\n%s", want, got)
		}
	}

	if strings.Contains(got, "\x1b[") {
		t.Errorf("renderTree(...): wrote escape codes with color off:\n%q", got)
	}
}

func TestRenderTreeColor(t *testing.T) {
	nodes := []*node{{Name: "database", Kind: "NopResource", Exists: true, Ready: true}}

	got := renderTree(xr(), nodes, style{color: true})

	// The glyph carries the state where colour isn't available, so both
	// should be present when it is.
	if !strings.Contains(got, "\x1b[32m✔") {
		t.Errorf("renderTree(...): want a green tick, got:\n%q", got)
	}
}

func TestRenderTreeShowsWhyNotReady(t *testing.T) {
	nodes := []*node{{Name: "traefik", Kind: "Release", Exists: true, Reason: "Ready=Installing"}}

	got := renderTree(xr(), nodes, style{color: false})

	// A stuck resource should say why on its own row, rather than needing a
	// separate kubectl describe.
	if !strings.Contains(got, "Ready=Installing") {
		t.Errorf("renderTree(...): want the reason, got:\n%s", got)
	}
}

func TestRenderTreeNoEdges(t *testing.T) {
	nodes := []*node{{Name: "lonely", Kind: "NopResource"}}

	got := renderTree(xr(), nodes, style{color: false})

	// Worth saying out loud: an empty graph usually means the feature flag
	// is off, not that the pipeline declared nothing.
	if !strings.Contains(got, "No ordering constraints") {
		t.Errorf("renderTree(...): want the no-constraints note, got:\n%s", got)
	}
}

func TestRenderDot(t *testing.T) {
	nodes := []*node{
		{Name: "database", Kind: "NopResource", Exists: true, Ready: true},
		{Name: "app", Kind: "NopResource", DependsOn: []string{"database"}},
	}

	got := renderDot(xr(), nodes)

	// Edges point at what a resource waits for, which is also the order
	// teardown runs in.
	for _, want := range []string{
		"digraph",
		`"app" -> "database"`,
		"green4",
		// A real newline escape, not an escaped backslash: Graphviz renders
		// the latter as a literal \n in the node.
		`label="database\nNopResource"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderDot(...): missing %q in:\n%s", want, got)
		}
	}
}

// TestReadDependsOnBothForms covers an edge written as a bare name and as an
// object. xpgraph is pointed at clusters it didn't build, which will be on
// either side of that change for as long as both exist.
func TestReadDependsOnBothForms(t *testing.T) {
	cases := map[string]struct {
		reason       string
		ref          map[string]any
		wantComposed []string
		wantRequires []string
	}{
		"Strings": {
			reason:       "The original form is a list of composition resource names.",
			ref:          map[string]any{"dependsOn": []any{"vpc", "subnet"}},
			wantComposed: []string{"vpc", "subnet"},
		},
		"Objects": {
			reason: "The object form names the same thing under a key.",
			ref: map[string]any{"dependsOn": []any{
				map[string]any{"name": "vpc"},
				map[string]any{"name": "subnet", "type": "ComposedResource"},
			}},
			wantComposed: []string{"vpc", "subnet"},
		},
		"RequiredResource": {
			reason: "A required resource isn't a node, so it's kept apart from the edges between composed resources.",
			ref: map[string]any{"dependsOn": []any{
				map[string]any{"name": "vpc"},
				map[string]any{
					"type":        "RequiredResource",
					"requirement": map[string]any{"name": "cluster-kubeconfig"},
				},
			}},
			wantComposed: []string{"vpc"},
			wantRequires: []string{"cluster-kubeconfig"},
		},
		"Mixed": {
			reason:       "Nothing stops a cluster carrying both during a migration.",
			ref:          map[string]any{"dependsOn": []any{"vpc", map[string]any{"name": "subnet"}}},
			wantComposed: []string{"vpc", "subnet"},
		},
		"Absent": {
			reason: "A resource with no edges reads as no edges, not an error.",
			ref:    map[string]any{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			composed, requires := readDependsOn(tc.ref)

			if diff := cmp.Diff(tc.wantComposed, composed, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nreadDependsOn(...) composed: -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.wantRequires, requires, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nreadDependsOn(...) requires: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestReadPending covers the case xpgraph could not show at all before: a
// resource the graph is holding back from being created has no composed
// resource reference, so the XR's status is the only place it appears.
func TestReadPending(t *testing.T) {
	x := xr(ref("vpc", "NopResource"))
	_ = unstructured.SetNestedSlice(x.Object, []any{
		map[string]any{
			"apiVersion":   "nop.crossplane.io/v1alpha1",
			"kind":         "NopResource",
			"resourceName": "subnet",
			"operation":    "Create",
			"dependsOn":    []any{map[string]any{"name": "vpc"}},
			"reason":       "waiting for vpc to be ready",
		},
		map[string]any{
			"apiVersion":   "nop.crossplane.io/v1alpha1",
			"kind":         "NopResource",
			"resourceName": "vpc",
			"operation":    "Delete",
			"reason":       "no longer desired; subnet still depends on it",
			"deadlocked":   true,
		},
	}, "status", "crossplane", "pendingResources")

	nodes, err := readGraph(x)
	if err != nil {
		t.Fatalf("readGraph(...): %v", err)
	}

	nodes = readPending(x, nodes)

	got := map[string]string{}
	for _, n := range nodes {
		got[n.Name] = n.state()
	}

	want := map[string]string{
		// Held back from creation, and referenced nowhere else.
		"subnet": "blocked",
		// Held back from deletion, and deadlocked, which outranks it.
		"vpc": "deadlocked",
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("readPending(...): -want states, +got states:\n%s", diff)
	}

	for _, n := range nodes {
		if n.Name == "subnet" && !cmp.Equal(n.DependsOn, []string{"vpc"}) {
			t.Errorf("a pending resource should carry its edges, got %v", n.DependsOn)
		}
	}
}

// TestRenderTreeShowsHeldAndDeadlocked is a readability check as much as a
// correctness one: the states the graph adds have to be distinguishable from
// the states a resource reaches on its own.
func TestRenderTreeShowsHeldAndDeadlocked(t *testing.T) {
	nodes := []*node{
		{Name: "vpc", Kind: "NopResource", Exists: true, Ready: true},
		{
			Name: "subnet", Kind: "NopResource", DependsOn: []string{"vpc"},
			Requires: []string{"cluster-kubeconfig"},
			Held:     true, Operation: "Create", Reason: "waiting for cluster-kubeconfig",
		},
		{
			Name: "gateway", Kind: "NopResource", DependsOn: []string{"subnet"},
			Exists: true, Ready: true, Held: true, Operation: "Delete",
			Deadlocked: true, Reason: "subnet depends on it and cannot be deleted",
		},
	}

	got := renderTree(xr(), nodes, style{})
	t.Logf("\n%s", got)

	for _, want := range []string{
		"blocked",                         // held back from creation
		"deadlocked",                      // and the state that outranks all
		"⇠ cluster-kubeconfig",            // a required resource, not a node
		"deadlocked, so waiting will not", // its own block at the bottom
		"subnet depends on it",            // with the reason
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderTree(...) = %q\nwant it to contain %q", got, want)
		}
	}
}
