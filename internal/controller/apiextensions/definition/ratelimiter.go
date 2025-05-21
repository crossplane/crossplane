/*
Copyright 2020 The Crossplane Authors.

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

package definition

import (
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

var _ workqueue.TypedRateLimiter[reconcile.Request] = &DebuggingRateLimiter[reconcile.Request]{}

// A DebuggingRateLimiter wraps a rate limiter with debugging information.
type DebuggingRateLimiter[T comparable] struct {
	name    string
	wrapped workqueue.TypedRateLimiter[T]
	log     logging.Logger
}

// NewDebuggingRateLimiter wraps a rate limiter with debug logging.
func NewDebuggingRateLimiter[T comparable](name string, wrapped workqueue.TypedRateLimiter[T], log logging.Logger) *DebuggingRateLimiter[T] {
	return &DebuggingRateLimiter[T]{name: name, wrapped: wrapped, log: log}
}

// When returns the duration after which the item can be requeued.
func (r *DebuggingRateLimiter[T]) When(item T) time.Duration {
	tries := r.wrapped.NumRequeues(item)
	when := r.wrapped.When(item)

	if when > 0 {
		r.log.Debug("Rate limiting item", "rate-limiter", r.name, "item", item, "requeues", tries, "requeue-after", when)
	}

	return when
}

// NumRequeues returns the number of times the item has been requeued.
func (r *DebuggingRateLimiter[T]) NumRequeues(item T) int {
	return r.wrapped.NumRequeues(item)
}

// Forget the item.
func (r *DebuggingRateLimiter[T]) Forget(item T) {
	tries := r.wrapped.NumRequeues(item)
	if tries > 0 {
		r.log.Debug("Forgetting rate limited item", "rate-limiter", r.name, "item", item)
	}
	r.wrapped.Forget(item)
}
