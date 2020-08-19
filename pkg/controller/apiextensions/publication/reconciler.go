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
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1/ccrd"
	"github.com/crossplane/crossplane/pkg/controller/apiextensions/claim"
)

const (
	// TODO(negz): Use exponential backoff instead of RetryAfter durations.
	tinyWait  = 3 * time.Second
	shortWait = 30 * time.Second

	timeout        = 1 * time.Minute
	maxConcurrency = 5
	finalizer      = "published.apiextensions.crossplane.io"
)

// Error strings.
const (
	errGetXRD          = "cannot get CompositeResourceDefinition"
	errGetXRP          = "cannot get CompositeResourcePublication"
	errRenderCRD       = "cannot generate CustomResourceDefinition"
	errGetCRD          = "cannot get CustomResourceDefinition"
	errApplyCRD        = "cannot apply CustomResourceDefinition"
	errUpdateStatus    = "cannot update status of CompositeResourcePublication"
	errStartController = "cannot start controller"
	errAddFinalizer    = "cannot add finalizer"
	errRemoveFinalizer = "cannot remove finalizer"
	errDeleteCRD       = "cannot delete CustomResourceDefinition"
	errListCRs         = "cannot list defined custom resources"
	errDeleteCR        = "cannot delete defined custom resource"
)

// Wait strings.
const (
	waitCRDelete     = "waiting for defined custom resources to be deleted"
	waitCRDEstablish = "waiting for CustomResourceDefinition to be established"
)

// Event reasons.
const (
	reasonGetDef    event.Reason = "GetCompositeResourceDefinition"
	reasonRenderCRD event.Reason = "RenderCustomResourceDefinition"
	reasonDeletePub event.Reason = "DeleteCompositeResourcePublication"
	reasonApplyPub  event.Reason = "ApplyCompositeResourcePublication"
)

// A Finalizer adds and removes finalizers to and from object metadata.
type Finalizer interface {
	AddFinalizer(ctx context.Context, o ...resource.Object) error
	RemoveFinalizer(ctx context.Context, o ...resource.Object) error
}

// TODO(negz): Update resource.Finalizer in crossplane-runtime to support
// multiple objects? Not sure this is a common enough use case.

// An MultiFinalizer manages a finalizer for multiple objects.
type MultiFinalizer struct {
	f resource.Finalizer
}

// NewMultiFinalizer returns a resource.Finalizer that manages a finalizer for
// multiple objects.
func NewMultiFinalizer(f resource.Finalizer) *MultiFinalizer {
	return &MultiFinalizer{f: f}
}

