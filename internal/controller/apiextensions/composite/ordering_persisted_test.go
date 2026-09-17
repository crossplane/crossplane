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
