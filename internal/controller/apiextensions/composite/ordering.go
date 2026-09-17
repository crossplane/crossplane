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
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	"github.com/crossplane/crossplane/v2/internal/xfn/ordering"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// EdgesFromRefs rebuilds a dependency graph's composed-to-composed edges from
// an XR's persisted composed resource references. This is how core gets a graph
// on teardown, when no function has run: the references record what the
// pipeline declared on the last reconcile that succeeded.
//
// Edges to required resources are not persisted, so they are never rebuilt.
// They gate creation only, and nothing is created during teardown.
func EdgesFromRefs(refs []reference.Composed) []ordering.Edge {
	edges := []ordering.Edge{}

	for _, r := range refs {
		if r.ResourceName == "" {
			// A reference written before this field existed, or by a
			// controller that doesn't know about it. We can't place it in the
			// graph, and guessing would be worse than leaving it out.
			continue
		}

		for _, d := range r.DependsOn {
			edges = append(edges, ordering.Edge{
				Resource:  r.ResourceName,
				DependsOn: ordering.Target{ComposedResource: d},
			})
		}
	}

	return edges
}

// AsEdges converts protobuf dependencies to ordering edges.
func AsEdges(deps []*fnv1.Dependency) []ordering.Edge {
	out := make([]ordering.Edge, 0, len(deps))

	for _, d := range deps {
		e := ordering.Edge{
			Resource:            d.GetResource(),
			CreateBeforeDestroy: d.GetLifecycle() == fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_CREATE_BEFORE_DESTROY,
		}

		if r := d.GetRequiredResource(); r != nil {
			e.DependsOn.RequiredResource = &ordering.RequiredRef{
				RequirementName: r.GetRequirementName(),
				Name:            r.GetName(),
				Namespace:       r.GetNamespace(),
			}
		} else {
			e.DependsOn.ComposedResource = d.GetComposedResource()
		}

		out = append(out, e)
	}

	return out
}

// AsOrderingState builds the state an ordering decision consumes.
//
// A composed resource's readiness is the pipeline's own verdict, taken from the
// final desired state. A required resource has no such verdict - no function
// reports readiness for a resource it didn't compose - so we read its Ready
// status condition, treating existence alone as ready for kinds that don't have
// one.
func AsOrderingState(desired, observed ComposedResourceStates, required map[string]*fnv1.Resources) ordering.State {
	s := ordering.State{
		Composed: make(map[string]ordering.ComposedState, len(desired)+len(observed)),
		Required: make(map[string]ordering.RequiredState, len(required)),
	}

	for name, cd := range desired {
		cs := s.Composed[string(name)]
		cs.Desired = true
		cs.Ready = cd.Ready
		s.Composed[string(name)] = cs
	}

	for name := range observed {
		cs := s.Composed[string(name)]
		cs.Observed = true
		s.Composed[string(name)] = cs
	}

	for name, rs := range required {
		ready := make(map[string]bool, len(rs.GetItems()))

		for _, r := range rs.GetItems() {
			cd := composed.New()
			if err := xfn.FromStruct(cd, r.GetResource()); err != nil {
				// We can't tell whether it's ready, so we assume it isn't. A
				// resource we can't parse isn't something to proceed on.
				continue
			}

			ready[ordering.RequiredResourceKey(cd.GetNamespace(), cd.GetName())] = isReady(cd)
		}

		s.Required[name] = ordering.RequiredState{Ready: ready}
	}

	return s
}

// isReady reports whether a required resource should count as ready. A resource
// whose kind has no Ready condition counts as ready once it exists.
func isReady(cd *composed.Unstructured) bool {
	c := cd.GetCondition(xpv2.TypeReady)
	if c.Type == "" {
		// The resource has no Ready condition. Many kinds a function might
		// require - a ConfigMap, a Secret - never will. Existence is all the
		// readiness they have.
		return true
	}

	return c.Status == corev1.ConditionTrue
}

// RequirementNames returns the set of requirement names the pipeline resolved,
// for validating that an edge refers to a requirement that exists.
func RequirementNames(required map[string]*fnv1.Resources) map[string]bool {
	out := make(map[string]bool, len(required))
	for name := range required {
		out[name] = true
	}

	return out
}

// desiredStates adapts the in-flight desired State from a function pipeline to
// the ComposedResourceStates that AsOrderingState consumes. It's a partial view
// - it carries names and the pipeline's readiness verdict, not the rendered
// resources - which is all an ordering decision needs mid-pipeline.
func desiredStates(d *fnv1.State) ComposedResourceStates {
	out := make(ComposedResourceStates, len(d.GetResources()))
	for name, dr := range d.GetResources() {
		out[ResourceName(name)] = ComposedResourceState{Ready: dr.GetReady() == fnv1.Ready_READY_TRUE}
	}

	return out
}

// retainDependencies returns the edges to carry forward, given what a function
// returned and what it was sent. It keeps any edge the function dropped whose
// target has left desired state but is still observed - the pending-deletion
// case, and the only one where losing an edge does damage.
func retainDependencies(prev, next []*fnv1.Dependency, s ordering.State) []*fnv1.Dependency {
	have := make(map[dependencyIdentity]bool, len(next))
	for _, d := range next {
		have[dependencyKey(d)] = true
	}

	out := next

	for _, d := range prev {
		if have[dependencyKey(d)] {
			continue
		}

		// Only composed resources are deleted by Crossplane, so only their
		// edges are worth retaining.
		t := d.GetComposedResource()
		if t == "" {
			continue
		}

		if cs := s.Composed[t]; !cs.Desired && cs.Observed {
			out = append(out, d)
		}
	}

	return out
}