// AddFinalizer to the supplied objects.
func (a *MultiFinalizer) AddFinalizer(ctx context.Context, o ...resource.Object) error {
	for _, obj := range o {
		if err := a.f.AddFinalizer(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

// RemoveFinalizer from the supplies objects.
func (a *MultiFinalizer) RemoveFinalizer(ctx context.Context, o ...resource.Object) error {
	for _, obj := range o {
		if err := a.f.RemoveFinalizer(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

// A ControllerEngine can start and stop Kubernetes controllers on demand.
type ControllerEngine interface {
	IsRunning(name string) bool
	Start(name string, o kcontroller.Options, w ...controller.Watch) error
	Stop(name string)
}

// A CRDRenderer renders an CompositeResourcePublication's corresponding
// CustomResourceDefinition.
type CRDRenderer interface {
	Render(d *v1alpha1.CompositeResourceDefinition, p *v1alpha1.CompositeResourcePublication) (*v1beta1.CustomResourceDefinition, error)
}

// A CRDRenderFn renders an CompositeResourcePublication's corresponding
// CustomResourceDefinition.
type CRDRenderFn func(d *v1alpha1.CompositeResourceDefinition, p *v1alpha1.CompositeResourcePublication) (*v1beta1.CustomResourceDefinition, error)

// Render the supplied CompositeResourcePublication's corresponding
// CustomResourceDefinition.
func (fn CRDRenderFn) Render(d *v1alpha1.CompositeResourceDefinition, p *v1alpha1.CompositeResourcePublication) (*v1beta1.CustomResourceDefinition, error) {
	return fn(d, p)
}

// Setup adds a controller that reconciles CompositeResourcePublications.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "apiextensions/" + strings.ToLower(v1alpha1.CompositeResourcePublicationGroupKind)

	// This controller is for (i.e. reconciles) CompositeResourcePublications, and
	// owns (i.e. creates) CustomResourceDefinitions. It also watches for
	// CompositeResourceDefinitions, because an CompositeResourcePublication publishes
	// an CompositeResourceDefinition. A change to the CompositeResourceDefinition may
	// require a change to the CompositeResourcePublication's CRD. Note that we
	// (ab)use EnqueueRequestForObject when watching CompositeResourceDefinitions.
	// We require (by convention) that the CompositeResourcePublication share the
	// name of the CompositeResourceDefinition it publishes, so when we enqueue a
	// request for the name of the CompositeResourceDefinition we're actually
	// enqueueing a request for the CompositeResourcePublication of the same name.
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.CompositeResourcePublication{}).
		Owns(&v1beta1.CustomResourceDefinition{}).
		Watches(&source.Kind{Type: &v1alpha1.CompositeResourceDefinition{}}, &handler.EnqueueRequestForObject{}).
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
// CompositeResourcePublications and CompositeResourceDefinitions.
func WithFinalizer(f Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.publication.Finalizer = f
	}
}

// WithControllerEngine specifies how the Reconciler should manage the
// lifecycles of claim controllers.
func WithControllerEngine(c ControllerEngine) ReconcilerOption {
	return func(r *Reconciler) {
		r.publication.ControllerEngine = c
	}
}

// WithCRDRenderer specifies how the Reconciler should render an
// CompositeResourcePublication's corresponding CustomResourceDefinition.
func WithCRDRenderer(c CRDRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.publication.CRDRenderer = c
	}
}

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = ca
	}
}

// NewReconciler returns a Reconciler of CompositeResourcePublications.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())
	rd := func(d *v1alpha1.CompositeResourceDefinition, p *v1alpha1.CompositeResourcePublication) (*v1beta1.CustomResourceDefinition, error) {
		return ccrd.New(ccrd.PublishesCompositeResourceDefinition(d, p))
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

		publication: publication{
			CRDRenderer:      CRDRenderFn(rd),
			ControllerEngine: controller.NewEngine(mgr),
			Finalizer:        NewMultiFinalizer(resource.NewAPIFinalizer(kube, finalizer)),
		},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

type publication struct {
	CRDRenderer
	ControllerEngine
	Finalizer
}

// A Reconciler reconciles CompositeResourcePublications.
type Reconciler struct {
	mgr    manager.Manager
	client resource.ClientApplicator

	publication publication

	log    logging.Logger
	record event.Recorder
}

// Reconcile an CompositeResourcePublication.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): Like most Reconcile methods, this one is over our cyclomatic
	// complexity goal. Be wary when adding branches, and look for functionality
	// that could be reasonably moved into an injected dependency.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	p := &v1alpha1.CompositeResourcePublication{}
	if err := r.client.Get(ctx, req.NamespacedName, p); err != nil {
		log.Debug(errGetXRP, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetXRP)
	}

	log = log.WithValues(
		"uid", p.GetUID(),
		"version", p.GetResourceVersion(),
		"name", p.GetName(),
	)

	d := &v1alpha1.CompositeResourceDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, d); err != nil {
		log.Debug(errGetXRD)
		r.record.Event(p, event.Warning(reasonGetDef, err))
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errGetXRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	r.record.Event(p, event.Normal(reasonGetDef, "Got published CompositeResourceDefinition"))

	crd, err := r.publication.Render(d, p)
	if err != nil {
		log.Debug(errRenderCRD, "error", err)
		r.record.Event(p, event.Warning(reasonRenderCRD, err))
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errRenderCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	r.record.Event(p, event.Normal(reasonRenderCRD, "Rendered CustomResourceDefinition"))

	if meta.WasDeleted(p) {
		p.Status.SetConditions(v1alpha1.Deleting())
		r.record.Event(p, event.Normal(reasonDeletePub, "Deleting CompositeResourcePublication"))

		nn := types.NamespacedName{Name: crd.GetName()}
		if err := r.client.Get(ctx, nn, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errGetCRD, "error", err)
			r.record.Event(p, event.Warning(reasonDeletePub, err))
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errGetCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
		}

		// The CRD has no creation timestamp, or we don't control it. Most
		// likely we successfully deleted it on a previous reconcile. It's also
		// possible that we're being asked to delete it before we got around to
		// creating it, or that we lost control of it around the same time we
		// were deleted. In the (presumably exceedingly rare) latter case we'll
		// orphan the CRD.
		if !meta.WasCreated(crd) || !metav1.IsControlledBy(crd, p) {
			// It's likely that we've already stopped this controller on a
			// previous reconcile, but we try again just in case. This is a
			// no-op if the controller was already stopped.
			r.publication.Stop(claim.ControllerName(p.GetName()))

			if err := r.publication.RemoveFinalizer(ctx, d, p); err != nil {
				log.Debug(errRemoveFinalizer, "error", err)
				r.record.Event(p, event.Warning(reasonDeletePub, err))
				p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errRemoveFinalizer)))
				return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
			}

			// We're all done deleting and have removed our finalizer. There's
			// no need to requeue because there's nothing left to do.
			r.record.Event(p, event.Normal(reasonDeletePub, "Successfully deleted CompositeResourcePublication"))
			return reconcile.Result{Requeue: false}, nil
		}

		l := &kunstructured.UnstructuredList{}
		l.SetGroupVersionKind(Published(d.GetDefinedGroupVersionKind()))
		if err := r.client.List(ctx, l); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
			log.Debug(errListCRs, "error", err)
			r.record.Event(p, event.Warning(reasonDeletePub, err))
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errListCRs)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
		}

		// Ensure all the custom resources we defined are gone before stopping
		// the controller we started to reconcile them. This ensures the
		// controller has a chance to execute its cleanup logic, if any.
		if len(l.Items) > 0 {
			// TODO(negz): DeleteAllOf does not work here, despite working in
			// the definition controller. Could this be due to claims
			// being namespaced rather than cluster scoped?
			for i := range l.Items {
				if err := r.client.Delete(ctx, &l.Items[i]); resource.IgnoreNotFound(err) != nil {
					log.Debug(errDeleteCR, "error", err)
					r.record.Event(p, event.Warning(reasonDeletePub, err))
					p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errDeleteCR)))
					return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
				}
			}

			// We requeue to confirm that all the custom resources we just
			// deleted are actually gone. We need to requeue after a tiny wait
			// because we won't be requeued implicitly when the CRs are deleted.
			log.Debug(waitCRDelete)
			r.record.Event(p, event.Normal(reasonDeletePub, waitCRDelete))
			p.Status.SetConditions(runtimev1alpha1.ReconcileSuccess().WithMessage(waitCRDelete))
			return reconcile.Result{RequeueAfter: tinyWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
		}

		// The controller should be stopped before the deletion of CRD so that
		// it doesn't crash.
		r.publication.Stop(claim.ControllerName(p.GetName()))

		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteCRD, "error", err)
			r.record.Event(p, event.Warning(reasonDeletePub, err))
			p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errDeleteCRD)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
		}

		// We should be requeued implicitly because we're watching the
		// CustomResourceDefinition that we just deleted, but we requeue after
		// a tiny wait just in case the CRD isn't gone after the first requeue.
		log.Debug("Stopped claim controller and deleted CustomResourceDefinition")
		r.record.Event(p, event.Normal(reasonDeletePub, "Stopped claim controller and deleted CustomResourceDefinition"))
		p.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: tinyWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	// Note that we add a finalizer to the CompositeResourceDefinition that we
	// publish. Infrastructure cannot be published without first being defined.
	// If the CompositeResourceDefinition is deleted nothing is reconciling the
	// composite associated with the claim we offer. The definition does not
	// strictly own the publication, but we might consider making the definition
	// an owner of the publication so that the publication is deleted when its
	// corresponding definition is deleted.
	if err := r.publication.AddFinalizer(ctx, d, p); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		r.record.Event(p, event.Warning(reasonApplyPub, err))
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errAddFinalizer)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	if err := r.client.Apply(ctx, crd, resource.MustBeControllableBy(p.GetUID())); err != nil {
		log.Debug(errApplyCRD, "error", err)
		r.record.Event(p, event.Warning(reasonApplyPub, err))
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errApplyCRD)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	if !ccrd.IsEstablished(crd.Status) {
		log.Debug(waitCRDEstablish)
		r.record.Event(p, event.Normal(reasonApplyPub, waitCRDEstablish))
		p.Status.SetConditions(runtimev1alpha1.ReconcileSuccess().WithMessage(waitCRDEstablish))
		return reconcile.Result{RequeueAfter: tinyWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	o := kcontroller.Options{Reconciler: claim.NewReconciler(r.mgr,
		resource.CompositeClaimKind(Published(d.GetDefinedGroupVersionKind())),
		resource.CompositeKind(d.GetDefinedGroupVersionKind()),
		claim.WithLogger(log.WithValues("controller", claim.ControllerName(p.GetName()))),
		claim.WithRecorder(r.record.WithAnnotations("controller", claim.ControllerName(p.GetName()))),
	)}

	cm := &kunstructured.Unstructured{}
	cm.SetGroupVersionKind(Published(d.GetDefinedGroupVersionKind()))

	cp := &kunstructured.Unstructured{}
	cp.SetGroupVersionKind(d.GetDefinedGroupVersionKind())

	if err := r.publication.Start(claim.ControllerName(p.GetName()), o,
		controller.For(cm, &handler.EnqueueRequestForObject{}),
		controller.For(cp, &EnqueueRequestForClaim{}),
	); err != nil {
		log.Debug(errStartController, "error", err)
		r.record.Event(p, event.Warning(reasonApplyPub, err))
		p.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errStartController)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	p.Status.SetConditions(v1alpha1.Started())
	p.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	r.record.Event(p, event.Normal(reasonApplyPub, "Applied CustomResourceDefinition and (re)started claim controller"))
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
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
