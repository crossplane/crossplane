/*
Copyright 2023 The Crossplane Authors.

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
	"fmt"

	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

const (
	// compositeResourceRefGVKsIndex is an index of all GroupKinds that
	// are in use by a Composition. It indexes from spec.resourceRefs, not
	// from spec.resources. Hence, it will also work with functions.
	compositeResourceRefGVKsIndex = "compositeResourceRefsGVKs"
	// compositeResourcesRefsIndex is an index of resourceRefs that are owned
	// by a composite.
	compositeResourcesRefsIndex = "compositeResourcesRefs"
)

var (
	_ client.IndexerFunc = IndexCompositeResourceRefGVKs
	_ client.IndexerFunc = IndexCompositeResourcesRefs
)

// IndexCompositeResourceRefGVKs assumes the passed object is a composite. It
// returns gvk keys for every resource referenced in the composite.
func IndexCompositeResourceRefGVKs(o client.Object) []string {
	u, ok := o.(*kunstructured.Unstructured)
	if !ok {
		return nil // should never happen
	}
	xr := composite.Unstructured{Unstructured: *u}
	refs := xr.GetResourceReferences()
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		group, version := parseAPIVersion(ref.APIVersion)
		keys = append(keys, schema.GroupVersionKind{Group: group, Version: version, Kind: ref.Kind}.String())
	}
	// unification is done by the informer.
	return keys
}

// IndexCompositeResourcesRefs assumes the passed object is a composite. It
// returns keys for every composed resource referenced in the composite.
func IndexCompositeResourcesRefs(o client.Object) []string {
	u, ok := o.(*kunstructured.Unstructured)
	if !ok {
		return nil // should never happen
	}
	xr := composite.Unstructured{Unstructured: *u}
	refs := xr.GetResourceReferences()
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, refKey(ref.Namespace, ref.Name, ref.Kind, ref.APIVersion))
	}
	return keys
}

func refKey(ns, name, kind, apiVersion string) string {
	return fmt.Sprintf("%s.%s.%s.%s", name, ns, kind, apiVersion)
}

func enqueueXRsForMR(ca cache.Cache, xrGVK schema.GroupVersionKind, log logging.Logger) func(ctx context.Context, ev runtimeevent.UpdateEvent, q workqueue.RateLimitingInterface) {
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
