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
	"strings"
	"sync"

	"github.com/google/uuid"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	cache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	runtimeevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/xcrd"
)

// composedResourceInformers manages composed resource informers referenced by
// composite resources. It serves as an event source for realtime notifications
// of changed composed resources, with the composite reconcilers as sinks.
// It keeps composed resource informers alive as long as there are composites
// referencing them. In parallel, the composite reconcilers keep track of
// references to composed resources, and inform composedResourceInformers about
// them via the WatchComposedResources method.
type composedResourceInformers struct {
	log     logging.Logger
	cluster cluster.Cluster

	gvkRoutedCache *controller.GVKRoutedCache

	lock sync.RWMutex // everything below is protected by this lock

	// cdCaches holds the composed resource informers. These are dynamically
	// started and stopped based on the composites that reference them.
	cdCaches map[schema.GroupVersionKind]cdCache
	// xrCaches holds the composite resource informers. We use them to find
	// composites referencing a certain composed resource GVK. If no composite
	// is left doing so, a composed resource informer is stopped.
	xrCaches map[schema.GroupVersionKind]cache.Cache
	sinks    map[string]func(ev runtimeevent.UpdateEvent) // by some uid
}

type cdCache struct {
	cache    cache.Cache
	cancelFn context.CancelFunc
}

var _ source.Source = &composedResourceInformers{}

// Start implements source.Source, i.e. starting composedResourceInformers as
// source with h as the sink of update events. It keeps sending events until
// ctx is done.
// Note that Start can be called multiple times to deliver events to multiple
// (composite resource) controllers.
func (i *composedResourceInformers) Start(ctx context.Context, h handler.EventHandler, q workqueue.RateLimitingInterface, ps ...predicate.Predicate) error {
	id := uuid.New().String()

	i.lock.Lock()
	defer i.lock.Unlock()
	i.sinks[id] = func(ev runtimeevent.UpdateEvent) {
		for _, p := range ps {
			if !p.Update(ev) {
				return
			}
		}
		h.Update(ctx, ev, q)
	}

	go func() {
		<-ctx.Done()

		i.lock.Lock()
		defer i.lock.Unlock()
		delete(i.sinks, id)
	}()

	return nil
}

// RegisterComposite registers a composite resource cache with its GVK.
// Instances of this GVK will be considered to keep composed resource informers
// alive.
func (i *composedResourceInformers) RegisterComposite(gvk schema.GroupVersionKind, ca cache.Cache) {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.xrCaches == nil {
		i.xrCaches = make(map[schema.GroupVersionKind]cache.Cache)
	}
	i.xrCaches[gvk] = ca
}

// UnregisterComposite removes a composite resource cache from being considered
// to keep composed resource informers alive.
func (i *composedResourceInformers) UnregisterComposite(gvk schema.GroupVersionKind) {
	i.lock.Lock()
	defer i.lock.Unlock()
	delete(i.xrCaches, gvk)
}

