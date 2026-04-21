# Function-Controlled Deletion of Composed Resources

* Owner: Nic Cope (@negz)
* Reviewers: Adam Wolfe Gordon (@adamwg)
* Status: Draft

## Background

Composition functions already give authors full control over the order resources
are created and updated. A function can gate desired state on observed state:
don't add a Subnet to desired state until you see the VPC is ready in observed
state. Crossplane never creates the Subnet until the function says so.

The same pattern works for deletion while the XR is alive. If a function stops
returning a composed resource in its desired state, Crossplane garbage collects
it. A function can use this to orchestrate ordered teardown: omit the Subnet
from desired, wait until it's gone from observed, then omit the VPC. Each
reconcile loop deletes the next batch. The existing
[`GarbageCollectComposedResources`][gc] logic handles the actual deletion using
foreground propagation.

The pattern breaks down when the XR itself is deleted. Today, the XR reconciler
[adds a finalizer][finalizer] to the XR, but on deletion it [immediately removes
the finalizer][remove-finalizer] without running the composition function
pipeline. Kubernetes garbage collection cascades the delete to all composed
resources via controller owner references, in no particular order.

This means functions have no opportunity to control composed resource deletion
order when the XR is deleted. This is the most common deletion scenario, and has been
a [long-standing request][5092].

The challenge is backward compatibility. Existing functions have no concept of
checking the XR's `deletionTimestamp` and reacting accordingly. If we started
running the pipeline on deletion, every existing function would keep returning
its full set of desired resources, and the XR would hang in `Deleting` forever.

[gc]: https://github.com/crossplane/crossplane/blob/f108173392198c7aa79f3828e9b741ca3af86b9f/internal/controller/apiextensions/composite/composition_functions.go#L864
[finalizer]: https://github.com/crossplane/crossplane/blob/f108173392198c7aa79f3828e9b741ca3af86b9f/internal/controller/apiextensions/composite/reconciler.go#L607
[remove-finalizer]: https://github.com/crossplane/crossplane/blob/f108173392198c7aa79f3828e9b741ca3af86b9f/internal/controller/apiextensions/composite/reconciler.go#L588
[5092]: https://github.com/crossplane/crossplane/issues/5092

## Goals

* Allow functions to control the order composed resources are deleted when an XR
  is deleted, using the same desired-state-gating pattern they use for creates.
* Allow functions to perform cleanup work during XR deletion. For example a
  function could add a backup Job to desired state, wait until it completes in
  observed state, and then remove the database from desired state.
* Remain fully backward compatible. Existing functions and Compositions require
  zero changes. XR deletion behavior is identical to today unless every function
  in the pipeline explicitly opts in.

## Proposal

### Bidirectional Capability Advertisement

Crossplane already advertises its capabilities to functions via
[`RequestMeta.capabilities`][req-caps]. I propose making capability
advertisement bidirectional. Functions would advertise their capabilities back
to Crossplane via `ResponseMeta`.

The existing `Capability` enum would be renamed to `CrossplaneCapability` to
clarify that these are capabilities of Crossplane. A new `FunctionCapability`
enum would be added for capabilities of functions.

```protobuf
message RequestMeta {
  string tag = 1;
  repeated CrossplaneCapability capabilities = 2;
}

message ResponseMeta {
  string tag = 1;
  optional google.protobuf.Duration ttl = 2;
  repeated FunctionCapability capabilities = 3;
}
```

Renaming `Capability` to `CrossplaneCapability` is a source-breaking change to
generated code in the function SDKs, but both SDKs are pre-1.0 and the wire
format is unchanged. Only the numeric values matter for protobuf serialization.
Already-compiled functions communicating via gRPC are unaffected.

[req-caps]: https://github.com/crossplane/crossplane/blob/f108173392198c7aa79f3828e9b741ca3af86b9f/proto/fn/v1/run_function.proto#L170

### New Capabilities

```protobuf
enum CrossplaneCapability {
  CROSSPLANE_CAPABILITY_UNSPECIFIED = 0;
  CROSSPLANE_CAPABILITY_CAPABILITIES = 1;
  CROSSPLANE_CAPABILITY_REQUIRED_RESOURCES = 2;
  CROSSPLANE_CAPABILITY_CREDENTIALS = 3;
  CROSSPLANE_CAPABILITY_CONDITIONS = 4;
  CROSSPLANE_CAPABILITY_REQUIRED_SCHEMAS = 5;

  // Crossplane runs the function pipeline when an XR is being deleted,
  // and will honor desired state returned by functions during deletion.
  CROSSPLANE_CAPABILITY_DELETION = 6;
}

enum FunctionCapability {
  FUNCTION_CAPABILITY_UNSPECIFIED = 0;

  // This function advertises capabilities. If this is present, the
  // absence of another capability means the function genuinely does
  // not support it, not that the function predates capability
  // advertisement.
  FUNCTION_CAPABILITY_CAPABILITIES = 1;

  // This function handles XR deletion. When the XR is being deleted,
  // it will check for the deletion timestamp and manage composed
  // resource lifecycle via desired state accordingly.
  FUNCTION_CAPABILITY_DELETION = 2;
}
```

