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

// Package managed manages the lifecycle of MRD controllers.
package managed

import (
	"context"
	"time"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/ssa"
	"github.com/crossplane/crossplane/v2/internal/xcrd"
)

const (
	timeout = 2 * time.Minute
)

// Event reasons.
const (
	reasonReconcile event.Reason = "Reconcile"
	reasonPaused    event.Reason = "ReconciliationPaused"
	reasonApplyCRD  event.Reason = "ApplyCustomResourceDefinition"
	reasonDeleteCRD event.Reason = "DeleteCustomResourceDefinition"
)

// FieldOwnerMRD is the field manager name used when applying CRDs.
const FieldOwnerMRD = "apiextensions.crossplane.io/managed"

// A Reconciler reconciles ManagedResourceDefinitions.
type Reconciler struct {
	client client.Client

	managedFields ssa.ManagedFieldsUpgrader

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager
}

// Reconcile a CompositeResourceDefinition by defining a new kind of composite
// resource and starting a controller to reconcile it.
func (r *Reconciler) Reconcile(ogctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ogctx, timeout)
	defer cancel()

	mrd := &v1alpha1.ManagedResourceDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, mrd); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug("cannot get ManagedResourceDefinition", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get ManagedResourceDefinition")
	}

	status := r.conditions.For(mrd)

	log = log.WithValues(
		"uid", mrd.GetUID(),
		"version", mrd.GetResourceVersion(),
		"name", mrd.GetName(),
	)

	if meta.WasDeleted(mrd) {
		status.MarkConditions(v1alpha1.TerminatingManaged())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
	}
	// Check for pause annotation
	if meta.IsPaused(mrd) {
		log.Info("reconciliation is paused")
		r.record.Event(mrd, event.Normal(reasonPaused, "Reconciliation is paused via the pause annotation"))
		return reconcile.Result{}, nil
	}

	if !mrd.Spec.State.IsActive() {
		return r.reconcileInactive(ogctx, ctx, mrd, status, log)
	}

	// Read the CRD to upgrade its managed fields if needed.
	crd := &extv1.CustomResourceDefinition{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: mrd.GetName()}, crd); resource.IgnoreNotFound(err) != nil {
		log.Debug("cannot get CustomResourceDefinition", "error", err)
		r.record.Event(mrd, event.Warning(reasonReconcile, err))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to get CustomResourceDefinition, see events"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.Wrap(err, "cannot get CustomResourceDefinition")
	}

	// Upgrade the CRD's managed fields from client-side to server-side
	// apply. This is necessary when a CRD was previously managed using
	// client-side apply, but should now be managed using server-side apply.
	if err := r.managedFields.Upgrade(ctx, crd); err != nil {
		log.Debug("cannot upgrade managed fields", "error", err)
		r.record.Event(mrd, event.Warning(reasonReconcile, err))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to upgrade managed fields, see events"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.Wrap(err, "cannot upgrade managed fields")
	}

	// Form the desired CRD from the MRD as Unstructured. We use
	// Unstructured to ensure we only serialize fields we have opinions
	// about for server-side apply. Using typed CRDs can cause issues with
	// zero values and defaults being interpreted as desired state.
	patch, err := CRDAsUnstructured(mrd)
	if err != nil {
		log.Debug("cannot form CustomResourceDefinition", "error", err)
		r.record.Event(mrd, event.Warning(reasonReconcile, err))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to form CustomResourceDefinition, see events"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.Wrap(err, "cannot form CustomResourceDefinition")
	}

	// Server-side apply the CRD. This handles both create and update.
	// The Patch call updates patch in-place with the server response.
	//
	//nolint:staticcheck // TODO(adamwg): Stop using client.Apply after the v2.2 release.
	if err := r.client.Patch(ctx, patch, client.Apply, client.ForceOwnership, client.FieldOwner(FieldOwnerMRD)); err != nil {
		log.Debug("cannot apply CustomResourceDefinition", "error", err)
		r.record.Event(mrd, event.Warning(reasonApplyCRD, err))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to apply CustomResourceDefinition, see events"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.Wrap(err, "cannot apply CustomResourceDefinition")
	}

	r.record.Event(mrd, event.Normal(reasonApplyCRD, "Successfully applied CustomResourceDefinition"))

	// Convert the unstructured response to typed CRD to check status.
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(patch.Object, crd); err != nil {
		log.Debug("cannot convert CustomResourceDefinition from unstructured", "error", err)
		r.record.Event(mrd, event.Warning(reasonReconcile, err))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to form CustomResourceDefinition, see events"))
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
	}

	if !xcrd.IsEstablished(crd.Status) {
		log.Debug("waiting for managed resource CustomResourceDefinition to be established")
		status.MarkConditions(v1alpha1.PendingManaged())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
	}

	status.MarkConditions(v1alpha1.EstablishedManaged())
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
}

