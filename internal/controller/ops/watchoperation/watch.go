/*
Copyright 2025 The Crossplane Authors.

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

package watchoperation

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

// NewWatchedResourceHandler returns a handler that enqueues reconcile requests
// for the WatchOperation when watched resources change, filtering based on the
// WatchOperation's matchLabels and namespace specifications.
//
// The WatchOperation is read on every event instead of captured at watch
// start: the engine keeps an already-running watch as-is, so a handler holding
// the first fetched spec would keep filtering on a stale selector after a
// spec change.
func NewWatchedResourceHandler(c client.Client, key types.NamespacedName) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		// Convert to unstructured to handle any resource type
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return nil
		}

		wo := &v1alpha1.WatchOperation{}
		if err := c.Get(ctx, key, wo); err != nil {
			// The WatchOperation is gone, so there is nothing to run.
			if kerrors.IsNotFound(err) {
				return nil
			}

			// Fail open on any other error. A map handler has no requeue,
			// so an event dropped here is lost until the watched resource
			// changes again. Enqueue the resource and let the reconciler
			// decide: it re-fetches the WatchOperation and re-applies its
			// filters, and an error there becomes a requeue with backoff.
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: u.GetName(), Namespace: u.GetNamespace()}},
			}
		}

		// Apply namespace filtering if specified
		if wo.Spec.Watch.Namespace != "" {
			// For cluster-scoped resources, namespace filtering doesn't apply
			if u.GetNamespace() != "" && u.GetNamespace() != wo.Spec.Watch.Namespace {
				return nil
			}
		}

		// Apply label selector filtering if specified
		if len(wo.Spec.Watch.MatchLabels) > 0 {
			selector := labels.SelectorFromSet(wo.Spec.Watch.MatchLabels)
			if !selector.Matches(labels.Set(u.GetLabels())) {
				return nil
			}
		}

		// Resource matches filters, enqueue the watched resource for reconciliation
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Name: u.GetName(), Namespace: u.GetNamespace()}},
		}
	})
}
