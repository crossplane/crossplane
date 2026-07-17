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
* No orphaned external resources. No permanently stuck `Terminating` objects; if
  a fix can deadlock instead of orphaning, it must provide a force-delete escape
  hatch so the stuck state stays recoverable.

## Non-goals

* Redesigning the `ProviderConfig`/credentials model.
* Changing the user-facing `Usage` API's semantics (only its possible internal
  reuse is in scope).

## Design constraint (the invariant)

> Until every MR that references a `ProviderConfig` has completed the external
> `Delete()` of its resource, that `ProviderConfig` must retain its
> `in-use.crossplane.io` finalizer.

Equivalently: the "in use" signal must be tied to the **completion of the MR's
external cleanup**, not to a GC-managed marker whose lifetime is shorter under
foreground deletion.
Any accepted fix must make the failure sequence described in
[Root cause](#root-cause) impossible.

## Proposal

Base ProviderConfig protection on **real MR existence** rather than GC-lifecycle
PCU counting. There are two horizons: a near-term fix that stabilizes the
existing PCU mechanism, and a long-term move to the watch-real-instances model
already emerging elsewhere in Crossplane.

### Near-term: add a `ProviderConfigUsage` finalizer (Option A)

Add a finalizer to the PCU when it is created and remove it only after the MR's
external `Delete()` succeeds, so the PCU outlives the MR's termination and the
count cannot reach zero early. Of the options that fix both **v1 and v2**, it is
the smallest change that satisfies the invariant, and it preserves the automatic
UX users rely on. Concretely:

* Add the finalizer to the PCU when the provider creates it on connect.
* Give crossplane-runtime's **managed reconciler** ownership of the PCU
  lifecycle: remove the PCU finalizer only after the external `Delete()`
  succeeds, and only **after** the MR's own finalizer has been removed — the PCU
  finalizer is released **last** (see [Teardown ordering](#teardown-ordering)).
* Add a **reaper to the `ProviderConfig` reconciler** as a safety net: if a PCU
  is stuck on its finalizer but its owning MR no longer exists, release it there.
  Once the MR is gone its own reconcile can no longer release the finalizer, so
  without the reaper a failed teardown could strand the PCU — and the PC —
  permanently (see [Teardown ordering](#teardown-ordering)).

The wrinkle is that there is no matching teardown step today. The PCU is created
in the provider's connection path, but the finalizer has to be removed in the
managed **reconciler** after `Delete()`, which currently neither holds the
tracker nor can derive the PCU GVK from the MR (in **upjet family providers** the
PC group is a subset of the MR group). The fix is to extend the
`ProviderConfigUsageTracker` so the managed reconciler owns the full PCU
lifecycle. Providers already hand crossplane-runtime their PCU type when they
build a tracker, so the per-provider change is small; the real cost is that every
provider must still adopt it and the rollout has to be coordinated (see
[Cost and risks](#cost-and-risks)).

### Teardown ordering

The two finalizer removals must happen in a specific order: remove the MR's
own finalizer first, then release the PCU finalizer last.

The managed reconciler calls `Connect()` — which resolves and reads the
`ProviderConfig` — at the top of every reconcile, including the ones that only
finalize an already-deleted MR. Once the PC is itself being deleted, its
`in-use.crossplane.io` finalizer — held only because the PCU still exists — is
the only thing keeping it alive. So if the PCU finalizer is removed
first, the count can hit zero and the PC is deleted; if the subsequent removal of
the MR's own finalizer then needs a retry — e.g. an API conflict, which is common
under foreground deletion where the GC concurrently strips the
`foregroundDeletion` finalizer off the same MR — that retry can no longer
`Connect` (the PC is gone) and the MR is left stuck `Terminating`, recoverable
only by re-creating the PC. Removing the MR finalizer first keeps the PC protected
until the MR is durably finalized, so `Connect` keeps working while the PC is
still needed.

Ordering alone still leaves one gap: under **background** deletion the MR is
garbage-collected the instant its own finalizer is gone, so if the PCU-finalizer
removal then fails (e.g. a transient API error) nothing re-queues that MR to
retry — its reconcile no longer exists. The reaper in the **`ProviderConfig`
reconciler** closes it. That reconciler already watches PCUs, so when it sees a
PCU that is being deleted, still holds its finalizer, and whose owning MR no
longer exists (checked against the API server, not the cache), it releases the
finalizer itself. Being independent of the MR's reconcile, it recovers the stuck
PCU — and therefore the ProviderConfig — no matter how the MR-side teardown
failed.

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

* **Ecosystem rollout with an ordering footgun.** Every PCU-creating provider
  must adopt it; until then it stays broken. The finalizer-**add** must never
  ship before the **removal** path in the same binary, or PCUs become
  undeletable. This must be gated (feature flag / version guard) and coordinated
  across crossplane-contrib, upjet, and third-party providers.
* **Upgrading existing resources (migration).** MRs created before the fix have
  PCUs with no finalizer. On upgrade the managed reconciler heals them
  automatically for MRs that are **not yet deleting** — the connect-path `Track`
  treats a missing finalizer as an update and adds it on the MR's next reconcile,
  so no manual intervention or re-creation is needed. The gap is an MR that is
  **already `Terminating`** when the upgrade lands: Kubernetes forbids adding a
  finalizer once `deletionTimestamp` is set, so that PCU can't be healed and its
  MR could delete unprotected. This is a narrow window (an MR deleting at the
  exact moment of upgrade); if it must be closed, gate the rollout on no target
  MRs being mid-deletion.
* **Upjet family providers.** The PCU group is a subset of the MR group and one
  binary serves many groups, so wiring the correct PCU type per controller is
  more than a one-liner — exactly where most MRs live.
* **New failure mode: stuck resources.** If removal fails (bug, unreachable
  external system, credentials already gone, `Orphan` policy), the PCU sticks and
  the ProviderConfig is stuck `Terminating`. The [reaper](#teardown-ordering)
  auto-recovers the case where the owning MR is already gone; but where the MR
  still exists yet its removal can't proceed (e.g. `Orphan`, an unreachable
  external system), a permanent deadlock can be worse than an orphan, so a
  force-delete escape hatch remains a hard requirement of this option (see
  [Goals](#goals)), not optional edge-case handling.
* **v2 namespace deletion.** A namespaced PCU finalizer will block namespace
  finalization until external deletes complete (arguably correct, but a behavior
  change) and, combined with the above, risks a permanently stuck namespace.
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
| A. PCU finalizer (proposed) | crossplane-runtime | yes | yes | S–M | rollout; upjet GVK |
| B. Generic `Usage` | core + runtime | yes | yes | L | webhook scale; UX regression |
| C. `protection.*` PCU | core/runtime | yes | yes | M–L | new API + migration |
| D. Watch instances (#7362 style) | core (MRD) | **no**\* | yes | M–L | v2-only; not reusable |

\* Option D as it ships in [crossplane#7362] today is core-hosted and driven by
the v2-only `ManagedResourceDefinition`, hence "Covers v1: no". The
[long-term proposal](#long-term-watch-real-managed-resource-instances-option-d)
is to re-home this model in crossplane-runtime so it also covers **v1**; the
table rates the mechanism as it exists now, not that re-homed variant.

### B. Converge on the generic `Usage` type

Drop PCUs; auto-create the existing [`Usage`][usage-onepager] resource
(webhook-based, `replayDeletion`) per MR.

* **Pros:** One mechanism instead of two; `Usage` already handles ordered
  deletion correctly.
* **Cons:** `Usage` is webhook-backed and would now be created **per MR** — far
  higher scale than today's hand-authored Usages (needs scale testing). UX
  regression: `kubectl get usage` would show `(#MRs) + user-authored`. Largest
  behavior change; maintainers have flagged both concerns in
  [#4661].

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
* [one-pager-generic-usage-type.md][usage-onepager]

[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime
[usage-onepager]: ./one-pager-generic-usage-type.md
[crossplane/crossplane#4661]: https://github.com/crossplane/crossplane/issues/4661
[#4661]: https://github.com/crossplane/crossplane/issues/4661
[#5849]: https://github.com/crossplane/crossplane/issues/5849
[crossplane-runtime#1049]: https://github.com/crossplane/crossplane-runtime/issues/1049
[#848]: https://github.com/crossplane/crossplane-runtime/issues/848
[crossplane#7362]: https://github.com/crossplane/crossplane/pull/7362
[crossplane#7442]: https://github.com/crossplane/crossplane/pull/7442
[crossplane-runtime#855]: https://github.com/crossplane/crossplane-runtime/pull/855
