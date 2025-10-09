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

// Package transaction implements the Transaction controller.
package transaction

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record Kubernetes events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithLockManager specifies how the Reconciler should manage Lock access.
func WithLockManager(lm LockManager) ReconcilerOption {
	return func(r *Reconciler) {
		r.lock = lm
	}
}

// WithDependencySolver specifies how the Reconciler should resolve dependencies.
func WithDependencySolver(ds DependencySolver) ReconcilerOption {
	return func(r *Reconciler) {
		r.solver = ds
	}
}

// WithValidator specifies how the Reconciler should validate Transactions.
func WithValidator(v Validator) ReconcilerOption {
	return func(r *Reconciler) {
		r.validator = v
	}
}

// NewReconciler returns a Reconciler of Transactions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		lock:       NewAtomicLockManager(mgr.GetClient()),
		validator:  ValidatorChain{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}
