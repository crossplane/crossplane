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

package gate_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/gate"
)

func TestGateRegister(t *testing.T) {
	type args struct {
		depends []string
	}

	type want struct {
		called bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoDependencies": {
			reason: "Should immediately call function when no dependencies are required",
			args: args{
				depends: []string{},
			},
			want: want{
				called: true,
			},
		},
		"SingleDependency": {
			reason: "Should not call function when dependency is not met",
			args: args{
				depends: []string{"condition1"},
			},
			want: want{
				called: false,
			},
		},
		"MultipleDependencies": {
			reason: "Should not call function when multiple dependencies are not met",
			args: args{
				depends: []string{"condition1", "condition2"},
			},
			want: want{
				called: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := new(gate.Gate[string])

			called := false

			g.Register(func() {
				called = true
			}, tc.args.depends...)

			// Give some time for goroutine to execute
			time.Sleep(10 * time.Millisecond)

			if diff := cmp.Diff(tc.want.called, called); diff != "" {
				t.Errorf("\n%s\nRegister(...): -want called, +got called:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGateIntegration(t *testing.T) {
	type want struct {
		called bool
	}

	cases := map[string]struct {
		reason string
		setup  func(g *gate.Gate[string]) chan bool
		want   want
	}{
		"SingleDependencyMet": {
			reason: "Should call function when single dependency is met",
			setup: func(g *gate.Gate[string]) chan bool {
				called := make(chan bool, 1)

				g.Register(func() {
					called <- true
				}, "condition1")

				// Set condition to true (will be initialized as false first)
				g.Set("condition1", true)

				return called
			},
			want: want{
				called: true,
			},
		},
		"MultipleDependenciesMet": {
			reason: "Should call function when all dependencies are met",
			setup: func(g *gate.Gate[string]) chan bool {
				called := make(chan bool, 1)

				g.Register(func() {
					called <- true
				}, "condition1", "condition2")

				// Set both conditions to true
				g.Set("condition1", true)
				g.Set("condition2", true)

				return called
			},
			want: want{
				called: true,
			},
		},
		"PartialDependenciesMet": {
			reason: "Should not call function when only some dependencies are met",
			setup: func(g *gate.Gate[string]) chan bool {
				called := make(chan bool, 1)

				g.Register(func() {
					called <- true
				}, "condition1", "condition2")

				// Set only one condition to true
				g.Set("condition1", true)

				return called
			},
			want: want{
				called: false,
			},
		},
		"DependenciesAlreadyMet": {
			reason: "Should call function when dependencies are already met",
			setup: func(g *gate.Gate[string]) chan bool {
				called := make(chan bool, 1)

				g.Set("condition1", true)
				g.Set("condition2", true)

				g.Register(func() {
					called <- true
				}, "condition1", "condition2")

				return called
			},
			want: want{
				called: true,
			},
		},
		"DependencySetThenUnset": {
			reason: "Should call function when dependency is met, even if unset later",
			setup: func(g *gate.Gate[string]) chan bool {
				called := make(chan bool, 1)

				g.Register(func() {
					called <- true
				}, "condition1")

				// Set condition to true then false (function already called when true)
				g.Set("condition1", true)
				g.Set("condition1", false)

				return called
			},
			want: want{
				called: true,
			},
		},
		"FunctionCalledOnlyOnce": {
			reason: "Should call function only once even if conditions change after",
			setup: func(g *gate.Gate[string]) chan bool {
				called := make(chan bool, 2) // Buffer for potential multiple calls

				g.Register(func() {
					called <- true
				}, "condition1")

				// Set condition multiple times
				g.Set("condition1", true)
				g.Set("condition1", false)
				g.Set("condition1", true)

				return called
			},
			want: want{
				called: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := new(gate.Gate[string])

			callChannel := tc.setup(g)

			var got bool
			select {
			case got = <-callChannel:
			case <-time.After(100 * time.Millisecond):
				got = false
			}

			if diff := cmp.Diff(tc.want.called, got); diff != "" {
				t.Errorf("\n%s\nIntegration test: -want called, +got called:\n%s", tc.reason, diff)
			}

			// For the "only once" test, ensure no additional calls
			if name == "FunctionCalledOnlyOnce" && tc.want.called {
				select {
				case <-callChannel:
					t.Errorf("\n%s\nFunction was called more than once", tc.reason)
				case <-time.After(50 * time.Millisecond):
					// Good - no additional calls
				}
			}
		})
	}
}

func TestGateConcurrency(t *testing.T) {
	g := new(gate.Gate[string])

	const numGoroutines = 100

	var wg sync.WaitGroup

	callCount := make(chan struct{}, numGoroutines)

	// Register functions concurrently
	for range numGoroutines {
		wg.Go(func() {
			g.Register(func() {
				callCount <- struct{}{}
			}, "shared-condition")
		})
	}

	// Wait for all registrations
	wg.Wait()

	// Set condition to true once
	g.Set("shared-condition", true)

	// Give some time for goroutines to execute
	time.Sleep(100 * time.Millisecond)

	// Count how many functions were called
	close(callCount)

	count := 0
	for range callCount {
		count++
	}

	if count != numGoroutines {
		t.Errorf("Expected %d function calls, got %d", numGoroutines, count)
	}
}

func TestGateTypeSafety(t *testing.T) {
	intGate := new(gate.Gate[int])

	called := false

	intGate.Register(func() {
		called = true
	}, 1, 2, 3)

	intGate.Set(1, true)
	intGate.Set(2, true)
	intGate.Set(3, true)

	// Give some time for goroutine to execute
	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("Function should have been called when all int conditions were met")
	}
}
