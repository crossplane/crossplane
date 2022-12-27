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

// Package composite implements Crossplane composite resources.
package composite

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	env "github.com/crossplane/crossplane/internal/controller/apiextensions/composite/environment"
)

const (
	timeout             = 2 * time.Minute
	defaultPollInterval = 1 * time.Minute
	finalizer           = "composite.apiextensions.crossplane.io"
)

// Error strings
const (
	errGet               = "cannot get composite resource"
	errUpdate            = "cannot update composite resource"
	errUpdateStatus      = "cannot update composite resource status"
	errAddFinalizer      = "cannot add composite resource finalizer"
	errRemoveFinalizer   = "cannot remove composite resource finalizer"
	errSelectComp        = "cannot select Composition"
	errFetchComp         = "cannot fetch Composition"
	errConfigure         = "cannot configure composite resource"
	errPublish           = "cannot publish connection details"
	errUnpublish         = "cannot unpublish connection details"
	errRenderCD          = "cannot render composed resource"
	errRenderCR          = "cannot render composite resource"
	errValidate          = "refusing to use invalid Composition"
	errInline            = "cannot inline Composition patch sets"
	errAssociate         = "cannot associate composed resources with Composition resource templates"
	errFetchEnvironment  = "cannot fetch environment"
	errSelectEnvironment = "cannot select environment"

	errFmtPatchEnvironment = "cannot apply environment patch at index %d"
	errFmtRender           = "cannot render composed resource from resource template at index %d"
)

// Event reasons.
const (
	reasonResolve event.Reason = "SelectComposition"
	reasonCompose event.Reason = "ComposeResources"
	reasonPublish event.Reason = "PublishConnectionSecret"
	reasonInit    event.Reason = "InitializeCompositeResource"
	reasonDelete  event.Reason = "DeleteCompositeResource"
	reasonPaused  event.Reason = "ReconciliationPaused"
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

// A CompositionFetcher fetches an appropriate Composition for the supplied
// composite resource.
type CompositionFetcher interface {
	Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error)
}

// A CompositionFetcherFn fetches an appropriate Composition for the supplied
// composite resource.
type CompositionFetcherFn func(ctx context.Context, cr resource.Composite) (*v1.Composition, error)

// Fetch an appropriate Composition for the supplied Composite resource.
func (fn CompositionFetcherFn) Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error) {
	return fn(ctx, cr)
}

// EnvironmentSelector selects environment references for a composition environment.
type EnvironmentSelector interface {
	SelectEnvironment(ctx context.Context, cr resource.Composite, comp *v1.Composition) error
}

// A EnvironmentSelectorFn selects a composition reference.
type EnvironmentSelectorFn func(ctx context.Context, cr resource.Composite, comp *v1.Composition) error

// SelectEnvironment for the supplied composite resource.
func (fn EnvironmentSelectorFn) SelectEnvironment(ctx context.Context, cr resource.Composite, comp *v1.Composition) error {
	return fn(ctx, cr, comp)
}

// An EnvironmentFetcher fetches an appropriate environment for the supplied
// composite resource.
type EnvironmentFetcher interface {
	Fetch(ctx context.Context, cr resource.Composite) (*env.Environment, error)
}

// An EnvironmentFetcherFn fetches an appropriate environment for the supplied
// composite resource.
type EnvironmentFetcherFn func(ctx context.Context, cr resource.Composite) (*env.Environment, error)

// Fetch an appropriate environment for the supplied Composite resource.
func (fn EnvironmentFetcherFn) Fetch(ctx context.Context, cr resource.Composite) (*env.Environment, error) {
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
	Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error
}

// A RendererFn may be used to render a composed resource.
type RendererFn func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error

// Render the supplied composed resource using the supplied composite resource
// and template as inputs.
func (fn RendererFn) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error {
	return fn(ctx, cp, cd, t, env)
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

// WithPollInterval specifies how long the Reconciler should wait before queueing
// a new reconciliation after a successful reconcile. The Reconciler requeues
// after a specified duration when it is not actively waiting for an external
// operation, but wishes to check whether resources it does not have a watch on
// (i.e. composed resources) need to be reconciled.
func WithPollInterval(after time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.pollInterval = after
	}
}

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = ca
	}
}

