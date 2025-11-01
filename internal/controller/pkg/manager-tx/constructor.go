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

// Package manager implements transaction-aware package controllers.
package manager

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

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/controller"
)

// SetupProvider adds a controller that reconciles Providers by creating Transactions.
func SetupProvider(mgr ctrl.Manager, o controller.Options) error {
	name := "pkg/" + strings.ToLower(v1.ProviderGroupKind)

	r := NewProviderReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Provider{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha1.Transaction{}, EnqueuePackageForTransaction(mgr.GetClient(), &v1.ProviderList{})).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackagesForImageConfig(mgr.GetClient(), &v1.ProviderList{})).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// SetupConfiguration adds a controller that reconciles Configurations by creating Transactions.
func SetupConfiguration(mgr ctrl.Manager, o controller.Options) error {
	name := "pkg/" + strings.ToLower(v1.ConfigurationGroupKind)

	r := NewConfigurationReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Configuration{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha1.Transaction{}, EnqueuePackageForTransaction(mgr.GetClient(), &v1.ConfigurationList{})).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackagesForImageConfig(mgr.GetClient(), &v1.ConfigurationList{})).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// SetupFunction adds a controller that reconciles Functions by creating Transactions.
func SetupFunction(mgr ctrl.Manager, o controller.Options) error {
	name := "pkg/" + strings.ToLower(v1.FunctionGroupKind)

	r := NewFunctionReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Function{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha1.Transaction{}, EnqueuePackageForTransaction(mgr.GetClient(), &v1.FunctionList{})).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackagesForImageConfig(mgr.GetClient(), &v1.FunctionList{})).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
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

// NewProviderReconciler returns a Reconciler that reconciles Providers by creating Transactions.
func NewProviderReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		pkg:        &v1.Provider{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// NewConfigurationReconciler returns a Reconciler that reconciles Configurations by creating Transactions.
func NewConfigurationReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		pkg:        &v1.Configuration{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// NewFunctionReconciler returns a Reconciler that reconciles Functions by creating Transactions.
func NewFunctionReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		pkg:        &v1.Function{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}
