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

package resource

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	controllerName   = "resourceclaim.crossplane.io"
	finalizerName    = "finalizer." + controllerName
	reconcileTimeout = 1 * time.Minute
	aShortWait       = 30 * time.Second
)

// Reasons a resource claim is or is not ready.
const (
	ReasonBinding = "Managed claim is waiting for managed resource to become bindable"
)

// Error strings.
const (
	errGetClaim          = "cannot get resource claim"
	errUpdateClaimStatus = "cannot update resource claim status"
)

var log = logging.Logger.WithName("controller").WithValues("controller", controllerName)

// A ClaimKind contains the type metadata for a kind of resource claim.
type ClaimKind schema.GroupVersionKind

// A ManagedKind contains the type metadata for a kind of managed resource.
type ManagedKind schema.GroupVersionKind

// A ManagedConfigurator configures a resource, typically by converting it to
// a known type and populating its spec.
type ManagedConfigurator interface {
	Configure(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error
}

// A ManagedConfiguratorFn is a function that sastisfies the
// ManagedConfigurator interface.
type ManagedConfiguratorFn func(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error

// Configure the supplied resource using the supplied claim and class.
func (fn ManagedConfiguratorFn) Configure(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error {
	return fn(ctx, cm, cs, mg)
}

// A ManagedCreator creates a resource, typically by submitting it to an API
// server. ManagedCreators must not modify the supplied resource class, but are
// responsible for final modifications to the claim and resource, for example
// ensuring resource, class, claim, and owner references are set.
type ManagedCreator interface {
	Create(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error
}

// A ManagedCreatorFn is a function that sastisfies the ManagedCreator interface.
type ManagedCreatorFn func(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error

// Create the supplied resource.
func (fn ManagedCreatorFn) Create(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error {
	return fn(ctx, cm, cs, mg)
}

// A ManagedConnectionPropagator is responsible for propagating information
// required to connect to a managed resource (for example the connection secret)
// from the managed resource to its resource claim.
type ManagedConnectionPropagator interface {
	PropagateConnection(ctx context.Context, cm Claim, mg Managed) error
}

// A ManagedConnectionPropagatorFn is a function that sastisfies the
// ManagedConnectionPropagator interface.
type ManagedConnectionPropagatorFn func(ctx context.Context, cm Claim, mg Managed) error

// PropagateConnection information from the supplied managed resource to the
// supplied resource claim.
func (fn ManagedConnectionPropagatorFn) PropagateConnection(ctx context.Context, cm Claim, mg Managed) error {
	return fn(ctx, cm, mg)
}

// A ManagedBinder binds a resource claim to a managed resource.
type ManagedBinder interface {
	Bind(ctx context.Context, cm Claim, mg Managed) error
}

// A ManagedBinderFn is a function that sastisfies the ManagedBinder interface.
type ManagedBinderFn func(ctx context.Context, cm Claim, mg Managed) error

// Bind the supplied resource claim to the supplied managed resource.
func (fn ManagedBinderFn) Bind(ctx context.Context, cm Claim, mg Managed) error {
	return fn(ctx, cm, mg)
}

// A ManagedFinalizer finalizes the deletion of a resource claim.
type ManagedFinalizer interface {
	Finalize(ctx context.Context, cm Managed) error
}

// A ManagedFinalizerFn is a function that sastisfies the ManagedFinalizer interface.
type ManagedFinalizerFn func(ctx context.Context, cm Managed) error

// Finalize the supplied managed resource.
func (fn ManagedFinalizerFn) Finalize(ctx context.Context, cm Managed) error {
	return fn(ctx, cm)
}

// A ClaimFinalizer finalizes the deletion of a resource claim.
type ClaimFinalizer interface {
	Finalize(ctx context.Context, cm Claim) error
}

// A ClaimFinalizerFn is a function that sastisfies the ClaimFinalizer interface.
type ClaimFinalizerFn func(ctx context.Context, cm Claim) error

// Finalize the supplied managed resource.
func (fn ClaimFinalizerFn) Finalize(ctx context.Context, cm Claim) error {
	return fn(ctx, cm)
}

// A ClaimReconciler reconciles resource claims by creating exactly one kind of
// concrete managed resource. Each resource class should create an instance of
// this controller for each managed resource kind they can bind to, using watch
// predicates to ensure each controller is responsible for exactly one type of
// resource claim provisioner. Each controller must watch its subset of resource
// claims and any managed resources they control.
type ClaimReconciler struct {
	client     client.Client
	newClaim   func() Claim
	newManaged func() Managed
	managed    managed
	claim      claim
}

type managed struct {
	ManagedConfigurator
	ManagedCreator
	ManagedConnectionPropagator
	ManagedBinder
	ManagedFinalizer
}

func defaultManaged(m manager.Manager) managed {
	return managed{
		ManagedConfigurator:         NewObjectMetaConfigurator(m.GetScheme()),
		ManagedCreator:              NewAPIManagedCreator(m.GetClient(), m.GetScheme()),
		ManagedConnectionPropagator: NewAPIManagedConnectionPropagator(m.GetClient(), m.GetScheme()),
		ManagedBinder:               NewAPIManagedBinder(m.GetClient()),
		ManagedFinalizer:            NewAPIManagedFinalizer(m.GetClient()),
	}
}

type claim struct {
	ClaimFinalizer
}

func defaultClaim(m manager.Manager) claim {
	return claim{ClaimFinalizer: NewAPIClaimFinalizer(m.GetClient())}
}

// A ClaimReconcilerOption configures a Reconciler.
type ClaimReconcilerOption func(*ClaimReconciler)

// WithManagedConfigurators specifies which configurators should be used to
// configure each managed resource. Configurators will be applied in the order
// they are specified.
func WithManagedConfigurators(c ...ManagedConfigurator) ClaimReconcilerOption {
	return func(r *ClaimReconciler) {
		r.managed.ManagedConfigurator = ConfiguratorChain(c)
	}
}

// WithManagedCreator specifies which ManagedCreator should be used to create
// managed resources.
func WithManagedCreator(c ManagedCreator) ClaimReconcilerOption {
	return func(r *ClaimReconciler) {
		r.managed.ManagedCreator = c
	}
}

// WithManagedConnectionPropagator specifies which ManagedConnectionPropagator
// should be used to propagate resource connection details to their claim.
func WithManagedConnectionPropagator(p ManagedConnectionPropagator) ClaimReconcilerOption {
	return func(r *ClaimReconciler) {
		r.managed.ManagedConnectionPropagator = p
	}
}

// WithManagedBinder specifies which ManagedBinder should be used to bind
// resources to their claim.
func WithManagedBinder(b ManagedBinder) ClaimReconcilerOption {
	return func(r *ClaimReconciler) {
		r.managed.ManagedBinder = b
	}
}

// WithManagedFinalizer specifies which ManagedFinalizer should be used to
// finalize managed resources when their claims are deleted.
func WithManagedFinalizer(f ManagedFinalizer) ClaimReconcilerOption {
	return func(r *ClaimReconciler) {
		r.managed.ManagedFinalizer = f
	}
}

// WithClaimFinalizer specifies which ClaimFinalizer should be used to finalize
// claims when they are deleted.
func WithClaimFinalizer(f ClaimFinalizer) ClaimReconcilerOption {
	return func(r *ClaimReconciler) {
		r.claim.ClaimFinalizer = f
	}
}

// NewClaimReconciler returns a ClaimReconciler that reconciles resource claims
// of the supplied ClaimType with resources of the supplied ManagedType. It
// panics if asked to reconcile a claim or resource kind that is not registered
// with the supplied manager's runtime.Scheme. The returned ClaimReconciler will
// apply only the ObjectMetaConfigurator by default; most callers should supply
// one or more ManagedConfigurators to configure their managed resources.
func NewClaimReconciler(m manager.Manager, of ClaimKind, with ManagedKind, o ...ClaimReconcilerOption) *ClaimReconciler {
	nc := func() Claim { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Claim) }
	nr := func() Managed { return MustCreateObject(schema.GroupVersionKind(with), m.GetScheme()).(Managed) }

	// Panic early if we've been asked to reconcile a claim or resource kind
	// that has not been registered with our controller manager's scheme.
	_, _ = nc(), nr()

	r := &ClaimReconciler{
		client:     m.GetClient(),
		newClaim:   nc,
		newManaged: nr,
		managed:    defaultManaged(m),
		claim:      defaultClaim(m),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a resource claim with a concrete managed resource.
func (r *ClaimReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is a little over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log.V(logging.Debug).Info("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	claim := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, claim); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetClaim)
	}

	managed := r.newManaged()
	if ref := claim.GetResourceReference(); ref != nil {
		if err := IgnoreNotFound(r.client.Get(ctx, meta.NamespacedNameOf(ref), managed)); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			claim.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}
	}

	if meta.WasDeleted(claim) {
		if err := r.managed.Finalize(ctx, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			claim.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		if err := r.claim.Finalize(ctx, claim); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			claim.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		// We've successfully processed the delete, so there's no further
		// reconciliation to do. There's a good chance our claim no longer
		// exists, but we try update its status just in case it sticks around,
		// for example due to additional finalizers.
		claim.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	if !meta.WasCreated(managed) {
		class := &v1alpha1.ResourceClass{}
		// Class reference should always be set by the time we get this far; our
		// watch predicates require it.
		if err := r.client.Get(ctx, meta.NamespacedNameOf(claim.GetClassReference()), class); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or the
			// class is (re)created.
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		if err := r.managed.Configure(ctx, claim, class, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or some
			// issue with the resource class was resolved.
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		if err := r.managed.Create(ctx, claim, class, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}
	}

	if !IsBindable(managed) && !IsBound(managed) {
		// If this claim was not already binding we'll be requeued due to the
		// status update. Otherwise there's no need to requeue. We should be
		// watching both the resource claims and the resources we own, so we'll
		// be queued if anything changes.
		claim.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
	}

	if IsBindable(managed) {
		if err := r.managed.PropagateConnection(ctx, claim, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued implicitly
			// due to the status update. Otherwise we want to retry after a brief
			// wait in case this was a transient error, or the resource connection
			// secret is created.
			claim.SetConditions(Binding(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		if err := r.managed.Bind(ctx, claim, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued implicitly
			// due to the status update. Otherwise we want to retry after a brief
			// wait, in case this was a transient error.
			claim.SetConditions(Binding(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}
	}

	// No need to requeue. We should be watching both the resource claims and
	// the resources we own, so we'll be queued if anything changes.
	claim.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
}

// Binding returns a condition that indicates the resource claim is currently
// waiting for its managed resource to become bindable.
func Binding() v1alpha1.Condition {
	return v1alpha1.Condition{
		Type:               v1alpha1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonBinding,
	}
}
