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
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
	"github.com/crossplane/crossplane/v2/internal/xpkg/dependency"
)

const (
	// Default namespace for package operations.
	defaultNamespace = "crossplane-system"

	// Default maximum concurrent package establishers.
	defaultMaxConcurrentEstablishers = 10
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

// WithValidators specifies a chain of validators for the Reconciler.
// Validators run in sequence, failing fast on the first error.
func WithValidators(validators ...Validator) ReconcilerOption {
	return func(r *Reconciler) {
		r.validator = ValidatorChain(validators)
	}
}

// WithInstaller specifies how the Reconciler should install packages.
func WithInstaller(i Installer) ReconcilerOption {
	return func(r *Reconciler) {
		r.installer = i
	}
}

// NewReconciler returns a Reconciler of Transactions.
func NewReconciler(mgr manager.Manager, pkg xpkg.Client, opts ...ReconcilerOption) *Reconciler {
	c := mgr.GetClient()
	e := revision.NewAPIEstablisher(c, defaultNamespace, defaultMaxConcurrentEstablishers)

	r := &Reconciler{
		client:     c,
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		lock:       NewAtomicLockManager(c),
		solver:     dependency.NewTighteningConstraintSolver(pkg),
		validator: ValidatorChain{
			// TODO(negz): Validate RBAC, etc.
			NewSchemaValidator(c, pkg),
		},
		installer: NewPackageInstaller(c, pkg, e),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Setup adds a controller that reconciles Transactions.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "pkg/" + strings.ToLower(v1alpha1.TransactionGroupKind)

	r := NewReconciler(mgr, o.Client,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Transaction{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1beta1.Lock{}, EnqueueIncompleteTransactionsForLock(mgr.GetClient(), o.Logger)).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}
