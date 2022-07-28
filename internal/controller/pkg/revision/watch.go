/*
Copyright 2021 The Crossplane Authors.

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
func (e *EnqueueRequestForReferencingProviderRevisions) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Update enqueues a request for all provider revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingProviderRevisions) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.ObjectOld, q)
	e.add(evt.ObjectNew, q)
}

// Delete enqueues a request for all provider revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingProviderRevisions) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Generic enqueues a request for all provider revisions that reference a given
// ControllerConfig.
func (e *EnqueueRequestForReferencingProviderRevisions) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

func (e *EnqueueRequestForReferencingProviderRevisions) add(obj runtime.Object, queue adder) {
	cc, ok := obj.(*v1alpha1.ControllerConfig)
	if !ok {
		return
	}

	l := &v1.ProviderRevisionList{}
	if err := e.client.List(context.TODO(), l); err != nil {
		// TODO(hasheddan): Handle this error?
		return
	}

	for _, pr := range l.Items {
		ref := pr.GetControllerConfigRef()
		if ref != nil && ref.Name == cc.GetName() {
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: pr.GetName()}})
		}
	}
}
