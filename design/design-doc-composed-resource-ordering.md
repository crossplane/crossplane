# Composed Resource Ordering <!-- omit in toc -->

* Owner: Steven Borrelli (@stevendborrelli)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Table of Contents <!-- omit in toc -->

* [Background](#background)
* [Proposal](#proposal)
* [Goals](#goals)
* [Non-Goals](#non-goals)
* [Proposed Implementation](#proposed-implementation)
  * [Updating `RunFunctionRequest` and `RunFunctionResponse` to Support Dependencies](#updating-runfunctionrequest-and-runfunctionresponse-to-support-dependencies)
  * [Add the ability to Depend on a Required Resource in the gRPC Message](#add-the-ability-to-depend-on-a-required-resource-in-the-grpc-message)
  * [Dependencies accumulate like Desired Resources](#dependencies-accumulate-like-desired-resources)
  * [Backward Compatibility for Functions](#backward-compatibility-for-functions)
  * [Validation in the Core Engine for Valid and Acyclic Graphs](#validation-in-the-core-engine-for-valid-and-acyclic-graphs)
  * [Using the Graph to Order Creation and Deletion](#using-the-graph-to-order-creation-and-deletion)
    * [Determining Readiness](#determining-readiness)
    * [Blocked resources must still be reported](#blocked-resources-must-still-be-reported)
  * [Dealing With Contradictions in the Graph](#dealing-with-contradictions-in-the-graph)
    * [Resources waiting for Dependencies to be Created](#resources-waiting-for-dependencies-to-be-created)
    * [Resources pending Deletion](#resources-pending-deletion)
  * [Persisting Dependencies in `spec.resourceRefs`](#persisting-dependencies-in-specresourcerefs)
  * [Dependency Lifecycle: create-before-destroy](#dependency-lifecycle-create-before-destroy)
* [Enabling Adoption with `function-ordering`](#enabling-adoption-with-function-ordering)
  * [How it works](#how-it-works)
* [API Impact and Capabilities](#api-impact-and-capabilities)
* [Alternatives Considered](#alternatives-considered)

## Background

See
[assets/design-doc-composed-resource-ordering/background.md](assets/design-doc-composed-resource-ordering/background.md)
for the background that led to this proposal.

## Proposal

Implement a directed resource graph in order to enable the ordered creation and
deletion of Composed Resources. This will be accomplished by having the function
pipeline define dependency pairs and core Crossplane managing the graph.

## Goals

* Enable Crossplane's reconciler to sequence creation and deletion of resources.
* Make dependency a first-class representation in the function protocol, so that
  "this resource is waiting" is distinguishable from "this resource is not
  wanted."
* Make it easy for Composition Authors to define dependencies between resources.
* Let Crossplane report accurately when an XR's composition is incomplete
  because a resource is blocked.
* Enable a composed resource's creation to depend on a required resource, so
  that it can depend on a resource outside the XR.
* Maintain consistency with Crossplane patterns: dependencies are accumulated in
  the pipeline, graphs are created by the core.
* Stay backward compatible with functions built with previous SDK versions, and
  enable tooling to adopt dependencies in legacy Compositions.
* Represent dependencies in the XR `resourceRefs` so external tools can read the
  graph.
* Enable ordering policies: for example, create a resource before another is
  deleted.
* Enable a pathway for deprecating Managed Resource References.

## Non-Goals

* Ordering the deletion of resources that belong to different Composite
  Resources. Deletion ordering is scoped to a single XR.
* Changing how a provider determines a managed resource's readiness. For
  composed resources this proposal consumes the readiness verdict the pipeline
  already produces.
* Making the function pipeline run during XR deletion. Ordering on teardown is
  driven by the graph persisted on the XR instead, so it does not need the
  pipeline to run.

## Proposed Implementation

* Update the gRPC messages to support dependency information.
* Update XR types in Crossplane runtime to support composition name and
  `dependsOn` in `spec.resourceRefs`.
* Implement a directed graph in the core composition engine.
* Update Applicator to support ordered creation and deletion.

### Updating `RunFunctionRequest` and `RunFunctionResponse` to Support Dependencies

A dependency is a pair of resources that must be created and deleted in order. A
Subnet cannot be created until the Network exists, and the Subnet must be
deleted before the Network.

```test
Subnet dependsOn Network
```

This is accomplished by adding top-level `dependencies` field to
`RunFunctionRequest` and `RunFunctionResponse`, alongside `desired`, `observed`,
and `context`:

```protobuf
message Dependency {
  // Name of the composed resource that has the dependency.
  // A key into State.resources.
  string resource = 1;

  // Name of the composed resource it depends on.
  // A key into State.resources.
  string depends_on = 2;

  // How this dependency constrains the order its endpoints are created and
  // deleted. Defaults to symmetric ordering. See "Dependency Lifecycle" below.
  DependencyLifecycle lifecycle = 3;
}

enum DependencyLifecycle {
  // Create `depends_on` first, delete it last. The default.
  DEPENDENCY_LIFECYCLE_UNSPECIFIED = 0;

  // `resource` may be created without waiting for `depends_on` to be deleted.
  // It must still exist and be Ready before `depends_on` is deleted.
  DEPENDENCY_LIFECYCLE_CREATE_BEFORE_DESTROY = 1;
}

// Wrapped in a message, not a bare repeated field, so that unset can be told
// apart from empty.
message Dependencies {
  repeated Dependency items = 1;
}

message RunFunctionRequest {
  // ...existing fields: meta = 1, observed = 2, desired = 3, input = 4,
  // context = 5, extra_resources = 6 [deprecated], credentials = 7,
  // required_resources = 8, required_schemas = 9...
  Dependencies dependencies = 10; // next unused field number
}

message RunFunctionResponse {
  // ...existing fields: meta = 1, desired = 2, results = 3, context = 4,
  // requirements = 5, conditions = 6, output = 7...
  Dependencies dependencies = 8; // next unused field number
}
```

A `Dependency` carries no copy of either resource — `resource` and `depends_on`
are the same string keys already used in `State.Resources`. That keeps the
marginal cost of this feature low: a Composite with a few dozen composed
resources rarely has more than a handful of ordering constraints per resource,
so `dependencies` stays small even though `desired` and `observed` already carry
full resource payloads on every call.

Dependencies are defined as resource pairs using desired composed resource
names, so that any step in the pipeline can create and modify the dependency
graph:

```text
[ // (resource, depends_on)
  (subnet-a, vpc),
  (subnet-b, vpc),
  (route-table-a, vpc),
  (route-a, route-table-a)
]
```

The creates a clear separation of responsibility; the Function is responsible
for returning dependency pairs. The Composition engine reads the pairs and
generates the resource graph which it uses to create and delete resources in
order.

### Add the ability to Depend on a Required Resource in the gRPC Message

Dependencies on resources outside the Composite Resource can also be defined
using Kubernetes references and selectors to support use cases where creation
depends on the existence of a centralized Configuration or a resource like a
shared VPC.

Crossplane already has the plumbing. A function declares
`requirements.resources`, core fetches the matching objects and returns them in
`RunFunctionRequest.required_resources`, and the core already tracks those
resources for watches, matched by name or label, alongside the XR's composed
resources. The event that unblocks a required-resource dependency therefore
already reaches the reconciler; nothing new is needed to make a blocked resource
wake up. As a replacement for Managed Resource References lookups and resource
fetching are removed from the Provider Pod and into the Crossplane engine.

The target of an edge becomes a `oneof`:

```protobuf
message Dependency {
  // Name of the composed resource that has the dependency.
  // A key into State.resources.
  string resource = 1;

  // What `resource` depends on.
  oneof depends_on {
    // Name of another composed resource. A key into State.resources.
    string composed_resource = 2;

    // A resource the pipeline required, rather than composed.
    RequiredResourceDependency required_resource = 4;
  }

  // How this dependency constrains the order its endpoints are created and
  // deleted. Only DEPENDENCY_LIFECYCLE_UNSPECIFIED is valid when depends_on
  // is a required resource.
  DependencyLifecycle lifecycle = 3;
}

message RequiredResourceDependency {
  // The requirement name. A key into RunFunctionRequest.required_resources,
  // and into a RunFunctionResponse's requirements.resources.
  string requirement_name = 1;

  // Optional name of a single resource within the set the requirement
  // matched. If unset, every matched resource must be ready.
  optional string name = 2;

  // Namespace of name for a namespaced resource. Leave unset for a
  // cluster-scoped resource.
  optional string namespace = 3;
}
```

The same pair notation extends to required resources. An edge naming only the
requirement waits on every object that requirement matched; adding `name`
narrows it to a single one of them, and `namespace` qualifies that name when the
resource is namespaced:

```text
[ // (resource, depends_on)
  (subnet-a, required(shared-vpc)),
  (subnet-b, required(shared-vpc)),
  (route-table-a, required(shared-vpc, name=vpc-prod)),
  (database, required(platform-config, name=prod, namespace=platform))
]
```

Because the XR does not own the lifecycle of a Required resource, the effect a
dependency is different than within the composition:

* **It only supports Creation ordering** Any `lifecycle` other than
  `DEPENDENCY_LIFECYCLE_UNSPECIFIED` should be rejected by the core engine.
* **Missing/Multiple requirements affect the graph** Crossplane returns an empty
  `Resources` message when a requirement matched no objects, which will block
  the creation of Composed resources. When a requirement matches several objects
  and `name` is unset, all of them must be ready to unblock creation.
* **Readiness is read from the object.** Ready means a `Ready: True` status
  condition, or, for object kinds that have no `Ready` condition at all,
  existence. We may need to examine the real-world effects of this, as `Ready`
  state is often set at the end of the pipeline.

Validation gains one check: a `requirement_name` must correspond to a
requirement the pipeline actually declared. An edge naming a requirement no
function requested is the same class of error as an edge naming a composed
resource that exists in neither desired nor observed state. Required resources
are always leaves in the graph, so they cannot introduce cycles.

This facilitates ordering across Composite Resources. If the resource an XR
requires happens to be another XR, this expresses "don't create my resource
until that XR is ready" in a read-only mode. It is only enforced only on the
create side. Sequencing the deletion of resources across XR boundaries remains
out of scope, and remains what `Usage` is for.

### Dependencies accumulate like Desired Resources

Every function receives the full `dependencies` list accumulated so far and
returns the full list it wants going forward, the same way `desired` already
works. A function with no ordering opinion copies `request.dependencies` into
`response.dependencies` unchanged. SDKs should make this the default behavior of
their request and response builders, so unaware function code doesn't have to do
anything special to avoid dropping edges it doesn't understand.

### Backward Compatibility for Functions

Functions compiled against SDK versions that predate the implementation will
have no way to know they are expected to carry forward the `dependencies` field,
like they do with desired state. In the proposal a `Dependencies` message is
used to wrap a repeated field. By using this pattern (which is also used by the
`State` message) there is a clear difference between a function returning no
data and one returning an empty list of dependencies.

When building the request for pipeline step N+1, Crossplane will default
`dependencies` to what step N received if step N's response left the field
unset. "Unset" means "unchanged," never "empty." A function only needs to touch
`dependencies` when it actually has an opinion about ordering. In this way
functions can be upgraded over time without affecting the overall dependency
graph.

By convention many Function SDKs implement a `to()` function that sets the
response at the start of a pipeline. SDKs should be updated to include the
dependencies from previous functions in the pipeline as an additional
improvement.

### Validation in the Core Engine for Valid and Acyclic Graphs

After each function's response, before building the next request, Crossplane
checks:

1. Every `resource` and `depends_on` name is resolved against the union of
   `Desired.Resources` and `Observed.Resources`. A composed resource dropped
   from `Desired`, because a function decided it should be deleted, but still
   present in `Observed` is exactly the case where a dependency edge matters
   most: it is what tells the reconciler not to delete it yet.
2. An edge naming a composed resource in neither is pruned, not rejected. A
   function with a fixed rule set goes on declaring edges for resources it has
   finished deleting, and core cannot tell a typo from a completed deletion, so
   rejecting them would fail composition permanently.
3. An edge naming an undeclared *requirement* is an error - but "declared" means
   declared by any function in this run, not present in the request. A function
   returns `requirements.resources` and the edges over them together, and
   Crossplane fetches required resources only after seeing that response, so the
   first request carries none. Validating against the request alone would reject
   every required-resource edge on its first pass and never recover.
4. The accumulated edge set remains acyclic.

A violation is reported the way Crossplane reports other invalid function
output: the pipeline run fails, and Crossplane records a warning event and a
`Synced: False` condition on the XR, naming the function whose response
introduced the violation.  Doing both checks in Crossplane means every SDK
doesn't need to separately implement validation.

### Using the Graph to Order Creation and Deletion

The graph's primary consumer is Crossplane's own applier and garbage collector
that runs as part of the reconciler loop.

* **Create and update.** The applier issues a patch for a composed resource only
  once every resource it `depends_on` is ready.
* **Delete.** The garbage collector issues a delete only once every resource
  that `depends_on` it has left observed state.

One pass over the graph produces one decision per composed resource, and those
decisions define how Crossplane's applicator will behave.

```mermaid
flowchart TD
    A["Edges accumulated across the pipeline"]
    A --> B["Prune: drop edges naming resources that are<br/>neither desired nor observed"]
    B --> C{"Validate: endpoints known, one target per edge,<br/>requirement declared, acyclic"}
    C -- invalid --> X["Fail the reconcile:<br/>Synced=False, warning naming the function"]
    C -- valid --> D{"For each composed resource:<br/>is it desired? is it observed?"}

    D -- "desired, already exists" --> AP["Apply"]
    D -- "desired, not created yet" --> G1{"Is everything it depends on ready?"}
    D -- "not desired, still exists" --> G2{"Has everything that depends on it gone?"}
    D -- "neither" --> Z["Nothing to do"]

    G1 -- yes --> AP
    G1 -- "no, still coming up" --> W1["Blocked from creation"]
    G1 -- "no, dependency is being deleted" --> DL["Deadlocked"]

    G2 -- yes --> DE["Delete"]
    G2 -- "no, dependent still exists" --> W2["Blocked from deletion"]
    G2 -- "no, dependent is still desired" --> DL

    AP --> PATCH["Applier patches the resource"]
    DE --> GC["Garbage collector deletes it"]
    W2 --> HIDE["Hidden from the garbage collector"]

    AP --> REFS["spec.resourceRefs:<br/>what exists, plus what is about to be applied"]
    W2 --> REFS
    W1 -. "excluded on purpose:<br/>it was never created" .-> REFS

    W1 --> SYNC["XR reports Synced=False,<br/>naming the resource and the reason"]
    W2 --> SYNC
    DL --> SYNC
    DL --> WARN["Warning event, and readiness withheld so the<br/>XR cannot report Available while permanently stuck"]
```

Once a resource exists it keeps reconciling, so a dependency that goes un-ready
cannot freeze it.

A resource blocked this reconcile is picked up on a later pass, so no new
scheduling primitive is needed. With realtime compositions enabled the
reconciler does not requeue on a fixed interval. The later pass comes from the
watch Crossplane already establishes on each composed resource through its
dependency tracker: a dependency becoming ready is itself the event that
unblocks its dependents.

#### Determining Readiness

"Ready" here means the pipeline's own verdict — the value Crossplane already
derives from `Resource.ready` in the final desired state and uses to compute the
XR's `Ready` condition. That keeps a single definition of composed resource
readiness rather than introducing a second: a function that already influences
readiness, `function-auto-ready` being the common case, influences gating the
same way. A resource absent from desired state has no verdict, and so never
satisfies a dependency. Required resources are the documented exception: they
have no verdict either, so core reads their status directly — see "Add the
ability to Depend on a Required Resource in the gRPC Message."

#### Blocked resources must still be reported

`Compose` returns a `[]ComposedResource` that the reconciler uses to compute the
XR's `Synced` and `Ready` conditions, and today that slice is built from the
resources the applier actually patched. A resource held back by the graph has to
appear in it as not synced, with a reason. Otherwise the XR reports `Synced:
True` while composition is deliberately incomplete.

### Dealing With Contradictions in the Graph

The `Deadlocked` condition is for cases where the graph contradicts the desired
state. For example, if the pipeline drops a resource from desired state while
another resource that depends on it stays desired.

#### Resources waiting for Dependencies to be Created

Resources waiting on dependencies before being created are not populated in
`spec.resourceRefs`, as they have not been applied to the cluster.

#### Resources pending Deletion

Deferred deletes must stay in `spec.resourceRefs`. Crossplane rebuilds the XR's
`spec.resourceRefs` from desired state, and observes composed resources next
reconcile by reading it. A resource whose deletion the graph defers is by
definition not in desired state, so it would drop out of the references array
and become invisible on the next pass. Therefore, references must be the union
of desired state and the resources still present in observed state.

### Persisting Dependencies in `spec.resourceRefs`

Edge data can be persisted in `spec.resourceRefs` using two new fields
`resourceName` and `dependsOn`. These additional fields will require
Crossplane-runtime schemas to be updated:

```yaml
resourceRefs:
- apiVersion: ec2.aws.upbound.io/v1beta1
  kind: Subnet
  name: my-xr-subnet-a-p4m9x
  resourceName: subnet-a       # composition resource name
  dependsOn: [vpc]             # composition resource names
```

`resourceName` is what makes `dependsOn` resolvable: that name lives only in the
`crossplane.io/composition-resource-name` annotation today, so recording it here
also answers "which reference is which template" without reading every composed
resource.

On teardown core rebuilds the edges from the references, prunes those naming
resources that have left observed state, and asks the same `Decide` the live
path uses. No function runs.

The graph becomes readable by any tool with a `GET` on the XR, so `crossplane
resource trace` and dashboards can render it without replaying a pipeline. And
unlike references, `dependsOn` comes from function output, so an XR deleted
before the pipeline next succeeds tears down by the previous graph which
describes what was actually built.

End users can run commands to extract dependencies:

```shell
kubectl get xordering ordered -o jsonpath='{range .spec.crossplane.resourceRefs[*]}{.resourceName}{" <- "}{.dependsOn}{"\n"}{end}'
```

### Dependency Lifecycle: create-before-destroy

The default for every edge is symmetric: forward order for create, strict
reverse for delete. That covers the common cases, like a VPC before a subnet, or
a subnet before an instance. Some replacements need the opposite on the way out,
where the new resource should exist before the old one is torn down, as in a
Kubernetes node pool.

Rather than introduce a second edge type, each edge carries a `lifecycle` enum,
whose `DEPENDENCY_LIFECYCLE_CREATE_BEFORE_DESTROY` value mirrors Terraform's
`create_before_destroy`. When set, the reconciler may create `resource` without
waiting for `depends_on` to be deleted first, but must still wait for `resource`
to exist and be ready before deleting `depends_on`. This keeps the graph a
single, simple structure: the value changes only which side of a transition may
proceed early, not the topology.

Using an enum enables the API to include other lifecycle policies in the future,
like
Pulumi's [`deletedWith`](https://www.pulumi.com/docs/iac/concepts/resources/options/deletedwith/).

## Enabling Adoption with `function-ordering`

If we required every function author to adopt a new SDK version and emit
`dependencies`, the feature would arrive slowly and unevenly.

Thanks to the accumulation of state in Crossplane functions, we can place a
function at the end of the pipeline that defines dependency pairs.

[**`function-ordering`**](https://github.com/stevendborrelli/function-ordering) is a fork of **`function-sequencer`**, that keeps
its input schema and its user-facing behavior but emits `dependencies` edges.

### How it works

The fork reads the same `rules` block Composition authors already write, and
translates it into edges over composed resource names:

```yaml
  - step: order-resources
    functionRef:
      name: function-dependency-graph
    input:
      apiVersion: ordering.fn.crossplane.io/v1beta1
      kind: Input
      rules:
        - sequence: [vpc, subnet, security-group]
```

becomes the edge set `subnet -> vpc`, `security-group -> vpc`, `security-group
-> subnet`. Note that this is every predecessor pair, not just adjacent ones,
matching `function-sequencer`'s existing semantics: a resource at position *i*
waits on all of `sequence[:i]`, not only on `sequence[i-1]`.

Three properties make this the right shape for adoption:

* **No other function has to change.** The shim is a pipeline step operating on
  composed resource names in accumulated state. Whatever produced those
  resources — a templating function, a general-purpose function, a function
  compiled years before this proposal — is unaffected and unaware. A Composition
  adopts the graph by changing one `functionRef`.
* **Pattern matching stays out of the protocol.** `function-sequencer`'s regex
  rules expand against the names present in desired and observed state at
  translation time, so the wire format can stay exact-name-only while authors
  keep writing `first-subresource-.*`. Expansion is a user-experience concern;
  edges are the interchange format. This is a better split than teaching core to
  match patterns.
* **It degrades to today's behavior.** If Crossplane doesn't advertise the
  dependencies capability, the fork does exactly what `function-sequencer` does
  now — hold resources back by omission, compose `Usage` objects for teardown,
  and set `resetCompositeReadiness` if configured. One function works against
  old and new cores, and the same Composition keeps working through an upgrade.

In graph mode `resetCompositeReadiness` becomes unnecessary: core knows a
resource is blocked and reports it, which is the whole point of moving the
signal into the protocol.

Two of `function-sequencer`'s inputs have no equivalent in a symmetric edge. Its
per-rule `deleteOnly` and `createOnly` modifiers decouple the create and delete
directions of a sequence, and its per-rule CEL `condition` disables creation
sequencing while deliberately keeping teardown order. Rather than grow the
protocol a lifecycle knob per case, the fork keeps those rules on the legacy
path: a rule that sets any of the three is evaluated the way
`function-sequencer` evaluates it today, and only plain rules become edges. A
Composition can mix both in one step, and nothing an author already wrote stops
working. If demand for asymmetric edges turns out to be real, it is an additive
field later — but the evidence for it is one contrib function's options, not a
constraint the graph cannot otherwise meet.

## API Impact and Capabilities

This proposal changes a wire protocol that every installed Composition Function
and every running Crossplane core already depends on, so its compatibility
impact needs to stand on its own rather than be inferred from the rest of this
document.

**No new API version is required.** Adding a field with a previously-unused
number is wire-compatible by construction. `proto/fn/v1/run_function.proto` has
already grown this way several times without ever moving to a `v2` package:
`required_resources` shipped in Crossplane v1.15, `credentials` in v1.16,
`conditions` in v1.17, and `required_schemas` alongside the `capabilities`
mechanism itself in v2.2.

**Runtime behavior compatibility is handled by the existing capability
mechanism, extended by one value.** `RequestMeta.capabilities` already exists so
a function can tell "this core doesn't support X" apart from "this core predates
capability advertising entirely" — which is what the `CAPABILITY_CAPABILITIES`
value itself is for. Without a capability flag here, a function talking to an
older core would have its `dependencies` silently accepted on the wire, because
protobuf servers don't reject fields they don't recognize, but never enforced,
with nothing telling the function that happened. A new capability value closes
that gap the same way it is already closed for every other optional feature in
this protocol.

At a glance, the concrete additions to `proto/fn/v1/run_function.proto` and
where they land:

* **`Dependency`** and **`RequiredResourceDependency`** — new top-level
  messages. No existing message changes shape.
* **`RunFunctionRequest.dependencies`**, field `10`. Additive; unset on old
  clients, and carried forward by the runtime rather than by the function.
* **`RunFunctionResponse.dependencies`**, field `8`. Additive; unset means "no
  opinion," not "empty."
* **`CAPABILITY_DEPENDENCIES`**, `Capability` enum value `6`. Additive; lets a
  function detect an older core that will accept the field without enforcing it.

Functions should treat the capability's absence as "this core will not enforce
ordering," not as an error, the same graceful-degradation posture the protocol
already expects for every other optional capability.

## Alternatives Considered

* **Leaving ordering to `function-sequencer` as it exists today.** The
  status-quo already covers the common case: declarative rules, pattern
  matching, works with any upstream function, ships now, no core changes. This
  proposal aims to move ordering into Crossplane instead of using delayed
  rendering and Usages.
* **Composing `Usage` resources instead of declaring edges.** The mechanism
  `function-sequencer` and `function-deletion-protection` both use. Usage
  enforcement at the Kubernetes API level applies even when there is a deletion
  attempt on the XR itself. There are several problems with using Usages for
  ordering. Each Usage is an extra resource on the cluster and it is not
  symmetric with creation. It can be difficult to generate complex dependency
  graphs using Usages. With ordering moved to the core the role of Usage is
  clarified to protecting deletion of critical resources.
* **Leaving ordering to observed-state gating written by hand in each
  function.** Possible today, and not rejected — it remains the right mechanism
  for anything conditional on more than existence and readiness, such as waiting
  for a Job to report success. Rejected as the *only* mechanism because it
  requires every function to implement a per-resource state machine, which is
  why `function-sequencer` exists at all.
* **Expressing ordering entirely through patches.** Piping a value from one
  resource's status into another's spec forces an implicit order, and is how
  many Compositions sequence resources today. Rejected as the sole mechanism
  because it can only express an order that coincides with a data dependency.