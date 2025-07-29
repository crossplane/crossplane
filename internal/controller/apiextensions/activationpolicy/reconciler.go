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

// Package activationpolicy manages the lifecycle of MRAP controllers.
package activationpolicy

import (
	"context"
	"fmt"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	timeout = 2 * time.Minute

	errGetMRAP              = "cannot get ManagedResourceActivationPolicy"
	errListMRD              = "cannot list ManagedResourceDefinition"
	errUpdateStatus         = "cannot update status of ManagedResourceDefinition"
	errFailedToActivateMRDs = "failed to activate %d of %d ManagedResourceDefinitions"
)

// Event reasons.
const (
	reasonPaused       event.Reason = "ReconciliationPaused"
	reasonActivatedMRD event.Reason = "ActivateManagedResourceDefinition"

	// Messages.
	reconcilePausedMsg          = "Reconciliation is paused via the pause annotation"
	reconcileActivateSuccessMsg = "Successfully activated ManagedResourceDefinition"
)

// A Reconciler reconciles CompositeResourceDefinitions.
type Reconciler struct {
	client.Client

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager
}

// Reconcile a ManagedResourceActivationPolicy, matched ManagedResourceDefinitions are set to Active.
func (r *Reconciler) Reconcile(ogctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ogctx, timeout)
	defer cancel()

	mrap := &v1alpha1.ManagedResourceActivationPolicy{}
	if err := r.Get(ctx, req.NamespacedName, mrap); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetMRAP, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetMRAP)
	}

	status := r.conditions.For(mrap)

	log = log.WithValues(
		"uid", mrap.GetUID(),
		"version", mrap.GetResourceVersion(),
		"name", mrap.GetName(),
	)

	if meta.WasDeleted(mrap) {
		status.MarkConditions(v1alpha1.TerminatingActivationPolicy())
		if err := r.Status().Update(ogctx, mrap); err != nil {
			log.Debug(errUpdateStatus, "error", err)
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err = errors.Wrap(err, errUpdateStatus)
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
	// Check for pause annotation
	if meta.IsPaused(mrap) {
		log.Info("reconciliation is paused")
		r.record.Event(mrap, event.Normal(reasonPaused, reconcilePausedMsg))
		return reconcile.Result{}, nil
	}

	// List all MRDs
	mrds := &v1alpha1.ManagedResourceDefinitionList{}
	if err := r.List(ctx, mrds); err != nil {
		log.Debug(errListMRD, "error", err)

		status.MarkConditions(v1alpha1.BlockedActivationPolicy().WithMessage(errListMRD))
		if err := r.Status().Update(ogctx, mrap); err != nil {
			log.Debug(errUpdateStatus, "error", err)
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err = errors.Wrap(err, errUpdateStatus)
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, errors.Wrap(err, errListMRD)
	}

	// Start fresh.
	mrap.Status.ClearActivated()

	// For each, see if it is activated by the activation policy.
	var errs []error
	for _, mrd := range mrds.Items {
		if mrap.Activates(mrd.GetName()) {
			if mrd.Spec.State != v1alpha1.ManagedResourceDefinitionActive {
				orig := mrd.DeepCopy()
				mrd.Spec.State = v1alpha1.ManagedResourceDefinitionActive
				// Patch to ignore any other updates. Just focused on the spec.state value.
				if err := r.Patch(ctx, &mrd, client.MergeFrom(orig)); err != nil {
					log.Debug("Error when patching the mrd to set state to active", "err", err)
					errs = append(errs, err)
					r.record.Event(mrap, event.Warning(reasonActivatedMRD, err))
					continue
				}
				r.record.Event(mrap, event.Normal(reasonActivatedMRD, reconcileActivateSuccessMsg))
			}
			mrap.Status.AppendActivated(mrd.GetName())
		}
	}
	if errs != nil {
		status.MarkConditions(v1alpha1.Unhealthy().WithMessage(
			fmt.Sprintf(errFailedToActivateMRDs, len(errs), len(mrap.Status.Activated))))
	} else {
		status.MarkConditions(v1alpha1.Healthy())
	}

	// TODO: we should really do a diff of the status to see if we should update or not.
	return reconcile.Result{}, errors.Wrap(r.Status().Update(ogctx, mrap), errUpdateStatus)
}