// WithCompositionFetcher specifies how the composition to be used should be
// fetched.
func WithCompositionFetcher(f CompositionFetcher) ReconcilerOption {
	return func(r *Reconciler) {
		r.composition.CompositionFetcher = f
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

// WithCompositeFinalizer specifies how the composition to be used should be
// selected.
// WithCompositeFinalizer specifies which Finalizer should be used to finalize
// composites when they are deleted.
func WithCompositeFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Finalizer = f
	}
}

// WithCompositionSelector specifies how the composition to be used should be
// selected.
func WithCompositionSelector(s CompositionSelector) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CompositionSelector = s
	}
}

// WithEnvironmentSelector specifies how the environment to be used should be
// selected.
func WithEnvironmentSelector(s EnvironmentSelector) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.EnvironmentSelector = s
	}
}

// WithEnvironmentFetcher specifies how the environment to be used should be
// fetched.
func WithEnvironmentFetcher(f EnvironmentFetcher) ReconcilerOption {
	return func(r *Reconciler) {
		r.environment.EnvironmentFetcher = f
	}
}

// WithConfigurator specifies how the Reconciler should configure
// composite resources using their composition.
func WithConfigurator(c Configurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Configurator = c
	}
}

// WithConnectionPublishers specifies how the Reconciler should publish
// connection secrets.
func WithConnectionPublishers(p ...managed.ConnectionPublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ConnectionPublisher = managed.PublisherChain(p)
	}
}

// WithCompositeRenderer specifies how the Reconciler should render composite resources.
func WithCompositeRenderer(rd Renderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Renderer = rd
	}
}

// WithOptions lets the Reconciler know which options to pass to new composite
// resource claim controllers.
func WithOptions(o controller.Options) ReconcilerOption {
	return func(r *Reconciler) {
		r.options = o
	}
}

type composition struct {
	CompositionFetcher
	CompositionValidator
	CompositionTemplateAssociator
}

type environment struct {
	EnvironmentFetcher
}

