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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
)

// CompositionRevisionMapFunc returns a MapFunc that maps CompositionRevisions to affected XRs.
func CompositionRevisionMapFunc(of schema.GroupVersionKind, s composite.Schema, c client.Reader, log logging.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		rev, ok := obj.(*v1.CompositionRevision)
		if !ok {
			return nil
		}

		// We don't know what composition this revision is for,
		// so we can't determine whether an XR might use it.
		// This should never happen in practice - the
		// composition controller sets this label when it
		// creates a revision.
		compName := rev.Labels[v1.LabelCompositionName]
		if compName == "" {
			return nil
		}

		// This handler is for a specific type of XR. This
		// revision isn't compatible with that type.
		if rev.Spec.CompositeTypeRef.APIVersion != of.GroupVersion().String() {
			return nil
		}
		if rev.Spec.CompositeTypeRef.Kind != of.Kind {
			return nil
		}

		xrs := kunstructured.UnstructuredList{}
		xrs.SetGroupVersionKind(of)
		xrs.SetKind(of.Kind + "List")
		// TODO(negz): Index XRs by composition revision name?
		if err := c.List(ctx, &xrs); err != nil {
			// Logging is most we can do here. This is a programming error if it happens.
			log.Info("cannot list in CompositionRevision handler", "type", of.String(), "error", err)
			return nil
		}

		requests := make([]reconcile.Request, 0)
		for _, u := range xrs.Items {
			xr := composite.Unstructured{Unstructured: u, Schema: s}

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

			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      xr.GetName(),
				Namespace: xr.GetNamespace(),
			}})
		}

		return requests
	}
}

// SelfMapFunc returns a MapFunc that maps an object to itself for reconciliation.
// This is equivalent to handler.EnqueueRequestForObject but as a MapFunc.
func SelfMapFunc() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}}
	}
}

// CompositeResourcesMapFunc returns a MapFunc that maps composed resources to affected XRs.
func CompositeResourcesMapFunc(of schema.GroupVersionKind, c client.Reader, log logging.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		cdGVK := obj.GetObjectKind().GroupVersionKind()
		key := refKey(obj.GetNamespace(), obj.GetName(), cdGVK.Kind, cdGVK.GroupVersion().String())

		composites := kunstructured.UnstructuredList{}
		composites.SetGroupVersionKind(of.GroupVersion().WithKind(of.Kind + "List"))
		if err := c.List(ctx, &composites, client.MatchingFields{compositeResourcesRefsIndex: key}); err != nil {
			log.Debug("cannot list composite resources related to a composed resource change", "error", err, "gvk", of.String(), "fieldSelector", compositeResourcesRefsIndex+"="+key)
			return nil
		}

		requests := make([]reconcile.Request, len(composites.Items))
		for i, xr := range composites.Items {
			log.Debug("Mapping composite resource because composed resource changed",
				"namespace", xr.GetNamespace(),
				"name", xr.GetName(),
				"cdGVK", cdGVK.String(),
				"cdName", obj.GetName(),
			)
			requests[i] = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&xr)}
		}

		return requests
	}
}
