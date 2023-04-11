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
	"fmt"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
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
	errGet                    = "cannot get composite resource"
	errUpdate                 = "cannot update composite resource"
	errUpdateStatus           = "cannot update composite resource status"
	errAddFinalizer           = "cannot add composite resource finalizer"
	errRemoveFinalizer        = "cannot remove composite resource finalizer"
	errSelectComp             = "cannot select Composition"
	errSelectCompUpdatePolicy = "cannot select CompositionUpdatePolicy"
	errFetchComp              = "cannot fetch Composition"
	errConfigure              = "cannot configure composite resource"
	errPublish                = "cannot publish connection details"
	errUnpublish              = "cannot unpublish connection details"
	errValidate               = "refusing to use invalid Composition"
	errAssociate              = "cannot associate composed resources with Composition resource templates"
	errFetchEnvironment       = "cannot fetch environment"
	errSelectEnvironment      = "cannot select environment"
	errCompose                = "cannot compose resources"
	errRenderCD               = "cannot render composed resource"

	errFmtPatchEnvironment = "cannot apply environment patch at index %d"
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

// A CompositionRevisionFetcher fetches an appropriate Composition for the supplied
// composite resource.
type CompositionRevisionFetcher interface {
	Fetch(ctx context.Context, cr resource.Composite) (*v1.CompositionRevision, error)
}

// A CompositionRevisionFetcherFn fetches an appropriate CompositionRevision for
// the supplied composite resource.
type CompositionRevisionFetcherFn func(ctx context.Context, cr resource.Composite) (*v1.CompositionRevision, error)

// Fetch an appropriate Composition for the supplied Composite resource.
func (fn CompositionRevisionFetcherFn) Fetch(ctx context.Context, cr resource.Composite) (*v1.CompositionRevision, error) {
	return fn(ctx, cr)
}

// A CompositionUpdatePolicySelector selects a composition update policy.
type CompositionUpdatePolicySelector interface {
	SelectCompositionUpdatePolicy(ctx context.Context, cr resource.Composite) error
}

// A CompositionUpdatePolicySelectorFn selects a composition update policy.
type CompositionUpdatePolicySelectorFn func(ctx context.Context, cr resource.Composite) error

// SelectCompositionUpdatePolicy for the supplied composite resource.
func (fn CompositionUpdatePolicySelectorFn) SelectCompositionUpdatePolicy(ctx context.Context, cr resource.Composite) error {
	return fn(ctx, cr)
}

// EnvironmentSelector selects environment references for a composition environment.
type EnvironmentSelector interface {
	SelectEnvironment(ctx context.Context, cr resource.Composite, rev *v1.CompositionRevision) error
}

// A EnvironmentSelectorFn selects a composition reference.
type EnvironmentSelectorFn func(ctx context.Context, cr resource.Composite, rev *v1.CompositionRevision) error

// SelectEnvironment for the supplied composite resource.
func (fn EnvironmentSelectorFn) SelectEnvironment(ctx context.Context, cr resource.Composite, rev *v1.CompositionRevision) error {
	return fn(ctx, cr, rev)
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
	Configure(ctx context.Context, cr resource.Composite, rev *v1.CompositionRevision) error
}

// A ConfiguratorFn configures a composite resource using its composition.
type ConfiguratorFn func(ctx context.Context, cr resource.Composite, rev *v1.CompositionRevision) error

// Configure the supplied composite resource using its composition.
func (fn ConfiguratorFn) Configure(ctx context.Context, cr resource.Composite, rev *v1.CompositionRevision) error {
	return fn(ctx, cr, rev)
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

// A CompositionRequest is a request to compose resources.
// It should be treated as immutable.
type CompositionRequest struct {
	Revision    *v1.CompositionRevision
	Environment *env.Environment
}

// A CompositionResult is the result of the composition process.
type CompositionResult struct {
	Composed          []ComposedResource
	ConnectionDetails managed.ConnectionDetails
	Events            []event.Event
}

// A Composer composes (i.e. creates, updates, or deletes) resources given the
// supplied composite resource and composition request.
type Composer interface {
	Compose(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error)
}

// A ComposerFn composes resources.
type ComposerFn func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error)

// Compose resources.
func (fn ComposerFn) Compose(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
	return fn(ctx, xr, req)
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

// WithClient specifies how the Reconciler should interact with the Kubernetes
// API.
func WithClient(c client.Client) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = c
	}
}

// WithCompositionRevisionFetcher specifies how the composition to be used should be
// fetched.
func WithCompositionRevisionFetcher(f CompositionRevisionFetcher) ReconcilerOption {
	return func(r *Reconciler) {
		r.revision.CompositionRevisionFetcher = f
	}
}

