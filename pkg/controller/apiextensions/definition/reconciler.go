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
	finalizer      = "defined.apiextensions.crossplane.io"

	errGetXRD          = "cannot get CompositeResourceDefinition"
	errRenderCRD       = "cannot render composite resource CustomResourceDefinition"
	errGetCRD          = "cannot get composite resource CustomResourceDefinition"
	errApplyCRD        = "cannot apply rendered composite resource CustomResourceDefinition"
	errUpdateStatus    = "cannot update status of CompositeResourceDefinition"
	errStartController = "cannot start composite resource controller"
	errAddFinalizer    = "cannot add composite resource finalizer"
	errRemoveFinalizer = "cannot remove composite resource finalizer"
	errDeleteCRD       = "cannot delete composite resource CustomResourceDefinition"
	errListCRs         = "cannot list defined composite resources"
	errDeleteCRs       = "cannot delete defined composite resources"
)

// Wait strings.
const (
	waitCRDelete     = "waiting for defined composite resources to be deleted"
	waitCRDEstablish = "waiting for composite resource CustomResourceDefinition to be established"
)

// Event reasons.
const (
	reasonRenderCRD   event.Reason = "RenderCRD"
	reasonEstablishXR event.Reason = "EstablishComposite"
	reasonTerminateXR event.Reason = "TerminateComposite"
)

// A ControllerEngine can start and stop Kubernetes controllers on demand.
type ControllerEngine interface {
	IsRunning(name string) bool
	Start(name string, o kcontroller.Options, w ...controller.Watch) error
	Stop(name string)
}

// A CRDRenderer renders an CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderer interface {
	Render(d *v1alpha1.CompositeResourceDefinition) (*v1beta1.CustomResourceDefinition, error)
}

// A CRDRenderFn renders an CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderFn func(d *v1alpha1.CompositeResourceDefinition) (*v1beta1.CustomResourceDefinition, error)

// Render the supplied CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
func (fn CRDRenderFn) Render(d *v1alpha1.CompositeResourceDefinition) (*v1beta1.CustomResourceDefinition, error) {
	return fn(d)
}

// Setup adds a controller that reconciles ApplicationConfigurations.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "defined/" + strings.ToLower(v1alpha1.CompositeResourceDefinitionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.CompositeResourceDefinition{}).
		Owns(&v1beta1.CustomResourceDefinition{}).
		WithOptions(kcontroller.Options{MaxConcurrentReconciles: maxConcurrency}).
		Complete(NewReconciler(mgr,
			WithLogger(log.WithValues("controller", name)),
			WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
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
// CompositeResourceDefinitions.
func WithFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Finalizer = f
	}
}

// WithControllerEngine specifies how the Reconciler should manage the
// lifecycles of composite controllers.
func WithControllerEngine(c ControllerEngine) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ControllerEngine = c
	}
}

// WithCRDRenderer specifies how the Reconciler should render an
// CompositeResourceDefinition's corresponding CustomResourceDefinition.
func WithCRDRenderer(c CRDRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CRDRenderer = c
	}
}

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = ca
	}
}

type definition struct {
	CRDRenderer
	ControllerEngine
	resource.Finalizer
}

