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

// Package definition manages the lifecycle of XR controllers.
package definition

import (
	"context"
	"fmt"
	"strings"
	"time"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	ucomposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/shared"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite/watch"
	apiextensionscontroller "github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/engine"
	"github.com/crossplane/crossplane/internal/features"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	timeout   = 2 * time.Minute
	finalizer = "defined.apiextensions.crossplane.io"

	errGetXRD                         = "cannot get CompositeResourceDefinition"
	errRenderCRD                      = "cannot render composite resource CustomResourceDefinition"
	errGetCRD                         = "cannot get composite resource CustomResourceDefinition"
	errApplyCRD                       = "cannot apply rendered composite resource CustomResourceDefinition"
	errUpdateStatus                   = "cannot update status of CompositeResourceDefinition"
	errStartController                = "cannot start composite resource controller"
	errStopController                 = "cannot stop composite resource controller"
	errStartWatches                   = "cannot start composite resource controller watches"
	errAddIndex                       = "cannot add composite GVK index"
	errAddFinalizer                   = "cannot add composite resource finalizer"
	errRemoveFinalizer                = "cannot remove composite resource finalizer"
	errDeleteCRD                      = "cannot delete composite resource CustomResourceDefinition"
	errListCRs                        = "cannot list defined composite resources"
	errDeleteCRs                      = "cannot delete defined composite resources"
	errListCRDs                       = "cannot list CustomResourceDefinitions"
	errCannotAddInformerLoopToManager = "cannot add resources informer loop to manager"
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
//
//nolint:interfacebloat // We use this interface to stub the engine for testing, and we need all of its functionality.
type ControllerEngine interface {
	Start(name string, o ...engine.ControllerOption) error
	Stop(ctx context.Context, name string) error
	IsRunning(name string) bool
	GetWatches(name string) ([]engine.WatchID, error)
	StartWatches(ctx context.Context, name string, ws ...engine.Watch) error
	StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
	GetCached() client.Client
	GetUncached() client.Client
	GetFieldIndexer() client.FieldIndexer
}

// A NopEngine does nothing.
type NopEngine struct{}

// Start does nothing.
func (e *NopEngine) Start(_ string, _ ...engine.ControllerOption) error {
	return nil
}

// Stop does nothing.
func (e *NopEngine) Stop(_ context.Context, _ string) error { return nil }

// IsRunning always returns true.
func (e *NopEngine) IsRunning(_ string) bool { return true }

// GetWatches does nothing.
func (e *NopEngine) GetWatches(_ string) ([]engine.WatchID, error) { return nil, nil }

// StartWatches does nothing.
func (e *NopEngine) StartWatches(_ context.Context, _ string, _ ...engine.Watch) error { return nil }

// StopWatches does nothing.
func (e *NopEngine) StopWatches(_ context.Context, _ string, _ ...engine.WatchID) (int, error) {
	return 0, nil
}

// GetCached returns a nil client.
func (e *NopEngine) GetCached() client.Client {
	return nil
}

// GetUncached returns a nil client.
func (e *NopEngine) GetUncached() client.Client {
	return nil
}

// GetFieldIndexer returns a nil field indexer.
func (e *NopEngine) GetFieldIndexer() client.FieldIndexer {
	return nil
}

// A CRDRenderer renders a CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderer interface {
	Render(d *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error)
}

// A CRDRenderFn renders a CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderFn func(d *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error)

// Render the supplied CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
func (fn CRDRenderFn) Render(d *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
	return fn(d)
}

// Setup adds a controller that reconciles CompositeResourceDefinitions by
// defining a composite resource and starting a controller to reconcile it.
func Setup(mgr ctrl.Manager, o apiextensionscontroller.Options) error {
	name := "defined/" + strings.ToLower(v2.CompositeResourceDefinitionGroupKind)

	r := NewReconciler(NewClientApplicator(mgr.GetClient()),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithControllerEngine(o.ControllerEngine),
		WithOptions(o))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v2.CompositeResourceDefinition{}).
		Owns(&extv1.CustomResourceDefinition{}, builder.WithPredicates(resource.NewPredicates(IsCompositeResourceCRD()))).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
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

