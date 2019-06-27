/*
Copyright 2019 The Crossplane Authors.

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

package resource

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/meta"
)

// EnqueueRequestForClaim enqueues a reconcile.Request for the NamespacedName
// of a ClaimReferencer's ClaimReference.
type EnqueueRequestForClaim struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// ClaimReferencers.
func (e *EnqueueRequestForClaim) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.ObjectOld, q)
	addClaim(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

type adder interface {
	Add(item interface{})
}

func addClaim(obj runtime.Object, queue adder) {
	if cr, ok := obj.(ClaimReferencer); ok && cr.GetClaimReference() != nil {
		queue.Add(reconcile.Request{NamespacedName: meta.NamespacedNameOf(cr.GetClaimReference())})
	}
}
