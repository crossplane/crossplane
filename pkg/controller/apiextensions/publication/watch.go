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

package publication

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

type adder interface {
	Add(item interface{})
}

// EnqueueRequestForRequirement enqueues a reconcile.Request for the
// NamespacedName of a RequirementReferencer's RequirementReference.
type EnqueueRequestForRequirement struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// RequirementReferencer.
func (e *EnqueueRequestForRequirement) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	addRequirement(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// RequirementReferencers.
func (e *EnqueueRequestForRequirement) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	addRequirement(evt.ObjectOld, q)
	addRequirement(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// RequirementReferencer.
func (e *EnqueueRequestForRequirement) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	addRequirement(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// a RequirementReferencer.
func (e *EnqueueRequestForRequirement) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	addRequirement(evt.Object, q)
}

func addRequirement(obj runtime.Object, queue adder) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	cp := &composite.Unstructured{Unstructured: *u}
	if cp.GetRequirementReference() != nil {
		queue.Add(reconcile.Request{NamespacedName: meta.NamespacedNameOf(cp.GetRequirementReference())})
	}
}
