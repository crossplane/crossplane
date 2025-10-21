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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewMapFunc wraps a handler.MapFunc with circuit breaker functionality.
// It records events for each target resource and filters out requests when the
// circuit breaker is open, allowing occasional requests through in half-open state.
func NewMapFunc(wrapped handler.MapFunc, breaker Breaker, m Metrics, controller string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		if m == nil {
			m = &NopMetrics{}
		}
		// Get the original requests
		requests := wrapped(ctx, obj)

		recordEvent := func(result string) {
			m.IncEvent(controller, result)
		}

		// Record events for each target resource
		source := EventSource{
			GVK:       obj.GetObjectKind().GroupVersionKind(),
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}

		// Filter out requests for resources with open circuit breakers
		keep := make([]reconcile.Request, 0, len(requests))
		for _, req := range requests {
			// If object is marked for deletion, always allow the
			// event through without affecting circuit breaker
			// timing. This ensures deletion events (which are
			// MODIFIED events with deletionTimestamp set) can reach
			// the reconciler to remove finalizers.
			if obj.GetDeletionTimestamp() != nil {
				recordEvent(CircuitBreakerResultAllowed)
				keep = append(keep, req)
				continue
			}

			// Always record the event for tracking
			breaker.RecordEvent(ctx, req.NamespacedName, source)

			// Get current state
			state := breaker.GetState(ctx, req.NamespacedName)

			// If breaker is closed, allow the request
			if !state.IsOpen {
				keep = append(keep, req)
				recordEvent(CircuitBreakerResultAllowed)
				continue
			}

			// Breaker is open - check if we should allow in half-open state
			if time.Now().After(state.NextAllowedAt) {
				keep = append(keep, req)
				breaker.RecordAllowed(ctx, req.NamespacedName)
				recordEvent(CircuitBreakerResultHalfOpenAllowed)
				continue
			}
			// Otherwise filter out - fully open state
			recordEvent(CircuitBreakerResultDropped)
		}

		return keep
	}
}
