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
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured2 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/internal/names"
)

const (
	finalizer        = "finalizer.apiextensions.crossplane.io"
	reconcileTimeout = 1 * time.Minute
)

// Error strings.
const (
	errGetClaim                   = "cannot get composite resource claim"
	errGetComposite               = "cannot get referenced composite resource"
	errDeleteComposite            = "cannot delete referenced composite resource"
	errDeleteUnbound              = "refusing to delete composite resource that is not bound to this claim"
	errDeleteCDs                  = "cannot delete connection details"
	errRemoveFinalizer            = "cannot remove composite resource claim finalizer"
	errConfigureComposite         = "cannot configure composite resource"
	errPatchComposite             = "cannot patch composite resource"
	errFixFieldOwnershipComposite = "cannot fix field ownerships on composite resource"
	errFixFieldOwnershipClaim     = "cannot fix field ownerships on claim resource"
	errConfigureClaim             = "cannot configure composite resource claim"
	errPropagateCDs               = "cannot propagate connection details from composite"

	errUpdateClaimStatus = "cannot update composite resource claim status"

	reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"
)

// Event reasons.
const (
	reasonBind               event.Reason = "BindCompositeResource"
	reasonDelete             event.Reason = "DeleteCompositeResource"
	reasonCompositeConfigure event.Reason = "ConfigureCompositeResource"
	reasonClaimConfigure     event.Reason = "ConfigureClaim"
	reasonPropagate          event.Reason = "PropagateConnectionSecret"
	reasonPaused             event.Reason = "ReconciliationPaused"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite resource claim.
func ControllerName(name string) string {
	return "claim/" + name
}

type compositeConfiguratorFn func(ctx context.Context, cm *claim.Unstructured, cp, desiredCp *composite.Unstructured) error
type claimConfiguratorFn func(ctx context.Context, cm, desiredCm *claim.Unstructured, cp, desiredCp *composite.Unstructured) error

// A ConnectionPropagator is responsible for propagating information required to
// connect to a resource.
type ConnectionPropagator interface {
	PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error)
}

// A ConnectionPropagatorFn is responsible for propagating information required
// to connect to a resource.
type ConnectionPropagatorFn func(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error)

// PropagateConnection details from one resource to the other.
func (fn ConnectionPropagatorFn) PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
	return fn(ctx, to, from)
}

// A ConnectionPropagatorChain runs multiple connection propagators.
type ConnectionPropagatorChain []ConnectionPropagator

// PropagateConnection details from one resource to the other.
// This method calls PropagateConnection for all ConnectionPropagator's in the
// chain and returns propagated if at least one ConnectionPropagator propagates
// the connection details but exits with an error if any of them fails without
// calling the remaining ones.
func (pc ConnectionPropagatorChain) PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
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
	UnpublishConnection(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error
}

// A ConnectionUnpublisherFn is responsible for cleaning up connection secret.
type ConnectionUnpublisherFn func(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error

// UnpublishConnection details of a local connection secret owner.
func (fn ConnectionUnpublisherFn) UnpublishConnection(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error {
	return fn(ctx, so, c)
}

// A DefaultsSelector copies default values from the CompositeResourceDefinition when the corresponding field
// in the Claim is not set.
type DefaultsSelector interface {
	// SelectDefaults from CompositeResourceDefinition when needed.
	SelectDefaults(ctx context.Context, cm resource.CompositeClaim) error
}

// A DefaultsSelectorFn is responsible for copying default values from the CompositeResourceDefinition
type DefaultsSelectorFn func(ctx context.Context, cm resource.CompositeClaim) error

// SelectDefaults copies default values from the XRD if necessary
func (fn DefaultsSelectorFn) SelectDefaults(ctx context.Context, cm resource.CompositeClaim) error {
	return fn(ctx, cm)
}

// A Reconciler reconciles composite resource claims by creating exactly one kind of
// concrete composite resource. Each composite resource claim kind should create an instance
// of this controller for each composite resource kind they can bind to, using
// watch predicates to ensure each controller is responsible for exactly one
// type of resource class provisioner. Each controller must watch its subset of
// composite resource claims and any composite resources they control.
type Reconciler struct {
	client       client.Client
	newClaim     func() *claim.Unstructured
	newComposite func() *composite.Unstructured

	// The below structs embed the set of interfaces used to implement the
	// composite resource claim reconciler. We do this primarily for readability, so that
	// the reconciler logic reads r.composite.Create(), r.claim.Finalize(), etc.
	composite crComposite
	claim     crClaim

	log          logging.Logger
	record       event.Recorder
	pollInterval time.Duration
	fieldOwner   client.FieldOwner
	patchOptions []client.PatchOption
}

type crComposite struct {
	Configure compositeConfiguratorFn
	ConnectionPropagator
}

func defaultCRComposite(c client.Client) crComposite {
	conf := &apiCompositeConfigurator{NameGenerator: names.NewNameGenerator(c)}
	return crComposite{
		Configure:            conf.Configure,
		ConnectionPropagator: NewAPIConnectionPropagator(c),
	}
}

type crClaim struct {
	resource.Finalizer
	Configure claimConfiguratorFn
	ConnectionUnpublisher
}

func defaultCRClaim(c client.Client) crClaim {
	return crClaim{
		Finalizer:             resource.NewAPIFinalizer(c, finalizer),
		ConnectionUnpublisher: NewNopConnectionUnpublisher(),
		Configure:             configureClaim,
	}
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithClient specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClient(c client.Client) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = c
	}
}

// withCompositeConfigurator specifies how the Reconciler should configure the bound
// composite resource.
func withCompositeConfigurator(cf compositeConfiguratorFn) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Configure = cf
	}
}

