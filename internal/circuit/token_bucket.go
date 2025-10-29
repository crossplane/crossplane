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
	"math"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// Config controls circuit breaker behavior using a token bucket approach.
type Config struct {
	capacity            float64       // Token bucket capacity (burst allowance)
	refillRatePerSecond float64       // Tokens per second refill rate
	cooldownTime        time.Duration // How long circuit stays open after opening
	halfOpenInterval    time.Duration // How often to allow requests when open
	expireAfter         time.Duration // How long to keep inactive target states
}

// Option configures a circuit breaker.
type Option func(*Config)

// WithBurst sets the token bucket burst allowance.
func WithBurst(b float64) Option {
	return func(c *Config) {
		c.capacity = b
	}
}

// WithRefillRatePerSecond sets the token bucket refill rate.
func WithRefillRatePerSecond(r float64) Option {
	return func(c *Config) {
		c.refillRatePerSecond = r
	}
}

// WithOpenDuration sets how long the circuit stays open before auto-closing.
func WithOpenDuration(d time.Duration) Option {
	return func(c *Config) {
		c.cooldownTime = d
	}
}

// WithHalfOpenInterval sets how often to allow requests in half-open state.
func WithHalfOpenInterval(i time.Duration) Option {
	return func(c *Config) {
		c.halfOpenInterval = i
	}
}

// WithGarbageCollectTargetsAfter sets how long to keep inactive target states
// before garbage collection.
func WithGarbageCollectTargetsAfter(d time.Duration) Option {
	return func(c *Config) {
		c.expireAfter = d
	}
}

// TokenBucketBreaker is a concrete implementation of the Breaker interface that uses
// a token bucket approach to rate limit reconciliation events.
type TokenBucketBreaker struct {
	config     Config
	mu         sync.RWMutex
	targets    map[types.NamespacedName]*state
	controller string
	metrics    Metrics
}

// state tracks the circuit breaker state for a single target resource.
type state struct {
	mu sync.RWMutex

	// Token bucket for rate limiting.
	tokens     float64
	lastRefill time.Time

	// Ring buffer for source tracking.
	recentSources [16]string
	recentIdx     int

	// Circuit state.
	isOpen        bool
	openedAt      time.Time
	lastAllowed   time.Time
	triggerSource string // Most frequent watched resource when circuit opened
}

// NewTokenBucketBreaker creates a new token bucket-based circuit breaker.
func NewTokenBucketBreaker(m Metrics, controller string, opts ...Option) *TokenBucketBreaker {
	config := Config{
		capacity:            50.0,             // Allow 50-event burst.
		refillRatePerSecond: 0.5,              // Allow 1 every 2s sustained.
		cooldownTime:        5 * time.Minute,  // Circuit stays open for 5 minutes.
		halfOpenInterval:    30 * time.Second, // Allow probe every 30s when open.
		expireAfter:         24 * time.Hour,   // Clean up targets after 24 hours.
	}

	for _, opt := range opts {
		opt(&config)
	}

	if m == nil {
		m = &NopMetrics{}
	}

	b := &TokenBucketBreaker{
		config:     config,
		targets:    make(map[types.NamespacedName]*state),
		metrics:    m,
		controller: controller,
	}

	return b
}

// RecordEvent records a reconciliation event for the target resource.
func (b *TokenBucketBreaker) RecordEvent(_ context.Context, target types.NamespacedName, source EventSource) {
	b.mu.Lock()

	now := time.Now()

	if b.targets[target] == nil {
		// Garbage collect stale targets when adding new ones
		for t, s := range b.targets {
			s.mu.RLock()
			shouldDelete := now.Sub(s.lastRefill) > b.config.expireAfter
			s.mu.RUnlock()

			if shouldDelete {
				delete(b.targets, t)
			}
		}

		b.targets[target] = &state{
			tokens:     b.config.capacity, // Start with full bucket
			lastRefill: now,
		}
	}
	state := b.targets[target]
	b.mu.Unlock()

	state.mu.Lock()
	defer state.mu.Unlock()

	// Refill tokens based on elapsed time
	elapsed := now.Sub(state.lastRefill).Seconds()
	state.tokens = math.Min(b.config.capacity, state.tokens+b.config.refillRatePerSecond*elapsed)
	state.lastRefill = now

	// Add source to ring buffer
	state.recentSources[state.recentIdx] = source.String()
	state.recentIdx = (state.recentIdx + 1) % len(state.recentSources)

	if state.isOpen {
		// Circuit is open and cooldown time hasn't expired yet.
		if now.Sub(state.openedAt) < b.config.cooldownTime {
			return
		}

		// Cooldown period has expired. Close the circuit.
		state.isOpen = false
		b.observeClose()

		// Clear ring buffer on close
		for i := range state.recentSources {
			state.recentSources[i] = ""
		}
		state.recentIdx = 0
	}

	// If there's a token available, consume it.
	if state.tokens >= 1.0 {
		state.tokens -= 1.0
		return
	}

	// No tokens available - open circuit.
	state.isOpen = true
	state.openedAt = now
	state.lastAllowed = now
	b.observeOpen()

	// Analyze ring buffer to find most frequent source
	events := make(map[string]int)
	maxEvents := 0

	for _, src := range state.recentSources {
		if src == "" {
			continue
		}
		events[src]++
		if events[src] < maxEvents {
			continue
		}
		maxEvents = events[src]
		state.triggerSource = src
	}
}

// GetState returns the current circuit breaker state for the target resource.
func (b *TokenBucketBreaker) GetState(_ context.Context, target types.NamespacedName) State {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state := b.targets[target]
	if state == nil {
		return State{IsOpen: false}
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	if !state.isOpen {
		return State{IsOpen: false}
	}

	return State{
		IsOpen:        state.isOpen,
		TriggeredBy:   state.triggerSource,
		NextAllowedAt: state.lastAllowed.Add(b.config.halfOpenInterval),
	}
}

// RecordAllowed updates the last reconcile time for half-open state tracking.
func (b *TokenBucketBreaker) RecordAllowed(_ context.Context, target types.NamespacedName) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state := b.targets[target]
	if state == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	state.lastAllowed = time.Now()
}

func (b *TokenBucketBreaker) observeOpen() {
	b.metrics.IncOpen(b.controller)
}

func (b *TokenBucketBreaker) observeClose() {
	b.metrics.IncClose(b.controller)
}
