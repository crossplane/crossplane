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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	"github.com/crossplane/crossplane/v2/internal/xfn/ordering"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// composedResource returns a desired composed resource with the given name.
func composedResource(name string, ready bool) *fnv1.Resource {
	r := &fnv1.Resource{
		Resource: MustStruct(map[string]any{
			"apiVersion": "test.crossplane.io/v1",
			"kind":       "Composed",
			"metadata":   map[string]any{"name": name},
		}),
	}
	if ready {
		r.Ready = fnv1.Ready_READY_TRUE
	}

	return r
}

// observedResource returns an observed composed resource state.
func observedResource(name string) ComposedResourceState {
	cd := composed.New()
	cd.SetAPIVersion("test.crossplane.io/v1")
	cd.SetKind("Composed")
	cd.SetName(name)

	return ComposedResourceState{Resource: cd}
}

// singleStep returns a CompositionRequest with one pipeline step.
func singleStep() CompositionRequest {
	return CompositionRequest{
		Revision: &v1.CompositionRevision{
			Spec: v1.CompositionRevisionSpec{
				Pipeline: []v1.PipelineStep{{
					Step:        "run-cool-function",
					FunctionRef: v1.FunctionReference{Name: "cool-function"},
				}},
			},
		},
	}
}

// dependsOn returns an edge from resource to dependsOn.
func dependsOn(res, dep string) *fnv1.Dependency {
	return &fnv1.Dependency{
		Resource:  res,
		DependsOn: &fnv1.Dependency_ComposedResource{ComposedResource: dep},
	}
}

// orderingParams builds the composer options common to these tests. observed is
// what already exists; gc records what the garbage collector was asked to
// consider deleting.
func orderingParams(observed ComposedResourceStates, gc *ComposedResourceStates, enabled bool) []FunctionComposerOption {
	return []FunctionComposerOption{
		WithComposedResourceOrdering(enabled),
		WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(_ context.Context, _ ConnectionSecretOwner) (managed.ConnectionDetails, error) {
			return nil, nil
		})),
		WithComposedResourceObserver(ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
			return observed, nil
		})),
		WithComposedResourceGarbageCollector(ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, obs, _ ComposedResourceStates) error {
			if gc != nil {
				*gc = obs
			}

			return nil
		})),
	}
}

