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

// Package gate contains a gated function callback registration implementation.
package gate

import (
	"slices"
	"sync"
)

// Gate implements a gated function callback registration with comparable conditions.
type Gate[T comparable] struct {
	mux       sync.RWMutex
	satisfied map[T]bool
	fns       []gated[T]
}

// gated is an internal tracking resource.
type gated[T comparable] struct {
	// fn is the function callback we will invoke when all the dependent conditions are true.
	fn func()
	// depends is the list of conditions this gated function is waiting on. This is an AND.
	depends []T
	// released means the gated function has been invoked and we can garbage collect this gated function.
	released bool
}

// Register a callback function that will be called when all the provided dependent conditions are true.
// After all conditions are true, the callback function is removed from the registration and will not be called again.
// Thread Safe.
func (g *Gate[T]) Register(fn func(), depends ...T) {
	g.mux.Lock()
	g.fns = append(g.fns, gated[T]{fn: fn, depends: depends})
	g.mux.Unlock()

	g.process()
}

// Set marks the associated condition to the given value. If the condition is already set as that value, then this is a
// no-op. Returns true if there was an update detected. Thread safe.
func (g *Gate[T]) Set(condition T, value bool) bool {
	g.mux.Lock()

	if g.satisfied == nil {
		g.satisfied = make(map[T]bool)
	}

	old, found := g.satisfied[condition]

	updated := false
	if !found || old != value {
		updated = true
		g.satisfied[condition] = value
	}
	// process() would also like to lock the mux, so we must unlock here directly and not use defer.
	g.mux.Unlock()

	if updated {
		g.process()
	}

	return updated
}

func (g *Gate[T]) process() {
	g.mux.Lock()
	defer g.mux.Unlock()

	for i := range g.fns {
		// release controls if we should release the function.
		release := true

		for _, dep := range g.fns[i].depends {
			if !g.satisfied[dep] {
				release = false
			}
		}

		if release {
			fn := g.fns[i].fn
			// mark the function released so we can garbage collect after we are done with the loop.
			g.fns[i].released = true
			// Need to capture a copy of fn or else we would be accessing a deleted member when the go routine runs.
			go fn()
		}
	}

	// garbage collect released functions.
	g.fns = slices.DeleteFunc(g.fns, func(a gated[T]) bool {
		return a.released
	})
}
