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

package composite

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"

	"github.com/crossplane/crossplane/v2/internal/xfn/ordering"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// dep returns a composed-to-composed dependency.
// byEdge sorts edges so two producers of the same graph compare equal.
func byEdge() cmp.Option {
	return cmpopts.SortSlices(func(a, b ordering.Edge) bool {
		return a.Resource+"|"+a.DependsOn.ComposedResource < b.Resource+"|"+b.DependsOn.ComposedResource
	})
}

func dep(resource, dependsOn string) *fnv1.Dependency {
	return &fnv1.Dependency{
		Resource:  resource,
		DependsOn: &fnv1.Dependency_ComposedResource{ComposedResource: dependsOn},
	}
}

// state returns a composed resource state for a resource of the supplied kind.
func state(kind, name string) ComposedResourceState {
	cd := composed.New()
	cd.SetAPIVersion("example.org/v1")
	cd.SetKind(kind)
	cd.SetName(name)

	return ComposedResourceState{Resource: cd}
}

func TestUpdateComposedResourceRefsPersistsGraph(t *testing.T) {
	xr := composite.New()

	desired := ComposedResourceStates{
		"vpc":      state("VPC", "xr-vpc-8xk2p"),
		"subnet-a": state("Subnet", "xr-subnet-a-p4m9x"),
		"instance": state("Instance", "xr-instance-2jd7q"),
	}

	deps := []*fnv1.Dependency{
		dep("subnet-a", "vpc"),
		dep("instance", "subnet-a"),
		// A duplicate edge must not produce a duplicate entry: dependsOn is a
		// set in the schema.
		dep("instance", "subnet-a"),
		// A required-resource edge is not persisted.
		{
			Resource: "vpc",
			DependsOn: &fnv1.Dependency_RequiredResource{
				RequiredResource: &fnv1.RequiredResourceDependency{RequirementName: "shared-config"},
			},
		},
	}

	UpdateComposedResourceRefs(xr, desired, deps)

	want := []reference.Composed{
		{APIVersion: "example.org/v1", Kind: "Instance", Name: "xr-instance-2jd7q", ResourceName: "instance", DependsOn: []string{"subnet-a"}},
		{APIVersion: "example.org/v1", Kind: "Subnet", Name: "xr-subnet-a-p4m9x", ResourceName: "subnet-a", DependsOn: []string{"vpc"}},
		{APIVersion: "example.org/v1", Kind: "VPC", Name: "xr-vpc-8xk2p", ResourceName: "vpc"},
	}

	got := xr.GetComposedResourceReferences()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("\nEach reference records its composition resource name and the composed resources it depends on.\nGetComposedResourceReferences(...): -want, +got:\n%s", diff)
	}
}

func TestEdgesFromRefsRoundTrip(t *testing.T) {
	// The claim the design rests on: a graph rebuilt from what was persisted
	// orders teardown the same way the function-declared one does.
	desired := ComposedResourceStates{
		"vpc":      state("VPC", "xr-vpc-8xk2p"),
		"subnet":   state("Subnet", "xr-subnet-p4m9x"),
		"instance": state("Instance", "xr-instance-2jd7q"),
	}

	deps := []*fnv1.Dependency{
		dep("subnet", "vpc"),
		dep("instance", "subnet"),
	}

	xr := composite.New()
	UpdateComposedResourceRefs(xr, desired, deps)

	want := AsEdges(deps)

	got := EdgesFromRefs(xr.GetComposedResourceReferences())
	if diff := cmp.Diff(want, got, byEdge()); diff != "" {
		t.Errorf("\nA graph rebuilt from persisted references matches the one the pipeline declared.\nEdgesFromRefs(...): -want, +got:\n%s", diff)
	}
}