// WatchComposedResources starts informers for the given composed resource GVKs.
// The is wired into the composite reconciler, which will call this method on
// every reconcile to make composedResourceInformers aware of the composed
// resources the given composite resource references.
//
// Note that this complements cleanupComposedResourceInformers which regularly
// garbage collects composed resource informers that are no longer referenced by
// any composite.
func (i *composedResourceInformers) WatchComposedResources(gvks ...schema.GroupVersionKind) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	// start new informers
	for _, gvk := range gvks {
		if _, found := i.cdCaches[gvk]; found {
			continue
		}

		log := i.log.WithValues("gvk", gvk.String())

		ca, err := cache.New(i.cluster.GetConfig(), cache.Options{})
		if err != nil {
			log.Debug("failed creating a cache", "error", err)
			continue
		}

		// don't forget to call cancelFn in error cases to avoid leaks. In the
		// happy case it's called from the go routine starting the cache below.
		ctx, cancelFn := context.WithCancel(context.Background())

		u := kunstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		inf, err := ca.GetInformer(ctx, &u, cache.BlockUntilSynced(false)) // don't block. We wait in the go routine below.
		if err != nil {
			cancelFn()
			log.Debug("failed getting informer", "error", err)
			continue
		}

		if _, err := inf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				old := oldObj.(client.Object) //nolint:forcetypeassert // Will always be client.Object.
				obj := newObj.(client.Object) //nolint:forcetypeassert // Will always be client.Object.
				if old.GetResourceVersion() == obj.GetResourceVersion() {
					return
				}

				i.lock.RLock()
				defer i.lock.RUnlock()

				ev := runtimeevent.UpdateEvent{
					ObjectOld: old,
					ObjectNew: obj,
				}
				for _, handleFn := range i.sinks {
					handleFn(ev)
				}
			},
		}); err != nil {
			cancelFn()
			log.Debug("failed adding event handler", "error", err)
			continue
		}

		go func() {
			defer cancelFn()

			log.Info("Starting composed resource watch")
			_ = ca.Start(ctx)
		}()

		i.cdCaches[gvk] = cdCache{
			cache:    ca,
			cancelFn: cancelFn,
		}

		// wait for in the background, and only when synced add to the routed cache
		go func(gvk schema.GroupVersionKind) {
			if synced := ca.WaitForCacheSync(ctx); synced {
				log.Debug("Composed resource cache synced")
				i.gvkRoutedCache.AddDelegate(gvk, ca)
			}
		}(gvk)
	}
}

// cleanupComposedResourceInformers garbage collects composed resource informers
// that are no longer referenced by any composite resource.
//
// Note that this complements WatchComposedResources which starts informers for
// the composed resources referenced by a composite resource.
func (i *composedResourceInformers) cleanupComposedResourceInformers(ctx context.Context) {
	crds := extv1.CustomResourceDefinitionList{}
	if err := i.cluster.GetClient().List(ctx, &crds); err != nil {
		i.log.Debug(errListCRDs, "error", err)
		return
	}

	// copy map to avoid locking it for the entire duration of the loop
	xrCaches := make(map[schema.GroupVersionKind]cache.Cache, len(i.xrCaches))
	i.lock.RLock()
	for gvk, ca := range i.xrCaches {
		xrCaches[gvk] = ca
	}
	i.lock.RUnlock()

	// find all CRDs that some XR is referencing. This is O(CRDs * XRDs * versions).
	// In practice, CRDs are 1000ish max, and XRDs are 10ish. So this is
	// fast enough for now. It's all in-memory.
	referenced := make(map[schema.GroupVersionKind]bool)
	for _, crd := range crds.Items {
		crd := crd

		if !xcrd.IsEstablished(crd.Status) {
			continue
		}

		for _, v := range crd.Spec.Versions {
			cdGVK := schema.GroupVersionKind{Group: crd.Spec.Group, Version: v.Name, Kind: crd.Spec.Names.Kind}
			for xrGVK, xrCache := range xrCaches {
				// list composites that reference this composed GVK
				list := kunstructured.UnstructuredList{}
				list.SetGroupVersionKind(xrGVK.GroupVersion().WithKind(xrGVK.Kind + "List"))
				if err := xrCache.List(ctx, &list, client.MatchingFields{compositeResourceRefGVKsIndex: cdGVK.String()}); err != nil {
					i.log.Debug("cannot list composite resources referencing a certain composed resource GVK", "error", err, "gvk", xrGVK.String(), "fieldSelector", compositeResourceRefGVKsIndex+"="+cdGVK.String())
					continue
				}
				if len(list.Items) > 0 {
					referenced[cdGVK] = true
				}
			}
		}
	}

	// stop old informers
	for gvk, inf := range i.cdCaches {
		if referenced[gvk] {
			continue
		}
		inf.cancelFn()
		i.gvkRoutedCache.RemoveDelegate(gvk)
		i.log.Info("Stopped composed resource watch", "gvk", gvk.String())
		delete(i.cdCaches, gvk)
	}
}

func parseAPIVersion(v string) (string, string) {
	parts := strings.SplitN(v, "/", 2)
	switch len(parts) {
	case 1:
		return "", parts[0]
	case 2:
		return parts[0], parts[1]
	default:
		return "", ""
	}
}
