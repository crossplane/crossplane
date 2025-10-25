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

package transaction

import (
	"context"
	"time"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

const (
	timeout = 5 * time.Minute

	defaultRetryLimit = 5
)

const (
	reasonLockAcquisition = "LockAcquisition"
	reasonDependencySolve = "DependencySolve"
	reasonValidation      = "Validation"
	reasonInstallation    = "Installation"
)

// DependencySolver resolves package dependencies to concrete digests.
type DependencySolver interface {
	// Solve takes a package reference and the current Lock state, then:
	// - Fetches the package from the OCI registry
	// - Resolves tags to specific digests
	// - Recursively resolves all dependencies
	// - Validates version constraints are satisfiable
	// - Detects circular dependencies
	// - Returns the complete proposed Lock state with all packages at specific digests
	Solve(ctx context.Context, source string, currentLock []v1beta1.LockPackage) ([]v1beta1.LockPackage, error)
}

// Installer installs packages by creating Package and PackageRevision resources
// and establishing control of CRDs and other package objects.
type Installer interface {
	InstallPackages(ctx context.Context, tx *v1alpha1.Transaction) error
}

// A Reconciler reconciles Transactions.
type Reconciler struct {
	client client.Client

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager

	lock      LockManager
	solver    DependencySolver
	validator Validator
	installer Installer
}

// Reconcile a Transaction by acquiring the lock, resolving dependencies,
// validating, installing packages, and committing the lock.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tx := &v1alpha1.Transaction{}
	if err := r.client.Get(ctx, req.NamespacedName, tx); err != nil {
		log.Debug("cannot get Transaction", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get Transaction")
	}

	status := r.conditions.For(tx)

	log = log.WithValues(
		"uid", tx.GetUID(),
		"version", tx.GetResourceVersion(),
		"name", tx.GetName(),
	)

	if meta.WasDeleted(tx) {
		return reconcile.Result{}, nil
	}

	if tx.IsComplete() {
		log.Debug("Transaction is already complete")
		return reconcile.Result{}, nil
	}

	limit := ptr.Deref(tx.Spec.RetryLimit, defaultRetryLimit)
	if tx.Status.Failures >= limit {
		log.Debug("Transaction failure limit reached", "limit", limit)

		// Release the lock if we're holding it - retry if this fails
		if err := r.lock.Release(ctx, tx); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot release lock after hitting failure limit")
		}

		status.MarkConditions(
			xpv1.ReconcileSuccess(),
			v1alpha1.TransactionFailed("failure limit reached"),
		)
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, tx), "cannot update Transaction status")
	}

	status.MarkConditions(v1alpha1.TransactionRunning())
	if err := r.client.Status().Update(ctx, tx); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Transaction status")
	}

	currentPackages, err := r.lock.Acquire(ctx, tx)
	if err != nil {
		if errors.Is(err, ErrLockHeldByAnotherTransaction) {
			log.Debug("Lock is held by another transaction, will retry when lock is released")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrap(err, "cannot acquire lock")
	}

	if err := r.client.Status().Update(ctx, tx); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Transaction status with transaction number")
	}

	var source string
	switch tx.Spec.Change {
	case v1alpha1.ChangeTypeInstall:
		source = tx.Spec.Install.Package.Spec.Package
	case v1alpha1.ChangeTypeDelete:
		source = tx.Spec.Delete.Source
	case v1alpha1.ChangeTypeReplace:
		source = tx.Spec.Replace.Package.Spec.Package
	}

	proposedPackages, err := r.solver.Solve(ctx, source, currentPackages)
	if err != nil {
		log.Debug("cannot solve dependencies", "error", err)
		r.record.Event(tx, event.Warning(reasonDependencySolve, errors.Wrap(err, "cannot solve dependencies")))
		tx.Status.Failures++
		status.MarkConditions(
			xpv1.ReconcileError(errors.Wrap(err, "cannot solve dependencies")),
			v1alpha1.TransactionFailed("cannot solve dependencies"),
		)
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, tx), "cannot update Transaction status")
	}

	tx.Status.ProposedLockPackages = proposedPackages
	if err := r.client.Status().Update(ctx, tx); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Transaction status with proposed packages")
	}

	status.MarkConditions(v1alpha1.ValidationPassed())
	if err := r.client.Status().Update(ctx, tx); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Transaction status")
	}

	if err := r.validator.Validate(ctx, tx); err != nil {
		log.Debug("validation failed", "error", err)
		r.record.Event(tx, event.Warning(reasonValidation, errors.Wrap(err, "validation failed")))
		tx.Status.Failures++
		status.MarkConditions(
			xpv1.ReconcileError(errors.Wrap(err, "validation failed")),
			v1alpha1.TransactionFailed("validation failed"),
		)
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, tx), "cannot update Transaction status")
	}

	status.MarkConditions(v1alpha1.InstallationInProgress())
	if err := r.client.Status().Update(ctx, tx); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Transaction status")
	}

	if err := r.installer.InstallPackages(ctx, tx); err != nil {
		log.Debug("cannot install packages", "error", err)
		r.record.Event(tx, event.Warning(reasonInstallation, errors.Wrap(err, "cannot install packages")))
		tx.Status.Failures++
		status.MarkConditions(
			xpv1.ReconcileError(errors.Wrap(err, "cannot install packages")),
			v1alpha1.TransactionFailed("cannot install packages"),
		)
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, tx), "cannot update Transaction status")
	}

	status.MarkConditions(v1alpha1.InstallationComplete())
	if err := r.client.Status().Update(ctx, tx); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Transaction status")
	}

	if err := r.lock.Commit(ctx, tx, proposedPackages); err != nil {
		log.Debug("cannot commit lock", "error", err)
		r.record.Event(tx, event.Warning(reasonLockAcquisition, errors.Wrap(err, "cannot commit lock")))
		tx.Status.Failures++
		status.MarkConditions(
			xpv1.ReconcileError(errors.Wrap(err, "cannot commit lock")),
			v1alpha1.TransactionFailed("cannot commit lock"),
		)
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, tx), "cannot update Transaction status")
	}

	status.MarkConditions(
		xpv1.ReconcileSuccess(),
		v1alpha1.TransactionComplete(),
	)

	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, tx), "cannot update Transaction status")
}
