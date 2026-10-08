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

Deleting an XR also doesn't wait for its composed resources. Crossplane v1
could: a claim's `compositeDeletePolicy: Foreground` made the claim delete its
XR with foreground propagation, so the claim and XR stayed until the composed
resources were gone. v2 removed claims, and with them this behavior for most
XRs. The [v2 proposal review][v2-foreground] suggested requiring
`--cascade=foreground` with a `ValidatingAdmissionPolicy` instead, but that can
only reject a plain `kubectl delete`, not fix it.

[gc]: https://github.com/crossplane/crossplane/blob/f108173392198c7aa79f3828e9b741ca3af86b9f/internal/controller/apiextensions/composite/composition_functions.go#L864
[finalizer]: https://github.com/crossplane/crossplane/blob/f108173392198c7aa79f3828e9b741ca3af86b9f/internal/controller/apiextensions/composite/reconciler.go#L607
[remove-finalizer]: https://github.com/crossplane/crossplane/blob/f108173392198c7aa79f3828e9b741ca3af86b9f/internal/controller/apiextensions/composite/reconciler.go#L588
[5092]: https://github.com/crossplane/crossplane/issues/5092
[v2-foreground]: https://github.com/crossplane/crossplane/pull/6255#discussion_r1938188224

## Goals

* Allow functions to control the order composed resources are deleted when an XR
  is deleted, using the same desired-state-gating pattern they use for creates.
* Allow functions to perform cleanup work during XR deletion. For example a
  function could add a backup Job to desired state, wait until it completes in
  observed state, and then remove the database from desired state.
* Support foreground deletion defined by the Composition, so that a plain
  `kubectl delete` keeps an XR until its composed resources are gone, as v1
  claims could.
* Remain fully backward compatible. Existing functions and Compositions require
  zero changes. XR deletion behavior is identical to today unless a Composition
  explicitly opts in.

## Proposal

Function-controlled deletion ships behind an alpha feature gate,
`--enable-function-controlled-deletion`. When it's disabled Crossplane behaves
exactly as it does today: it removes its finalizer as soon as an XR is deleted,
ignores a Composition's delete policy, and doesn't retain references to composed
resources it has garbage collected.

### Composition Delete Policies

A Composition declares what happens when an XR that uses it is deleted, with a
new `spec.deletePolicy` field. It supports multiple deletion modes:

| `deletePolicy` | When the XR is deleted |
| --- | --- |
| `Background` (default) | Crossplane removes its finalizer, and Kubernetes deletes the composed resources once the XR is gone. This is today's behavior. |
| `Foreground` | Crossplane deletes the composed resources, and keeps the XR until they're gone. The pipeline doesn't run. This is the foreground deletion v1 claims offered. Crossplane already deletes nested XRs this way, but v2 lacks it for an XR deleted directly. |
| `Pipeline` | Like `Foreground`, but Crossplane keeps running the function pipeline, which decides when each composed resource may be deleted. |

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: databases
spec:
  compositeTypeRef:
    apiVersion: platform.example.org/v1
    kind: Database
  deletePolicy: Pipeline
  pipeline:
  - step: render
    functionRef:
      name: function-kcl
```

`Foreground` is foreground deletion performed by Crossplane rather than the
Kubernetes garbage collector, so it works for a plain `kubectl delete`.

The Composition declares this, not the functions. Whether a pipeline handles
deletion is a property of the whole pipeline, and for templating functions of
their templates, so only the Composition author knows. Functions don't advertise
anything to Crossplane.

The schema doesn't default the field. Leaving it unset means `Background`, so
existing Compositions keep their hash and don't produce new revisions.

### Decision Logic on XR Deletion

When the XR has a `deletionTimestamp`, the reconciler reads the delete policy
the XR recorded while it was live (see [Recording the Delete
Policy](#recording-the-delete-policy)):

1. **`Background`.** Crossplane removes its finalizer and lets Kubernetes
   garbage collection cascade via owner references, as it does today.

2. **`Foreground`.** Crossplane garbage collects the XR's composed resources and
   requeues. When no composed resources remain, it removes the finalizer. The
   pipeline doesn't run.

3. **`Pipeline`.** Crossplane runs the pipeline instead of immediately removing
   the finalizer. It garbage collects composed resources not in desired state,
   server-side applies resources that are in desired state, and requeues. When
   no composed resources remain, it removes the finalizer.

With `Pipeline`, every function in the pipeline must handle deletion. A single
unaware function could hold resources in desired state indefinitely. In
practice, steps that only render or decorate resources, like
`function-auto-ready`, don't need to change, as long as a later deletion-aware
step removes resources from desired state. Such a step at the end of the
pipeline can also delete resources in order. For example a sequencing function
like [`function-sequencer`][function-sequencer] could remove resources from
desired state in the reverse of the order it creates them.

[function-sequencer]: https://github.com/crossplane-contrib/function-sequencer

If the pipeline fails while the XR is deleted, Crossplane holds the XR and
retries. It never falls back to `Foreground`, which would delete resources a
function wanted to do work on first.

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

### Recording the Delete Policy

By the time an XR is deleted, its Composition may have changed, or be gone. So
the XR records its Composition's delete policy in its status on every reconcile
while it's live:

```yaml
status:
  crossplane:
    deletePolicy: Pipeline
