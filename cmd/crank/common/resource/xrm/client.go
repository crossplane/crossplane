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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpunstructured "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/v2/cmd/crank/common/resource"
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

	switch obj.GroupVersionKind().GroupKind() {
	case schema.GroupKind{Group: "", Kind: "Secret"},
		v1alpha1.UsageGroupVersionKind.GroupKind(),
		v1beta1.EnvironmentConfigGroupVersionKind.GroupKind():
		// nothing to do here, it's a resource we know not to have any reference
		return nil
	}

	// collect object references for the
	var refs []v1.ObjectReference

	// treat it like a claim and look for a XR ref
	cm := claim.Unstructured{Unstructured: obj}
	if ref := cm.GetResourceReference(); ref != nil {
		// it is in fact a claim, grab the ref to its XR
		refs = append(refs, v1.ObjectReference{
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Name:       ref.Name,
			Namespace:  ptr.Deref(ref.Namespace, ""),
		})

		if getConnectionSecrets {
			// grab any connection secret from the claim if it has one
			if cmSecretRef := cm.GetWriteConnectionSecretToReference(); cmSecretRef != nil {
				ref := v1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       cmSecretRef.Name,
					Namespace:  cm.GetNamespace(),
				}
				refs = append(refs, ref)
			}
		}

		// we're done, the only ref a claim would have is to its XR
		return refs
	}

	// treat it like a modern XR then grab all the references (this will no-op
	// if it's not a modern XR). We don't try to get connection secrets here
	// because modern XRs don't support them.
	xr := composite.Unstructured{Schema: composite.SchemaModern, Unstructured: obj}
	refs = append(refs, xr.GetResourceReferences()...)

	// treat it like a legacy XR then grab all the references (this will no-op
	// if it's not a legacy XR), and any potential connection secret (only
	// legacy XRs have connection secrets).
	xr = composite.Unstructured{Schema: composite.SchemaLegacy, Unstructured: obj}
	refs = append(refs, xr.GetResourceReferences()...)

	if getConnectionSecrets {
		if xrSecretRef := xr.GetWriteConnectionSecretToReference(); xrSecretRef != nil {
			ref := v1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       xrSecretRef.Name,
				Namespace:  xrSecretRef.Namespace,
			}
			refs = append(refs, ref)
		}
	}

	if ns := obj.GetNamespace(); ns != "" {
		// the XR is namespaced, so it's references will not explicitly declare
		// their namespaces (they are implicit). We need to infer it from the XR so
		// we have a complete reference to return to the caller.
		for i := range refs {
			if refs[i].Namespace == "" {
				refs[i].Namespace = ns
			}
		}
	}

	return refs
}
