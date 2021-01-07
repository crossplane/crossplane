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
	errValidate     = "refusing to use invalid Composition"
	errInline       = "cannot inline Composition patch sets"
	errCompose      = "cannot compose resources"
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

// A CompositionResult is the result of composing several composed resources
// into a composite resource.
type CompositionResult struct {
	// The connection details of all composed resources.
	ConnectionDetails managed.ConnectionDetails

	// The desired number of composed resources.
	DesiredResources int

	// The number of composed resources that are ready.
	ReadyResources int
}

// A Composer composes (i.e. CRUDs) resources per the supplied composite
// resource and composition.
type Composer interface {
	Compose(ctx context.Context, cr resource.Composite, comp *v1.Composition) (CompositionResult, error)
}

// A ComposerFn may compose resources per the supplied composite resource and
// composition.
type ComposerFn func(ctx context.Context, cr resource.Composite, comp *v1.Composition) (CompositionResult, error)

// Compose resources from the supplied composite resource and Composition.
func (fn ComposerFn) Compose(ctx context.Context, cr resource.Composite, comp *v1.Composition) (CompositionResult, error) {
	return fn(ctx, cr, comp)
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
		r.composition = v
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

// WithComposer specifies how the Reconciler should compose resources.
func WithComposer(c Composer) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Composer = c
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
	Composer
	ConnectionPublisher
	Renderer
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

		composition: ValidationChain{
			CompositionValidatorFn(RejectMixedTemplates),
			CompositionValidatorFn(RejectDuplicateNames),
		},

		composite: compositeResource{
			Composer:            NewNamedComposer(kube),
			CompositionSelector: NewAPILabelSelectorResolver(kube),
			Configurator:        NewConfiguratorChain(NewAPINamingConfigurator(kube), NewAPIConfigurator(kube)),
			ConnectionPublisher: NewAPIFilteredSecretPublisher(kube, []string{}),
			Renderer:            RendererFn(RenderComposite),
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
	client       client.Client
	newComposite func() resource.Composite

	composition CompositionValidator
	composite   compositeResource

	log    logging.Logger
	record event.Recorder
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

	if err := r.composite.Configure(ctx, cr, comp); err != nil {
		log.Debug(errConfigure, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	composed, err := r.composite.Compose(ctx, cr, comp)
	if err != nil {
		log.Debug(errCompose, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		return reconcile.Result{RequeueAfter: shortWait}, nil
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

	published, err := r.composite.PublishConnection(ctx, cr, composed.ConnectionDetails)
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
	if composed.ReadyResources < composed.DesiredResources {
		cr.SetConditions(xpv1.Creating())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(xpv1.Available())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}
