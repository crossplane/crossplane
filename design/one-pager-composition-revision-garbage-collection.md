# Composition Revision Garbage Collection

* Owner: Adam Wolfe Gordon (@adamwg)
* Reviewers: Philippe Scorsolini (@phisco), Jared Watts (@jbw976)
* Status: Draft

## Background

Crossplane creates a new `CompositionRevision` every time a `Composition`'s
spec, labels, or annotations change. The revision is a snapshot of the
`Composition`. XRs use the latest revision by default, but can be pinned to a
specific revision by name or using label selectors. Today,
`CompositionRevision`s are never deleted automatically. They're only removed
when their `Composition` is deleted, or manually.

A `Composition` that lives for a long time and changes often can accumulate many
revisions, most of them unused. They take up space in etcd and make `kubectl get
compositionrevisions` hard to read (see [issue #4837]).

## Goals

* Automatically clean up unused `CompositionRevisions`.
* Let users control how many unused revisions to keep or disable garbage
  collection altogether.
* Never delete a revision that is referenced by an XR or is the most recent
  revision of its `Composition` (the default revision).

## Proposal

### Precedent

The package manager also creates revisions of its package types
(`Configuration`, `Provider`, and `Function`). These types include a
`spec.revisionHistoryLimit` field which defaults to 1 and controls how many
inactive revisions to retain. Older revisions are garbage collected by the
`manager` reconciler.

Note that, unlike with compositions, only one revision of a package can be
active at any given time, and in the common case the most recent revision is the
active one. This makes package revision garbage collection somewhat simpler,
since there are no external references to consider.

### API

I propose that we add a `revisionHistoryLimit` field to the Composition spec,
similar to the field of the same name on packages:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: example.org/v1
    kind: XExample
  # Keep up to 3 unused revisions.
  # Defaults to 1. 0 disables garbage collection.
  revisionHistoryLimit: 3
  mode: Pipeline
  pipeline: []
```

The limit counts only revisions that are unused and aren't the latest. So with
the default of 1, Crossplane keeps the latest revision, one previous revision,
and every revision an XR uses.

### Garbage collection

A revision is in use if any of the following is true:

* An XR names it in `compositionRevisionRef`.
* It's the newest revision that matches an XR's `compositionRevisionSelector`.
* It's the newest revision that matches an XRD's
  `defaultCompositionRevisionSelector`.

Note that `compositionRevisionRef` and `compositionRevisionSelector` are at
different locations depending on the XRD version: for v1 XRs they are directly
under `spec`, while v2 XRs nest them under `spec.crossplane`. Garbage collection
will need to consider both. For efficiency, I propose that we consider both
possibilities for every XR rather than resolving each relevant XRD to determine
which field location to read.

Each time the Composition revision controller reconciles a Composition, it:

1. Keeps the latest revision.
2. Keeps every revision that's in use.
3. Keeps the newest `revisionHistoryLimit` of the remaining revisions.
4. Deletes any other revisions.

Note that the garbage collected revisions may not always be the oldest
revisions. If an older revision is in use and a newer revision is unused, the
newer one will be deleted. This avoids revisions accumulating indefinitely if
some XR is pinned to a very old revision.

### Upgrade concerns

When users upgrade to a new Crossplane version that supports garbage collection,
all their compositions will get the default `revisionHistoryLimit` of 1. This
has two implications.

First, the composition reconciler will start cleaning up old revisions
immediately on upgrade. Users who wish to retain old revisions will need to do a
three step upgrade: update the `Compositions` CRD, set the
`revisionHistoryLimit` on their existing compositions to 0 (or their desired
value), then upgrade the Crossplane controller.

Second, without an additional change to the composition reconciler new revisions
would be created on upgrade for all existing compositions. Crossplane determines
whether to create a new revision by hashing the Composition using the YAML
representations of the relevant parts (labels, annotations, and spec). Naively,
`spec.revisionHistoryLimit` would become part of this hash after the upgrade,
resulting in new revisions for all compositions. Updating the hash logic to
ignore `spec.revisionHistoryLimit` will solve this problem. Given that the
`revisionHistoryLimit` is not part of the `CompositionRevision`, it's natural to
keep it out of the hash anyway (a user may also later change the
`revisionHistoryLimit` without a new revision being created).

### Edge cases

To find the XRs, the controller lists them from the cache the XR controllers
already use. It doesn't make a new API call for each reconcile. If it can't
list the XRs or the XRD for any reason, it skips cleanup and emits a warning
event. It never deletes a revision when it doesn't know whether that revision
is in use. Cleanup errors don't fail the reconcile, because the controller's
main job of creating revisions already succeeded.

When there are no more than `revisionHistoryLimit + 1` revisions, nothing can
be deleted. The controller returns early without listing XRs. This is the
normal steady state.

There's one race condition: if a user points a Manual XR at an unused revision
at the same moment Crossplane deletes it, the XR will reference a non-existent
revision and fail to reconcile. The latest revision is never deleted, so XRs
with an Automatic update policy are never affected. This kind of race is
unavoidable in a distributed system.

## Alternatives Considered

* **Only delete revisions older than the oldest one in use.** This is simpler,
  but XRs pinned with a Manual update policy are common and can stay pinned for
  months. One pinned XR would stop cleanup for its Composition.
* **Track usage from the XR side**, for example with finalizers or labels that
  XR controllers add to revisions. This would mean XR controllers write to
  revisions on every revision change, and it adds new ways for things to go
  wrong, such as finalizers left behind. Listing XRs from the cache gives the
  same answer without new state.
* **List XRs without the cache.** This always gives fresh data, but it does a
  full list of every XR of the type on every reconcile. The cache is already
  running, so using it costs almost nothing.
* **Delete one revision per reconcile**, as the package manager does. This is
  slow for Compositions with a lot of history, and there's no benefit to going
  slowly. The controller already knows which revisions it can delete.

[issue #4837]: https://github.com/crossplane/crossplane/issues/4837
