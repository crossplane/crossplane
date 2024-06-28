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
	"math/rand"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/engine"
)

const (
	timeout             = 2 * time.Minute
	defaultPollInterval = 1 * time.Minute
	finalizer           = "composite.apiextensions.crossplane.io"
)

// Error strings.
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
	errInvalidResources       = "some resources were invalid, check events"
	errRenderCD               = "cannot render composed resource"
	errSyncResources          = "cannot sync composed resources"
	errGetClaim               = "cannot get referenced claim"
	errParseClaimRef          = "cannot parse claim reference"

	reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"
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

// Condition reasons.
const (
	reasonFatalError xpv1.ConditionReason = "FatalError"
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

// EnvironmentFetcherRequest describes the payload for an
// EnvironmentFetcher.
type EnvironmentFetcherRequest struct {
	Composite resource.Composite
	Revision  *v1.CompositionRevision
	Required  bool
}

// An EnvironmentFetcher fetches an appropriate environment for the supplied
// composite resource.
type EnvironmentFetcher interface {
	Fetch(ctx context.Context, req EnvironmentFetcherRequest) (*Environment, error)
}

// An EnvironmentFetcherFn fetches an appropriate environment for the supplied
// composite resource.
type EnvironmentFetcherFn func(ctx context.Context, req EnvironmentFetcherRequest) (*Environment, error)

// Fetch an appropriate environment for the supplied Composite resource.
func (fn EnvironmentFetcherFn) Fetch(ctx context.Context, req EnvironmentFetcherRequest) (*Environment, error) {
	return fn(ctx, req)
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

// A CompositionRequest is a request to compose resources.
// It should be treated as immutable.
type CompositionRequest struct {
	Revision    *v1.CompositionRevision
	Environment *Environment
}

// A CompositionResult is the result of the composition process.
type CompositionResult struct {
	Composed          []ComposedResource
	ConnectionDetails managed.ConnectionDetails
	Events            []TargetedEvent
	Conditions        []TargetedCondition
}

// A CompositionTarget is the target of a composition event or condition.
type CompositionTarget string

// Composition event and condition targets.
const (
	CompositionTargetComposite         CompositionTarget = "Composite"
	CompositionTargetCompositeAndClaim CompositionTarget = "CompositeAndClaim"
)

// A TargetedEvent represents an event produced by the composition process. It
// can target either the XR only, or both the XR and the claim.
type TargetedEvent struct {
	event.Event
	Target CompositionTarget
	// Detail about the event to be included in the composite resource event but
	// not the claim.
	Detail string
}

// AsEvent produces the base event.
func (e *TargetedEvent) AsEvent() event.Event {
	return event.Event{Type: e.Type, Reason: e.Reason, Message: e.Message, Annotations: e.Annotations}
}

// AsDetailedEvent produces an event with additional detail if available.
func (e *TargetedEvent) AsDetailedEvent() event.Event {
	if e.Detail == "" {
		return e.AsEvent()
	}
	msg := fmt.Sprintf("%s: %s", e.Detail, e.Message)
	return event.Event{Type: e.Type, Reason: e.Reason, Message: msg, Annotations: e.Annotations}
}

// A TargetedCondition represents a condition produced by the composition
// process. It can target either the XR only, or both the XR and the claim.
type TargetedCondition struct {
	xpv1.Condition
	Target CompositionTarget
}

// A Composer composes (i.e. creates, updates, or deletes) resources given the
// supplied composite resource and composition request.
type Composer interface {
	Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error)
}

// A ComposerFn composes resources.
type ComposerFn func(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error)

// Compose resources.
func (fn ComposerFn) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error) {
	return fn(ctx, xr, req)
}

// A ComposerSelectorFn selects the appropriate Composer for a mode.
type ComposerSelectorFn func(*v1.CompositionMode) Composer