// WithClaimConfigurator specifies how the Reconciler should configure the bound
// claim resource.
func withClaimConfigurator(cf claimConfiguratorFn) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Configure = cf
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
func NewReconciler(m manager.Manager, of resource.CompositeClaimKind, with resource.CompositeKind, o ...ReconcilerOption) *Reconciler {
	c := unstructured.NewClient(m.GetClient())
	r := &Reconciler{
		client: c,
		newClaim: func() *claim.Unstructured {
			return claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind(of)))
		},
		newComposite: func() *composite.Unstructured {
			return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(with)))
		},
		composite:  defaultCRComposite(c),
		claim:      defaultCRClaim(c),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		fieldOwner: client.FieldOwner(fmt.Sprintf("crossplane-claim-controller/%s", strings.ToLower(schema.GroupVersionKind(of).GroupKind().String()))),
	}

	for _, ro := range o {
		ro(r)
	}

	r.patchOptions = []client.PatchOption{client.ForceOwnership, r.fieldOwner}

	return r
}

// Reconcile a composite resource claim with a concrete composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Complexity is tough to avoid here.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	cm := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, cm); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise
		// we'll be requeued implicitly because we return an error.
		log.Debug(errGetClaim, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetClaim)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(cm))
	log = log.WithValues(
		"uid", cm.GetUID(),
		"version", cm.GetResourceVersion(),
		"external-name", meta.GetExternalName(cm),
	)

	// Previous versions of crossplane did not use server-side apply for updating claims.
	// We need to fix the manager name so that future reconciliations do not
	// create shared field ownership between old and actual managers
	if ownershipFixed := r.maybeFixFieldOwnership(cm.GetUnstructured(), true); ownershipFixed {
		if err := r.client.Update(ctx, cm); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			record.Event(cm, event.Warning(reasonClaimConfigure, err))
			return reconcile.Result{Requeue: true}, errors.Wrap(err, errFixFieldOwnershipClaim)
		}
	}

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(cm) {
		r.record.Event(cm, event.Normal(reasonPaused, reconcilePausedMsg))
		cm.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
	}

	cp := r.newComposite()
	if ref := cm.GetResourceReference(); ref != nil {
		record = record.WithAnnotations("composite-name", cm.GetResourceReference().Name)
		log = log.WithValues("composite-name", cm.GetResourceReference().Name)

		if err := r.client.Get(ctx, meta.NamespacedNameOf(ref), cp); resource.IgnoreNotFound(err) != nil {
			err = errors.Wrap(err, errGetComposite)
			record.Event(cm, event.Warning(reasonBind, err))
			cm.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
		}
	}

	if meta.WasDeleted(cm) {
		log = log.WithValues("deletion-timestamp", cm.GetDeletionTimestamp())

		cm.SetConditions(xpv1.Deleting())
		if meta.WasCreated(cp) {
			requiresForegroundDeletion := false
			if cdp := cm.GetCompositeDeletePolicy(); cdp != nil && *cdp == xpv1.CompositeDeleteForeground {
				requiresForegroundDeletion = true
			}
			if meta.WasDeleted(cp) {
				if requiresForegroundDeletion {
					log.Debug("Waiting for the Composite to finish deleting (foreground deletion)")
					return reconcile.Result{Requeue: true}, nil
				}
			}
			ref := cp.GetClaimReference()
			want := cm.GetReference()
			if !cmp.Equal(want, ref) {
				// We don't requeue (or return an error, which
				// would requeue) in this situation because the
				// claim will need human intervention before we
				// can proceed (e.g. fixing the ref), and we'll
				// be queued implicitly when the claim is
				// edited.
				err := errors.New(errDeleteUnbound)
				record.Event(cm, event.Warning(reasonDelete, err))
				cm.SetConditions(xpv1.ReconcileError(err))
				return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
			}

			do := &client.DeleteOptions{}
			if requiresForegroundDeletion {
				client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(do)
			}
			if err := r.client.Delete(ctx, cp, do); resource.IgnoreNotFound(err) != nil {
				err = errors.Wrap(err, errDeleteComposite)
				record.Event(cm, event.Warning(reasonDelete, err))
				cm.SetConditions(xpv1.ReconcileError(err))
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
			}
			if requiresForegroundDeletion {
				log.Debug("Requeue to wait for the Composite to finish deleting (foreground deletion)")
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
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
		}

		record.Event(cm, event.Normal(reasonDelete, "Successfully deleted composite resource"))

		if err := r.claim.RemoveFinalizer(ctx, cm); err != nil {
			err = errors.Wrap(err, errRemoveFinalizer)
			record.Event(cm, event.Warning(reasonDelete, err))
			cm.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
		}

		log.Debug("Successfully deleted composite resource claim")
		cm.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
	}

	// If composite exists, i.e. got created in the previous iteration,
	// it might be needed to fix field ownerships,
	// so that the claim controller becomes an exclusive owner
	// of the fields mirrored from the claim to the composite
	if meta.WasCreated(cp) {
		if ownershipFixed := r.maybeFixFieldOwnership(cp.GetUnstructured(), false); ownershipFixed {
			if err := r.client.Update(ctx, cp); err != nil {
				if kerrors.IsConflict(err) {
					return reconcile.Result{Requeue: true}, nil
				}
				record.Event(cp, event.Warning(reasonCompositeConfigure, err))
				return reconcile.Result{Requeue: true}, errors.Wrap(err, errFixFieldOwnershipComposite)
			}
		}
	}

	// create object that is going to hold the full claim patch eventually
	cmPatch := r.newClaim()
	cmPatch.SetName(cm.GetName())
	cmPatch.SetNamespace(cm.GetNamespace())

	meta.AddFinalizer(cmPatch, finalizer)

	// create object that is going to hold the full composite patch eventually
	cpPatch := r.newComposite()
	if err := r.composite.Configure(ctx, cm, cp, cpPatch); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		if errors.Is(err, ErrBindCompositeConflict) {
			// the claim refers to a composite belonging to a different claim
			// this case can occur if:
			// 1. composite name gets generated
			// 2. claim sets and persists the reference to composite with the generated name
			// 3. composite creation fails because the generated name is already taken
			// 4. in the next reconcile loop we get the above conflict
			// to unblock us, we need to remove the composite reference at the claim
			// otherwise, we can move forward even if we requeue
			_ = r.client.Patch(ctx, cmPatch, client.Apply, r.patchOptions...)
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errConfigureComposite)
		record.Event(cm, event.Warning(reasonCompositeConfigure, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
	}

	// We'll know our composite resource's name at this point because it was
	// set by the above configure step.
	record = record.WithAnnotations("composite-name", cpPatch.GetName())
	log = log.WithValues("composite-name", cpPatch.GetName())

	if err := r.claim.Configure(ctx, cm, cmPatch, cp, cpPatch); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errConfigureClaim)
		record.Event(cm, event.Warning(reasonClaimConfigure, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
	}

	// The following patch operation is going to override the status part of the claim patch
	// with the status of the actual claim version.
	// given that the status update come later, we need to preserve it temporarily.
	desiredClaimStatus := cmPatch.Object["status"]
	err := r.client.Patch(ctx, cmPatch, client.Apply, r.patchOptions...)
	if err != nil {
		return reconcile.Result{}, err
	}

	cm = cmPatch
	// Restore the status calculated in this round
	// and use cmPatch until the end of the reconciliation
	// for setting claim conditions
	cm.Object["status"] = desiredClaimStatus

	// If composite did not exist at the beginning of the loop, we want to create it
	// but there is a probability (very unlikely) that the generated name
	// gets taken before we submit the request. In that case, we are going to
	// update a composite, hijacking it from another claim.
	// TODO: investigate if we need to prevent it.
	err = r.client.Patch(ctx, cpPatch, client.Apply, r.patchOptions...)

	switch {
	case resource.IsNotAllowed(err):
		log.Debug("Skipped no-op composite resource server-side apply")
	case err != nil:
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errPatchComposite)
		record.Event(cm, event.Warning(reasonCompositeConfigure, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
	default:
		cp = cpPatch
		record.Event(cm, event.Normal(reasonCompositeConfigure, "Successfully patched composite resource"))
	}

	cm.SetConditions(xpv1.ReconcileSuccess())

	if !resource.IsConditionTrue(cp.GetCondition(xpv1.TypeReady)) {
		record.Event(cm, event.Normal(reasonBind, "Composite resource is not yet ready"))

		// We should be watching the composite resource and will have a
		// request queued if it changes, so no need to requeue.
		cm.SetConditions(Waiting())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
	}

	record.Event(cm, event.Normal(reasonBind, "Successfully bound composite resource"))

	propagated, err := r.composite.PropagateConnection(ctx, cm, cp)
	if err != nil {
		err = errors.Wrap(err, errPropagateCDs)
		record.Event(cm, event.Warning(reasonPropagate, err))
		cm.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
	}
	if propagated {
		cm.SetConnectionDetailsLastPublishedTime(&metav1.Time{Time: time.Now()})
		record.Event(cm, event.Normal(reasonPropagate, "Successfully propagated connection details from composite resource"))
	}

	// We have a watch on both the claim and its composite, so there's no
	// need to requeue here.
	cm.SetConditions(xpv1.Available())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cm, r.fieldOwner), errUpdateClaimStatus)
}

