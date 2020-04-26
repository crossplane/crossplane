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

package publication

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	"github.com/crossplane/crossplane/pkg/controller/apiextensions/requirement"
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
	errGetInfraPub           = "cannot get infrastructure publication"
	errNewCRD                = "cannot generate CRD from infrastructure publication"
	errGetCRD                = "cannot get generated CRD"
	errApplyCRD              = "cannot apply generated CRD"
	errUpdateInfraPubStatus  = "cannot update status of infrastructure publication"
	errCannotStartController = "cannot start controller"
	errAddFinalizer          = "cannot add finalizer"
	errRemoveFinalizer       = "cannot remove finalizer"
	errDeleteCRD             = "cannot delete crd"

	waitingInstanceDeletion    = "waiting for all defined resources to be deleted"
	waitingCRDEstablish        = "waiting for generated CRD to be established"
	errNotControllerByInfraPub = "cannot start a controller for a CRD that this InfrastructurePublication does not control"
)

// Setup adds a controller that reconciles ApplicationConfigurations.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "apiextensions/" + strings.ToLower(v1alpha1.InfrastructurePublicationGroupKind)
	r := NewReconciler(mgr,
		WithLogger(log.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	// TODO(muvaf): register this reconciler to events from CRDs, too.
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.InfrastructurePublication{}).
		WithOptions(ctrlr.Options{MaxConcurrentReconciles: maxConcurrency}).
		Complete(r)
}

// NewReconciler returns a new Reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	// TODO(negz): Make these dependencies injectable for testing.

	kube := unstructured.NewClient(mgr.GetClient())
	r := &Reconciler{
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
type ReconcilerOption func(*Reconciler)

// WithLogger returns a ReconcilerOption that configures the definer reconciler
// with given logger.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder returns a ReconcilerOption that configures the definer reconciler
// with given event.Recorder.
func WithRecorder(recorder event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.recorder = recorder
	}
}

// TODO(muvaf): There are three things that are managed: definer, crd and controller.
// We should consider having nice sub structs to represent those separately. This
// will probably be a must when it's used by AppDefinition as well.

// A Reconciler reconciles resource requirements.
type Reconciler struct {
	client resource.ClientApplicator
	mgr    manager.Manager
	ctrl   *controller.Engine
	resource.Finalizer

	log      logging.Logger
	recorder event.Recorder
}