// WithOptions lets the Reconciler know which options to pass to new composite
// resource controllers.
func WithOptions(o apiextensionscontroller.Options) ReconcilerOption {
	return func(r *Reconciler) {
		r.options = o
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
		r.engine = c
	}
}

// WithCRDRenderer specifies how the Reconciler should render a
// CompositeResourceDefinition's corresponding CustomResourceDefinition.
func WithCRDRenderer(c CRDRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CRDRenderer = c
	}
}

type definition struct {
	CRDRenderer
	resource.Finalizer
}

// NewClientApplicator returns a ClientApplicator suitable for use by the
// definition controller.
func NewClientApplicator(c client.Client) resource.ClientApplicator {
	// TODO(negz): Use server-side apply instead of a ClientApplicator.
	return resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)}
}

// NewReconciler returns a Reconciler of CompositeResourceDefinitions.
func NewReconciler(ca resource.ClientApplicator, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: ca,

		composite: definition{
			CRDRenderer: CRDRenderFn(xcrd.ForCompositeResource),
			Finalizer:   resource.NewAPIFinalizer(ca, finalizer),
		},

		engine: &NopEngine{},

		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},

		options: apiextensionscontroller.Options{
			Options: controller.DefaultOptions(),
		},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// A Reconciler reconciles CompositeResourceDefinitions.
type Reconciler struct {
	// This client should only be used by this XRD controller, not the XR
	// controllers it manages. XR controllers should use the engine's client.
	// This ensures XR controllers will use a client backed by the same cache
	// used to power their watches.
	client resource.ClientApplicator

	composite definition

	engine ControllerEngine

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager

	options apiextensionscontroller.Options
}