// reconcileInactive handles MRD deactivation by cleaning up the CRD when no
// managed resource instances exist.
func (r *Reconciler) reconcileInactive(ogctx, ctx context.Context, mrd *v1alpha1.ManagedResourceDefinition, status conditions.ConditionSet, log logging.Logger) (reconcile.Result, error) {
	// Check if the CRD exists.
	crd := &extv1.CustomResourceDefinition{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: mrd.GetName()}, crd); err != nil {
		if kerrors.IsNotFound(err) {
			// CRD doesn't exist — was never active, or already cleaned up.
			status.MarkConditions(v1alpha1.InactiveManaged())
			return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
		}
		log.Debug("cannot get CustomResourceDefinition", "error", err)
		r.record.Event(mrd, event.Warning(reasonReconcile, err))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to get CustomResourceDefinition, see events"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.Wrap(err, "cannot get CustomResourceDefinition")
	}

	// CRD exists but is not controlled by this MRD — don't delete it.
	if !metav1.IsControlledBy(crd, mrd) {
		status.MarkConditions(v1alpha1.InactiveManaged())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
	}

	// CRD exists — check if any managed resource instances exist.
	ver := storageVersion(mrd)
	if ver == "" {
		log.Debug("MRD has no versions defined, cannot list managed resources")
		r.record.Event(mrd, event.Warning(reasonReconcile, errors.New("MRD has no versions defined")))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("MRD has no versions defined"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.New("MRD has no versions defined, cannot list managed resources")
	}
	gvk := schema.GroupVersionKind{
		Group:   mrd.Spec.Group,
		Version: ver,
		Kind:    mrd.Spec.Names.ListKind,
	}

	l := &kunstructured.UnstructuredList{}
	l.SetGroupVersionKind(gvk)

	if err := r.client.List(ctx, l, client.Limit(1)); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
		log.Debug("cannot list managed resources", "error", err, "gvk", gvk.String())
		r.record.Event(mrd, event.Warning(reasonReconcile, errors.Wrap(err, "cannot list managed resources")))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to list managed resources, see events"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.Wrap(err, "cannot list managed resources")
	}

	if len(l.Items) > 0 {
		log.Debug("waiting for managed resource instances to be deleted before removing CRD")
		r.record.Event(mrd, event.Normal(reasonDeleteCRD, "Waiting for managed resource instances to be deleted"))
		status.MarkConditions(v1alpha1.WaitingForInstanceDeletion().WithMessage("waiting for managed resource instances to be deleted"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{Requeue: true}, nil
	}

	// No instances — delete the CRD.
	if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
		log.Debug("cannot delete CustomResourceDefinition", "error", err)
		r.record.Event(mrd, event.Warning(reasonDeleteCRD, err))
		status.MarkConditions(v1alpha1.BlockedManaged().WithMessage("unable to delete CustomResourceDefinition, see events"))
		_ = r.client.Status().Update(ogctx, mrd)
		return reconcile.Result{}, errors.Wrap(err, "cannot delete CustomResourceDefinition")
	}

	log.Debug("Deleted CustomResourceDefinition for inactive MRD")
	r.record.Event(mrd, event.Normal(reasonDeleteCRD, "Deleted CustomResourceDefinition"))
	status.MarkConditions(v1alpha1.InactiveManaged())
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
}

// storageVersion returns the name of the storage version from the MRD spec.
func storageVersion(mrd *v1alpha1.ManagedResourceDefinition) string {
	for _, v := range mrd.Spec.Versions {
		if v.Storage {
			return v.Name
		}
	}
	// Fall back to the first version if no storage version is marked.
	if len(mrd.Spec.Versions) > 0 {
		return mrd.Spec.Versions[0].Name
	}
	return ""
}