```

The XR records it as soon as it selects a CompositionRevision, before anything
that can fail, so that an XR whose pipeline is failing still runs it when it's
deleted. `Background` and `Foreground` don't need the Composition, so Crossplane
uses them as recorded. For `Pipeline` Crossplane must fetch the XR's revision to
run its pipeline. If the revision now has a weaker policy, Crossplane uses that
instead.

This tells a user how an XR will be deleted before they try to delete it.
Crossplane also reads it itself, to decide how to delete an XR that another XR
composes - see [Nested XRs](#nested-xrs).

### Walkthrough

A function creates a VPC and a Subnet. The Subnet should be deleted before the
VPC. The pipeline is `[function-templates, function-auto-ready]`, and the
Composition's `deletePolicy` is `Pipeline`.

**Normal operation:** `function-templates` returns VPC and Subnet in desired
state. `function-auto-ready` passes them through with readiness annotations. The
XR records the `Pipeline` delete policy.

**User deletes the XR.**

Reconcile 1:

* Crossplane sees `deletionTimestamp` and the recorded `Pipeline` policy, and runs
  the pipeline.
* `function-templates` sees the deletion timestamp on the observed XR. It omits
  Subnet from desired state, keeps VPC.
* `function-auto-ready` passes through desired state (VPC only).
* Crossplane garbage collects the Subnet. Server-side applies the VPC. Requeues.

Reconcile 2:

* The Subnet still exists, so Crossplane still references it.
  `function-templates` sees it in observed state with a `deletionTimestamp`, so
  it knows the Subnet is still being deleted. It keeps VPC in desired state and
  waits.

Reconcile 3:

* The Subnet is gone, so Crossplane no longer references it and it's absent from
  observed state. `function-templates` omits the VPC from desired state (returns
  empty desired).
* `function-auto-ready` passes through (nothing to pass).
* Crossplane garbage collects the VPC. Once it's gone too, no composed resources
  remain. Removes the finalizer. XR is deleted.

**If the Composition's `deletePolicy` is `Foreground` instead:**

Crossplane doesn't run the pipeline. It deletes the VPC and the Subnet, and
removes its finalizer once they're gone.

### Templating-Based Functions

This feature is most naturally used by functions written in general-purpose
languages that already support ordered creation by gating desired state on
observed state. Templating-based functions like `function-go-templating` and
`function-kcl` don't typically gate on observed state today. A Composition using
them can still use `Foreground`. With `Pipeline`, their templates can read the
XR's `deletionTimestamp` and observed state like anything else, and decide what
to keep in desired state.

If a templating-based function wanted to offer some level of deletion control,
it could implement sync waves: let users annotate desired resources with a
deletion wave number, and have the function's runtime (not the templates) handle
the gating logic. This is a choice for individual function authors, not
something this design prescribes.

### SDK Helpers

Functions don't advertise anything to Crossplane, so the SDKs only need a
convenience helper.

**Go SDK (`function-sdk-go`)**

```go
// IsDeleting returns true if the observed XR has a deletion timestamp.
func IsDeleting(req *v1.RunFunctionRequest) bool
```

**Python SDK (`function-sdk-python`)**

```python
def is_deleting(req: fnv1.RunFunctionRequest) -> bool:
    """Return True if the observed XR has a deletion timestamp."""
