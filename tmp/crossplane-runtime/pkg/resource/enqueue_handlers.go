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
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type adder interface {
	Add(item reconcile.Request)
}

// RateLimitingInterface for an EnqueueRequestForProviderConfig.
type RateLimitingInterface = workqueue.TypedRateLimitingInterface[reconcile.Request]

// EnqueueRequestForProviderConfig enqueues a reconcile.Request for a referenced
// ProviderConfig.
type EnqueueRequestForProviderConfig struct {
	// Kind is the expected ProviderConfig kind this handler should process.
	// If empty, all kinds are processed (backward compatibility).
	Kind string
}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Create(_ context.Context, evt event.CreateEvent, q RateLimitingInterface) {
	e.addProviderConfig(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// a ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Update(_ context.Context, evt event.UpdateEvent, q RateLimitingInterface) {
	e.addProviderConfig(evt.ObjectOld, q)
	e.addProviderConfig(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Delete(_ context.Context, evt event.DeleteEvent, q RateLimitingInterface) {
	e.addProviderConfig(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// a ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Generic(_ context.Context, evt event.GenericEvent, q RateLimitingInterface) {
	e.addProviderConfig(evt.Object, q)
}

func (e *EnqueueRequestForProviderConfig) addProviderConfig(obj runtime.Object, queue adder) {
	switch pcr := obj.(type) {
	case TypedProviderConfigUsage:
		ref := pcr.GetProviderConfigReference()
		refKind := ref.Kind
		if refKind == "" {
			refKind = "ProviderConfig"
		}
		if e.Kind != "" && refKind != e.Kind {
			return
		}

		if strings.HasPrefix(refKind, "Cluster") {
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: ref.Name}})
		} else {
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: ref.Name, Namespace: pcr.GetNamespace()}})
		}
	case LegacyProviderConfigUsage:
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: pcr.GetProviderConfigReference().Name}})
	}
}
