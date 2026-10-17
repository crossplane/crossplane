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

package ordering

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestValidate(t *testing.T) {
	both := State{Composed: map[string]ComposedState{"a": live, "b": live, "c": live}}
	reqs := map[string]bool{"shared": true}

	cases := map[string]struct {
		reason  string
		graph   *Graph
		state   State
		wantErr bool
	}{
		"ValidChain": {
			reason: "A simple chain is acyclic and every name resolves.",
			graph:  New(on("b", "a"), on("c", "b")),
			state:  both,
		},
		"UnknownSource": {
			reason:  "An edge from a name in neither desired nor observed state is an error.",
			graph:   New(on("nope", "a")),
			state:   both,
			wantErr: true,
		},
		"UnknownTarget": {
			reason:  "An edge to a name in neither desired nor observed state is an error.",
			graph:   New(on("a", "nope")),
			state:   both,
			wantErr: true,
		},
		"TargetPresentOnlyInObserved": {
			reason: "A resource dropped from desired but still observed is exactly when an edge matters most.",
			graph:  New(on("a", "b")),
			state:  State{Composed: map[string]ComposedState{"a": live, "b": doomed}},
		},
		"DirectCycle": {
			reason:  "Two resources depending on each other is a cycle.",
			graph:   New(on("a", "b"), on("b", "a")),
			state:   both,
			wantErr: true,
		},
		"TransitiveCycle": {
			reason:  "A cycle through a third resource is still a cycle.",
			graph:   New(on("a", "b"), on("b", "c"), on("c", "a")),
			state:   both,
			wantErr: true,
		},
		"SelfEdge": {
			reason:  "A resource cannot depend on itself.",
			graph:   New(on("a", "a")),
			state:   both,
			wantErr: true,
		},
		"DiamondIsNotACycle": {
			reason: "Two paths to the same dependency are fine.",
			graph:  New(on("b", "a"), on("c", "a"), on("d", "b"), on("d", "c")),
			state:  State{Composed: map[string]ComposedState{"a": live, "b": live, "c": live, "d": live}},
		},
		"ValidRequirement": {
			reason: "An edge to a declared requirement is valid.",
			graph:  New(onReq("a", "shared")),
			state:  both,
		},
		"UndeclaredRequirement": {
			reason:  "An edge to a requirement no function asked for is an error.",
			graph:   New(onReq("a", "missing")),
			state:   both,
			wantErr: true,
		},
		"RequirementCannotCreateBeforeDestroy": {
			reason:  "Crossplane never deletes a required resource, so there is no delete direction to opt out of.",
			graph:   New(Edge{Resource: "a", DependsOn: Target{RequiredResource: &RequiredRef{RequirementName: "shared"}}, Lifecycle: LifecycleCreateBeforeDestroy}),
			state:   both,
			wantErr: true,
		},
		"RequirementsAreLeavesSoCannotCycle": {
			reason: "Many resources may depend on the same requirement without forming a cycle.",
			graph:  New(onReq("a", "shared"), onReq("b", "shared"), on("b", "a")),
			state:  both,
		},
		"EdgeWithNoTarget": {
			reason:  "An edge must depend on something.",
			graph:   New(Edge{Resource: "a"}),
			state:   both,
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.graph.Validate(tc.state, reqs)
			if tc.wantErr && err == nil {
				t.Errorf("\n%s\nValidate(...): want error, got none", tc.reason)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("\n%s\nValidate(...): unexpected error: %v", tc.reason, err)
			}
		})
	}
}

