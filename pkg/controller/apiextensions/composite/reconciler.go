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
	v1 "k8s.io/api/core/v1"
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

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
	timeout   = 2 * time.Minute

	finalizer = "finalizer.apiextensions.crossplane.io"
)

// ConnectionSecretFilterer returns a set of allowed keys.
type ConnectionSecretFilterer interface {
	GetConnectionSecretKeys() []string
}

// A ConnectionPublisher manages the supplied ConnectionDetails for the
// supplied resource. ManagedPublishers must handle the case in which
// the supplied ConnectionDetails are empty.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied resource. Publishing
	// must be additive; i.e. if details (a, b, c) are published, subsequently
	// publicing details (b, c, d) should update (b, c) but not remove a.
	PublishConnection(ctx context.Context, owner resource.ConnectionSecretOwner, c managed.ConnectionDetails) error

	// UnpublishConnection details for the supplied resource.
	UnpublishConnection(ctx context.Context, owner resource.ConnectionSecretOwner, c managed.ConnectionDetails) error
}

// NewCompositeReconciler returns a new *compositeReconciler.
func NewCompositeReconciler(name string, mgr manager.Manager, gvk schema.GroupVersionKind, log logging.Logger, filterer ConnectionSecretFilterer) reconcile.Reconciler {
	nc := func() resource.Composite { return unstructured.NewComposite(unstructured.WithGroupVersionKind(gvk)) }
	kube := NewClientForUnregistered(mgr.GetClient())

	return &compositeReconciler{
		client:       kube,
		newComposite: nc,
		Resolver:     NewSelectorResolver(kube),
		composed:     NewAPIComposedReconciler(kube),
		connection:   NewAPIFilteredSecretPublisher(kube, filterer.GetConnectionSecretKeys()),
		finalizer:    resource.NewAPIFinalizer(kube, finalizer),
		log:          log,
		record:       event.NewAPIRecorder(mgr.GetEventRecorderFor(name)),
	}
}

// ComposableReconciler is able to reconcile a member of the composite resource.
type ComposableReconciler interface {
	Reconcile(ctx context.Context, cr resource.Composite, composedRef v1.ObjectReference, tmpl v1alpha1.ComposedTemplate) (Observation, error)
}

// Resolver selects the composition reference with the information given as selector.
type Resolver interface {
	ResolveSelector(ctx context.Context, cr resource.Composite) error
}

// compositeReconciler reconciles the generic CRD that is generated via InfrastructureDefinition.
type compositeReconciler struct {
	client       client.Client
	newComposite func() resource.Composite
	composed     ComposableReconciler
	connection   ConnectionPublisher
	finalizer    resource.Finalizer

	// TODO(muvaf): Implement `Initializer` interface to be satisfied by both
	// selector resolver and empty connection secret ref defaulter.
	Resolver

	log    logging.Logger
	record event.Recorder
}

// Reconcile reconciles given custom resource.
func (r *compositeReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cr := r.newComposite()
	if err := r.client.Get(ctx, req.NamespacedName, cr); err != nil {
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get composite resource")
	}

	if meta.WasDeleted(cr) && len(cr.GetResourceReferences()) == 0 {
		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot remove finalizer")
		}
		return reconcile.Result{}, nil
	}

	if err := r.ResolveSelector(ctx, cr); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot resolve composition selector")
	}
	// TODO(muvaf): We should lock the deletion of Composition via finalizer
	// because its deletion will break the field propagation.
	comp := &v1alpha1.Composition{}
	if err := r.client.Get(ctx, meta.NamespacedNameOf(cr.GetCompositionReference()), comp); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot get the composition")
	}

	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot add finalizer")
	}

	// We start with empty ObjectRefs and fill them up as they are provisioned.
	refs := make([]v1.ObjectReference, len(comp.Spec.To))
	copy(refs, cr.GetResourceReferences())
	conn := managed.ConnectionDetails{}
	for i, composedRef := range refs {
		obs, err := r.composed.Reconcile(ctx, cr, composedRef, comp.Spec.To[i])
		if err != nil {
			return reconcile.Result{RequeueAfter: shortWait}, err
		}
		refs[i] = obs.Ref
		cr.SetResourceReferences(refs)
		if err := r.client.Update(ctx, cr); err != nil {
			return reconcile.Result{RequeueAfter: shortWait}, err
		}
		for key, val := range obs.ConnectionDetails {
			conn[key] = val
		}
	}

	if err := r.connection.PublishConnection(ctx, cr, conn); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(err, "cannot publish connection secret")
	}

	cr.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, cr), "cannot update status of composite resource")
}
