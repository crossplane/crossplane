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
//
// A lifecycle this Crossplane doesn't know is an error, not a default. A
// function built against a newer protocol can send one, and what it asks for
// cannot be guessed: treating it as ordinary ordering could delete something
// the function meant to keep alive, which is the outcome ordering exists to
// prevent. Better to refuse the graph and say so.
func AsEdges(deps []*fnv1.Dependency) ([]ordering.Edge, error) {
	out := make([]ordering.Edge, 0, len(deps))

	for _, d := range deps {
		l, err := asLifecycle(d.GetLifecycle())
		if err != nil {
			return nil, errors.Wrapf(err, "dependency from composed resource %q", d.GetResource())
		}

		e := ordering.Edge{
			Resource:  d.GetResource(),
			Lifecycle: l,
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

	return out, nil
}

// asLifecycle maps a protocol lifecycle onto the graph's own, which exists so
// the ordering package need not know about protobuf.
func asLifecycle(l fnv1.DependencyLifecycle) (ordering.Lifecycle, error) {
	switch l {
	case fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_UNSPECIFIED:
		return ordering.LifecycleSymmetric, nil
	case fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_CREATE_BEFORE_DESTROY:
		return ordering.LifecycleCreateBeforeDestroy, nil
	default:
		return ordering.LifecycleSymmetric, errors.Errorf("unsupported dependency lifecycle %q (%d)", l, int32(l))
	}
}

// AsOrderingState builds the state an ordering decision consumes.
//
// A composed resource's readiness is the pipeline's own verdict, taken from the
// final desired state. A required resource has no such verdict - no function
// reports readiness for a resource it didn't compose - so we read its Ready
// status condition. See isReady.
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

// isReady reports whether a required resource should count as ready.
//
// A resource with a Ready condition is ready when it's True. A resource with
// no Ready condition counts as ready once it exists. That's a heuristic, and
// it's checked per object, not per kind: Crossplane can't tell a kind that
// never has a Ready condition - a ConfigMap, a Secret - from an object that
// just hasn't been given one yet, like a managed resource its provider hasn't
// observed. The second can be released one reconcile early. Authors depending
// on such a resource should narrow the edge to it by name, and not lean on
// existence alone.
func isReady(cd *composed.Unstructured) bool {
	c := cd.GetCondition(xpv2.TypeReady)
	if c.Type == "" {
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

// AsDependencies converts ordering edges to protobuf dependencies. It's the
// inverse of AsEdges, used to bring edges rebuilt from an XR's references back
// alongside the ones a pipeline declared.
func AsDependencies(edges []ordering.Edge) []*fnv1.Dependency {
	out := make([]*fnv1.Dependency, 0, len(edges))

	for _, e := range edges {
		d := &fnv1.Dependency{Resource: e.Resource}

		switch t := e.DependsOn; {
		case t.RequiredResource != nil:
			d.DependsOn = &fnv1.Dependency_RequiredResource{RequiredResource: &fnv1.RequiredResourceDependency{
				RequirementName: t.RequiredResource.RequirementName,
				Name:            ptrIfSet(t.RequiredResource.Name),
				Namespace:       ptrIfSet(t.RequiredResource.Namespace),
			}}
		default:
			d.DependsOn = &fnv1.Dependency_ComposedResource{ComposedResource: t.ComposedResource}
		}

		if e.Lifecycle == ordering.LifecycleCreateBeforeDestroy {
			d.Lifecycle = fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_CREATE_BEFORE_DESTROY
		}

		out = append(out, d)
	}

	return out
}

func ptrIfSet(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// CarryForward returns the edges, from candidates, that should keep ordering
// this reconcile although the pipeline no longer declares them. candidates is
// the graph persisted by the last successful reconcile, plus every edge any
// step declared during this one.
//
// The rule is that history orders the exit, but never blocks the present. An
// edge is carried only when its dependent has left desired state but still
// exists, and what it depends on still exists too. A function that drops a
// resource very often drops that resource's edges with it - it no longer
// reasons about the resource at all - and that's exactly when the garbage
// collector needs them: the dependency must outlive its dependent. XR teardown
// is the same rule with nothing desired.
//
// An edge whose dependent is still desired is never carried. The pipeline
// declares what it wants now, and if it stopped declaring that edge it may
// have meant to - so carrying it could hold a resource back, or deadlock it,
// on the strength of a constraint nobody asserts any more. Those edges are
// returned as orphaned instead, keyed by the resource leaving desired state,
// so the composer can warn that a dependency it once had no longer protects
// it.
//
// Nor is any edge carried that would close a cycle with the declared edges or
// the edges carried before it, so carried edges can never fail validation.
func CarryForward(declared, candidates []*fnv1.Dependency, desired, observed ComposedResourceStates) ([]*fnv1.Dependency, map[string][]string) {
	have := make(map[dependencyIdentity]bool, len(declared))

	// What each composed resource depends on, to detect cycles.
	deps := map[string][]string{}

	for _, d := range declared {
		have[dependencyKey(d)] = true

		if t := d.GetComposedResource(); t != "" {
			deps[d.GetResource()] = append(deps[d.GetResource()], t)
		}
	}

	carried := []*fnv1.Dependency{}
	orphaned := map[string][]string{}

	for _, d := range candidates {
		// Only composed resources are deleted by Crossplane, so only edges
		// between them order an exit.
		r, t := d.GetResource(), d.GetComposedResource()
		if t == "" {
			continue
		}

		k := dependencyKey(d)
		if have[k] {
			continue
		}

		have[k] = true

		_, rDesired := desired[ResourceName(r)]
		_, rObserved := observed[ResourceName(r)]
		_, tDesired := desired[ResourceName(t)]
		_, tObserved := observed[ResourceName(t)]

		if !tObserved {
			continue
		}

		if rDesired {
			if !tDesired && !slices.Contains(orphaned[t], r) {
				orphaned[t] = append(orphaned[t], r)
			}

			continue
		}

		if !rObserved || reaches(deps, t, r) {
			continue
		}

		deps[r] = append(deps[r], t)
		carried = append(carried, d)
	}

	for t := range orphaned {
		slices.Sort(orphaned[t])
	}

	return carried, orphaned
}

// reaches reports whether from depends, directly or transitively, on to.
func reaches(deps map[string][]string, from, to string) bool {
	visited := map[string]bool{}
	stack := []string{from}

	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if n == to {
			return true
		}

		if visited[n] {
			continue
		}

		visited[n] = true
		stack = append(stack, deps[n]...)
	}

	return false
}

// attributeToSteps adds the pipeline steps that declared the edges behind a
// validation error to its message. The graph is validated once, after the whole
// pipeline has run, so without this an author learns that the pipeline's
// dependencies are invalid but not which function to look at.
func attributeToSteps(err error, declaredBy map[dependencyIdentity]string) error {
	var edges []ordering.Edge

	ee := &ordering.EdgeError{}
	ce := &ordering.CycleError{}

	switch {
	case errors.As(err, &ee):
		edges = []ordering.Edge{ee.Edge}
	case errors.As(err, &ce):
		edges = ce.Edges()
	default:
		return err
	}

	steps := []string{}

	for _, d := range AsDependencies(edges) {
		s, ok := declaredBy[dependencyKey(d)]
		if !ok || slices.Contains(steps, s) {
			continue
		}

		steps = append(steps, s)
	}

	switch len(steps) {
	case 0:
		return err
	case 1:
		return errors.Errorf("%w (declared by pipeline step %q)", err, steps[0])
	default:
		return errors.Errorf("%w (declared by pipeline steps %s)", err, quoted(steps))
	}
}

func quoted(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = fmt.Sprintf("%q", s)
	}

	return strings.Join(q, ", ")
}

// A dependencyIdentity identifies an edge by its endpoints, so it can be used
// as a map key to detect the same edge across two slices.
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
