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

package controller

import (
	"context"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// GVKRoutedCache is a cache that routes requests by GVK to other caches.
type GVKRoutedCache struct {
	scheme *runtime.Scheme

	fallback cache.Cache

	lock      sync.RWMutex
	delegates map[schema.GroupVersionKind]cache.Cache
}

// NewGVKRoutedCache returns a new routed cache.
func NewGVKRoutedCache(scheme *runtime.Scheme, fallback cache.Cache) *GVKRoutedCache {
	return &GVKRoutedCache{
		scheme:    scheme,
		fallback:  fallback,
		delegates: make(map[schema.GroupVersionKind]cache.Cache),
	}
}

var _ cache.Cache = &GVKRoutedCache{}

// AddDelegate adds a delegated cache for a given GVK.
func (c *GVKRoutedCache) AddDelegate(gvk schema.GroupVersionKind, delegate cache.Cache) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.delegates[gvk] = delegate
}

// RemoveDelegate removes a delegated cache for a given GVK.
func (c *GVKRoutedCache) RemoveDelegate(gvk schema.GroupVersionKind) {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.delegates, gvk)
}

// Get retrieves an object for a given ObjectKey backed by a cache.
func (c *GVKRoutedCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return errors.Errorf("failed to get GVK for type %T: %w", obj, err)
	}

	c.lock.RLock()
	delegate, ok := c.delegates[gvk]
	c.lock.RUnlock()

	if ok {
		return delegate.Get(ctx, key, obj, opts...)
	}

	return c.fallback.Get(ctx, key, obj, opts...)
}

// List lists objects for a given ObjectList backed by a cache.
func (c *GVKRoutedCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvk, err := apiutil.GVKForObject(list, c.scheme)
	if err != nil {
		return errors.Errorf("failed to get GVK for type %T: %w", list, err)
	}

	if !strings.HasSuffix(gvk.Kind, "List") {
		// following controller-runtime here which does not support non
		// <Kind>List types.
		return errors.Errorf("non-list type %T (kind %q) passed as output", list, gvk)
	}
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

	c.lock.RLock()
	delegate, ok := c.delegates[gvk]
	c.lock.RUnlock()

	if ok {
		return delegate.List(ctx, list, opts...)
	}

	return c.fallback.List(ctx, list, opts...)
}

// GetInformer returns an informer for the given object.
func (c *GVKRoutedCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return nil, errors.Errorf("failed to get GVK for type %T: %w", obj, err)
	}

	c.lock.RLock()
	delegate, ok := c.delegates[gvk]
	c.lock.RUnlock()

	if ok {
		return delegate.GetInformer(ctx, obj, opts...)
	}

	return c.fallback.GetInformer(ctx, obj, opts...)
}

// GetInformerForKind returns an informer for the given GVK.
func (c *GVKRoutedCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	c.lock.RLock()
	delegate, ok := c.delegates[gvk]
	c.lock.RUnlock()

	if ok {
		return delegate.GetInformerForKind(ctx, gvk, opts...)
	}

	return c.fallback.GetInformerForKind(ctx, gvk, opts...)
}

// RemoveInformer removes an informer entry and stops it if it was running.
func (c *GVKRoutedCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return errors.Errorf("failed to get GVK for type %T: %w", obj, err)
	}

	c.lock.RLock()
	delegate, ok := c.delegates[gvk]
	c.lock.RUnlock()

	if ok {
		return delegate.RemoveInformer(ctx, obj)
	}

	return c.fallback.RemoveInformer(ctx, obj)
}

// Start for a GVKRoutedCache is a no-op. Start must be called for each delegate.
func (c *GVKRoutedCache) Start(_ context.Context) error {
	return nil
}

// WaitForCacheSync for a GVKRoutedCache waits for all delegates and the
// fallback to sync, and returns false if any of them fails to sync.
func (c *GVKRoutedCache) WaitForCacheSync(ctx context.Context) bool {
	c.lock.RLock()
	syncedCh := make(chan bool, len(c.delegates)+1)
	cas := make([]cache.Cache, 0, len(c.delegates))
	for _, ca := range c.delegates {
		cas = append(cas, ca)
	}
	cas = append(cas, c.fallback)
	c.lock.RUnlock()

	var wg sync.WaitGroup
	ctx, cancelFn := context.WithCancel(ctx)

	for _, ca := range cas {
		wg.Add(1)
		go func(ca cache.Cache) {
			defer wg.Done()
			synced := ca.WaitForCacheSync(ctx)
			if !synced {
				// first unsynced cache breaks the whole wait
				cancelFn()
			}
			syncedCh <- synced
		}(ca)
	}

	wg.Wait()
	close(syncedCh)
	cancelFn()

	// any not synced?
	for synced := range syncedCh {
		if !synced {
			return false
		}
	}

	return c.fallback.WaitForCacheSync(ctx)
}

// IndexField adds an index with the given field name on the given object type
// by using the given function to extract the value for that field.
func (c *GVKRoutedCache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return errors.Errorf("failed to get GVK for type %T: %w", obj, err)
	}

	c.lock.RLock()
	delegate, ok := c.delegates[gvk]
	c.lock.RUnlock()

	if ok {
		return delegate.IndexField(ctx, obj, field, extractValue)
	}

	return c.fallback.IndexField(ctx, obj, field, extractValue)
}

// cachedRoutedClient wraps a client and routes read requests by GVK to a cache.
type cachedRoutedClient struct {
	client.Client

	scheme *runtime.Scheme
	cache  *GVKRoutedCache
}

func (c *cachedRoutedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return errors.Errorf("failed to get GVK for type %T: %w", obj, err)
	}

	c.cache.lock.RLock()
	delegate, ok := c.cache.delegates[gvk]
	c.cache.lock.RUnlock()

	if ok {
		return delegate.Get(ctx, key, obj, opts...)
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *cachedRoutedClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvk, err := apiutil.GVKForObject(list, c.scheme)
	if err != nil {
		return errors.Errorf("failed to get GVK for type %T: %w", list, err)
	}

	if !strings.HasSuffix(gvk.Kind, "List") {
		// following controller-runtime here which does not support non
		// <Kind>List types.
		return errors.Errorf("non-list type %T (kind %q) passed as output", list, gvk)
	}
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

	c.cache.lock.RLock()
	delegate, ok := c.cache.delegates[gvk]
	c.cache.lock.RUnlock()

	if ok {
		return delegate.List(ctx, list, opts...)
	}

	return c.Client.List(ctx, list, opts...)
}

// WithGVKRoutedCache returns a manager backed by a GVKRoutedCache. The client
// returned by the manager will route read requests to cached GVKs.
func WithGVKRoutedCache(c *GVKRoutedCache, mgr controllerruntime.Manager) controllerruntime.Manager {
	return &routedManager{
		Manager: mgr,
		client: &cachedRoutedClient{
			Client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
			cache:  c,
		},
		cache: c,
	}
}

type routedManager struct {
	controllerruntime.Manager

	client client.Client
	cache  cache.Cache
}

func (m *routedManager) GetClient() client.Client {
	return m.client
}

func (m *routedManager) GetCache() cache.Cache {
	return m.cache
}
