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
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/managed/resources"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	timeout = 2 * time.Minute
)

// Event reasons.
const (
	reasonReconcile event.Reason = "Reconcile"
	reasonPaused    event.Reason = "ReconciliationPaused"
	reasonCreateCRD event.Reason = "CreateCustomResourceDefinition"
	reasonUpdateCRD event.Reason = "UpdateCustomResourceDefinition"
)

// A Reconciler reconciles CompositeResourceDefinitions.
type Reconciler struct {
	client.Client

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

	mrd := &v2alpha1.ManagedResourceDefinition{}
	if err := r.Get(ctx, req.NamespacedName, mrd); err != nil {
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
		status.MarkConditions(v2alpha1.TerminatingManaged())
		if err := r.Status().Update(ogctx, mrd); err != nil {
			log.Debug("cannot update status of ManagedResourceDefinition", "error", err)
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err = errors.Wrap(err, "cannot update status of ManagedResourceDefinition")
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
	// Check for pause annotation
	if meta.IsPaused(mrd) {
		log.Info("reconciliation is paused")
		r.record.Event(mrd, event.Normal(reasonPaused, "Reconciliation is paused via the pause annotation"))
		return reconcile.Result{}, nil
	}

	if !mrd.Spec.State.IsActive() {
		status.MarkConditions(v2alpha1.InactiveManaged())
		return reconcile.Result{}, errors.Wrap(r.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
	}

	// Check that the CRD exists and is up to date.
	crd, err := r.reconcileCustomResourceDefinition(ctx, log, mrd)
	if err != nil {
		log.Debug("failed to reconcile CustomResourceDefinition", "error", err)
		r.record.Event(mrd, event.Warning(reasonReconcile, err))
		status.MarkConditions(v2alpha1.BlockedManaged().WithMessage("unable to reconcile CustomResourceDefinition, see events"))
		if err := r.Status().Update(ogctx, mrd); err != nil {
			log.Info("cannot update status of ManagedResourceDefinition", "error", err)
		}
		return reconcile.Result{}, errors.Wrap(err, "cannot reconcile CustomResourceDefinition")
	}

	if xcrd.IsEstablished(crd.Status) {
		status.MarkConditions(v2alpha1.EstablishedManaged())
	} else {
		log.Debug("waiting for managed resource CustomResourceDefinition to be established")
		status.MarkConditions(v2alpha1.PendingManaged())
	}

	return reconcile.Result{}, errors.Wrap(r.Status().Update(ogctx, mrd), "cannot update status of ManagedResourceDefinition")
}

const (
	actionCreate = iota
	actionUpdate
)

func (r *Reconciler) reconcileCustomResourceDefinition(ctx context.Context, log logging.Logger, mrd *v2alpha1.ManagedResourceDefinition) (*extv1.CustomResourceDefinition, error) {
	want := resources.EmptyCustomResourceDefinition(mrd)
	nn := types.NamespacedName{
		Namespace: want.Namespace,
		Name:      want.Name,
	}
	action := actionUpdate
	if err := r.Get(ctx, nn, want); err != nil && !kerrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "cannot get CustomResourceDefinition")
	} else if err != nil && kerrors.IsNotFound(err) {
		log.Debug("CustomResourceDefinition not found, will create", "crd", nn)
		action = actionCreate
	}
	existing := want.DeepCopy()

	if meta.WasDeleted(want) {
		log.Debug("CustomResourceDefinition is being deleted", "crd", nn)
		return nil, errors.New("crd was deleted")
	}

	// Stage changes.
	if err := resources.MergeCustomResourceDefinitionInto(mrd, want); err != nil {
		return nil, errors.Wrap(err, "cannot merge CustomResourceDefinition")
	}

	// Apply changes.
	switch action {
	case actionCreate:
		if err := r.Create(ctx, want); err != nil {
			return nil, r.handleErrorWithEvent(log, mrd, err, "cannot create CustomResourceDefinition", reasonCreateCRD)
		}
		r.record.Event(mrd, event.Normal(reasonCreateCRD, "Successfully created CustomResourceDefinition"))
	case actionUpdate:
		if !equality.Semantic.DeepEqual(existing.Spec, want.Spec) {
			if err := r.Update(ctx, want); err != nil {
				return nil, r.handleErrorWithEvent(log, mrd, err, "cannot update CustomResourceDefinition", reasonUpdateCRD)
			}
			r.record.Event(mrd, event.Normal(reasonUpdateCRD, "Successfully updated CustomResourceDefinition"))
		}
	default:
		// Noop.
	}

	return want, nil
}

// handleErrorWithEvent provides centralized error handling with event recording.
func (r *Reconciler) handleErrorWithEvent(log logging.Logger, obj client.Object, err error, errMsg string, reason event.Reason) error {
	// Handle conflicts with immediate requeue - no event needed as this is transient
	if kerrors.IsConflict(err) {
		return err // Controller runtime will requeue automatically
	}

	// Record warning event for all other errors
	r.record.Event(obj, event.Warning(reason, err))
	log.Debug(errMsg, "reason", reason, "error", err)

	// Handle validation errors to prevent endless reconciliation
	if kerrors.IsInvalid(err) {
		// API Server's invalid errors may be unstable due to pointers in
		// the string representation, causing endless reconciliation
		err = errors.New("invalid resource configuration")
	}

	return errors.Wrap(err, errMsg)
}
