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

package check

import (
	"context"
	"slices"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// managedResourceCategory is the CRD category every Crossplane provider
// applies to its managed resource CRDs.
const managedResourceCategory = "managed"

// DiscoveredType is an instance type of note that has been discovered on the control plane. The
// sources for these types are XRs, Claims, and MRs.
type DiscoveredType struct {
	GVK        schema.GroupVersionKind
	Namespaced bool
}

// DiscoverXRAndClaimTypes returns the GVKs of every XR and Claim type defined by
// an XRD on the cluster. Prefers each XRD's referenceable version, falling
// back to the first served one. The caller can optionally supply a list of XRDs
// if they have already fetched them, if the caller does not provide this list
// then this function will make the List call to fetch them.
func DiscoverXRAndClaimTypes(ctx context.Context, c client.Client, xrds *apiextensionsv1.CompositeResourceDefinitionList) ([]DiscoveredType, error) {
	if xrds == nil {
		// the caller has not supplied a pre-fetched list of XRDs, fetch them now
		xrds = &apiextensionsv1.CompositeResourceDefinitionList{}
		if err := c.List(ctx, xrds); err != nil {
			return nil, err
		}
	}

	types := make([]DiscoveredType, 0, len(xrds.Items)*2)
	for _, xrd := range xrds.Items {
		v := pickVersion(xrd)
		if v == "" {
			continue
		}
		types = append(types, DiscoveredType{
			GVK:        schema.GroupVersionKind{Group: xrd.Spec.Group, Version: v, Kind: xrd.Spec.Names.Kind},
			Namespaced: false,
		})
		if xrd.Spec.ClaimNames != nil {
			types = append(types, DiscoveredType{
				GVK:        schema.GroupVersionKind{Group: xrd.Spec.Group, Version: v, Kind: xrd.Spec.ClaimNames.Kind},
				Namespaced: true,
			})
		}
	}
	return types, nil
}

// pickVersion returns the XRD version to list instances against. Must be a
// served version - the apiserver silently returns nothing for non-served
// versions, which would hide resources from the check. Returns "" if no
// served version exists.
func pickVersion(xrd apiextensionsv1.CompositeResourceDefinition) string {
	// prefer the referenceable/served version
	for _, v := range xrd.Spec.Versions {
		if v.Referenceable && v.Served {
			return v.Name
		}
	}

	// we didn't find a referenceable version, fall back to the first served version
	for _, v := range xrd.Spec.Versions {
		if v.Served {
			return v.Name
		}
	}

	return ""
}

// ListInstances lists instances of the given discovered type. For namespaced types, a non-empty
// namespace restricts the list; empty lists across all namespaces. The namespace argument is
// ignored for cluster-scoped types.
func ListInstances(ctx context.Context, c client.Client, t DiscoveredType, namespace string) ([]unstructured.Unstructured, error) {
	listGVK := t.GVK
	listGVK.Kind += "List"

	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(listGVK)
	var opts []client.ListOption
	if t.Namespaced && namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}
	if err := c.List(ctx, u, opts...); err != nil {
		return nil, err
	}
	return u.Items, nil
}

// DiscoverManagedResources returns the GVKs of every CRD in the "managed"
// category, preferring each CRD's storage version and falling back to its
// first served version.
func DiscoverManagedResources(ctx context.Context, c client.Client) ([]DiscoveredType, error) {
	crds := &extv1.CustomResourceDefinitionList{}
	if err := c.List(ctx, crds); err != nil {
		return nil, err
	}
	types := make([]DiscoveredType, 0, len(crds.Items))
	for i := range crds.Items {
		crd := &crds.Items[i]
		if !slices.Contains(crd.Spec.Names.Categories, managedResourceCategory) {
			// not in the "managed" category, skip it
			continue
		}
		v := pickCRDVersion(crd)
		if v == "" {
			continue
		}
		types = append(types, DiscoveredType{
			GVK: schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: v,
				Kind:    crd.Spec.Names.Kind,
			},
			Namespaced: crd.Spec.Scope == extv1.NamespaceScoped,
		})
	}
	return types, nil
}

// pickCRDVersion returns the CRD version to list instances against. Must be
// a served version (see pickVersion). Returns "" if no served version exists.
func pickCRDVersion(crd *extv1.CustomResourceDefinition) string {
	// prefer the storage/served version
	for _, v := range crd.Spec.Versions {
		if v.Storage && v.Served {
			return v.Name
		}
	}

	// fall back to first served version
	for _, v := range crd.Spec.Versions {
		if v.Served {
			return v.Name
		}
	}
	return ""
}

// ResourceRefFromUnstructured builds a ResourceRef from an unstructured object.
func ResourceRefFromUnstructured(u unstructured.Unstructured) ResourceRef {
	return ResourceRef{
		Group:     u.GroupVersionKind().Group,
		Kind:      u.GetKind(),
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
	}
}
