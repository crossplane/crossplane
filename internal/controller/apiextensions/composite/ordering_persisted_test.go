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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
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

	want, err := AsEdges(deps)
	if err != nil {
		t.Fatalf("AsEdges(...): %v", err)
	}

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

func TestAsEdgesLifecycle(t *testing.T) {
	cases := map[string]struct {
		reason    string
		lifecycle fnv1.DependencyLifecycle
		want      ordering.Lifecycle
		wantErr   bool
	}{
		"UnspecifiedIsSymmetric": {
			reason:    "An unset lifecycle should order both directions, which is the protocol's documented default.",
			lifecycle: fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_UNSPECIFIED,
			want:      ordering.LifecycleSymmetric,
		},
		"CreateBeforeDestroyCarriesThrough": {
			reason:    "The one non-default lifecycle should reach the graph.",
			lifecycle: fnv1.DependencyLifecycle_DEPENDENCY_LIFECYCLE_CREATE_BEFORE_DESTROY,
			want:      ordering.LifecycleCreateBeforeDestroy,
		},
		"UnknownIsRejected": {
			reason:    "A lifecycle from a newer protocol should fail the reconcile rather than silently becoming the default, which could delete a resource the function meant to keep.",
			lifecycle: fnv1.DependencyLifecycle(99),
			wantErr:   true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			deps := []*fnv1.Dependency{{
				Resource:  "subnet",
				DependsOn: &fnv1.Dependency_ComposedResource{ComposedResource: "vpc"},
				Lifecycle: tc.lifecycle,
			}}

			got, err := AsEdges(deps)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("\n%s\nAsEdges(...): want an error, got none", tc.reason)
				}

				return
			}

			if err != nil {
				t.Fatalf("\n%s\nAsEdges(...): unexpected error: %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want, got[0].Lifecycle); diff != "" {
				t.Errorf("\n%s\nAsEdges(...) lifecycle: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestPendingResources covers what the XR reports about resources ordering is
// holding back - including the one case that appears nowhere else, a resource
// held back from being created, which has no composed resource reference
// because Crossplane deliberately never applied it.
func TestPendingResources(t *testing.T) {
	cd := func(name string) ComposedResourceState {
		r := composed.New()
		r.SetAPIVersion("example.org/v1")
		r.SetKind("Thing")
		r.SetName("cool-xr-" + name)

		return ComposedResourceState{Resource: r}
	}

	deps := []*fnv1.Dependency{
		{
			Resource:  "subnet",
			DependsOn: &fnv1.Dependency_ComposedResource{ComposedResource: "vpc"},
		},
		{
			Resource: "subnet",
			DependsOn: &fnv1.Dependency_RequiredResource{
				RequiredResource: &fnv1.RequiredResourceDependency{RequirementName: "kubeconfig"},
			},
		},
	}

	cases := map[string]struct {
		reason    string
		decisions ordering.Decisions
		desired   ComposedResourceStates
		observed  ComposedResourceStates
		want      []reference.Pending
	}{
		"NothingBlocked": {
			reason:    "An XR with nothing held back carries no pending resources at all, rather than an empty list saying so.",
			decisions: ordering.Decisions{Blocked: map[string]ordering.Decision{}},
			desired:   ComposedResourceStates{"subnet": cd("subnet")},
		},
		"HeldFromCreation": {
			reason: "Still desired, so what's held back is creating it - and it carries the edges it's waiting on, since it has no reference to carry them.",
			decisions: ordering.Decisions{Blocked: map[string]ordering.Decision{
				"subnet": {Blocked: true, Reason: "waiting for vpc to be ready"},
			}},
			desired: ComposedResourceStates{"subnet": cd("subnet")},
			want: []reference.Pending{{
				APIVersion:   "example.org/v1",
				Kind:         "Thing",
				ResourceName: "subnet",
				Operation:    reference.OperationCreate,
				Reason:       "waiting for vpc to be ready",
				DependsOn: []reference.Dependency{
					{Name: "vpc"},
					{
						Type:        reference.DependencyTypeRequiredResource,
						Requirement: &reference.RequirementDependency{Name: "kubeconfig"},
					},
				},
			}},
		},
		"HeldFromDeletion": {
			reason: "No longer desired but still observed, so what's held back is deleting it - and unlike a pending creation, the object exists and is worth naming.",
			decisions: ordering.Decisions{Blocked: map[string]ordering.Decision{
				"vpc": {Blocked: true, Reason: "subnet still depends on it"},
			}},
			desired:  ComposedResourceStates{},
			observed: ComposedResourceStates{"vpc": cd("vpc")},
			want: []reference.Pending{{
				APIVersion:   "example.org/v1",
				Kind:         "Thing",
				Name:         "cool-xr-vpc",
				ResourceName: "vpc",
				Operation:    reference.OperationDelete,
				Reason:       "subnet still depends on it",
			}},
		},
		"Deadlocked": {
			reason: "A deadlock is carried as a field, not a prefix on a sentence, so something can alert on it.",
			decisions: ordering.Decisions{Blocked: map[string]ordering.Decision{
				"subnet": {Blocked: true, Deadlocked: true, Reason: "depends on vpc, which is not desired"},
			}},
			desired: ComposedResourceStates{"subnet": cd("subnet")},
			want: []reference.Pending{{
				APIVersion:   "example.org/v1",
				Kind:         "Thing",
				ResourceName: "subnet",
				Operation:    reference.OperationCreate,
				Reason:       "depends on vpc, which is not desired",
				Deadlocked:   true,
				DependsOn: []reference.Dependency{
					{Name: "vpc"},
					{
						Type:        reference.DependencyTypeRequiredResource,
						Requirement: &reference.RequirementDependency{Name: "kubeconfig"},
					},
				},
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := pendingResources(tc.decisions, tc.desired, tc.observed, deps)

			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\npendingResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestOrderingOnLegacyXRs covers the schema branch that differs.
//
// Ordering is supported on legacy v1 XRs, and works there because the graph
// rides on composed resource references, which both schemas carry. But legacy
// XRs keep those references at spec.resourceRefs where modern ones nest them
// under spec.crossplane, and that path is chosen by the same accessor
// teardown rebuilds the graph from. Nothing else in ordering branches on
// schema, so this is the whole of what "v1 is supported" means - and it was
// working by accident, with no test, until this one.
func TestOrderingOnLegacyXRs(t *testing.T) {
	xr := composite.New(composite.WithSchema(composite.SchemaLegacy))

	desired := ComposedResourceStates{
		"vpc":    state("VPC", "xr-vpc-8xk2p"),
		"subnet": state("Subnet", "xr-subnet-p4m9x"),
	}

	UpdateComposedResourceRefs(xr, desired, []*fnv1.Dependency{dep("subnet", "vpc")})

	// Legacy XRs keep machinery at the top of spec, so the graph has to be
	// there rather than under spec.crossplane - otherwise teardown rebuilds
	// an empty graph and cascades, which looks exactly like having finished.
	refs, found, err := unstructured.NestedSlice(xr.Object, "spec", "resourceRefs")
	if err != nil || !found {
		t.Fatalf("a legacy XR keeps its references at spec.resourceRefs: found=%v err=%v", found, err)
	}

	if nested, found, _ := unstructured.NestedSlice(xr.Object, "spec", "crossplane", "resourceRefs"); found {
		t.Errorf("a legacy XR must not nest its references: %v", nested)
	}

	if len(refs) != 2 {
		t.Fatalf("want 2 references, got %d: %v", len(refs), refs)
	}

	// And the edges have to survive the round trip through that path, since
	// teardown reads them back rather than recomputing them.
	want := []reference.Composed{
		{APIVersion: "example.org/v1", Kind: "Subnet", Name: "xr-subnet-p4m9x", ResourceName: "subnet", DependsOn: []string{"vpc"}},
		{APIVersion: "example.org/v1", Kind: "VPC", Name: "xr-vpc-8xk2p", ResourceName: "vpc"},
	}

	if diff := cmp.Diff(want, xr.GetComposedResourceReferences()); diff != "" {
		t.Errorf("a legacy XR's references carry the graph: -want, +got:\n%s", diff)
	}

	// What ordering does with them is the same on both schemas: EdgesFromRefs
	// is what teardown rebuilds from, and it reads the accessor rather than a
	// path.
	if edges := EdgesFromRefs(xr.GetComposedResourceReferences()); len(edges) != 1 {
		t.Errorf("teardown rebuilds one edge from a legacy XR's references, got %d: %v", len(edges), edges)
	}
}

// TestTeardownOrdersALegacyXR is the other half: that the reconciler's
// teardown path, given a legacy XR, rebuilds the graph and holds back what
// still has dependents.
func TestTeardownOrdersALegacyXR(t *testing.T) {
	xr := composite.New(composite.WithSchema(composite.SchemaLegacy))
	xr.SetName("cool-xr")
	xr.SetComposedResourceReferences([]reference.Composed{
		{APIVersion: "example.org/v1", Kind: "Thing", Name: "cool-xr-vpc", ResourceName: "vpc"},
		{APIVersion: "example.org/v1", Kind: "Thing", Name: "cool-xr-subnet", ResourceName: "subnet", DependsOn: []string{"vpc"}},
	})

	deleted := []string{}

	r := &Reconciler{
		log:        logging.NewNopLogger(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		observer: ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
			return ComposedResourceStates{
				"vpc":    state("Thing", "cool-xr-vpc"),
				"subnet": state("Thing", "cool-xr-subnet"),
			}, nil
		}),
		gc: ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, observed, _ ComposedResourceStates) error {
			for n := range observed {
				deleted = append(deleted, string(n))
			}

			return nil
		}),
	}

	done, err := r.teardown(context.Background(), xr, r.conditions.For(xr))
	if err != nil {
		t.Fatalf("teardown(...): %v", err)
	}

	if done {
		t.Error("teardown(...): reported done while a composed resource still has a dependent")
	}

	if diff := cmp.Diff([]string{"subnet"}, deleted); diff != "" {
		t.Errorf("teardown(...) deletes the leaf first on a legacy XR too: -want, +got:\n%s", diff)
	}
}