// NewReconciler returns a Reconciler of CompositeResourceDefinitions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())

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

		composite: definition{
			CRDRenderer: CRDRenderFn(func(d *v1alpha1.CompositeResourceDefinition) (*v1beta1.CustomResourceDefinition, error) {
				return ccrd.New(ccrd.ForCompositeResource(d))
			}),
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

// A Reconciler reconciles CompositeResourceDefinitions.
type Reconciler struct {
	client resource.ClientApplicator
	mgr    manager.Manager

	composite definition

	log    logging.Logger
	record event.Recorder
}

// Reconcile a CompositeResourceDefinition by defining a new kind of composite
// resource and starting a controller to reconcile it.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): Like most Reconcile methods, this one is over our cyclomatic
	// complexity goal. Be wary when adding branches, and look for functionality
	// that could be reasonably moved into an injected dependency.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	d := &v1alpha1.CompositeResourceDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, d); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetXRD, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetXRD)
	}

	log = log.WithValues(
		"uid", d.GetUID(),
		"version", d.GetResourceVersion(),
		"name", d.GetName(),
	)

	crd, err := r.composite.Render(d)
	if err != nil {
		log.Debug(errRenderCRD, "error", err)
		r.record.Event(d, event.Warning(reasonRenderCRD, errors.Wrap(err, errRenderCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	r.record.Event(d, event.Normal(reasonRenderCRD, "Rendered composite resource CustomResourceDefinition"))

	if meta.WasDeleted(d) {
		nn := types.NamespacedName{Name: crd.GetName()}
		if err := r.client.Get(ctx, nn, crd); resource.IgnoreNotFound(err) != nil {
			r.record.Event(d, event.Warning(reasonTerminateXR, errors.Wrap(err, errGetCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
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
			r.composite.Stop(composite.ControllerName(d.GetName()))
			log.Debug("Stopped composite resource controller")
			r.record.Event(d, event.Normal(reasonTerminateXR, "Stopped composite resource controller"))

			if err := r.composite.RemoveFinalizer(ctx, d); err != nil {
				log.Debug(errRemoveFinalizer, "error", err)
				r.record.Event(d, event.Warning(reasonTerminateXR, errors.Wrap(err, errRemoveFinalizer)))
				return reconcile.Result{RequeueAfter: shortWait}, nil
			}

			// We're all done deleting and have removed our finalizer. There's
			// no need to requeue because there's nothing left to do.
			return reconcile.Result{Requeue: false}, nil
		}

		// NOTE(muvaf): When user deletes CompositeResourceDefinition object the
		// deletion signal does not cascade to the owned resource until owner is
		// gone. But owner has its own finalizer that depends on having no
		// instance of the CRD because it cannot go away before stopping the
		// controller. So, we need to delete all instances of CRD manually here.
		o := &kunstructured.Unstructured{}
		o.SetGroupVersionKind(d.GetCompositeGroupVersionKind())
		if err := r.client.DeleteAllOf(ctx, o); err != nil && !kmeta.IsNoMatchError(err) && !kerrors.IsNotFound(err) {
			log.Debug(errDeleteCRs, "error", err)
			r.record.Event(d, event.Warning(reasonTerminateXR, errors.Wrap(err, errDeleteCRs)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		l := &kunstructured.UnstructuredList{}
		l.SetGroupVersionKind(d.GetCompositeGroupVersionKind())
		if err := r.client.List(ctx, l); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
			log.Debug(errListCRs, "error", err)
			r.record.Event(d, event.Warning(reasonTerminateXR, errors.Wrap(err, errListCRs)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		// Controller should be stopped only after all instances are gone so
		// that deletion logic of the instances are processed by the controller.
		if len(l.Items) > 0 {
			log.Debug(waitCRDelete)
			r.record.Event(d, event.Normal(reasonTerminateXR, waitCRDelete))
			return reconcile.Result{RequeueAfter: tinyWait}, nil
		}

		// The controller should be stopped before the deletion of CRD so that
		// it doesn't crash.
		r.composite.Stop(composite.ControllerName(d.GetName()))
		log.Debug("Stopped composite resource controller")
		r.record.Event(d, event.Normal(reasonTerminateXR, "Stopped composite resource controller"))

		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteCRD, "error", err)
			r.record.Event(d, event.Warning(reasonTerminateXR, errors.Wrap(err, errDeleteCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}
		log.Debug("Deleted composite resource CustomResourceDefinition")
		r.record.Event(d, event.Normal(reasonTerminateXR, "Deleted composite resource CustomResourceDefinition"))

		// We should be requeued implicitly because we're watching the
		// CustomResourceDefinition that we just deleted, but we requeue after
		// a tiny wait just in case the CRD isn't gone after the first requeue.
		return reconcile.Result{RequeueAfter: tinyWait}, nil
	}

	if err := r.composite.AddFinalizer(ctx, d); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		r.record.Event(d, event.Warning(reasonEstablishXR, errors.Wrap(err, errAddFinalizer)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	if err := r.client.Apply(ctx, crd, resource.MustBeControllableBy(d.GetUID())); err != nil {
		log.Debug(errApplyCRD, "error", err)
		r.record.Event(d, event.Warning(reasonEstablishXR, errors.Wrap(err, errApplyCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}
	r.record.Event(d, event.Normal(reasonEstablishXR, "Applied composite resource CustomResourceDefinition"))

	if !ccrd.IsEstablished(crd.Status) {
		log.Debug(waitCRDEstablish)
		r.record.Event(d, event.Normal(reasonEstablishXR, waitCRDEstablish))
		return reconcile.Result{RequeueAfter: tinyWait}, nil
	}
	recorder := r.record.WithAnnotations("controller", composite.ControllerName(d.GetName()))
	o := kcontroller.Options{Reconciler: composite.NewReconciler(r.mgr,
		resource.CompositeKind(d.GetCompositeGroupVersionKind()),
		composite.WithConnectionPublisher(composite.NewAPIFilteredSecretPublisher(r.client, d.GetConnectionSecretKeys())),
		composite.WithCompositionSelector(composite.NewCompositionSelectorChain(
			composite.NewEnforcedCompositionSelector(*d, recorder),
			composite.NewAPIDefaultCompositionSelector(r.client, *meta.ReferenceTo(d, v1alpha1.CompositeResourceDefinitionGroupVersionKind), recorder),
			composite.NewAPILabelSelectorResolver(r.client),
		)),
		composite.WithLogger(log.WithValues("controller", composite.ControllerName(d.GetName()))),
		composite.WithRecorder(recorder),
	)}

	u := &kunstructured.Unstructured{}
	u.SetGroupVersionKind(d.GetCompositeGroupVersionKind())

	if err := r.composite.Start(composite.ControllerName(d.GetName()), o, controller.For(u, &handler.EnqueueRequestForObject{})); err != nil {
		log.Debug(errStartController, "error", err)
		r.record.Event(d, event.Warning(reasonEstablishXR, errors.Wrap(err, errStartController)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	d.Status.SetConditions(v1alpha1.WatchingComposite())
	r.record.Event(d, event.Normal(reasonEstablishXR, "(Re)started composite resource controller"))
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
}
