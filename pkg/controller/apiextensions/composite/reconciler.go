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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	composedctrl "github.com/crossplane/crossplane/pkg/controller/apiextensions/composite/composed"
)

const (
	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
	timeout   = 2 * time.Minute
)

// Error strings
const (
	errGet          = "cannot get composite infrastructure resource"
	errUpdate       = "cannot update composite infrastructure resource"
	errUpdateStatus = "cannot update composite infrastructure resource status"
	errSelectComp   = "cannot select Composition"
	errGetComp      = "cannot get Composition"
	errConfigure    = "cannot configure composite infrastructure resource"
	errReconcile    = "cannot reconcile composed infrastructure resource"
	errPublish      = "cannot publish connection details"
)

// Event reasons.
const (
	reasonResolve event.Reason = "SelectComposition"
	reasonCompose event.Reason = "ComposeResources"
	reasonPublish event.Reason = "PublishConnectionSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite infrastructure resource.
func ControllerName(name string) string {
	return "composite/" + name
}

// ConnectionSecretFilterer returns a set of allowed keys.
type ConnectionSecretFilterer interface {
	GetConnectionSecretKeys() []string
}

// A ConnectionPublisher manages the supplied ConnectionDetails for the
// supplied resource. Publishers must handle the case in which
// the supplied ConnectionDetails are empty.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied resource. Publishing
	// must be additive; i.e. if details (a, b, c) are published, subsequently
	// publishing details (b, c, d) should update (b, c) but not remove a.
	PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error

	// UnpublishConnection details for the supplied resource.
	UnpublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error
}

// TODO(muvaf): Interface should not depend on composedctrl package but that's
// the easiest way for now to not have circular dependency.

// A Composer composes infrastructure resources.
type Composer interface {
	Compose(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) (composedctrl.Observation, error)
}

// SelectorResolver selects the composition reference with the information given
// as selector.
type SelectorResolver interface {
	ResolveSelector(ctx context.Context, cr resource.Composite) error
}

// A Configurator configures a composite resource using its
// composition.
type Configurator interface {
	Configure(ctx context.Context, cr resource.Composite, cp *v1alpha1.Composition) error
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

// WithSelectorResolver specifies how the Reconciler should publish
// connection secrets.
func WithSelectorResolver(p SelectorResolver) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.SelectorResolver = p
	}
}

// WithConfigurator specifies how the Reconciler should configure
// composite resources using their composition
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

// WithComposer specifies how the Reconciler should compose resources.
func WithComposer(rc Composer) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource = rc
	}
}

type compositeResource struct {
	SelectorResolver
	Configurator
	ConnectionPublisher
}

// NewReconciler returns a new Reconciler of composite infrastructure resources.
func NewReconciler(mgr manager.Manager, of resource.CompositeKind, opts ...ReconcilerOption) *Reconciler {
	nc := func() resource.Composite {
		return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(of)))
	}
	kube := unstructured.NewClient(mgr.GetClient())

	r := &Reconciler{
		client:       kube,
		newComposite: nc,

		composite: compositeResource{
			SelectorResolver:    NewAPISelectorResolver(kube),
			Configurator:        NewAPIConfigurator(kube),
			ConnectionPublisher: NewAPIFilteredSecretPublisher(kube, []string{}),
		},

		resource: composedctrl.NewComposer(kube),

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles composite infrastructure resources.
type Reconciler struct {
	client       client.Client
	newComposite func() resource.Composite

	composite compositeResource
	resource  Composer

	log    logging.Logger
	record event.Recorder
}

// Reconcile a composite infrastructure resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
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

	if err := r.composite.ResolveSelector(ctx, cr); err != nil {
		log.Debug(errSelectComp, "error", err)
		r.record.Event(cr, event.Warning(reasonResolve, err))
		cr.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errSelectComp)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}
	r.record.Event(cr, event.Normal(reasonResolve, "Successfully selected composition"))

	// TODO(muvaf): We should lock the deletion of Composition via finalizer
	// because its deletion will break the field propagation.
	comp := &v1alpha1.Composition{}
	if err := r.client.Get(ctx, meta.NamespacedNameOf(cr.GetCompositionReference()), comp); err != nil {
		log.Debug(errGetComp, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errGetComp)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.composite.Configure(ctx, cr, comp); err != nil {
		log.Debug(errConfigure, "error", err)
		r.record.Event(cr, event.Warning(reasonCompose, err))
		cr.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errConfigure)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
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
	refs := make([]corev1.ObjectReference, len(comp.Spec.To))
	copy(refs, cr.GetResourceReferences())
	conn := managed.ConnectionDetails{}
	ready := 0
	for i, ref := range refs {
		tmpl := comp.Spec.To[i]

		obs, err := r.resource.Compose(ctx, cr, composed.New(composed.FromReference(ref)), tmpl)
		if err != nil {
			log.Debug(errReconcile, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			cr.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errReconcile)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		for key, val := range obs.ConnectionDetails {
			conn[key] = val
		}

		if obs.Ready {
			ready++
		}

		// We need to update our composite resource with any new or updated
		// references to the resources it composes. We do this immediately after
		// each composed resource has been reconciled to ensure that we don't
		// forget all of our references if we hit an error. We avoid calling
		// update if the reconcile didn't change anything.
		if cmp.Equal(refs[i], obs.Ref) {
			continue
		}

		refs[i] = obs.Ref
		cr.SetResourceReferences(refs)
		if err := r.client.Update(ctx, cr); err != nil {
			log.Debug(errUpdate, "error", err)
			r.record.Event(cr, event.Warning(reasonCompose, err))
			cr.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errUpdate)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}
	}

	if err := r.composite.PublishConnection(ctx, cr, conn); err != nil {
		log.Debug(errPublish, "error", err)
		r.record.Event(cr, event.Warning(reasonPublish, err))
		cr.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(err, errPublish)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	// TODO(negz): Update status.composedResources and status.readyResources if
	// and when https://github.com/crossplane/crossplane-runtime/pull/166 lands.

	// TODO(negz): Add a bespoke 'partial' TypeReady condition?
	wait := longWait
	switch {
	case ready == 0:
		cr.SetConditions(runtimev1alpha1.Creating())
		wait = shortWait
	case ready == len(refs):
		cr.SetConditions(runtimev1alpha1.Available())
	}

	r.record.Event(cr, event.Normal(reasonPublish, "Successfully published connection details"))
	r.record.Event(cr, event.Normal(reasonCompose, "Successfully composed resources"))
	cr.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: wait}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}
