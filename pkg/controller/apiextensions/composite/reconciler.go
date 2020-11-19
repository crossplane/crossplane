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

package composite

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
)

const (
	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
	timeout   = 2 * time.Minute
)

// Error strings
const (
	errGet          = "cannot get composite resource"
	errUpdate       = "cannot update composite resource"
	errUpdateStatus = "cannot update composite resource status"
	errSelectComp   = "cannot select Composition"
	errGetComp      = "cannot get Composition"
	errConfigure    = "cannot configure composite resource"
	errPublish      = "cannot publish connection details"
	errRender       = "cannot render composed resource"

	errFmtRender = "cannot render composed resource at index %d"
)

// Event reasons.
const (
	reasonResolve event.Reason = "SelectComposition"
	reasonCompose event.Reason = "ComposeResources"
	reasonPublish event.Reason = "PublishConnectionSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite resource.
func ControllerName(name string) string {
	return "composite/" + name
}

// ConnectionSecretFilterer returns a set of allowed keys.
type ConnectionSecretFilterer interface {
	GetConnectionSecretKeys() []string
}

// A ConnectionPublisher publishes the supplied ConnectionDetails for the
// supplied resource. Publishers must handle the case in which the supplied
// ConnectionDetails are empty.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied resource. Publishing must be
	// additive; i.e. if details (a, b, c) are published, subsequently
	// publishing details (b, c, d) should update (b, c) but not remove a.
	// Returns 'published' if the publish was not a no-op.
	PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error)
}

// A ConnectionPublisherFn publishes the supplied ConnectionDetails for the
// supplied resource.
type ConnectionPublisherFn func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error)

// PublishConnection details for the supplied resource.
func (fn ConnectionPublisherFn) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
	return fn(ctx, o, c)
}

// A CompositionSelector selects a composition reference.
type CompositionSelector interface {
	SelectComposition(ctx context.Context, cr resource.Composite) error
}

// A CompositionSelectorFn selects a composition reference.
type CompositionSelectorFn func(ctx context.Context, cr resource.Composite) error

// SelectComposition for the supplied composite resource.
func (fn CompositionSelectorFn) SelectComposition(ctx context.Context, cr resource.Composite) error {
	return fn(ctx, cr)
}

// A Configurator configures a composite resource using its composition.
type Configurator interface {
	Configure(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error
}

// A ConfiguratorFn configures a composite resource using its composition.
type ConfiguratorFn func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error

// Configure the supplied composite resource using its composition.
func (fn ConfiguratorFn) Configure(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
	return fn(ctx, cr, cp)
}

// A Renderer is used to render a composed resource.
type Renderer interface {
	Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error
}

// A RendererFn may be used to render a composed resource.
type RendererFn func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error

// Render the supplied composed resource using the supplied composite resource
// and template as inputs.
func (fn RendererFn) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
	return fn(ctx, cp, cd, t)
}

// ConnectionDetailsFetcher fetches the connection details of the Composed resource.
type ConnectionDetailsFetcher interface {
	FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (managed.ConnectionDetails, error)
}

// A ConnectionDetailsFetcherFn fetches the connection details of the supplied
// composed resource, if any.
type ConnectionDetailsFetcherFn func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (managed.ConnectionDetails, error)

// FetchConnectionDetails calls the FetchConnectionDetailsFn.
func (f ConnectionDetailsFetcherFn) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (managed.ConnectionDetails, error) {
	return f(ctx, cd, t)
}

// A ReadinessChecker checks whether a composed resource is ready or not.
type ReadinessChecker interface {
	IsReady(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (ready bool, err error)
}

// A ReadinessCheckerFn checks whether a composed resource is ready or not.
type ReadinessCheckerFn func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (ready bool, err error)

// IsReady reports whether a composed resource is ready or not.
func (fn ReadinessCheckerFn) IsReady(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (ready bool, err error) {
	return fn(ctx, cd, t)
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

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = ca
	}
}

// WithRenderer specifies how the Reconciler should render composed resources.
func WithRenderer(rd Renderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composed.Renderer = rd
	}
}

// WithConnectionDetailsFetcher specifies how the Reconciler should fetch the
// connection details of composed resources.
func WithConnectionDetailsFetcher(f ConnectionDetailsFetcher) ReconcilerOption {
	return func(r *Reconciler) {
		r.composed.ConnectionDetailsFetcher = f
	}
}

// WithReadinessChecker specifies how the Reconciler should fetch the connection
// details of composed resources.
func WithReadinessChecker(c ReadinessChecker) ReconcilerOption {
	return func(r *Reconciler) {
		r.composed.ReadinessChecker = c
	}
}

// WithCompositionSelector specifies how the composition to be used should be
// selected.
func WithCompositionSelector(p CompositionSelector) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CompositionSelector = p
	}
}

// WithConfigurator specifies how the Reconciler should configure
// composite resources using their composition.
func WithConfigurator(c Configurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Configurator = c
	}
}

// WithConnectionPublisher specifies how the Reconciler should publish
// connection secrets.
func WithConnectionPublisher(p ConnectionPublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ConnectionPublisher = p
	}
}

type compositeResource struct {
	CompositionSelector
	Configurator
	ConnectionPublisher
}

type composedResource struct {
	Renderer
	ConnectionDetailsFetcher
	ReadinessChecker
}

