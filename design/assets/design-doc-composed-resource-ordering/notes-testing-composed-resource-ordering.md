# Trying Out Composed Resource Ordering

A guide to exercising the composed resource ordering prototype in a local kind
cluster.

This is a prototype, open as [crossplane#7842][pr]. It isn't merged, isn't
proposed for merge as it stands, and the protocol shape may still change.

[pr]: https://github.com/crossplane/crossplane/pull/7842

## What it does

Composition Functions can declare that one composed resource depends on
another. Crossplane then sequences the resources it creates and deletes:
nothing is created until what it depends on is ready, and nothing is deleted
until everything depending on it is gone. Updates are not gated - once a
resource exists it keeps reconciling, so a dependency that goes un-ready can't
freeze it.

The feature is behind an alpha flag, `--enable-composed-resource-ordering`.
With the flag off, Crossplane neither advertises the capability nor honors any
dependencies a function returns.

## What you need

* Docker, `kind`, `kubectl`, `helm`, and Go 1.26.
* A checkout of the prototype, which `gh` will fetch for you:

  ```shell
  gh pr checkout 7842 --repo crossplane/crossplane
  ```

Everything below runs against a throwaway cluster and leaves nothing behind on
your machine except a kind cluster and two local images.

If you want to *show* this to someone rather than explore it yourself, the
`demo/` directory beside this file sets the same thing up with one script, and
carries a run-of-show.

## Set up a cluster

Create the cluster and install a released Crossplane, which gives you the CRDs,
RBAC and TLS secrets:

```shell
kind create cluster --name xp-ordering
kubectl create namespace crossplane-system
helm install crossplane ./cluster/charts/crossplane \
  -n crossplane-system \
  --set image.repository=crossplane/crossplane --set image.tag=v2.4.0 \
  --wait
```

Then replace the Crossplane image with one built from this branch, and turn the
feature on:

```shell
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/crossplane ./cmd/crossplane
# Use GOARCH=amd64 on an Intel or AMD machine.

# A directory of its own, not /tmp. The build context is sent to the daemon
# in full, so anything unreadable in a shared /tmp fails the build.
ctx="$(mktemp -d)"
cp /tmp/crossplane "${ctx}/crossplane"

cat > "${ctx}/Dockerfile" <<'DOCKERFILE'
FROM gcr.io/distroless/static-debian12:nonroot
COPY crossplane /usr/local/bin/crossplane
USER 65532
ENTRYPOINT ["crossplane"]
DOCKERFILE
docker build -t xp-ordering/crossplane:dev "${ctx}"
kind load docker-image xp-ordering/crossplane:dev --name xp-ordering

kubectl -n crossplane-system patch deploy crossplane --type=json -p '[
 {"op":"replace","path":"/spec/template/spec/containers/0/image","value":"xp-ordering/crossplane:dev"},
 {"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"},
 {"op":"replace","path":"/spec/template/spec/containers/0/args","value":["core","start","--debug","--enable-composed-resource-ordering"]}
]'
kubectl -n crossplane-system rollout status deploy/crossplane
```

Confirm the flag took:

```shell
kubectl -n crossplane-system logs deploy/crossplane | grep "Alpha feature enabled"
```

## Install the function and provider

The test function declares ordering constraints. No published function can -
the protocol field is new - so this one is built from
`test/e2e/functions/ordering` and published for testing:

```shell
kubectl apply -f test/e2e/manifests/apiextensions/composition/ordering/setup/functions.yaml
```

`provider-nop` provides resources whose readiness *and deletion* you can delay,
which is what makes ordering observable rather than instantaneous. Delayed
deletion is new - see
[provider-nop#27](https://github.com/crossplane-contrib/provider-nop/pull/27) -
so until that merges this needs a build from that branch:

```shell
kubectl apply -f test/e2e/manifests/apiextensions/composition/ordering/setup/provider.yaml

kubectl wait --for=condition=Healthy --timeout=3m function/function-ordering provider/provider-nop-ordering
```

That manifest also carries a `DeploymentRuntimeConfig` setting `--poll=1s`, and
it matters more than it looks. provider-nop flips a NopResource's conditions,
and finishes its deletion, when it next *polls* the resource - not when the
configured duration elapses. At the default 10s poll a `readyAfter` of 1s still
costs up to 10s per wave. Set it low or the delays you configure will be
rounding errors on top of the poll interval.

Then the XRD:

```shell
kubectl apply -f test/e2e/manifests/apiextensions/composition/ordering/setup/definition.yaml
```

## The scenarios

All of these live in
`test/e2e/manifests/apiextensions/composition/ordering`.

Each is a standalone Composition - apply one, then apply the XR.

* `setup/composition.yaml` - a linear chain of three resources.
* `setup/composition-nested.yaml` - a four-level graph with a diamond,
  fan-out and independent branches.
* `setup/composition-required.yaml` - a resource waiting on an
  EnvironmentConfig the XR requires but doesn't compose.
* `setup/composition-cbd-before.yaml`, then
  `setup/composition-cbd-after.yaml`, for a create-before-destroy replacement.
* `setup/composition-contradiction-before.yaml`, then
  `setup/composition-contradiction.yaml` - a graph that contradicts desired
  state, reported rather than left to stall.
* `delete/composition.yaml` and `delete/composition-nested.yaml` - ordered
  teardown while the XR is alive.
* `teardown/composition.yaml` - a chain of NopResources that each take 5s to
  delete, for watching the XR itself be torn down a wave at a time.

The nested graph is the most informative:

```shell
kubectl apply -f test/e2e/manifests/apiextensions/composition/ordering/setup/composition-nested.yaml
kubectl apply -f test/e2e/manifests/apiextensions/composition/ordering/xr.yaml
```

Watch resources appear a level at a time. Only the nested fixture sets
`readyAfter`, so only it composes `NopResource`s - the others compose
ConfigMaps, and this command comes back empty against them:

```shell
kubectl -n default get nopresources.nop.crossplane.io \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns='CREATED:.metadata.creationTimestamp,\
NAME:.metadata.annotations.crossplane\.io/composition-resource-name'
```

Roots first, then everything that depends only on them, and so on. Resources
with no dependencies are never held back.

For the fixtures that compose ConfigMaps instead, swap
`nopresources.nop.crossplane.io` for `configmaps` and read `.data.name`:

```shell
kubectl -n default get configmaps -l crossplane.io/composite=ordered \
  -o custom-columns='CREATED:.metadata.creationTimestamp,NAME:.data.name'
```

To see why something is waiting, read the XR's events:

```shell
kubectl -n default describe xordering ordered | sed -n '/Events:/,$p'
```

Ordering emits one event per reconcile, not one per resource, because a wide
graph can hold back a dozen resources at once and a dozen near-identical events
would bury everything else in the XR's event stream. The message names up to
five of them with their reasons, then the count of the rest:

```text
Ordering is holding back 2 composed resource(s): second waiting for [first] to
be ready; third waiting for [second] to be ready. Function pipeline took 12ms.
```

The pipeline duration is there to separate a slow pipeline from a reconcile
that wasn't triggered. Compare it to the gap between two consecutive events: if
that gap is far larger than the duration, the delay is in Crossplane being
woken up, not in doing the work.

## Reading the results

**The graph itself is readable on the XR.** Each entry in
`spec.crossplane.resourceRefs` records the composition resource name it
corresponds to and the names it depends on, so the whole graph comes back from
one `kubectl get`:

```shell
kubectl get xordering ordered -o jsonpath='{range .spec.crossplane.resourceRefs[*]}{.resourceName}{" <- "}{.dependsOn}{"\n"}{end}'
```

This is what core reads on teardown, when no function runs, so it is also the
thing to check first if teardown orders itself oddly. It is written on every
reconcile, before any composed resource is created.

**Timestamps are the evidence.** Resources at the same graph depth share a
creation timestamp; each level is separated by however long the level below
took to report ready.

**A NopResource says how long its deletion has left.** Deletion progress is on
a condition of its own, because the managed reconciler overwrites `Ready` on
every pass:

```
Type: Deletion  Reason: DeletionPending
Message: External resource will be deleted in 3s (deleteAfter: 5s)
```

Don't read the repeated "Successfully requested deletion of external resource"
event as a bug. The managed reconciler calls `Delete` once per reconcile for as
long as the external resource exists, so every slow delete produces it - a real
provider tearing down an RDS instance does the same.

**Events undercount waves.** Kubernetes collapses repeats of an identical
message into one entry with a bumped count and `LAST SEEN`, so two waves that
hold back the same resources for the same reasons look like one event seen
twice. Delete-side gating often produces no event at all, because a teardown
wave completes in well under a second. Timestamps on the resources themselves
are the reliable record.

**Blocked resources are absent from `crossplane resource trace`, not broken.**
A resource the graph is holding back from creation is deliberately left out of
the XR's `spec.resourceRefs` - referencing an object that was never created
makes the trace report it as missing, which reads as a failure rather than as
waiting. So the trace shows the graph filling in as resources are created.
A resource that already exists keeps its reference even while it's held back
from deletion, so ordered teardown is visible in the trace.

**A contradiction gets its own warning.** Waiting is summarized; a graph that
can't be satisfied by waiting is not. If the pipeline wants a resource deleted
while something it still wants depends on it, or an edge names a required
resource that matched nothing, the XR gets a `Warning` naming the resource and
saying the desired state and the dependencies contradict each other. A required
resource dependency that names a namespaced resource without its namespace
lands here too - it matches nothing, and the warning says so rather than
reporting an indefinite wait.

**The XR tells you when it's incomplete.** While resources are held back the XR
reports `Synced=False` with `Unsynced resources: ...`. That's deliberate - an
XR shouldn't claim to be synced while composition is knowingly unfinished.

**Timing isn't uniform.** The first wave lands at the delay configured in
`readyAfter`; later waves also wait on provider-nop's poll interval, so they
can take a minute. Judge ordering by sequence, never by elapsed time.

## Limitations

* **XR deletion is ordered, from the graph on the XR itself.** Crossplane used
  to drop its finalizer on seeing a deletion timestamp and let Kubernetes
  cascade. It now holds the finalizer and deletes composed resources leaves
  first, rebuilding the graph from `spec.crossplane.resourceRefs` rather than
  running the pipeline - so this does not depend on
  [#7242](https://github.com/crossplane/crossplane/pull/7242), which remains
  what is needed to do *work* during teardown.
* **A composed resource that will not delete blocks teardown indefinitely.**
  That is deliberate: abandoning the order and cascading would delete the rest
  out of order. The XR reports what it is waiting on and waits for someone to
  intervene. Set `deleteError` on a NopResource to try it.
* **`kubectl delete --cascade=foreground` bypasses ordering**, because
  Kubernetes starts deleting dependents as soon as the owner has a deletion
  timestamp.
* **The test function is pinned to this branch's protocol.** It's compiled
  against `proto/fn/v1` here. If the `Dependency` messages change, republish it
  with `test/e2e/functions/ordering/build.sh` or tests fail confusingly.
* **Nothing stops an out-of-band delete.** The graph constrains Crossplane's
  own calls, not anyone else's. `kubectl delete` on a composed resource
  succeeds even while something depends on it. A `Usage` would block it.
* **Deep graphs get slower part way through.** An eight-deep chain tripped the
  realtime compositions watch circuit breaker after four waves, and the
  remaining waves fell back to the half-open interval - roughly a minute each
  instead of ten seconds. Expect wall-clock times well above the sum of your
  `readyAfter` values, and don't read a slow later wave as a hang.
  `notes-circuit-breaker-scale-findings.md` has the measurements and what
  paces a degraded graph. `--circuit-breaker-half-open-interval` shortens it
  for testing; it isn't a fix.
* **A cycle or a fatal graph error leaves `Ready` alone.** Crossplane reports
  `Synced=False` but readiness reflects the composed resources it last
  observed, so an XR can read `Ready=True` while composition is halted. That's
  existing Crossplane behavior for any composition error, not specific to
  ordering, but it's confusing the first time you see it.
* **The core behaviors are automated; the rest is manual.** Five end-to-end
  tests cover creation ordering, teardown ordering, the blocking policy,
  create-before-destroy and restart resilience:

  ```shell
  ./nix.sh run .#e2e -- -test.run TestComposedResourceOrdering \
                        --test-suite=composed-resource-ordering
  ```

  Required resources and graph contradictions still have fixtures but no
  end-to-end test; both are covered by unit tests.

## The automated tests

Each one is built around an assertion that composition *without* a graph would
fail, so they demonstrate ordering rather than merely exercising it.

| Test | The assertion that matters |
| --- | --- |
| `CreatesInWaves` | Only the roots exist before the level below them is ready, over a four-level graph with a diamond, fan-out and parallel branches. Unordered composition creates all ten at once. |
| `TeardownIsOrdered` | Only the leaf carries a deletion timestamp while what it depends on is untouched. A cascade stamps all three at once. |
| `TeardownBlocks` | A resource that refuses to delete holds the XR indefinitely and says so; clearing the cause lets teardown resume where it stopped. |
| `CreateBeforeDestroy` | A replacement and its predecessor exist at the same time, which a symmetric edge would never allow. |
| `TeardownSurvivesRestart` | Teardown continues in order after Crossplane is restarted mid-way. The replacement process never ran the pipeline for that XR, so the order can only have come from `spec.crossplane.resourceRefs`. |

Two things about running them.

**They are slower than they look.** A four-level graph trips the realtime
compositions watch circuit breaker - the XR reports "Too many watch events" -
after which each remaining wave takes about a minute rather than seconds. The
creation test allows three minutes per level for that reason. See
`notes-circuit-breaker-scale-findings.md`.

**They pin published images.** The suite installs
`function-ordering` and a `provider-nop` build with timed deletes, both by tag.
Change the function and you must republish it and bump the tag in
`setup/functions.yaml`, or the cluster keeps running the old one and the
failures make no sense. That is not hypothetical: it cost several confusing
runs.

## Rebuilding the function

```shell
./test/e2e/functions/ordering/build.sh                       # build only
./test/e2e/functions/ordering/build.sh <registry>/<repo>:<tag>  # build and push
```

It cross-compiles both architectures and produces a multi-arch package.

## Tearing down

```shell
kind delete cluster --name xp-ordering
```
