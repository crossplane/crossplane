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

package roles

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

type adder interface {
	Add(item reconcile.Request)
}

// EnqueueRequestForAllRevisionsInFamily enqueues a request for all
// provider revisions with the same family as one that changed.
type EnqueueRequestForAllRevisionsInFamily struct {
	client client.Client
}

// Create enqueues a request for all provider revisions within the same family.
func (e *EnqueueRequestForAllRevisionsInFamily) Create(ctx context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	e.add(ctx, evt.Object, q)
}

// Update enqueues a request for all provider revisions within the same family.
func (e *EnqueueRequestForAllRevisionsInFamily) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	e.add(ctx, evt.ObjectOld, q)
	e.add(ctx, evt.ObjectNew, q)
}

// Delete enqueues a request for all provider revisions within the same family.
func (e *EnqueueRequestForAllRevisionsInFamily) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	e.add(ctx, evt.Object, q)
}

// Generic enqueues a request for all provider revisions within the same family.
func (e *EnqueueRequestForAllRevisionsInFamily) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	e.add(ctx, evt.Object, q)
}

func (e *EnqueueRequestForAllRevisionsInFamily) add(ctx context.Context, obj runtime.Object, queue adder) {
	pr, ok := obj.(*v1.ProviderRevision)
	if !ok {
		return
	}
	family := pr.GetLabels()[v1.LabelProviderFamily]
	if family == "" {
		// This revision is not part of a family.
		return
	}

	l := &v1.ProviderRevisionList{}
	if err := e.client.List(ctx, l, client.MatchingLabels{v1.LabelProviderFamily: family}); err != nil {
		// TODO(negz): Handle this error?
		return
	}

	for _, member := range l.Items {
		if member.GetUID() == pr.GetUID() {
			// No need to enqueue a request for the ProviderRevision that
			// triggered this enqueue.
			continue
		}
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: member.GetName()}})
	}
}
