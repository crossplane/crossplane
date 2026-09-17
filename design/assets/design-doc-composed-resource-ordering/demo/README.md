# Demo: ordered creation and deletion of composed resources

A run-of-show for showing composed resource ordering to other people. About ten
minutes of material, plus a few minutes of unattended setup.

## The one-sentence version

Crossplane creates and deletes a Composite Resource's composed resources in no
particular order, by design. This lets a function declare a dependency graph,
and Crossplane sequences its own creates and deletes from it — including when
the XR itself is deleted, which no function can influence today.

## Setup

`up.sh` builds Crossplane from the working tree, so check out the prototype
first. Every command below is run from this directory:

```shell
gh pr checkout 7842 --repo crossplane/crossplane
cd design/assets/design-doc-composed-resource-ordering/demo

./up.sh          # a few minutes, mostly pulling images
./watch.sh       # in a second pane, projected
```

Both the function and provider images are published — nothing else is built
locally.

Keep `watch.sh` on screen throughout. Every beat below is watched there, not
read from command output.

## Beat 1 — creation happens in waves

> "Ten resources. One Composition. Watch the order they appear in."

```shell
kubectl apply -f manifests/01-create.yaml
kubectl apply -f manifests/xr.yaml
```

In the watch pane, resources appear a level at a time, roughly four seconds
apart:

```
standalone, vpc                          ← nothing to wait for
subnet-a, subnet-b, gateway, sg          ← once vpc is Ready
instance, database                       ← the diamond converges
app, backup
```

The two points worth making:

- **`standalone` appears immediately.** Ordering holds back only what actually
  has a dependency; it is not a global sequencer.
- **`subnet-a` and `subnet-b` appear together**, and `gateway` and `sg` with
  them. Resources at the same depth still go in parallel. This is ordering, not
  serialization.

Afterwards, the timestamps make the waves exact — this is the most convincing
single artifact in the demo, and worth leaving on screen:

```shell
kubectl -n default get nopresources.nop.crossplane.io \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns='CREATED:.metadata.creationTimestamp,RESOURCE:.metadata.annotations.crossplane\.io/composition-resource-name'
```

```text
CREATED                RESOURCE
2026-09-17T14:59:31Z   vpc          ← roots, together
2026-09-17T14:59:31Z   standalone
2026-09-17T14:59:35Z   subnet-b     ← +4s, the vpc's dependents, together
2026-09-17T14:59:35Z   gateway
2026-09-17T14:59:35Z   sg
2026-09-17T14:59:35Z   subnet-a
2026-09-17T14:59:39Z   database     ← +4s
2026-09-17T14:59:48Z   instance     ← waited for all three of its dependencies
2026-09-17T14:59:49Z   backup
2026-09-17T14:59:55Z   app
```

Then show why the XR isn't just slow:

```shell
kubectl -n default describe xnetwork demo | sed -n '/Events:/,$p'
```

> "It's not stuck, and it's not silent. It says what it's waiting for."

One event per reconcile, naming what's held back and why — because a wide graph
can hold back a dozen resources at once, and a dozen near-identical events would
bury everything else.

## Beat 2 — the order survives on the XR

> "Where does the order live? Not in the function's memory."

```shell
kubectl -n default get xnetwork demo \
  -o jsonpath='{range .spec.crossplane.resourceRefs[*]}{.resourceName}{" <- "}{.dependsOn}{"\n"}{end}'
```

The graph is persisted in the XR's own references. That is what makes the next
beat possible: **teardown does not run the function pipeline**, so without this
there would be nothing to order by.

It also means any tool with a `GET` on the XR can read the graph.

## Beat 3 — deletion runs in reverse

Reset onto the composition whose resources also take time to *delete*:

```shell
kubectl delete -f manifests/xr.yaml --wait
kubectl apply -f manifests/02-teardown.yaml
kubectl apply -f manifests/xr.yaml
# wait for everything to go Ready in the watch pane, then:
kubectl delete -f manifests/xr.yaml --wait=false
```

Watch the `DELETING` column fill in from the leaves inward — `app`, `backup`,
`standalone` first; `vpc` last. A normal cascade stamps all ten at once.

> "Kubernetes garbage collection would delete these in whatever order it felt
> like. This is the case you cannot fix with a function today, because when the
> XR is being deleted, the pipeline doesn't run."

## Beat 4 — something refuses to delete

This is the beat people remember, because it shows the failure mode.

```shell
kubectl delete -f manifests/xr.yaml --wait
kubectl apply -f manifests/03-blocked.yaml
kubectl apply -f manifests/xr.yaml
# once Ready:
kubectl delete -f manifests/xr.yaml --wait=false
```

`app` refuses to delete. Teardown makes partial progress — `standalone`,
`gateway`, `backup`, then `database` — and then stops. The XR stays, holding its
finalizer:

```shell
kubectl -n default get xnetwork demo \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}{"\n"}'
```

> "It blocks, and it tells you what it's blocked on and for how long. It does
> not give up and cascade — cascading would delete the rest out of order, which
> is the whole thing we're trying to prevent."

Then fix it live:

```shell
kubectl -n default patch nopresource \
  "$(kubectl -n default get nopresources -o jsonpath='{range .items[?(@.metadata.annotations.crossplane\.io/composition-resource-name=="app")]}{.metadata.name}{end}')" \
  --type=merge -p '{"spec":{"forProvider":{"deleteError":null}}}'
```

Teardown resumes from where it stopped and runs to completion.

## Questions you should expect

**"Is this merged?"** No. It's an alpha prototype behind
`--enable-composed-resource-ordering`, open as
[crossplane#7842](https://github.com/crossplane/crossplane/pull/7842),
alongside the design in
[crossplane#7841](https://github.com/crossplane/crossplane/pull/7841). With the
flag off, nothing changes.

**"Does it stop *me* deleting a composed resource?"** No. Ordering constrains
Crossplane's own calls, not anyone else's — a direct `kubectl delete` on a
composed resource still succeeds. `Usage` is the tool for that.

**"What about `--cascade=foreground` on the XR?"** It bypasses ordering.
Kubernetes deletes dependents as soon as the owner has a deletion timestamp.
Known, documented, not fixed.

**"Who writes the graph?"** A function. The demo uses a test function built for
this; the intent is that existing functions adopt the field, and functions that
don't set it keep working unchanged.

## A caveat to be honest about

`up.sh` raises `--circuit-breaker-burst` far above its default. Realtime
compositions trip a per-XR circuit breaker on a burst of watch events, after
which events are let through every 30s — sensible for a runaway XR, but a
four-level graph trips it and later waves then take about a minute each. That
would be minutes of dead air. It changes nothing about ordering itself, but if
someone runs this without the flag and sees it crawl, this is why.

## Tearing down

```shell
./down.sh
```
