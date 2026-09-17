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
	"fmt"
	"testing"
)

// These benchmarks exist to keep a pass over the graph linear in the size of
// the composition. They caught it when it wasn't: Decide used to scan the edge
// set for every resource, and again per dependent, which made a thousand
// resources with ten edges each cost 103ms a pass. See the "Cost" section on
// Decide.
//
// Four shapes, because the cost depends on the shape and not only the size:
//
//   - chain: n resources in a line, m = n. Depth without width.
//   - fanIn: each resource depends on the previous k, m = n*k. Width, which is
//     what used to square.
//   - layered: levels of ten, two dependencies each. What a large composition
//     of independent branches actually looks like.
//
// Teardown is benchmarked separately from creation because it is the more
// expensive side: blockedFromDelete considers every dependent of a resource,
// where blockedFromApply considers only its own edges.
//
// Note that fanIn varies both n and m together, because m = n*k. That is the
// realistic shape - edges arrive with the resources that declare them - but it
// means these numbers describe growth in composition size, not in n alone.

// chain builds n resources in a line: each depends on the one before it.
func chain(n int, cs ComposedState) (*Graph, State) {
	edges := make([]Edge, 0, n)
	composed := make(map[string]ComposedState, n)

	for i := range n {
		name := fmt.Sprintf("r%d", i)
		composed[name] = cs

		if i > 0 {
			edges = append(edges, on(name, fmt.Sprintf("r%d", i-1)))
		}
	}

	return New(edges...), State{Composed: composed}
}

// fanIn builds n resources where each depends on k earlier ones, giving n*k
// edges - the shape a wide composition with shared prerequisites produces.
func fanIn(n, k int, cs ComposedState) (*Graph, State) {
	edges := make([]Edge, 0, n*k)
	composed := make(map[string]ComposedState, n)

	for i := range n {
		name := fmt.Sprintf("r%d", i)
		composed[name] = cs

		for j := 1; j <= k && j <= i; j++ {
			edges = append(edges, on(name, fmt.Sprintf("r%d", i-j)))
		}
	}

	return New(edges...), State{Composed: composed}
}

func BenchmarkDecideChainCreate(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000, 2000} {
		g, s := chain(n, pending)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				g.Decide(s)
			}
		})
	}
}

func BenchmarkDecideChainTeardown(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000, 2000} {
		g, s := chain(n, doomed)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				g.Decide(s)
			}
		})
	}
}

func BenchmarkDecideFanInTeardown(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000} {
		g, s := fanIn(n, 10, doomed)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				g.Decide(s)
			}
		})
	}
}

// layered builds a DAG of n resources in levels of 10, each depending on two
// resources in the level above - roughly what a large composition looks like.
func layered(n int) ([]Edge, State) {
	edges := []Edge{}
	s := State{Composed: make(map[string]ComposedState, n)}

	for i := range n {
		name := fmt.Sprintf("r%04d", i)
		s.Composed[name] = ComposedState{Desired: true, Observed: true, Ready: true}

		if i >= 10 {
			edges = append(edges,
				Edge{Resource: name, DependsOn: Target{ComposedResource: fmt.Sprintf("r%04d", i-10)}},
				Edge{Resource: name, DependsOn: Target{ComposedResource: fmt.Sprintf("r%04d", i-9)}},
			)
		}
	}

	return edges, s
}

func BenchmarkReconcileGraph(b *testing.B) {
	for _, n := range []int{50, 100, 250, 500, 1000} {
		edges, s := layered(n)
		reqs := map[string]bool{}

		b.Run(fmt.Sprintf("N=%d/E=%d", n, len(edges)), func(b *testing.B) {
			for b.Loop() {
				g, _ := New(edges...).Prune(s)
				if err := g.Validate(s, reqs); err != nil {
					b.Fatal(err)
				}

				g.Decide(s)
			}
		})
	}
}

// Teardown is the other hot path: nothing desired, everything observed, so
// every node goes through blockedFromDelete - which is the more expensive side.
func BenchmarkTeardownGraph(b *testing.B) {
	for _, n := range []int{50, 100, 250, 500, 1000} {
		edges, s := layered(n)
		for k, v := range s.Composed {
			v.Desired = false
			s.Composed[k] = v
		}

		b.Run(fmt.Sprintf("N=%d/E=%d", n, len(edges)), func(b *testing.B) {
			for b.Loop() {
				g, _ := New(edges...).Prune(s)
				g.Decide(s)
			}
		})
	}
}