func TestFunctionComposeOrderingApplyGate(t *testing.T) {
	cases := map[string]struct {
		reason   string
		enabled  bool
		deps     []*fnv1.Dependency
		desired  map[string]*fnv1.Resource
		observed ComposedResourceStates
		want     []ComposedResource
	}{
		"BlockedUntilDependencyReady": {
			reason:  "A resource whose dependency isn't ready is reported as not synced rather than applied.",
			enabled: true,
			deps:    []*fnv1.Dependency{dependsOn("subnet", "vpc")},
			desired: map[string]*fnv1.Resource{
				"vpc":    composedResource("vpc", false),
				"subnet": composedResource("subnet", false),
			},
			want: []ComposedResource{
				{ResourceName: "subnet", Ready: false, Synced: false, Reason: "waiting for [vpc] to be ready"},
				{ResourceName: "vpc", Ready: false, Synced: true},
			},
		},
		"AppliedOnceDependencyReady": {
			reason:  "Once the dependency reports ready the dependent is applied.",
			enabled: true,
			deps:    []*fnv1.Dependency{dependsOn("subnet", "vpc")},
			desired: map[string]*fnv1.Resource{
				"vpc":    composedResource("vpc", true),
				"subnet": composedResource("subnet", false),
			},
			want: []ComposedResource{
				{ResourceName: "subnet", Ready: false, Synced: true},
				{ResourceName: "vpc", Ready: true, Synced: true},
			},
		},
		"IgnoredWhenFeatureDisabled": {
			reason:  "With the alpha feature off, dependencies a function returns have no effect.",
			enabled: false,
			deps:    []*fnv1.Dependency{dependsOn("subnet", "vpc")},
			desired: map[string]*fnv1.Resource{
				"vpc":    composedResource("vpc", false),
				"subnet": composedResource("subnet", false),
			},
			want: []ComposedResource{
				{ResourceName: "subnet", Ready: false, Synced: true},
				{ResourceName: "vpc", Ready: false, Synced: true},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
				return &fnv1.RunFunctionResponse{
					Desired:      &fnv1.State{Resources: tc.desired},
					Dependencies: &fnv1.Dependencies{Items: tc.deps},
				}, nil
			})

			c := NewFunctionComposer(
				&test.MockClient{
					MockPatch:              test.NewMockPatchFn(nil),
					MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
					MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
				},
				&test.MockClient{},
				r,
				orderingParams(tc.observed, nil, tc.enabled)...,
			)

			res, err := c.Compose(context.Background(), WithParentLabel(), singleStep())
			if err != nil {
				t.Fatalf("\n%s\nCompose(...): unexpected error: %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want, res.Composed, cmpopts.SortSlices(func(a, b ComposedResource) bool {
				return a.ResourceName < b.ResourceName
			})); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestFunctionComposeOrderingUsesDynamicallyFetchedRequirements verifies that
// an edge may use a requirement returned by the same function invocation. The
// fetching runner resolves the requirement by reusing and mutating fnreq; the
// composer must consume that final request state when it decides ordering.
func TestFunctionComposeOrderingUsesDynamicallyFetchedRequirements(t *testing.T) {
	runner := xfn.NewFetchingFunctionRunner(
		FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			return &fnv1.RunFunctionResponse{
				Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{
					"app": composedResource("app", true),
				}},
				Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{{
					Resource: "app",
					DependsOn: &fnv1.Dependency_RequiredResource{RequiredResource: &fnv1.RequiredResourceDependency{
						RequirementName: "environment",
					}},
				}}},
				Requirements: &fnv1.Requirements{Resources: map[string]*fnv1.ResourceSelector{
					"environment": {ApiVersion: "v1", Kind: "ConfigMap", Match: &fnv1.ResourceSelector_MatchName{MatchName: "environment"}},
				}},
			}, nil
		}),
		xfn.RequiredResourcesFetcherFn(func(context.Context, *fnv1.ResourceSelector) (*fnv1.Resources, error) {
			// ConfigMaps do not carry a Ready condition, so existence means ready.
			return &fnv1.Resources{Items: []*fnv1.Resource{composedResource("environment", true)}}, nil
		}),
		xfn.NopRequiredSchemasFetcher{},
	)

	c := NewFunctionComposer(
		&test.MockClient{
			MockPatch:              test.NewMockPatchFn(nil),
			MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
			MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
		},
		&test.MockClient{},
		runner,
		orderingParams(nil, nil, true)...,
	)

	res, err := c.Compose(context.Background(), WithParentLabel(), singleStep())
	if err != nil {
		t.Fatalf("Compose(...): unexpected error: %v", err)
	}

	if len(res.Composed) != 1 || !res.Composed[0].Synced {
		t.Errorf("A ready requirement fetched during this run should not block its dependent, got %#v", res.Composed)
	}
}

func TestOrderingRequiredResourceNamespace(t *testing.T) {
	dependency := &fnv1.Dependency{
		Resource: "app",
		DependsOn: &fnv1.Dependency_RequiredResource{RequiredResource: &fnv1.RequiredResourceDependency{
			RequirementName: "networks",
			Name:            new("shared"),
			Namespace:       new("platform-a"),
		}},
	}

	required := map[string]*fnv1.Resources{"networks": {Items: []*fnv1.Resource{
		{Resource: MustStruct(map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "shared", "namespace": "platform-a"},
		})},
		{Resource: MustStruct(map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "shared", "namespace": "platform-b"},
			"status": map[string]any{"conditions": []any{
				map[string]any{"type": "Ready", "status": "False"},
			}},
		})},
	}}}

	state := AsOrderingState(ComposedResourceStates{
		"app": {Ready: true},
	}, nil, required)
	got := ordering.New(AsEdges([]*fnv1.Dependency{dependency})...).Decide(state)

	if len(got.Apply) != 1 || got.Apply[0] != "app" {
		t.Errorf("The ready platform-a/shared resource should unblock app independently of platform-b/shared, got %#v", got)
	}
}

