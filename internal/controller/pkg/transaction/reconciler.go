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
	"fmt"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/crossplane/crossplane/v2/internal/xpkg"
	"github.com/crossplane/crossplane/v2/internal/xpkg/dependency"
)

const (
	timeout = 10 * time.Minute

	defaultRetryLimit = 5
)

const (
	reasonLockAcquisition = "LockAcquisition"
	reasonDependencySolve = "DependencySolve"
	reasonValidation      = "Validation"
	reasonInstallation    = "Installation"
)

// LockManager manages exclusive access to the Lock resource for Transactions.
type LockManager interface {
	// Acquire attempts to gain exclusive access to the Lock for a Transaction.
	// Returns the current Lock packages (what's currently installed).
	// Returns ErrLockHeldByAnotherTransaction if lock is held by a different Transaction.
	// If the Transaction already holds the lock, returns the current Lock state.
	Acquire(ctx context.Context, tx *v1alpha1.Transaction) ([]v1beta1.LockPackage, error)

	// Commit releases exclusive access and updates Lock state with new packages.
	// Only the Transaction that currently holds the lock can commit it.
	Commit(ctx context.Context, tx *v1alpha1.Transaction, packages []v1beta1.LockPackage) error

	// Release releases the lock without updating packages (for failures/cancellations).
	// Returns nil if the lock was successfully released, or if the Transaction never held it.
	Release(ctx context.Context, tx *v1alpha1.Transaction) error
}

// DependencySolver resolves package dependencies to concrete digests.
type DependencySolver interface {
	Solve(ctx context.Context, name, source string, currentLock []v1beta1.LockPackage) ([]v1beta1.LockPackage, error)
}

// Validator validates a Transaction.
type Validator interface {
	Validate(ctx context.Context, tx *v1alpha1.Transaction) error
}

