/*
Copyright 2019 The Crossplane Authors.

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

package requirement

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/requirement"
)

const (
	finalizer        = "finalizer.apiextensions.crossplane.io"
	reconcileTimeout = 1 * time.Minute

	aShortWait = 30 * time.Second
)

// Reasons a resource requirement is or is not ready.
const (
	ReasonWaiting = "Resource requirement is waiting for composite resource to become Ready"
)

// Error strings.
const (
	errGetRequirement          = "cannot get resource requirement"
	errUpdateRequirementStatus = "cannot update resource requirement status"
)

// Event reasons.
const (
	reasonBind      event.Reason = "BindCompositeResource"
	reasonUnbind    event.Reason = "UnbindCompositeResource"
	reasonConfigure event.Reason = "ConfigureCompositeResource"
	reasonPropagate event.Reason = "PropagateConnectionSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of resource requirement.
func ControllerName(name string) string {
	return "requirement/" + name
}

// A CompositeConfigurator configures a resource, typically by converting it to
// a known type and populating its spec.
type CompositeConfigurator interface {
	Configure(ctx context.Context, rq resource.Requirement, cp resource.Composite) error
}

// A CompositeConfiguratorFn is a function that satisfies the
// CompositeConfigurator interface.
type CompositeConfiguratorFn func(ctx context.Context, rq resource.Requirement, cp resource.Composite) error

// Configure the supplied resource using the supplied requirement and class.
func (fn CompositeConfiguratorFn) Configure(ctx context.Context, rq resource.Requirement, cp resource.Composite) error {
	return fn(ctx, rq, cp)
}

// A CompositeCreator creates a resource, typically by submitting it to an API
// server. CompositeCreators must not modify the supplied resource class, but are
// responsible for final modifications to the requirement and resource, for example
// ensuring resource, class, requirement, and owner references are set.
type CompositeCreator interface {
	Create(ctx context.Context, rq resource.Requirement, cp resource.Composite) error
}

// A CompositeCreatorFn is a function that satisfies the CompositeCreator interface.
type CompositeCreatorFn func(ctx context.Context, rq resource.Requirement, cp resource.Composite) error

// Create the supplied resource.
func (fn CompositeCreatorFn) Create(ctx context.Context, rq resource.Requirement, cp resource.Composite) error {
	return fn(ctx, rq, cp)
}

// A Binder binds a resource requirement to a composite resource.
type Binder interface {
	// Bind the supplied Requirement to the supplied Composite resource.
	Bind(ctx context.Context, rq resource.Requirement, cp resource.Composite) error

	// Unbind the supplied Requirement from the supplied Composite resource.
	Unbind(ctx context.Context, rq resource.Requirement, cp resource.Composite) error
}

// BinderFns satisfy the Binder interface.
type BinderFns struct {
	BindFn   func(ctx context.Context, rq resource.Requirement, cp resource.Composite) error
	UnbindFn func(ctx context.Context, rq resource.Requirement, cp resource.Composite) error
}

// Bind the supplied Requirement to the supplied Composite resource.
func (b BinderFns) Bind(ctx context.Context, rq resource.Requirement, cp resource.Composite) error {
	return b.BindFn(ctx, rq, cp)
}

// Unbind the supplied Requirement from the supplied Composite resource.
func (b BinderFns) Unbind(ctx context.Context, rq resource.Requirement, cp resource.Composite) error {
	return b.UnbindFn(ctx, rq, cp)
}

// A Reconciler reconciles resource requirements by creating exactly one kind of
// concrete composite resource. Each resource requirement kind should create an instance
// of this controller for each composite resource kind they can bind to, using
// watch predicates to ensure each controller is responsible for exactly one
// type of resource class provisioner. Each controller must watch its subset of
// resource requirements and any composite resources they control.
type Reconciler struct {
	client         client.Client
	newRequirement func() resource.Requirement
	newComposite   func() resource.Composite

	// The below structs embed the set of interfaces used to implement the
	// resource requirement reconciler. We do this primarily for readability, so that
	// the reconciler logic reads r.composite.Create(), r.requirement.Finalize(), etc.
	composite   crComposite
	requirement crRequirement

	log    logging.Logger
	record event.Recorder
}

type crComposite struct {
	CompositeConfigurator
	CompositeCreator
	resource.ConnectionPropagator
}

func defaultCRComposite(c client.Client, t runtime.ObjectTyper) crComposite {
	return crComposite{
		CompositeConfigurator: CompositeConfiguratorFn(Configure),
		CompositeCreator:      NewAPICompositeCreator(c, t),
		ConnectionPropagator:  resource.NewAPIConnectionPropagator(c, t),
	}
}

type crRequirement struct {
	resource.Finalizer
	Binder
}

func defaultCRRequirement(c client.Client, t runtime.ObjectTyper) crRequirement {
	return crRequirement{
		Finalizer: resource.NewAPIFinalizer(c, finalizer),
		Binder:    NewAPIBinder(c, t),
	}
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithCompositeCreator specifies which CompositeCreator should be used to create
// composite resources.
func WithCompositeCreator(c CompositeCreator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CompositeCreator = c
	}
}

// WithConnectionPropagator specifies which ConnectionPropagator should be used
// to propagate resource connection details to their requirement.
func WithConnectionPropagator(p resource.ConnectionPropagator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ConnectionPropagator = p
	}
}

// WithBinder specifies which Binder should be used to bind
// resources to their requirement.
func WithBinder(b Binder) ReconcilerOption {
	return func(r *Reconciler) {
		r.requirement.Binder = b
	}
}

// WithRequirementFinalizer specifies which RequirementFinalizer should be used to finalize
// requirements when they are deleted.
func WithRequirementFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.requirement.Finalizer = f
	}
}

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(l logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = l
	}
}

// WithRecorder specifies how the Reconciler should record events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// NewReconciler returns a Reconciler that reconciles resource requirements of
// the supplied RequirementKind with resources of the supplied CompositeKind.
// The returned Reconciler will apply only the ObjectMetaConfigurator by
// default; most callers should supply one or more CompositeConfigurators to
// configure their composite resources.
func NewReconciler(m manager.Manager, of resource.RequirementKind, with resource.CompositeKind, o ...ReconcilerOption) *Reconciler {
	nr := func() resource.Requirement {
		return requirement.New(requirement.WithGroupVersionKind(schema.GroupVersionKind(of)))
	}
	nc := func() resource.Composite {
		return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(with)))
	}
	c := unstructured.NewClient(m.GetClient())
	r := &Reconciler{
		client:         c,
		newRequirement: nr,
		newComposite:   nc,
		composite:      defaultCRComposite(c, m.GetScheme()),
		requirement:    defaultCRRequirement(c, m.GetScheme()),
		log:            logging.NewNopLogger(),
		record:         event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a resource requirement with a concrete composite resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is well over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	rq := r.newRequirement()
	if err := r.client.Get(ctx, req.NamespacedName, rq); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get resource requirement", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetRequirement)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(rq))
	log = log.WithValues(
		"uid", rq.GetUID(),
		"version", rq.GetResourceVersion(),
		"external-name", meta.GetExternalName(rq),
	)

	cp := r.newComposite()
	if ref := rq.GetResourceReference(); ref != nil {
		record = record.WithAnnotations("composite-name", rq.GetResourceReference().Name)
		log = log.WithValues("composite-name", rq.GetResourceReference().Name)

		err := r.client.Get(ctx, meta.NamespacedNameOf(ref), cp)
		if kerrors.IsNotFound(err) {

			// Our composite was not found, but we're being deleted too. There's
			// nothing to finalize.
			if meta.WasDeleted(rq) {
				// TODO(negz): Can we refactor to avoid this deletion logic that
				// is almost identical to the meta.WasDeleted block below?
				log = log.WithValues("deletion-timestamp", rq.GetDeletionTimestamp())
				if err := r.requirement.RemoveFinalizer(ctx, rq); err != nil {
					// If we didn't hit this error last time we'll be requeued
					// implicitly due to the status update. Otherwise we want to retry
					// after a brief wait, in case this was a transient error.
					log.Debug("Cannot remove finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
					record.Event(rq, event.Warning(reasonUnbind, err))
					rq.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
					return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
				}

				// We've successfully deleted our requirement and removed our finalizer. If we
				// assume we were the only controller that added a finalizer to this
				// requirement then it should no longer exist and thus there is no point
				// trying to update its status.
				log.Debug("Successfully deleted resource requirement")
				return reconcile.Result{Requeue: false}, nil
			}

			// If the composite resource we explicitly reference doesn't exist yet
			// we want to retry after a brief wait, in case it is created. We
			// must explicitly requeue because our EnqueueRequestForRequirement
			// handler can only enqueue reconciles for composite resources that
			// have their requirement reference set, so we can't expect to be queued
			// implicitly when the composite resource we want to bind to appears.
			log.Debug("Referenced composite resource not found", "requeue-after", time.Now().Add(aShortWait))
			record.Event(rq, event.Warning(reasonBind, err))
			rq.SetConditions(Waiting(), v1alpha1.ReconcileSuccess())
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
		}
		if err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot get referenced composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(rq, event.Warning(reasonBind, err))
			rq.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
		}
	}

	if meta.WasDeleted(rq) {
		log = log.WithValues("deletion-timestamp", rq.GetDeletionTimestamp())

		if err := r.requirement.Unbind(ctx, rq, cp); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot unbind requirement", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(rq, event.Warning(reasonUnbind, err))
			rq.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
		}

		log.Debug("Successfully unbound composite resource")
		record.Event(rq, event.Normal(reasonUnbind, "Successfully unbound composite resource"))

		if err := r.requirement.RemoveFinalizer(ctx, rq); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot remove finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(rq, event.Warning(reasonUnbind, err))
			rq.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
		}

		// We've successfully deleted our requirement and removed our finalizer. If we
		// assume we were the only controller that added a finalizer to this
		// requirement then it should no longer exist and thus there is no point
		// trying to update its status.
		log.Debug("Successfully deleted resource requirement")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.requirement.AddFinalizer(ctx, rq); err != nil {
		// If we didn't hit this error last time we'll be requeued
		// implicitly due to the status update. Otherwise we want to retry
		// after a brief wait, in case this was a transient error.
		log.Debug("Cannot add resource requirement finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(rq, event.Warning(reasonBind, err))
		rq.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
	}

	// Requirement reconcilers (should) watch for either requirements with a resource ref,
	// requirements with a class ref, or composite resources with a requirement ref. In the
	// first case the composite resource always exists by the time we get here. In
	// the second case the class reference is set. The third case exposes us to
	// a pathological scenario in which a composite resource references a requirement
	// that has no resource ref or class ref, so we can't assume the class ref
	// is always set at this point.
	if !meta.WasCreated(cp) {

		if err := r.composite.Configure(ctx, rq, cp); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or some
			// issue with the resource class was resolved.
			log.Debug("Cannot configure composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(rq, event.Warning(reasonConfigure, err))
			rq.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
		}

		// We'll know our composite resource's name at this point because it was
		// set by the above configure step.
		record = record.WithAnnotations("composite-name", cp.GetName())
		log = log.WithValues("composite-name", cp.GetName())

		if err := r.composite.Create(ctx, rq, cp); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot create composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(rq, event.Warning(reasonConfigure, err))
			rq.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
		}

		log.Debug("Successfully created composite resource")
		record.Event(rq, event.Normal(reasonConfigure, "Successfully configured composite resource"))
	}

	if !resource.IsConditionTrue(cp.GetCondition(v1alpha1.TypeReady)) {
		log.Debug("Composite resource is not yet ready")
		record.Event(rq, event.Normal(reasonBind, "Composite resource is not yet ready"))

		// We should be watching the composite resource and will have a request
		// queued if it changes.
		rq.SetConditions(Waiting(), v1alpha1.ReconcileSuccess())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
	}

	if err := r.requirement.Bind(ctx, rq, cp); err != nil {
		// If we didn't hit this error last time we'll be requeued implicitly
		// due to the status update. Otherwise we want to retry after a brief
		// wait, in case this was a transient error.
		log.Debug("Cannot bind to composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(rq, event.Warning(reasonBind, err))
		rq.SetConditions(Waiting(), v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
	}

	log.Debug("Successfully bound composite resource")
	record.Event(rq, event.Normal(reasonBind, "Successfully bound composite resource"))

	if err := r.composite.PropagateConnection(ctx, rq, cp); err != nil {
		// If we didn't hit this error last time we'll be requeued implicitly
		// due to the status update. Otherwise we want to retry after a brief
		// wait in case this was a transient error, or the resource connection
		// secret is created.
		log.Debug("Cannot propagate connection details from composite resource to requirement", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(rq, event.Warning(reasonPropagate, err))
		rq.SetConditions(Waiting(), v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
	}

	// We have a watch on both the requirement and its composite, so there's no
	// need to requeue here.
	record.Event(rq, event.Normal(reasonPropagate, "Successfully propagated connection details from composite resource"))
	rq.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, rq), errUpdateRequirementStatus)
}

// Waiting returns a condition that indicates the resource requirement is
// currently waiting for its composite resource to become ready.
func Waiting() v1alpha1.Condition {
	return v1alpha1.Condition{
		Type:               v1alpha1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWaiting,
	}
}
