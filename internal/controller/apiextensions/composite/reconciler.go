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
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/engine"
	"github.com/crossplane/crossplane/internal/features"
	"github.com/crossplane/crossplane/internal/xresource"
	"github.com/crossplane/crossplane/internal/xresource/unstructured/claim"
	"github.com/crossplane/crossplane/internal/xresource/unstructured/composite"
)

const (
	timeout   = 2 * time.Minute
	finalizer = "composite.apiextensions.crossplane.io"
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
	errWatch                  = "cannot watch resource for changes"
	errAssociate              = "cannot associate composed resources with Composition resource templates"
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
	reasonWatch   event.Reason = "WatchComposedResources"
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
	SelectComposition(ctx context.Context, cr xresource.Composite) error
}

// A CompositionRevisionSelector selects a composition revision via selector.
type CompositionRevisionSelector interface {
	SelectCompositionRevision(ctx context.Context, cr xresource.Composite) error
}

// A CompositionRevisionSelectorFn selects a composition revsion by label.
type CompositionRevisionSelectorFn func(ctx context.Context, cr xresource.Composite) error

// SelectCompositionRevision for the supplied composite resource.
func (fn CompositionRevisionSelectorFn) SelectCompositionRevision(ctx context.Context, cr xresource.Composite) error {
	return fn(ctx, cr)
}

// A CompositionSelectorFn selects a composition reference.
type CompositionSelectorFn func(ctx context.Context, cr xresource.Composite) error

// SelectComposition for the supplied composite resource.
func (fn CompositionSelectorFn) SelectComposition(ctx context.Context, cr xresource.Composite) error {
	return fn(ctx, cr)
}

// A ConnectionPublisher publishes the supplied ConnectionDetails for the
// supplied resource.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied resource. Publishing
	// must be additive; i.e. if details (a, b, c) are published, subsequently
	// publishing details (b, c, d) should update (b, c) but not remove a.
	PublishConnection(ctx context.Context, so xresource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error)
}

// A ConnectionPublisherFn publishes the supplied ConnectionDetails for the
// supplied resource.
type ConnectionPublisherFn func(ctx context.Context, o xresource.ConnectionSecretOwner, c managed.ConnectionDetails) (bool, error)

// PublishConnection details for the supplied resource.
func (fn ConnectionPublisherFn) PublishConnection(ctx context.Context, o xresource.ConnectionSecretOwner, c managed.ConnectionDetails) (bool, error) {
	return fn(ctx, o, c)
}

// A PublisherChain chains multiple ManagedPublishers.
type PublisherChain []ConnectionPublisher

// PublishConnection calls each ConnectionPublisher.PublishConnection serially. It returns the first error it
// encounters, if any.
func (pc PublisherChain) PublishConnection(ctx context.Context, o xresource.ConnectionSecretOwner, c managed.ConnectionDetails) (bool, error) {
	published := false
	for _, p := range pc {
		pb, err := p.PublishConnection(ctx, o, c)
		if err != nil {
			return published, err
		}
		if pb {
			published = true
		}
	}
	return published, nil
}

// A CompositionRevisionFetcher fetches an appropriate Composition for the supplied
// composite resource.
type CompositionRevisionFetcher interface {
	Fetch(ctx context.Context, cr xresource.Composite) (*v1.CompositionRevision, error)
}

// A CompositionRevisionFetcherFn fetches an appropriate CompositionRevision for
// the supplied composite resource.
type CompositionRevisionFetcherFn func(ctx context.Context, cr xresource.Composite) (*v1.CompositionRevision, error)

// Fetch an appropriate Composition for the supplied Composite resource.
func (fn CompositionRevisionFetcherFn) Fetch(ctx context.Context, cr xresource.Composite) (*v1.CompositionRevision, error) {
	return fn(ctx, cr)
}

// A Configurator configures a composite resource using its composition.
type Configurator interface {
	Configure(ctx context.Context, cr xresource.Composite, rev *v1.CompositionRevision) error
}