type compositeResource struct {
	resource.Finalizer
	CompositionSelector
	EnvironmentSelector
	Configurator
	Renderer
	managed.ConnectionPublisher
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
		client:       resource.ClientApplicator{Client: kube, Applicator: resource.NewAPIPatchingApplicator(kube)},
		newComposite: nc,

		composition: composition{
			CompositionFetcher: NewAPICompositionFetcher(kube),
			CompositionValidator: ValidationChain{
				CompositionValidatorFn(RejectMixedTemplates),
				CompositionValidatorFn(RejectDuplicateNames),
			},
			CompositionTemplateAssociator: NewGarbageCollectingAssociator(kube),
		},

		environment: environment{
			EnvironmentFetcher: env.NewNilEnvironmentFetcher(),
		},

		composite: compositeResource{
			Finalizer:           resource.NewAPIFinalizer(kube, finalizer),
			CompositionSelector: NewAPILabelSelectorResolver(kube),
			EnvironmentSelector: env.NewNoopEnvironmentSelector(),
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

		pollInterval: defaultPollInterval,
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

	environment environment

	composition composition
	composite   compositeResource
	composed    composedResource

	log    logging.Logger
	record event.Recorder

	pollInterval time.Duration

	options controller.Options
}

// composedRenderState is a wrapper around a composed resource that tracks whether
// it was successfully rendered or not, together with a list of patches defined
// on its template that have been applied (not filtered out).
type composedRenderState struct {
	resource       resource.Composed
	rendered       bool
	appliedPatches []v1.Patch
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Reconcile methods are often very complex. Be wary.
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

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(cr) {
		log.Debug("Reconciliation is paused via the pause annotation", "annotation", meta.AnnotationKeyReconciliationPaused, "value", "true")
		r.record.Event(cr, event.Normal(reasonPaused, "Reconciliation is paused via the pause annotation"))
		cr.SetConditions(xpv1.ReconcilePaused())
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if meta.WasDeleted(cr) {
		log = log.WithValues("deletion-timestamp", cr.GetDeletionTimestamp())

		cr.SetConditions(xpv1.Deleting())
		if err := r.composite.UnpublishConnection(ctx, cr, nil); err != nil {
			log.Debug(errUnpublish, "error", err)
			err = errors.Wrap(err, errUnpublish)
			r.record.Event(cr, event.Warning(reasonDelete, err))
			cr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		if err := r.composite.RemoveFinalizer(ctx, cr); err != nil {
			log.Debug(errRemoveFinalizer, "error", err)
			err = errors.Wrap(err, errRemoveFinalizer)
			r.record.Event(cr, event.Warning(reasonDelete, err))
			cr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		log.Debug("Successfully deleted composite resource")
		cr.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.composite.AddFinalizer(ctx, cr); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(cr, event.Warning(reasonInit, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.composite.SelectComposition(ctx, cr); err != nil {
		log.Debug(errSelectComp, "error", err)
		err = errors.Wrap(err, errSelectComp)
		r.record.Event(cr, event.Warning(reasonResolve, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}
	r.record.Event(cr, event.Normal(reasonResolve, "Successfully selected composition"))
	// Note that this 'Composition' will be derived from a
	// CompositionRevision if the relevant feature flag is enabled.
	comp, err := r.composition.Fetch(ctx, cr)
	if err != nil {
		log.Debug(errFetchComp, "error", err)
		err = errors.Wrap(err, errFetchComp)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	// Prepare the environment.
	// Note that environments are optional, so env can be nil.
	if err := r.composite.SelectEnvironment(ctx, cr, comp); err != nil {
		log.Debug(errSelectEnvironment, "error", err)
		err = errors.Wrap(err, errSelectEnvironment)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{}, err
	}

	env, err := r.environment.Fetch(ctx, cr)
	if err != nil {
		log.Debug(errFetchEnvironment, "error", err)
		err = errors.Wrap(err, errFetchEnvironment)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{}, err
	}

	// TODO(negz): Composition validation should be handled by a validation
	// webhook, not by this controller.
	if err := r.composition.Validate(comp); err != nil {
		log.Debug(errValidate, "error", err)
		err = errors.Wrap(err, errValidate)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.composite.Configure(ctx, cr, comp); err != nil {
		log.Debug(errConfigure, "error", err)
		err = errors.Wrap(err, errConfigure)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	// Inline PatchSets from Composition Spec before composing resources.
	ct, err := ComposedTemplates(comp.Spec)
	if err != nil {
		log.Debug(errInline, "error", err)
		err = errors.Wrap(err, errInline)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	tas, err := r.composition.AssociateTemplates(ctx, cr, ct)
	if err != nil {
		log.Debug(errAssociate, "error", err)
		err = errors.Wrap(err, errAssociate)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	// If we have an environment, run all environment patches before composing
	// resources.
	if env != nil && comp.Spec.Environment != nil {
		for i, p := range comp.Spec.Environment.Patches {
			if err := ApplyEnvironmentPatch(p, cr, env); err != nil {
				return reconcile.Result{}, errors.Wrapf(err, errFmtPatchEnvironment, i)
			}
		}
	}

	// We optimistically render all composed resources that we are able to
	// with the expectation that any that we fail to render will
	// subsequently have their error corrected by manual intervention or
	// propagation of a required input.
	refs := make([]corev1.ObjectReference, len(tas))
	cds := make([]composedRenderState, len(tas))
	for i, ta := range tas {
		cd := composed.New(composed.FromReference(ta.Reference))
		rendered := true
		if err := r.composed.Render(ctx, cr, cd, ta.Template, env); err != nil {
			log.Debug(errRenderCD, "error", err, "index", i)
			r.record.Event(cr, event.Warning(reasonCompose, errors.Wrapf(err, errFmtRender, i)))
			rendered = false
		}

		cds[i] = composedRenderState{
			resource:       cd,
			rendered:       rendered,
			appliedPatches: filterPatches(ta.Template.Patches, patchTypesFromXR()...),
		}
		refs[i] = *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind())
	}

	// We persist references to our composed resources before we create
	// them. This way we can render composed resources with
	// non-deterministic names, and also potentially recover from any errors
	// we encounter while applying composed resources without leaking them.
	cr.SetResourceReferences(refs)
	if err := r.client.Update(ctx, cr); err != nil {
		log.Debug(errUpdate, "error", err)
		err = errors.Wrap(err, errUpdate)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	// We apply all of our composed resources before we observe them and
	// update the composite resource accordingly in the loop below. This
	// ensures that issues observing and processing one composed resource
	// won't block the application of another.
	for _, cd := range cds {
		// If we were unable to render the composed resource we should not try
		// and apply it.
		if !cd.rendered {
			continue
		}
		if err := r.client.Apply(ctx, cd.resource, append(mergeOptions(cd.appliedPatches), resource.MustBeControllableBy(cr.GetUID()))...); err != nil {
			log.Debug(errApply, "error", err)
			err = errors.Wrap(err, errApply)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			cr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
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

		if err := r.composite.Render(ctx, cr, cd.resource, tpl, env); err != nil {
			log.Debug(errRenderCR, "error", err)
			err = errors.Wrap(err, errRenderCR)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			cr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		c, err := r.composed.FetchConnectionDetails(ctx, cd.resource, tpl)
		if err != nil {
			log.Debug(errFetchSecret, "error", err)
			err = errors.Wrap(err, errFetchSecret)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			cr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		for key, val := range c {
			conn[key] = val
		}

		rdy, err := r.composed.IsReady(ctx, cd.resource, tpl)
		if err != nil {
			log.Debug(errReadiness, "error", err)
			err = errors.Wrap(err, errReadiness)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			cr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		if rdy {
			ready++
		}
	}

	// Call Apply so that we do not just replace fields on existing XR but
	// merge fields for which a merge configuration has been specified. For
	// fields for which a merge configuration does not exist, the behavior
	// will be a replace from updated. We pass a deepcopy because the Apply
	// method doesn't update status, but calling Apply resets any pending
	// status changes.
	updated := cr.DeepCopyObject().(client.Object)
	if err := r.client.Apply(ctx, updated, mergeOptions(filterToXRPatches(tas))...); err != nil {
		log.Debug(errUpdate, "error", err)
		err = errors.Wrap(err, errUpdate)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if updated.GetResourceVersion() != cr.GetResourceVersion() {
		// If our deepcopy's resource version changed we know that our
		// update was not a no-op. Our original object has a stale
		// resource version, so any attempt to update it will fail. The
		// safest thing for us to do here is to return early. The update
		// will have immediately enqueued a new reconcile because this
		// controller is watching for this kind of resource. The
		// remaining reconcile logic will proceed when no new spec
		// changes are persisted.
		log.Debug("Composite resource spec or metadata was patched - terminating reconcile early")
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	r.record.Event(cr, event.Normal(reasonCompose, "Successfully composed resources"))

	published, err := r.composite.PublishConnection(ctx, cr, conn)
	if err != nil {
		log.Debug(errPublish, "error", err)
		err = errors.Wrap(err, errPublish)
		r.record.Event(cr, event.Warning(reasonPublish, err))
		cr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}
	if published {
		cr.SetConnectionDetailsLastPublishedTime(&metav1.Time{Time: time.Now()})
		log.Debug("Successfully published connection details")
		r.record.Event(cr, event.Normal(reasonPublish, "Successfully published connection details"))
	}

	cr.SetConditions(xpv1.ReconcileSuccess())

	// TODO(muvaf):
	// * Report which resources are not ready.
	// * If a resource becomes Unavailable at some point, should we still report
	//   it as Creating?
	if ready != len(refs) {
		// We want to requeue to wait for our composed resources to
		// become ready, since we can't watch them.
		cr.SetConditions(xpv1.Creating())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	// We requeue after our poll interval because we can't watch composed
	// resources - we can't know what type of resources we might compose
	// when this controller is started.
	cr.SetConditions(xpv1.Available())
	return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}

// filterToXRPatches selects patches defined in composed templates,
// whose type is one of the XR-targeting patches
// (e.g. v1.PatchTypeToCompositeFieldPath or v1.PatchTypeCombineToComposite)
func filterToXRPatches(tas []TemplateAssociation) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(tas))
	for _, ta := range tas {
		filtered = append(filtered, filterPatches(ta.Template.Patches,
			patchTypesToXR()...)...)
	}
	return filtered
}

// filterPatches selects patches whose type belong to the list onlyTypes
func filterPatches(pas []v1.Patch, onlyTypes ...v1.PatchType) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(pas))
	for _, p := range pas {
		for _, t := range onlyTypes {
			if t == p.Type {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered
}
