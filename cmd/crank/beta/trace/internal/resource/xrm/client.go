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

// Package xrm contains the client to get a Crossplane resource with all its
// children as a tree of Resource.
package xrm

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// defaultConcurrency is the concurrency using which the resource tree if loaded when not explicitly specified.
const defaultConcurrency = 5

// Client to get a Resource with all its children.
type Client struct {
	getConnectionSecrets bool

	client      client.Client
	concurrency int
}

// ResourceClientOption is a functional option for a Client.
type ResourceClientOption func(*Client)

// WithConnectionSecrets is a functional option that sets the client to get secrets to the desired value.
func WithConnectionSecrets(v bool) ResourceClientOption {
	return func(c *Client) {
		c.getConnectionSecrets = v
	}
}

// WithConcurrency is a functional option that sets the concurrency for the resource load.
func WithConcurrency(n int) ResourceClientOption {
	return func(c *Client) {
		c.concurrency = n
	}
}

// NewClient returns a new Client.
func NewClient(in client.Client, opts ...ResourceClientOption) (*Client, error) {
	uClient := xpunstructured.NewClient(in)

	c := &Client{
		client:      uClient,
		concurrency: defaultConcurrency,
	}

	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// GetResourceTree returns the requested Crossplane Resource and all its children.
func (kc *Client) GetResourceTree(ctx context.Context, root *resource.Resource) (*resource.Resource, error) {
	q := newLoader(root, kc, defaultChannelCapacity)
	q.load(ctx, kc.concurrency)
	return root, nil
}

// loadResource returns the resource for the specified object reference.
func (kc *Client) loadResource(ctx context.Context, ref *v1.ObjectReference) *resource.Resource {
	return resource.GetResource(ctx, kc.client, ref)
}

// getResourceChildrenRefs returns the references to the children for the given
// Resource, assuming it's a Crossplane resource, XR or XRC.
func (kc *Client) getResourceChildrenRefs(r *resource.Resource) []v1.ObjectReference {
	return getResourceChildrenRefs(r, kc.getConnectionSecrets)
}

// getResourceChildrenRefs returns the references to the children for the given
// Resource, assuming it's a Crossplane resource, XR or XRC.
func getResourceChildrenRefs(r *resource.Resource, getConnectionSecrets bool) []v1.ObjectReference {
	obj := r.Unstructured
	// collect object references for the
	var refs []v1.ObjectReference

	switch obj.GroupVersionKind().GroupKind() {
	case schema.GroupKind{Group: "", Kind: "Secret"},
		v1alpha1.UsageGroupVersionKind.GroupKind(),
		v1alpha1.EnvironmentConfigGroupVersionKind.GroupKind():
		// nothing to do here, it's a resource we know not to have any reference
		return nil
	}

	if xrcNamespace := obj.GetNamespace(); xrcNamespace != "" {
		// This is an XRC, get the XR ref, we leave the connection secret
		// handling to the XR
		xrc := claim.Unstructured{Unstructured: obj}
		if ref := xrc.GetResourceReference(); ref != nil {
			refs = append(refs, v1.ObjectReference{
				APIVersion: ref.APIVersion,
				Kind:       ref.Kind,
				Name:       ref.Name,
				Namespace:  ref.Namespace,
			})
		}
		if getConnectionSecrets {
			xrcSecretRef := xrc.GetWriteConnectionSecretToReference()
			if xrcSecretRef != nil {
				ref := v1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       xrcSecretRef.Name,
					Namespace:  xrcNamespace,
				}
				refs = append(refs, ref)
			}
		}
		return refs
	}
	// This could be an XR or an MR
	xr := composite.Unstructured{Unstructured: obj}
	xrRefs := xr.GetResourceReferences()
	if len(xrRefs) == 0 {
		// This is an MR
		return refs
	}
	// This is an XR, get the Composed resources refs and the
	// connection secret if required
	refs = append(refs, xrRefs...)

	if !getConnectionSecrets {
		// We don't need the connection secret, so we can stop here
		return refs
	}
	xrSecretRef := xr.GetWriteConnectionSecretToReference()
	if xrSecretRef != nil {
		ref := v1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Name:       xrSecretRef.Name,
			Namespace:  xrSecretRef.Namespace,
		}
		refs = append(refs, ref)
	}

	return refs
}
