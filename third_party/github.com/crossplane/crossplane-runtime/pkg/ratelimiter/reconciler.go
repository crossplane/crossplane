/*
Copyright 2021 The Crossplane Authors.

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

package ratelimiter

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// A Reconciler rate limits an inner, wrapped Reconciler. Requests that are rate
// limited immediately return RequeueAfter: d without calling the wrapped
// Reconciler, where d is imposed by the rate limiter.
type Reconciler struct {
	name  string
	inner reconcile.Reconciler
	limit ratelimiter.RateLimiter

	limited  map[string]struct{}
	limitedL sync.RWMutex
}

// NewReconciler wraps the supplied Reconciler, ensuring requests are passed to
// it no more frequently than the supplied RateLimiter allows. Multiple uniquely
// named Reconcilers can share the same RateLimiter.
func NewReconciler(name string, r reconcile.Reconciler, l ratelimiter.RateLimiter) *Reconciler {
	return &Reconciler{name: name, inner: r, limit: l, limited: make(map[string]struct{})}
}

// Reconcile the supplied request subject to rate limiting.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	item := r.name + req.String()
	if d := r.when(req); d > 0 {
		return reconcile.Result{RequeueAfter: d}, nil
	}
	r.limit.Forget(item)
	return r.inner.Reconcile(ctx, req)
}

// when adapts the upstream rate limiter's 'When' method such that rate limited
// requests can call it again when they return and will be allowed to proceed
// immediately without being subject to further rate limiting. It is optimised
// for handling requests that have not been and will not be rate limited without
// blocking.
func (r *Reconciler) when(req reconcile.Request) time.Duration {
	item := r.name + req.String()

	r.limitedL.RLock()
	_, limited := r.limited[item]
	r.limitedL.RUnlock()

	// If we already rate limited this request we trust that it complied and
	// let it pass immediately.
	if limited {
		r.limitedL.Lock()
		delete(r.limited, item)
		r.limitedL.Unlock()
		return 0
	}

	d := r.limit.When(item)

	// Record that this request was rate limited so that we can let it
	// through immediately when it requeues after the supplied duration.
	if d != 0 {
		r.limitedL.Lock()
		r.limited[item] = struct{}{}
		r.limitedL.Unlock()
	}

	return d
}
