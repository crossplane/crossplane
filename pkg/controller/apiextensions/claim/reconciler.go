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

package claim

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
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

const (
	finalizer        = "finalizer.apiextensions.crossplane.io"
	reconcileTimeout = 1 * time.Minute

	aShortWait = 30 * time.Second
)

// Reasons a composite resource claim is or is not ready.
const (
	ReasonWaiting = "Composite resource claim is waiting for composite resource to become Ready"
)

// Error strings.
const (
	errGetClaim          = "cannot get composite resource claim"
	errUpdateClaimStatus = "cannot update composite resource claim status"
)

// Event reasons.
const (
	reasonBind      event.Reason = "BindCompositeResource"
	reasonDelete    event.Reason = "DeleteCompositeResource"
	reasonConfigure event.Reason = "ConfigureCompositeResource"
	reasonPropagate event.Reason = "PropagateConnectionSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite resource claim.
func ControllerName(name string) string {
	return "claim/" + name
}

// A CompositeConfigurator configures a resource, typically by converting it to
// a known type and populating its spec.
type CompositeConfigurator interface {
	Configure(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

// A CompositeConfiguratorFn is a function that satisfies the
// CompositeConfigurator interface.
type CompositeConfiguratorFn func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error

// Configure the supplied resource using the supplied claim.
func (fn CompositeConfiguratorFn) Configure(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	return fn(ctx, cm, cp)
}

// A CompositeCreator creates a resource, typically by submitting it to an API
// server. CompositeCreators must not modify the supplied resource class, but are
// responsible for final modifications to the claim and resource, for example
// ensuring resource, claim, and owner references are set.
type CompositeCreator interface {
	Create(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

// A CompositeCreatorFn is a function that satisfies the CompositeCreator interface.
type CompositeCreatorFn func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error

// Create the supplied resource.
func (fn CompositeCreatorFn) Create(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	return fn(ctx, cm, cp)
}

// A CompositeDeleter deletes a composite resource.
type CompositeDeleter interface {
	// Delete the supplied Claim to the supplied Composite resource.
	Delete(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

// A Binder binds a composite resource claim to a composite resource.
type Binder interface {
	// Bind the supplied Claim to the supplied Composite resource.
	Bind(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

// BinderFns satisfy the Binder interface.
type BinderFns struct {
	BindFn   func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
	UnbindFn func(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error
}

// Bind the supplied Claim to the supplied Composite resource.
func (b BinderFns) Bind(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	return b.BindFn(ctx, cm, cp)
}

// Unbind the supplied Claim from the supplied Composite resource.
func (b BinderFns) Unbind(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	return b.UnbindFn(ctx, cm, cp)
}

// A Reconciler reconciles composite resource claims by creating exactly one kind of
// concrete composite resource. Each composite resource claim kind should create an instance
// of this controller for each composite resource kind they can bind to, using
// watch predicates to ensure each controller is responsible for exactly one
// type of resource class provisioner. Each controller must watch its subset of
// composite resource claims and any composite resources they control.
type Reconciler struct {
	client       client.Client
	newClaim     func() resource.CompositeClaim
	newComposite func() resource.Composite

	// The below structs embed the set of interfaces used to implement the
	// composite resource claim reconciler. We do this primarily for readability, so that
	// the reconciler logic reads r.composite.Create(), r.claim.Finalize(), etc.
	composite crComposite
	claim     crClaim

	log    logging.Logger
	record event.Recorder
}

type crComposite struct {
	CompositeConfigurator
	CompositeCreator
	CompositeDeleter
	resource.ConnectionPropagator
}

func defaultCRComposite(c client.Client, t runtime.ObjectTyper) crComposite {
	return crComposite{
		CompositeConfigurator: CompositeConfiguratorFn(Configure),
		CompositeCreator:      NewAPICompositeCreator(c, t),
		CompositeDeleter:      NewAPICompositeDeleter(c),
		ConnectionPropagator:  resource.NewAPIConnectionPropagator(c, t),
	}
}

type crClaim struct {
	resource.Finalizer
	Binder
}

func defaultCRClaim(c client.Client, t runtime.ObjectTyper) crClaim {
	return crClaim{
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
// to propagate resource connection details to their claim.
func WithConnectionPropagator(p resource.ConnectionPropagator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ConnectionPropagator = p
	}
}

// WithBinder specifies which Binder should be used to bind
// resources to their claim.
func WithBinder(b Binder) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Binder = b
	}
}

// WithClaimFinalizer specifies which ClaimFinalizer should be used to finalize
// claims when they are deleted.
func WithClaimFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Finalizer = f
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

// NewReconciler returns a Reconciler that reconciles composite resource claims of
// the supplied CompositeClaimKind with resources of the supplied CompositeKind.
// The returned Reconciler will apply only the ObjectMetaConfigurator by
// default; most callers should supply one or more CompositeConfigurators to
// configure their composite resources.
func NewReconciler(m manager.Manager, of resource.CompositeClaimKind, with resource.CompositeKind, o ...ReconcilerOption) *Reconciler {
	c := unstructured.NewClient(m.GetClient())
	r := &Reconciler{
		client: c,
		newClaim: func() resource.CompositeClaim {
			return claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind(of)))
		},
		newComposite: func() resource.Composite {
			return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(with)))
		},
		composite: defaultCRComposite(c, m.GetScheme()),
		claim:     defaultCRClaim(c, m.GetScheme()),
		log:       logging.NewNopLogger(),
		record:    event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a composite resource claim with a concrete composite resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is well over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	cm := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, cm); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get composite resource claim", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetClaim)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(cm))
	log = log.WithValues(
		"uid", cm.GetUID(),
		"version", cm.GetResourceVersion(),
		"external-name", meta.GetExternalName(cm),
	)

	cp := r.newComposite()
	if ref := cm.GetResourceReference(); ref != nil {
		record = record.WithAnnotations("composite-name", cm.GetResourceReference().Name)
		log = log.WithValues("composite-name", cm.GetResourceReference().Name)

		err := r.client.Get(ctx, meta.NamespacedNameOf(ref), cp)
		if kerrors.IsNotFound(err) {

			// Our composite was not found, but we're being deleted too. There's
			// nothing to finalize.
			if meta.WasDeleted(cm) {
				// TODO(negz): Can we refactor to avoid this deletion logic that
				// is almost identical to the meta.WasDeleted block below?
				log = log.WithValues("deletion-timestamp", cm.GetDeletionTimestamp())
				if err := r.claim.RemoveFinalizer(ctx, cm); err != nil {
					// If we didn't hit this error last time we'll be requeued
					// implicitly due to the status update. Otherwise we want to retry
					// after a brief wait, in case this was a transient error.
					log.Debug("Cannot remove finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
					record.Event(cm, event.Warning(reasonDelete, err))
					cm.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
					return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
				}

				// We've successfully deleted our claim and removed our finalizer. If we
				// assume we were the only controller that added a finalizer to this
				// claim then it should no longer exist and thus there is no point
				// trying to update its status.
				log.Debug("Successfully deleted composite resource claim")
				return reconcile.Result{Requeue: false}, nil
			}

			// If the composite resource we explicitly reference doesn't exist yet
			// we want to retry after a brief wait, in case it is created. We
			// must explicitly requeue because our EnqueueRequestForClaim
			// handler can only enqueue reconciles for composite resources that
			// have their claim reference set, so we can't expect to be queued
			// implicitly when the composite resource we want to bind to appears.
			log.Debug("Referenced composite resource not found", "requeue-after", time.Now().Add(aShortWait))
			record.Event(cm, event.Warning(reasonBind, err))
			cm.SetConditions(Waiting(), v1alpha1.ReconcileSuccess())
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}
		if err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot get referenced composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(cm, event.Warning(reasonBind, err))
			cm.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}
	}

	if meta.WasDeleted(cm) {
		log = log.WithValues("deletion-timestamp", cm.GetDeletionTimestamp())

		if err := r.composite.Delete(ctx, cm, cp); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot delete composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(cm, event.Warning(reasonDelete, err))
			cm.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}

		log.Debug("Successfully deleted composite resource")
		record.Event(cm, event.Normal(reasonDelete, "Successfully deleted composite resource"))

		if err := r.claim.RemoveFinalizer(ctx, cm); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot remove finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(cm, event.Warning(reasonDelete, err))
			cm.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}

		// We've successfully deleted our claim and removed our finalizer. If we
		// assume we were the only controller that added a finalizer to this
		// claim then it should no longer exist and thus there is no point
		// trying to update its status.
		log.Debug("Successfully deleted composite resource claim")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.claim.AddFinalizer(ctx, cm); err != nil {
		// If we didn't hit this error last time we'll be requeued
		// implicitly due to the status update. Otherwise we want to retry
		// after a brief wait, in case this was a transient error.
		log.Debug("Cannot add composite resource claim finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(cm, event.Warning(reasonBind, err))
		cm.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	// Claim reconcilers (should) watch for either claims with a resource ref,
	// claims with a class ref, or composite resources with a claim ref. In the
	// first case the composite resource always exists by the time we get here. In
	// the second case the class reference is set. The third case exposes us to
	// a pathological scenario in which a composite resource references a claim
	// that has no resource ref or class ref, so we can't assume the class ref
	// is always set at this point.
	if !meta.WasCreated(cp) {

		if err := r.composite.Configure(ctx, cm, cp); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or some
			// issue with the resource class was resolved.
			log.Debug("Cannot configure composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(cm, event.Warning(reasonConfigure, err))
			cm.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}

		// We'll know our composite resource's name at this point because it was
		// set by the above configure step.
		record = record.WithAnnotations("composite-name", cp.GetName())
		log = log.WithValues("composite-name", cp.GetName())

		if err := r.composite.Create(ctx, cm, cp); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot create composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(cm, event.Warning(reasonConfigure, err))
			cm.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}

		log.Debug("Successfully created composite resource")
		record.Event(cm, event.Normal(reasonConfigure, "Successfully configured composite resource"))
	}

	if !resource.IsConditionTrue(cp.GetCondition(v1alpha1.TypeReady)) {
		log.Debug("Composite resource is not yet ready")
		record.Event(cm, event.Normal(reasonBind, "Composite resource is not yet ready"))

		// We should be watching the composite resource and will have a request
		// queued if it changes.
		cm.SetConditions(Waiting(), v1alpha1.ReconcileSuccess())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	if err := r.claim.Bind(ctx, cm, cp); err != nil {
		// If we didn't hit this error last time we'll be requeued implicitly
		// due to the status update. Otherwise we want to retry after a brief
		// wait, in case this was a transient error.
		log.Debug("Cannot bind to composite resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(cm, event.Warning(reasonBind, err))
		cm.SetConditions(Waiting(), v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	log.Debug("Successfully bound composite resource")
	record.Event(cm, event.Normal(reasonBind, "Successfully bound composite resource"))

	if err := r.composite.PropagateConnection(ctx, cm, cp); err != nil {
		// If we didn't hit this error last time we'll be requeued implicitly
		// due to the status update. Otherwise we want to retry after a brief
		// wait in case this was a transient error, or the resource connection
		// secret is created.
		log.Debug("Cannot propagate connection details from composite resource to claim", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(cm, event.Warning(reasonPropagate, err))
		cm.SetConditions(Waiting(), v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	// We have a watch on both the claim and its composite, so there's no
	// need to requeue here.
	record.Event(cm, event.Normal(reasonPropagate, "Successfully propagated connection details from composite resource"))
	cm.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
}

// Waiting returns a condition that indicates the composite resource claim is
// currently waiting for its composite resource to become ready.
func Waiting() v1alpha1.Condition {
	return v1alpha1.Condition{
		Type:               v1alpha1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWaiting,
	}
}
