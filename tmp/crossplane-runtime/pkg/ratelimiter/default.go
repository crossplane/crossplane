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

// Package ratelimiter contains suggested default ratelimiters for Crossplane.
package ratelimiter

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewGlobal returns a token bucket rate limiter meant for limiting the number
// of average total requeues per second for all controllers registered with a
// controller manager. The bucket size (i.e. allowed burst) is rps * 10.
func NewGlobal(rps int) *BucketRateLimiter {
	return &workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(rps), rps*10)}
}

// ControllerRateLimiter to work with [sigs.k8s.io/controller-runtime/pkg/controller.Options].
type ControllerRateLimiter = workqueue.TypedRateLimiter[reconcile.Request]

// NewController returns a rate limiter that takes the maximum delay between the
// passed rate limiter and a per-item exponential backoff limiter. The
// exponential backoff limiter has a base delay of 1s and a maximum of 60s.
func NewController() ControllerRateLimiter {
	return workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](1*time.Second, 60*time.Second)
}

// LimitRESTConfig returns a copy of the supplied REST config with rate limits
// derived from the supplied rate of reconciles per second.
func LimitRESTConfig(cfg *rest.Config, rps int) *rest.Config {
	// The Kubernetes controller manager and controller-runtime controller
	// managers use 20qps with 30 burst. We default to 10 reconciles per
	// second so our defaults are designed to accommodate that.
	out := rest.CopyConfig(cfg)
	out.QPS = float32(rps * 5)
	out.Burst = rps * 10

	return out
}
