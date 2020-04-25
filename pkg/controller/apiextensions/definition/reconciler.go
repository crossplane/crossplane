/*
Copyright 2020 The Crossplane Authors.

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

package definition

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlr "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1/ccrd"
	"github.com/crossplane/crossplane/pkg/controller/apiextensions/composite"
)

const (
	// TODO(muvaf): consider adding _shorterWait_ for non-error cases where the result
	// is expected to be quick. For example, deletion or CRD establishment takes
	// unnecessarily long just because shortWait is 10 seconds.
	shortWait      = 10 * time.Second
	longWait       = 1 * time.Minute
	timeout        = 2 * time.Minute
	maxConcurrency = 5
	finalizerName  = "finalizer.apiextensions.crossplane.io"

	errGetInfraDef           = "cannot get infrastructure definition"
	errNewCRD                = "cannot generate CRD from infrastructure definition"
	errGetCRD                = "cannot get CRD"
	errApplyCRD              = "cannot apply the generated CRD"
	errUpdateInfraDefStatus  = "cannot update status of infrastructure definition"
	errCannotStartController = "cannot start controller"
	errAddFinalizer          = "cannot add finalizer"
	errRemoveFinalizer       = "cannot remove finalizer"
	errDeleteCRD             = "cannot delete crd"

	waitingInstanceDeletion   = "waiting for all cr instances to be gone"
	waitingCRDEstablish       = "waiting for crd to be established"
	errNotControllerByDefiner = "cannot start a controller for a crd that this definer does not control"
)

// Setup adds a controller that reconciles ApplicationConfigurations.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "apiextensions/" + strings.ToLower(v1alpha1.InfrastructureDefinitionGroupKind)
	r := NewReconciler(mgr,
		WithLogger(log.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	// TODO(muvaf): register this reconciler to events from CRDs, too.
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.InfrastructureDefinition{}).
		WithOptions(ctrlr.Options{MaxConcurrentReconciles: maxConcurrency}).
		Complete(r)
}

// NewReconciler returns a new *reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) reconcile.Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())
	r := &reconciler{
		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewAPIPatchingApplicator(kube),
		},
		mgr:       mgr,
		ctrl:      controller.NewEngine(mgr),
		Finalizer: resource.NewAPIFinalizer(kube, finalizerName),
		log:       logging.NewNopLogger(),
		recorder:  event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// ReconcilerOption is used to configure the definer reconciler.
type ReconcilerOption func(*reconciler)

// WithLogger returns a ReconcilerOption that configures the definer reconciler
// with given logger.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *reconciler) {
		r.log = log
	}
}

// WithRecorder returns a ReconcilerOption that configures the definer reconciler
// with given event.Recorder.
func WithRecorder(recorder event.Recorder) ReconcilerOption {
	return func(r *reconciler) {
		r.recorder = recorder
	}
}

// TODO(muvaf): There are three things that are managed: definer, crd and controller.
// We should consider having nice sub structs to represent those separately. This
// will probably be a must when it's used by AppDefinition as well.
type reconciler struct {
	client resource.ClientApplicator
	mgr    manager.Manager
	ctrl   *controller.Engine
	resource.Finalizer

	log      logging.Logger
	recorder event.Recorder
}

// Reconcile is the loop function of reconciliation.
func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// TODO(muvaf): this will be generic as there will be ApplicationDefinition
	// type, too.
	definer := &v1alpha1.InfrastructureDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, definer); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We don't
		// need to take any action in that case.
		log.Debug(errGetInfraDef, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetInfraDef)
	}

	log = log.WithValues(
		"uid", definer.GetUID(),
		"version", definer.GetResourceVersion(),
		"name", definer.GetName(),
	)

	crd, err := ccrd.New(ccrd.ForInfrastructureDefinition(definer), ccrd.DefinesCompositeInfrastructure())
	if err != nil {
		log.Debug(errNewCRD, "error", err)
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, errNewCRD)
	}

	nn := types.NamespacedName{Name: crd.GetName()}
	if err := r.client.Get(ctx, nn, crd); resource.IgnoreNotFound(err) != nil {
		log.Debug(errGetCRD, "error", err)
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, errGetCRD)
	}

	// We make sure the CRD that we'll start a controller for is not marked for
	// deletion so that when it does get deleted our controller and cache don't
	// crash. Since we are the ones explicitly calling deletion of CRD after
	// all instances are gone, it should be OK not start the controller.
	if meta.WasCreated(crd) && !meta.WasDeleted(crd) {
		// TODO(muvaf): controller and establishment checks could go into controller
		// engine.

		// We only want to operate on CRDs we own.
		if !metav1.IsControlledBy(crd, definer) {
			log.Debug(errNotControllerByDefiner)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(errNotControllerByDefiner)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
		}

		// It takes a while for api-server to be able to work with the new kind.
		if !ccrd.IsEstablished(crd.Status) {
			log.Debug(waitingCRDEstablish)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(waitingCRDEstablish)))
			return reconcile.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
		}

		// We know that CRD is ready and we are in control of it. So, we'll spin up
		// an instance controller to reconcile it.
		o := ctrlr.Options{Reconciler: composite.NewReconciler(r.mgr,
			definer.GetDefinedGroupVersionKind(),
			r.log.WithValues("controller", composite.ControllerName(definer.GetName())),
			definer)}
		u := &kunstructured.Unstructured{}
		u.SetGroupVersionKind(definer.GetDefinedGroupVersionKind())

		if err := r.ctrl.Start(composite.ControllerName(definer.GetName()), o, controller.For(u, &handler.EnqueueRequestForObject{})); err != nil {
			log.Debug(errCannotStartController, "error", err)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errCannotStartController)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errCannotStartController)
		}
		// TODO(muvaf): Consider using _Established_ condition for Definers
		// similar to CRD.

		// If the CRD is established and controller is up, then we are available.
		definer.Status.SetConditions(runtimev1alpha1.Available())
	}

	if meta.WasDeleted(definer) {
		if !meta.WasCreated(crd) {
			// Controller probably crashed if it's still up but we need to clean
			// up.
			r.ctrl.Stop(composite.ControllerName(definer.GetName()))

			// At this point, CRD is deleted and controller is not running. The
			// cleanup has been completed, we can remove the finalizer.
			if err := r.RemoveFinalizer(ctx, definer); err != nil {
				log.Debug(errRemoveFinalizer, "error", err)
				definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errRemoveFinalizer)))
				return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
			}
			return reconcile.Result{Requeue: false}, nil
		}

		// We only want to operate on CRDs we own.
		if !metav1.IsControlledBy(crd, definer) {
			log.Debug(errNotControllerByDefiner)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(errNotControllerByDefiner)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
		}

		// NOTE(muvaf): When user deletes InfrastructureDefinition object the deletion
		// signal does not cascade to the owned resource until owner is gone. But
		// owner has its own finalizer that depends on having no instance of the CRD
		// because it cannot go away before stopping the controller.
		// So, we need to delete all instances of CRD manually here.
		o := &kunstructured.Unstructured{}
		o.SetGroupVersionKind(definer.GetDefinedGroupVersionKind())
		if err := r.client.DeleteAllOf(ctx, o); err != nil && !kmeta.IsNoMatchError(err) && !kerrors.IsNotFound(err) {
			log.Debug("cannot delete defined resources", "error", err)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, "cannot delete defined resources")))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
		}

		l := &kunstructured.UnstructuredList{}
		l.SetGroupVersionKind(definer.GetDefinedGroupVersionKind())
		if err := r.client.List(ctx, l); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
			log.Debug("cannot list defined resources", "error", err)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, "cannot list defined resources")))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
		}

		// Controller should be stopped only after all instances are gone so that
		// deletion logic of the instances are processed by the controller.
		if len(l.Items) > 0 {
			log.Debug(waitingInstanceDeletion, "info", err)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileSuccess().WithMessage(waitingInstanceDeletion))
			return reconcile.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
		}
		// Controller should be stopped before the deletion of CRD so that it
		// doesn't crash.
		r.ctrl.Stop(definer.GetName())
		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteCRD, "error", err)
			definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errDeleteCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
		}

		definer.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
	}

	// We will do some operations now that we will have to cleanup, so, we need
	// a finalizer to have the chance.
	if err := r.AddFinalizer(ctx, definer); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errAddFinalizer)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
	}

	// At this point, we are sure that either CRD does not exist or it had been
	// created by us. We create if it doesn't exist and update if it does.
	if err := r.client.Apply(ctx, crd, resource.MustBeControllableBy(definer.GetUID())); err != nil {
		log.Debug(errApplyCRD, "error", err)
		definer.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errApplyCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
	}
	definer.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, definer), errUpdateInfraDefStatus)
}