// A dependencyIdentity identifies an edge by its endpoints, so
// retainDependencies can use it as a map key to detect the same edge across
// two slices.
//
// Composed resource names and requirement names are arbitrary strings a
// function pipeline chooses, with no constraint on their characters.
// Encoding them into a single string with a separator, as this used to (e.g.
// resource+"->"+target), risks two distinct edges colliding on the same key
// if a name happens to contain the separator. A comparable struct of the
// actual fields has no such risk.
type dependencyIdentity struct {
	resource         string
	composedResource string
	requiredResource ordering.RequiredRef
}

// dependencyKey returns d's identity for use as a dependencyIdentity map key.
//
// It records both targets for the same reason ordering.key does: a malformed
// edge naming a composed resource and a requirement reaches this function,
// because validation happens after retention. Such an edge is a third thing,
// and must not collide with either edge naming one of its targets.
func dependencyKey(d *fnv1.Dependency) dependencyIdentity {
	k := dependencyIdentity{resource: d.GetResource(), composedResource: d.GetComposedResource()}

	if r := d.GetRequiredResource(); r != nil {
		k.requiredResource = ordering.RequiredRef{
			RequirementName: r.GetRequirementName(),
			Name:            r.GetName(),
			Namespace:       r.GetNamespace(),
		}
	}

	return k
}

// staleEdgeNames summarizes pruned edges for an event message.
func staleEdgeNames(edges []ordering.Edge) string {
	seen := map[string]bool{}
	out := make([]string, 0, len(edges))

	for _, e := range edges {
		s := e.Resource + " -> " + e.DependsOn.Describe()
		if seen[s] {
			continue
		}

		seen[s] = true

		out = append(out, s)
	}

	sort.Strings(out)

	return strings.Join(out, ", ")
}

// mergeRequiredResources unions one pipeline step's required resources into the
// set an ordering edge resolves against.
//
// Requirement names are per step everywhere else in core. Each step's request
// is built from its own bootstrap requirements and its own tracked ones - see
// Tracker.Requirements, which takes a step - so two functions can use the same
// requirement name for different resources without ever interfering.
//
// Ordering edges are different: an edge names a requirement declared anywhere
// in the pipeline, not only by the function that drew the edge. That means the
// per-step sets have to be brought together, and how they are brought together
// decides what a name shared by two steps means.
//
// Copying would let whichever step ran last decide, silently, and an edge from
// the first function would then be gated on the second function's resources.
// Unioning instead means an edge naming a requirement two steps declared waits
// for everything either of them matched. Where both matched the same resource -
// the ordinary case, since the same name usually means the same thing - the
// union is that one resource and nothing changes. Where they matched different
// ones, waiting for both is the conservative reading, and the same one the
// protocol already takes for a requirement that matches several resources with
// no name set.
func mergeRequiredResources(into, from map[string]*fnv1.Resources) {
	for name, res := range from {
		existing, ok := into[name]
		if !ok {
			into[name] = res
			continue
		}

		// slices.Concat rather than append: it always allocates, where
		// appending in place could write into the spare capacity of a slice
		// still owned by a function's request.
		items := slices.Concat(existing.GetItems(), res.GetItems())

		into[name] = &fnv1.Resources{Items: items}
	}
}

// blockedReport says how one resource the graph held back should be reported.
//
// A resource that is merely waiting becomes a line in this reconcile's summary
// event, and keeps whatever readiness the pipeline gave it - it may well be
// ready, just not yet allowed to change. A deadlocked one is different in both
// respects. Waiting cannot resolve it, so it is rare and actionable enough to
// warrant its own warning rather than a line in a summary; and it can never
// reach desired state, so counting it towards the XR's readiness would report
// an XR that is Available and permanently stuck.
//
// Both the apply-side and delete-side gates report through here. They used to
// carry their own copies of this rule, and the copies drifted the moment a
// decision moved from one side to the other.
func blockedReport(name string, b ordering.Decision, ready bool) (summary string, stillReady bool, warning error) {
	if b.Deadlocked {
		return "", false, errors.Errorf("composed resource %q %s; the pipeline's desired state and its dependencies contradict each other", name, b.Reason)
	}

	return fmt.Sprintf("%s %s", name, b.Reason), ready, nil
}

// blockedMessage summarizes the resources ordering held back this reconcile.
//
// It carries how long the function pipeline took, because that's what
// distinguishes a slow pipeline from a reconcile that wasn't triggered. The
// interval between two of these events is the ordering wave time; if that
// interval is far larger than the pipeline duration, the delay is in being
// woken up rather than in doing the work.
func blockedMessage(blocked []string, pipeline time.Duration) string {
	sort.Strings(blocked)

	const listed = 5

	shown := blocked
	suffix := ""

	if len(blocked) > listed {
		shown = blocked[:listed]
		suffix = fmt.Sprintf(" (and %d more)", len(blocked)-listed)
	}

	return fmt.Sprintf("Ordering is holding back %d composed resource(s): %s%s. Function pipeline took %s.",
		len(blocked), strings.Join(shown, "; "), suffix, pipeline.Round(time.Millisecond))
}
