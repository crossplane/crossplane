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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type adder interface {
	Add(item interface{})
}

// EnqueueRequestIfNamed enqueues a request only for objects of a specific name.
type EnqueueRequestIfNamed struct {
	Name string
}

// Create adds a NamespacedName for the supplied CreateEvent if its Object has
// the desired name.
func (e *EnqueueRequestIfNamed) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Object has
// the desired name.
func (e *EnqueueRequestIfNamed) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.ObjectOld, q)
	e.add(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object has
// the desired name.
func (e *EnqueueRequestIfNamed) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object has
// the desired name
func (e *EnqueueRequestIfNamed) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

func (e *EnqueueRequestIfNamed) add(obj runtime.Object, queue adder) {
	if m, ok := obj.(v1.Object); ok && m.GetName() == e.Name {
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: m.GetName()}})
	}
}
