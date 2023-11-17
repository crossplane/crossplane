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

package resource

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"
	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errFmtResourceTypeNotFound = "the server doesn't have a resource type %q"
)

// Client to get a Resource with all its children.
type Client struct {
	getConnectionSecrets bool

	client  client.Client
	rmapper meta.RESTMapper
}

// ClientOption is a functional option for a Client.
type ClientOption func(*Client)

// WithConnectionSecrets is a functional option that sets the client to get secrets to the desired value.
func WithConnectionSecrets(v bool) ClientOption {
	return func(c *Client) {
		c.getConnectionSecrets = v
	}
}

// NewClient returns a new Client.
func NewClient(in client.Client, rmapper meta.RESTMapper, opts ...ClientOption) (*Client, error) {
	uClient := xpunstructured.NewClient(in)

	c := &Client{
		client:  uClient,
		rmapper: rmapper,
	}

	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// GetResourceTree returns the requested Resource and all its children.
func (kc *Client) GetResourceTree(ctx context.Context, rootRef *v1.ObjectReference) (*Resource, error) {
	root := kc.getResource(ctx, rootRef)
	// We should just surface any error getting the root resource immediately.
	if err := root.Error; err != nil {
		return nil, err
	}

	// Set up a FIFO queue to traverse the resource tree breadth first.
	queue := []*Resource{root}

	for len(queue) > 0 {
		// Pop the first element from the queue.
		res := queue[0]
		queue = queue[1:]

		refs := getResourceChildrenRefs(res, kc.getConnectionSecrets)

		for i := range refs {
			child := kc.getResource(ctx, &refs[i])

			res.Children = append(res.Children, child)
			queue = append(queue, child)
		}
	}

	return root, nil
}

// getResource returns the requested Resource, setting any error as Resource.Error.
func (kc *Client) getResource(ctx context.Context, ref *v1.ObjectReference) *Resource {
	result := unstructured.Unstructured{}
	result.SetGroupVersionKind(ref.GroupVersionKind())

	err := kc.client.Get(ctx, xpmeta.NamespacedNameOf(ref), &result)

	if err != nil {
		// If the resource is not found, we still want to return a Resource
		// object with the name and namespace set, so that the caller can
		// still use it.
		result.SetName(ref.Name)
		result.SetNamespace(ref.Namespace)
	}
	return &Resource{Unstructured: result, Error: err}
}

// getResourceChildrenRefs returns the references to the children for the given
// Resource, assuming it's a Crossplane resource, XR or XRC.
func getResourceChildrenRefs(r *Resource, getConnectionSecrets bool) []v1.ObjectReference {
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

// MappingFor returns the RESTMapping for the given resource or kind argument.
// Copied over from cli-runtime pkg/resource Builder,
// https://github.com/kubernetes/cli-runtime/blob/9a91d944dd43186c52e0162e12b151b0e460354a/pkg/resource/builder.go#L768
func (kc *Client) MappingFor(resourceOrKindArg string) (*meta.RESTMapping, error) {
	// TODO(phisco): actually use the Builder.
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKindArg)
	gvk := schema.GroupVersionKind{}
	if fullySpecifiedGVR != nil {
		gvk, _ = kc.rmapper.KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		gvk, _ = kc.rmapper.KindFor(groupResource.WithVersion(""))
	}
	if !gvk.Empty() {
		return kc.rmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKindArg)
	if fullySpecifiedGVK == nil {
		gvk := groupKind.WithVersion("")
		fullySpecifiedGVK = &gvk
	}
	if !fullySpecifiedGVK.Empty() {
		if mapping, err := kc.rmapper.RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version); err == nil {
			return mapping, nil
		}
	}
	mapping, err := kc.rmapper.RESTMapping(groupKind, gvk.Version)
	if err != nil {
		// if we error out here, it is because we could not match a resource or a kind
		// for the given argument. To maintain consistency with previous behavior,
		// announce that a resource type could not be found.
		// if the error is _not_ a *meta.NoKindMatchError, then we had trouble doing discovery,
		// so we should return the original error since it may help a user diagnose what is actually wrong
		if meta.IsNoMatchError(err) {
			return nil, fmt.Errorf(errFmtResourceTypeNotFound, groupResource.Resource)
		}
		return nil, err
	}
	return mapping, nil
}
