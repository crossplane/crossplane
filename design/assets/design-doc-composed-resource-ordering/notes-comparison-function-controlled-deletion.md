# A declared graph, or function-controlled deletion?

Two proposals address ordered teardown, and they are being discussed as
alternatives:

* This one: functions declare ordering constraints, core sequences creation and
  deletion from them.
* [#7242][7242], "Function-Controlled Deletion of Composed Resources": core
  runs the pipeline when an XR is deleted, and functions control teardown by
  shrinking desired state, the same way they already control creation by
  gating desired state on observed state.

They overlap in exactly one place - ordered teardown - and differ everywhere
else. This note argues they are complementary, that neither depends on the
other, and that if only one ships, it should be the graph.

[7242]: https://github.com/crossplane/crossplane/pull/7242

## The difference that decides it

Ordered teardown here needs no pipeline run. The graph is persisted on the XR
in `spec.crossplane.resourceRefs[].dependsOn`, so the reconciler rebuilds it
from the XR's own references, observes what is left, and deletes a wave at a
time (`internal/controller/apiextensions/composite/reconciler.go`, the delete
path). Functions are not called. `TeardownSurvivesRestart` passes for this
reason: Crossplane is killed mid-teardown and the replacement process finishes
in order, having never run the pipeline for that XR.

Under #7242, teardown ordering is a property of the pipeline running. That
gives three differences that matter operationally:

**An unavailable pipeline.** Deleting an XR is often part of decommissioning
the configuration that defined it. The Function package may be uninstalled, its
image unpullable, its endpoint down. lsviben's POC raises exactly this case: a
Composition deleted before its XRs, which is easy to do by deleting a
Configuration, and which under #7242 leaves stranded XRs. The discussion
settles on falling back to unordered deletion. A persisted graph keeps ordering
through all of it, because core needs nothing but the XR.

**All-or-nothing negotiation.** #7242 orders a teardown only when *every*
function in the pipeline advertises `FUNCTION_CAPABILITY_DELETION`. A pipeline
of `[function-templates, function-auto-ready]` gets nothing until both are
updated, and the failure is silent: you get today's unordered cascade. A
declared graph asks nothing of the other functions in the pipeline.

**Templating functions.** #7242 expects `function-go-templating` and
`function-kcl` to decline the capability, because gating desired state on
observed state is beyond what they express, and suggests sync waves as a
per-function workaround. Edges are data rather than control flow, so a
templating function can emit them directly. That difference covers a large part
of the ecosystem.

## What #7242 does that a graph cannot

Function *participation* in teardown. Running a backup Job and waiting for it,
draining a queue, calling an external API, deciding what to delete from live
state. An edge expresses sequence, not completion, and no amount of graph makes
a function run during deletion.

If ordered teardown were the only goal, #7242 would be a heavy way to reach it.
It isn't the only goal, which is why both should exist.

## Evidence from converting a platform

Modelplane's compositions were moved onto declared ordering over three days,
which is the closest thing available to a controlled comparison: they had
already hand-written the desired-state-gating pattern #7242 generalizes, but
for creation.

What that pattern costs in practice, in one function:

* 29 reads of observed state, most of them gates.
* `provider_configs_observed()`, a gate that existed solely to keep composed
  resources from being created against a ProviderConfig Crossplane had not yet
  persisted.
* `deps_ready()`, re-implementing wave computation over the component list.
* `or key in observed` escape hatches on every gate, because a resource
  withheld from desired state after it exists is a resource Crossplane deletes.
  Getting that wrong deletes live infrastructure.

Converting to edges deleted all of it. The ordering did not get simpler because
the platform got simpler; it got simpler because sequence stopped being control
flow and became data.

That last escape hatch is the part worth dwelling on. The gating pattern
expresses "not yet" and "never again" with the same absence from desired state,
and the function author has to keep them apart by hand, on every gate, forever.
A graph has no such ambiguity: an edge says when, desired state says whether.

## Which of lsviben's POC findings apply to which proposal

His POC surfaced four issues. Two are specific to requiring the pipeline at
teardown; two are shared.

| Finding | #7242 | Declared graph |
| --- | --- | --- |
| Functions cannot wait for a resource to disappear: refs are rewritten from desired each pass, so a resource leaves observed the pass it is deleted | Central. The fix - retain refs until the resource is really gone - is a prerequisite for the whole mechanism | Same fix needed, for the same reason, and this proposal already specifies it |
| Composition deleted before its XRs strands them | Real. Resolution is to fall back to unordered | Not applicable: no pipeline run |
| Foreground deletion races the ordering | Shared | Shared, and documented as inherited |
| Nested XRs are foreground-deleted by #6599, so they cannot use function-controlled deletion | Needs a dynamic decision from a new `DeletionOrdered` condition | Shared to the extent nested XRs are foreground-deleted |

The first row is worth noting: both proposals need Crossplane to keep
referencing a composed resource until it is actually gone. That is shared
groundwork, and it should land once rather than twice.

## Costs of the graph, stated plainly

* New API surface in the function protocol, and new persisted fields on every
  XR that uses it.
* A graph engine in core: cycle detection, contradiction reporting, and a
  decision it must explain when a resource is held back.
* Unproven past 18 composed resources. Deeper graphs trip the realtime
  compositions watch circuit breaker, after which each wave takes about a
  minute rather than seconds.
* A function must return its full edge set on every pass. Dropping edges for
  resources it has removed from desired state discards the ordering exactly
  when it is needed, which is a new way to get it wrong.
* It orders. It does not do work.

## Recommendation

Ship both, with the division of labor this design already names: **desired
state decides what, the graph decides when.** #7242 shrinks desired state
during teardown and lets functions do work there; the graph sequences the
resulting delete set, and keeps sequencing it when the pipeline cannot run.

If only one ships first, ship the graph. It delivers ordered teardown with no
change to existing functions, no per-pipeline opt-in, no silent fallback, and
coverage for templating functions - and it holds when the pipeline is gone,
which is a state every XR reaches eventually.

Land the shared groundwork first either way: retaining references to composed
resources that are deleting, so that "gone from observed" means gone.
