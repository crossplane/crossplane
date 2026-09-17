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

// Decide scans every edge for every resource, so its cost grows with the
// product of the two: n composed resources and m edges, not one or the other.
// A composition with many resources and few edges is cheap, and so is one with
// few resources and many edges. These benchmarks say where the product starts
// to matter. See the "Cost" section on Decide for what to do about it.
//
// The teardown case is the expensive one: blockedFromDelete walks the
// dependents of each resource and asks createBeforeDestroy about each, and
// that question is itself a scan of every edge.
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
