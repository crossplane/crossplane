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

// Package revision implements the CompositionRevision controller.
package revision

import (
	"context"
	"strings"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/xfn"
)

const (
	timeout = 2 * time.Minute
)

// Event reasons.
const (
	reasonCheckCapabilities event.Reason = "CheckCapabilities"
)

// Setup adds a controller that reconciles CompositionRevisions by validating
// that all functions in the pipeline have the required composition capability.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "revision/" + strings.ToLower(v1.CompositionRevisionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithCapabilityChecker(xfn.NewRevisionCapabilityChecker(mgr.GetClient())))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.CompositionRevision{}).
		Watches(&pkgv1.FunctionRevision{}, EnqueueCompositionRevisionsForFunctionRevision(mgr.GetClient(), o.Logger)).
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

// WithCapabilityChecker specifies the CapabilityChecker the Reconciler should use.
func WithCapabilityChecker(cc xfn.CapabilityChecker) ReconcilerOption {
	return func(r *Reconciler) {
		r.functions = cc
	}
}

// NewReconciler returns a Reconciler of CompositionRevisions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		functions:  xfn.NewRevisionCapabilityChecker(mgr.GetClient()),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// A Reconciler reconciles CompositionRevisions by validating that all functions
// in the pipeline have the required composition capability.
type Reconciler struct {
	client client.Client

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager
	functions  xfn.CapabilityChecker
}

// Reconcile a CompositionRevision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rev := &v1.CompositionRevision{}
	if err := r.client.Get(ctx, req.NamespacedName, rev); err != nil {
		log.Debug("Cannot get CompositionRevision", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get CompositionRevision")
	}

	status := r.conditions.For(rev)

	if meta.WasDeleted(rev) {
		return reconcile.Result{}, nil
	}

	log = log.WithValues(
		"uid", rev.GetUID(),
		"version", rev.GetResourceVersion(),
		"name", rev.GetName(),
		"revision", rev.Spec.Revision,
	)

	// Extract function names from the pipeline
	names := make([]string, 0, len(rev.Spec.Pipeline))
	for _, fn := range rev.Spec.Pipeline {
		names = append(names, fn.FunctionRef.Name)
	}

	// Check that all functions have the composition capability
	if err := r.functions.CheckCapabilities(ctx, []string{pkgmetav1.FunctionCapabilityComposition}, names...); err != nil {
		log.Debug("Function capability check failed", "error", err)
		r.record.Event(rev, event.Warning(reasonCheckCapabilities, err))
		status.MarkConditions(xpv1.ReconcileSuccess(), v1.MissingCapabilities(err.Error()))

		// Update status but don't return the error - capability failures are informational
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, rev), "cannot update CompositionRevision status")
	}

	log.Debug("All functions have required composition capability")
	r.record.Event(rev, event.Normal(reasonCheckCapabilities, "All functions have required composition capability"))
	status.MarkConditions(xpv1.ReconcileSuccess(), v1.ValidPipeline())

	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, rev), "cannot update CompositionRevision status")
}
