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

// Package claim implements composite resource claims.
package claim

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/internal/names"
	"github.com/crossplane/crossplane/internal/xresource"
)

const (
	finalizer        = "finalizer.apiextensions.crossplane.io"
	reconcileTimeout = 1 * time.Minute
)

// Error strings.
const (
	errGetClaim             = "cannot get claim"
	errGetComposite         = "cannot get bound composite resource"
	errDeleteComposite      = "cannot delete bound composite resource"
	errDeleteCDs            = "cannot delete connection details"
	errRemoveFinalizer      = "cannot remove finalizer from claim"
	errAddFinalizer         = "cannot add finalizer to claim"
	errUpgradeManagedFields = "cannot upgrade composite resource's managed fields from client-side to server-side apply"
	errSync                 = "cannot bind and sync claim with composite resource"
	errPropagateCDs         = "cannot propagate connection details from composite resource"
	errUpdateClaimStatus    = "cannot update claim status"

	errFmtUnbound = "refusing to operate on composite resource %q that is not bound to this claim: bound to claim %q"
)

const reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"

// Event reasons.
const (
	reasonBind      event.Reason = "BindCompositeResource"
	reasonDelete    event.Reason = "DeleteCompositeResource"
	reasonPropagate event.Reason = "PropagateConnectionSecret"
	reasonPaused    event.Reason = "ReconciliationPaused"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite resource claim.
func ControllerName(name string) string {
	return "claim/" + name
}

// A ManagedFieldsUpgrader upgrades an objects managed fields from client-side
// apply to server-side apply. This is necessary when an object was previously
// managed using client-side apply, but should now be managed using server-side
// apply. See https://github.com/kubernetes/kubernetes/issues/99003 for details.
type ManagedFieldsUpgrader interface {
	Upgrade(ctx context.Context, obj client.Object, ssaManager string) error
}

// A CompositeSyncer binds and syncs the supplied claim with the supplied
// composite resource (XR).
type CompositeSyncer interface {
	Sync(ctx context.Context, cm *claim.Unstructured, xr *composite.Unstructured) error
}

// A CompositeSyncerFn binds and syncs the supplied claim with the supplied
// composite resource (XR).
type CompositeSyncerFn func(ctx context.Context, cm *claim.Unstructured, xr *composite.Unstructured) error

// Sync the supplied claim with the supplied composite resource..
func (fn CompositeSyncerFn) Sync(ctx context.Context, cm *claim.Unstructured, xr *composite.Unstructured) error {
	return fn(ctx, cm, xr)
}

// A ConnectionPropagator is responsible for propagating information required to
// connect to a resource.
type ConnectionPropagator interface {
	PropagateConnection(ctx context.Context, to xresource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error)
}

// A ConnectionPropagatorFn is responsible for propagating information required
// to connect to a resource.
type ConnectionPropagatorFn func(ctx context.Context, to xresource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error)

// PropagateConnection details from one resource to the other.
func (fn ConnectionPropagatorFn) PropagateConnection(ctx context.Context, to xresource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
	return fn(ctx, to, from)
}

// A ConnectionPropagatorChain runs multiple connection propagators.
type ConnectionPropagatorChain []ConnectionPropagator

// PropagateConnection details from one resource to the other.
// This method calls PropagateConnection for all ConnectionPropagator's in the
// chain and returns propagated if at least one ConnectionPropagator propagates
// the connection details but exits with an error if any of them fails without
// calling the remaining ones.
func (pc ConnectionPropagatorChain) PropagateConnection(ctx context.Context, to xresource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
	for _, p := range pc {
		var pg bool
		pg, err = p.PropagateConnection(ctx, to, from)
		if pg {
			propagated = true
		}
		if err != nil {
			return propagated, err
		}
	}
	return propagated, nil
}

// A ConnectionUnpublisher is responsible for cleaning up connection secret.
type ConnectionUnpublisher interface {
	// UnpublishConnection details for the supplied Managed resource.
	UnpublishConnection(ctx context.Context, so xresource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error
}

// A ConnectionUnpublisherFn is responsible for cleaning up connection secret.
type ConnectionUnpublisherFn func(ctx context.Context, so xresource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error

// UnpublishConnection details of a local connection secret owner.
func (fn ConnectionUnpublisherFn) UnpublishConnection(ctx context.Context, so xresource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error {
	return fn(ctx, so, c)
}

// A DefaultsSelector copies default values from the CompositeResourceDefinition when the corresponding field
// in the Claim is not set.
type DefaultsSelector interface {
	// SelectDefaults from CompositeResourceDefinition when needed.
	SelectDefaults(ctx context.Context, cm resource.CompositeClaim) error
}

// A DefaultsSelectorFn is responsible for copying default values from the CompositeResourceDefinition.
type DefaultsSelectorFn func(ctx context.Context, cm resource.CompositeClaim) error

// SelectDefaults copies default values from the XRD if necessary.
func (fn DefaultsSelectorFn) SelectDefaults(ctx context.Context, cm resource.CompositeClaim) error {
	return fn(ctx, cm)
}

// A Reconciler reconciles claims by creating exactly one kind of composite
// resource (XR). Each claim kind should create an instance of this controller
// for each XR kind they can bind to. Each controller must watch its subset of
// claims and any XRs they bind to.
type Reconciler struct {
	client client.Client

	gvkClaim schema.GroupVersionKind
	gvkXR    schema.GroupVersionKind

	managedFields ManagedFieldsUpgrader

	// The below structs embed the set of interfaces used to implement the
	// composite resource claim reconciler. We do this primarily for
	// readability, so that the reconciler logic reads r.composite.Sync(),
	// r.claim.Finalize(), etc.
	composite crComposite
	claim     crClaim

	log          logging.Logger
	record       event.Recorder
	pollInterval time.Duration
}

type crComposite struct {
	CompositeSyncer
	ConnectionPropagator
}

func defaultCRComposite(c client.Client) crComposite {
	return crComposite{
		CompositeSyncer:      NewClientSideCompositeSyncer(c, names.NewNameGenerator(c)),
		ConnectionPropagator: NewAPIConnectionPropagator(c),
	}
}

type crClaim struct {
	resource.Finalizer
	ConnectionUnpublisher
}

func defaultCRClaim(c client.Client) crClaim {
	return crClaim{
		Finalizer:             resource.NewAPIFinalizer(c, finalizer),
		ConnectionUnpublisher: NewNopConnectionUnpublisher(),
	}
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithManagedFieldsUpgrader specifies how the Reconciler should upgrade claim
// and composite resource (XR) managed fields from client-side apply to
// server-side apply.
func WithManagedFieldsUpgrader(u ManagedFieldsUpgrader) ReconcilerOption {
	return func(r *Reconciler) {
		r.managedFields = u
	}
}

// WithCompositeSyncer specifies how the Reconciler should sync claims with
// composite resources (XRs).
func WithCompositeSyncer(cs CompositeSyncer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CompositeSyncer = cs
	}
}

// WithConnectionPropagator specifies which ConnectionPropagator should be used
// to propagate resource connection details to their claim.
func WithConnectionPropagator(p ConnectionPropagator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ConnectionPropagator = p
	}
}

// WithConnectionUnpublisher specifies which ConnectionUnpublisher should be
// used to unpublish resource connection details.
func WithConnectionUnpublisher(u ConnectionUnpublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.ConnectionUnpublisher = u
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

// WithPollInterval specifies how long the Reconciler should wait before queueing
// a new reconciliation after a successful reconcile. The Reconciler requeues
// after a specified duration when it is not actively waiting for an external
// operation, but wishes to check whether resources it does not have a watch on
// (i.e. composed resources) need to be reconciled.
func WithPollInterval(after time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.pollInterval = after
	}
}

// NewReconciler returns a Reconciler that reconciles composite resource claims of
// the supplied CompositeClaimKind with resources of the supplied CompositeKind.
// The returned Reconciler will apply only the ObjectMetaConfigurator by
// default; most callers should supply one or more CompositeConfigurators to
// configure their composite resources.
func NewReconciler(c client.Client, of resource.CompositeClaimKind, with resource.CompositeKind, o ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:        c,
		gvkClaim:      schema.GroupVersionKind(of),
		gvkXR:         schema.GroupVersionKind(with),
		managedFields: &NopManagedFieldsUpgrader{},
		composite:     defaultCRComposite(c),
		claim:         defaultCRClaim(c),
		log:           logging.NewNopLogger(),
		record:        event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a composite resource claim with a concrete composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Complexity is tough to avoid here.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	cm := claim.New(claim.WithGroupVersionKind(r.gvkClaim))
	if err := r.client.Get(ctx, req.NamespacedName, cm); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug(errGetClaim, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetClaim)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(cm))
	log = log.WithValues(
		"uid", cm.GetUID(),
		"version", cm.GetResourceVersion(),
		"external-name", meta.GetExternalName(cm),
	)

	// Check the pause annotation and return if it has the value "true" after
	// logging, publishing an event and updating the Synced status condition.
	if meta.IsPaused(cm) {
		r.record.Event(cm, event.Normal(reasonPaused, reconcilePausedMsg))
		cm.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		// If the pause annotation is removed, we will have a chance to
		// reconcile again and resume and if status update fails, we will
		// reconcile again to retry to update the status.
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	xr := composite.New(composite.WithGroupVersionKind(r.gvkXR))
	if ref := cm.GetResourceReference(); ref != nil {
		record = record.WithAnnotations("composite-name", cm.GetResourceReference().Name)
		log = log.WithValues("composite-name", cm.GetResourceReference().Name)

		if err := r.client.Get(ctx, types.NamespacedName{Name: ref.Name}, xr); resource.IgnoreNotFound(err) != nil {
			err = errors.Wrap(err, errGetComposite)
			record.Event(cm, event.Warning(reasonBind, err))
			cm.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}
	}

	// Return early if the claim references an XR that doesn't reference it.
	//
	// We don't requeue in this situation because the claim will need human
	// intervention before we can proceed (e.g. fixing the ref), and we'll be
	// queued implicitly when the claim is edited.
	//
	// A claim might be able to delete an XR it's not bound to, as long as the
	// XR is bindable but not yet bound. This is because we only check the claim
	// ref if the XR has one - this allows us to bind unbound claims. Given that
	// the claim could bind this XR, then be deleted and in turn delete the XR
	// this is not an issue.
	if ref := xr.GetClaimReference(); meta.WasCreated(xr) && ref != nil && !cmp.Equal(cm.GetReference(), ref) {
		err := errors.Errorf(errFmtUnbound, xr.GetName(), ref.Name)
		record.Event(cm, event.Warning(reasonBind, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	// TODO(negz): Remove this call to Upgrade once no supported version of
	// Crossplane uses client-side apply to sync claims with XRs. We only need
	// to upgrade field managers if _this controller_ might have applied the XR
	// before using the default client-side apply field manager "crossplane",
	// but now wants to use server-side apply instead.
	if err := r.managedFields.Upgrade(ctx, xr, FieldOwnerXR); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errUpgradeManagedFields)
		record.Event(cm, event.Warning(reasonBind, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	if meta.WasDeleted(cm) {
		log = log.WithValues("deletion-timestamp", cm.GetDeletionTimestamp())

		cm.SetConditions(xpv1.Deleting())
		if meta.WasCreated(xr) {
			requiresForegroundDeletion := false
			if cdp := cm.GetCompositeDeletePolicy(); cdp != nil && *cdp == xpv1.CompositeDeleteForeground {
				requiresForegroundDeletion = true
			}
			if meta.WasDeleted(xr) && requiresForegroundDeletion {
				log.Debug("Waiting for the XR to finish deleting (foreground deletion)")
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
			}
			do := &client.DeleteOptions{}
			if requiresForegroundDeletion {
				client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(do)
			}
			if err := r.client.Delete(ctx, xr, do); resource.IgnoreNotFound(err) != nil {
				err = errors.Wrap(err, errDeleteComposite)
				record.Event(cm, event.Warning(reasonDelete, err))
				cm.SetConditions(xpv1.ReconcileError(err))
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
			}
			if requiresForegroundDeletion {
				log.Debug("Waiting for the XR to finish deleting (foreground deletion)")
				return reconcile.Result{Requeue: true}, nil
			}
		}

		// Claims do not publish connection details but may propagate XR
		// secrets. Hence, we need to clean up propagated secrets when the
		// claim is deleted.
		if err := r.claim.UnpublishConnection(ctx, cm, nil); err != nil {
			err = errors.Wrap(err, errDeleteCDs)
			record.Event(cm, event.Warning(reasonDelete, err))
			cm.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}

		record.Event(cm, event.Normal(reasonDelete, "Successfully deleted composite resource"))

		if err := r.claim.RemoveFinalizer(ctx, cm); err != nil {
			err = errors.Wrap(err, errRemoveFinalizer)
			record.Event(cm, event.Warning(reasonDelete, err))
			cm.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
		}

		log.Debug("Successfully deleted claim")
		cm.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	if err := r.claim.AddFinalizer(ctx, cm); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errAddFinalizer)
		record.Event(cm, event.Warning(reasonBind, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	// The XR's claim reference before syncing. Used to determine if we bind it.
	before := xr.GetClaimReference()

	// Create (if necessary), bind, and sync an XR with the claim.
	if err := r.composite.Sync(ctx, cm, xr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errSync)
		record.Event(cm, event.Warning(reasonBind, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	// The XR didn't reference the claim before the sync, but does now.
	if ref := cm.GetReference(); !cmp.Equal(before, ref) && cmp.Equal(xr.GetClaimReference(), ref) {
		record.Event(cm, event.Normal(reasonBind, "Successfully bound composite resource"))
	}

	cm.SetConditions(xpv1.ReconcileSuccess())

	// Copy any custom status conditions from the XR to the claim.
	for _, cType := range xr.GetClaimConditionTypes() {
		c := xr.GetCondition(cType)
		cm.SetConditions(c)
	}

	if !resource.IsConditionTrue(xr.GetCondition(xpv1.TypeReady)) {
		record.Event(cm, event.Normal(reasonBind, "Composite resource is not yet ready"))

		// We should be watching the composite resource and will have a
		// request queued if it changes, so no need to requeue.
		cm.SetConditions(Waiting())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}

	propagated, err := r.composite.PropagateConnection(ctx, cm, xr)
	if err != nil {
		err = errors.Wrap(err, errPropagateCDs)
		record.Event(cm, event.Warning(reasonPropagate, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
	}
	if propagated {
		cm.SetConnectionDetailsLastPublishedTime(&metav1.Time{Time: time.Now()})
		record.Event(cm, event.Normal(reasonPropagate, "Successfully propagated connection details from composite resource"))
	}

	// We have a watch on both the claim and its composite, so there's no
	// need to requeue here.
	cm.SetConditions(xpv1.Available())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm), errUpdateClaimStatus)
}

// Waiting returns a condition that indicates the composite resource claim is
// currently waiting for its composite resource to become ready.
func Waiting() xpv1.Condition {
	return xpv1.Condition{
		Type:               xpv1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             xpv1.ConditionReason("Waiting"),
		Message:            "Claim is waiting for composite resource to become Ready",
	}
}