`FUNCTION_CAPABILITY_CAPABILITIES` mirrors the existing Crossplane-side
sentinel. It lets Crossplane distinguish "this function doesn't support
deletion" from "this function predates capability advertisement."

### Decision Logic on XR Deletion

When the XR has a `deletionTimestamp`, the reconciler runs the pipeline instead
of immediately removing the finalizer. It then inspects the capabilities
returned by each function:

1. **Every function returned `FUNCTION_CAPABILITY_DELETION`.** Crossplane trusts
   the pipeline. It garbage collects composed resources not in desired state,
   server-side applies resources that are in desired state, and requeues. When no
   composed resources remain, it removes the finalizer.

2. **Any function did not return the capability.** At least one function doesn't
   handle deletion. Crossplane falls back to current behavior: remove the
   finalizer and let Kubernetes garbage collection cascade via owner references.

This is an all-or-nothing check. Crossplane can only rely on function-controlled
deletion if every function in the pipeline understands it. A single unaware
function could hold resources in desired state indefinitely.

In practice, this isn't as restrictive as it sounds. Most utility functions
(like `function-auto-ready`) are trivial to update. They just need to return the
capability and continue passing through desired state as they already do. They
don't need complex deletion logic.

### Walkthrough

A function creates a VPC and a Subnet. The Subnet should be deleted before the
VPC. The pipeline is `[function-templates, function-auto-ready]`.

**Normal operation:** Both functions return `FUNCTION_CAPABILITY_DELETION`
(among other capabilities). `function-templates` returns VPC and Subnet in
desired state. `function-auto-ready` passes them through with readiness
annotations.

**User deletes the XR.**

Reconcile 1:

* Crossplane sees `deletionTimestamp`, runs the pipeline.
* `function-templates` sees the deletion timestamp on the observed XR. It omits
  Subnet from desired state, keeps VPC. Returns `FUNCTION_CAPABILITY_DELETION`.
* `function-auto-ready` passes through desired state (VPC only). Returns
  `FUNCTION_CAPABILITY_DELETION`.
* All functions returned the capability. Active deletion path.
* Crossplane garbage collects the Subnet. Server-side applies the VPC. Requeues.

Reconcile 2:

* `function-templates` sees Subnet is gone from observed state. Omits VPC from
  desired state (returns empty desired). Returns `FUNCTION_CAPABILITY_DELETION`.
* `function-auto-ready` passes through (nothing to pass). Returns
  `FUNCTION_CAPABILITY_DELETION`.
* Crossplane garbage collects the VPC. No composed resources remain. Removes the
  finalizer. XR is deleted.

**If `function-auto-ready` hasn't been updated yet:**

Reconcile 1:

* `function-templates` returns `FUNCTION_CAPABILITY_DELETION`.
* `function-auto-ready` doesn't return the capability.
* Not all functions support deletion. Fall back. Remove finalizer, Kubernetes
  garbage collection cascades. Identical to today.

### Templating-Based Functions

This feature is most naturally used by functions written in general-purpose
languages that already support ordered creation by gating desired state on
observed state. Templating-based functions like `function-go-templating` and
`function-kcl` don't typically gate on observed state today. These functions
would simply not return `FUNCTION_CAPABILITY_DELETION`, and the pipeline would
fall back to current behavior.

If a templating-based function wanted to offer some level of deletion control,
it could implement sync waves: let users annotate desired resources with a
deletion wave number, and have the function's runtime (not the templates) handle
the gating logic. This is a choice for individual function authors, not
something this design prescribes.

### SDK Helpers

Both function SDKs should provide helpers so that function authors don't need to
work with proto fields directly.

**Go SDK (`function-sdk-go`)**

The `request` package currently has helpers like `GetObservedCompositeResource`.
New helpers:

```go
// HasCapability returns true if Crossplane advertised the given capability.
func HasCapability(req *v1.RunFunctionRequest, c v1.CrossplaneCapability) bool

// IsDeleting returns true if the observed XR has a deletion timestamp.
func IsDeleting(req *v1.RunFunctionRequest) bool
```

The `response` package's `To()` function bootstraps a response from the request.
It would also set the function's capabilities:

```go
// To creates a new response to the supplied request, advertising the
// supplied function capabilities. It copies desired state and context
// through, and sets the response tag and TTL.
func To(req *v1.RunFunctionRequest, ttl time.Duration, caps ...v1.FunctionCapability) *v1.RunFunctionResponse
```

A `DefaultCapabilities()` helper would return the capabilities that most
functions should advertise. This includes `FUNCTION_CAPABILITY_CAPABILITIES` but
not `FUNCTION_CAPABILITY_DELETION`. Deletion support must be an explicit opt-in.
Otherwise a function that bumps its SDK dependency would start advertising
deletion support without implementing any deletion logic.

**Python SDK (`function-sdk-python`)**

The `request` module already has `advertises_capabilities` and `has_capability`.
New helpers:

