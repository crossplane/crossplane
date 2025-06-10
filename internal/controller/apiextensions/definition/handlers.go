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
		CreateFunc: func(ctx context.Context, e kevent.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			rev, ok := e.Object.(*v1.CompositionRevision)
			if !ok {
				return
			}

			// We don't know what composition this revision is for,
			// so we can't determine whether an XR might use it.
			// This should never happen in practice - the
			// composition controller sets this label when it
			// creates a revision.
			compName := rev.Labels[v1.LabelCompositionName]
			if compName == "" {
				return
			}

			// This handler is for a specific type of XR. This
			// revisionisn't compatible with that type.
			if rev.Spec.CompositeTypeRef.APIVersion != schema.GroupVersionKind(of).GroupVersion().String() {
				return
			}
			if rev.Spec.CompositeTypeRef.Kind != of.Kind {
				return
			}

			xrs := kunstructured.UnstructuredList{}
			xrs.SetGroupVersionKind(schema.GroupVersionKind(of))
			xrs.SetKind(schema.GroupVersionKind(of).Kind + "List")
			// TODO(negz): Index XRs by composition revision name?
			if err := c.List(ctx, &xrs); err != nil {
				// Logging is most we can do here. This is a programming error if it happens.
				log.Info("cannot list in CompositionRevision handler", "type", schema.GroupVersionKind(of).String(), "error", err)
				return
			}

			// Enqueue all those that reference the composition of
			// this revision.
			for _, u := range xrs.Items {
				xr := composite.Unstructured{Unstructured: u}

				// We only care about XRs that would
				// automatically update to this new revision.
				if pol := xr.GetCompositionUpdatePolicy(); pol != nil && *pol == xpv1.UpdateManual {
					continue
				}

				// We only care about XRs that reference the
				// composition this revision derives from.
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
		UpdateFunc: func(ctx context.Context, ev kevent.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
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
