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
	"slices"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// OffersClaim accepts any CompositeResourceDefinition that offers a claim.
func OffersClaim() resource.PredicateFn {
	return func(obj runtime.Object) bool {
		d, ok := obj.(*v2.CompositeResourceDefinition)
		if !ok {
			return false
		}

		return d.OffersClaim()
	}
}

// IsClaimCRD accepts any CustomResourceDefinition that represents a Claim.
func IsClaimCRD() resource.PredicateFn {
	return func(obj runtime.Object) bool {
		d, ok := obj.(*extv1.CustomResourceDefinition)
		if !ok {
			return false
		}

		return slices.Contains(d.Spec.Names.Categories, xcrd.CategoryClaim)
	}
}

type adder interface {
	Add(item reconcile.Request)
}

// EnqueueRequestForClaim enqueues a reconcile.Request for the
// NamespacedName of a ClaimReferencer's ClaimReference.
type EnqueueRequestForClaim struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Create(_ context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	addClaim(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// ClaimReferencers.
func (e *EnqueueRequestForClaim) Update(_ context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	addClaim(evt.ObjectOld, q)
	addClaim(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Delete(_ context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	addClaim(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// a ClaimReferencer.
func (e *EnqueueRequestForClaim) Generic(_ context.Context, evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	addClaim(evt.Object, q)
}

func addClaim(obj runtime.Object, queue adder) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}

	cp := &composite.Unstructured{Unstructured: *u, Schema: composite.SchemaLegacy}
	if ref := cp.GetClaimReference(); ref != nil {
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}})
	}
}