// A ConfiguratorFn configures a composite resource using its composition.
type ConfiguratorFn func(ctx context.Context, cr xresource.Composite, rev *v1.CompositionRevision) error

// Configure the supplied composite resource using its composition.
func (fn ConfiguratorFn) Configure(ctx context.Context, cr xresource.Composite, rev *v1.CompositionRevision) error {
	return fn(ctx, cr, rev)
}

// A CompositionRequest is a request to compose resources.
// It should be treated as immutable.
type CompositionRequest struct {
	Revision *v1.CompositionRevision
}

// A CompositionResult is the result of the composition process.
type CompositionResult struct {
	// Composed resource details.
	Composed []ComposedResource

	// XR connection details.
	ConnectionDetails managed.ConnectionDetails

	// XR readiness. When nil readiness is derived from composed resources.
	Ready *bool

	// XR and claim events.
	Events []TargetedEvent

	// XR and claim conditions.
	Conditions []TargetedCondition

	// TTL for this composition result.
	TTL time.Duration
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

// WithFeatures specifies what feature flags the Reconciler should enable.
func WithFeatures(f *feature.Flags) ReconcilerOption {
	return func(r *Reconciler) {
		r.features = f
	}
}

// WithPollInterval specifies how long the Reconciler should wait before
// queueing a new reconciliation after a successful reconcile. The Reconciler
// uses the interval jittered +/- 10%.
func WithPollInterval(interval time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.pollInterval = interval
	}
}

