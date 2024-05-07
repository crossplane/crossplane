/*
Copyright 2024 The Crossplane Authors.

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

package definition

import (
	"context"

	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	cache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// For comp rev
// EnqueueForCompositionRevisionFunc returns a function that enqueues (the
// related) XRs when a new CompositionRevision is created. This speeds up
// reconciliation of XRs on changes to the Composition by not having to wait for
// the 60s sync period, but be instant.
func EnqueueForCompositionRevisionFunc(of resource.CompositeKind, list func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error, log logging.Logger) func(ctx context.Context, createEvent runtimeevent.TypedCreateEvent[*v1.CompositionRevision], q workqueue.RateLimitingInterface) {
	return func(ctx context.Context, createEvent runtimeevent.TypedCreateEvent[*v1.CompositionRevision], q workqueue.RateLimitingInterface) {
		rev := createEvent.Object

		// get all XRs
		xrs := kunstructured.UnstructuredList{}
		xrs.SetGroupVersionKind(schema.GroupVersionKind(of))
		xrs.SetKind(schema.GroupVersionKind(of).Kind + "List")
		if err := list(ctx, &xrs); err != nil {
			// logging is most we can do here. This is a programming error if it happens.
			log.Info("cannot list in CompositionRevision handler", "type", schema.GroupVersionKind(of).String(), "error", err)
			return
		}

		// enqueue all those that reference the Composition of this revision
		compName := rev.Labels[v1.LabelCompositionName]
		if compName == "" {
			return
		}
		for _, u := range xrs.Items {
			xr := composite.Unstructured{Unstructured: u}

			// only automatic
			if pol := xr.GetCompositionUpdatePolicy(); pol != nil && *pol == xpv1.UpdateManual {
				continue
			}

			// only those that reference the right Composition
			if ref := xr.GetCompositionReference(); ref == nil || ref.Name != compName {
				continue
			}

			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      xr.GetName(),
				Namespace: xr.GetNamespace(),
			}})
		}
	}
}

// TODO(negz): Figure out a way to plumb this with controller-runtime v0.18.x
// style sources.

func enqueueXRsForMR(ca cache.Cache, xrGVK schema.GroupVersionKind, log logging.Logger) func(ctx context.Context, ev runtimeevent.UpdateEvent, q workqueue.RateLimitingInterface) { //nolint:unused // See comment above.
	return func(ctx context.Context, ev runtimeevent.UpdateEvent, q workqueue.RateLimitingInterface) {
		mrGVK := ev.ObjectNew.GetObjectKind().GroupVersionKind()
		key := refKey(ev.ObjectNew.GetNamespace(), ev.ObjectNew.GetName(), mrGVK.Kind, mrGVK.GroupVersion().String())
		composites := kunstructured.UnstructuredList{}
		composites.SetGroupVersionKind(xrGVK.GroupVersion().WithKind(xrGVK.Kind + "List"))
		if err := ca.List(ctx, &composites, client.MatchingFields{compositeResourcesRefsIndex: key}); err != nil {
			log.Debug("cannot list composite resources related to a MR change", "error", err, "gvk", xrGVK.String(), "fieldSelector", compositeResourcesRefsIndex+"="+key)
			return
		}
		// queue those composites for reconciliation
		for _, xr := range composites.Items {
			log.Info("Enqueueing composite resource because managed resource changed", "name", xr.GetName(), "mrGVK", mrGVK.String(), "mrName", ev.ObjectNew.GetName())
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: xr.GetName()}})
		}
	}
}
