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
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A wave records what one simulated reconcile did.
type wave struct {
	Applied []string
	Deleted []string
}

// simulate runs Decide repeatedly, feeding each pass's outcome back into
// state, until nothing changes or maxPasses is reached. Applied resources
// become observed and ready on the next pass, which stands in for a provider
// reconciling them. Deleted resources disappear immediately.
//
// It returns one wave per pass that did something.
func simulate(g *Graph, s State, maxPasses int) []wave {
	waves := []wave{}

	for range maxPasses {
		d := g.Decide(s)
		if len(d.Apply) == 0 && len(d.Delete) == 0 {
			break
		}

		w := wave{}

		for _, n := range d.Apply {
			cs := s.Composed[n]
			if !cs.Observed || !cs.Ready {
				w.Applied = append(w.Applied, n)
			}

			cs.Observed = true
			cs.Ready = true
			s.Composed[n] = cs
		}

		for _, n := range d.Delete {
			w.Deleted = append(w.Deleted, n)
			delete(s.Composed, n)
		}

		if len(w.Applied) == 0 && len(w.Deleted) == 0 {
			break
		}

		waves = append(waves, w)
	}

	return waves
}

func TestSimulateCreate(t *testing.T) {
	// vpc <- subnet <- instance, plus a second subnet off the same vpc.
	g := New(
		on("subnet-a", "vpc"),
		on("subnet-b", "vpc"),
		on("instance", "subnet-a"),
	)

	s := State{Composed: map[string]ComposedState{
		"vpc":      pending,
		"subnet-a": pending,
		"subnet-b": pending,
		"instance": pending,
	}}

	want := []wave{
		{Applied: []string{"vpc"}},
		{Applied: []string{"subnet-a", "subnet-b"}},
		{Applied: []string{"instance"}},
	}

	got := simulate(g, s, 10)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("\nCreation releases one dependency level per reconcile, and independent resources go together.\nsimulate(...): -want, +got:\n%s", diff)
	}
}

func TestSimulateTeardown(t *testing.T) {
	// The worked example in the design doc, extended one level.
	g := New(
		on("subnet", "vpc"),
		on("instance", "subnet"),
	)

	// Everything exists; the pipeline wants none of it. This is what an XR
	// being deleted looks like once the pipeline runs during deletion.
	s := State{Composed: map[string]ComposedState{
		"vpc":      doomed,
		"subnet":   doomed,
		"instance": doomed,
	}}

	want := []wave{
		{Deleted: []string{"instance"}},
		{Deleted: []string{"subnet"}},
		{Deleted: []string{"vpc"}},
	}

	got := simulate(g, s, 10)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("\nTeardown is the exact reverse of creation, one level per reconcile.\nsimulate(...): -want, +got:\n%s", diff)
	}
}

func TestSimulateReplacement(t *testing.T) {
	// A create-before-destroy replacement: new must exist and be ready
	// before old is torn down.
	g := New(Edge{
		Resource:            "new",
		DependsOn:           Target{ComposedResource: "old"},
		CreateBeforeDestroy: true,
	})

	s := State{Composed: map[string]ComposedState{
		"old": doomed,
		"new": pending,
	}}

	want := []wave{
		{Applied: []string{"new"}},
		{Deleted: []string{"old"}},
	}

	got := simulate(g, s, 10)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("\nThe replacement is created first, then the predecessor is destroyed.\nsimulate(...): -want, +got:\n%s", diff)
	}
}

func TestSimulateMixedCreateAndTeardown(t *testing.T) {
	// One branch is being built up while another is being torn down in the
	// same reconcile. They should not interfere.
	g := New(
		on("subnet", "vpc"),
		on("old-instance", "old-subnet"),
	)

	s := State{Composed: map[string]ComposedState{
		"vpc":          pending,
		"subnet":       pending,
		"old-subnet":   doomed,
		"old-instance": doomed,
	}}

	want := []wave{
		{Applied: []string{"vpc"}, Deleted: []string{"old-instance"}},
		{Applied: []string{"subnet"}, Deleted: []string{"old-subnet"}},
	}

	got := simulate(g, s, 10)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("\nIndependent create and delete chains make progress in the same pass.\nsimulate(...): -want, +got:\n%s", diff)
	}
}

func TestSimulateDeadlockTerminates(t *testing.T) {
	// The pipeline dropped the vpc but kept a subnet that depends on it.
	// Nothing can progress; the simulation must not spin.
	g := New(on("subnet", "vpc"))

	s := State{Composed: map[string]ComposedState{
		"vpc":    doomed,
		"subnet": live,
	}}

	if got := simulate(g, s, 10); len(got) != 0 {
		t.Errorf("\nA contradictory graph makes no progress rather than looping.\nsimulate(...): want no waves, got %v", got)
	}

	// The deadlock belongs to the vpc: it is the resource that cannot move.
	// The subnet already exists and keeps reconciling, because ordering gates
	// creation rather than updates.
	d := g.Decide(s)
	if !d.Blocked["vpc"].Deadlocked {
		t.Errorf("\nCrossplane needs to know the stall cannot resolve on its own.\nDecide(...): want vpc deadlocked, got %+v", d.Blocked["vpc"])
	}
}
