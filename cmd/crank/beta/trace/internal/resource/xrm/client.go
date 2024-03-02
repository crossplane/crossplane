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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// Client to get a Resource with all its children.
type Client struct {
	getConnectionSecrets     bool
	setKnownResourceChildren bool

	client client.Client
}

// ResourceClientOption is a functional option for a Client.
type ResourceClientOption func(*Client)

// WithConnectionSecrets is a functional option that sets the client to get secrets to the desired value.
func WithConnectionSecrets(v bool) ResourceClientOption {
	return func(c *Client) {
		c.getConnectionSecrets = v
	}
}

// WithKnownResourceChildren is a functional option that sets the client to get known resource children to the desired value.
func WithKnownResourceChildren(v bool) ResourceClientOption {
	return func(c *Client) {
		c.setKnownResourceChildren = v
	}
}

// NewClient returns a new Client.
func NewClient(in client.Client, opts ...ResourceClientOption) (*Client, error) {
	uClient := xpunstructured.NewClient(in)

	c := &Client{
		client: uClient,
	}

	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// GetResourceTree returns the requested Crossplane Resource and all its children.
func (kc *Client) GetResourceTree(ctx context.Context, root *resource.Resource) (*resource.Resource, error) {
	// Set up a FIFO queue to traverse the resource tree breadth first.
	queue := []*resource.Resource{root}

	for len(queue) > 0 {
		// Pop the first element from the queue.
		res := queue[0]
		queue = queue[1:]

		if kc.setKnownResourceChildren {
			// We didn't get any reference for this resource, so either it's an
			// MR or an XR with no composed resources (yet).
			// In the former case, we could still want to show some useful info
			// as children.
			// NOTE(phisco): We don't want to actually fetch them in this case,
			//   as these could either not exist at all, e.g. some kind of fake
			//   placeholder resource, or just be unreachable, e.g. in a
			//   different Kubernetes cluster. So we just add them as children
			//   instead of adding them to the refs below.
			res.Children = getKnownResourceChildren(res)
		}

		refs := getResourceChildrenRefs(res, kc.getConnectionSecrets)

		for i := range refs {
			child := resource.GetResource(ctx, kc.client, &refs[i])

			res.Children = append(res.Children, child)
			queue = append(queue, child)
		}
	}

	return root, nil
}

// getKnownResourceChildren returns children for known resources.
func getKnownResourceChildren(res *resource.Resource) (children []*resource.Resource) {
	gvk := res.Unstructured.GroupVersionKind()
	switch {
	// provider-kubernetes Objects, we want to show the manifest as a child
	case gvk.GroupKind() == schema.GroupKind{Group: "kubernetes.crossplane.io", Kind: "Object"}:
		o := map[string]interface{}{}
		p := fieldpath.Pave(res.Unstructured.Object)
		err := p.GetValueInto("status.atProvider.manifest", &o)
		if err != nil {
			// If there is no status, we fall back at the spec definition
			err = p.GetValueInto("spec.forProvider.manifest", &o)
		}
		if err == nil {
			children = append(children, &resource.Resource{Unstructured: unstructured.Unstructured{Object: o}})
		}
	default:
		// not a known resource, so we don't have any children
		return nil
	}
	return children
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