// TestValidateCyclePathIsNotGarbled pins the reported cycle path against the
// append-aliasing that made it wrong.
//
// The cycle detector builds the path it reports by appending each visited
// node onto its parent's path. Appending in place lets one sibling branch
// overwrite the backing array another branch's recursion is still reading,
// so the "dependency cycle: [...]" message could name nodes that were never
// on the cycle it found. The shape that triggers it: a node with several
// dependents, where the cycle is behind one of them.
func TestValidateCyclePathIsNotGarbled(t *testing.T) {
	// d hangs off b, and the cycle runs b -> c -> b. Visiting d first (nodes
	// are sorted, so d precedes nothing here - b is visited first with d's
	// path still spare) must not leave d in the path the b-c cycle reports.
	g := New(
		on("d", "b"),
		on("b", "c"),
		on("c", "b"),
	)

	s := State{Composed: map[string]ComposedState{
		"b": live,
		"c": live,
		"d": live,
	}}

	err := g.Validate(s, nil)
	if err == nil {
		t.Fatal("Validate(...): want a cycle error, got none")
	}

	// The reported path must start and end at the cycle, and must not contain
	// d: it is a dependent of the cycle, not part of it. The exact message is
	// "dependency cycle: [b c b]" - the visit order is deterministic because
	// dependenciesOf is sorted.
	want := "dependency cycle: [b c b]"
	if err.Error() != want {
		t.Errorf("Validate(...):\nwant: %s\n got: %s", want, err)
	}
}

