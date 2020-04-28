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
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
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
	tinyWait  = 3 * time.Second
	shortWait = 30 * time.Second

	timeout        = 2 * time.Minute
	maxConcurrency = 5
	finalizer      = "finalizer.apiextensions.crossplane.io"

	errGetInfraDef     = "cannot get InfrastructureDefinition"
	errNewCRD          = "cannot generate CustomResourceDefinition"
	errGetCRD          = "cannot get CustomResourceDefinition"
	errApplyCRD        = "cannot apply the generated CustomResourceDefinition"
	errUpdateStatus    = "cannot update status of InfrastructureDefinition"
	errStartController = "cannot start controller"
	errAddFinalizer    = "cannot add finalizer"
	errRemoveFinalizer = "cannot remove finalizer"
	errDeleteCRD       = "cannot delete CustomResourceDefinition"
	errListCRs         = "cannot list defined custom resources"
	errDeleteCRs       = "cannot delete defined custom resources"
)

// Wait strings.
const (
	waitCRDelete     = "waiting for defined custom resources to be deleted"
	waitCRDEstablish = "waiting for CustomResourceDefinition to be established"
)

// Event reasons.
const (
	reasonRenderCRD event.Reason = "RenderCustomResourceDefinition"
	reasonDeleteDef event.Reason = "DeleteInfrastructureDefinition"
	reasonApplyDef  event.Reason = "ApplyInfrastructureDefinition"
)

// A ControllerEngine can start and stop Kubernetes controllers on demand.
type ControllerEngine interface {
	IsRunning(name string) bool
	Start(name string, o kcontroller.Options, w ...controller.Watch) error
	Stop(name string)
}

// A CRDRenderer renders an InfrastructureDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderer interface {
	Render(d *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error)
}

// A CRDRenderFn renders an InfrastructureDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderFn func(d *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error)

// Render the supplied InfrastructureDefinition's corresponding
// CustomResourceDefinition.
func (fn CRDRenderFn) Render(d *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
	return fn(d)
}

// Setup adds a controller that reconciles ApplicationConfigurations.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "apiextensions/" + strings.ToLower(v1alpha1.InfrastructureDefinitionGroupKind)
	r := NewReconciler(mgr,
		WithLogger(log.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.InfrastructureDefinition{}).
		Owns(&v1beta1.CustomResourceDefinition{}).
		WithOptions(kcontroller.Options{MaxConcurrentReconciles: maxConcurrency}).
		Complete(r)
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record Kubernetes events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithFinalizer specifies how the Reconciler should finalize
// InfrastructureDefinitions.
func WithFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.definition.Finalizer = f
	}
}

// WithControllerEngine specifies how the Reconciler should manage the
// lifecycles of composite controllers.
func WithControllerEngine(c ControllerEngine) ReconcilerOption {
	return func(r *Reconciler) {
		r.definition.ControllerEngine = c
	}
}

// WithCRDRenderer specifies how the Reconciler should render an
// InfrastructureDefinition's corresponding CustomResourceDefinition.
func WithCRDRenderer(c CRDRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.definition.CRDRenderer = c
	}
}

type definition struct {
	CRDRenderer
	ControllerEngine
	resource.Finalizer
}

// NewReconciler returns a Reconciler of InfrastructureDefinitions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())
	rd := func(d *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
		return ccrd.New(ccrd.ForInfrastructureDefinition(d))
	}

	r := &Reconciler{
		mgr: mgr,

		// TODO(negz): We don't need to patch (it's fine to overwrite an
		// existing CRD's fields when applying) but we can't use
		// resource.APIUpdatingApplicator due to the below issue.
		// https://github.com/crossplane/crossplane-runtime/issues/165
		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewAPIPatchingApplicator(kube),
		},

		definition: definition{
			CRDRenderer:      CRDRenderFn(rd),
			ControllerEngine: controller.NewEngine(mgr),
			Finalizer:        resource.NewAPIFinalizer(kube, finalizer),
		},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles InfrastructureDefinitions.
type Reconciler struct {
	client resource.ClientApplicator
	mgr    manager.Manager

	definition definition

	log    logging.Logger
	record event.Recorder
}

// TODO(muvaf,negz): Consider deduplicating this Reconciler with the
// InfrastructurePublication and (as yet unwritten) ApplicationDefinition
// reconcilers.