// Compose calls the Composer returned by calling fn.
func (fn ComposerSelectorFn) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error) {
	return fn(req.Revision.Spec.Mode).Compose(ctx, xr, req)
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

// A PollIntervalHook determines how frequently the XR should poll its composed
// resources.
type PollIntervalHook func(ctx context.Context, xr *composite.Unstructured) time.Duration

// WithPollIntervalHook specifies how to determine how long the Reconciler
// should wait before queueing a new reconciliation after a successful
// reconcile.
func WithPollIntervalHook(h PollIntervalHook) ReconcilerOption {
	return func(r *Reconciler) {
		r.pollInterval = h
	}
}

// WithPollInterval specifies how long the Reconciler should wait before
// queueing a new reconciliation after a successful reconcile. The Reconciler
// uses the interval jittered +/- 10% when all composed resources are ready. It
// polls twice as frequently (i.e. at half the supplied interval) +/- 10% when
// waiting for composed resources to become ready.
func WithPollInterval(interval time.Duration) ReconcilerOption {
	return WithPollIntervalHook(func(_ context.Context, _ *composite.Unstructured) time.Duration {
		// Jitter the poll interval +/- 10%.
		return interval + time.Duration((rand.Float64()-0.5)*2*(float64(interval)*0.1)) //nolint:gosec // No need for secure randomness
	})
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

// WithWatchStarter specifies how the Reconciler should start watches for any
// resources it composes.
func WithWatchStarter(controllerName string, h handler.EventHandler, w WatchStarter) ReconcilerOption {
	return func(r *Reconciler) {
		r.controllerName = controllerName
		r.watchHandler = h
		r.engine = w
	}
}

type revision struct {
	CompositionRevisionFetcher
	CompositionRevisionValidator
}

// A CompositionRevisionValidator validates the supplied CompositionRevision.
type CompositionRevisionValidator interface {
	Validate(rev *v1.CompositionRevision) error
}

// A CompositionRevisionValidatorFn is a function that validates a
// CompositionRevision.
type CompositionRevisionValidatorFn func(*v1.CompositionRevision) error

// Validate the supplied CompositionRevision.
func (fn CompositionRevisionValidatorFn) Validate(c *v1.CompositionRevision) error {
	return fn(c)
}

// A WatchStarter can start a new watch. XR controllers use this to dynamically
// start watches when they compose new kinds of resources.
type WatchStarter interface {
	// StartWatches starts the supplied watches, if they're not running already.
	StartWatches(name string, ws ...engine.Watch) error
}

// A NopWatchStarter does nothing.
type NopWatchStarter struct{}

// StartWatches does nothing.
func (n *NopWatchStarter) StartWatches(_ string, _ ...engine.Watch) error { return nil }

// A WatchStarterFn is a function that can start a new watch.
type WatchStarterFn func(name string, ws ...engine.Watch) error

// StartWatches starts the supplied watches, if they're not running already.
func (fn WatchStarterFn) StartWatches(name string, ws ...engine.Watch) error {
	return fn(name, ws...)
}

type environment struct {
	EnvironmentFetcher
}

type compositeResource struct {
	resource.Finalizer
	CompositionSelector
	EnvironmentSelector
	Configurator
	managed.ConnectionPublisher
}

// NewReconciler returns a new Reconciler of composite resources.
func NewReconciler(c client.Client, of resource.CompositeKind, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: c,

		gvk: schema.GroupVersionKind(of),

		revision: revision{
			CompositionRevisionFetcher: NewAPIRevisionFetcher(resource.ClientApplicator{Client: c, Applicator: resource.NewAPIPatchingApplicator(c)}),
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
			EnvironmentFetcher: NewNilEnvironmentFetcher(),
		},

		composite: compositeResource{
			Finalizer:           resource.NewAPIFinalizer(c, finalizer),
			CompositionSelector: NewAPILabelSelectorResolver(c),
			EnvironmentSelector: NewNoopEnvironmentSelector(),
			Configurator:        NewConfiguratorChain(NewAPINamingConfigurator(c), NewAPIConfigurator(c)),

			// TODO(negz): In practice this is a filtered publisher that will
			// never filter any keys. Is there an unfiltered variant we could
			// use by default instead?
			ConnectionPublisher: NewAPIFilteredSecretPublisher(c, []string{}),
		},

		resource: NewPTComposer(c),

		// Dynamic watches are disabled by default.
		engine: &NopWatchStarter{},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),

		pollInterval: func(_ context.Context, _ *composite.Unstructured) time.Duration { return defaultPollInterval },
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// A Reconciler reconciles composite resources.
type Reconciler struct {
	client client.Client

	gvk schema.GroupVersionKind

	environment environment

	revision  revision
	composite compositeResource

	resource Composer

	// Used to dynamically start composed resource watches.
	controllerName string
	engine         WatchStarter
	watchHandler   handler.EventHandler

	log    logging.Logger
	record event.Recorder

	pollInterval PollIntervalHook
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcile methods are often very complex. Be wary.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	xr := composite.New(composite.WithGroupVersionKind(r.gvk))
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
		r.record.Event(xr, event.Normal(reasonPaused, "Reconciliation is paused via the pause annotation"))
		xr.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if meta.WasDeleted(xr) {
		log = log.WithValues("deletion-timestamp", xr.GetDeletionTimestamp())

		xr.SetConditions(xpv1.Deleting())
		if err := r.composite.UnpublishConnection(ctx, xr, nil); err != nil {
			err = errors.Wrap(err, errUnpublish)
			r.record.Event(xr, event.Warning(reasonDelete, err))
			xr.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
		}

		if err := r.composite.RemoveFinalizer(ctx, xr); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
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
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(xr, event.Warning(reasonInit, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	orig := xr.GetCompositionReference()
	if err := r.composite.SelectComposition(ctx, xr); err != nil {
		err = errors.Wrap(err, errSelectComp)
		r.record.Event(xr, event.Warning(reasonResolve, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}
	if compRef := xr.GetCompositionReference(); compRef != nil && (orig == nil || *compRef != *orig) {
		r.record.Event(xr, event.Normal(reasonResolve, fmt.Sprintf("Successfully selected composition: %s", compRef.Name)))
	}

	// Select (if there is a new one) and fetch the composition revision.
	origRev := xr.GetCompositionRevisionReference()
	rev, err := r.revision.Fetch(ctx, xr)
	if err != nil {
		log.Debug(errFetchComp, "error", err)
		err = errors.Wrap(err, errFetchComp)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}
	if rev := xr.GetCompositionRevisionReference(); rev != nil && (origRev == nil || *rev != *origRev) {
		r.record.Event(xr, event.Normal(reasonResolve, fmt.Sprintf("Selected composition revision: %s", rev.Name)))
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
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
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
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	env, err := r.environment.Fetch(ctx, EnvironmentFetcherRequest{
		Composite: xr,
		Revision:  rev,
		Required:  rev.Spec.Environment.IsRequired(),
	})
	if err != nil {
		log.Debug(errFetchEnvironment, "error", err)
		err = errors.Wrap(err, errFetchEnvironment)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		xr.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	res, err := r.resource.Compose(ctx, xr, CompositionRequest{Revision: rev, Environment: env})
	if err != nil {
		log.Debug(errCompose, "error", err)
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errCompose)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		if kerrors.IsInvalid(err) {
			// API Server's invalid errors may be unstable due to pointers in
			// the string representation of invalid structs (%v), among other
			// reasons. Setting these errors in conditions could cause the
			// resource version to increment continuously, leading to endless
			// reconciliation of the resource. To avoid this, we only log these
			// errors and emit an event. The conditions' message will then just
			// point to the event.
			err = errors.Wrap(errors.New(errInvalidResources), errCompose)
		}
		xr.SetConditions(xpv1.ReconcileError(err))

		meta := r.handleCommonCompositionResult(ctx, res, xr)
		// We encountered a fatal error. For any custom status conditions that were
		// not received due to the fatal error, mark them as unknown.
		for _, c := range xr.GetConditions() {
			if xpv1.IsSystemConditionType(c.Type) {
				continue
			}
			if !meta.conditionTypesSeen[c.Type] {
				c.Status = corev1.ConditionUnknown
				c.Reason = reasonFatalError
				c.Message = "A fatal error occurred before the status of this condition could be determined."
				xr.SetConditions(c)
			}
		}

		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	ws := make([]engine.Watch, len(xr.GetResourceReferences()))
	for i, ref := range xr.GetResourceReferences() {
		ws[i] = engine.WatchFor(composed.New(composed.FromReference(ref)), engine.WatchTypeComposedResource, r.watchHandler)
	}

	// StartWatches is a no-op unless the realtime compositions feature flag is
	// enabled. When the flag is enabled, the ControllerEngine that starts this
	// controller also starts a garbage collector for its watches.
	if err := r.engine.StartWatches(r.controllerName, ws...); err != nil {
		// TODO(negz): If we stop polling this will be a more serious error.
		log.Debug("Cannot start watches for composed resources. Relying on polling to know when they change.", "controller-name", r.controllerName, "error", err)
	}

	published, err := r.composite.PublishConnection(ctx, xr, res.ConnectionDetails)
	if err != nil {
		log.Debug(errPublish, "error", err)
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
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

	meta := r.handleCommonCompositionResult(ctx, res, xr)

	if meta.numWarningEvents == 0 {
		// We don't consider warnings severe enough to prevent the XR from being
		// considered synced (i.e. severe enough to return a ReconcileError) but
		// they are severe enough that we probably shouldn't say we successfully
		// composed resources.
		r.record.Event(xr, event.Normal(reasonCompose, "Successfully composed resources"))
	}

	var unready []ComposedResource
	var unsynced []ComposedResource
	for i, cd := range res.Composed {
		// Specifying a name for P&T templates is optional but encouraged.
		// If there was no name, fall back to using the index.
		id := string(cd.ResourceName)
		if id == "" {
			id = strconv.Itoa(i)
		}

		if !cd.Synced {
			log.Debug("Composed resource is not yet valid", "id", id)
			unsynced = append(unsynced, cd)
			r.record.Event(xr, event.Normal(reasonCompose, fmt.Sprintf("Composed resource %q is not yet valid", id)))
		}

		if !cd.Ready {
			log.Debug("Composed resource is not yet ready", "id", id)
			unready = append(unready, cd)
			r.record.Event(xr, event.Normal(reasonCompose, fmt.Sprintf("Composed resource %q is not yet ready", id)))
		}
	}

	if updateXRConditions(xr, unsynced, unready) {
		// This requeue is subject to rate limiting. Requeues will exponentially
		// backoff from 1 to 30 seconds. See the 'definition' (XRD) reconciler
		// that sets up the ratelimiter.
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	// We requeue after our poll interval because we can't watch composed
	// resources - we can't know what type of resources we might compose
	// when this controller is started.
	return reconcile.Result{RequeueAfter: r.pollInterval(ctx, xr)}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
}

// updateXRConditions updates the conditions of the supplied composite resource
// based on the supplied composed resources. It returns true if the XR should be
// requeued immediately.
func updateXRConditions(xr *composite.Unstructured, unsynced, unready []ComposedResource) (requeueImmediately bool) {
	readyCond := xpv1.Available()
	syncedCond := xpv1.ReconcileSuccess()
	if len(unsynced) > 0 {
		// We want to requeue to wait for our composed resources to
		// become ready, since we can't watch them.
		syncedCond = xpv1.ReconcileError(errors.New(errSyncResources)).WithMessage(fmt.Sprintf("Invalid resources: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, getComposerResourcesNames(unsynced))))
		requeueImmediately = true
	}
	if len(unready) > 0 {
		// We want to requeue to wait for our composed resources to
		// become ready, since we can't watch them.
		readyCond = xpv1.Creating().WithMessage(fmt.Sprintf("Unready resources: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, getComposerResourcesNames(unready))))
		requeueImmediately = true
	}
	xr.SetConditions(syncedCond, readyCond)
	return requeueImmediately
}

func getComposerResourcesNames(cds []ComposedResource) []string {
	names := make([]string, len(cds))
	for i, cd := range cds {
		names[i] = string(cd.ResourceName)
	}
	return names
}

type compositionResultMeta struct {
	numWarningEvents   int
	conditionTypesSeen map[xpv1.ConditionType]bool
}

func (r *Reconciler) handleCommonCompositionResult(ctx context.Context, res CompositionResult, xr *composite.Unstructured) compositionResultMeta {
	log := r.log.WithValues(
		"uid", xr.GetUID(),
		"version", xr.GetResourceVersion(),
		"name", xr.GetName(),
	)

	cm, err := getClaimFromXR(ctx, r.client, xr)
	if err != nil {
		log.Debug(errGetClaim, "error", err)
	}

	numWarningEvents := 0
	for _, e := range res.Events {
		if e.Event.Type == event.TypeWarning {
			numWarningEvents++
		}

		detailedEvent := e.AsDetailedEvent()
		log.Debug(detailedEvent.Message)
		r.record.Event(xr, detailedEvent)

		if e.Target == CompositionTargetCompositeAndClaim && cm != nil {
			r.record.Event(cm, e.AsEvent())
		}
	}

	conditionTypesSeen := make(map[xpv1.ConditionType]bool)
	for _, c := range res.Conditions {
		if xpv1.IsSystemConditionType(c.Condition.Type) {
			// Do not let users update system conditions.
			continue
		}
		conditionTypesSeen[c.Condition.Type] = true
		xr.SetConditions(c.Condition)
		if c.Target == CompositionTargetCompositeAndClaim {
			// We can ignore the error as it only occurs if given a system condition.
			_ = xr.SetClaimConditionTypes(c.Condition.Type)
		}
	}

	return compositionResultMeta{
		numWarningEvents:   numWarningEvents,
		conditionTypesSeen: conditionTypesSeen,
	}
}

func getClaimFromXR(ctx context.Context, c client.Client, xr *composite.Unstructured) (*claim.Unstructured, error) {
	if xr.GetClaimReference() == nil {
		return nil, nil
	}

	gv, err := schema.ParseGroupVersion(xr.GetClaimReference().APIVersion)
	if err != nil {
		return nil, errors.Wrap(err, errParseClaimRef)
	}

	claimGVK := gv.WithKind(xr.GetClaimReference().Kind)
	cm := claim.New(claim.WithGroupVersionKind(claimGVK))
	claimNN := types.NamespacedName{Namespace: xr.GetClaimReference().Namespace, Name: xr.GetClaimReference().Name}
	if err := c.Get(ctx, claimNN, cm); err != nil {
		return nil, errors.Wrap(err, errGetClaim)
	}
	return cm, nil
}
