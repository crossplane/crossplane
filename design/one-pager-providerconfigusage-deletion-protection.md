# ProviderConfig deletion protection under foreground deletion

* Owner: Ezgi Demirel (@ezgidemirel)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

A `ProviderConfig` (PC) carries the credentials a managed resource (MR) uses to
talk to its external system. Deleting a PC while MRs still depend on it strands
those MRs: they can no longer authenticate, so their external resources are
never cleaned up (orphaned) and the objects hang in `Terminating`.

Crossplane protects a PC through a marker-counting mechanism in
[crossplane-runtime] that runs inside every provider:

* Each time a provider connects to an MR's external system, it creates a
  `ProviderConfigUsage` (PCU) — a marker that records "MR X uses PC Y".
* The PC reconciler adds an `in-use.crossplane.io` finalizer to the PC and keeps
  it as long as at least one PCU exists, removing it (and allowing deletion) only
  once the count reaches zero.

In effect, PC protection treats **"PCU count > 0" as a proxy for "MRs still
exist."** That proxy breaks down under foreground deletion.

### Root cause

Two facts collide:

1. Each PCU is **owned by its MR** through a controller `ownerReference` with
   `blockOwnerDeletion: true`, and has **no finalizer of its own**. Its lifetime
   is therefore governed by the Kubernetes garbage collector (GC).

2. `blockOwnerDeletion` is only consulted under **foreground** deletion, so the
   two propagation policies behave differently:
   * **Background** (kubectl default): the GC removes the owner (MR) first and its
     dependents afterwards, so a PCU is collected only *after* its MR is gone. The
     count does not drop early, the proxy holds, and there is no bug — but note
     this is ordinary GC ordering, not a `blockOwnerDeletion` guarantee.
   * **Foreground**: the GC must delete a `blockOwnerDeletion` dependent (the PCU)
     **before** it removes the owner (the MR). The PCU disappears while the MR
     is still `Terminating`; the count hits `0`; the PC's finalizer is removed;
     the PC is deleted out from under a still-terminating MR.

This is why the failure is specific to `compositeDeletePolicy: Foreground`
(claims) and, more broadly, any foreground/cascading delete
(`kubectl delete --cascade=foreground`).

### It is a race, not a hard failure

