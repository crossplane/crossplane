// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package revision

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

type adder interface {
	Add(item any)
}

// EnqueueRequestForReferencingProviderRevisions enqueues a request for all
// provider revisions that reference a ControllerConfig when the given
// ControllerConfig changes.
type EnqueueRequestForReferencingProviderRevisions struct {
	client client.Client
}

// Create enqueues a request for all provider revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingProviderRevisions) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

// Update enqueues a request for all provider revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingProviderRevisions) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.ObjectOld, q)
	e.add(ctx, evt.ObjectNew, q)
}

// Delete enqueues a request for all provider revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingProviderRevisions) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

// Generic enqueues a request for all provider revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingProviderRevisions) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

func (e *EnqueueRequestForReferencingProviderRevisions) add(ctx context.Context, obj runtime.Object, queue adder) { //nolint:gocyclo // it will be simplified soon when we clean up the controller config
	cc, isCC := obj.(*v1alpha1.ControllerConfig)
	rc, isRC := obj.(*v1beta1.DeploymentRuntimeConfig)

	if !isCC && !isRC {
		return
	}

	l := &v1.ProviderRevisionList{}
	if err := e.client.List(ctx, l); err != nil {
		// TODO(hasheddan): Handle this error?
		return
	}

	for _, pr := range l.Items {
		if isCC {
			ref := pr.GetControllerConfigRef()
			if ref != nil && ref.Name == cc.GetName() {
				queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: pr.GetName()}})
			}
		}
		if isRC {
			ref := pr.GetRuntimeConfigRef()
			if ref != nil && ref.Name == rc.GetName() {
				queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: pr.GetName()}})
			}
		}
	}
}

// EnqueueRequestForReferencingFunctionRevisions enqueues a request for all
// function revisions that reference a ControllerConfig when the given
// ControllerConfig changes.
type EnqueueRequestForReferencingFunctionRevisions struct {
	client client.Client
}

// Create enqueues a request for all function revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingFunctionRevisions) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

// Update enqueues a request for all function revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingFunctionRevisions) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.ObjectOld, q)
	e.add(ctx, evt.ObjectNew, q)
}

// Delete enqueues a request for all function revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingFunctionRevisions) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

// Generic enqueues a request for all function revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingFunctionRevisions) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

func (e *EnqueueRequestForReferencingFunctionRevisions) add(ctx context.Context, obj runtime.Object, queue adder) { //nolint:gocyclo // it will be simplified soon when we clean up the controller config
	cc, isCC := obj.(*v1alpha1.ControllerConfig)
	rc, isRC := obj.(*v1beta1.DeploymentRuntimeConfig)

	if !isCC && !isRC {
		return
	}

	l := &v1beta1.FunctionRevisionList{}
	if err := e.client.List(ctx, l); err != nil {
		// TODO(hasheddan): Handle this error?
		return
	}

	for _, pr := range l.Items {
		if isCC {
			ref := pr.GetControllerConfigRef()
			if ref != nil && ref.Name == cc.GetName() {
				queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: pr.GetName()}})
			}
		}
		if isRC {
			ref := pr.GetRuntimeConfigRef()
			if ref != nil && ref.Name == rc.GetName() {
				queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: pr.GetName()}})
			}
		}
	}
}