// WithCompositionRevisionValidator specifies how the Reconciler should validate
// CompositionRevisions.
func WithCompositionRevisionValidator(v CompositionRevisionValidator) ReconcilerOption {
	return func(r *Reconciler) {
		r.revision.CompositionRevisionValidator = v
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

// WithCompositionUpdatePolicySelector specifies how the composition update policy to be used should be
// selected.
func WithCompositionUpdatePolicySelector(s CompositionUpdatePolicySelector) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CompositionUpdatePolicySelector = s
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

// WithComposer specifies how the Reconciler should compose resources.
func WithComposer(c Composer) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource = c
	}
}

type revision struct {
	CompositionRevisionFetcher
	CompositionRevisionValidator
}

// A CompositionRevisionValidator validates the supplied CompositionRevision.
type CompositionRevisionValidator interface {
	Validate(*v1.CompositionRevision) error
}

// A CompositionRevisionValidatorFn is a function that validates a
// CompositionRevision.
type CompositionRevisionValidatorFn func(*v1.CompositionRevision) error

// Validate the supplied CompositionRevision.
func (fn CompositionRevisionValidatorFn) Validate(c *v1.CompositionRevision) error {
	return fn(c)
}

type environment struct {
	EnvironmentFetcher
}

type compositeResource struct {
	resource.Finalizer
	CompositionSelector
	CompositionUpdatePolicySelector
	EnvironmentSelector
	Configurator
	managed.ConnectionPublisher
}