func TestEdgesFromRefsSkipsUnnamedRefs(t *testing.T) {
	// References written before resourceName existed can't be placed in the
	// graph. They must be skipped, not guessed at.
	refs := []reference.Composed{
		{APIVersion: "example.org/v1", Kind: "VPC", Name: "xr-vpc-8xk2p"},
		{ResourceName: "subnet", DependsOn: []string{"vpc"}},
	}

	want := []ordering.Edge{{Resource: "subnet", DependsOn: ordering.Target{ComposedResource: "vpc"}}}

	got := EdgesFromRefs(refs)
	if diff := cmp.Diff(want, got, byEdge()); diff != "" {
		t.Errorf("\nA reference with no composition resource name is skipped.\nEdgesFromRefs(...): -want, +got:\n%s", diff)
	}
}

// named builds a required resource identifiable by its metadata name, which is
// all these tests need to tell merged items apart.
func named(n string) *fnv1.Resource {
	s, _ := structpb.NewStruct(map[string]any{"metadata": map[string]any{"name": n}})

	return &fnv1.Resource{Resource: s}
}

func nameOf(r *fnv1.Resource) string {
	return r.GetResource().GetFields()["metadata"].GetStructValue().GetFields()["name"].GetStringValue()
}

func TestMergeRequiredResources(t *testing.T) {
	res := func(names ...string) *fnv1.Resources {
		items := make([]*fnv1.Resource, 0, len(names))
		for _, n := range names {
			items = append(items, named(n))
		}

		return &fnv1.Resources{Items: items}
	}

	cases := map[string]struct {
		reason string
		into   map[string]*fnv1.Resources
		from   map[string]*fnv1.Resources
		want   map[string][]string
	}{
		"TakeRequirementOnlyOneStepDeclared": {
			reason: "A requirement only the later step declared should be taken as-is.",
			into:   map[string]*fnv1.Resources{"vpc": res("vpc-a")},
			from:   map[string]*fnv1.Resources{"subnet": res("subnet-a")},
			want:   map[string][]string{"vpc": {"vpc-a"}, "subnet": {"subnet-a"}},
		},
		"UnionRequirementBothStepsDeclared": {
			reason: "Where two steps declare the same requirement, an edge over it should wait for everything either of them matched.",
			into:   map[string]*fnv1.Resources{"vpc": res("vpc-a")},
			from:   map[string]*fnv1.Resources{"vpc": res("vpc-b")},
			want:   map[string][]string{"vpc": {"vpc-a", "vpc-b"}},
		},
		"UnionRequirementThatMatchedNothing": {
			reason: "A requirement that matched nothing in either step should stay empty rather than error.",
			into:   map[string]*fnv1.Resources{"vpc": res()},
			from:   map[string]*fnv1.Resources{"vpc": res()},
			want:   map[string][]string{"vpc": {}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mergeRequiredResources(tc.into, tc.from)

			got := map[string][]string{}
			for k, v := range tc.into {
				ns := make([]string, 0, len(v.GetItems()))
				for _, i := range v.GetItems() {
					ns = append(ns, nameOf(i))
				}
				got[k] = ns
			}

			if diff := cmp.Diff(tc.want, got, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
				t.Errorf("\n%s\nmergeRequiredResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestMergeRequiredResourcesDoesNotAliasInputs covers why the merged slice is
// always allocated: the slices being merged are still owned by a function's
// request, so appending in place could write into their spare capacity and
// change what an earlier pipeline step sees.
func TestMergeRequiredResourcesDoesNotAliasInputs(t *testing.T) {
	// Spare capacity is what makes aliasing possible, so build one that has it.
	backing := make([]*fnv1.Resource, 1, 4)
	backing[0] = named("vpc-a")

	into := map[string]*fnv1.Resources{"vpc": {Items: backing}}
	from := map[string]*fnv1.Resources{"vpc": {Items: []*fnv1.Resource{named("vpc-b")}}}

	mergeRequiredResources(into, from)

	// A write through the original backing array must not be visible in the
	// merged result.
	backing = append(backing, named("clobbered"))
	_ = backing

	got := make([]string, 0, len(into["vpc"].GetItems()))
	for _, i := range into["vpc"].GetItems() {
		got = append(got, nameOf(i))
	}

	if diff := cmp.Diff([]string{"vpc-a", "vpc-b"}, got); diff != "" {
		t.Errorf("merged items alias the input's backing array: -want, +got:\n%s", diff)
	}
}
