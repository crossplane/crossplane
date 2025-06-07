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

// Package unstructured contains utilities unstructured Kubernetes objects.
package unstructured

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Wrapper returns the underlying *unstructured.Unstructured.
type Wrapper interface {
	GetUnstructured() *unstructured.Unstructured
}

// ListWrapper allows the *unstructured.UnstructuredList to be accessed.
type ListWrapper interface {
	GetUnstructuredList() *unstructured.UnstructuredList
}

// NewClient returns a client.Client that will operate on the underlying
// *unstructured.Unstructured if the object satisfies the Wrapper or ListWrapper
// interfaces. It relies on *unstructured.Unstructured instead of simpler
// map[string]any to avoid unnecessary copying.
func NewClient(c client.Client) *WrapperClient {
	return &WrapperClient{kube: c}
}

// A WrapperClient is a client.Client that will operate on the underlying
// *unstructured.Unstructured if the object satisfies the Wrapper or ListWrapper
// interfaces.
type WrapperClient struct {
	kube client.Client
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server.
func (c *WrapperClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Get(ctx, key, u.GetUnstructured(), opts...)
	}
	return c.kube.Get(ctx, key, obj, opts...)
}

// List retrieves list of objects for a given namespace and list options. On a
// successful call, Items field in the list will be populated with the
// result returned from the server.
func (c *WrapperClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if u, ok := list.(ListWrapper); ok {
		return c.kube.List(ctx, u.GetUnstructuredList(), opts...)
	}
	return c.kube.List(ctx, list, opts...)
}

// Create saves the object obj in the Kubernetes cluster.
func (c *WrapperClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Create(ctx, u.GetUnstructured(), opts...)
	}
	return c.kube.Create(ctx, obj, opts...)
}

// Delete deletes the given obj from Kubernetes cluster.
func (c *WrapperClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Delete(ctx, u.GetUnstructured(), opts...)
	}
	return c.kube.Delete(ctx, obj, opts...)
}

// Update updates the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (c *WrapperClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Update(ctx, u.GetUnstructured(), opts...)
	}
	return c.kube.Update(ctx, obj, opts...)
}

// Patch patches the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (c *WrapperClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Patch(ctx, u.GetUnstructured(), patch, opts...)
	}
	return c.kube.Patch(ctx, obj, patch, opts...)
}

// DeleteAllOf deletes all objects of the given type matching the given options.
func (c *WrapperClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.DeleteAllOf(ctx, u.GetUnstructured(), opts...)
	}
	return c.kube.DeleteAllOf(ctx, obj, opts...)
}

// Status returns a client for the Status subresource.
func (c *WrapperClient) Status() client.StatusWriter {
	return &wrapperStatusClient{
		kube: c.kube.Status(),
	}
}

// SubResource returns the underlying client's SubResource client, unwrapped.
func (c *WrapperClient) SubResource(subResource string) client.SubResourceClient {
	// TODO(negz): Is there anything to wrap here?
	return c.kube.SubResource(subResource)
}

// Scheme returns the scheme this client is using.
func (c *WrapperClient) Scheme() *runtime.Scheme {
	return c.kube.Scheme()
}

// RESTMapper returns the rest this client is using.
func (c *WrapperClient) RESTMapper() meta.RESTMapper {
	return c.kube.RESTMapper()
}

// GroupVersionKindFor returns the GVK for the given obj.
func (c *WrapperClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.GroupVersionKindFor(u.GetUnstructured())
	}
	return c.kube.GroupVersionKindFor(obj)
}

// IsObjectNamespaced checks whether the object is namespaced.
func (c *WrapperClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.IsObjectNamespaced(u.GetUnstructured())
	}
	return c.kube.IsObjectNamespaced(obj)
}

type wrapperStatusClient struct {
	kube client.StatusWriter
}

// Create creates the fields corresponding to the status subresource for the
// given obj. obj must be a struct pointer so that obj can be updated
// with the content returned by the Server.
func (c *wrapperStatusClient) Create(ctx context.Context, obj, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	// TODO(negz): Could subResource be wrapped?
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Create(ctx, u.GetUnstructured(), subResource, opts...)
	}
	return c.kube.Create(ctx, obj, subResource, opts...)
}

// Update updates the fields corresponding to the status subresource for the
// given obj. obj must be a struct pointer so that obj can be updated
// with the content returned by the Server.
func (c *wrapperStatusClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Update(ctx, u.GetUnstructured(), opts...)
	}
	return c.kube.Update(ctx, obj, opts...)
}

// Patch patches the given object's subresource. obj must be a struct
// pointer so that obj can be updated with the content returned by the
// Server.
func (c *wrapperStatusClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if u, ok := obj.(Wrapper); ok {
		return c.kube.Patch(ctx, u.GetUnstructured(), patch, opts...)
	}
	return c.kube.Patch(ctx, obj, patch, opts...)
}
