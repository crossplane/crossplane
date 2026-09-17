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

// An Edge declares that Resource depends on Target.
type Edge struct {
	// Resource is the composed resource that has the dependency.
	Resource string

	// DependsOn is what it depends on.
	DependsOn Target

	// CreateBeforeDestroy allows Resource to be created without waiting for
	// DependsOn to be deleted. Only meaningful for composed targets.
	CreateBeforeDestroy bool
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
}

// New returns a Graph over the supplied edges. It does not validate them;
// call Validate for that.
func New(edges ...Edge) *Graph {
	return &Graph{edges: edges}
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

	return &Graph{edges: kept}, dropped
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
			if e.CreateBeforeDestroy {
				return errors.Errorf("edge from %q to requirement %q cannot set CreateBeforeDestroy", e.Resource, t.RequiredResource.RequirementName)
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
	out := []string{}
	for _, e := range g.edges {
		if e.Resource == n && e.DependsOn.ComposedResource != "" {
			out = append(out, e.DependsOn.ComposedResource)
		}
	}

	sort.Strings(out)

	return out
}

// dependentsOf returns the composed resources that depend on n.
func (g *Graph) dependentsOf(n string) []string {
	out := []string{}
	for _, e := range g.edges {
		if e.DependsOn.ComposedResource == n {
			out = append(out, e.Resource)
		}
	}

	sort.Strings(out)

	return out
}
