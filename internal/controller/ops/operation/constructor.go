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

// Package operation implements day two operations.
package operation

import (
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	opscontroller "github.com/crossplane/crossplane/internal/controller/ops/controller"
	"github.com/crossplane/crossplane/internal/xfn"
)

// Setup adds a controller that reconciles Usages by
// defining a composite resource and starting a controller to reconcile it.
func Setup(mgr ctrl.Manager, o opscontroller.Options) error {
	name := "ops/" + strings.ToLower(v1alpha1.OperationGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithFunctionRunner(o.FunctionRunner))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Operation{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// ReconcilerOption is used to configure the Reconciler.
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

// WithFunctionRunner specifies how the Reconciler should run functions.
func WithFunctionRunner(fr xfn.FunctionRunner) ReconcilerOption {
	return func(r *Reconciler) {
		r.pipeline = fr
	}
}

// WithCapabilityChecker specifies how the Reconciler should check function capabilities.
func WithCapabilityChecker(cc xfn.CapabilityChecker) ReconcilerOption {
	return func(r *Reconciler) {
		r.functions = cc
	}
}

// WithRequiredResourcesFetcher specifies how the Reconciler should fetch required resources.
func WithRequiredResourcesFetcher(f xfn.RequiredResourcesFetcher) ReconcilerOption {
	return func(r *Reconciler) {
		r.resources = f
	}
}

// NewReconciler returns a Reconciler of Usages.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		functions:  xfn.NewRevisionCapabilityChecker(mgr.GetClient()),
		resources:  xfn.NewExistingRequiredResourcesFetcher(mgr.GetClient()),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}