// NewReconciler returns a new Reconciler of composite resources.
func NewReconciler(mgr manager.Manager, of resource.CompositeKind, opts ...ReconcilerOption) *Reconciler {
	nc := func() resource.Composite {
		return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(of)))
	}
	kube := unstructured.NewClient(mgr.GetClient())

	r := &Reconciler{
		client:       kube,
		newComposite: nc,

		revision: revision{
			CompositionRevisionFetcher: NewAPIRevisionFetcher(resource.ClientApplicator{Client: kube, Applicator: resource.NewAPIPatchingApplicator(kube)}),
			CompositionRevisionValidator: CompositionRevisionValidatorFn(func(rev *v1.CompositionRevision) error {
				// TODO(negz): Presumably this validation will eventually be
				// removed in favor of the new Composition validation
				// webhook.
				// This is the last remaining use of conv.FromRevisionSpec -
				// we can stop generating that once this is removed.
				conv := &v1.GeneratedRevisionSpecConverter{}
				comp := &v1.Composition{Spec: conv.FromRevisionSpec(rev.Spec)}
				_, errs := comp.Validate()
				return errs.ToAggregate()
			}),
		},

		environment: environment{
			EnvironmentFetcher: env.NewNilEnvironmentFetcher(),
		},

		composite: compositeResource{
			Finalizer:           resource.NewAPIFinalizer(kube, finalizer),
			CompositionSelector: NewAPILabelSelectorResolver(kube),
			EnvironmentSelector: env.NewNoopEnvironmentSelector(),
			Configurator:        NewConfiguratorChain(NewAPINamingConfigurator(kube), NewAPIConfigurator(kube)),

			// TODO(negz): In practice this is a filtered publisher that will
			// never filter any keys. Is there an unfiltered variant we could
			// use by default instead?
			ConnectionPublisher: NewAPIFilteredSecretPublisher(kube, []string{}),
		},

		resource: NewPTComposer(kube),

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
	client       client.Client
	newComposite func() resource.Composite

	environment environment

	revision  revision
	composite compositeResource

	resource Composer

	log    logging.Logger
	record event.Recorder

	pollInterval time.Duration
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Reconcile methods are often very complex. Be wary.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	xr := r.newComposite()
	if err := r.client.Get(ctx, req.NamespacedName, xr); err != nil {
		log.Debug(errGet, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGet)
	}

	log = log.WithValues(
		"uid", xr.GetUID(),
		"version", xr.GetResourceVersion(),
		"name", xr.GetName(),
	)

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(xr) {
		log.Debug("Reconciliation is paused via the pause annotation", "annotation", meta.AnnotationKeyReconciliationPaused, "value", "true")
		r.record.Event(xr, event.Normal(reasonPaused, "Reconciliation is paused via the pause annotation"))
		xr.SetConditions(xpv1.ReconcilePaused())
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if meta.WasDeleted(xr) {
		log = log.WithValues("deletion-timestamp", xr.GetDeletionTimestamp())

		xr.SetConditions(xpv1.Deleting())
		if err := r.composite.UnpublishConnection(ctx, xr, nil); err != nil {
			log.Debug(errUnpublish, "error", err)
			err = errors.Wrap(err, errUnpublish)
			r.record.Event(xr, event.Warning(reasonDelete, err))
			xr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
		}

		if err := r.composite.RemoveFinalizer(ctx, xr); err != nil {
			log.Debug(errRemoveFinalizer, "error", err)
			err = errors.Wrap(err, errRemoveFinalizer)
			r.record.Event(xr, event.Warning(reasonDelete, err))
			xr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
		}

		log.Debug("Successfully deleted composite resource")
		xr.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if err := r.composite.AddFinalizer(ctx, xr); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(xr, event.Warning(reasonInit, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if err := r.composite.SelectCompositionUpdatePolicy(ctx, xr); err != nil {
		log.Debug(errSelectCompUpdatePolicy, "error", err)
		err = errors.Wrap(err, errSelectCompUpdatePolicy)
		r.record.Event(xr, event.Warning(reasonResolve, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if err := r.composite.SelectComposition(ctx, xr); err != nil {
		log.Debug(errSelectComp, "error", err)
		err = errors.Wrap(err, errSelectComp)
		r.record.Event(xr, event.Warning(reasonResolve, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}
	r.record.Event(xr, event.Normal(reasonResolve, "Successfully selected composition"))

	// Note that this 'Composition' will be derived from a
	// CompositionRevision if the relevant feature flag is enabled.
	rev, err := r.revision.Fetch(ctx, xr)
	if err != nil {
		log.Debug(errFetchComp, "error", err)
		err = errors.Wrap(err, errFetchComp)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	// TODO(negz): Update this to validate the revision? In practice that's what
	// it's doing today when revis are enabled.
	if err := r.revision.Validate(rev); err != nil {
		log.Debug(errValidate, "error", err)
		err = errors.Wrap(err, errValidate)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if err := r.composite.Configure(ctx, xr, rev); err != nil {
		log.Debug(errConfigure, "error", err)
		err = errors.Wrap(err, errConfigure)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	// Prepare the environment.
	// Note that environments are optional, so env can be nil.
	if err := r.composite.SelectEnvironment(ctx, xr, rev); err != nil {
		log.Debug(errSelectEnvironment, "error", err)
		err = errors.Wrap(err, errSelectEnvironment)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		return reconcile.Result{}, err
	}

	env, err := r.environment.Fetch(ctx, xr)
	if err != nil {
		log.Debug(errFetchEnvironment, "error", err)
		err = errors.Wrap(err, errFetchEnvironment)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		return reconcile.Result{}, err
	}

	// TODO(negz): Pass this method a copy of xr, to make very clear that
	// anything it does won't be reflected in the state of xr?
	res, err := r.resource.Compose(ctx, xr, CompositionRequest{Revision: rev, Environment: env})
	if err != nil {
		log.Debug(errCompose, "error", err)
		err = errors.Wrap(err, errCompose)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	published, err := r.composite.PublishConnection(ctx, xr, res.ConnectionDetails)
	if err != nil {
		log.Debug(errPublish, "error", err)
		err = errors.Wrap(err, errPublish)
		r.record.Event(xr, event.Warning(reasonPublish, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}
	if published {
		xr.SetConnectionDetailsLastPublishedTime(&metav1.Time{Time: time.Now()})
		log.Debug("Successfully published connection details")
		r.record.Event(xr, event.Normal(reasonPublish, "Successfully published connection details"))
	}

	warnings := 0
	for _, e := range res.Events {
		if e.Type == event.TypeWarning {
			warnings++
		}
		log.Debug(e.Message)
		r.record.Event(xr, e)
	}

	if warnings == 0 {
		// We don't consider warnings severe enough to prevent the XR from being
		// considered synced (i.e. severe enough to return a ReconcileError) but
		// they are severe enough that we probably shouldn't say we successfully
		// composed resources.
		r.record.Event(xr, event.Normal(reasonCompose, "Successfully composed resources"))
	}

	ready := 0
	for i, cd := range res.Composed {
		// Specifying a name for P&T templates is optional but encouraged.
		// If there was no name, fall back to using the index.
		id := cd.ResourceName
		if id == "" {
			id = strconv.Itoa(i)
		}

		if !cd.Ready {
			log.Debug("Composed resource is not yet ready", "id", id)
			r.record.Event(xr, event.Normal(reasonCompose, fmt.Sprintf("Composed resource %q is not yet ready", id)))
			continue
		}

		ready++
	}

	xr.SetConditions(xpv1.ReconcileSuccess())

	// TODO(muvaf): If a resource becomes Unavailable at some point, should we
	// still report it as Creating?
	if ready != len(res.Composed) {
		// We want to requeue to wait for our composed resources to
		// become ready, since we can't watch them.
		xr.SetConditions(xpv1.Creating())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	// We requeue after our poll interval because we can't watch composed
	// resources - we can't know what type of resources we might compose
	// when this controller is started.
	xr.SetConditions(xpv1.Available())
	return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
}
