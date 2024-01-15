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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	runtimeevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/secrets/v1alpha1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	apiextensionscontroller "github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
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
type ControllerEngine interface {
	IsRunning(name string) bool
	Create(name string, o kcontroller.Options, w ...controller.Watch) (controller.NamedController, error)
	Start(name string, o kcontroller.Options, w ...controller.Watch) error
	Stop(name string)
	Err(name string) error
}

// A CRDRenderer renders a CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderer interface {
	Render(d *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error)
}

// A CRDRenderFn renders a CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
type CRDRenderFn func(d *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error)

// Render the supplied CompositeResourceDefinition's corresponding
// CustomResourceDefinition.
func (fn CRDRenderFn) Render(d *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
	return fn(d)
}

// Setup adds a controller that reconciles CompositeResourceDefinitions by
// defining a composite resource and starting a controller to reconcile it.
func Setup(mgr ctrl.Manager, o apiextensionscontroller.Options) error {
	name := "defined/" + strings.ToLower(v1.CompositeResourceDefinitionGroupKind)

	r := NewReconciler(mgr, o,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	if o.Features.Enabled(features.EnableRealtimeCompositions) {
		// Register a runnable regularly checking whether the watch composed
		// resources are still referenced by composite resources. If not, the
		// composed resource informer is stopped.
		if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			// Run every minute.
			wait.UntilWithContext(ctx, r.xrInformers.cleanupComposedResourceInformers, time.Minute)
			return nil
		})); err != nil {
			return errors.Wrap(err, errCannotAddInformerLoopToManager)
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.CompositeResourceDefinition{}).
		Owns(&extv1.CustomResourceDefinition{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
		r.xrInformers.log = log
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
func NewReconciler(mgr manager.Manager, o apiextensionscontroller.Options, opts ...ReconcilerOption) *Reconciler {
	kube := unstructured.NewClient(mgr.GetClient())

	ca := controller.NewGVKRoutedCache(mgr.GetScheme(), mgr.GetCache())
	if o.Features.Enabled(features.EnableRealtimeCompositions) {
		// wrap the manager's cache to route requests to dynamically started
		// informers for managed resources.
		mgr = controller.WithGVKRoutedCache(ca, mgr)
	}

	r := &Reconciler{
		mgr: mgr,

		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewAPIUpdatingApplicator(kube),
		},

		composite: definition{
			CRDRenderer:      CRDRenderFn(xcrd.ForCompositeResource),
			ControllerEngine: controller.NewEngine(mgr),
			Finalizer:        resource.NewAPIFinalizer(kube, finalizer),
		},

		xrInformers: composedResourceInformers{
			log:     logging.NewNopLogger(),
			cluster: mgr,

			gvkRoutedCache: ca,
			cdCaches:       make(map[schema.GroupVersionKind]cdCache),
			sinks:          make(map[string]func(ev runtimeevent.UpdateEvent)),
		},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),

		options: o,
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

	xrInformers composedResourceInformers

	options apiextensionscontroller.Options
}

// Reconcile a CompositeResourceDefinition by defining a new kind of composite
// resource and starting a controller to reconcile it.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Reconcilers are complex. Be wary of adding more.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d := &v1.CompositeResourceDefinition{}
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
		err = errors.Wrap(err, errRenderCRD)
		r.record.Event(d, event.Warning(reasonRenderCRD, err))
		return reconcile.Result{}, err
	}

	if meta.WasDeleted(d) {
		d.Status.SetConditions(v1.TerminatingComposite())
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
			// It's likely that we've already stopped this
			// controller on a previous reconcile, but we try again
			// just in case. This is a no-op if the controller was
			// already stopped.
			r.composite.Stop(composite.ControllerName(d.GetName()))
			r.record.Event(d, event.Normal(reasonTerminateXR, "Stopped composite resource controller"))

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

		// The controller should be stopped before the deletion of CRD
		// so that it doesn't crash.
		r.composite.Stop(composite.ControllerName(d.GetName()))
		log.Debug("Stopped composite resource controller")
		r.record.Event(d, event.Normal(reasonTerminateXR, "Stopped composite resource controller"))

		if err := r.client.Delete(ctx, crd); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteCRD, "error", err)
			err = errors.Wrap(err, errDeleteCRD)
			r.record.Event(d, event.Warning(reasonTerminateXR, err))
			return reconcile.Result{}, err
		}
		log.Debug("Deleted composite resource CustomResourceDefinition")
		r.record.Event(d, event.Normal(reasonTerminateXR, "Deleted composite resource CustomResourceDefinition"))

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

	if err := r.composite.Err(composite.ControllerName(d.GetName())); err != nil {
		log.Debug("Composite resource controller encountered an error", "error", err)
	}

	observed := d.Status.Controllers.CompositeResourceTypeRef
	desired := v1.TypeReferenceTo(d.GetCompositeGroupVersionKind())
	if observed.APIVersion != "" && observed != desired {
		r.composite.Stop(composite.ControllerName(d.GetName()))
		if r.options.Features.Enabled(features.EnableRealtimeCompositions) {
			r.xrInformers.UnregisterComposite(d.GetCompositeGroupVersionKind())
		}
		log.Debug("Referenceable version changed; stopped composite resource controller",
			"observed-version", observed.APIVersion,
			"desired-version", desired.APIVersion)
	}

	ro := CompositeReconcilerOptions(r.options, d, r.client, r.log, r.record)
	ck := resource.CompositeKind(d.GetCompositeGroupVersionKind())
	if r.options.Features.Enabled(features.EnableRealtimeCompositions) {
		ro = append(ro, composite.WithKindObserver(composite.KindObserverFunc(r.xrInformers.WatchComposedResources)))
	}
	cr := composite.NewReconciler(r.mgr, ck, ro...)
	ko := r.options.ForControllerRuntime()
	ko.Reconciler = ratelimiter.NewReconciler(composite.ControllerName(d.GetName()), errors.WithSilentRequeueOnConflict(cr), r.options.GlobalRateLimiter)

	xrGVK := d.GetCompositeGroupVersionKind()

	u := &kunstructured.Unstructured{}
	u.SetGroupVersionKind(xrGVK)

	name := composite.ControllerName(d.GetName())
	var ca cache.Cache
	watches := []controller.Watch{
		controller.For(u, &handler.EnqueueRequestForObject{}),
		// enqueue composites whenever a matching CompositionRevision is created
		controller.TriggeredBy(source.Kind(r.mgr.GetCache(), &v1.CompositionRevision{}), handler.Funcs{
			CreateFunc: composite.EnqueueForCompositionRevisionFunc(ck, r.mgr.GetCache().List, r.log),
		}),
	}
	if r.options.Features.Enabled(features.EnableRealtimeCompositions) {
		// enqueue XRs that when a relevant MR is updated
		watches = append(watches, controller.TriggeredBy(&r.xrInformers, handler.Funcs{
			UpdateFunc: func(ctx context.Context, ev runtimeevent.UpdateEvent, q workqueue.RateLimitingInterface) {
				enqueueXRsForMR(ca, xrGVK, log)(ctx, ev, q)
			},
		}))
	}

	c, err := r.composite.Create(name, ko, watches...)
	if err != nil {
		log.Debug(errStartController, "error", err)
		err = errors.Wrap(err, errStartController)
		r.record.Event(d, event.Warning(reasonEstablishXR, err))
		return reconcile.Result{}, err
	}

	if r.options.Features.Enabled(features.EnableRealtimeCompositions) {
		ca = c.GetCache()
		if err := ca.IndexField(ctx, u, compositeResourceRefGVKsIndex, IndexCompositeResourceRefGVKs); err != nil {
			log.Debug(errAddIndex, "error", err)
			// Nothing we can do. At worst, we won't have realtime updates.
		}
		if err := ca.IndexField(ctx, u, compositeResourcesRefsIndex, IndexCompositeResourcesRefs); err != nil {
			log.Debug(errAddIndex, "error", err)
			// Nothing we can do. At worst, we won't have realtime updates.
		}
	}

	if err := c.Start(context.Background()); err != nil { //nolint:contextcheck // the controller actually runs in the background.
		log.Debug(errStartController, "error", err)
		err = errors.Wrap(err, errStartController)
		r.record.Event(d, event.Warning(reasonEstablishXR, err))
		return reconcile.Result{}, err
	}

	if r.options.Features.Enabled(features.EnableRealtimeCompositions) {
		r.xrInformers.RegisterComposite(xrGVK, ca)
	}

	d.Status.Controllers.CompositeResourceTypeRef = v1.TypeReferenceTo(d.GetCompositeGroupVersionKind())
	d.Status.SetConditions(v1.WatchingComposite())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, d), errUpdateStatus)
}