func TestRetain(t *testing.T) {
	cases := map[string]struct {
		reason string
		prev   []Edge
		next   []Edge
		state  State
		want   []Edge
	}{
		"KeepsEdgeToResourcePendingDeletion": {
			reason: "A function that drops a resource from desired will naturally drop its edges in the same response. That is exactly when the garbage collector needs them.",
			prev:   []Edge{on("subnet", "vpc")},
			next:   []Edge{},
			state:  State{Composed: map[string]ComposedState{"subnet": doomed, "vpc": doomed}},
			want:   []Edge{on("subnet", "vpc")},
		},
		"DropsEdgeRetractedDuringNormalOperation": {
			reason: "An edge to a live, desired resource may be retracted freely.",
			prev:   []Edge{on("subnet", "vpc")},
			next:   []Edge{},
			state:  State{Composed: map[string]ComposedState{"subnet": live, "vpc": live}},
			want:   []Edge{},
		},
		"DropsEdgeToResourceFullyGone": {
			reason: "Once a resource is in neither desired nor observed state its edges are stale.",
			prev:   []Edge{on("subnet", "vpc")},
			next:   []Edge{},
			state:  State{Composed: map[string]ComposedState{"subnet": live, "vpc": {}}},
			want:   []Edge{},
		},
		"DoesNotDuplicateEdgeStillPresent": {
			reason: "An edge the function kept is not added twice.",
			prev:   []Edge{on("subnet", "vpc")},
			next:   []Edge{on("subnet", "vpc")},
			state:  State{Composed: map[string]ComposedState{"subnet": doomed, "vpc": doomed}},
			want:   []Edge{on("subnet", "vpc")},
		},
		"KeyDoesNotCollideAcrossUnrelatedEdges": {
			reason: `Composed resource names are arbitrary strings a function
				pipeline chooses. "a->b" depends on "c", and "a" depends on
				"b->c", are different edges, but a naive resource+"->"+target
				string key would give them the identical key "a->b->c". The
				second edge must still be retained on its own merits, not
				silently treated as a duplicate of the first.`,
			prev: []Edge{on("a->b", "c"), on("a", "b->c")},
			next: []Edge{on("a->b", "c")},
			state: State{Composed: map[string]ComposedState{
				"a->b": live,
				"c":    live,
				"a":    live,
				"b->c": doomed,
			}},
			want: []Edge{on("a->b", "c"), on("a", "b->c")},
		},
		"KeepsNewEdges": {
			reason: "Edges the function added are passed through.",
			prev:   []Edge{},
			next:   []Edge{on("a", "b")},
			state:  State{Composed: map[string]ComposedState{"a": live, "b": live}},
			want:   []Edge{on("a", "b")},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Retain(tc.prev, tc.next, tc.state)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRetain(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPrune(t *testing.T) {
	cases := map[string]struct {
		reason      string
		graph       *Graph
		state       State
		wantKept    int
		wantDropped int
	}{
		"KeepsResolvableEdges": {
			reason:   "An edge whose endpoints both exist is kept.",
			graph:    New(on("b", "a")),
			state:    State{Composed: map[string]ComposedState{"a": live, "b": live}},
			wantKept: 1,
		},
		"DropsEdgeWhoseTargetFinishedDeleting": {
			reason:      "Once a resource is gone from both desired and observed its edges are stale, not invalid.",
			graph:       New(on("b", "a")),
			state:       State{Composed: map[string]ComposedState{"b": live}},
			wantDropped: 1,
		},
		"DropsEdgeWhoseSourceFinishedDeleting": {
			reason:      "The same applies to the resource that had the dependency.",
			graph:       New(on("b", "a")),
			state:       State{Composed: map[string]ComposedState{"a": live}},
			wantDropped: 1,
		},
		"KeepsEdgeToResourcePendingDeletion": {
			reason:   "A target that has left desired but is still observed is exactly what the delete gate needs.",
			graph:    New(on("b", "a")),
			state:    State{Composed: map[string]ComposedState{"a": doomed, "b": live}},
			wantKept: 1,
		},
		"KeepsRequiredResourceEdges": {
			reason:   "A required resource isn't in composed state, so it is never pruned by name.",
			graph:    New(onReq("a", "shared")),
			state:    State{Composed: map[string]ComposedState{"a": live}},
			wantKept: 1,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, dropped := tc.graph.Prune(tc.state)
			if len(got.Edges()) != tc.wantKept {
				t.Errorf("\n%s\nPrune(...): want %d kept, got %d", tc.reason, tc.wantKept, len(got.Edges()))
			}

			if len(dropped) != tc.wantDropped {
				t.Errorf("\n%s\nPrune(...): want %d dropped, got %d", tc.reason, tc.wantDropped, len(dropped))
			}
		})
	}
}

func TestTargetDescribe(t *testing.T) {
	cases := map[string]struct {
		reason string
		target Target
		want   string
	}{
		"ComposedResource": {
			reason: "A composed resource is named directly.",
			target: Target{ComposedResource: "vpc"},
			want:   "vpc",
		},
		"RequirementOnly": {
			reason: "Name and namespace are both optional, and the common case sets neither. Joining them unconditionally would render 'env//' - separators with nothing between them.",
			target: Target{RequiredResource: &RequiredRef{RequirementName: "env"}},
			want:   "requirement env",
		},
		"RequirementWithClusterScopedName": {
			reason: "An edge singling out one cluster-scoped resource says which, without a namespace it doesn't have.",
			target: Target{RequiredResource: &RequiredRef{RequirementName: "env", Name: "shared"}},
			want:   "requirement env (shared)",
		},
		"RequirementWithNamespacedName": {
			reason: "A namespaced resource needs both halves to be identified, so both are shown.",
			target: Target{RequiredResource: &RequiredRef{RequirementName: "env", Name: "shared", Namespace: "platform-a"}},
			want:   "requirement env (platform-a/shared)",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.target.Describe(); got != tc.want {
				t.Errorf("\n%s\nDescribe(): want %q, got %q", tc.reason, tc.want, got)
			}
		})
	}
}

// TestValidateErrorsCarryTheirEdges pins that validation failures say which
// edges caused them, so the composer can name the pipeline step that declared
// them.
func TestValidateErrorsCarryTheirEdges(t *testing.T) {
	live := ComposedState{Desired: true, Observed: true}
	s := State{Composed: map[string]ComposedState{"a": live, "b": live, "c": live, "d": live}}

	t.Run("EdgeError", func(t *testing.T) {
		bad := onReq("a", "undeclared")
		err := New(on("b", "a"), bad).Validate(s, map[string]bool{})

		ee := &EdgeError{}
		if !errors.As(err, &ee) {
			t.Fatalf("Validate(...): want an *EdgeError, got %T: %v", err, err)
		}

		if diff := cmp.Diff(bad, ee.Edge); diff != "" {
			t.Errorf("EdgeError.Edge: -want, +got:\n%s", diff)
		}
	})

	t.Run("CycleError", func(t *testing.T) {
		// d leads into the cycle but isn't part of it.
		err := New(on("d", "a"), on("a", "b"), on("b", "c"), on("c", "a")).Validate(s, nil)

		ce := &CycleError{}
		if !errors.As(err, &ce) {
			t.Fatalf("Validate(...): want a *CycleError, got %T: %v", err, err)
		}

		got := ce.Edges()
		for _, e := range got {
			if e.Resource == "d" {
				t.Errorf("CycleError.Edges(): %v includes an edge leading into the cycle, not on it", got)
			}
		}

		if len(got) != 3 {
			t.Errorf("CycleError.Edges(): want the 3 edges of the cycle, got %v", got)
		}
	})
}
