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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xcrd"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/engine"
	"github.com/crossplane/crossplane/v2/internal/features"
	"github.com/crossplane/crossplane/v2/internal/ssa"
)

const (
	timeout = 2 * time.Minute
)

// Event reasons.
const (
	reasonReconcile event.Reason = "Reconcile"
	reasonPaused    event.Reason = "ReconciliationPaused"
	reasonApplyCRD  event.Reason = "ApplyCustomResourceDefinition"
)

// FieldOwnerMRD is the field manager name used when applying CRDs.
const FieldOwnerMRD = "apiextensions.crossplane.io/managed"

// A Reconciler reconciles ManagedResourceDefinitions.
type Reconciler struct {
	client client.Client

	managedFields ssa.ManagedFieldsUpgrader

	// engine is used to dynamically start protection controllers that watch
	// MR instances when provider deletion protection is enabled. It is nil
	// when the feature is disabled.
	engine   *engine.ControllerEngine
	features *feature.Flags

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
		r.cleanupProtection(ctx, log, mrd.GetName())
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
		r.cleanupProtection(ctx, log, mrd.GetName())
		status.MarkConditions(v1alpha1.InactiveManaged())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
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

	// The CRD's controller follows the MRD's, so an upgrade hands the CRD
	// over between package revisions just as it hands the MRD over. See
	// handOverControl for why the apply below can't do that on its own.
	handOverControl(patch, crd)

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

	// If provider deletion protection is enabled, start a protection
	// controller that watches MR instances and manages ClusterUsage objects.
	if r.protectionEnabled() {
		if err := r.startProtection(ctx, log, mrd); err != nil {
			log.Debug("Cannot start provider deletion protection", "error", err)
			r.record.Event(mrd, event.Warning(reasonReconcile, err))
			return reconcile.Result{}, errors.Wrap(err, "cannot start provider deletion protection")
		}
	}

	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
}

// handOverControl prepares patch, the CRD we are about to apply, to take
// control of crd, the live CRD, from the package revision that still controls
// it, by declaring that revision's owner reference demoted to a plain owner.
//
// Server-side apply can't complete the hand-off on its own when the outgoing
// revision's reference was written by another field manager, as an older
// Crossplane's establisher did client-side: owner references are merged by
// UID and only entries we own are pruned, so the old entry survives our apply,
// and declaring a second controller beside it is rejected, since only one
// owner reference may control an object. Declaring the old entry demoted sets
// the new controller and demotes the old one in one write, and gives our field
// manager the entry, so a later apply that no longer declares it prunes it.
//
// Only a package revision of the same group and kind as the MRD's controller
// is taken over. The MRD decides which revision controls what is derived from
// it, and the establisher only hands it over between revisions of the same
// package, so the CRD follows it. Anything else keeps control, and the apply
// fails loudly rather than take the CRD from it.
func handOverControl(patch, crd metav1.Object) {
	want := metav1.GetControllerOf(patch)
	got := metav1.GetControllerOf(crd)

	if want == nil || got == nil || got.UID == want.UID {
		return
	}

	// Compare group and kind rather than the whole API version, since the API
	// server may serve the revision's type at more than one.
	if schema.FromAPIVersionAndKind(got.APIVersion, got.Kind).GroupKind() != schema.FromAPIVersionAndKind(want.APIVersion, want.Kind).GroupKind() {
		return
	}

	got.Controller = new(false)
	meta.AddOwnerReference(patch, *got)
}

// protectionEnabled returns true if provider deletion protection is enabled.
func (r *Reconciler) protectionEnabled() bool {
	return r.engine != nil && r.features != nil &&
		r.features.Enabled(features.EnableAlphaProviderDeletionProtection)
}

// startProtection starts a protection controller for the given MRD that
// watches MR instances and creates/deletes a ClusterUsage to protect the
// owning Provider from deletion.
func (r *Reconciler) startProtection(ctx context.Context, log logging.Logger, mrd *v1alpha1.ManagedResourceDefinition) error {
	mrGVK := schema.GroupVersionKind{
		Group:   mrd.Spec.Group,
		Version: storageVersion(mrd),
		Kind:    mrd.Spec.Names.Kind,
	}

	controllerName := ProtectionControllerName(mrd.GetName())

	pr := &ProtectionReconciler{
		cached:  r.engine.GetCached(),
		writer:  r.client,
		mrdName: mrd.GetName(),
		gvk:     mrGVK,
		log:     log.WithValues("controller", controllerName),
	}

	ko := kcontroller.Options{Reconciler: pr}

	//nolint:contextcheck // Start intentionally does not take a context; it creates its own so the controller outlives the caller.
	if err := r.engine.Start(controllerName,
		engine.WithRuntimeOptions(ko),
	); err != nil {
		return errors.Wrap(err, "cannot start protection controller")
	}

	// Start a watch on MR instances. This is idempotent - it only starts
	// watches that don't already exist.
	mr := &kunstructured.Unstructured{}
	mr.SetGroupVersionKind(mrGVK)

	h := handler.EnqueueRequestsFromMapFunc(ResourceMapFunc(mrd.GetName()))

	if err := r.engine.StartWatches(ctx, controllerName,
		engine.WatchFor(mr, engine.WatchTypeManagedResource, h),
	); err != nil {
		return errors.Wrap(err, "cannot start managed resource watch")
	}

	return nil
}

// cleanupProtection stops the protection controller and deletes the
// ClusterUsage for the given MRD. It is a no-op if provider deletion
// protection is not enabled.
func (r *Reconciler) cleanupProtection(ctx context.Context, log logging.Logger, mrdName string) {
	if !r.protectionEnabled() {
		return
	}

	controllerName := ProtectionControllerName(mrdName)
	if err := r.engine.Stop(ctx, controllerName); err != nil {
		log.Debug("Cannot stop protection controller", "error", err)
	}

	cu := &protectionv1beta1.ClusterUsage{}
	cu.SetName(ClusterUsageName(mrdName))
	if err := r.client.Delete(ctx, cu); resource.IgnoreNotFound(err) != nil {
		log.Debug("Cannot delete ClusterUsage", "error", err)
	}
}
