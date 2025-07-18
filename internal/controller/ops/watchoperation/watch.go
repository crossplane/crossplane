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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
)

// NewWatchedResourceHandler returns a handler that enqueues reconcile requests
// for the WatchOperation when watched resources change, filtering based on the
// WatchOperation's matchLabels and namespace specifications.
func NewWatchedResourceHandler(wo *v1alpha1.WatchOperation) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		// Convert to unstructured to handle any resource type
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return nil
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
