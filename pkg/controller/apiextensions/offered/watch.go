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

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// OffersClaim accepts objects that are a CompositeResourceDefinition and offer
// a composite resource claim.
func OffersClaim() resource.PredicateFn {
	return func(obj runtime.Object) bool {
		d, ok := obj.(*v1alpha1.CompositeResourceDefinition)
		if !ok {
			return false
		}
		return d.OffersClaim()
	}
}

type adder interface {
	Add(item interface{})
}

// EnqueueRequestForClaim enqueues a reconcile.Request for the NamespacedName of
// a composite resource's ClaimReference.
type EnqueueRequestForClaim struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// composite resource.
func (e *EnqueueRequestForClaim) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// composite resources.
func (e *EnqueueRequestForClaim) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.ObjectOld, q)
	addClaim(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// composite resource.
func (e *EnqueueRequestForClaim) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// a composite resource.
func (e *EnqueueRequestForClaim) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

func addClaim(obj runtime.Object, queue adder) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	cp := &composite.Unstructured{Unstructured: *u}
	if cp.GetClaimReference() != nil {
		queue.Add(reconcile.Request{NamespacedName: meta.NamespacedNameOf(cp.GetClaimReference())})
	}
}

// EnqueueRequestForControllersClaim enqueues a reconcile.Request for the
// NamespacedName of a Secret's controller's ClaimReference. The controller must
// be a composite resource.
type EnqueueRequestForControllersClaim struct{ client client.Reader }

// Create adds a NamespacedName for the supplied CreateEvent if its Object is
// the connection secret of a claimed composite resource.
func (e *EnqueueRequestForControllersClaim) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.addClaim(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// the connection secret of a claimed composite resource.
func (e *EnqueueRequestForControllersClaim) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.addClaim(evt.ObjectOld, q)
	e.addClaim(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is
// the connection secret of a claimed composite resource.
func (e *EnqueueRequestForControllersClaim) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.addClaim(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// the connection secret of a claimed composite resource.
func (e *EnqueueRequestForControllersClaim) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.addClaim(evt.Object, q)
}

func (e *EnqueueRequestForControllersClaim) addClaim(obj runtime.Object, queue adder) {
	s, ok := obj.(*corev1.Secret)
	if !ok {
		return
	}

	if s.Type != resource.SecretTypeConnection {
		return
	}

	ref := v1.GetControllerOf(s)
	if ref == nil {
		return
	}

	nn := types.NamespacedName{Name: ref.Name}
	cp := &composite.Unstructured{}
	cp.SetAPIVersion(ref.APIVersion)
	cp.SetKind(ref.Kind)
	if err := e.client.Get(context.TODO(), nn, cp); err != nil {
		return
	}

	if cp.GetClaimReference() != nil {
		queue.Add(reconcile.Request{NamespacedName: meta.NamespacedNameOf(cp.GetClaimReference())})
	}
}
