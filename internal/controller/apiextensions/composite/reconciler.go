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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
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
	errRenderCD     = "cannot render composed resource"
	errRenderCR     = "cannot render composite resource"
	errValidate     = "refusing to use invalid Composition"
	errInline       = "cannot inline Composition patch sets"
	errAssociate    = "cannot associate composed resources with Composition resource templates"

	errFmtRender = "cannot render composed resource from resource template at index %d"
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
	Configure(ctx context.Context, cr resource.Composite, cp *v1.Composition) error
}

// A ConfiguratorFn configures a composite resource using its composition.
type ConfiguratorFn func(ctx context.Context, cr resource.Composite, cp *v1.Composition) error

// Configure the supplied composite resource using its composition.
func (fn ConfiguratorFn) Configure(ctx context.Context, cr resource.Composite, cp *v1.Composition) error {
	return fn(ctx, cr, cp)
}

// A Renderer is used to render a composed resource.
type Renderer interface {
	Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error
}

// A RendererFn may be used to render a composed resource.
type RendererFn func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error

// Render the supplied composed resource using the supplied composite resource
// and template as inputs.
func (fn RendererFn) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
	return fn(ctx, cp, cd, t)
}

// ConnectionDetailsFetcher fetches the connection details of the Composed resource.
type ConnectionDetailsFetcher interface {
	FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error)
}

// A ConnectionDetailsFetcherFn fetches the connection details of the supplied
// composed resource, if any.
type ConnectionDetailsFetcherFn func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error)

// FetchConnectionDetails calls the FetchConnectionDetailsFn.
func (f ConnectionDetailsFetcherFn) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
	return f(ctx, cd, t)
}

// A ReadinessChecker checks whether a composed resource is ready or not.
type ReadinessChecker interface {
	IsReady(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error)
}

// A ReadinessCheckerFn checks whether a composed resource is ready or not.
type ReadinessCheckerFn func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error)

// IsReady reports whether a composed resource is ready or not.
func (fn ReadinessCheckerFn) IsReady(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
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

// WithCompositionValidator specifies how the Reconciler should validate
// Compositions.
func WithCompositionValidator(v CompositionValidator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composition.CompositionValidator = v
	}
}

// WithCompositionTemplateAssociator specifies how the Reconciler should
// associate composition templates with composed resources.
func WithCompositionTemplateAssociator(a CompositionTemplateAssociator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composition.CompositionTemplateAssociator = a
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

// WithCompositeRenderer specifies how the Reconciler should render composite resources.
func WithCompositeRenderer(rd Renderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Renderer = rd
	}
}

type composition struct {
	CompositionValidator
	CompositionTemplateAssociator
}

type compositeResource struct {
	CompositionSelector
	Configurator
	ConnectionPublisher
	Renderer
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

		composition: composition{
			CompositionValidator: ValidationChain{
				CompositionValidatorFn(RejectMixedTemplates),
				CompositionValidatorFn(RejectDuplicateNames),
			},
			CompositionTemplateAssociator: NewGarbageCollectingAssociator(kube),
		},

		composite: compositeResource{
			CompositionSelector: NewAPILabelSelectorResolver(kube),
			Configurator:        NewConfiguratorChain(NewAPINamingConfigurator(kube), NewAPIConfigurator(kube)),
			ConnectionPublisher: NewAPIFilteredSecretPublisher(kube, []string{}),
			Renderer:            RendererFn(RenderComposite),
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

	composition composition
	composite   compositeResource
	composed    composedResource

	log    logging.Logger
	record event.Recorder
}

// composedRendered is a wrapper around a composed resource that tracks whether
// it was successfully rendered or not.
type composedRendered struct {
	resource resource.Composed
	rendered bool
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo
	// NOTE(negz): Like most Reconcile methods, this one is over our cyclomatic
	// complexity goal. Be wary when adding branches, and look for functionality
	// that could be reasonably moved into an injected dependency.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
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
	comp := &v1.Composition{}
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

	// TODO(negz): Composition validation should be handled by a validation
	// webhook, not by this controller.
	if err := r.composition.Validate(comp); err != nil {
		log.Debug(errValidate, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	// Inline PatchSets from Composition Spec before composing resources.
	if err := comp.Spec.InlinePatchSets(); err != nil {
		log.Debug(errInline, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	tas, err := r.composition.AssociateTemplates(ctx, cr, comp)
	if err != nil {
		log.Debug(errAssociate, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	// We optimistically render all composed resources that we are able to with
	// the expectation that any that we fail to render will subsequently have
	// their error corrected by manual intervention or propagation of a required
	// input.
	refs := make([]corev1.ObjectReference, len(tas))
	cds := make([]composedRendered, len(tas))
	for i, ta := range tas {
		cd := composed.New(composed.FromReference(ta.Reference))
		rendered := true
		if err := r.composed.Render(ctx, cr, cd, ta.Template); err != nil {
			log.Debug(errRenderCD, "error", err, "index", i)
			r.record.Event(cr, event.Warning(reasonCompose, errors.Wrapf(err, errFmtRender, i)))
			rendered = false
		}

		cds[i] = composedRendered{
			resource: cd,
			rendered: rendered,
		}
		refs[i] = *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind())
	}

	// We persist references to our composed resources before we create them.
	// This way we can render composed resources with non-deterministic names,
	// and also potentially recover from any errors we encounter while applying
	// composed resources without leaking them.
	cr.SetResourceReferences(refs)
	if err := r.client.Update(ctx, cr); err != nil {
		log.Debug(errUpdate, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	// We apply all of our composed resources before we observe them and update
	// the composite resource accordingly in the loop below. This ensures that
	// issues observing and processing one composed resource won't block the
	// application of another.
	for _, cd := range cds {
		// If we were unable to render the composed resource we should not try
		// and apply it.
		if !cd.rendered {
			continue
		}
		if err := r.client.Apply(ctx, cd.resource, resource.MustBeControllableBy(cr.GetUID())); err != nil {
			log.Debug(errApply, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}
	}

	conn := managed.ConnectionDetails{}
	ready := 0
	for i, tpl := range comp.Spec.Resources {
		cd := cds[i]

		// If we were unable to render the composed resource we should not try
		// and to observe it.
		if !cd.rendered {
			continue
		}

		if err := r.composite.Render(ctx, cr, cd.resource, tpl); err != nil {
			log.Debug(errRenderCR, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		c, err := r.composed.FetchConnectionDetails(ctx, cd.resource, tpl)
		if err != nil {
			log.Debug(errFetchSecret, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		for key, val := range c {
			conn[key] = val
		}

		rdy, err := r.composed.IsReady(ctx, cd.resource, tpl)
		if err != nil {
			log.Debug(errReadiness, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		if rdy {
			ready++
		}
	}

	// We pass a deepcopy because the update method doesn't update status,
	// but calling update resets any pending status changes.
	updated := cr.DeepCopyObject().(client.Object)
	if err := r.client.Update(ctx, updated); err != nil {
		log.Debug(errUpdate, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	if updated.GetResourceVersion() != cr.GetResourceVersion() {
		// If our deepcopy's resource version changed we know that our update
		// was not a no-op. Our original object has a stale resource
		// version, so any attempt to update it will fail. The safest thing for
		// us to do here is to return early. The update will have immediately enqueued a
		// new reconcile because this controller is watching for this kind of resource.
		// The remaining reconcile logic will proceed when no new spec changes are persisted.
		log.Debug("Composite resource spec or metadata was patched - terminating reconcile early")
		return reconcile.Result{}, nil
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
		cr.SetConditions(xpv1.Creating())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(xpv1.Available())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}