// WithCompositionRevisionFetcher specifies how the composition to be used should be
// fetched.
func WithCompositionRevisionFetcher(f CompositionRevisionFetcher) ReconcilerOption {
	return func(r *Reconciler) {
		r.revision.CompositionRevisionFetcher = f
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

// WithCompositionRevisionSelector specifies how the composition revision to be used should be
// selected.
func WithCompositionRevisionSelector(s CompositionRevisionSelector) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.CompositionRevisionSelector = s
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
func WithConnectionPublishers(p ...ConnectionPublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.ConnectionPublisher = PublisherChain(p)
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

// WithCompositeSchema specifies whether the Reconciler should reconcile a
// modern or a legacy type of composite resource.
func WithCompositeSchema(s composite.Schema) ReconcilerOption {
	return func(r *Reconciler) {
		r.schema = s
	}
}

type revision struct {
	CompositionRevisionFetcher
}

// A WatchStarter can start a new watch. XR controllers use this to dynamically
// start watches when they compose new kinds of resources.
type WatchStarter interface {
	// StartWatches starts the supplied watches, if they're not running already.
	StartWatches(ctx context.Context, name string, ws ...engine.Watch) error
}

// A NopWatchStarter does nothing.
type NopWatchStarter struct{}

// StartWatches does nothing.
func (n *NopWatchStarter) StartWatches(_ context.Context, _ string, _ ...engine.Watch) error {
	return nil
}

// A WatchStarterFn is a function that can start a new watch.
type WatchStarterFn func(ctx context.Context, name string, ws ...engine.Watch) error

// StartWatches starts the supplied watches, if they're not running already.
func (fn WatchStarterFn) StartWatches(ctx context.Context, name string, ws ...engine.Watch) error {
	return fn(ctx, name, ws...)
}

type compositeResource struct {
	resource.Finalizer
	CompositionSelector
	CompositionRevisionSelector
	Configurator
	ConnectionPublisher
}

// NewReconciler returns a new Reconciler of composite resources.
func NewReconciler(cached client.Client, of schema.GroupVersionKind, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: cached,

		gvk: of,

		revision: revision{
			CompositionRevisionFetcher: NewAPIRevisionFetcher(resource.ClientApplicator{Client: cached, Applicator: resource.NewAPIPatchingApplicator(cached)}),
		},

		composite: compositeResource{
			Finalizer:           resource.NewAPIFinalizer(cached, finalizer),
			CompositionSelector: NewAPILabelSelectorResolver(cached),
			Configurator:        NewConfiguratorChain(NewAPINamingConfigurator(cached), NewAPIConfigurator(cached)),

			// TODO(negz): In practice this is a filtered publisher that will
			// never filter any keys. Is there an unfiltered variant we could
			// use by default instead?
			ConnectionPublisher: NewAPIFilteredSecretPublisher(cached, []string{}),
		},

		// We use a nop Composer by default. The real composed is passed in by
		// the definition controller when it sets up this XR controller.
		resource: ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
			return CompositionResult{}, nil
		}),

		// Dynamic watches are disabled by default.
		engine: &NopWatchStarter{},

		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// A Reconciler reconciles composite resources.
type Reconciler struct {
	client client.Client

	gvk    schema.GroupVersionKind
	schema composite.Schema

	features *feature.Flags

	revision  revision
	composite compositeResource

	resource Composer

	// Used to dynamically start composed resource watches.
	controllerName string
	engine         WatchStarter
	watchHandler   handler.EventHandler

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager

	pollInterval time.Duration
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcile methods are often very complex. Be wary.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	xr := composite.New(composite.WithGroupVersionKind(r.gvk), composite.WithSchema(r.schema))
	if err := r.client.Get(ctx, req.NamespacedName, xr); err != nil {
		log.Debug(errGet, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGet)
	}
	status := r.conditions.For(xr)

	log = log.WithValues(
		"uid", xr.GetUID(),
		"version", xr.GetResourceVersion(),
		"name", xr.GetName(),
	)

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(xr) {
		r.record.Event(xr, event.Normal(reasonPaused, "Reconciliation is paused via the pause annotation"))
		status.MarkConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if meta.WasDeleted(xr) {
		log = log.WithValues("deletion-timestamp", xr.GetDeletionTimestamp())

		status.MarkConditions(xpv1.Deleting())

		if err := r.composite.RemoveFinalizer(ctx, xr); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err = errors.Wrap(err, errRemoveFinalizer)
			r.record.Event(xr, event.Warning(reasonDelete, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
		}

		log.Debug("Successfully deleted composite resource")
		status.MarkConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if err := r.composite.AddFinalizer(ctx, xr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(xr, event.Warning(reasonInit, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	orig := xr.GetCompositionReference()
	if err := r.composite.SelectComposition(ctx, xr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errSelectComp)
		r.record.Event(xr, event.Warning(reasonResolve, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if compRef := xr.GetCompositionReference(); compRef != nil && (orig == nil || *compRef != *orig) {
		r.record.Event(xr, event.Normal(reasonResolve, fmt.Sprintf("Successfully selected composition: %s", compRef.Name)))
	}

	origCompRev := xr.GetCompositionRevisionReference()
	if err := r.composite.SelectCompositionRevision(ctx, xr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errSelectComp)
		r.record.Event(xr, event.Warning(reasonResolve, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	if compRevRef := xr.GetCompositionRevisionReference(); compRevRef != nil && (orig == nil || *compRevRef != *origCompRev) {
		r.record.Event(xr, event.Normal(reasonResolve, fmt.Sprintf("Successfully selected composition revision: %s", compRevRef.Name)))
	}

	// Select (if there is a new one) and fetch the composition revision.
	origRev := xr.GetCompositionRevisionReference()
	rev, err := r.revision.Fetch(ctx, xr)
	if err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		log.Debug(errFetchComp, "error", err)
		err = errors.Wrap(err, errFetchComp)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}
	if rev := xr.GetCompositionRevisionReference(); rev != nil && (origRev == nil || *rev != *origRev) {
		r.record.Event(xr, event.Normal(reasonResolve, fmt.Sprintf("Selected composition revision: %s", rev.Name)))
	}

	if err := r.composite.Configure(ctx, xr, rev); err != nil {
		log.Debug(errConfigure, "error", err)
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errConfigure)
		r.record.Event(xr, event.Warning(reasonCompose, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	res, err := r.resource.Compose(ctx, xr, CompositionRequest{Revision: rev})
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
		status.MarkConditions(xpv1.ReconcileError(err))

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
				status.MarkConditions(c)
			}
		}

		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	ws := make([]engine.Watch, len(xr.GetResourceReferences()))
	for i, ref := range xr.GetResourceReferences() {
		cr := &kunstructured.Unstructured{}
		cr.SetGroupVersionKind(ref.GroupVersionKind())
		ws[i] = engine.WatchFor(cr, engine.WatchTypeComposedResource, r.watchHandler)
	}

	// The ControllerEngine that starts this controller also starts a
	// garbage collector for its watches.
	if err := r.engine.StartWatches(ctx, r.controllerName, ws...); err != nil {
		err = errors.Wrap(err, errWatch)
		r.record.Event(xr, event.Warning(reasonWatch, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	published, err := r.composite.PublishConnection(ctx, xr, res.ConnectionDetails)
	if err != nil {
		log.Debug(errPublish, "error", err)
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errPublish)
		r.record.Event(xr, event.Warning(reasonPublish, err))
		status.MarkConditions(xpv1.ReconcileError(err))
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
		log.Debug("Successfully composed resources")
	}

	var unsynced []string
	var unready []string
	for i, cd := range res.Composed {
		// Specifying a name for P&T templates is optional but encouraged.
		// If there was no name, fall back to using the index.
		id := string(cd.ResourceName)
		if id == "" {
			id = strconv.Itoa(i)
		}

		if !cd.Synced {
			log.Debug("Composed resource is not yet valid", "id", id)
			unsynced = append(unsynced, id)
			r.record.Event(xr, event.Normal(reasonCompose, fmt.Sprintf("Composed resource %q is not yet valid", id)))
		}

		if !cd.Ready {
			log.Debug("Composed resource is not yet ready", "id", id)
			unready = append(unready, id)
			r.record.Event(xr, event.Normal(reasonCompose, fmt.Sprintf("Composed resource %q is not yet ready", id)))
		}
	}

	synced := xpv1.ReconcileSuccess()
	if len(unsynced) > 0 {
		synced = xpv1.ReconcileError(errors.New(errSyncResources)).WithMessage(fmt.Sprintf("Unsynced resources: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, unsynced)))
	}

	ready := xpv1.Available()
	if len(unready) > 0 {
		ready = xpv1.Creating().WithMessage(fmt.Sprintf("Unready resources: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, unready)))
	}

	// If the composer explicitly specified the XR's readiness it
	// supersedes readiness derived from composed resources.
	if res.Ready != nil {
		ready = xpv1.Creating()
		if *res.Ready {
			ready = xpv1.Available()
		}
	}

	status.MarkConditions(synced, ready)

	// Requeue after the configured poll interval by default. If realtime
	// compositions is enabled this'll be RequeueAfter: 0, i.e. no requeue.
	result := reconcile.Result{RequeueAfter: jitter(r.pollInterval)}

	switch {
	case !r.features.Enabled(features.EnableBetaRealtimeCompositions) && len(unsynced)+len(unready) > 0:
		// Realtime compositions isn't enabled, and one of our composed
		// resources is unsynced or unready. Requeue immediately
		// (subject to backoff) while we wait for them.
		result = reconcile.Result{Requeue: true}
	case res.TTL > 0:
		// The composer (e.g. the function pipeline) explicitly returned
		// a TTL for the composition result. Requeue after the TTL
		// expires.
		result = reconcile.Result{RequeueAfter: jitter(res.TTL)}
	}

	return result, errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
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

// Jitter the supplied duration by up to +/- 10%.
func jitter(d time.Duration) time.Duration {
	return d + time.Duration((rand.Float64()-0.5)*2*(float64(d)*0.1)) //nolint:gosec // No need for secure randomness
}
