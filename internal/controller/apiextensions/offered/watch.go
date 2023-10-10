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

package offered

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// OffersClaim accepts objects that are a CompositeResourceDefinition and offer
// a composite resource claim.
func OffersClaim() resource.PredicateFn {
	return func(obj runtime.Object) bool {
		d, ok := obj.(*v1.CompositeResourceDefinition)
		if !ok {
			return false
		}
		return d.OffersClaim()
	}
}

type adder interface {
	Add(item any)
}

// EnqueueRequestForClaim enqueues a reconcile.Request for the
// NamespacedName of a ClaimReferencer's ClaimReference.
type EnqueueRequestForClaim struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Create(_ context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// ClaimReferencers.
func (e *EnqueueRequestForClaim) Update(_ context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.ObjectOld, q)
	addClaim(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Delete(_ context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// a ClaimReferencer.
func (e *EnqueueRequestForClaim) Generic(_ context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

func addClaim(obj runtime.Object, queue adder) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	cp := &composite.Unstructured{Unstructured: *u}
	if ref := cp.GetClaimReference(); ref != nil {
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}})
	}
}
