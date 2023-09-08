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

package controller

import (
	"crypto/tls"
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
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
	}
}

// Options frequently used by most Crossplane controllers.
type Options struct {
	// The Logger controllers should use.
	Logger logging.Logger

	// The GlobalRateLimiter used by this controller manager. The rate of
	// reconciles across all controllers will be subject to this limit.
	GlobalRateLimiter workqueue.RateLimiter

	// PollInterval at which each controller should speculatively poll to
	// determine whether it has work to do.
	PollInterval time.Duration

	// MaxConcurrentReconciles for each controller.
	MaxConcurrentReconciles int

	// Features that should be enabled.
	Features *feature.Flags

	// ESSOptions for External Secret Stores.
	ESSOptions *ESSOptions
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