// Reconcile is the loop function of reconciliation.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	p := &v1alpha1.InfrastructurePublication{}
	if err := r.client.Get(ctx, req.NamespacedName, p); err != nil {
		log.Debug(errGetInfraPub, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetInfraPub)
	}

	log = log.WithValues(
		"uid", p.GetUID(),
		"version", p.GetResourceVersion(),
		"name", p.GetName(),
	)

	// TODO(negz): Handle the case in which the InfrastructureDefinition is
	// deleted before the InfrastructurePublication. Currently we'll get stuck
	// because we derive the name of our CRD from the InfrastructureDefinition.
	// We could probably either infer the name of our CRD from the name of the
	// InfrastructurePublication, or just require the InfrastructurePublication
	// to be named foorequirements.example.org rather than foos.example.org.
	d := &v1alpha1.InfrastructureDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, d); err != nil {
		log.Debug(errGetInfraDef)
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(errGetInfraDef)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
	}

	crd, err := ccrd.New(ccrd.PublishesInfrastructureDefinition(d, p))
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
		if !metav1.IsControlledBy(crd, p) {
			log.Debug(errNotControllerByInfraPub)
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(errNotControllerByInfraPub)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
		}

		// It takes a while for api-server to be able to work with the new kind.
		if !ccrd.IsEstablished(crd.Status) {
			log.Debug(waitingCRDEstablish)
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(waitingCRDEstablish)))
			return reconcile.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
		}

		// We know that CRD is ready and we are in control of it. So, we'll spin up
		// an instance controller to reconcile it.
		o := ctrlr.Options{Reconciler: requirement.NewReconciler(r.mgr,
			resource.RequirementKind(Published(d.GetDefinedGroupVersionKind())),
			resource.CompositeKind(d.GetDefinedGroupVersionKind()),
			requirement.WithLogger(log.WithValues("controller", requirement.ControllerName(p.GetName()))),
		)}

		// TODO(negz): Add a Watch for our composite resource kind too.
		u := &kunstructured.Unstructured{}
		u.SetGroupVersionKind(Published(d.GetDefinedGroupVersionKind()))
		if err := r.ctrl.Start(requirement.ControllerName(p.GetName()), o, controller.For(u, &handler.EnqueueRequestForObject{})); err != nil {
			log.Debug(errCannotStartController, "error", err)
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errCannotStartController)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errCannotStartController)
		}
		// TODO(negz): Consider using the Established condition for
		// InfrastructurePublication, similar to CRD.

		// If the CRD is established and controller is up, then we are available.
		p.Status.SetConditions(runtimev1alpha1.Available())
	}

	if meta.WasDeleted(p) {
		if !meta.WasCreated(crd) {
			// Controller probably crashed if it's still up but we need to clean
			// up.
			r.ctrl.Stop(requirement.ControllerName(p.GetName()))

			// At this point, CRD is deleted and controller is not running. The
			// cleanup has been completed, we can remove the finalizer.
			if err := r.RemoveFinalizer(ctx, p); err != nil {
				log.Debug(errRemoveFinalizer, "error", err)
				p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errRemoveFinalizer)))
				return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
			}
			return reconcile.Result{Requeue: false}, nil
		}

		// We only want to operate on CRDs we own.
		if !metav1.IsControlledBy(crd, p) {
			log.Debug(errNotControllerByInfraPub)
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(errNotControllerByInfraPub)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
		}

		// NOTE(muvaf): When user deletes InfrastructureDefinition object the deletion
		// signal does not cascade to the owned resource until owner is gone. But
		// owner has its own finalizer that depends on having no instance of the CRD
		// because it cannot go away before stopping the controller.
		// So, we need to delete all instances of CRD manually here.
		l := &kunstructured.UnstructuredList{}
		l.SetGroupVersionKind(Published(d.GetDefinedGroupVersionKind()))
		if err := r.client.List(ctx, l); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
			log.Debug("cannot list published resources", "error", err)
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, "cannot list published resources")))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
		}

		// Controller should be stopped only after all instances are gone so that
		// deletion logic of the instances are processed by the controller.
		if len(l.Items) > 0 {
			// TODO(negz): For some reason DeleteAllOf does not work here,
			// despite working just fine in the definition controller. Possibly
			// due to requirements being namespaced rather than cluster scoped?
			for i := range l.Items {
				if err := r.client.Delete(ctx, &l.Items[i]); resource.IgnoreNotFound(err) != nil {
					log.Debug("cannot delete published resource", "error", err)
					p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, "cannot delete published resource")))
					return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
				}
			}
			log.Debug(waitingInstanceDeletion, "info", err)
			p.Status.SetConditions(runtimev1alpha1.ReconcileSuccess().WithMessage(waitingInstanceDeletion))
			return reconcile.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
		}
		// Controller should be stopped before the deletion of CRD so that it
		// doesn't crash.
		r.ctrl.Stop(p.GetName())
		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteCRD, "error", err)
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errDeleteCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
		}

		p.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
	}

	// We will do some operations now that we will have to cleanup, so, we need
	// a finalizer to have the chance.
	if err := r.AddFinalizer(ctx, p); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errAddFinalizer)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
	}

	// At this point, we are sure that either CRD does not exist or it had been
	// created by us. We create if it doesn't exist and update if it does.
	if err := r.client.Apply(ctx, crd, resource.MustBeControllableBy(p.GetUID())); err != nil {
		log.Debug(errApplyCRD, "error", err)
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errApplyCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
	}
	p.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateInfraPubStatus)
}

// Published GroupVersionKind that corresponds to the supplied defined
// GroupVersionKind.
func Published(defined schema.GroupVersionKind) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   defined.Group,
		Version: defined.Version,
		Kind:    defined.Kind + ccrd.PublishedInfrastructureSuffixKind,
	}
}
