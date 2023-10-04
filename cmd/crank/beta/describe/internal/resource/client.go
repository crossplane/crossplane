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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

const (
	errCouldntGetRootResource         = "couldn't get root resource"
	errCouldntGetChildResource        = "couldn't get child resource"
	errCouldntGetRESTMapping          = "couldn't get REST mapping for resource"
	errCouldntGetResource             = "couldn't get resource"
	errCouldntGetEventForResource     = "couldn't get event for resource"
	errCouldntGetEventListForResource = "couldn't get event list for resource"
	errFmtResourceTypeNotFound        = "the server doesn't have a resource type %q"
)

// Client to get a Resource with all its children and latest events.
type Client struct {
	dynClient       *dynamic.DynamicClient
	clientset       *kubernetes.Clientset
	rmapper         meta.RESTMapper
	discoveryClient *discovery.DiscoveryClient
}

// GetResourceTree returns the requested Resource and all its children, with
// latest events for each of them, if any.
func (kc *Client) GetResourceTree(ctx context.Context, rootRef *v1.ObjectReference) (*Resource, error) {
	// Get the root resource
	root, err := kc.getResource(ctx, rootRef)
	if err != nil {
		return nil, errors.Wrap(err, errCouldntGetRootResource)
	}

	// breadth-first search of children
	queue := []*Resource{root}

	for len(queue) > 0 {
		res := queue[0]   // Take the first element
		queue = queue[1:] // Dequeue the first element

		refs := getResourceChildrenRefs(res)
		if err != nil {
			return nil, errors.Wrap(err, errCouldntGetRootResource)
		}
		for i := range refs {
			child, err := kc.getResource(ctx, &refs[i])
			if err != nil {
				return nil, errors.Wrap(err, errCouldntGetChildResource)
			}
			res.Children = append(res.Children, child)
			queue = append(queue, child) // Enqueue child
		}
	}

	return root, nil
}

// getResource returns the requested Resource with the latest event set.
func (kc *Client) getResource(ctx context.Context, ref *v1.ObjectReference) (*Resource, error) {
	rm, err := kc.rmapper.RESTMapping(ref.GroupVersionKind().GroupKind(), ref.GroupVersionKind().Version)
	if err != nil {
		return nil, errors.Wrap(err, errCouldntGetRESTMapping)
	}

	result, err := kc.dynClient.Resource(rm.Resource).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, errCouldntGetResource)
	}
	// Get event
	event, err := kc.getLatestEvent(ctx, *ref)
	if err != nil {
		return nil, errors.Wrap(err, errCouldntGetEventForResource)
	}

	res := &Resource{Unstructured: *result, LatestEvent: event}
	return res, nil
}

// getResourceChildrenRefs returns the references to the children for the given
// Resource, assuming it's a Crossplane resource, XR or XRC.
func getResourceChildrenRefs(r *Resource) []v1.ObjectReference {
	obj := r.Unstructured
	// collect owner references
	var refs []v1.ObjectReference

	xr := composite.Unstructured{Unstructured: obj}
	refs = append(refs, xr.GetResourceReferences()...)

	xrc := claim.Unstructured{Unstructured: obj}
	if ref := xrc.GetResourceReference(); ref != nil {
		refs = append(refs, v1.ObjectReference{
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Name:       ref.Name,
			Namespace:  ref.Namespace,
			UID:        ref.UID,
		})
	}
	return refs
}

// The getLatestEvent returns the latest Event for the given resource reference.
func (kc *Client) getLatestEvent(ctx context.Context, ref v1.ObjectReference) (*v1.Event, error) {
	// List events for the resource.
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s,involvedObject.apiVersion=%s", ref.Name, ref.Kind, ref.APIVersion)
	if ref.UID != "" {
		fieldSelector = fmt.Sprintf("%s,involvedObject.uid=%s", fieldSelector, ref.UID)
	}
	eventList, err := kc.clientset.CoreV1().Events(ref.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, errCouldntGetEventListForResource)
	}

	// Check if there are any events.
	if len(eventList.Items) == 0 {
		return nil, nil
	}

	latestEvent := eventList.Items[0]
	for _, event := range eventList.Items {
		if event.LastTimestamp.After(latestEvent.LastTimestamp.Time) {
			latestEvent = event
		}
	}

	// Get the latest event.
	return &latestEvent, nil
}

// MappingFor returns the RESTMapping for the given resource or kind argument.
// Copied over from cli-runtime pkg/resource Builder.
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

// NewClient returns a new Client.
func NewClient(config *rest.Config) (*Client, error) {
	// Use to get custom resources
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}

	// Use to discover API resources
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	rmapper, err := apiutil.NewDynamicRESTMapper(config, httpClient)
	if err != nil {
		return nil, err
	}

	return &Client{
		dynClient:       dynClient,
		clientset:       clientset,
		rmapper:         rmapper,
		discoveryClient: discoveryClient,
	}, nil
}