func TestFunctionComposeOrderingDeleteGate(t *testing.T) {
	cases := map[string]struct {
		reason    string
		deps      []*fnv1.Dependency
		observed  ComposedResourceStates
		wantGC    []string
		wantRefs  []string
		wantBlock map[string]string
	}{
		"DeferredDeleteIsHiddenFromCollector": {
			reason: "The VPC isn't offered to the collector while the subnet that depends on it still exists.",
			deps:   []*fnv1.Dependency{dependsOn("subnet", "vpc")},
			observed: ComposedResourceStates{
				"vpc":    observedResource("vpc"),
				"subnet": observedResource("subnet"),
			},
			wantGC: []string{"subnet"},
			// Both stay referenced. The VPC because its deletion is deferred,
			// and the subnet because a delete has only been issued - it may
			// sit in Terminating behind a finalizer for some time, and losing
			// its reference would unblock the VPC prematurely.
			wantRefs: []string{"subnet", "vpc"},
			wantBlock: map[string]string{
				"vpc": "waiting for [subnet] to be deleted",
			},
		},
		"DeleteProceedsOnceDependentGone": {
			reason:   "With the subnet gone from observed state the VPC may be collected. It stays referenced until it's confirmed gone, then drops out on a later pass.",
			deps:     []*fnv1.Dependency{dependsOn("subnet", "vpc")},
			observed: ComposedResourceStates{"vpc": observedResource("vpc")},
			wantGC:   []string{"vpc"},
			wantRefs: []string{"vpc"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gc := ComposedResourceStates{}
			refs := []string{}

			r := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
				// Nothing is desired: everything should go, in order.
				return &fnv1.RunFunctionResponse{
					Desired:      &fnv1.State{Resources: map[string]*fnv1.Resource{}},
					Dependencies: &fnv1.Dependencies{Items: tc.deps},
				}, nil
			})

			c := NewFunctionComposer(
				&test.MockClient{
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						u, ok := obj.(*composite.Unstructured)
						if !ok {
							return nil
						}

						for _, ref := range u.GetResourceReferences() {
							refs = append(refs, ref.Name)
						}

						return nil
					},
					MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
					MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
				},
				&test.MockClient{},
				r,
				orderingParams(tc.observed, &gc, true)...,
			)

			res, err := c.Compose(context.Background(), WithParentLabel(), singleStep())
			if err != nil {
				t.Fatalf("\n%s\nCompose(...): unexpected error: %v", tc.reason, err)
			}

			got := make([]string, 0, len(gc))
			for n := range gc {
				got = append(got, string(n))
			}

			less := cmpopts.SortSlices(func(a, b string) bool { return a < b })

			if diff := cmp.Diff(tc.wantGC, got, less, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nCompose(...): garbage collector saw -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.wantRefs, refs, less, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nCompose(...): spec.resourceRefs -want, +got:\n%s", tc.reason, diff)
			}

			gotBlocked := map[string]string{}
			for _, cd := range res.Composed {
				if !cd.Synced && cd.Reason != "" {
					gotBlocked[string(cd.ResourceName)] = cd.Reason
				}
			}
			if diff := cmp.Diff(tc.wantBlock, gotBlocked, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nCompose(...): blocked resource reasons -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// steps returns a CompositionRequest with the named pipeline steps.
func steps(names ...string) CompositionRequest {
	p := make([]v1.PipelineStep, 0, len(names))
	for _, n := range names {
		p = append(p, v1.PipelineStep{Step: n, FunctionRef: v1.FunctionReference{Name: n}})
	}

	return CompositionRequest{
		Revision: &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{Pipeline: p}},
	}
}

// TestFunctionComposeOrderingAcrossSteps covers what happens to dependencies as
// they pass between pipeline steps. This is where an unaware function can do
// damage, so it's the behavior most worth pinning down.
func TestFunctionComposeOrderingAcrossSteps(t *testing.T) {
	vpcAndSubnet := map[string]*fnv1.Resource{
		"vpc":    composedResource("vpc", false),
		"subnet": composedResource("subnet", false),
	}

	cases := map[string]struct {
		reason string
		// responses is indexed by function name.
		responses map[string]*fnv1.RunFunctionResponse
		pipeline  []string
		observed  ComposedResourceStates
		want      []ComposedResource
	}{
		"UnawareLaterFunctionDoesNotEraseEdges": {
			reason: "A function that leaves dependencies unset has no opinion. Crossplane carries the edges forward on its behalf, so the subnet stays blocked.",
			pipeline: []string{
				"declares-edges",
				"unaware-passthrough",
			},
			responses: map[string]*fnv1.RunFunctionResponse{
				"declares-edges": {
					Desired:      &fnv1.State{Resources: vpcAndSubnet},
					Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")}},
				},
				"unaware-passthrough": {
					// Written before this field existed: passes desired state
					// through, knows nothing about dependencies.
					Desired: &fnv1.State{Resources: vpcAndSubnet},
				},
			},
			want: []ComposedResource{
				{ResourceName: "subnet", Synced: false, Reason: "waiting for [vpc] to be ready"},
				{ResourceName: "vpc", Synced: true},
			},
		},
		"LaterFunctionCanAddEdges": {
			reason: "A function may constrain resources an earlier step contributed.",
			pipeline: []string{
				"creates-resources",
				"declares-edges",
			},
			responses: map[string]*fnv1.RunFunctionResponse{
				"creates-resources": {
					Desired: &fnv1.State{Resources: vpcAndSubnet},
				},
				"declares-edges": {
					Desired:      &fnv1.State{Resources: vpcAndSubnet},
					Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")}},
				},
			},
			want: []ComposedResource{
				{ResourceName: "subnet", Synced: false, Reason: "waiting for [vpc] to be ready"},
				{ResourceName: "vpc", Synced: true},
			},
		},
		"LaterFunctionCanReplaceEdges": {
			reason: "A function that has an opinion returns the whole set, replacing what came before.",
			pipeline: []string{
				"declares-edges",
				"overrides-edges",
			},
			responses: map[string]*fnv1.RunFunctionResponse{
				"declares-edges": {
					Desired:      &fnv1.State{Resources: vpcAndSubnet},
					Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")}},
				},
				"overrides-edges": {
					Desired: &fnv1.State{Resources: vpcAndSubnet},
					// Reverses the constraint.
					Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{dependsOn("vpc", "subnet")}},
				},
			},
			want: []ComposedResource{
				{ResourceName: "subnet", Synced: true},
				{ResourceName: "vpc", Synced: false, Reason: "waiting for [subnet] to be ready"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := FunctionRunnerFn(func(_ context.Context, name string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
				return tc.responses[name], nil
			})

			c := NewFunctionComposer(
				&test.MockClient{
					MockPatch:              test.NewMockPatchFn(nil),
					MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
					MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
				},
				&test.MockClient{},
				r,
				orderingParams(tc.observed, nil, true)...,
			)

			res, err := c.Compose(context.Background(), WithParentLabel(), steps(tc.pipeline...))
			if err != nil {
				t.Fatalf("\n%s\nCompose(...): unexpected error: %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want, res.Composed, cmpopts.SortSlices(func(a, b ComposedResource) bool {
				return a.ResourceName < b.ResourceName
			})); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestFunctionComposeOrderingEdgeRetention covers the case a function is most
// likely to get wrong: dropping a resource from desired state, and dropping its
// edges in the same response, at exactly the moment the collector needs them.
func TestFunctionComposeOrderingEdgeRetention(t *testing.T) {
	observed := ComposedResourceStates{
		"vpc":    observedResource("vpc"),
		"subnet": observedResource("subnet"),
	}

	gc := ComposedResourceStates{}

	r := FunctionRunnerFn(func(_ context.Context, name string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		switch name {
		case "declares-edges":
			return &fnv1.RunFunctionResponse{
				Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{
					"vpc":    composedResource("vpc", true),
					"subnet": composedResource("subnet", true),
				}},
				Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")}},
			}, nil
		default:
			// Decides the VPC should go, and drops its edge in the same breath.
			return &fnv1.RunFunctionResponse{
				Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{
					"subnet": composedResource("subnet", true),
				}},
				Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{}},
			}, nil
		}
	})

	c := NewFunctionComposer(
		&test.MockClient{
			MockPatch:              test.NewMockPatchFn(nil),
			MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
			MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
		},
		&test.MockClient{},
		r,
		orderingParams(observed, &gc, true)...,
	)

	res, err := c.Compose(context.Background(), WithParentLabel(), steps("declares-edges", "removes-vpc"))
	if err != nil {
		t.Fatalf("Compose(...): unexpected error: %v", err)
	}

	// The retained edge means the VPC isn't offered to the collector while the
	// subnet that depends on it is still around.
	if _, offered := gc["vpc"]; offered {
		t.Errorf("Retaining the edge should stop the VPC being collected while the subnet still exists, but it was offered to the collector")
	}

	// The pipeline wants the VPC gone and wants the subnet, which depends on
	// it, to stay. Waiting cannot resolve that, so the VPC is reported rather
	// than left to stall. The subnet already exists and keeps reconciling:
	// ordering gates creation, not updates.
	got := map[ResourceName]ComposedResource{}
	for _, cd := range res.Composed {
		got[cd.ResourceName] = cd
	}

	if cd, ok := got["vpc"]; !ok || cd.Synced {
		t.Errorf("The VPC can never be deleted while the subnet depends on it, so it should be reported as not synced, got %+v", cd)
	}

	if cd := got["vpc"]; !strings.Contains(cd.Reason, "cannot be satisfied") {
		t.Errorf("The VPC's reason should say the contradiction can't be waited out, got %q", cd.Reason)
	}

	if cd, ok := got["subnet"]; !ok || !cd.Synced {
		t.Errorf("The subnet exists already, so it should keep being applied, got %+v", cd)
	}

	// A contradiction can never resolve by waiting, so it must not be folded
	// into the summary of resources that are merely waiting. It gets a warning
	// of its own whichever side of the graph reports it.
	warned := false

	for _, e := range res.Events {
		if e.Type == event.TypeWarning && strings.Contains(e.Message, "contradict each other") {
			warned = true
		}
	}

	if !warned {
		t.Errorf("A contradiction should be reported as a warning, not folded into the waiting summary. Got events: %+v", res.Events)
	}

	// An XR that can never converge must not report Available. The apply-side
	// gate has always withheld readiness from a deadlocked resource; the
	// delete-side gate has to do the same, or moving a contradiction from one
	// side to the other quietly changes what the XR claims about itself.
	if cd := got["vpc"]; cd.Ready {
		t.Errorf("A deadlocked resource must not count towards the XR's readiness, got %+v", cd)
	}
}

// TestRetainDependenciesKeyCollision guards against a specific regression: a
// naive resource+"->"+target string key for detecting whether an edge is
// still present would let two unrelated edges collide, because composed
// resource names are arbitrary strings a function pipeline chooses and
// aren't guaranteed free of "->".
func TestRetainDependenciesKeyCollision(t *testing.T) {
	// Two distinct edges that produce the identical string
	// "a->b" + "->" + "c" == "a" + "->" + "b->c" == "a->b->c" under the old
	// key scheme.
	kept := dependsOn("a->b", "c")
	dropped := dependsOn("a", "b->c")

	prev := []*fnv1.Dependency{kept, dropped}
	next := []*fnv1.Dependency{kept} // The function didn't return "dropped" this pass.

	// "b->c" has left desired state but is still observed - exactly the
	// pending-deletion case retention exists to protect.
	state := ordering.State{Composed: map[string]ordering.ComposedState{
		"a->b": {Desired: true, Observed: true, Ready: true},
		"c":    {Desired: true, Observed: true, Ready: true},
		"a":    {Desired: true, Observed: true, Ready: true},
		"b->c": {Observed: true},
	}}

	got := retainDependencies(prev, next, state)

	found := false

	for _, d := range got {
		if proto.Equal(d, dropped) {
			found = true
		}
	}

	if !found {
		t.Errorf("retainDependencies(...) did not retain an edge to a pending-deletion resource; its key likely collided with an unrelated edge's key: %+v", got)
	}
}

// TestFunctionComposeOrderingInvalid covers graphs Crossplane refuses.
func TestFunctionComposeOrderingInvalid(t *testing.T) {
	cases := map[string]struct {
		reason string
		deps   []*fnv1.Dependency
	}{
		"Cycle": {
			reason: "A cycle can never be satisfied, so we fail rather than block forever.",
			deps: []*fnv1.Dependency{
				dependsOn("subnet", "vpc"),
				dependsOn("vpc", "subnet"),
			},
		},
		"UndeclaredRequirement": {
			reason: "An edge naming a requirement no function asked for is an authoring error.",
			deps: []*fnv1.Dependency{{
				Resource: "subnet",
				DependsOn: &fnv1.Dependency_RequiredResource{
					RequiredResource: &fnv1.RequiredResourceDependency{RequirementName: "nonexistent"},
				},
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
				return &fnv1.RunFunctionResponse{
					Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{
						"vpc":    composedResource("vpc", true),
						"subnet": composedResource("subnet", true),
					}},
					Dependencies: &fnv1.Dependencies{Items: tc.deps},
				}, nil
			})

			c := NewFunctionComposer(
				&test.MockClient{
					MockPatch:              test.NewMockPatchFn(nil),
					MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
					MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
				},
				&test.MockClient{},
				r,
				orderingParams(nil, nil, true)...,
			)

			if _, err := c.Compose(context.Background(), WithParentLabel(), singleStep()); err == nil {
				t.Errorf("\n%s\nCompose(...): want error, got none", tc.reason)
			}
		})
	}
}