// Reconcile an InfrastructureDefinition.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): Like most Reconcile methods, this one is over our cyclomatic
	// complexity goal. Be wary when adding branches, and look for functionality
	// that could be reasonably moved into an injected dependency.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	d := &v1alpha1.InfrastructureDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, d); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetInfraDef, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetInfraDef)
	}

	log = log.WithValues(
		"uid", d.GetUID(),
		"version", d.GetResourceVersion(),
		"name", d.GetName(),
	)

	crd, err := r.definition.Render(d)
	if err != nil {
		log.Debug(errNewCRD, "error", err)
		r.record.Event(d, event.Warning(reasonRenderCRD, err))
		d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errNewCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
	}

	r.record.Event(d, event.Normal(reasonRenderCRD, "Rendered CustomResourceDefinition"))

	if meta.WasDeleted(d) {
		d.Status.SetConditions(v1alpha1.Deleting())
		r.record.Event(d, event.Normal(reasonDeleteDef, "Deleting InfrastructureDefinition"))

		nn := types.NamespacedName{Name: crd.GetName()}
		if err := r.client.Get(ctx, nn, crd); resource.IgnoreNotFound(err) != nil {
			r.record.Event(d, event.Warning(reasonDeleteDef, err))
			d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errGetCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
		}

		// The CRD has no creation timestamp, or we don't control it. Most
		// likely we successfully deleted it on a previous reconcile. It's also
		// possible that we're being asked to delete it before we got around to
		// creating it, or that we lost control of it around the same time we
		// were deleted. In the (presumably exceedingly rare) latter case we'll
		// orphan the CRD.
		if !meta.WasCreated(crd) || !metav1.IsControlledBy(crd, d) {
			// It's likely that we've already stopped this controller on a
			// previous reconcile, but we try again just in case. This is a
			// no-op if the controller was already stopped.
			r.definition.Stop(composite.ControllerName(d.GetName()))

			if err := r.definition.RemoveFinalizer(ctx, d); err != nil {
				log.Debug(errRemoveFinalizer, "error", err)
				r.record.Event(d, event.Warning(reasonDeleteDef, err))
				d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errRemoveFinalizer)))
				return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
			}

			// We're all done deleting and have removed our finalizer. There's
			// no need to requeue because there's nothing left to do.
			r.record.Event(d, event.Normal(reasonDeleteDef, "Successfully deleted InfrastructureDefinition"))
			return reconcile.Result{Requeue: false}, nil
		}

		// NOTE(muvaf): When user deletes InfrastructureDefinition object the
		// deletion signal does not cascade to the owned resource until owner is
		// gone. But owner has its own finalizer that depends on having no
		// instance of the CRD because it cannot go away before stopping the
		// controller. So, we need to delete all instances of CRD manually here.
		o := &kunstructured.Unstructured{}
		o.SetGroupVersionKind(d.GetDefinedGroupVersionKind())
		if err := r.client.DeleteAllOf(ctx, o); err != nil && !kmeta.IsNoMatchError(err) && !kerrors.IsNotFound(err) {
			log.Debug(errDeleteCRs, "error", err)
			r.record.Event(d, event.Warning(reasonDeleteDef, err))
			d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errDeleteCRs)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
		}

		l := &kunstructured.UnstructuredList{}
		l.SetGroupVersionKind(d.GetDefinedGroupVersionKind())
		if err := r.client.List(ctx, l); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
			log.Debug(errListCRs, "error", err)
			r.record.Event(d, event.Warning(reasonDeleteDef, err))
			d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errListCRs)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
		}

		// Controller should be stopped only after all instances are gone so
		// that deletion logic of the instances are processed by the controller.
		if len(l.Items) > 0 {
			log.Debug(waitCRDelete)
			r.record.Event(d, event.Normal(reasonDeleteDef, waitCRDelete))
			d.Status.SetConditions(runtimev1alpha1.ReconcileSuccess().WithMessage(waitCRDelete))
			return reconcile.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
		}

		// The controller should be stopped before the deletion of CRD so that
		// it doesn't crash.
		r.definition.Stop(composite.ControllerName(d.GetName()))

		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteCRD, "error", err)
			r.record.Event(d, event.Warning(reasonDeleteDef, err))
			d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errDeleteCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
		}

		// We should be requeued implicitly because we're watching the
		// CustomResourceDefinition that we just deleted, but we requeue after
		// a tiny wait just in case the CRD isn't gone after the first requeue.
		log.Debug("Stopped composite controller and deleted CustomResourceDefinition")
		r.record.Event(d, event.Normal(reasonDeleteDef, "Stopped composite controller and deleted CustomResourceDefinition"))
		d.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: tinyWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
	}

	if err := r.definition.AddFinalizer(ctx, d); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		r.record.Event(d, event.Warning(reasonApplyDef, err))
		d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errAddFinalizer)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
	}

	if err := r.client.Apply(ctx, crd, resource.MustBeControllableBy(d.GetUID())); err != nil {
		log.Debug(errApplyCRD, "error", err)
		r.record.Event(d, event.Warning(reasonApplyDef, err))
		d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errApplyCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
	}

	if !ccrd.IsEstablished(crd.Status) {
		log.Debug(waitCRDEstablish)
		r.record.Event(d, event.Normal(reasonApplyDef, waitCRDEstablish))
		d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.New(waitCRDEstablish)))
		return reconcile.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
	}

	o := kcontroller.Options{Reconciler: composite.NewReconciler(r.mgr,
		resource.CompositeKind(d.GetDefinedGroupVersionKind()),
		composite.WithConnectionPublisher(composite.NewAPIFilteredSecretPublisher(r.client, d.GetConnectionSecretKeys())),
		composite.WithLogger(log.WithValues("controller", composite.ControllerName(d.GetName()))),
		composite.WithRecorder(event.NewAPIRecorder(r.mgr.GetEventRecorderFor(composite.ControllerName(d.GetName())))),
	)}

	u := &kunstructured.Unstructured{}
	u.SetGroupVersionKind(d.GetDefinedGroupVersionKind())

	if err := r.definition.Start(composite.ControllerName(d.GetName()), o, controller.For(u, &handler.EnqueueRequestForObject{})); err != nil {
		log.Debug(errStartController, "error", err)
		r.record.Event(d, event.Warning(reasonApplyDef, err))
		d.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errStartController)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, d), errStartController)
	}

	d.Status.SetConditions(v1alpha1.Started())
	d.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	r.record.Event(d, event.Normal(reasonApplyDef, "Applied CustomResourceDefinition and (re)started composite controller"))
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
}
