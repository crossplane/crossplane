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

package engine

import (
	"context"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

var (
	_ cache.Cache       = &InformerTrackingCache{}
	_ TrackingInformers = &InformerTrackingCache{}
)

// An InformerTrackingCache wraps a cache.Cache and keeps track of what GVKs it
// has started informers for. It takes a blocking lock whenever a new informer
// is started or stopped, but so does the standard controller-runtime Cache
// implementation.
type InformerTrackingCache struct {
	// The wrapped cache.
	cache.Cache

	scheme *runtime.Scheme

	mx     sync.RWMutex
	active map[schema.GroupVersionKind]bool
}

// TrackInformers wraps the supplied cache, adding a method to query which
// informers are active.
func TrackInformers(c cache.Cache, s *runtime.Scheme) *InformerTrackingCache {
	return &InformerTrackingCache{
		Cache:  c,
		scheme: s,
		active: make(map[schema.GroupVersionKind]bool),
	}
}

// ActiveInformers returns the GVKs of the informers believed to currently be
// active. The InformerTrackingCache considers an informer to become active when
// a caller calls Get, List, or one of the GetInformer methods. It considers an
// informer to become inactive when a caller calls the RemoveInformer method.
func (c *InformerTrackingCache) ActiveInformers() []schema.GroupVersionKind {
	c.mx.RLock()
	defer c.mx.RUnlock()

	out := make([]schema.GroupVersionKind, 0, len(c.active))
	for gvk := range c.active {
		out = append(out, gvk)
	}
	return out
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server.
//
// Getting an object marks the informer for the object's GVK active.
func (c *InformerTrackingCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return errors.Wrap(err, "cannot determine group, version, and kind of supplied object")
	}

	c.mx.RLock()
	if _, active := c.active[gvk]; active {
		defer c.mx.RUnlock()
		return c.Cache.Get(ctx, key, obj, opts...)
	}
	c.mx.RUnlock()

	c.mx.Lock()
	defer c.mx.Unlock()
	c.active[gvk] = true
	return c.Cache.Get(ctx, key, obj, opts...)
}

// List retrieves list of objects for a given namespace and list options. On a
// successful call, Items field in the list will be populated with the result
// returned from the server.
//
// Listing objects marks the informer for the object's GVK active.
func (c *InformerTrackingCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvk, err := apiutil.GVKForObject(list, c.scheme)
	if err != nil {
		return errors.Wrap(err, "cannot determine group, version, and kind of supplied object")
	}
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

	c.mx.RLock()
	if _, active := c.active[gvk]; active {
		defer c.mx.RUnlock()
		return c.Cache.List(ctx, list, opts...)
	}
	c.mx.RUnlock()

	c.mx.Lock()
	defer c.mx.Unlock()
	c.active[gvk] = true
	return c.Cache.List(ctx, list, opts...)
}

// GetInformer fetches or constructs an informer for the given object that
// corresponds to a single API kind and resource.
//
// Getting an informer for an object marks the informer as active.
func (c *InformerTrackingCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return nil, errors.Wrap(err, "cannot determine group, version, and kind of supplied object")
	}

	c.mx.RLock()
	if _, active := c.active[gvk]; active {
		defer c.mx.RUnlock()
		return c.Cache.GetInformer(ctx, obj, opts...)
	}
	c.mx.RUnlock()

	c.mx.Lock()
	defer c.mx.Unlock()
	c.active[gvk] = true
	return c.Cache.GetInformer(ctx, obj, opts...)
}

// GetInformerForKind is similar to GetInformer, except that it takes a
// group-version-kind, instead of the underlying object.
//
// Getting an informer marks the informer as active.
func (c *InformerTrackingCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	c.mx.RLock()
	if _, active := c.active[gvk]; active {
		defer c.mx.RUnlock()
		return c.Cache.GetInformerForKind(ctx, gvk, opts...)
	}
	c.mx.RUnlock()

	c.mx.Lock()
	defer c.mx.Unlock()
	c.active[gvk] = true
	return c.Cache.GetInformerForKind(ctx, gvk, opts...)
}

// RemoveInformer removes an informer entry and stops it if it was running.
//
// Removing an informer marks the informer as inactive.
func (c *InformerTrackingCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return errors.Wrap(err, "cannot determine group, version, and kind of supplied object")
	}

	c.mx.RLock()
	if _, active := c.active[gvk]; !active {
		// This should only happen if RemoveInformer is called for an informer
		// that was never started.
		defer c.mx.RUnlock()
		return c.Cache.RemoveInformer(ctx, obj)
	}
	c.mx.RUnlock()

	c.mx.Lock()
	defer c.mx.Unlock()
	delete(c.active, gvk)
	return c.Cache.RemoveInformer(ctx, obj)
}
