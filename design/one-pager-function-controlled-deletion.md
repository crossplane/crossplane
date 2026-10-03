# Function-Controlled Deletion of Composed Resources

* Owners: Nic Cope (@negz), Lovro Sviben (@lsviben)
* Reviewers: Adam Wolfe Gordon (@adamwg)
* Status: Draft

## Background

Composition functions already give authors full control over the order resources
are created and updated. A function can gate desired state on observed state:
don't add a Subnet to desired state until you see the VPC is ready in observed
state. Crossplane never creates the Subnet until the function says so.

The same pattern nearly works for deletion while the XR is alive. If a function
stops returning a composed resource in its desired state, Crossplane garbage
collects it. A function can use this to orchestrate ordered teardown: omit the
Subnet from desired, wait until it's gone from observed, then omit the VPC. Each
reconcile loop deletes the next batch. The existing
[`GarbageCollectComposedResources`][gc] logic handles the actual deletion.

Nearly, because Crossplane forgets a composed resource as soon as it garbage
collects it. Observed state is built from the XR's `spec.resourceRefs`, and
Crossplane replaces those references with the pipeline's desired state on every
reconcile. A resource the function stops desiring is deleted and dereferenced in
the same pass, so it's gone from observed state on the next reconcile whether or
not it still exists. A function waiting for it to disappear proceeds
immediately, while its provider may still be deleting the external resource.
See [Referencing Deleted Resources](#referencing-deleted-resources).

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

Function-controlled deletion ships behind an alpha feature gate,
`--enable-function-controlled-deletion`. When it's disabled Crossplane behaves
exactly as it does today: it removes its finalizer as soon as an XR is deleted,
doesn't advertise `CROSSPLANE_CAPABILITY_DELETION` to functions, and doesn't
retain references to composed resources it has garbage collected.

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
of immediately removing the finalizer. Before it garbage collects or applies any
of the desired state the pipeline returned, it inspects the capabilities
returned by each function:

1. **Every function returned `FUNCTION_CAPABILITY_DELETION`.** Crossplane trusts
   the pipeline. It garbage collects composed resources not in desired state,
   server-side applies resources that are in desired state, and requeues. When no
   composed resources remain, it removes the finalizer.

2. **Any function did not return the capability.** At least one function doesn't
   handle deletion. Crossplane discards the desired state the pipeline returned
   and falls back to current behavior: remove the finalizer and let Kubernetes
   garbage collection cascade via owner references.

This is an all-or-nothing check. Crossplane can only rely on function-controlled
deletion if every function in the pipeline understands it. A single unaware
function could hold resources in desired state indefinitely.

In practice, this isn't as restrictive as it sounds. Most utility functions
(like `function-auto-ready`) are trivial to update. They just need to return the
capability and continue passing through desired state as they already do. They
don't need complex deletion logic.

### Referencing Deleted Resources

Crossplane records the composed resources it creates in the XR's
`spec.resourceRefs`, and builds observed state by reading them back. It replaces
those references with the pipeline's desired state on every reconcile, so it
stops referencing a composed resource the moment it asks for it to be deleted.

Crossplane will instead keep referencing a composed resource it has garbage
collected until that resource is actually gone from the API server. This is what
makes ordered teardown implementable: a resource a function stopped desiring
stays in observed state, carrying its own `deletionTimestamp`, until it really
disappears. It also stops Crossplane forgetting resources whose deletion never
completes. Crossplane watches the resources it references, so it reconciles the
XR when one it's waiting on is finally deleted.

This costs one extra reconcile whenever a pipeline stops desiring a resource,
including while the XR is alive: the reference is dropped the reconcile after
the resource is gone, rather than the reconcile that deleted it.

Crossplane only retains a reference to a resource it would actually delete. It
never garbage collects a composed resource that has no controller reference,
because it can't prove it composed it, so waiting for one to disappear would
mean waiting forever. Crossplane stops referencing those as it does today, and
doesn't count them when deciding whether an XR's composed resources are all
gone.

### Advertising Deletion Support

Crossplane knows whether an XR's pipeline handles deletion long before the XR is
deleted, because it runs the pipeline on every reconcile. It will record this in
a `DeletionOrdered` status condition on the XR: true when every function in the
pipeline advertised `FUNCTION_CAPABILITY_DELETION`, false otherwise, naming the
steps that didn't.

This tells a user whether an XR will be deleted in order before they try to
delete it. Crossplane also reads it itself, to decide how to delete an XR that
another XR composes - see [Nested XRs](#nested-xrs).

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

* The Subnet still exists, so Crossplane still references it.
  `function-templates` sees it in observed state with a `deletionTimestamp`, so
  it knows the Subnet is still being deleted. It keeps VPC in desired state and
  waits.

Reconcile 3:

* The Subnet is gone, so Crossplane no longer references it and it's absent from
  observed state. `function-templates` omits the VPC from desired state (returns
  empty desired). Returns `FUNCTION_CAPABILITY_DELETION`.
* `function-auto-ready` passes through (nothing to pass). Returns
  `FUNCTION_CAPABILITY_DELETION`.
* Crossplane garbage collects the VPC. Once it's gone too, no composed resources
  remain. Removes the finalizer. XR is deleted.

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

### Foreground Deletion

Composed resources have an owner reference to the XR. Whether Kubernetes
garbage collection races the function pipeline depends on how the XR is
deleted.

With background deletion (the default for `kubectl delete`), the GC only deletes
dependents once their owner is actually removed from the API server, not merely
when it gets a `deletionTimestamp`. The XR's finalizer keeps it around while the
pipeline runs, so the GC leaves composed resources alone. By the time the
finalizer is removed, the pipeline has already deleted them and there's nothing
to cascade to.

With foreground deletion (`kubectl delete --cascade=foreground`), the API server
adds a `foregroundDeletion` finalizer to the XR and the GC deletes the XR's
dependents straight away - all of them, whether or not their owner reference
sets `blockOwnerDeletion`; that field only decides whether the owner's own
removal waits for them.

Crossplane and the GC then fight. The GC deletes a composed resource the
pipeline still desires, so Crossplane recreates it, so the GC deletes it again.
Neither side converges, and any work the pipeline composed to complete first -
the backup Job in the example above - is destroyed and recreated before it can
finish, so the XR never finishes deleting at all.

So Crossplane must not attempt function-controlled deletion against a foreground
deletion of the same resources. **When a deleted XR has the `foregroundDeletion`
finalizer, Crossplane removes its own finalizer and defers to garbage
collection**, exactly as it would for a pipeline that doesn't handle deletion.
It emits a warning event saying so. The two intents are contradictory -
foreground deletion says "delete the dependents before the owner", while
function-controlled deletion says "let me control the order" - and someone
passing `--cascade=foreground` is explicitly asking for the former.

### Nested XRs

That leaves a problem for XRs that compose other XRs, because nobody has to ask
for foreground deletion to get it. Crossplane's garbage collector [always
deletes composed resources with foreground propagation][6599], so an XR deleted
by its parent hits the conflict above on an ordinary `kubectl delete` of that
parent. Composed XRs are the most common place ordered teardown matters, so
leaving it here would limit the feature to XRs that compose only managed
resources.

Crossplane will instead choose a propagation policy per composed resource. When
it garbage collects a composed resource whose `DeletionOrdered` condition is
true, it deletes that resource with background propagation, so the resource's
own finalizer and pipeline control its teardown. Everything else keeps the
foreground propagation [#6599][6599] introduced - including every managed
resource, where the two policies behave identically because managed resources
have no dependents of their own.

The guarantee [#6599][6599] wanted - a composed XR's own resources deleted
before the XR itself - still holds, and more strictly: Crossplane doesn't
remove the composed XR's finalizer until its pipeline reports every resource
gone.

[6599]: https://github.com/crossplane/crossplane/pull/6599

### Edge Cases

**Stuck-in-deleting XRs.** If all functions claim `FUNCTION_CAPABILITY_DELETION`
but one is buggy and never empties desired state, the XR hangs with a stuck
finalizer. This is standard Kubernetes behavior. The same thing happens with
any stuck finalizer. Users can manually remove it. Crossplane should emit
warning events if the XR has been in `Deleting` for an extended period with
composed resources remaining.

Crossplane won't offer a way to opt a stuck XR out of function-controlled
deletion - an annotation, say. Setting one would be the same amount of work as
removing the finalizer, and Crossplane shouldn't give up on ordered deletion on
a timer either: deleting a database because a backup function was unreachable
for a few minutes is worse than waiting for a human.

**The XR's Composition is gone.** Nothing stops someone deleting a Composition,
or the Configuration that brought it, while XRs that use it still exist. A
deleted XR then can't run its pipeline at all, and would hang forever - which
would also stop an XRD finishing its own deletion, wedging a package uninstall.

Crossplane falls back to unordered deletion in this case. It can't do better:
by the time it needs to decide, the Composition it would need to tell whether
this XR wanted ordered deletion is already gone. Blocking the Composition's
deletion while XRs reference it would avoid the situation, but that affects
every XR and Composition, not just those using function-controlled deletion, so
it isn't worth the complexity for a first iteration.

**Pipeline failure during deletion.** If the pipeline fails during deletion (e.g.
function pod unavailable), the reconciler returns an error and requeues, same as
normal reconciliation failures. The XR stays in `Deleting` until the pipeline
succeeds. This is the safe default. It avoids unordered deletion when the
function intended to control the order.

**`crank render` support.** The `crank render` CLI also runs pipelines. Function
authors can test their deletion logic locally by setting `deletionTimestamp` on
the XR they feed into `crank render`. `render` needs to advertise
`CROSSPLANE_CAPABILITY_DELETION` for this to work, and its embedded `contextfn`
function - which seeds and retrieves pipeline context - needs to advertise
`FUNCTION_CAPABILITY_DELETION`, or every rendered pipeline will look like one
that doesn't handle deletion.

**Composed resources with deletion timestamps.** A composed resource Crossplane
has garbage collected stays in observed state, carrying its own
`deletionTimestamp`, until it's really gone. Functions should treat a composed
resource with a deletion timestamp as still being deleted, and wait for it to
disappear from observed state before they stop desiring the next one.

## Alternatives Considered

### Package-level capability advertisement

Functions already advertise capabilities in their package metadata - that's how
Crossplane knows whether a function supports compositions, operations, or both.
Deletion could be advertised the same way, which would let Crossplane decide
whether a Composition supports ordered deletion before any XR exists, and
without calling a function.

It can't express the cases that matter most, though. For a function like
`function-go-templating` or `function-kcl`, whether deletion is handled is a
property of the templates in a particular Composition, not of the function. A
package-level capability can only claim it for every use of the function or
none: claim it, and every XR composed by a template that ignores
`deletionTimestamp` hangs; don't, and nobody using those functions can use this
feature at all.

The existing package-level mechanism stays as it is. `composition` and
`operation` are properties of the function binary, so a CompositionRevision can
be validated against them before an XR exists. Deletion can't be known that way.

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