// TestFunctionComposeOrderingStaleEdge covers the case that broke composition
// outright before Prune existed: a function that keeps declaring edges for
// resources it has finished deleting.
func TestFunctionComposeOrderingStaleEdge(t *testing.T) {
	r := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		return &fnv1.RunFunctionResponse{
			Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{
				"subnet": composedResource("subnet", true),
			}},
			// The VPC is long gone, but a function returning a fixed set of
			// rules keeps declaring the edge.
			Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")}},
		}, nil
	})

	c := NewFunctionComposer(
		&test.MockClient{
			MockPatch:              test.NewMockPatchFn(nil),
			MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
			MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
		},
		&test.MockClient{},
		r,
		orderingParams(nil, nil, true)...,
	)

	res, err := c.Compose(context.Background(), WithParentLabel(), singleStep())
	if err != nil {
		t.Fatalf("A stale edge should be pruned, not fail composition: %v", err)
	}

	want := []ComposedResource{{ResourceName: "subnet", Ready: true, Synced: true}}
	if diff := cmp.Diff(want, res.Composed); diff != "" {
		t.Errorf("Compose(...): -want, +got:\n%s", diff)
	}
}

// TestDependenciesSurviveWireRoundTrip pins down why dependencies are wrapped
// in a message rather than being a bare repeated field.
//
// Crossplane treats an unset dependencies field as "this function has no
// opinion, carry my constraints forward," and an empty one as "this function
// wants no constraints." Proto3 has no presence for repeated fields, so a bare
// repeated field would make those two indistinguishable after serialization -
// and, worse, distinguishable in process, so a function tested with crank
// render would behave differently in a cluster.
func TestDependenciesSurviveWireRoundTrip(t *testing.T) {
	cases := map[string]struct {
		reason  string
		rsp     *fnv1.RunFunctionResponse
		wantSet bool
	}{
		"UnsetStaysUnset": {
			reason:  "A function that never touches the field has no opinion about ordering.",
			rsp:     &fnv1.RunFunctionResponse{},
			wantSet: false,
		},
		"EmptyStaysEmpty": {
			reason:  "A function that returns no constraints wants no constraints, and must not be mistaken for one with no opinion.",
			rsp:     &fnv1.RunFunctionResponse{Dependencies: &fnv1.Dependencies{}},
			wantSet: true,
		},
		"PopulatedStaysPopulated": {
			reason: "The ordinary case.",
			rsp: &fnv1.RunFunctionResponse{Dependencies: &fnv1.Dependencies{
				Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")},
			}},
			wantSet: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b, err := proto.Marshal(tc.rsp)
			if err != nil {
				t.Fatalf("proto.Marshal(...): %v", err)
			}

			got := &fnv1.RunFunctionResponse{}
			if err := proto.Unmarshal(b, got); err != nil {
				t.Fatalf("proto.Unmarshal(...): %v", err)
			}

			if set := got.GetDependencies() != nil; set != tc.wantSet {
				t.Errorf("\n%s\nAfter a wire round trip: want set=%v, got set=%v", tc.reason, tc.wantSet, set)
			}

			if diff := cmp.Diff(tc.rsp.GetDependencies().GetItems(), got.GetDependencies().GetItems(), protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nAfter a wire round trip: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestFunctionComposeOrderingTerminatingDependent covers the case that made
// delete ordering unravel: a dependent that takes more than one reconcile to
// disappear.
//
// Every managed resource with a finalizer behaves this way - a delete is
// issued, the object sits in Terminating for as long as the external delete
// takes, and it stays in observed state the whole time. If it stops being
// referenced while it's terminating it vanishes from observed on the next
// pass, its edges are pruned as stale, and the resource it was blocking gets
// deleted out from under it.
func TestFunctionComposeOrderingTerminatingDependent(t *testing.T) {
	// subnet depends on vpc. Both have left desired state, and subnet has been
	// deleted but is still terminating.
	terminating := observedResource("subnet")
	terminating.Resource.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})

	observed := ComposedResourceStates{
		"vpc":    observedResource("vpc"),
		"subnet": terminating,
	}

	gc := ComposedResourceStates{}
	refs := []string{}

	r := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		return &fnv1.RunFunctionResponse{
			Desired:      &fnv1.State{Resources: map[string]*fnv1.Resource{}},
			Dependencies: &fnv1.Dependencies{Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")}},
		}, nil
	})

	c := NewFunctionComposer(
		&test.MockClient{
			MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
				if u, ok := obj.(*composite.Unstructured); ok {
					for _, ref := range u.GetResourceReferences() {
						refs = append(refs, ref.Name)
					}
				}

				return nil
			},
			MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
			MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
		},
		&test.MockClient{},
		r,
		orderingParams(observed, &gc, true)...,
	)

	if _, err := c.Compose(context.Background(), WithParentLabel(), singleStep()); err != nil {
		t.Fatalf("Compose(...): unexpected error: %v", err)
	}

	// The vpc must not be offered to the collector while the subnet that
	// depends on it is still terminating.
	if _, offered := gc["vpc"]; offered {
		t.Errorf("The vpc was offered to the garbage collector while a terminating dependent still existed")
	}

	// Both must stay referenced. Losing the subnet's reference is what makes
	// the vpc look unblocked on the next pass.
	for _, want := range []string{"vpc", "subnet"} {
		if !slices.Contains(refs, observed[ResourceName(want)].Resource.GetName()) {
			t.Errorf("Composed resource %q must stay in spec.resourceRefs while it exists, or the graph loses track of it", want)
		}
	}
}

