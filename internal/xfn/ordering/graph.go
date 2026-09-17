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

// Package ordering is a prototype of the composed resource ordering
// described in design/design-doc-composed-resource-ordering.md. It models the
// decision logic only - which composed resources may be applied or deleted
// this reconcile - so the semantics can be tested before any protocol or
// reconciler changes are made.
package ordering

import (
	"fmt"
	"sort"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// A RequiredRef identifies a resource the pipeline required rather than
// composed. Name is optional; when empty, every resource the requirement
// matched must be ready.
type RequiredRef struct {
	RequirementName string
	Name            string
	Namespace       string
}

// A Target is what an Edge depends on: either a composed resource, by name, or
// a required resource. Exactly one must be set.
type Target struct {
	ComposedResource string
	RequiredResource *RequiredRef
}

// A Lifecycle says how an edge constrains the two resources it connects
// relative to each other.
//
// This mirrors the protocol's DependencyLifecycle without depending on it, so
// that adding a lifecycle later is one more value here rather than one more
// flag on Edge - which is the reason the protocol carries an enum rather than
// a boolean in the first place.
type Lifecycle int

const (
	// LifecycleSymmetric orders both directions: Resource is created only once
	// DependsOn is ready, and DependsOn is deleted only once Resource is gone.
	//
	// It is the zero value, so an Edge built without an opinion - by teardown,
	// rebuilding the graph from the XR's references - gets it.
	LifecycleSymmetric Lifecycle = iota

	// LifecycleCreateBeforeDestroy lets Resource be created without waiting
	// for DependsOn to be deleted. Resource must still exist and be ready
	// before DependsOn is deleted. Use it for a replacement that must exist
	// before its predecessor is torn down. Only valid for a composed target.
	LifecycleCreateBeforeDestroy
)

// String satisfies fmt.Stringer, so a lifecycle reads as itself in an error.
func (l Lifecycle) String() string {
	switch l {
	case LifecycleSymmetric:
		return "symmetric"
	case LifecycleCreateBeforeDestroy:
		return "create-before-destroy"
	default:
		return fmt.Sprintf("unknown(%d)", int(l))
	}
}

// An Edge declares that Resource depends on Target.
type Edge struct {
	// Resource is the composed resource that has the dependency.
	Resource string

	// DependsOn is what it depends on.
	DependsOn Target

	// Lifecycle says how this edge constrains creation and deletion. Only
	// LifecycleSymmetric is valid when DependsOn is a required resource.
	Lifecycle Lifecycle
}

// A ComposedState is what the reconciler knows about one composed resource at
// the point ordering decisions are made.
type ComposedState struct {
	// Desired is true if the resource is in the pipeline's final desired
	// state. A resource that isn't desired is a candidate for deletion.
	Desired bool

	// Observed is true if the resource exists in the cluster. A resource
	// that is terminating is still observed.
	Observed bool

	// Ready is the pipeline's readiness verdict, taken from the final desired
	// state. A resource that isn't desired has no verdict.
	Ready bool
}

// A RequiredState is what the reconciler knows about the resources one
// requirement matched. Ready is keyed by resource name.
type RequiredState struct {
	// Ready is keyed by namespace/name. Namespace is empty for cluster-scoped
	// resources, so identically named resources in separate namespaces retain
	// their distinct readiness verdicts.
	Ready map[string]bool
}

// RequiredResourceKey returns the identity used for a required resource in an
// ordering State. Kubernetes names and namespaces cannot contain '/', so the
// separator is unambiguous.
func RequiredResourceKey(namespace, name string) string {
	return namespace + "/" + name
}

// State is everything the ordering decision consumes.
type State struct {
	Composed map[string]ComposedState
	Required map[string]RequiredState
}

// A Graph is a validated set of ordering edges.
type Graph struct {
	edges []Edge

	// Adjacency, indexed once at construction.
	//
	// Every decision asks what a resource depends on, or what depends on it.
	// Answering that by scanning the edge list makes a single Decide cost
	// O(N*E), and the create-before-destroy lookup inside it another O(E) per
	// dependent - so a dense graph costs O(E^2) and a reconcile becomes
	// quadratic in the size of the composition. Indexing here trades one pass
	// over the edges for constant-time lookups afterwards.
	out   map[string][]Edge
	deps  map[string][]string
	rdeps map[string][]string
	cbd   map[[2]string]bool
}

// New returns a Graph over the supplied edges. It does not validate them;
// call Validate for that.
func New(edges ...Edge) *Graph {
	g := &Graph{
		edges: edges,
		out:   make(map[string][]Edge),
		deps:  make(map[string][]string),
		rdeps: make(map[string][]string),
		cbd:   make(map[[2]string]bool),
	}

	for _, e := range edges {
		// Kept in the order they were declared, so that a resource's edges are
		// considered in the same order a scan would have considered them.
		g.out[e.Resource] = append(g.out[e.Resource], e)

		t := e.DependsOn.ComposedResource
		if t == "" {
			// A required resource is always a leaf: nothing can depend on it,
			// so it takes no part in the adjacency used to order composed
			// resources against each other.
			continue
		}

		g.deps[e.Resource] = append(g.deps[e.Resource], t)
		g.rdeps[t] = append(g.rdeps[t], e.Resource)

		if e.Lifecycle == LifecycleCreateBeforeDestroy {
			g.cbd[[2]string{e.Resource, t}] = true
		}
	}

	// Sorted once here rather than on every lookup, so that events, errors and
	// decisions stay deterministic.
	for _, m := range []map[string][]string{g.deps, g.rdeps} {
		for k := range m {
			sort.Strings(m[k])
		}
	}

	return g
}

// Edges returns the graph's edges.
func (g *Graph) Edges() []Edge {
	return g.edges
}

// Prune returns the graph with stale edges removed, and reports which it
// dropped.
//
// An edge whose composed endpoint is in neither desired nor observed state is
// stale: the resource it names has finished being deleted, or was never
// composed at all. Those two cases are indistinguishable, so this is a prune
// rather than an error. Rejecting them would fail composition permanently for
// any function that returns a fixed set of rules - which is exactly what a
// declarative shim over composed resource names does - because its edges
// outlive the resources they order.
func (g *Graph) Prune(s State) (*Graph, []Edge) {
	kept := make([]Edge, 0, len(g.edges))
	dropped := []Edge{}

	for _, e := range g.edges {
		if _, ok := s.Composed[e.Resource]; !ok {
			dropped = append(dropped, e)
			continue
		}

		if t := e.DependsOn.ComposedResource; t != "" {
			if _, ok := s.Composed[t]; !ok {
				dropped = append(dropped, e)
				continue
			}
		}

		kept = append(kept, e)
	}

	// New again: pruning changes the adjacency, so the index is rebuilt.
	return New(kept...), dropped
}

// Validate checks that every edge refers to something that could exist, and
// that the composed-resource edges are acyclic. Required resources are always
// leaves, so they cannot take part in a cycle.
//
// Call Prune first. Validate treats an unresolvable composed resource name as
// an error, which is right for a graph that has already had its stale edges
// removed.
//
// requirements is the set of requirement names the pipeline declared.
func (g *Graph) Validate(s State, requirements map[string]bool) error {
	for _, e := range g.edges {
		if _, ok := s.Composed[e.Resource]; !ok {
			return errors.Errorf("edge refers to unknown composed resource %q", e.Resource)
		}

		switch t := e.DependsOn; {
		case t.RequiredResource != nil && t.ComposedResource != "":
			return errors.Errorf("edge from %q sets both a composed and a required target", e.Resource)

		case t.RequiredResource != nil:
			if !requirements[t.RequiredResource.RequirementName] {
				return errors.Errorf("edge from %q refers to undeclared requirement %q", e.Resource, t.RequiredResource.RequirementName)
			}
			// Crossplane never deletes a resource it doesn't compose, so
			// there is no delete direction to opt out of.
			if e.Lifecycle != LifecycleSymmetric {
				return errors.Errorf("edge from %q to requirement %q cannot set lifecycle %s", e.Resource, t.RequiredResource.RequirementName, e.Lifecycle)
			}

		case t.ComposedResource != "":
			if _, ok := s.Composed[t.ComposedResource]; !ok {
				return errors.Errorf("edge from %q refers to unknown composed resource %q", e.Resource, t.ComposedResource)
			}

		default:
			return errors.Errorf("edge from %q has no target", e.Resource)
		}
	}

	return g.acyclic()
}

// acyclic reports whether the composed-resource edges form a DAG.
func (g *Graph) acyclic() error {
	const (
		white = 0 // Unvisited.
		grey  = 1 // On the current path.
		black = 2 // Done.
	)

	color := map[string]int{}

	var visit func(n string, path []string) error

	visit = func(n string, path []string) error {
		switch color[n] {
		case black:
			return nil
		case grey:
			return errors.Errorf("dependency cycle: %v", append(path, n))
		}

		color[n] = grey

		for _, d := range g.dependenciesOf(n) {
			if err := visit(d, append(path, n)); err != nil {
				return err
			}
		}

		color[n] = black

		return nil
	}

	for _, n := range g.nodes() {
		if err := visit(n, nil); err != nil {
			return err
		}
	}

	return nil
}

// nodes returns every composed resource name the graph mentions, sorted so
// that errors are deterministic.
func (g *Graph) nodes() []string {
	seen := map[string]bool{}
	for _, e := range g.edges {
		seen[e.Resource] = true
		if e.DependsOn.ComposedResource != "" {
			seen[e.DependsOn.ComposedResource] = true
		}
	}

	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}

	sort.Strings(out)

	return out
}

// dependenciesOf returns the composed resources n depends on.
func (g *Graph) dependenciesOf(n string) []string {
	return g.deps[n]
}

// dependentsOf returns the composed resources that depend on n.
func (g *Graph) dependentsOf(n string) []string {
	return g.rdeps[n]
}
