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

package engine

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// InvalidatableClient is a client.Client that supports cache invalidation.
// This is useful when CRDs are deleted and recreated with different properties
// (like scope changes) and we need to ensure the client reflects the updated
// CRD definition.
type InvalidatableClient struct {
	mu     sync.RWMutex
	client client.Client
	cfg    *rest.Config
	opts   client.Options
}

// NewInvalidatableClient creates a new InvalidatableClient that wraps a
// controller-runtime client with cache invalidation capabilities.
func NewInvalidatableClient(cfg *rest.Config, opts client.Options) (*InvalidatableClient, error) {
	c, err := client.New(cfg, opts)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create client")
	}

	return &InvalidatableClient{
		client: c,
		cfg:    cfg,
		opts:   opts,
	}, nil
}

// Invalidate invalidates both the client's resourceMeta cache and its RESTMapper
// cache by recreating the underlying client. The cache layer (if any) is preserved
// since it's passed through the client options.
func (c *InvalidatableClient) Invalidate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// First try to invalidate the RESTMapper if it supports it
	if inv, ok := c.opts.Mapper.(Invalidatable); ok {
		if err := inv.Invalidate(); err != nil {
			return errors.Wrap(err, "cannot invalidate client REST mapper")
		}
	}

	// Recreate the underlying client to get fresh resourceMeta cache
	client, err := client.New(c.cfg, c.opts)
	if err != nil {
		return errors.Wrap(err, "cannot recreate client for cache invalidation")
	}

	c.client = client
	return nil
}

// The following methods implement client.Client by delegating to the underlying client.
// We use a read lock for all operations to allow concurrent access.

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
func (c *InvalidatableClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.Get(ctx, key, obj, opts...)
}

// List retrieves list of objects for a given namespace and list options.
func (c *InvalidatableClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.List(ctx, list, opts...)
}

// Create saves the object obj in the Kubernetes cluster.
func (c *InvalidatableClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.Create(ctx, obj, opts...)
}

// Delete deletes the given obj from Kubernetes cluster.
func (c *InvalidatableClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.Delete(ctx, obj, opts...)
}

// Update updates the given obj in the Kubernetes cluster.
func (c *InvalidatableClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.Update(ctx, obj, opts...)
}

// Patch patches the given obj in the Kubernetes cluster.
func (c *InvalidatableClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.Patch(ctx, obj, patch, opts...)
}

// DeleteAllOf deletes all objects of the given type matching the given options.
func (c *InvalidatableClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

// Status returns a client which can update status subresource.
func (c *InvalidatableClient) Status() client.StatusWriter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.Status()
}

// SubResource returns a client which can update the specified subresource.
func (c *InvalidatableClient) SubResource(subResource string) client.SubResourceClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.SubResource(subResource)
}

// Scheme returns the scheme this client is using.
func (c *InvalidatableClient) Scheme() *runtime.Scheme {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.Scheme()
}

// RESTMapper returns the rest this client is using.
func (c *InvalidatableClient) RESTMapper() meta.RESTMapper {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.RESTMapper()
}

// GroupVersionKindFor returns the GroupVersionKind for the given object.
func (c *InvalidatableClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.GroupVersionKindFor(obj)
}

// IsObjectNamespaced returns true if the GroupVersionKind of the object is namespaced.
func (c *InvalidatableClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.IsObjectNamespaced(obj)
}
