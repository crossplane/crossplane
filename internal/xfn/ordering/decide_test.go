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

package ordering

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// on returns an edge from resource to a composed dependency.
func on(resource, dependsOn string) Edge {
	return Edge{Resource: resource, DependsOn: Target{ComposedResource: dependsOn}}
}

// onReq returns an edge from resource to a required resource.
func onReq(resource, requirement string) Edge {
	return Edge{Resource: resource, DependsOn: Target{RequiredResource: &RequiredRef{RequirementName: requirement}}}
}

// live is a resource that is desired, observed and ready.
var live = ComposedState{Desired: true, Observed: true, Ready: true}

// pending is a resource that is desired but not yet created.
var pending = ComposedState{Desired: true}

// notReady is a resource that exists but hasn't come up yet.
var notReady = ComposedState{Desired: true, Observed: true}

// doomed is a resource that has left desired state but still exists.
var doomed = ComposedState{Observed: true}

func TestDecideCreate(t *testing.T) {
	cases := map[string]struct {
		reason string
		graph  *Graph
		state  State
		want   Decisions
	}{
		"NoEdgesAppliesEverything": {
			reason: "With no ordering constraints every desired resource is applied at once.",
			graph:  New(),
			state: State{Composed: map[string]ComposedState{
				"vpc":    pending,
				"subnet": pending,
			}},
			want: Decisions{Apply: []string{"subnet", "vpc"}, Blocked: map[string]Decision{}},
		},
		"DependentWaitsForUncreatedDependency": {
			reason: "A subnet is not applied until the VPC it depends on exists and is ready.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    pending,
				"subnet": pending,
			}},
			want: Decisions{
				Apply: []string{"vpc"},
				Blocked: map[string]Decision{
					"subnet": {Blocked: true, BlockedBy: []string{"vpc"}, Reason: "waiting for [vpc] to be ready"},
				},
			},
		},
		"DependentWaitsForUnreadyDependency": {
			reason: "Existing but not ready is still not ready.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    notReady,
				"subnet": pending,
			}},
			want: Decisions{
				Apply: []string{"vpc"},
				Blocked: map[string]Decision{
					"subnet": {Blocked: true, BlockedBy: []string{"vpc"}, Reason: "waiting for [vpc] to be ready"},
				},
			},
		},
		"DependentProceedsOnceDependencyReady": {
			reason: "Once the VPC is ready the subnet is applied.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    live,
				"subnet": pending,
			}},
			want: Decisions{Apply: []string{"subnet", "vpc"}, Blocked: map[string]Decision{}},
		},
		"TransitiveChainReleasesOneWavePerPass": {
			reason: "A three-deep chain releases one resource at a time.",
			graph:  New(on("subnet", "vpc"), on("instance", "subnet"), on("instance", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":      live,
				"subnet":   pending,
				"instance": pending,
			}},
			want: Decisions{
				Apply: []string{"subnet", "vpc"},
				Blocked: map[string]Decision{
					"instance": {Blocked: true, BlockedBy: []string{"subnet"}, Reason: "waiting for [subnet] to be ready"},
				},
			},
		},
		"UpdateOfLiveResourceIsNotGated": {
			reason: "Ordering gates creation, not updates. The subnet already exists, so a VPC that has gone un-ready must not freeze it - the update being held back could be the one that fixes the VPC.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    notReady,
				"subnet": live,
			}},
			want: Decisions{Apply: []string{"subnet", "vpc"}, Blocked: map[string]Decision{}},
		},
		"UpdateIsNotGatedByAnUnmatchedRequirement": {
			reason: "The same rule covers required resources. An edge onto a requirement that matched nothing is a mistake worth reporting, but not one worth freezing a live resource over.",
			graph:  New(onReq("subnet", "net")),
			state: State{
				Composed: map[string]ComposedState{"subnet": live},
				Required: map[string]RequiredState{"net": {Ready: map[string]bool{
					RequiredResourceKey("", "other"): true,
				}}},
			},
			want: Decisions{Apply: []string{"subnet"}, Blocked: map[string]Decision{}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.graph.Decide(tc.state)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDecide(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDecideDelete(t *testing.T) {
	cases := map[string]struct {
		reason string
		graph  *Graph
		state  State
		want   Decisions
	}{
		"TeardownDeletesLeavesFirst": {
			reason: "The subnet depends on the VPC, so it is deleted first. This is pass 1 of the design's worked example.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    doomed,
				"subnet": doomed,
			}},
			want: Decisions{
				Delete: []string{"subnet"},
				Blocked: map[string]Decision{
					"vpc": {Blocked: true, BlockedBy: []string{"subnet"}, Reason: "waiting for [subnet] to be deleted"},
				},
			},
		},
		"TeardownProceedsOnceDependentGone": {
			reason: "Pass 2: the subnet has left observed state, so the VPC may go.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    doomed,
				"subnet": {},
			}},
			want: Decisions{Delete: []string{"vpc"}, Blocked: map[string]Decision{}},
		},
		"TerminatingDependentStillBlocks": {
			reason: "A dependent under foreground propagation lingers in observed state; it has not finished going away.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    doomed,
				"subnet": doomed, // Observed, not desired: terminating.
			}},
			want: Decisions{
				Delete: []string{"subnet"},
				Blocked: map[string]Decision{
					"vpc": {Blocked: true, BlockedBy: []string{"subnet"}, Reason: "waiting for [subnet] to be deleted"},
				},
			},
		},
		"ContradictionIsReportedNotStalled": {
			reason: "The pipeline dropped the VPC but kept a subnet that depends on it. The VPC can never be deleted, because the subnet is not leaving. The subnet already exists, so it keeps reconciling - the deadlock is the VPC's, and it is reported there rather than by freezing a healthy resource.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    doomed,
				"subnet": live,
			}},
			want: Decisions{
				Apply: []string{"subnet"},
				Blocked: map[string]Decision{
					"vpc": {
						Blocked:    true,
						BlockedBy:  []string{"subnet"},
						Reason:     "cannot be satisfied: the pipeline still wants [subnet], which depends on it",
						Deadlocked: true,
					},
				},
			},
		},
		"ContradictionBlocksADependentThatDoesNotExistYet": {
			reason: "The apply-side gate still catches this shape when the dependent has never been created. It cannot be created, because the thing it depends on is on its way out and will never be ready again.",
			graph:  New(on("subnet", "vpc")),
			state: State{Composed: map[string]ComposedState{
				"vpc":    doomed,
				"subnet": pending,
			}},
			want: Decisions{
				Delete: []string{"vpc"},
				Blocked: map[string]Decision{
					"subnet": {
						Blocked:    true,
						BlockedBy:  []string{"vpc"},
						Reason:     "cannot be satisfied: vpc is being deleted by the pipeline",
						Deadlocked: true,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.graph.Decide(tc.state)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDecide(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDecideCreateBeforeDestroy(t *testing.T) {
	cbd := Edge{Resource: "new", DependsOn: Target{ComposedResource: "old"}, CreateBeforeDestroy: true}

	cases := map[string]struct {
		reason string
		graph  *Graph
		state  State
		want   Decisions
	}{
		"ReplacementCreatedWhilePredecessorStillExists": {
			reason: "create-before-destroy lets the replacement be applied without waiting for its predecessor to go.",
			graph:  New(cbd),
			state: State{Composed: map[string]ComposedState{
				"old": doomed,
				"new": pending,
			}},
			want: Decisions{
				Apply: []string{"new"},
				Blocked: map[string]Decision{
					"old": {Blocked: true, BlockedBy: []string{"new"}, Reason: "waiting for [new] to be created and ready"},
				},
			},
		},
		"PredecessorDeletedOnceReplacementReady": {
			reason: "The predecessor may only go once the replacement exists and is ready.",
			graph:  New(cbd),
			state: State{Composed: map[string]ComposedState{
				"old": doomed,
				"new": live,
			}},
			want: Decisions{Apply: []string{"new"}, Delete: []string{"old"}, Blocked: map[string]Decision{}},
		},
		"PredecessorHeldWhileReplacementUnready": {
			reason: "A replacement that exists but isn't ready is not yet a reason to tear down the predecessor.",
			graph:  New(cbd),
			state: State{Composed: map[string]ComposedState{
				"old": doomed,
				"new": notReady,
			}},
			want: Decisions{
				Apply: []string{"new"},
				Blocked: map[string]Decision{
					"old": {Blocked: true, BlockedBy: []string{"new"}, Reason: "waiting for [new] to be created and ready"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.graph.Decide(tc.state)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDecide(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDecideRequiredResource(t *testing.T) {
	cases := map[string]struct {
		reason string
		graph  *Graph
		state  State
		want   Decisions
	}{
		"WaitsForRequiredResource": {
			reason: "A composed resource may depend on a resource the XR requires but does not compose.",
			graph:  New(onReq("instance", "shared-vpc")),
			state: State{
				Composed: map[string]ComposedState{"instance": pending},
				Required: map[string]RequiredState{"shared-vpc": {Ready: map[string]bool{"vpc-1": false}}},
			},
			want: Decisions{Blocked: map[string]Decision{
				"instance": {Blocked: true, BlockedBy: []string{"requirement shared-vpc"}, Reason: "waiting for [requirement shared-vpc] to be ready"},
			}},
		},
		"ProceedsOnceRequiredResourceReady": {
			reason: "Once the required resource is ready the dependent is applied.",
			graph:  New(onReq("instance", "shared-vpc")),
			state: State{
				Composed: map[string]ComposedState{"instance": pending},
				Required: map[string]RequiredState{"shared-vpc": {Ready: map[string]bool{"vpc-1": true}}},
			},
			want: Decisions{Apply: []string{"instance"}, Blocked: map[string]Decision{}},
		},
		"EmptyMatchBlocks": {
			reason: "A requirement that matched nothing is unsatisfied, not vacuously satisfied. It says so in its own words: this is answered by creating the resource, not by waiting for one that exists to come up.",
			graph:  New(onReq("instance", "shared-vpc")),
			state: State{
				Composed: map[string]ComposedState{"instance": pending},
				Required: map[string]RequiredState{"shared-vpc": {Ready: map[string]bool{}}},
			},
			want: Decisions{Blocked: map[string]Decision{
				"instance": {Blocked: true, BlockedBy: []string{"requirement shared-vpc"}, Reason: "waiting for [requirement shared-vpc] to match a resource"},
			}},
		},
		"UnfetchedRequirementIsAnOrdinaryWait": {
			reason: "A requirement Crossplane hasn't fetched this pass is not the same as one that matched nothing. It resolves on its own, and nobody needs to go create anything.",
			graph:  New(onReq("instance", "shared-vpc")),
			state: State{
				Composed: map[string]ComposedState{"instance": pending},
				Required: map[string]RequiredState{},
			},
			want: Decisions{Blocked: map[string]Decision{
				"instance": {Blocked: true, BlockedBy: []string{"requirement shared-vpc"}, Reason: "waiting for [requirement shared-vpc] to be ready"},
			}},
		},
		"MixedWaitsAreReportedSeparately": {
			reason: "A resource waiting on one requirement that matched nothing and another that isn't ready is waiting for two different things, and each is named for what it is.",
			graph:  New(onReq("instance", "shared-vpc"), onReq("instance", "shared-subnet")),
			state: State{
				Composed: map[string]ComposedState{"instance": pending},
				Required: map[string]RequiredState{
					"shared-vpc":    {Ready: map[string]bool{"vpc-1": false}},
					"shared-subnet": {Ready: map[string]bool{}},
				},
			},
			want: Decisions{Blocked: map[string]Decision{
				"instance": {
					Blocked:   true,
					BlockedBy: []string{"requirement shared-subnet", "requirement shared-vpc"},
					Reason: "waiting for [requirement shared-vpc] to be ready; " +
						"waiting for [requirement shared-subnet] to match a resource",
				},
			}},
		},
		"AllMatchesMustBeReady": {
			reason: "With no name set, every resource the requirement matched must be ready.",
			graph:  New(onReq("instance", "shared-vpc")),
			state: State{
				Composed: map[string]ComposedState{"instance": pending},
				Required: map[string]RequiredState{"shared-vpc": {Ready: map[string]bool{"vpc-1": true, "vpc-2": false}}},
			},
			want: Decisions{Blocked: map[string]Decision{
				"instance": {Blocked: true, BlockedBy: []string{"requirement shared-vpc"}, Reason: "waiting for [requirement shared-vpc] to be ready"},
			}},
		},
		"RequiredResourceIsNeverDeleted": {
			reason: "A required resource is not composed, so teardown of the dependent is not gated on it.",
			graph:  New(onReq("instance", "shared-vpc")),
			state: State{
				Composed: map[string]ComposedState{"instance": doomed},
				Required: map[string]RequiredState{"shared-vpc": {Ready: map[string]bool{"vpc-1": true}}},
			},
			want: Decisions{Delete: []string{"instance"}, Blocked: map[string]Decision{}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.graph.Decide(tc.state)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDecide(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDecideRequiredResourceNamespace(t *testing.T) {
	graph := New(Edge{Resource: "instance", DependsOn: Target{RequiredResource: &RequiredRef{
		RequirementName: "networks",
		Name:            "shared",
		Namespace:       "platform-a",
	}}})

	got := graph.Decide(State{
		Composed: map[string]ComposedState{"instance": pending},
		Required: map[string]RequiredState{"networks": {Ready: map[string]bool{
			RequiredResourceKey("platform-a", "shared"): true,
			RequiredResourceKey("platform-b", "shared"): false,
		}}},
	})

	if len(got.Apply) != 1 || got.Apply[0] != "instance" {
		t.Errorf("A ready namespaced requirement should unblock its dependent, got %#v", got)
	}
}

func TestDecideRequiredResourceNotMatched(t *testing.T) {
	// A requirement that matched the same name in two namespaces. Keying
	// readiness by namespace and name keeps this deterministic, but it also
	// means an edge that names the resource without its namespace matches
	// nothing at all.
	twoNamespaces := State{
		Composed: map[string]ComposedState{"app": pending},
		Required: map[string]RequiredState{"net": {Ready: map[string]bool{
			RequiredResourceKey("team-a", "network"): true,
			RequiredResourceKey("team-b", "network"): true,
		}}},
	}

	cases := map[string]struct {
		reason         string
		state          State
		ref            RequiredRef
		wantDeadlocked bool
		wantReason     string
	}{
		"NamespaceOmitted": {
			reason:         "Naming a namespaced resource without its namespace matches nothing. Reporting that as 'not ready' would send someone to look at a resource that is already ready, so say what actually happened.",
			state:          twoNamespaces,
			ref:            RequiredRef{RequirementName: "net", Name: "network"},
			wantDeadlocked: true,
			wantReason: `cannot be satisfied: requirement net matched no resource named "network"; ` +
				`namespace is not set, and a namespaced resource needs one`,
		},
		"WrongNamespace": {
			reason:         "Naming a namespace nothing matched is the same kind of mistake, and names the namespace back.",
			state:          twoNamespaces,
			ref:            RequiredRef{RequirementName: "net", Name: "network", Namespace: "team-c"},
			wantDeadlocked: true,
			wantReason:     "cannot be satisfied: requirement net matched no resource team-c/network",
		},
		"MatchedButNotReady": {
			reason: "A resource that matched and isn't ready yet is an ordinary wait, not a mistake.",
			state: State{
				Composed: map[string]ComposedState{"app": pending},
				Required: map[string]RequiredState{"net": {Ready: map[string]bool{
					RequiredResourceKey("team-a", "network"): false,
				}}},
			},
			ref:            RequiredRef{RequirementName: "net", Name: "network", Namespace: "team-a"},
			wantDeadlocked: false,
			wantReason:     "waiting for [requirement net] to be ready",
		},
		"NothingFetchedYet": {
			reason: "A requirement Crossplane hasn't fetched yet will resolve on a later pass, so it must not look like a mistake.",
			state: State{
				Composed: map[string]ComposedState{"app": pending},
				Required: map[string]RequiredState{},
			},
			ref:            RequiredRef{RequirementName: "net", Name: "network", Namespace: "team-a"},
			wantDeadlocked: false,
			wantReason:     "waiting for [requirement net] to be ready",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := New(Edge{Resource: "app", DependsOn: Target{RequiredResource: &tc.ref}})
			got := g.Decide(tc.state).Blocked["app"]

			if got.Deadlocked != tc.wantDeadlocked {
				t.Errorf("\n%s\nDecide(...): want Deadlocked=%v, got %v", tc.reason, tc.wantDeadlocked, got.Deadlocked)
			}

			if got.Reason != tc.wantReason {
				t.Errorf("\n%s\nDecide(...): reason\n  want: %s\n  got:  %s", tc.reason, tc.wantReason, got.Reason)
			}
		})
	}
}

func TestDecideDeleteMixedWaits(t *testing.T) {
	// A resource with both an ordinary dependent and a create-before-destroy
	// successor waits on each for the opposite reason. Reporting them with one
	// phrasing would tell someone the replacement has to be deleted, which is
	// the reverse of what has to happen.
	g := New(
		on("subnet", "vpc"),
		Edge{Resource: "vpc-v2", DependsOn: Target{ComposedResource: "vpc"}, CreateBeforeDestroy: true},
	)

	s := State{Composed: map[string]ComposedState{
		"vpc":    doomed,
		"subnet": doomed,
		"vpc-v2": pending,
	}}

	want := Decision{
		Blocked:   true,
		BlockedBy: []string{"subnet", "vpc-v2"},
		Reason:    "waiting for [subnet] to be deleted; waiting for [vpc-v2] to be created and ready",
	}

	if diff := cmp.Diff(want, g.Decide(s).Blocked["vpc"]); diff != "" {
		t.Errorf("\nEach kind of wait is reported for what it is.\nDecide(...): -want, +got:\n%s", diff)
	}
}