```python
def is_deleting(req: fnv1.RunFunctionRequest) -> bool:
    """Return True if the observed XR has a deletion timestamp."""

def has_capability(req: fnv1.RunFunctionRequest, c: fnv1.CrossplaneCapability) -> bool:
    """Return True if Crossplane advertised the given capability."""
```

The `response` module's `to()` would gain a `capabilities` parameter:

```python
def to(
    req: fnv1.RunFunctionRequest,
    ttl: datetime.timedelta = DEFAULT_TTL,
    capabilities: Sequence[fnv1.FunctionCapability] = DEFAULT_CAPABILITIES,
) -> fnv1.RunFunctionResponse:
```

Where `DEFAULT_CAPABILITIES` includes `FUNCTION_CAPABILITY_CAPABILITIES` but not
`FUNCTION_CAPABILITY_DELETION`. Functions must explicitly opt in to deletion
support.

### Edge Cases

**Stuck-in-deleting XRs.** If all functions claim `FUNCTION_CAPABILITY_DELETION`
but one is buggy and never empties desired state, the XR hangs with a stuck
finalizer. This is standard Kubernetes behavior. The same thing happens with
any stuck finalizer. Users can manually remove it. Crossplane should emit
warning events if the XR has been in `Deleting` for an extended period with
composed resources remaining.

**Pipeline failure during deletion.** If the pipeline fails during deletion (e.g.
function pod unavailable), the reconciler returns an error and requeues, same as
normal reconciliation failures. The XR stays in `Deleting` until the pipeline
succeeds. This is the safe default. It avoids unordered deletion when the
function intended to control the order.

**`crank render` support.** The `crank render` CLI also runs pipelines. Function
authors can test their deletion logic locally by setting `deletionTimestamp` on
the XR they feed into `crank render`.

**Foreground deletion bypasses function-controlled ordering.** Composed
resources have an [owner reference][owner-ref] to the XR with
`blockOwnerDeletion: true`. It might seem like Kubernetes GC would immediately
start deleting composed resources when the XR gets a `deletionTimestamp`, racing
with the function pipeline. Whether this happens depends on the deletion
propagation mode.

With background deletion (the default for `kubectl delete`), the Kubernetes GC
only deletes dependents after their owner is actually removed from the API
server, not merely when it gets a `deletionTimestamp`. The XR's finalizer
prevents it from being removed while the pipeline runs. The GC sees the owner
still exists, leaves composed resources alone, and there is no race. By the time
the finalizer is removed and the XR disappears, the pipeline has already cleaned
up all composed resources and there is nothing left for the GC to cascade to.

With explicit foreground deletion (`kubectl delete --cascade=foreground`), the
GC behaves differently. It sees the owner has a `deletionTimestamp` and
preemptively starts deleting dependents that have `blockOwnerDeletion: true`,
regardless of whether the owner has been removed. This does race with the
function pipeline and ordering guarantees are lost. These two intents are
fundamentally contradictory: foreground deletion says "cascade delete dependents
before the owner," while function-controlled deletion says "let me control the
order." The result is that foreground deletion degrades to unordered deletion,
which is the same behavior as today without this feature.

[owner-ref]:
https://github.com/crossplane/crossplane-runtime/blob/8fa945bd32c8fba420b88255eb6d0159371d11ee/pkg/meta/meta.go#L111

**Composed resources with deletion timestamps.** When a composed resource is
garbage collected using foreground propagation, it may linger in observed state
with its own `deletionTimestamp` while its dependents are being deleted (e.g. a
nested XR). Functions should treat a composed resource with a `deletionTimestamp`
as "deletion in progress" and wait for it to disappear from observed state
before proceeding to delete the next resource.

## Alternatives Considered

### Per-step `onDelete` configuration in the Composition

Instead of function-advertised capabilities, the Composition's pipeline steps
could have an `onDelete: Run | Skip` field. Steps marked `Run` would be called
during deletion; steps marked `Skip` (the default) would not.

This puts the knowledge in the wrong place. The Composition author has to know
whether a function handles deletion correctly. If they get it wrong and mark a
function as `onDelete: Run` when it doesn't check `deletionTimestamp`, the XR
hangs forever. The function knows whether it handles deletion; it should be the
one to say so.

### Per-response deletion status field

Instead of a capability, each response could include a `DeletionHandling` field
with values like `UNSPECIFIED`, `IN_PROGRESS`, and `COMPLETE`. Crossplane would
check these per-invocation to decide whether to trust the pipeline.

This conflates a static property of the function ("I understand deletion") with
per-invocation status. Capabilities are the right abstraction. They describe
what the function supports, not what it's doing right now. The completion signal
is implicit in the desired state: when it's empty, the function is done.

### Usages

The existing Usages API provides deletion ordering by creating `Usage` resources
that block deletion of dependencies. Functions could compose `Usage` resources
alongside other resources.

Usages are heavyweight (separate custom resources) and have surprising edge
cases. For example, a `Usage` can't prevent a Namespace from transitioning to
`Terminating` and blocking other operations. Function-controlled deletion via
desired state is a more natural fit for the composition model.
