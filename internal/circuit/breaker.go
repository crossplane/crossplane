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

// Package circuit provides circuit breaker functionality for Crossplane controllers.
// It helps prevent tight reconciliation loops when controllers fight over resource state.
package circuit

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// EventType indicates how a circuit breaker handled an event.
type EventType string

const (
	// EventAllowed indicates the event was processed while the circuit was closed.
	EventAllowed EventType = "Allowed"

	// EventDropped indicates the event was dropped while the circuit was open.
	EventDropped EventType = "Dropped"

	// EventHalfOpenAllowed indicates the event was processed while probing a
	// half-open circuit.
	EventHalfOpenAllowed EventType = "HalfOpenAllowed"
)

// Breaker tracks reconciliation events and opens when thresholds are exceeded.
type Breaker interface {
	// GetState returns the current circuit breaker state for a target resource.
	GetState(ctx context.Context, target types.NamespacedName) State

	// RecordEvent records a reconciliation event triggered by a watched resource.
	// The eventType indicates how the circuit breaker handled the event.
	RecordEvent(ctx context.Context, target types.NamespacedName, es EventSource, et EventType)
}

// Metrics records circuit breaker transitions and event outcomes.
type Metrics interface {
	// IncOpen records that the circuit opened for the supplied controller.
	IncOpen(controller string)

	// IncClose records that the circuit closed for the supplied controller.
	IncClose(controller string)

	// IncEvent records the outcome of a circuit-controlled event.
	IncEvent(controller, result string)
}

// EventSource identifies the watched resource that triggered a reconciliation.
type EventSource struct {
	// GVK is the GroupVersionKind of the watched resource that triggered the event.
	GVK schema.GroupVersionKind

	// Name is the name of the watched resource that triggered the event.
	Name string

	// Namespace is the namespace of the watched resource that triggered the event.
	// Empty for cluster-scoped resources.
	Namespace string
}

// String returns a human-readable representation of the watched resource.
func (es EventSource) String() string {
	if es.Namespace == "" {
		return fmt.Sprintf("%s/%s", es.GVK.Kind, es.Name)
	}
	return fmt.Sprintf("%s/%s (%s)", es.GVK.Kind, es.Name, es.Namespace)
}

// State represents the current circuit breaker state for a target.
type State struct {
	// IsOpen indicates whether the circuit breaker is currently open.
	IsOpen bool

	// NextAllowedAt is when the next request can be allowed in half-open state.
	NextAllowedAt time.Time

	// TriggeredBy is the most frequently seen watched resource when the circuit opened.
	TriggeredBy string
}

// NopBreaker is a no-op implementation of Breaker that never opens.
type NopBreaker struct{}

// GetState always returns a closed circuit.
func (n *NopBreaker) GetState(_ context.Context, _ types.NamespacedName) State {
	return State{IsOpen: false}
}

// RecordEvent does nothing.
func (n *NopBreaker) RecordEvent(_ context.Context, _ types.NamespacedName, _ EventSource, _ EventType) {
}
