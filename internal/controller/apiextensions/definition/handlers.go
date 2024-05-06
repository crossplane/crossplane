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
	"sigs.k8s.io/controller-runtime/pkg/client"
	kevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// EnqueueForCompositionRevision enqueues reconciles for all XRs that will use a
// newly created CompositionRevision.
func EnqueueForCompositionRevision(of resource.CompositeKind, c client.Reader, log logging.Logger) handler.Funcs {
	return handler.Funcs{
		CreateFunc: func(ctx context.Context, e kevent.CreateEvent, q workqueue.RateLimitingInterface) {
			rev, ok := e.Object.(*v1.CompositionRevision)
			if !ok {
				// should not happen
				return
			}

			// TODO(negz): Check whether the revision's compositeTypeRef matches
			// the supplied CompositeKind. If it doesn't, we can return early.

			// get all XRs
			xrs := kunstructured.UnstructuredList{}
			xrs.SetGroupVersionKind(schema.GroupVersionKind(of))
			xrs.SetKind(schema.GroupVersionKind(of).Kind + "List")
			// TODO(negz): Index XRs by composition revision name?
			if err := c.List(ctx, &xrs); err != nil {
				// logging is most we can do here. This is a programming error if it happens.
				log.Info("cannot list in CompositionRevision handler", "type", schema.GroupVersionKind(of).String(), "error", err)
				return
			}

			// enqueue all those that reference the Composition of this revision
			compName := rev.Labels[v1.LabelCompositionName]
			// TODO(negz): Check this before we get all XRs.
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
		},
	}
}

// EnqueueCompositeResources enqueues reconciles for all XRs that reference an
// updated composed resource.
func EnqueueCompositeResources(of resource.CompositeKind, c client.Reader, log logging.Logger) handler.Funcs {
	return handler.Funcs{
		UpdateFunc: func(ctx context.Context, ev kevent.UpdateEvent, q workqueue.RateLimitingInterface) {
			xrGVK := schema.GroupVersionKind(of)
			cdGVK := ev.ObjectNew.GetObjectKind().GroupVersionKind()
			key := refKey(ev.ObjectNew.GetNamespace(), ev.ObjectNew.GetName(), cdGVK.Kind, cdGVK.GroupVersion().String())

			composites := kunstructured.UnstructuredList{}
			composites.SetGroupVersionKind(xrGVK.GroupVersion().WithKind(xrGVK.Kind + "List"))
			if err := c.List(ctx, &composites, client.MatchingFields{compositeResourcesRefsIndex: key}); err != nil {
				log.Debug("cannot list composite resources related to a composed resource change", "error", err, "gvk", xrGVK.String(), "fieldSelector", compositeResourcesRefsIndex+"="+key)
				return
			}

			// queue those composites for reconciliation
			for _, xr := range composites.Items {
				log.Debug("Enqueueing composite resource because composed resource changed", "name", xr.GetName(), "cdGVK", cdGVK.String(), "cdName", ev.ObjectNew.GetName())
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: xr.GetName()}})
			}
		},
	}
}
