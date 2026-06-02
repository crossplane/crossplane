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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestPickXRDVersion(t *testing.T) {
	type want struct {
		version string
	}
	cases := map[string]struct {
		reason string
		xrd    apiextensionsv1.CompositeResourceDefinition
		want   want
	}{
		"PrefersReferenceableServed": {
			reason: "The referenceable and served version wins over other served versions.",
			xrd: apiextensionsv1.CompositeResourceDefinition{Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
				Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{
					{Name: "v1alpha1", Served: true, Referenceable: false},
					{Name: "v1", Served: true, Referenceable: true},
				},
			}},
			want: want{version: "v1"},
		},
		"FallsBackToFirstServed": {
			reason: "With no referenceable version, the first served version is used.",
			xrd: apiextensionsv1.CompositeResourceDefinition{Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
				Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{
					{Name: "v1alpha1", Served: false},
					{Name: "v1beta1", Served: true},
				},
			}},
			want: want{version: "v1beta1"},
		},
		"NoneServed": {
			reason: "When nothing is served there is no version to list against.",
			xrd: apiextensionsv1.CompositeResourceDefinition{Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
				Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{{Name: "v1", Served: false}},
			}},
			want: want{version: ""},
		},
		"ReferenceableButNotServedIsSkipped": {
			reason: "A referenceable-but-unserved version is ignored in favor of a served one.",
			xrd: apiextensionsv1.CompositeResourceDefinition{Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
				Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{
					{Name: "v1", Served: false, Referenceable: true},
					{Name: "v2", Served: true, Referenceable: false},
				},
			}},
			want: want{version: "v2"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := pickXRDVersion(tc.xrd)
			if diff := cmp.Diff(tc.want.version, got); diff != "" {
				t.Errorf("\n%s\npickVersion(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPickCRDVersion(t *testing.T) {
	type want struct {
		version string
	}
	cases := map[string]struct {
		reason string
		crd    extv1.CustomResourceDefinition
		want   want
	}{
		"PrefersStorageServed": {
			reason: "The storage+served version wins.",
			crd: extv1.CustomResourceDefinition{Spec: extv1.CustomResourceDefinitionSpec{
				Versions: []extv1.CustomResourceDefinitionVersion{
					{Name: "v1beta1", Served: true, Storage: false},
					{Name: "v1", Served: true, Storage: true},
				},
			}},
			want: want{version: "v1"},
		},
		"FallsBackToFirstServed": {
			reason: "With no storage version served, the first served version is used.",
			crd: extv1.CustomResourceDefinition{Spec: extv1.CustomResourceDefinitionSpec{
				Versions: []extv1.CustomResourceDefinitionVersion{
					{Name: "v1alpha1", Served: false, Storage: true},
					{Name: "v1beta1", Served: true, Storage: false},
				},
			}},
			want: want{version: "v1beta1"},
		},
		"NoneServed": {
			reason: "No served version means no version to list against.",
			crd: extv1.CustomResourceDefinition{Spec: extv1.CustomResourceDefinitionSpec{
				Versions: []extv1.CustomResourceDefinitionVersion{{Name: "v1", Served: false, Storage: true}},
			}},
			want: want{version: ""},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := pickCRDVersion(&tc.crd)
			if diff := cmp.Diff(tc.want.version, got); diff != "" {
				t.Errorf("\n%s\npickCRDVersion(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func xrd(group, kind, version string, claimKind string) apiextensionsv1.CompositeResourceDefinition {
	x := apiextensionsv1.CompositeResourceDefinition{}
	x.Spec.Group = group
	x.Spec.Names = extv1.CustomResourceDefinitionNames{Kind: kind}
	x.Spec.Versions = []apiextensionsv1.CompositeResourceDefinitionVersion{
		{Name: version, Served: true, Referenceable: true},
	}
	if claimKind != "" {
		x.Spec.ClaimNames = &extv1.CustomResourceDefinitionNames{Kind: claimKind}
	}
	return x
}

func TestDiscoverXRAndClaimTypes(t *testing.T) {
	type want struct {
		types []DiscoveredType
		err   error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		xrds   *apiextensionsv1.CompositeResourceDefinitionList
		want   want
	}{
		"UseExistingXRDsIfSupplied": {
			reason: "When the caller passes an XRD list, the function should use that instead of doing a list.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(_ client.ObjectList) error {
				return errBoom // we should not hit this list call
			})},
			xrds: &apiextensionsv1.CompositeResourceDefinitionList{Items: []apiextensionsv1.CompositeResourceDefinition{
				xrd("example.org", "XThing", "v1", "Thing"),
			}},
			want: want{types: []DiscoveredType{
				// both the XR and Claim type should be returned
				{GVK: schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XThing"}, Namespaced: false},
				{GVK: schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Thing"}, Namespaced: true},
			}},
		},
		"FetchesWhenXRDsNotSupplied": {
			reason: "When the caller passes no XRD list, the function lists them itself.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*apiextensionsv1.CompositeResourceDefinitionList).Items = []apiextensionsv1.CompositeResourceDefinition{
					xrd("example.org", "XThing", "v1", ""),
				}
				return nil
			})},
			xrds: nil,
			want: want{types: []DiscoveredType{
				{GVK: schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XThing"}, Namespaced: false},
			}},
		},
		"ListError": {
			reason: "With no XRD list supplied, the function lists XRDs itself; a failure of that List is propagated to the caller.",
			client: &test.MockClient{MockList: test.NewMockListFn(errBoom)},
			xrds:   nil,
			want:   want{err: cmpopts.AnyError},
		},
		"XRAndClaim": {
			reason: "An XRD with claim names yields both a cluster-scoped XR type and a namespaced Claim type.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*apiextensionsv1.CompositeResourceDefinitionList).Items = []apiextensionsv1.CompositeResourceDefinition{
					xrd("example.org", "XThing", "v1", "Thing"),
				}
				return nil
			})},
			want: want{types: []DiscoveredType{
				{GVK: schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XThing"}, Namespaced: false},
				{GVK: schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Thing"}, Namespaced: true},
			}},
		},
		"SkipsUnservedXRD": {
			reason: "An XRD with no served version is skipped entirely.",
			client: &test.MockClient{},
			xrds: &apiextensionsv1.CompositeResourceDefinitionList{Items: []apiextensionsv1.CompositeResourceDefinition{
				{Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
					Group:    "example.org",
					Names:    extv1.CustomResourceDefinitionNames{Kind: "XDead"},
					Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{{Name: "v1", Served: false}},
				}},
			}},
			want: want{types: []DiscoveredType{}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := DiscoverXRAndClaimTypes(context.Background(), tc.client, tc.xrds)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDiscoverXRAndClaimTypes(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.types, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDiscoverXRAndClaimTypes(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func managedCRD(group, kind, version string, namespaced bool) extv1.CustomResourceDefinition {
	scope := extv1.ClusterScoped
	if namespaced {
		scope = extv1.NamespaceScoped
	}
	return extv1.CustomResourceDefinition{Spec: extv1.CustomResourceDefinitionSpec{
		Group:    group,
		Scope:    scope,
		Names:    extv1.CustomResourceDefinitionNames{Kind: kind, Categories: []string{managedResourceCategory}},
		Versions: []extv1.CustomResourceDefinitionVersion{{Name: version, Served: true, Storage: true}},
	}}
}

func TestDiscoverManagedResources(t *testing.T) {
	type want struct {
		types []DiscoveredType
		err   error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		want   want
	}{
		"ListError": {
			reason: "A failure listing CRDs surfaces as an error.",
			client: &test.MockClient{MockList: test.NewMockListFn(errBoom)},
			want:   want{err: cmpopts.AnyError},
		},
		"OnlyManagedCategory": {
			reason: "Only CRDs in the \"managed\" category are returned, with scope reflected.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*extv1.CustomResourceDefinitionList).Items = []extv1.CustomResourceDefinition{
					managedCRD("aws.example.org", "Bucket", "v1beta1", false),
					managedCRD("ns.example.org", "Queue", "v1", true),
					{Spec: extv1.CustomResourceDefinitionSpec{ // not a "managed" CRD, should be ignored
						Group:    "example.org",
						Names:    extv1.CustomResourceDefinitionNames{Kind: "Other"},
						Versions: []extv1.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true}},
					}},
				}
				return nil
			})},
			want: want{types: []DiscoveredType{
				{GVK: schema.GroupVersionKind{Group: "aws.example.org", Version: "v1beta1", Kind: "Bucket"}, Namespaced: false},
				{GVK: schema.GroupVersionKind{Group: "ns.example.org", Version: "v1", Kind: "Queue"}, Namespaced: true},
			}},
		},
		"SkipsUnservedManagedCRD": {
			reason: "A managed CRD with no served version is skipped entirely.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*extv1.CustomResourceDefinitionList).Items = []extv1.CustomResourceDefinition{
					{Spec: extv1.CustomResourceDefinitionSpec{
						Group:    "aws.example.org",
						Names:    extv1.CustomResourceDefinitionNames{Kind: "Dead", Categories: []string{managedResourceCategory}},
						Versions: []extv1.CustomResourceDefinitionVersion{{Name: "v1", Served: false, Storage: true}},
					}},
				}
				return nil
			})},
			want: want{types: []DiscoveredType{}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := DiscoverManagedResources(context.Background(), tc.client)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDiscoverManagedResources(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.types, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDiscoverManagedResources(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestListInstances(t *testing.T) {
	type want struct {
		listGVK    schema.GroupVersionKind
		namespaced string // the namespace passed via client.InNamespace, "" if none
		err        error
	}
	cases := map[string]struct {
		reason    string
		dt        DiscoveredType
		namespace string
		want      want
	}{
		"ClusterScopedIgnoresNamespace": {
			reason:    "Cluster-scoped types list with no namespace option even when one is given.",
			dt:        DiscoveredType{GVK: schema.GroupVersionKind{Group: "g", Version: "v1", Kind: "XThing"}, Namespaced: false},
			namespace: "team-a",
			want: want{
				listGVK:    schema.GroupVersionKind{Group: "g", Version: "v1", Kind: "XThingList"},
				namespaced: "",
			},
		},
		"NamespacedWithNamespace": {
			reason:    "A namespaced type with a namespace restricts the List via InNamespace.",
			dt:        DiscoveredType{GVK: schema.GroupVersionKind{Group: "g", Version: "v1", Kind: "Thing"}, Namespaced: true},
			namespace: "team-a",
			want: want{
				listGVK:    schema.GroupVersionKind{Group: "g", Version: "v1", Kind: "ThingList"},
				namespaced: "team-a",
			},
		},
		"NamespacedAllNamespaces": {
			reason:    "A namespaced type with an empty namespace lists across all namespaces.",
			dt:        DiscoveredType{GVK: schema.GroupVersionKind{Group: "g", Version: "v1", Kind: "Thing"}, Namespaced: true},
			namespace: "",
			want: want{
				listGVK:    schema.GroupVersionKind{Group: "g", Version: "v1", Kind: "ThingList"},
				namespaced: "",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var gotGVK schema.GroupVersionKind
			var gotNS string
			c := &test.MockClient{
				MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
					gotGVK = list.GetObjectKind().GroupVersionKind()
					for _, o := range opts {
						if ns, ok := o.(client.InNamespace); ok {
							gotNS = string(ns)
						}
					}
					return nil
				},
			}
			_, err := ListInstances(context.Background(), c, tc.dt, tc.namespace)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nListInstances(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.listGVK, gotGVK); diff != "" {
				t.Errorf("\n%s\nListInstances() list GVK: -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.namespaced, gotNS); diff != "" {
				t.Errorf("\n%s\nListInstances() namespace: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResourceRefFromUnstructured(t *testing.T) {
	cases := map[string]struct {
		reason    string
		gvk       schema.GroupVersionKind
		namespace string
		name      string
		want      ResourceRef
	}{
		"Namespaced": {
			reason:    "Group/kind/namespace/name are kept; the version is dropped (ResourceRef carries no version).",
			gvk:       schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Thing"},
			namespace: "team-a",
			name:      "my-thing",
			want:      ResourceRef{Group: "example.org", Kind: "Thing", Namespace: "team-a", Name: "my-thing"},
		},
		"ClusterScoped": {
			reason: "A resource with no namespace set yields an empty Namespace.",
			gvk:    schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "XThing"},
			name:   "my-xthing",
			want:   ResourceRef{Group: "example.org", Kind: "XThing", Name: "my-xthing"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			u := unstructured.Unstructured{}
			u.SetGroupVersionKind(tc.gvk)
			u.SetNamespace(tc.namespace)
			u.SetName(tc.name)

			got := ResourceRefFromUnstructured(u)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nResourceRefFromUnstructured(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