```

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
collection**, whatever the XR's delete policy. It emits a warning event saying
so. The two intents are contradictory -
foreground deletion says "delete the dependents before the owner", while
function-controlled deletion says "let me control the order" - and someone
passing `--cascade=foreground` is explicitly asking for the former.

This differs from the `Foreground` delete policy. With the policy, Crossplane
deletes the composed resources itself, so it can order them and doesn't race the
garbage collector.

### Nested XRs

That leaves a problem for XRs that compose other XRs, because nobody has to ask
for foreground deletion to get it. Crossplane's garbage collector [always
deletes composed resources with foreground propagation][6599], so an XR deleted
by its parent hits the conflict above on an ordinary `kubectl delete` of that
parent. Composed XRs are the most common place ordered teardown matters, so
leaving it here would limit the feature to XRs that compose only managed
resources.

Crossplane will instead choose a propagation policy per composed resource. When
it garbage collects a composed XR that recorded the `Foreground` or `Pipeline`
delete policy, it deletes that XR with background propagation, so the XR's own
delete policy controls its teardown. Everything else keeps the
foreground propagation [#6599][6599] introduced - including every managed
resource, where the two policies behave identically because managed resources
have no dependents of their own.

The guarantee [#6599][6599] wanted - a composed XR's own resources deleted
before the XR itself - still holds, and more strictly: Crossplane doesn't
remove the composed XR's finalizer until its composed resources are gone.

[6599]: https://github.com/crossplane/crossplane/pull/6599

### Edge Cases

**Stuck-in-deleting XRs.** If a Composition declares the `Pipeline` policy but
a function is buggy and never empties desired state, or a composed resource
can't be deleted, the XR hangs with a stuck finalizer. This is standard Kubernetes behavior. The same thing happens with
any stuck finalizer. Users can manually remove it. Crossplane should emit
warning events if the XR has been in `Deleting` for an extended period with
composed resources remaining.

Crossplane won't offer a way to opt a stuck XR out of function-controlled
deletion - an annotation, say. Setting one would be the same amount of work as
removing the finalizer, and Crossplane shouldn't give up on ordered deletion on
a timer either: deleting a database because a backup function was unreachable
for a few minutes is worse than waiting for a human.

**The XR's Composition is gone.** Nothing stops someone deleting a Composition,
or the Configuration that brought it, while XRs that use it still exist. XRs
with the `Background` or `Foreground` policy don't need their Composition to be
deleted, because they recorded their policy. An XR with the `Pipeline` policy
can't run its pipeline at all, so Crossplane holds it rather than skip work its
functions wanted to do first. Users who want to delete it anyway can remove its
finalizer, as for any stuck XR.

**Pipeline failure during deletion.** If the pipeline fails during deletion (e.g.
function pod unavailable), the reconciler returns an error and requeues, same as
normal reconciliation failures. The XR stays in `Deleting` until the pipeline
succeeds. This is the safe default. It avoids unordered deletion when the
function intended to control the order.

**`crank render` support.** The `crank render` CLI also runs pipelines. Function
authors can test their deletion logic locally by setting `deletionTimestamp` on
the XR they feed into `crank render`. For this to work `render` needs to run the
pipeline for a deleted XR when the Composition's delete policy is `Pipeline`.

**Composed resources with deletion timestamps.** A composed resource Crossplane
has garbage collected stays in observed state, carrying its own
`deletionTimestamp`, until it's really gone. Functions should treat a composed
resource with a deletion timestamp as still being deleted, and wait for it to
disappear from observed state before they stop desiring the next one.

## Future Work

**Declared ordering.** The [composed resource ordering proposal][7841] would let
functions declare dependencies between composed resources. If it's accepted,
Crossplane could use them to order the deletes it makes for the `Foreground`
policy, and to order the deletes a `Pipeline` asks for.

[7841]: https://github.com/crossplane/crossplane/pull/7841

## Alternatives Considered

### Function-advertised capabilities

An earlier version of this design had functions advertise a deletion capability
in their responses, and ran the pipeline during deletion only if every function
did. Any function that didn't advertise it, including ones that only pass
desired state through like `function-auto-ready`, turned the feature off for the
whole pipeline. Templating functions couldn't advertise it honestly, because
whether deletion is handled depends on the template. It also needed a second
capability mechanism, from functions to Crossplane.

A variant had only the last function advertise it, taking responsibility for
everything before it. The Composition author has to arrange the pipeline
correctly either way, so it's simpler for them to declare the delete policy
directly.

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

Instead of a pipeline-level delete policy, the Composition's pipeline steps
could have an `onDelete: Run | Skip` field. Steps marked `Run` would be called
during deletion; steps marked `Skip` (the default) would not.

Whether an XR finishes deleting depends on the pipeline's final desired state,
which every step contributes to, so it's a property of the whole pipeline. A
step that composes resources but skips deletion would keep desiring them
forever.

### Per-response deletion status field

Instead of a capability, each response could include a `DeletionHandling` field
with values like `UNSPECIFIED`, `IN_PROGRESS`, and `COMPLETE`. Crossplane would
check these per-invocation to decide whether to trust the pipeline.

This conflates a static property ("this pipeline handles deletion") with
per-invocation status. The static property belongs in the Composition. The
completion signal is implicit in the desired state: when it's empty, the
function is done.

### Usages

The existing Usages API provides deletion ordering by creating `Usage` resources
that block deletion of dependencies. Functions could compose `Usage` resources
alongside other resources.

Usages are heavyweight (separate custom resources) and have surprising edge
cases. For example, a `Usage` can't prevent a Namespace from transitioning to
`Terminating` and blocking other operations. Function-controlled deletion via
desired state is a more natural fit for the composition model.
