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

package namespace

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type adder interface {
	Add(item any)
}

// EnqueueRequestForNamespaces enqueues a reconcile for all namespaces whenever
// a ClusterRole with the aggregation labels we're concerned with changes. This
// is unusual, but we expect there to be relatively few ClusterRoles, and we
// have no way of relating a specific ClusterRoles back to the Roles that
// aggregate it. This is the approach the upstream aggregation controller uses.
// https://github.com/kubernetes/kubernetes/blob/323f348/pkg/controller/clusterroleaggregation/clusterroleaggregation_controller.go#L188
type EnqueueRequestForNamespaces struct {
	client client.Reader
}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is an
// aggregated ClusterRole.
func (e *EnqueueRequestForNamespaces) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Object is an
// aggregated ClusterRole.
func (e *EnqueueRequestForNamespaces) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.ObjectOld, q)
	e.add(ctx, evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is an
// aggregated ClusterRole.
func (e *EnqueueRequestForNamespaces) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// an aggregated ClusterRole.
func (e *EnqueueRequestForNamespaces) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.add(ctx, evt.Object, q)
}

func (e *EnqueueRequestForNamespaces) add(ctx context.Context, obj runtime.Object, queue adder) {
	cr, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return
	}

	if !aggregates(cr) {
		return
	}

	l := &corev1.NamespaceList{}
	if err := e.client.List(ctx, l); err != nil {
		return
	}

	for _, ns := range l.Items {
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: ns.GetName()}})
	}
}

func aggregates(obj metav1.Object) bool {
	for k := range obj.GetLabels() {
		if strings.HasPrefix(k, keyPrefix) {
			return true
		}
	}
	return false
}
