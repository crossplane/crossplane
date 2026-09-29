# Trying Out Composed Resource Ordering

A guide to exercising the composed resource ordering prototype in a local kind
cluster.

This is a prototype, open as [crossplane#7842][pr], with the design in
[crossplane#7841][design]. It isn't merged, isn't proposed for merge as it
stands, and the protocol shape may still change.

[pr]: https://github.com/crossplane/crossplane/pull/7842
[design]: https://github.com/crossplane/crossplane/pull/7841

## What it does

Composition Functions can declare that one composed resource depends on
another. Crossplane then sequences the resources it creates and deletes:
nothing is created until what it depends on is ready, and nothing is deleted
until everything depending on it is gone. Updates are not gated - once a
resource exists it keeps reconciling, so a dependency that goes un-ready can't
freeze it.

Crossplane records the graph on the XR, in `spec.crossplane.resourceRefs`, and
reports what it is holding back in `status.crossplane.pendingResources`. When
the XR itself is deleted, Crossplane tears it down in reverse order from the
recorded graph, without running the function pipeline.

The feature is behind an alpha flag, `--enable-composed-resource-ordering`.
With the flag off, Crossplane neither advertises the capability nor honors any
dependencies a function returns.

## What you need

* Docker, `kind`, `kubectl` and `helm`.
* [crossplane-graph][graph], which prints the graph off an XR. It needs Go
  1.26 to install.
* A checkout of the prototype, for the test fixtures:

  ```shell
  gh pr checkout 7842 --repo crossplane/crossplane
  ```

Everything below runs against a throwaway cluster and leaves nothing behind on
your machine except a kind cluster.

If you want to *show* this to someone rather than explore it yourself, the
`demo/` directory beside this file sets up a cluster with one script, carries a
run-of-show, and has `crossplane-graph-demo.sh`, which watches the graph while resources
are deleted and recreated.

## Set up a cluster

Create the cluster, and install a release of the prototype from ghcr. The
release is public, and its chart defaults to the prototype's image with
ordering turned on, so nothing needs building:

```shell
kind create cluster --name xp-ordering
helm install crossplane oci://ghcr.io/stevendborrelli/charts/crossplane \
  --version 2.5.0-ordering.2 \
  -n crossplane-system --create-namespace --wait
```

Confirm the flag took:

```shell
kubectl -n crossplane-system logs deploy/crossplane | grep "Alpha feature enabled"
```

The chart also raises `--circuit-breaker-burst` to 100000. Ordering converges
over one reconcile per dependency level, and at the default burst the realtime
compositions circuit breaker opens partway through a deep graph, after which
each wave waits for a periodic probe. See Limitations.

### Building Crossplane from the branch instead

To run the checkout you have rather than the release, build its image with Nix
and load it into the cluster:

```shell
image="$(nix run .#stream-image | docker load | sed -n 's/^Loaded image: //p')"
kind load docker-image "${image}" --name xp-ordering

helm install crossplane ./cluster/charts/crossplane \
  -n crossplane-system --create-namespace --wait \
  --set image.repository="${image%:*}" --set image.tag="${image#*:}" \
  --set image.pullPolicy=Never \
  --set 'args={--enable-composed-resource-ordering,--circuit-breaker-burst=100000}'
```

`./nix.sh run .#stream-image` does the same without installing Nix. The image
tag ends in the commit it was built from; check it matches the branch before
going further, because a stale image is the most confusing possible failure.

`scale/up.sh` beside this file does all of this, plus the function, provider
and a scale-test XRD.

## Install the function and provider

The test function declares ordering constraints. Few published functions can,
because the protocol field is new, so this one is built from
`test/e2e/functions/ordering` and published for testing:

```shell
M=test/e2e/manifests/apiextensions/composition/ordering

kubectl apply -f $M/setup/functions.yaml
```

`provider-nop` provides resources whose readiness *and deletion* you can
delay, which is what makes ordering observable rather than instantaneous.
Delayed deletion shipped in provider-nop v0.6.0. Apply its runtime config
first, since the provider references it:

```shell
kubectl apply -f $M/setup/runtimeconfig.yaml
kubectl apply -f $M/setup/provider.yaml

kubectl wait --for=condition=Healthy --timeout=3m \
  function/function-ordering provider/provider-nop-ordering
```

The runtime config sets `--poll=1s`, and it matters more than it looks.
provider-nop flips a NopResource's conditions, and finishes its deletion, when
it next *polls* the resource - not when the configured duration elapses. At the
default 10s poll a `readyAfter` of 1s still costs up to 10s per wave.

Then the XRD:

```shell
kubectl apply -f $M/setup/definition.yaml
```

## Install crossplane-graph

[crossplane-graph][graph] prints the graph an XR carries, grouped into the
waves Crossplane creates it in, with each composed resource's state and what
the graph is holding back:

```shell
go install github.com/stevendborrelli/crossplane-graph@latest
```

[graph]: https://github.com/stevendborrelli/crossplane-graph

## The scenarios

Each is a standalone Composition under `$M` - apply one, then apply the XR in
`$M/xr.yaml`.

| Fixture | What it shows |
| --- | --- |
| `create/composition-nested.yaml` | A four-level graph with a diamond, fan-out and independent branches, created a level at a time. Start here. |
| `delete/composition.yaml`, `delete/composition-nested.yaml` | Resources dropped from the pipeline while the XR stays up, deleted in reverse order. |
| `teardown/composition.yaml` | A chain whose resources each take 5s to delete, for watching the XR itself torn down a wave at a time. |
| `teardown/composition-blocked.yaml` | A resource that refuses to delete, holding teardown until someone intervenes. |
| `cbd/composition-before.yaml`, then `cbd/composition-after.yaml` | A create-before-destroy replacement: the new resource exists before the old one goes. |
| `contradiction/composition-before.yaml`, then `contradiction/composition.yaml` | A graph that contradicts desired state, reported as deadlocked rather than left to stall. |
| `required/composition.yaml`, with `environmentconfig.yaml` | A resource waiting on an EnvironmentConfig the XR requires but doesn't compose. |

The nested graph is the most informative. In one terminal:

```shell
watch -c -n1 crossplane-graph xordering/ordered -n default --color always
```

In another:

```shell
kubectl apply -f $M/create/composition-nested.yaml
kubectl apply -f $M/xr.yaml
```

Resources move from `blocked` to `creating` to `ready` a wave at a time. Each
blocked resource says what it is waiting for. Resources with no dependencies
are never held back.

Afterwards, the creation timestamps make the waves exact:

```shell
kubectl -n default get nopresources.nop.crossplane.io \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns='CREATED:.metadata.creationTimestamp,RESOURCE:.metadata.annotations.crossplane\.io/composition-resource-name'
```

Only fixtures that set `readyAfter` or `deleteAfter` compose `NopResource`s.
The others compose ConfigMaps, which are ready as soon as they exist. For
those, swap `nopresources.nop.crossplane.io` for `configmaps`.

Then delete the XR and watch the waves run the other way:

```shell
kubectl -n default delete xordering ordered --wait=false
```

The nested fixture's resources delete instantly, so teardown is over before
there is much to see. `teardown/composition.yaml` is the one to watch: its
three resources take 5s each to delete, so the leaf goes first while the other
two are `held`, and the XR is gone about 15s after the delete.

## Reading the results

**What is held back, and why, is in the XR's status.**
`status.crossplane.pendingResources` lists every composed resource ordering
isn't creating or deleting yet:

```shell
kubectl -n default get xordering ordered -o jsonpath='{range .status.crossplane.pendingResources[*]}{.operation}{" "}{.resourceName}{": "}{.reason}{"\n"}{end}'
```

```text
Create app: waiting for [instance] to be ready
Create backup: waiting for [database] to be ready
Create instance: waiting for [sg subnet-b] to be ready
```

Entries are sorted by composition resource name, so an unchanged situation
produces an unchanged status. During teardown the same field shows what is
held back from deletion:

```text
Delete first (ordered-73834cd42df2): waiting for [second third] to be deleted
Delete second (ordered-dfcdbc436d9c): waiting for [third] to be deleted
```

A pending creation carries the dependencies it waits on, and no object name,
because it doesn't exist yet. A pending deletion carries the object's name, and
its reason names the dependents it waits on. `deadlocked: true` marks the cases
waiting cannot resolve. The field is empty, and removed, once nothing is held
back. It's never read to make a decision, so losing it costs visibility until
the next reconcile and nothing else.

**The graph itself is on the XR.** Each entry in
`spec.crossplane.resourceRefs` records the composition resource name it
corresponds to and the names it depends on:

```shell
kubectl -n default get xordering ordered -o jsonpath='{range .spec.crossplane.resourceRefs[*]}{.resourceName}{" <- "}{.dependsOn}{"\n"}{end}'
```

This is what Crossplane reads on teardown, when no function runs, so it is the
thing to check first if teardown orders itself oddly. A resource held back
from creation has no entry here - a reference to an object that was never
created reads as an error to anything that follows references, including
`crossplane resource trace`. It appears in `pendingResources` instead.

Legacy v1 XRs are ordered too. They keep their references at
`spec.resourceRefs` rather than `spec.crossplane.resourceRefs`, and don't
report `pendingResources`.

**Events summarize each reconcile.** Ordering emits one event per reconcile,
not one per resource, because a wide graph can hold back a dozen resources at
once:

```shell
kubectl -n default describe xordering ordered | sed -n '/Events:/,$p'
```

```text
Ordering is holding back 2 composed resource(s): second waiting for [first] to
be ready; third waiting for [second] to be ready. Function pipeline took 12ms.
```

The message names up to five resources, then says how many more there are.
`pendingResources` always has the full list. The pipeline duration separates a
slow pipeline from a reconcile that wasn't triggered: if the gap between two
events is far larger than the duration, the delay is in Crossplane being woken
up, not in doing the work.

**Events undercount waves.** Kubernetes collapses repeats of an identical
message into one entry with a bumped count, so two waves that hold back the
same resources for the same reasons look like one event seen twice. Timestamps
on the resources themselves are the reliable record.

**The XR tells you when it's incomplete.** While resources are held back the XR
reports `Synced=False` with `Unsynced resources: ...`. That's deliberate - an
XR shouldn't claim to be synced while composition is knowingly unfinished.

**A contradiction gets its own warning.** Waiting is summarized; a graph that
can't be satisfied by waiting is not. If the pipeline wants a resource deleted
while something it still wants depends on it, or an edge names a required
resource that matched nothing, the XR gets a `Warning`, and the resource is
marked `deadlocked: true` in `pendingResources`. A required resource dependency
that names a namespaced resource without its namespace lands here too.

**A NopResource says how long its deletion has left.** Deletion progress is on
a condition of its own, because the managed reconciler overwrites `Ready` on
every pass:

```text
Type: Deletion  Reason: DeletionPending
Message: External resource will be deleted in 3s (deleteAfter: 5s)
```

Don't read the repeated "Successfully requested deletion of external resource"
event as a bug. The managed reconciler calls `Delete` once per reconcile for as
long as the external resource exists, so every slow delete produces it.

**crossplane-graph's readiness is a best-effort read.** It judges each composed
resource by its own conditions, where Crossplane uses the function pipeline's
verdict, which isn't persisted. The two can briefly disagree - a dependency
shown ready while its dependent is still blocked on it - until the next
reconcile.

## Writing a function that declares dependencies

A function returns dependencies in `RunFunctionResponse.dependencies`, and
should check for `CAPABILITY_DEPENDENCIES` in the request before relying on
them. An older Crossplane accepts the field without enforcing it.

* **Python.** [function-sdk-python#241][sdk-python] adds
  `response.add_dependency` and friends, and a `reference` module that records
  a dependency when a field is filled in from another resource.
* **Go.** Use the proto directly, as `test/e2e/functions/ordering/main.go`
  does. There is no SDK helper yet.
* **Without changing your functions.** A single function at the end of the
  pipeline can declare the graph for everything before it.
  [function-ordering][fn-ordering] reads `function-sequencer`'s rules and
  emits them as dependencies. It's published as
  `ghcr.io/stevendborrelli/function-ordering:v0.7.0-ordering.1`.

[sdk-python]: https://github.com/crossplane/function-sdk-python/pull/241
[fn-ordering]: https://github.com/stevendborrelli/function-ordering

To see the feature under a real platform, `notes-remote-e2e-plan.md` runs
[Modelplane's][modelplane] end-to-end suite against the prototype. Its serving
stack is ordered by dependencies rather than `Usage`s.

[modelplane]: https://github.com/stevendborrelli/modelplane/tree/composed-resource-ordering

## Rendering a Composition locally

`crossplane composition render` runs Crossplane's own composer, so it can
order composed resources too. It needs two things this prototype adds: a
Crossplane image that knows `--enable-composed-resource-ordering`, which the
release does from `v2.5.0-ordering.2`, and a CLI that passes the flag, which
is on the `composed-resource-ordering` branch of [stevendborrelli/cli][cli]:

```shell
git clone -b composed-resource-ordering https://github.com/stevendborrelli/cli.git
(cd cli && go build -o /tmp/crossplane ./cmd/crossplane)

/tmp/crossplane composition render $M/xr.yaml $M/create/composition-nested.yaml $M/setup/functions.yaml \
  --xrd $M/setup/definition.yaml \
  --crossplane-image ghcr.io/stevendborrelli/crossplane:v2.5.0-ordering.2 \
  --enable-composed-resource-ordering
```

Render runs one reconcile, so it shows the first wave: `vpc` and
`standalone` are rendered, and the XR's `status.crossplane.pendingResources`
lists the other eight with what each waits for. Without the flag all ten
render at once, because render then neither advertises dependencies nor
honors them - which is how a Composition that relies on ordering used to
render, whatever it did on a cluster.

`v2.5.0-ordering.1` predates the flag, and the render fails with
`unknown flag`.

[cli]: https://github.com/stevendborrelli/cli/tree/composed-resource-ordering

## Limitations

* **A composed resource that will not delete blocks teardown indefinitely.**
  That is deliberate: abandoning the order and cascading would delete the rest
  out of order. The XR reports what it is waiting on and waits for someone to
  intervene. `teardown/composition-blocked.yaml` shows it.
* **`kubectl delete --cascade=foreground` bypasses ordering**, because
  Kubernetes starts deleting dependents as soon as the owner has a deletion
  timestamp.
* **Nothing stops an out-of-band delete.** The graph constrains Crossplane's
  own calls, not anyone else's. `kubectl delete` on a composed resource
  succeeds even while something depends on it, and Crossplane recreates it. A
  `Usage` would block the delete.
* **Deep graphs need the circuit breaker's burst raised.** At the shipped
  default of 100, an eight-deep chain opened the realtime compositions watch
  circuit breaker after four waves, and each remaining wave took about a
  minute. The released chart raises the burst; a Crossplane you install
  another way needs `--circuit-breaker-burst` set, or the XR reports
  `Responsive=False` with `WatchCircuitOpen` and later waves crawl.
  `notes-circuit-breaker-scale-findings.md` has the measurements.
* **The test function is pinned to this branch's protocol.** It's compiled
  against `proto/fn/v1` here. If the `Dependency` messages change, republish it
  with `test/e2e/functions/ordering/build.sh` and bump the tag in
  `setup/functions.yaml`, or tests fail confusingly.
* **A cycle or a fatal graph error leaves `Ready` alone.** Crossplane reports
  `Synced=False` but readiness reflects the composed resources it last
  observed, so an XR can read `Ready=True` while composition is halted. That's
  existing Crossplane behavior for any composition error, not specific to
  ordering, but it's confusing the first time you see it.

## The automated tests

```shell
./nix.sh run .#e2e -- --test-suite=composed-resource-ordering
```

The suite installs Crossplane with the feature flag set, and pins the function
and provider by tag, so a change to either needs republishing before the tests
see it. Each test is built around an assertion that composition *without* a
graph would fail, so they demonstrate ordering rather than merely exercising
it.

| Test | The assertion that matters |
| --- | --- |
| `CreatesInWaves` | Only the roots exist before the level below them is ready, over a four-level graph with a diamond, fan-out and parallel branches. Unordered composition creates all ten at once. |
| `TeardownIsOrdered` | Only the leaf carries a deletion timestamp while what it depends on is untouched. A cascade stamps all three at once. |
| `TeardownBlocks` | A resource that refuses to delete holds the XR indefinitely and says so; clearing the cause lets teardown resume where it stopped. |
| `CreateBeforeDestroy` | A replacement and its predecessor exist at the same time, which a symmetric edge would never allow. |
| `TeardownSurvivesRestart` | Teardown continues in order after Crossplane is restarted mid-way. The replacement process never ran the pipeline for that XR, so the order can only have come from `spec.crossplane.resourceRefs`. |
| `TeardownIsOrderedOnLegacyXR` | A legacy v1 XR tears down a level at a time too, reading the graph from `spec.resourceRefs`. If teardown ever read only the modern path, it would find no graph and cascade, which looks exactly like finishing. |

Required resources and graph contradictions have fixtures but no end-to-end
test; both are covered by unit tests.

The whole suite takes about seven minutes on an 8 vCPU machine, including
building Crossplane. `CreatesInWaves` is most of it: the suite leaves the
circuit breaker at its default burst, so it opens partway through the
four-level graph and later waves wait for its periodic probe.

## Rebuilding the function

```shell
./test/e2e/functions/ordering/build.sh                          # build only
./test/e2e/functions/ordering/build.sh <registry>/<repo>:<tag>  # build and push
```

It cross-compiles both architectures and produces a multi-arch package.

## Tearing down

```shell
kind delete cluster --name xp-ordering
```