// Reconcile a CompositeResourceDefinition by defining a new kind of composite
// resource and starting a controller to reconcile it.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are complex. Be wary of adding more.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d := &v2.CompositeResourceDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, d); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetXRD, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetXRD)
	}

	status := r.conditions.For(d)

	log = log.WithValues(
		"uid", d.GetUID(),
		"version", d.GetResourceVersion(),
		"name", d.GetName(),
	)

	crd, err := r.composite.Render(d)
	if err != nil {
		err = errors.Wrap(err, errRenderCRD)
		r.record.Event(d, event.Warning(reasonRenderCRD, err))

		return reconcile.Result{}, err
	}

	if meta.WasDeleted(d) {
		status.MarkConditions(shared.TerminatingComposite())

		if err := r.client.Status().Update(ctx, d); err != nil {
			log.Debug(errUpdateStatus, "error", err)

			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errUpdateStatus)

			return reconcile.Result{}, err
		}

		nn := types.NamespacedName{Name: crd.GetName()}
		if err := r.client.Get(ctx, nn, crd); resource.IgnoreNotFound(err) != nil {
			err = errors.Wrap(err, errGetCRD)
			r.record.Event(d, event.Warning(reasonTerminateXR, err))

			return reconcile.Result{}, err
		}

		// The CRD has no creation timestamp, or we don't control it.
		// Most likely we successfully deleted it on a previous
		// reconcile. It's also possible that we're being asked to
		// delete it before we got around to creating it, or that we
		// lost control of it around the same time we were deleted. In
		// the (presumably exceedingly rare) latter case we'll orphan
		// the CRD.
		if !meta.WasCreated(crd) || !metav1.IsControlledBy(crd, d) {
			// It's likely that we've already stopped this controller on a
			// previous reconcile, but we try again just in case. This is a
			// no-op if the controller was already stopped.
			if err := r.engine.Stop(ctx, composite.ControllerName(d.GetName())); err != nil {
				err = errors.Wrap(err, errStopController)
				r.record.Event(d, event.Warning(reasonTerminateXR, err))

				return reconcile.Result{}, err
			}

			log.Debug("Stopped composite resource controller")

			if err := r.composite.RemoveFinalizer(ctx, d); err != nil {
				if kerrors.IsConflict(err) {
					return reconcile.Result{Requeue: true}, nil
				}

				err = errors.Wrap(err, errRemoveFinalizer)
				r.record.Event(d, event.Warning(reasonTerminateXR, err))

				return reconcile.Result{}, err
			}

			// We're all done deleting and have removed our
			// finalizer. There's no need to requeue because there's
			// nothing left to do.
			return reconcile.Result{Requeue: false}, nil
		}

		// NOTE(muvaf): When user deletes CompositeResourceDefinition
		// object the deletion signal does not cascade to the owned
		// resource until owner is gone. But owner has its own finalizer
		// that depends on having no instance of the CRD because it
		// cannot go away before stopping the controller. So, we need to
		// delete all defined custom resources manually here.
		o := &kunstructured.Unstructured{}
		o.SetGroupVersionKind(d.GetCompositeGroupVersionKind())

		if err := r.client.DeleteAllOf(ctx, o); err != nil && !kmeta.IsNoMatchError(err) && !kerrors.IsNotFound(err) {
			err = errors.Wrap(err, errDeleteCRs)
			r.record.Event(d, event.Warning(reasonTerminateXR, err))

			return reconcile.Result{}, err
		}

		l := &kunstructured.UnstructuredList{}
		l.SetGroupVersionKind(d.GetCompositeGroupVersionKind())

		if err := r.client.List(ctx, l); resource.Ignore(kmeta.IsNoMatchError, err) != nil {
			log.Debug("cannot list composite resources to check whether the XRD can be deleted", "error", err, "gvk", d.GetCompositeGroupVersionKind().String())
			err = errors.Wrap(err, errListCRs)
			r.record.Event(d, event.Warning(reasonTerminateXR, err))

			return reconcile.Result{}, err
		}

		// namedController should be stopped only after all instances are
		// gone so that deletion logic of the instances are processed by
		// the controller.
		if len(l.Items) > 0 {
			log.Debug(waitCRDelete)
			r.record.Event(d, event.Normal(reasonTerminateXR, waitCRDelete))

			return reconcile.Result{Requeue: true}, nil
		}

		// The controller must be stopped before the deletion of the CRD so that
		// it doesn't crash.
		if err := r.engine.Stop(ctx, composite.ControllerName(d.GetName())); err != nil {
			err = errors.Wrap(err, errStopController)
			r.record.Event(d, event.Warning(reasonTerminateXR, err))

			return reconcile.Result{}, err
		}

		log.Debug("Stopped composite resource controller")

		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteCRD, "error", err)
			err = errors.Wrap(err, errDeleteCRD)
			r.record.Event(d, event.Warning(reasonTerminateXR, err))

			return reconcile.Result{}, err
		}

		log.Debug("Deleted composite resource CustomResourceDefinition")
		r.record.Event(d, event.Normal(reasonTerminateXR, fmt.Sprintf("Deleted composite resource CustomResourceDefinition: %s", crd.GetName())))

		// We should be requeued implicitly because we're watching the
		// CustomResourceDefinition that we just deleted, but we requeue
		// just in case the CRD isn't gone after the first requeue.
		return reconcile.Result{Requeue: true}, nil
	}

	if err := r.composite.AddFinalizer(ctx, d); err != nil {
		log.Debug(errAddFinalizer, "error", err)

		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(d, event.Warning(reasonEstablishXR, err))

		return reconcile.Result{}, err
	}

	origRV := ""
	if err := r.client.Apply(ctx, crd, resource.MustBeControllableBy(d.GetUID()), resource.StoreCurrentRV(&origRV)); err != nil {
		log.Debug(errApplyCRD, "error", err)

		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errApplyCRD)
		r.record.Event(d, event.Warning(reasonEstablishXR, err))

		return reconcile.Result{}, err
	}

	if crd.GetResourceVersion() != origRV {
		r.record.Event(d, event.Normal(reasonEstablishXR, fmt.Sprintf("Applied composite resource CustomResourceDefinition: %s", crd.GetName())))
	}

	if !xcrd.IsEstablished(crd.Status) {
		log.Debug(waitCRDEstablish)
		r.record.Event(d, event.Normal(reasonEstablishXR, waitCRDEstablish))

		return reconcile.Result{Requeue: true}, nil
	}

	observed := d.Status.Controllers.CompositeResourceTypeRef

	desired := v2.TypeReferenceTo(d.GetCompositeGroupVersionKind())
	if observed.APIVersion != "" && observed != desired {
		if err := r.engine.Stop(ctx, composite.ControllerName(d.GetName())); err != nil {
			err = errors.Wrap(err, errStopController)
			r.record.Event(d, event.Warning(reasonEstablishXR, err))

			return reconcile.Result{}, err
		}

		log.Debug("Referenceable version changed; stopped composite resource controller",
			"observed-version", observed.APIVersion,
			"desired-version", desired.APIVersion)
	}

	if r.engine.IsRunning(composite.ControllerName(d.GetName())) {
		log.Debug("Composite resource controller is running")
		status.MarkConditions(shared.WatchingComposite())

		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
	}

	runner := composite.NewFetchingFunctionRunner(r.options.FunctionRunner, composite.NewExistingExtraResourcesFetcher(r.engine.GetCached()))
	fetcher := composite.NewSecretConnectionDetailsFetcher(r.engine.GetCached())
	fc := composite.NewFunctionComposer(r.engine.GetCached(), r.engine.GetUncached(), runner,
		composite.WithComposedResourceObserver(composite.NewExistingComposedResourceObserver(r.engine.GetCached(), r.engine.GetUncached(), fetcher)),
		composite.WithCompositeConnectionDetailsFetcher(fetcher),
	)

	// All XRs have modern schema unless their XRD's scope is LegacyCluster.
	schema := ucomposite.SchemaModern
	if d.Spec.Scope == "" || d.Spec.Scope == shared.CompositeResourceScopeLegacyCluster { //nolint:staticcheck // we are still supporting v1 XRD
		schema = ucomposite.SchemaLegacy
	}

	ro := []composite.ReconcilerOption{
		composite.WithCompositeSchema(schema),
		composite.WithCompositionSelector(composite.NewCompositionSelectorChain(
			composite.NewEnforcedCompositionSelector(*d, r.record),
			composite.NewAPIDefaultCompositionSelector(r.engine.GetCached(), *meta.ReferenceTo(d, v2.CompositeResourceDefinitionGroupVersionKind), r.record),
			composite.NewAPILabelSelectorResolver(r.engine.GetCached()),
		)),
		composite.WithLogger(r.log.WithValues("controller", composite.ControllerName(d.GetName()))),
		composite.WithRecorder(r.record.WithAnnotations("controller", composite.ControllerName(d.GetName()))),
		composite.WithPollInterval(r.options.PollInterval),
		composite.WithComposer(fc),
		composite.WithFeatures(r.options.Features),
	}

	if schema == ucomposite.SchemaLegacy {
		ro = append(ro,
			composite.WithConnectionPublishers(composite.NewAPIFilteredSecretPublisher(r.engine.GetCached(), d.GetConnectionSecretKeys())),
		)
	}

	// If realtime compositions are enabled we pass the ControllerEngine to the
	// XR reconciler so that it can start watches for composed resources.
	if r.options.Features.Enabled(features.EnableBetaRealtimeCompositions) {
		gvk := d.GetCompositeGroupVersionKind()
		u := &kunstructured.Unstructured{}
		u.SetAPIVersion(gvk.GroupVersion().String())
		u.SetKind(gvk.Kind)

		// Add an index to the controller engine's client.
		if err := r.engine.GetFieldIndexer().IndexField(ctx, u, compositeResourcesRefsIndex, IndexCompositeResourcesRefs(schema)); err != nil {
			r.log.Debug(errAddIndex, "error", err)
		}

		h := EnqueueCompositeResources(d.GetCompositeGroupVersionKind(), r.engine.GetCached(), r.log)
		ro = append(ro,
			composite.WithWatchStarter(composite.ControllerName(d.GetName()), h, r.engine),
			composite.WithPollInterval(0), // Disable polling.
		)
	}

	cr := composite.NewReconciler(r.engine.GetCached(), d.GetCompositeGroupVersionKind(), ro...)
	ko := r.options.ForControllerRuntime()

	// Most controllers use this type of rate limiter to backoff requeues from 1
	// to 60 seconds. Despite the name, it doesn't only rate limit requeues due
	// to errors. It also rate limits requeues due to a reconcile returning
	// {Requeue: true}. The XR reconciler returns {Requeue: true} while waiting
	// for composed resources to become ready, and we don't want to back off as
	// far as 60 seconds. Instead we cap the XR reconciler at 30 seconds.
	ko.RateLimiter = workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](1*time.Second, 30*time.Second)
	ko.Reconciler = ratelimiter.NewReconciler(composite.ControllerName(d.GetName()), errors.WithSilentRequeueOnConflict(cr), r.options.GlobalRateLimiter)

	gvk := d.GetCompositeGroupVersionKind()
	name := composite.ControllerName(d.GetName())

	// TODO(negz): Update CompositeReconcilerOptions to produce
	// ControllerOptions instead? It bothers me that this is the only feature
	// flagged block outside that method.
	co := []engine.ControllerOption{engine.WithRuntimeOptions(ko)}
	if r.options.Features.Enabled(features.EnableBetaRealtimeCompositions) {
		// If realtime composition is enabled we'll start watches dynamically,
		// so we want to garbage collect watches for composed resource kinds
		// that aren't used anymore.
		gc := watch.NewGarbageCollector(name, gvk, r.engine, watch.WithCompositeSchema(schema), watch.WithLogger(log))
		co = append(co, engine.WithWatchGarbageCollector(gc))
	}

	if err := r.engine.Start(name, co...); err != nil {
		log.Debug(errStartController, "error", err)
		err = errors.Wrap(err, errStartController)
		r.record.Event(d, event.Warning(reasonEstablishXR, err))

		return reconcile.Result{}, err
	}

	// This must be *unstructured.Unstructured, not *composite.Unstructured.
	// controller-runtime doesn't support watching types that satisfy the
	// runtime.Unstructured interface - only *unstructured.Unstructured.
	xr := &kunstructured.Unstructured{}
	xr.SetGroupVersionKind(gvk)

	crh := EnqueueForCompositionRevision(gvk, schema, r.engine.GetCached(), log)
	if err := r.engine.StartWatches(ctx, name,
		engine.WatchFor(xr, engine.WatchTypeCompositeResource, &handler.EnqueueRequestForObject{}),
		engine.WatchFor(&v1.CompositionRevision{}, engine.WatchTypeCompositionRevision, crh),
	); err != nil {
		log.Debug(errStartWatches, "error", err)
		err = errors.Wrap(err, errStartWatches)
		r.record.Event(d, event.Warning(reasonEstablishXR, err))

		return reconcile.Result{}, err
	}

	log.Debug("Started composite resource controller")

	d.Status.Controllers.CompositeResourceTypeRef = v2.TypeReferenceTo(d.GetCompositeGroupVersionKind())

	status.MarkConditions(shared.WatchingComposite())

	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
}
