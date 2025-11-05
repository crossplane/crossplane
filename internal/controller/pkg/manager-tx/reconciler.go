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
	"context"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
)

const (
	reconcileTimeout = 1 * time.Minute

	reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"
)

// Event reasons.
const (
	reasonCreateTransaction event.Reason = "CreateTransaction"
	reasonTransactionStatus event.Reason = "TransactionStatus"
	reasonPaused            event.Reason = "ReconciliationPaused"
)

// A Reconciler reconciles packages by creating and monitoring Transactions.
type Reconciler struct {
	client     client.Client
	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager
	pkg        v1.Package             // Template package to deep copy for each reconcile
	list       v1.PackageRevisionList // Template revision list to deep copy for each reconcile
}

// Reconcile a package by creating Transactions and reflecting Transaction status.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	// TODO(negz): We won't update status if this times out.
	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	// Get the package by deep copying the template
	pkg := r.pkg.DeepCopyObject().(v1.Package) //nolint:forcetypeassert // Will always be a package.
	if err := r.client.Get(ctx, req.NamespacedName, pkg); err != nil {
		log.Debug("cannot get package", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get package")
	}

	status := r.conditions.For(pkg)

	// Check the pause annotation and return if it has the value "true"
	if meta.IsPaused(pkg) {
		r.record.Event(pkg, event.Normal(reasonPaused, reconcilePausedMsg))
		status.MarkConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pkg), "cannot update package status")
	}

	if c := pkg.GetCondition(xpv1.TypeSynced); c.Reason == xpv1.ReasonReconcilePaused {
		pkg.CleanConditions()
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pkg), "cannot update package status")
	}

	// Determine change type and create Transaction
	changeType := v1alpha1.ChangeTypeInstall
	if meta.WasDeleted(pkg) {
		changeType = v1alpha1.ChangeTypeDelete
	}

	log = log.WithValues("package", pkg.GetName(), "source", pkg.GetSource(), "changeType", changeType)

	// Check if this Package generation has already been handled by a Transaction.
	// If the transaction-generation label matches the current generation, we skip
	// creating a new Transaction and just reflect the existing Transaction's status.
	handled := pkg.GetLabels()[v1alpha1.LabelTransactionGeneration]
	current := strconv.FormatInt(pkg.GetGeneration(), 10)

	// Create or get existing Transaction for this package change
	name := pkg.GetLabels()[v1alpha1.LabelTransactionName]
	if name == "" {
		name = TransactionName(pkg)
	}

	tx := &v1alpha1.Transaction{}
	err := r.client.Get(ctx, types.NamespacedName{Name: name}, tx)
	if resource.IgnoreNotFound(err) != nil {
		err = errors.Wrap(err, "cannot check for existing transaction")
		r.record.Event(pkg, event.Warning(reasonCreateTransaction, err))
		return reconcile.Result{}, err
	}

	if kerrors.IsNotFound(err) && handled != current {
		tx = NewTransaction(pkg, changeType)
		if err := r.client.Create(ctx, tx); err != nil {
			err = errors.Wrap(err, "cannot create transaction")
			r.record.Event(pkg, event.Warning(reasonCreateTransaction, err))
			return reconcile.Result{}, err
		}

		// Label the package to indicate we've created a Transaction for this generation
		meta.AddLabels(pkg, map[string]string{
			v1alpha1.LabelTransactionName:       tx.GetName(),
			v1alpha1.LabelTransactionGeneration: current,
		})
		if err := r.client.Update(ctx, pkg); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot update package labels")
		}
	}

	// Handle deletion completion
	if meta.WasDeleted(pkg) {
		if TransactionComplete(tx) {
			log.Debug("Delete transaction complete, removing finalizer", "transaction", name)
			// Note: Finalizer removal logic would go here if we add finalizers
			return reconcile.Result{}, nil
		}

		// Transaction not complete - we'll be requeued when it updates
		log.Debug("Delete transaction not complete, waiting for update", "transaction", name)
		return reconcile.Result{}, nil
	}

	// Update package status based on the active PackageRevision, not the
	// Transaction. The transaction handles installation, but package health
	// and status should reflect the actual running revision.
	revs := r.list.DeepCopyObject().(v1.PackageRevisionList) //nolint:forcetypeassert // Will always be a package revision list.

	if err := r.client.List(ctx, revs, client.MatchingLabels{v1.LabelParentPackage: pkg.GetName()}); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot list package revisions")
	}

	// Find the active revision if one exists
	var active v1.PackageRevision
	for _, rev := range revs.GetRevisions() {
		if rev.GetDesiredState() == v1.PackageRevisionActive {
			active = rev
			break
		}
	}

	if active == nil {
		status.MarkConditions(v1.Inactive())
		status.MarkConditions(v1alpha1.PackageTransacted(tx))
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pkg), "cannot update package status")
	}

	// Set Installed condition based on whether an active revision exists
	status.MarkConditions(v1.Active())

	// Set health from the active revision (if one exists)
	health := v1.PackageHealth(active)
	if health.Status == corev1.ConditionTrue && pkg.GetCondition(v1.TypeHealthy).Status != corev1.ConditionTrue {
		r.record.Event(pkg, event.Normal(reasonTransactionStatus, "Successfully installed package revision"))
	}
	status.MarkConditions(health)

	// Set Transacted condition based on the transaction status
	status.MarkConditions(v1alpha1.PackageTransacted(tx))

	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pkg), "cannot update package status")
}

// TransactionComplete returns true if the Transaction has completed (successfully or with failure).
func TransactionComplete(tx *v1alpha1.Transaction) bool {
	c := tx.Status.GetCondition(v1alpha1.TypeSucceeded)
	return c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse
}
