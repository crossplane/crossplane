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

package funcs

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// coordinate represents a coordinate in the cluster, i.e. everything necessary
// to identify an object and access it.
type coordinate struct {
	schema.GroupVersionResource

	// Namespace of the object. Empty string for cluster-scoped objects.
	Namespace string
	// Name of the object.
	Name string
}

// buildOwnershipGraph builds a graph of Kubernetes objects and their owners,
// with the owners as the keys and the objects they own as the values.
func buildOwnershipGraph(ctx context.Context, discoveryClient discovery.DiscoveryInterface, client metadata.Interface, mapper meta.RESTMapper) (map[coordinate][]coordinate, error) {
	// Discover all resource types
	resourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	// list all objects of each resource type and store their owners
	g := make(map[coordinate][]coordinate)
	for _, resourceList := range resourceLists {
		for _, resource := range resourceList.APIResources {
			group, version := parseAPIVersion(resourceList.GroupVersion)
			gvr := schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource.Name,
			}

			// list all objects of the current resource type
			objs, err := client.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				// skip over resource types we can't access
				continue
			}

			for _, obj := range objs.Items {
				ownerRefs := obj.GetOwnerReferences()
				this := coordinate{
					GroupVersionResource: gvr,
					Namespace:            obj.GetNamespace(),
					Name:                 obj.GetName(),
				}

				for _, ownerRef := range ownerRefs {
					group, version := parseAPIVersion(ownerRef.APIVersion)
					rm, err := mapper.RESTMapping(schema.GroupKind{Group: group, Kind: ownerRef.Kind}, version)
					if err != nil {
						fmt.Printf("cannot find REST mapping for %v: %v\n", ownerRef, err)
						continue
					}
					owner := coordinate{
						GroupVersionResource: rm.Resource,
						Namespace:            obj.GetNamespace(),
						Name:                 ownerRef.Name,
					}
					g[owner] = append(g[owner], this)
				}
			}
		}
	}

	return g, nil
}

// ParseAPIVersion takes an APIVersion string (e.g., "apps/v1" or "v1")
// and returns the API group and version as separate strings.
func parseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		// Core API, group is empty
		return "", parts[0]
	}
	return parts[0], parts[1]
}

// OwnedObjects returns all objects related to the supplied object through
// ownership, i.e. the returned objects are transitively owned by obj.
func OwnedObjects(ctx context.Context, config *rest.Config, objs ...client.Object) ([]client.Object, error) {
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	metadataClient, err := metadata.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(config, httpClient)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	ownershipGraph, err := buildOwnershipGraph(ctx, discoveryClient, metadataClient, mapper)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build ownership graph")
	}

	seen := make(map[coordinate]bool)
	coords := []coordinate{}
	for _, obj := range objs {
		gvk := obj.GetObjectKind().GroupVersionKind()
		rm, err := mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			fmt.Printf("cannot find REST mapping for %s: %v\n", gvk, err)
			continue
		}

		coords = append(coords, collectOwned(ownershipGraph, coordinate{
			GroupVersionResource: rm.Resource,
			Namespace:            obj.GetNamespace(),
			Name:                 obj.GetName(),
		}, seen)...)
	}

	return loadCoordinates(ctx, dynClient, coords), nil
}

func loadCoordinates(ctx context.Context, dynClient dynamic.Interface, coords []coordinate) []client.Object {
	ret := make([]client.Object, 0, len(coords))
	for _, coord := range coords {
		other, err := dynClient.Resource(coord.GroupVersionResource).Namespace(coord.Namespace).Get(ctx, coord.Name, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("cannot get %v: %v\n", coord, err)
			continue
		}
		ret = append(ret, other)
	}
	return ret
}

func collectOwned(g map[coordinate][]coordinate, owner coordinate, seen map[coordinate]bool) []coordinate {
	seen[owner] = true

	ret := []coordinate{}
	for _, obj := range g[owner] {
		if seen[obj] {
			continue
		}
		ret = append(ret, collectOwned(g, obj, seen)...)
		ret = append(ret, obj)
	}

	return ret
}