// NewReconciler returns a new Reconciler of composite resources.
func NewReconciler(mgr manager.Manager, of resource.CompositeKind, opts ...ReconcilerOption) *Reconciler {
	nc := func() resource.Composite {
		return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(of)))
	}
	kube := unstructured.NewClient(mgr.GetClient())

	r := &Reconciler{
		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewAPIPatchingApplicator(kube),
		},
		newComposite: nc,

		composite: compositeResource{
			CompositionSelector: NewAPILabelSelectorResolver(kube),
			Configurator:        NewConfiguratorChain(NewAPINamingConfigurator(kube), NewAPIConfigurator(kube)),
			ConnectionPublisher: NewAPIFilteredSecretPublisher(kube, []string{}),
		},

		composed: composedResource{
			Renderer:                 NewAPIDryRunRenderer(kube),
			ReadinessChecker:         ReadinessCheckerFn(IsReady),
			ConnectionDetailsFetcher: NewAPIConnectionDetailsFetcher(kube),
		},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles composite resources.
type Reconciler struct {
	client       resource.ClientApplicator
	newComposite func() resource.Composite

	composite compositeResource
	composed  composedResource

	log    logging.Logger
	record event.Recorder
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo
	// NOTE(negz): Like most Reconcile methods, this one is over our cyclomatic
	// complexity goal. Be wary when adding branches, and look for functionality
	// that could be reasonably moved into an injected dependency.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cr := r.newComposite()
	if err := r.client.Get(ctx, req.NamespacedName, cr); err != nil {
		log.Debug(errGet, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGet)
	}

	log = log.WithValues(
		"uid", cr.GetUID(),
		"version", cr.GetResourceVersion(),
		"name", cr.GetName(),
	)

	if err := r.composite.SelectComposition(ctx, cr); err != nil {
		log.Debug(errSelectComp, "error", err)
		r.record.Event(cr, event.Warning(reasonResolve, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}
	r.record.Event(cr, event.Normal(reasonResolve, "Successfully selected composition"))

	// TODO(muvaf): We should lock the deletion of Composition via finalizer
	// because its deletion will break the field propagation.
	comp := &v1beta1.Composition{}
	if err := r.client.Get(ctx, meta.NamespacedNameOf(cr.GetCompositionReference()), comp); err != nil {
		log.Debug(errGetComp, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	if err := r.composite.Configure(ctx, cr, comp); err != nil {
		log.Debug(errConfigure, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	log = log.WithValues(
		"composition-uid", comp.GetUID(),
		"composition-version", comp.GetResourceVersion(),
		"composition-name", comp.GetName(),
	)

	// TODO(muvaf): Since the composed reconciler returns only reference, it can
	// be parallelized via go routines.

	// In order to iterate over all composition targets, we create an empty ref
	// array with the same length. Then copy the already provisioned ones into
	// that array to not create new ones because composed reconciler assumes that
	// if the reference is empty, it needs to create the resource.

	// TODO(negz): This approach means that the resources of a Composition are
	// effectively append only. We may want to reconsider this per
	// https://github.com/crossplane/crossplane/issues/1909
	refs := make([]corev1.ObjectReference, len(comp.Spec.Resources))
	copy(refs, cr.GetResourceReferences())

	cds := make([]*composed.Unstructured, len(refs))
	for i := range refs {
		cd := composed.New(composed.FromReference(refs[i]))
		if err := r.composed.Render(ctx, cr, cd, comp.Spec.Resources[i]); err != nil {
			log.Debug(errRender, "error", err, "index", i)
			r.record.Event(cr, event.Warning(reasonCompose, errors.Wrapf(err, errFmtRender, i)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		cds[i] = cd
		refs[i] = *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind())
	}

	cr.SetResourceReferences(refs)
	if err := r.client.Update(ctx, cr); err != nil {
		log.Debug(errUpdate, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	conn := managed.ConnectionDetails{}
	ready := 0
	for i, cd := range cds {
		if err := r.client.Apply(ctx, cd, resource.MustBeControllableBy(cr.GetUID())); err != nil {
			log.Debug(errApply, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		// Connection details are fetched in all cases in a best-effort mode,
		// i.e. it doesn't return error if the secret does not exist or the
		// resource does not publish a secret at all.
		c, err := r.composed.FetchConnectionDetails(ctx, cd, comp.Spec.Resources[i])
		if err != nil {
			log.Debug(errFetchSecret, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		for key, val := range c {
			conn[key] = val
		}

		rdy, err := r.composed.IsReady(ctx, cd, comp.Spec.Resources[i])
		if err != nil {
			log.Debug(errReadiness, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		if rdy {
			ready++
		}
	}

	r.record.Event(cr, event.Normal(reasonCompose, "Successfully composed resources"))

	published, err := r.composite.PublishConnection(ctx, cr, conn)
	if err != nil {
		log.Debug(errPublish, "error", err)
		r.record.Event(cr, event.Warning(reasonPublish, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}
	if published {
		cr.SetConnectionDetailsLastPublishedTime(&metav1.Time{Time: time.Now()})
		log.Debug("Successfully published connection details")
		r.record.Event(cr, event.Normal(reasonPublish, "Successfully published connection details"))
	}

	// TODO(muvaf):
	// * Report which resources are not ready.
	// * If a resource becomes Unavailable at some point, should we still report
	//   it as Creating?
	if ready != len(refs) {
		cr.SetConditions(runtimev1alpha1.Creating())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(runtimev1alpha1.Available())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}