// TestFunctionComposeOrderingBlockedNotReferenced covers what a blocked
// resource looks like from outside.
//
// References are written before resources are applied, so a created resource
// can't be leaked. A resource the graph is holding back was never applied, so
// referencing it points anything reading spec.resourceRefs at an object that
// doesn't exist - `crossplane trace` reports "not found", which reads as a
// failure rather than as waiting.
func TestFunctionComposeOrderingBlockedNotReferenced(t *testing.T) {
	refs := []string{}

	r := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		return &fnv1.RunFunctionResponse{
			Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{
				"vpc":    composedResource("vpc", false),
				"subnet": composedResource("subnet", false),
			}},
			Dependencies: &fnv1.Dependencies{
				Items: []*fnv1.Dependency{dependsOn("subnet", "vpc")},
			},
		}, nil
	})

	c := NewFunctionComposer(
		&test.MockClient{
			MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
				if u, ok := obj.(*composite.Unstructured); ok {
					for _, ref := range u.GetResourceReferences() {
						refs = append(refs, ref.Name)
					}
				}

				return nil
			},
			MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
			MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
		},
		&test.MockClient{},
		r,
		// Nothing observed: neither resource exists yet, and subnet is blocked.
		orderingParams(nil, nil, true)...,
	)

	res, err := c.Compose(context.Background(), WithParentLabel(), singleStep())
	if err != nil {
		t.Fatalf("Compose(...): unexpected error: %v", err)
	}

	// The vpc is applied, so it is referenced. The subnet is not applied, so
	// it must not be.
	if len(refs) != 1 {
		t.Errorf("Only the applied resource should be referenced, got %d references: %v", len(refs), refs)
	}

	// It still has to be reported, or the XR would look complete.
	var reported bool

	for _, cd := range res.Composed {
		if cd.ResourceName == "subnet" {
			reported = true

			if cd.Synced {
				t.Errorf("A resource held back by the graph must not be reported as synced")
			}
		}
	}

	if !reported {
		t.Errorf("A resource held back by the graph must still appear in the composed resources")
	}
}