// CompositeReconcilerOptions builds the options for a composite resource
// reconciler. The options vary based on the supplied feature flags.
func CompositeReconcilerOptions(co apiextensionscontroller.Options, d *v1.CompositeResourceDefinition, c client.Client, l logging.Logger, e event.Recorder) []composite.ReconcilerOption {
	// The default set of reconciler options when no feature flags are enabled.
	o := []composite.ReconcilerOption{
		composite.WithConnectionPublishers(composite.NewAPIFilteredSecretPublisher(c, d.GetConnectionSecretKeys())),
		composite.WithCompositionSelector(composite.NewCompositionSelectorChain(
			composite.NewEnforcedCompositionSelector(*d, e),
			composite.NewAPIDefaultCompositionSelector(c, *meta.ReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind), e),
			composite.NewAPILabelSelectorResolver(c),
		)),
		composite.WithLogger(l.WithValues("controller", composite.ControllerName(d.GetName()))),
		composite.WithRecorder(e.WithAnnotations("controller", composite.ControllerName(d.GetName()))),
		composite.WithPollInterval(co.PollInterval),
	}

	// We only want to enable Composition environment support if the relevant
	// feature flag is enabled. Otherwise we will default to noop selector and
	// fetcher that will always return nil. All environment features are
	// subsequently skipped if the environment is nil.
	if co.Features.Enabled(features.EnableAlphaEnvironmentConfigs) {
		o = append(o,
			composite.WithEnvironmentSelector(composite.NewAPIEnvironmentSelector(c)),
			composite.WithEnvironmentFetcher(composite.NewAPIEnvironmentFetcher(c)))
	}

	// If external secret stores aren't enabled we just fetch connection details
	// from Kubernetes secrets.
	var fetcher managed.ConnectionDetailsFetcher = composite.NewSecretConnectionDetailsFetcher(c)

	// We only want to enable ExternalSecretStore support if the relevant
	// feature flag is enabled. Otherwise, we start the XR reconcilers with
	// their default ConnectionPublisher and ConnectionDetailsFetcher.
	// We also add a new Configurator for ExternalSecretStore which basically
	// reflects PublishConnectionDetailsWithStoreConfigRef in Composition to
	// the composite resource.
	if co.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		pc := []managed.ConnectionPublisher{
			composite.NewAPIFilteredSecretPublisher(c, d.GetConnectionSecretKeys()),
			composite.NewSecretStoreConnectionPublisher(connection.NewDetailsManager(c, v1alpha1.StoreConfigGroupVersionKind,
				connection.WithTLSConfig(co.ESSOptions.TLSConfig)), d.GetConnectionSecretKeys()),
		}

		// If external secret stores are enabled we need to support fetching
		// connection details from both secrets and external stores.
		fetcher = composite.ConnectionDetailsFetcherChain{
			composite.NewSecretConnectionDetailsFetcher(c),
			connection.NewDetailsManager(c, v1alpha1.StoreConfigGroupVersionKind, connection.WithTLSConfig(co.ESSOptions.TLSConfig)),
		}

		cc := composite.NewConfiguratorChain(
			composite.NewAPINamingConfigurator(c),
			composite.NewAPIConfigurator(c),
			composite.NewSecretStoreConnectionDetailsConfigurator(c),
		)

		o = append(o,
			composite.WithConnectionPublishers(pc...),
			composite.WithConfigurator(cc),
			composite.WithComposer(composite.NewPTComposer(c, composite.WithComposedConnectionDetailsFetcher(fetcher))))
	}

	// If Composition Functions are enabled we use two different Composer
	// implementations. One supports P&T (aka 'Resources mode') and the other
	// Functions (aka 'Pipeline mode').
	if co.Features.Enabled(features.EnableBetaCompositionFunctions) {
		ptc := composite.NewPTComposer(c, composite.WithComposedConnectionDetailsFetcher(fetcher))

		fcopts := []composite.FunctionComposerOption{
			composite.WithComposedResourceObserver(composite.NewExistingComposedResourceObserver(c, fetcher)),
			composite.WithCompositeConnectionDetailsFetcher(fetcher),
		}

		if co.Features.Enabled(features.EnableBetaCompositionFunctionsExtraResources) {
			fcopts = append(fcopts, composite.WithExtraResourcesGetter(composite.NewExistingExtraResourcesGetter(c)))
		}

		fc := composite.NewFunctionComposer(c, co.FunctionRunner, fcopts...)

		// Note that if external secret stores are enabled this will supersede
		// the WithComposer option specified in that block.
		o = append(o, composite.WithComposer(composite.ComposerSelectorFn(func(cm *v1.CompositionMode) composite.Composer {
			// Resources mode is the implicit default.
			m := v1.CompositionModeResources
			if cm != nil {
				m = *cm
			}
			switch m {
			case v1.CompositionModeResources:
				return ptc
			case v1.CompositionModePipeline:
				return fc
			default:
				// This shouldn't be possible, but just in case return the
				// default Composer.
				return ptc
			}
		})))
	}

	return o
}