The managed reconciler re-connects to the provider's external system on
**every** reconcile, including during deletion, and each connection creates the
PCU unconditionally. So the provider keeps **re-creating** the PCU, and usually
recreates it fast enough to hide the gap. The bug only surfaces when the PC
reconciler observes a count of zero in the window between the PCUs being deleted
and the provider re-creating them — a window that bulk concurrent deletion
reliably opens. A foreground cascade opens it through the GC (many MRs
garbage-collected at once, the PC deleted concurrently). **v2 namespace deletion**
opens the same window by a different route: the namespace lifecycle controller
bulk-deletes every object in the namespace — PCUs, MRs, and PC together — so the
count can still hit zero even without the `blockOwnerDeletion` ordering from
[Root cause](#root-cause). Here the race is the whole story.

## Goals

* PC deletion protection that is correct under **foreground/cascading**
  deletion, not just background.
* Cover both **v1** and **v2** environments.
* Do not orphan external resources unless the MR's deletion policy explicitly
  requests it. Do not permanently strand `Terminating` objects; if cleanup
  cannot complete, provide an explicit
  [force-orphan escape hatch](#force-orphan-escape-hatch).

## Non-goals

* Redesigning the `ProviderConfig`/credentials model.
* Changing the user-facing `Usage` API's semantics (only its possible internal
  reuse is in scope).

## Design constraint (the invariant)

> Until every MR that references a `ProviderConfig` has completed the deletion
> behavior selected by its policy and durably released its Crossplane finalizer,
> that `ProviderConfig` must retain its `in-use.crossplane.io` finalizer.

For the default Delete policy this means the external resource has been observed
gone. For Orphan, or management policies that omit Delete, it means intentional
orphaning has completed and the MR can be finalized. A paused MR remains
protected and cannot finish deletion until it is unpaused under today's pause
semantics. Equivalently: the "in use" signal must be tied to the **completion of
the MR's selected teardown behavior**, not to a GC-managed marker whose lifetime
is shorter under foreground deletion.

Any accepted fix must make the failure sequence described in
[Root cause](#root-cause) impossible.

## Proposal

Base ProviderConfig protection on **real MR existence** rather than GC-lifecycle
PCU counting. There are two horizons: a near-term fix that stabilizes the
existing PCU mechanism, and a long-term move to the watch-real-instances model
already emerging elsewhere in Crossplane.

Four approaches were considered, labelled **A**–**D** and compared side by side
in [Alternatives Considered](#alternatives-considered). The two referenced below
are **A**, the PCU finalizer proposed here, and **D**, watching real MR
instances, which is the long-term direction.

### Near-term: add a `ProviderConfigUsage` finalizer (Option A)

Add a finalizer to the PCU once its MR has connected, and remove it only after
the MR has completed the deletion behavior selected by its policy and its own
finalizer has been durably removed. This makes the PCU outlive the part of MR
termination that still needs the PC, so the count cannot reach zero early. Of the
options that cover both **v1 and v2**, it is the least invasive way to satisfy
the invariant — it changes how the existing marker behaves rather than
introducing a new API or a new controller — and it preserves the automatic UX
users rely on. Concretely ([crossplane-runtime#1113]):

* The **managed reconciler** owns both ends of the finalizer. Right after the
  MR connects it *protects* the PCU the provider's tracker has just recorded,
  adding the finalizer if it is missing and writing nothing otherwise — the
  connect path runs on every reconcile, so an unconditional update would be
  unacceptable API write amplification. The tracker itself never touches the
  finalizer, and preserves whatever finalizers the PCU already carries when it
  updates the PC reference.
* The reconciler *releases* the PCU after the MR has completed the deletion
  behavior selected by its policy — Delete cleanup observed complete, or
  Orphan/no-Delete selected — and only **after** the MR's own finalizer has been
  removed. The PCU finalizer is released **last**, on both paths (see
  [Teardown ordering](#teardown-ordering)).
* A **reaper in the `ProviderConfig` reconciler** backstops the MR-side release.
  When a PCU is terminating and still finalized but its owning MR is gone, has
  been replaced, or has itself finished tearing down (deleting, with no
  finalizers other than the garbage collector's), the reaper releases the
  finalizer, since nothing else will (see
  [Teardown ordering](#teardown-ordering)).

The PCU is created in the provider's connection path, but the finalizer has to
be added and removed by the managed **reconciler**, which until now neither held
the tracker nor could derive the PCU GVK from the MR (in **upjet family
providers** the PC group is a subset of the MR group). So the tracker gains a
protect and a release operation, and the managed reconciler gains an option
through which the provider hands it the tracker. Providers already give
crossplane-runtime their PCU type when they build a tracker, so the
per-provider change is one option per managed reconciler. Making that option
the thing that enables the finalizer keeps adoption safe per binary, without
requiring all providers in the ecosystem to upgrade in lockstep (see
[Rollout and adoption](#rollout-and-adoption)).

### Rollout and adoption

The governing rule: **a PCU finalizer must never be created unless the code that
removes it is wired up in the same provider binary.** Creation without removal
produces PCUs nothing will release, and ProviderConfigs that can never be deleted.
Upgrading crossplane-runtime must not, on its own, start creating finalized PCUs.

A feature flag alone does not deliver that — it still lets a provider enable the
feature without wiring removal — so the rule is enforced by the shape of the API
rather than by a flag. The finalizer is written only by the managed reconciler's
protect step and removed only by its release step, both through the tracker the
provider supplies with the option. A reconciler built without the option never
adds the finalizer, never removes one, and behaves exactly as today. A tracker
on its own cannot create a finalized PCU; adding the finalizer when the tracker
records the usage would turn creation on for every provider that merely
upgraded crossplane-runtime, with nothing wired to remove it.

Nothing requires determining that every provider in the ecosystem has adopted the
model first, and there is no reliable way to measure it. Providers that do not opt
in keep today's behavior and stay exposed to the race, but do not hold back those
that have.

### Teardown ordering

The two finalizer removals must happen in a specific order: remove the MR's
own finalizer first, then release the PCU finalizer last.

The managed reconciler calls `Connect()` — which resolves and reads the
`ProviderConfig` — at the top of every reconcile, including the ones that only
finalize an already-deleted MR. Once the PC is itself being deleted, its
`in-use.crossplane.io` finalizer — held only because the PCU still exists — is the
only thing keeping it alive.

Releasing the PCU finalizer first therefore lets the count hit zero and the PC be
deleted. If removing the MR's own finalizer then needs a retry, that retry can no
longer `Connect`, and the MR is left stuck `Terminating`, recoverable only by
re-creating the PC. Such a retry is not unlikely: an API conflict is common under
foreground deletion, where the GC concurrently strips the `foregroundDeletion`
finalizer off the same MR. Removing the MR finalizer first keeps the PC protected
until the MR is durably finalized, so `Connect` keeps working while the PC is
still needed.

Ordering alone still leaves a gap. Under **background** deletion the MR is
garbage-collected the instant its own finalizer is gone, so if the PCU-finalizer
removal then fails (e.g. a transient API error) nothing re-queues that MR to
retry — its reconcile no longer exists. Under **foreground** deletion the MR
object lingers on `foregroundDeletion` until the PCU is gone, but its reconcile
has finished its work and will not run again on the usage's behalf. The reaper in
the **`ProviderConfig` reconciler** closes both. That reconciler already watches
PCUs, so when it sees a PCU that is being deleted and still holds its finalizer,
it looks up the owning MR — against the API server rather than the cache, and by
UID, so a re-created MR of the same name does not count — and releases the
finalizer if the owner is gone, was replaced, or is itself being deleted with no
finalizers other than the garbage collector's own. While the owner still exists
and still holds its own finalizer, the reaper requeues and leaves the MR
reconciler responsible for retrying; it never overrides protection an MR still
needs.

The reaper does **not** need to confirm that external cleanup completed: the MR's
own finalizer comes off only after teardown has run, so "the owning MR is gone or
has released its finalizer" is already proof of it.

The reaper is a backstop, not the primary path; the MR-side release normally
runs first and the reaper finds nothing to do. Strictly, the ordering argument
applies only while the MR still needs the PC during teardown: under Orphan or
no-Delete it no longer does, and the PCU could be released first. The
implementation keeps the same order on both paths so that one rule holds
everywhere and the reaper backstops both.

### Foreground deletion and the delayed-delete gate (v2 only)

On the v2 line the PCU finalizer cannot ship on its own. It must be paired with a
change to the delayed-delete gate introduced by [crossplane-runtime#855], which
holds back the external `Delete()` call while the MR carries any finalizer beyond
its own, on the grounds that another controller may still need the external
resource. Without that change the near-term fix protects the `ProviderConfig`
correctly and then deadlocks the managed resource.

Foreground deletion always puts a second finalizer on the MR — Kubernetes' own
`foregroundDeletion` — so the gate always engages. Today that is harmless because
the PCU carries no finalizer: the GC collects it promptly and the extra finalizer
is stripped again. Give the PCU a finalizer and the wait becomes circular. The PCU
is released only after external `Delete()`, which the gate holds back until the GC
strips `foregroundDeletion`, which the GC will not do until the PCU is gone.
Nothing can move first, and the reaper cannot help because the MR never releases
its own finalizer — it is stuck before external deletion, not after it. Only the
Orphan/no-Delete path escapes, since it never reaches the gate.

The fix is for the gate to disregard Kubernetes' own garbage-collection
finalizers and count only those belonging to other controllers. That matches the
gate's stated intent — `foregroundDeletion` is not another controller and makes no
claim on the external system — and it is worth doing independently of this
proposal, since today every foreground MR deletion waits on a garbage-collection
round trip before external deletion for no benefit. The gate therefore counts
only finalizers other than `foregroundDeletion` and `orphan`. Anyone who runs
their own PCU-finalizer controller as a workaround for the namespace race hits
the same deadlock on today's gate, so the change is needed with or without the
finalizer proposed here.

### Force-orphan escape hatch

Some situations can never satisfy the invariant: an external system that stays
unreachable, or credentials that are already gone (see the v2 namespace-deletion
risk under [Cost and risks](#cost-and-risks)). A paused MR is not one of them:
pausing is an explicit instruction not to act, deletion included, and the
conflict with a pending delete is resolved by whoever paused it unpausing it, not
by the controller. Waiting indefinitely on the genuinely unrecoverable cases is
not an acceptable outcome, so the design requires a deliberate way out.

The escape hatch releases protection rather than repairing it. It abandons the
outstanding PCU finalizers so the `ProviderConfig` and its MRs can finalize, and
makes no attempt to clean up the external resources — those are left behind. That
is the point: it converts an indefinite block into an **explicit, recorded decision
to orphan**. It therefore has to be an operator action that leaves an audit trail,
never something a controller decides on its own, and it must not be reachable by
accident. It does not verify that anything was cleaned up; the operator invoking it
is asserting that orphaning is acceptable. The precise surface is left to
implementation.

### Long-term: watch real managed-resource instances (Option D)

Align with the watch-real-instances direction already shipped for Providers
([crossplane#7362]) and in review for XRDs/Configurations ([crossplane#7442]).
To also serve v1, that model should live in crossplane-runtime rather than only
in core/MRD. The near-term finalizer does not preclude it — it stabilizes the
current mechanism while the unified model is designed. See
[Alternatives Considered](#alternatives-considered) for why #7362 cannot be
reused as-is today.

### Cost and risks

The price of the near-term fix is rollout and edge cases, not the diff:

* **Per-provider rollout with an ordering footgun.** The finalizer-**add** must
  never ship active before the **removal** path in the same binary, or PCUs become
  undeletable; coupling the two in the API prevents that (see
  [Rollout and adoption](#rollout-and-adoption)). Providers that have not adopted
  the feature stay exposed to the existing race but do not block those that have.
* **Upgrading existing resources (migration).** MRs predating the fix have PCUs
  with no finalizer. Those heal on the MR's next reconcile: the protect step runs
  after every successful connect and adds the missing finalizer in place. The
  gap for MRs already **`Terminating`** at upgrade is narrower than it first
  appears. A PCU with no finalizer does not linger in `Terminating` — the
  garbage collector removes it as soon as it is marked — so there is no object
  to which Kubernetes could refuse a finalizer. Once it is gone, the MR's next
  reconcile re-creates the PCU and the protect step finalizes it, restoring
  protection. What remains is the pre-existing race between that
  collection and the re-creation, which the upgrade neither widens nor closes.
  Enabling the feature should still not be presented as protecting objects
  already terminating at the moment of upgrade.
* **New failure mode: stuck resources.** Failures before MR finalization, such as
  an unreachable external system under Delete policy, correctly keep the PCU and
  PC protected until the operator resolves them. A **paused** MR that is deleted
  looks like the sharpest case: pause short-circuits the reconcile before any
  teardown, so no finalizer is removed, and because the MR still exists and still
  holds its finalizer the [reaper](#teardown-ordering) stays out — the PC, and in
  v2 the namespace, is blocked until it is unpaused. That is correct under the
  invariant and, more to the point, it is what pause means: an explicit
  instruction not to act, deletion included. The controller should not resolve
  the conflicting intents; whoever paused the MR does, by unpausing it, after
  which deletion proceeds normally.
* **v2 namespace deletion.** This risk is v2-only: in v1 the ProviderConfig, its
  MRs and their PCUs are all cluster-scoped, so no namespace lifecycle applies. In
  v2 a namespaced PCU finalizer holds up namespace finalization until external
  deletes complete — arguably correct, but a behavior change. Worse, the namespace
  controller may delete the credentials `Secret` first, after which the provider
  cannot authenticate and MR, PCU, PC and namespace finalization can all block.
  Note the distinction from the route this proposal does close: a finalized PCU
  is not swept with the namespace, so the case where a PCU is deleted and cannot
  be re-created in a terminating namespace no longer arises.
  Finalizing the Secret is out of scope: Secrets are shared, often owned by other
  controllers, and only one of several credential sources. Credential lifetime
  needs its own dependency-protection design and should be tracked as a
  follow-up issue; until then this is an explicit limitation, and the main
  reason the [escape hatch](#force-orphan-escape-hatch) exists.
* **Hardens a possibly-deprecated mechanism.** If PCU is retired in v2 in favor
  of the unified model, the v2 half of this work is throwaway (still justified
  for v1), and it entrenches an indirect proxy rather than advancing the unified
  protection story.

## Alternatives Considered

The following approaches were weighed against the proposal. The deciding axes are
**where the fix lives** (provider/runtime vs. core) and **which versions it
covers**.

| Option | Lives in | Covers v1 | Covers v2 | Effort | Main risk |
|--------|----------|:---------:|:---------:|--------|-----------|
| A. PCU finalizer (proposed) | crossplane-runtime | yes | yes | S–M | rollout; upjet; gate† |
| B. Generic `Usage` | core + runtime | yes | yes | L | webhook scale; UX regression |
| C. `protection.*` PCU | core/runtime | yes | yes | M–L | new API + migration |
| D. Watch instances (#7362 style) | core (MRD) | **no**\* | yes | M–L | v2-only; not reusable |

† Option A is not symmetric across versions. On **v2** it additionally requires
the [delayed-delete gate change][gate-section], without which foreground deletion
deadlocks; the v1 line needs no equivalent.

\* Option D as it ships in [crossplane#7362] today is core-hosted and driven by
the v2-only `ManagedResourceDefinition`, hence "Covers v1: no". The
[long-term proposal](#long-term-watch-real-managed-resource-instances-option-d)
is to re-home this model in crossplane-runtime so it also covers **v1**; the
table rates the mechanism as it exists now, not that re-homed variant.

**B** and **C** share one property the proposal lacks, and it is worth stating on
its own because it is a genuine advantage over Option A. Both are webhook-backed,
and a rejected deletion gives the namespace controller something to report, so an
operator sees `NamespaceDeletionContentFailure` on the namespace itself. Option A
is not silent — the `ProviderConfig` reconciler reports a `Terminating` condition
and a user count explaining that deletion is blocked — but it reports on the PC
only. Nothing propagates the reason up to the namespace, which is where an
operator debugging a stuck namespace looks first. That also makes the
[force-orphan escape hatch](#force-orphan-escape-hatch) harder to find under A.

### B. Converge on the generic `Usage` type

Drop PCUs; auto-create the existing [`Usage`][usage-onepager] resource
(webhook-based, `replayDeletion`) per MR.

* **Pros:** One mechanism instead of two; `Usage` already handles ordered
  deletion correctly.
* **Cons:** `Usage` is webhook-backed and would now be created **per MR** — far
  higher scale than today's hand-authored Usages (needs scale testing). UX
  regression: `kubectl get usage` would show `(#MRs) + user-authored`. Largest
  behavior change; maintainers have flagged both concerns in [#4661].

### C. New `protection.crossplane.io` ProviderConfigUsage

A dedicated `protection.crossplane.io/v1alpha1 ProviderConfigUsage` with
`by`/`of`/`replayDeletion` semantics, scoped only to ProviderConfigs — reuses
`Usage`'s correct ordering logic without conflating with the user-facing
`Usage`.

* **Pros:** Correct semantics; preserves separation users expect.
* **Cons:** New API + migration path; medium effort.

### D. Watch real MR instances (controller-based)

This is the long-term direction of the [Proposal](#proposal); it is listed here
because it cannot be adopted as the near-term fix. Mirror the merged Provider
deletion-protection work ([crossplane#7362]): a controller that **watches actual
MR instances** and manages a `Usage`/`ClusterUsage` based on whether any MR still
references a given PC — keying protection off real existence rather than a
GC-lifecycle marker.

* **Pros:** Removes the faulty proxy entirely; no `blockOwnerDeletion`/GC-timing
  dependency and no per-MR webhook. Aligns with the direction already shipped
  for Providers and in review for XRDs/Configurations ([crossplane#7442]).
* **Cons — confirmed by reading #7362:** the only genuinely reusable part is the
  *engine scaffolding* — the machinery that dynamically starts a per-type
  controller to watch MR instances. The reconcile logic itself is per-MRD and
  Provider-specific: it walks MRD → ProviderRevision → Provider to resolve the
  provider name and hardcodes `Kind: Provider`, so it assumes a **1:1**
  relationship (one Provider per MRD). PC protection is **many-to-many** — each
  MR *instance* names its PC in `spec.providerConfigRef`, and instances of the
  same type may point at different PCs — so it would need a new reconciler that
  groups instances by `providerConfigRef`. On top of that, #7362 lives in
  **core**, is driven by `ManagedResourceDefinition` (a **v2-only** concept), and
  sits behind the alpha flag `EnableAlphaProviderDeletionProtection`. Adopting it
  as-is would make PC protection **v2-only**, whereas the mechanism today lives in
  the provider and already works on v1.

## References

* [crossplane/crossplane#4661] — original issue; [#5849] duplicate
* [crossplane-runtime#1049], [#848] — root-cause write-ups
* [crossplane#7362] — Provider deletion protection via `ClusterUsage` (merged)
* [crossplane#7442] — XRD/Configuration deletion protection (in review)
* [crossplane-runtime#855] — delay external delete while extra finalizers exist
* [crossplane-runtime#1113] — implementation of the near-term fix (Option A)
* [one-pager-generic-usage-type.md][usage-onepager]

[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime
[crossplane-runtime#1113]: https://github.com/crossplane/crossplane-runtime/pull/1113
[gate-section]: #foreground-deletion-and-the-delayed-delete-gate-v2-only
[usage-onepager]: ./one-pager-generic-usage-type.md
[crossplane/crossplane#4661]: https://github.com/crossplane/crossplane/issues/4661
[#4661]: https://github.com/crossplane/crossplane/issues/4661
[#5849]: https://github.com/crossplane/crossplane/issues/5849
[crossplane-runtime#1049]: https://github.com/crossplane/crossplane-runtime/issues/1049
[#848]: https://github.com/crossplane/crossplane-runtime/issues/848
[crossplane#7362]: https://github.com/crossplane/crossplane/pull/7362
[crossplane#7442]: https://github.com/crossplane/crossplane/pull/7442
[crossplane-runtime#855]: https://github.com/crossplane/crossplane-runtime/pull/855