// TestFunctionComposeOrderingRequirementNameSharedBetweenSteps covers what a
// requirement name means when two steps both use it.
//
// Requirement names are per step everywhere else in core: each function gets
// its own resources under its own names. Ordering edges are pipeline-wide,
// though, so the sets are unioned rather than overwritten. Overwriting would
// let whichever step ran last decide what a name means, and an edge from the
// first function would be gated on the second function's resources without
// anything saying so.
func TestFunctionComposeOrderingRequirementNameSharedBetweenSteps(t *testing.T) {
	// Both steps call their requirement "environment". The edge belongs to the
	// first step, whose requirement matches something that is NOT ready. The
	// second step's matches something that is.
	//
	// The polarity matters: with the readiness the other way round, both
	// unioning and overwriting would block, and the test would pass either way.
	// Overwriting lets the second step's ready resource decide what
	// "environment" means, and the edge from the first step stops holding
	// anything back.
	fetch := func(_ context.Context, sel *fnv1.ResourceSelector) (*fnv1.Resources, error) {
		if sel.GetMatchName() == "environment-a" {
			return &fnv1.Resources{Items: []*fnv1.Resource{{Resource: MustStruct(map[string]any{
				"apiVersion": "example.org/v1",
				"kind":       "Thing",
				"metadata":   map[string]any{"name": "environment-a"},
				"status": map[string]any{"conditions": []any{
					map[string]any{"type": "Ready", "status": "False"},
				}},
			})}}}, nil
		}

		return &fnv1.Resources{Items: []*fnv1.Resource{{Resource: MustStruct(map[string]any{
			"apiVersion": "example.org/v1",
			"kind":       "Thing",
			"metadata":   map[string]any{"name": "environment-b"},
			"status": map[string]any{"conditions": []any{
				map[string]any{"type": "Ready", "status": "True"},
			}},
		})}}}, nil
	}

	desired := map[string]*fnv1.Resource{"app": composedResource("app", true)}

	runner := xfn.NewFetchingFunctionRunner(
		FunctionRunnerFn(func(_ context.Context, name string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
			match := "environment-a"
			if name == "second" {
				match = "environment-b"
			}

			rsp := &fnv1.RunFunctionResponse{
				Desired: &fnv1.State{Resources: desired},
				Requirements: &fnv1.Requirements{Resources: map[string]*fnv1.ResourceSelector{
					"environment": {
						ApiVersion: "example.org/v1", Kind: "Thing",
						Match: &fnv1.ResourceSelector_MatchName{MatchName: match},
					},
				}},
			}

			if name == "first" {
				rsp.Dependencies = &fnv1.Dependencies{Items: []*fnv1.Dependency{{
					Resource: "app",
					DependsOn: &fnv1.Dependency_RequiredResource{RequiredResource: &fnv1.RequiredResourceDependency{
						RequirementName: "environment",
					}},
				}}}
			}

			return rsp, nil
		}),
		xfn.RequiredResourcesFetcherFn(fetch),
		xfn.NopRequiredSchemasFetcher{},
	)

	c := NewFunctionComposer(
		&test.MockClient{
			MockPatch:              test.NewMockPatchFn(nil),
			MockStatusPatch:        test.NewMockSubResourcePatchFn(nil),
			MockIsObjectNamespaced: test.NewMockIsObjectNamespacedFn(nil, false),
		},
		&test.MockClient{},
		runner,
		orderingParams(nil, nil, true)...,
	)

	res, err := c.Compose(context.Background(), WithParentLabel(), steps("first", "second"))
	if err != nil {
		t.Fatalf("Compose(...): unexpected error: %v", err)
	}

	if len(res.Composed) != 1 {
		t.Fatalf("Compose(...): want 1 composed resource, got %#v", res.Composed)
	}

	if res.Composed[0].Synced {
		t.Errorf("A requirement name two steps declared covers what either matched, so an edge naming it waits for both - including the first step's, which isn't ready. Overwriting would let the second step's ready resource answer for a name the first step also used. Got %#v", res.Composed[0])
	}
}
