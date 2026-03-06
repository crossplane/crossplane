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

// Package controller configures controller options.
package controller

import (
	"crypto/tls"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
)

// DefaultOptions returns a functional set of options with conservative
// defaults.
func DefaultOptions() Options {
	return Options{
		Logger:                  logging.NewNopLogger(),
		GlobalRateLimiter:       ratelimiter.NewGlobal(1),
		PollInterval:            1 * time.Minute,
		MaxConcurrentReconciles: 1,
		Features:                &feature.Flags{},
		EventFilterFunctions:    []event.FilterFn{},
	}
}

// Options frequently used by most Crossplane controllers.
type Options struct {
	// The Logger controllers should use.
	Logger logging.Logger

	// The GlobalRateLimiter used by this controller manager. The rate of
	// reconciles across all controllers will be subject to this limit.
	GlobalRateLimiter ratelimiter.RateLimiter

	// PollInterval at which each controller should speculatively poll to
	// determine whether it has work to do.
	PollInterval time.Duration

	// MaxConcurrentReconciles for each controller.
	MaxConcurrentReconciles int

	// Features that should be enabled.
	Features *feature.Flags

	// ESSOptions for External Secret Stores.
	ESSOptions *ESSOptions

	// MetricOptions for recording metrics.
	MetricOptions *MetricOptions

	// ChangeLogOptions for recording change logs.
	ChangeLogOptions *ChangeLogOptions

	// Gate implements a gated function callback pattern.
	Gate Gate

	// EventFilterFunctions used to filter events emitted by the controllers.
	EventFilterFunctions []event.FilterFn
}

// ForControllerRuntime extracts options for controller-runtime.
func (o Options) ForControllerRuntime() controller.Options {
	recoverPanic := true

	return controller.Options{
		MaxConcurrentReconciles: o.MaxConcurrentReconciles,
		RateLimiter:             ratelimiter.NewController(),
		RecoverPanic:            &recoverPanic,
	}
}

// ESSOptions for External Secret Stores.
type ESSOptions struct {
	TLSConfig     *tls.Config
	TLSSecretName *string
}

// MetricOptions for recording metrics.
type MetricOptions struct {
	// PollStateMetricInterval at which each controller should record state
	PollStateMetricInterval time.Duration

	// MetricsRecorder to use for recording metrics.
	MRMetrics managed.MetricRecorder

	// MRStateMetrics to use for recording state metrics.
	MRStateMetrics *statemetrics.MRStateMetrics
}

// ChangeLogOptions for recording changes to managed resources into the change
// logs.
type ChangeLogOptions struct {
	ChangeLogger managed.ChangeLogger
}
