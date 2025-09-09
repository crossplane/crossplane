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

package circuit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var _ Breaker = &TokenBucketBreaker{}

func TestTokenBucketBreakerRecordEvent(t *testing.T) {
	target := types.NamespacedName{Name: "test-xr", Namespace: "default"}
	source := EventSource{
		GVK:       schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
		Name:      "test-bucket",
		Namespace: "default",
	}

	type args struct {
		ctx    context.Context
		target types.NamespacedName
		source EventSource
	}

	type want struct {
		state State
	}

	cases := map[string]struct {
		reason  string
		breaker *TokenBucketBreaker
		setup   func(*TokenBucketBreaker)
		args    args
		want    want
	}{
		"EventWithinCapacity": {
			reason: "Recording an event within token capacity should keep circuit closed",
			breaker: NewTokenBucketBreaker(
				WithBurst(5),
				WithRefillRatePerSecond(1.0),
			),
			args: args{
				ctx:    context.Background(),
				target: target,
			},
			want: want{
				state: State{
					IsOpen: false,
				},
			},
		},
		"EventExceedsCapacity": {
			reason: "Recording events that exceed token capacity should open circuit",
			breaker: NewTokenBucketBreaker(
				WithBurst(2),
				WithRefillRatePerSecond(0.1),
				WithHalfOpenInterval(30*time.Second), // Explicit for test stability
			),
			setup: func(b *TokenBucketBreaker) {
				ctx := context.Background()

				// Consume all tokens
				b.RecordEvent(ctx, target, source)
				b.RecordEvent(ctx, target, source)
			},
			args: args{
				ctx:    context.Background(),
				target: target,
				source: source,
			},
			want: want{
				state: State{
					IsOpen:        true,
					TriggeredBy:   source.String(),
					NextAllowedAt: time.Now().Add(30 * time.Second),
				},
			},
		},
		"EventAfterCooldownWithTokens": {
			reason: "Recording an event after cooldown period with sufficient tokens should keep circuit closed",
			breaker: NewTokenBucketBreaker(
				WithBurst(2),
				WithRefillRatePerSecond(10), // Fast refill to ensure tokens are available
				WithOpenDuration(100*time.Millisecond),
				WithHalfOpenInterval(30*time.Second), // Explicit for test stability
			),
			setup: func(b *TokenBucketBreaker) {
				ctx := context.Background()
				// Consume tokens and open circuit
				b.RecordEvent(ctx, target, source)
				b.RecordEvent(ctx, target, source)
				b.RecordEvent(ctx, target, source) // This opens the circuit
				// Wait for cooldown and token refill
				time.Sleep(150 * time.Millisecond)
			},
			args: args{
				ctx:    context.Background(),
				target: target,
				source: source,
			},
			want: want{
				state: State{
					IsOpen: false,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(tc.breaker)
			}

			tc.breaker.RecordEvent(tc.args.ctx, tc.args.target, tc.args.source)
			got := tc.breaker.GetState(tc.args.ctx, tc.args.target)

			if diff := cmp.Diff(tc.want.state, got, cmpopts.EquateApproxTime(50*time.Millisecond)); diff != "" {
				t.Errorf("%s\nTokenBucketBreaker.RecordEvent(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTokenBucketBreakerGetState(t *testing.T) {
	target := types.NamespacedName{Name: "test-xr", Namespace: "default"}
	source := EventSource{
		GVK:       schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
		Name:      "test-bucket",
		Namespace: "default",
	}

	type args struct {
		ctx    context.Context
		target types.NamespacedName
	}

	type want struct {
		state State
	}

	cases := map[string]struct {
		reason  string
		breaker *TokenBucketBreaker
		setup   func(*TokenBucketBreaker)
		args    args
		want    want
	}{
		"UnknownTarget": {
			reason: "Getting state for unknown target should return closed circuit",
			breaker: NewTokenBucketBreaker(
				WithBurst(50.0),
				WithRefillRatePerSecond(0.5),
				WithOpenDuration(5*time.Minute),
				WithHalfOpenInterval(30*time.Second),
				WithGarbageCollectTargetsAfter(24*time.Hour),
			),
			args: args{
				ctx:    context.Background(),
				target: target,
			},
			want: want{
				state: State{
					IsOpen: false,
				},
			},
		},
		"ClosedCircuit": {
			reason: "Getting state for target with closed circuit should return correct state",
			breaker: NewTokenBucketBreaker(
				WithBurst(50.0),
				WithRefillRatePerSecond(0.5),
				WithOpenDuration(5*time.Minute),
				WithHalfOpenInterval(30*time.Second),
				WithGarbageCollectTargetsAfter(24*time.Hour),
			),
			setup: func(b *TokenBucketBreaker) {
				ctx := context.Background()
				b.RecordEvent(ctx, target, source)
			},
			args: args{
				ctx:    context.Background(),
				target: target,
			},
			want: want{
				state: State{
					IsOpen: false,
				},
			},
		},
		"OpenCircuit": {
			reason: "Getting state for target with open circuit should return correct state",
			breaker: NewTokenBucketBreaker(
				WithBurst(1),
				WithRefillRatePerSecond(0.1),
				WithHalfOpenInterval(30*time.Second), // Explicit for test stability
			),
			setup: func(b *TokenBucketBreaker) {
				ctx := context.Background()
				// Open circuit
				b.RecordEvent(ctx, target, source)
				b.RecordEvent(ctx, target, source)
			},
			args: args{
				ctx:    context.Background(),
				target: target,
			},
			want: want{
				state: State{
					IsOpen:        true,
					TriggeredBy:   source.String(),
					NextAllowedAt: time.Now().Add(30 * time.Second),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(tc.breaker)
			}

			got := tc.breaker.GetState(tc.args.ctx, tc.args.target)

			if diff := cmp.Diff(tc.want.state, got, cmpopts.EquateApproxTime(50*time.Millisecond)); diff != "" {
				t.Errorf("%s\nTokenBucketBreaker.GetState(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTokenBucketBreakerRecordAllowed(t *testing.T) {
	target := types.NamespacedName{Name: "test-xr", Namespace: "default"}
	source := EventSource{
		GVK:  schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
		Name: "test-bucket",
	}

	type args struct {
		ctx    context.Context
		target types.NamespacedName
	}

	type want struct {
		state State
	}

	cases := map[string]struct {
		reason  string
		breaker *TokenBucketBreaker
		setup   func(*TokenBucketBreaker)
		args    args
		want    want
	}{
		"UnknownTarget": {
			reason: "Recording allowed for unknown target should not panic",
			breaker: NewTokenBucketBreaker(
				WithBurst(50.0),
				WithRefillRatePerSecond(0.5),
				WithOpenDuration(5*time.Minute),
				WithHalfOpenInterval(30*time.Second),
				WithGarbageCollectTargetsAfter(24*time.Hour),
			),
			args: args{
				ctx:    context.Background(),
				target: types.NamespacedName{Name: "unknown", Namespace: "default"},
			},
			want: want{
				state: State{
					IsOpen: false,
				},
			},
		},
		"KnownTarget": {
			reason: "Recording allowed for known target should update last allowed time",
			breaker: NewTokenBucketBreaker(
				WithHalfOpenInterval(1 * time.Second),
			),
			setup: func(b *TokenBucketBreaker) {
				ctx := context.Background()
				b.RecordEvent(ctx, target, source)
			},
			args: args{
				ctx:    context.Background(),
				target: target,
			},
			want: want{
				state: State{
					IsOpen: false,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(tc.breaker)
			}

			tc.breaker.RecordAllowed(tc.args.ctx, tc.args.target)
			got := tc.breaker.GetState(tc.args.ctx, tc.args.target)

			if diff := cmp.Diff(tc.want.state, got, cmpopts.EquateApproxTime(50*time.Millisecond)); diff != "" {
				t.Errorf("%s\nTokenBucketBreaker.RecordAllowed(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTokenBucketBreakerSourceTracking(t *testing.T) {
	breaker := NewTokenBucketBreaker(
		WithBurst(1),
		WithRefillRatePerSecond(0.1),
		WithHalfOpenInterval(30*time.Second), // Explicit for test stability
	)

	ctx := context.Background()
	target := types.NamespacedName{Name: "test-xr", Namespace: "default"}

	// Record events from different sources
	sources := []EventSource{
		{
			GVK:       schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
			Name:      "bucket-1",
			Namespace: "default",
		},
		{
			GVK:       schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Database"},
			Name:      "db-1",
			Namespace: "default",
		},
		{
			GVK:       schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
			Name:      "bucket-1",
			Namespace: "default",
		},
	}

	// Consume token
	breaker.RecordEvent(ctx, target, sources[0])

	// Record events to fill ring buffer and trigger circuit
	for _, source := range sources {
		breaker.RecordEvent(ctx, target, source)
	}

	state := breaker.GetState(ctx, target)
	want := State{
		IsOpen:        true,
		NextAllowedAt: time.Now().Add(30 * time.Second),
		TriggeredBy:   "Bucket/bucket-1 (default)",
	}

	if diff := cmp.Diff(want, state, cmpopts.EquateApproxTime(50*time.Millisecond)); diff != "" {
		t.Errorf("Expected circuit to be open with correct trigger source: -want, +got:\n%s", diff)
	}
}

func TestTokenBucketBreakerTokenRefill(t *testing.T) {
	breaker := NewTokenBucketBreaker(
		WithBurst(2),
		WithRefillRatePerSecond(10),            // Fast refill for test
		WithOpenDuration(200*time.Millisecond), // Longer cooldown
		WithHalfOpenInterval(30*time.Second),   // Explicit for test stability
	)

	ctx := context.Background()
	target := types.NamespacedName{Name: "test-xr", Namespace: "default"}
	source := EventSource{
		GVK:  schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
		Name: "test-bucket",
	}

	// Consume all tokens
	breaker.RecordEvent(ctx, target, source)
	breaker.RecordEvent(ctx, target, source)

	// This should open the circuit
	breaker.RecordEvent(ctx, target, source)
	state := breaker.GetState(ctx, target)
	wantOpen := State{
		IsOpen:        true,
		NextAllowedAt: time.Now().Add(30 * time.Second),
		TriggeredBy:   source.String(),
	}
	if diff := cmp.Diff(wantOpen, state, cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
		t.Errorf("Expected circuit to be open after exhausting tokens: -want, +got:\n%s", diff)
	}

	// Wait for tokens to refill but not for cooldown to expire
	time.Sleep(100 * time.Millisecond)

	// Circuit should still be open due to cooldown, even though tokens refilled
	state = breaker.GetState(ctx, target)
	if diff := cmp.Diff(wantOpen, state, cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
		t.Errorf("Expected circuit to remain open during cooldown period: -want, +got:\n%s", diff)
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Record another event - tokens should have refilled enough to allow it
	breaker.RecordEvent(ctx, target, source)
	state = breaker.GetState(ctx, target)
	want := State{IsOpen: false}
	if diff := cmp.Diff(want, state, cmpopts.EquateApproxTime(50*time.Millisecond)); diff != "" {
		t.Errorf("Expected circuit to remain closed after cooldown and token refill: -want, +got:\n%s", diff)
	}
}

func TestTokenBucketBreakerConcurrency(t *testing.T) {
	breaker := NewTokenBucketBreaker(
		WithBurst(95), // Not quite enough for 10 goroutines making 10 requests in parallel.
		WithRefillRatePerSecond(1),
		WithHalfOpenInterval(30*time.Second), // Explicit for test stability
	)

	ctx := context.Background()
	target := types.NamespacedName{Name: "test-xr", Namespace: "default"}
	source := EventSource{
		GVK:  schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
		Name: "test-bucket",
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				breaker.RecordEvent(ctx, target, source)
				breaker.GetState(ctx, target)
				breaker.RecordAllowed(ctx, target)
			}
		}()
	}

	wg.Wait()

	// Should not panic and should have consistent state
	state := breaker.GetState(ctx, target)

	want := State{
		IsOpen:        true,
		NextAllowedAt: time.Now().Add(30 * time.Second),
		TriggeredBy:   source.String(),
	}
	if diff := cmp.Diff(want, state, cmpopts.EquateApproxTime(2*time.Second)); diff != "" {
		t.Errorf("Open circuit state mismatch (-want +got):\n%s", diff)
	}
}

// ExampleTokenBucketBreaker demonstrates circuit breaker behavior including
// triggering the breaker and half-open state management.
func ExampleTokenBucketBreaker() {
	// Create a circuit breaker with small capacity to easily trigger it
	breaker := NewTokenBucketBreaker(
		WithBurst(3),                    // Small capacity for demo
		WithRefillRatePerSecond(0.1),    // Slow refill
		WithOpenDuration(1*time.Minute), // Short cooldown for demo
		WithHalfOpenInterval(10*time.Second),
	)

	ctx := context.Background()
	target := types.NamespacedName{Name: "my-xr", Namespace: "default"}
	source := EventSource{
		GVK:       schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Bucket"},
		Name:      "my-bucket",
		Namespace: "default",
	}

	// Record events within capacity - circuit stays closed
	for range 3 {
		breaker.RecordEvent(ctx, target, source)
	}

	state := breaker.GetState(ctx, target)
	fmt.Printf("After 3 events - Open: %v\n", state.IsOpen)

	// This event will exhaust tokens and open the circuit
	breaker.RecordEvent(ctx, target, source)

	state = breaker.GetState(ctx, target)
	fmt.Printf("After 4th event - Open: %v, TriggeredBy: %s\n", state.IsOpen, state.TriggeredBy)

	// When circuit is open, RecordAllowed tracks when we allow requests through
	// (this would typically be called by the controller when it allows a reconcile)
	breaker.RecordAllowed(ctx, target)

	state = breaker.GetState(ctx, target)
	fmt.Printf("Half-open behavior - NextAllowed set: %v\n", !state.NextAllowedAt.IsZero())

	// Output:
	// After 3 events - Open: false
	// After 4th event - Open: true, TriggeredBy: Bucket/my-bucket (default)
	// Half-open behavior - NextAllowed set: true
}