// For the background, check https://github.com/kubernetes/kubernetes/issues/99003
// Managed fields owner key is a pair (the manager name, the used operation).
// Initially, we create a composite using CREATE request, because that is the only
// way to figure out if the generated name is already taken.
// If the operation is successful, managed fields operation will be set to "Update".
// Later patches are going to produce another managed fields set for the same manager,
// but with a different operation "Apply".
// Such configuration would prevent removal of claim fields on the composite, i.e.
// it would only remove the ownership.
// In order to fix that, we need to manually change operation to "Apply",
// before the first composite patch is sent to k8s api server.
// Returns true if the ownership was fixed.
func (r *Reconciler) maybeFixFieldOwnership(obj *unstructured2.Unstructured, fixSubresource bool) bool {
	mfs := obj.GetManagedFields()
	fixed := false
	for i := range mfs {
		if mfs[i].Subresource != "" && !fixSubresource {
			continue
		}
		if mfs[i].Manager == string(r.fieldOwner) && mfs[i].Operation == "Update" {
			mfs[i].Operation = "Apply"
			obj.SetManagedFields(mfs)
			fixed = true
			continue
		}

		// Even if the server-side apply is not used, api server derives manager name
		// from the submitted user agent (see net/http/request.go).
		// After crossplane update, we need to replace the ownership so that
		// field removals can be propagated properly.
		if mfs[i].Manager == "Go-http-client" && mfs[i].Operation == "Update" {
			mfs[i].Operation = "Apply"
			mfs[i].Manager = string(r.fieldOwner)
			obj.SetManagedFields(mfs)
			fixed = true
		}
	}
	return fixed

}

// Waiting returns a condition that indicates the composite resource claim is
// currently waiting for its composite resource to become ready.
func Waiting() xpv1.Condition {
	return xpv1.Condition{
		Type:               xpv1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             xpv1.ConditionReason("Waiting"),
		Message:            "Composite resource claim is waiting for composite resource to become Ready",
	}
}