// A Reconciler reconciles Transactions.
type Reconciler struct {
	kube client.Client
	pkg  xpkg.Client

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
	if err := r.kube.Get(ctx, req.NamespacedName, tx); err != nil {
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

		status.MarkConditions(xpv1.ReconcileSuccess(), v1alpha1.TransactionFailed(fmt.Sprintf("failure limit of %d reached", limit)))
		return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, tx), "cannot update Transaction status")
	}

	status.MarkConditions(v1alpha1.TransactionRunning())

	// Delete and Replace are not yet implemented. Fast-fail these transactions
	// with a clear error message.
	if tx.Spec.Change == v1alpha1.ChangeTypeDelete {
		status.MarkConditions(xpv1.ReconcileSuccess(), v1alpha1.TransactionFailed("Delete transactions are not yet implemented"))
		return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, tx), "cannot update Transaction status")
	}
	if tx.Spec.Change == v1alpha1.ChangeTypeReplace {
		status.MarkConditions(xpv1.ReconcileSuccess(), v1alpha1.TransactionFailed("Replace transactions are not yet implemented"))
		return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, tx), "cannot update Transaction status")
	}

	currentPackages, err := r.lock.Acquire(ctx, tx)
	if errors.Is(err, ErrLockHeldByAnotherTransaction) {
		log.Debug("Lock is held by another transaction")
		status.MarkConditions(v1alpha1.TransactionBlocked("waiting for lock held by another transaction"))
		return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, tx), "cannot update Transaction status")
	}
	if err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{}, err
		}

		log.Debug("cannot acquire lock", "error", err)
		r.record.Event(tx, event.Warning(reasonLockAcquisition, errors.Wrap(err, "cannot acquire lock")))

		status.MarkConditions(xpv1.ReconcileError(errors.Wrap(err, "cannot acquire lock")))
		_ = r.kube.Status().Update(ctx, tx)

		return reconcile.Result{}, errors.Wrap(err, "cannot acquire lock")
	}

	// Always release lock before returning, even on success. Release is
	// idempotent - if we committed, the annotation is already gone so this
	// is a no-op.
	defer func() {
		_ = r.lock.Release(ctx, tx)
	}()

	var name, source string
	switch tx.Spec.Change {
	case v1alpha1.ChangeTypeInstall:
		name = tx.Spec.Install.Package.Metadata.Name
		source = tx.Spec.Install.Package.Spec.Package
	case v1alpha1.ChangeTypeDelete:
		// TODO(negz): We might need to include the package name being
		// deleted in the spec.
		source = tx.Spec.Delete.Source
	case v1alpha1.ChangeTypeReplace:
		name = tx.Spec.Replace.Package.Metadata.Name
		source = tx.Spec.Replace.Package.Spec.Package
	}

	proposedPackages, err := r.solver.Solve(ctx, name, source, currentPackages)
	if err != nil {
		log.Debug("cannot solve dependencies", "error", err)
		r.record.Event(tx, event.Warning(reasonDependencySolve, errors.Wrap(err, "cannot solve dependencies")))
		tx.Status.Failures++
		status.MarkConditions(
			xpv1.ReconcileError(errors.Wrap(err, "cannot solve dependencies")),
			v1alpha1.ResolutionError(err.Error()),
		)
		_ = r.kube.Status().Update(ctx, tx)

		return reconcile.Result{}, errors.Wrap(err, "cannot solve dependencies")
	}
	status.MarkConditions(v1alpha1.ResolutionSuccess())

	tx.Status.ProposedLockPackages = proposedPackages

	if err := r.validator.Validate(ctx, tx); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{}, err
		}

		log.Debug("validation failed", "error", err)
		r.record.Event(tx, event.Warning(reasonValidation, errors.Wrap(err, "validation failed")))
		tx.Status.Failures++
		status.MarkConditions(
			xpv1.ReconcileError(errors.Wrap(err, "validation failed")),
			v1alpha1.ValidationError(err.Error()),
		)
		_ = r.kube.Status().Update(ctx, tx)

		return reconcile.Result{}, errors.Wrap(err, "validation failed")
	}
	status.MarkConditions(v1alpha1.ValidationSuccess())

	// Install all packages in dependency order.
	sorted, err := dependency.SortLockPackages(tx.Status.ProposedLockPackages)
	if err != nil {
		log.Debug("cannot sort packages by dependency order", "error", err)
		r.record.Event(tx, event.Warning(reasonInstallation, errors.Wrap(err, "cannot sort packages")))
		tx.Status.Failures++
		status.MarkConditions(
			xpv1.ReconcileError(errors.Wrap(err, "cannot sort packages")),
			v1alpha1.InstallationError(err.Error()),
		)
		_ = r.kube.Status().Update(ctx, tx)

		return reconcile.Result{}, errors.Wrap(err, "cannot sort packages")
	}

	for _, lockPkg := range sorted {
		xp, err := r.pkg.Get(ctx, xpkg.BuildReference(lockPkg.Source, lockPkg.Version))
		if err != nil {
			log.Debug("cannot fetch package", "error", err, "source", lockPkg.Source)
			r.record.Event(tx, event.Warning(reasonInstallation, errors.Wrapf(err, "cannot fetch package %s", lockPkg.Source)))
			tx.Status.Failures++
			status.MarkConditions(
				xpv1.ReconcileError(errors.Wrapf(err, "cannot fetch package %s", lockPkg.Source)),
				v1alpha1.InstallationError(err.Error()),
			)
			_ = r.kube.Status().Update(ctx, tx)

			return reconcile.Result{}, errors.Wrapf(err, "cannot fetch package %s", lockPkg.Source)
		}

		if err := r.installer.Install(ctx, tx, xp); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{}, err
			}

			log.Debug("cannot install package", "error", err, "source", lockPkg.Source)
			r.record.Event(tx, event.Warning(reasonInstallation, errors.Wrapf(err, "cannot install package %s", lockPkg.Source)))
			tx.Status.Failures++
			status.MarkConditions(
				xpv1.ReconcileError(errors.Wrapf(err, "cannot install package %s", lockPkg.Source)),
				v1alpha1.InstallationError(err.Error()),
			)
			_ = r.kube.Status().Update(ctx, tx)

			return reconcile.Result{}, errors.Wrapf(err, "cannot install package %s", lockPkg.Source)
		}
	}
	status.MarkConditions(v1alpha1.InstallationSuccess())

	if err := r.lock.Commit(ctx, tx, proposedPackages); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{}, err
		}

		log.Debug("cannot commit lock", "error", err)
		r.record.Event(tx, event.Warning(reasonLockAcquisition, errors.Wrap(err, "cannot commit lock")))
		tx.Status.Failures++
		status.MarkConditions(xpv1.ReconcileError(errors.Wrap(err, "cannot commit lock")))
		_ = r.kube.Status().Update(ctx, tx)

		return reconcile.Result{}, errors.Wrap(err, "cannot commit lock")
	}

	status.MarkConditions(xpv1.ReconcileSuccess(), v1alpha1.TransactionComplete())
	return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, tx), "cannot update Transaction status")
}
