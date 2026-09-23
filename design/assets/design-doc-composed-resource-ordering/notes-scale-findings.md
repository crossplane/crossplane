# Ordering at scale: what paced each run

Measurements on a 8-vCPU GCE VM running kind, with Crossplane built from this
branch and composed resources served by provider-nop. The harness is in
`scale/`: `up.sh` builds the cluster, `run.py` composes a graph of a given
shape and times creation and teardown, `limits.sh` changes what the cluster is
allowed to use.

Three things paced runs, and only one of them was the ordering.

## 1. The provider's reconcile rate, not the graph

The first sweep said ordering was catastrophically slow: a 100-resource fanout
took 200s to create against 2s unordered, and a 250-resource fanout took 63s,
which is not even monotonic in the size of the graph.

None of it was the ordering. Per-wave attribution showed the root resource
taking 480s to report Ready, with its dependents created 1s later - so the
graph was releasing each wave the moment it was allowed to, and waiting on the
provider. A direct probe with Crossplane out of the loop confirmed it: a
NopResource applied by hand never became Ready within 120s. After
`kubectl rollout restart` of provider-nop, the same probe was Ready in 3s.

The cause is `--poll=1s` on N resources outrunning controller-runtime's default
rate limiter. The provider's queue never drains, and the backlog is cumulative,
so each run starts further behind than the last. Raising the provider's
concurrency with `--max-reconcile-rate=100` (`./limits.sh big`) fixes it:

| Run | Before | After |
| --- | --- | --- |
| fanout-100, creation | 483s | 5s |
| fanout-100, root Ready | +480s | +2s (exactly its `readyAfter`) |
| fanout-100, ordering lag | - | 0s |

Peak Crossplane CPU during that run was 310m, under the chart's 500m limit, so
CPU was never what bound it.

The lesson for anyone repeating this: measure the provider before believing
anything about the engine. A saturated provider and a slow graph look identical
from the XR.

## 2. Teardown hammering a resource that is already deleting

`chain-50` wedged: 48 of 50 composed resources sat for half an hour with the XR
reporting

> Waiting for 1 composed resource(s) already asked to delete: r0047 (deleting
> for 3m41s). 48 remaining; teardown cannot continue until they are gone.

The resource had a deletion timestamp and provider-nop's finalizer, and
provider-nop was reconciling it - and failing, every time:

```
Cannot remove managed resource finalizer ... error: cannot update object:
Operation cannot be fulfilled on nopresources.nop.crossplane.io
"chain-50-...-22af649ce01c": the object has been modified; please apply your
changes to the latest version and try again
```

This is a defect in this branch, not in the provider. `Reconciler.teardown`
nominates the same wave on every reconcile until it is gone, and handed the
whole wave to `GarbageCollectComposedResources` each time. That collector
strips the composition labels with an `Update` before issuing the `Delete`, so
every XR reconcile wrote to a resource that was already deleting - while the
provider was doing its own read-modify-write to remove its finalizer. Both
sides conflicted, both sides retried, and the deletion never completed. A
teardown that keeps writing to a deleting resource is a teardown that prevents
it from finishing.

Chains show it worst because each wave is a single resource being written to by
every reconcile. Fanouts mostly survive it: the leaves go in one wide wave and
the conflict window is short.

The fix is to hand the collector only the part of the wave that has not been
asked yet, skipping anything with a deletion timestamp. Pinned by
`TestTeardown/SkipsResourcesAlreadyAsked` and
`TestTeardown/AsksTheRestOfTheWave`. The wedged XR above drained on its own,
completely, within minutes of the fixed build rolling out - with no other
change. Re-running the same shape confirms it:

| chain-50 | Before | After |
| --- | --- | --- |
| Creation | 160s, with a 112s stall mid-chain | 158s |
| Teardown | never completed in 900s | 55s |

158s is the floor for that shape rather than a cost of ordering: 50 links at
`readyAfter=2s` cannot finish faster than ~100s, and each wave adds 1-2s.

## 3. What a dense graph costs on the XR

Two of the dense runs failed before they started, and both were the harness
rather than Crossplane. 500 resources in 5 fully connected levels is 40,000
edges, which exceeded the API server's 3MiB request limit; shrinking it to
6,400 edges then hit `metadata.annotations: Too long: may not be more than
262144 bytes`, because `kubectl apply` keeps a copy of the manifest it sent in
an annotation. Server-side apply avoids the second and makes the first a long
way off.

The underlying question is real, though, because the graph is persisted on the
XR: `dependsOn` is recorded on every composed resource reference, so the XR
grows with the number of *edges*, not the number of resources. Measured on a
live XR partway through `layered-500`:

| | |
| --- | --- |
| Composed resource references | 400 |
| Edges recorded across them | 7,600 |
| Whole XR | 132KB |
| The `dependsOn` entries alone | 69KB |

So slightly over half the XR is the graph, at about 9 bytes per edge with
five-character resource names. Real names run 20-40 characters, making 30-50
bytes an edge the more honest figure.

That is comfortably inside etcd's 1.5MiB object limit, and the run producing it
was deliberately unrealistic: 32 dependencies per resource, every resource in a
level depending on every resource in the level below. Real compositions are
shallow - a thousand resources hanging off one VPC is 999 edges, and most
resources depend on a handful of things.

This is a storage cost and only a storage cost. Density turned out not to cost
convergence time at all (finding 5), so the two should not be run together:
edges make the XR bigger, and they do not make it slower.

## 4. How much of a run is the provider?

Every number above is a NopResource, which goes through a provider. A
ConfigMap is composed by core directly with nothing else in the loop, and
`function-ordering` already composes one when no `readyAfter` is asked for, so
the same graph can be run both ways. The difference is the provider's share:

| chain-50 | Creation |
| --- | --- |
| NopResources | 158s |
| ConfigMaps | 10s |

At depth the provider is essentially the whole measurement. It is not a
constant overhead either - it is paid once per wave, so it grows with depth
and vanishes with width. Any run that means to say something about the engine
should be a ConfigMap run, and `--kind configmap` is now the way to get one.

## 5. What the graph costs core

`run.py` reads `controller_runtime_reconcile_time_seconds` for the XR's
controller either side of a run, so the cost lands as reconciles and mean
reconcile time rather than as wall clock. The same shape with `--unordered` is
the control.

| Run | Creation | Reconciles | Core CPU | Mean reconcile |
| --- | --- | --- | --- | --- |
| fanout-500 ordered | 6s | 2 | 3.7s | 1842ms |
| fanout-500 unordered | 5s | 1 | 3.4s | 3389ms |
| fanout-100 ordered | 3s | 6 | 2.9s | 478ms |
| fanout-100 unordered | 3s | 5 | 2.7s | 550ms |
| chain-100 ordered | 32s | 104 | 31.5s | 303ms |
| chain-100 unordered | 3s | 5 | 2.7s | 547ms |
| chain-50 ordered | 10s | 54 | 9.8s | 182ms |
| chain-50 unordered | 3s | 10 | 3.0s | 295ms |
| fanout-250 ordered | 3s | 2 | 1.8s | 903ms |
| fanout-250 unordered | 3s | 2 | 3.0s | 1525ms |
| fanout-1000 ordered | 9s | 2 | 7.2s | 3585ms |
| fanout-1000 unordered | 9s | 4 | 11.5s | 2887ms |
| layered-200, 6400 edges, 5 waves | 5s | 12 | 4.6s | 382ms |
| layered-200 unordered | 3s | 4 | 3.2s | 801ms |
| layered-500, 9600 edges, 25 waves | 43s | 31 | 41.8s | 1348ms |
| layered-500 unordered | 6s | 5 | 7.5s | 1500ms |

Three things, and none of them is the graph being expensive.

Width is free. Ordering a 1000-wide fanout costs no extra reconciles at all
over composing the same 1000 resources at once, and in these runs cost less
core CPU than the unordered control did.

Density is nearly free, which corrects an earlier claim in these notes.
`layered-200` is 6,400 edges over 200 resources - 32 dependencies each, far
denser than anything real - and converges in 5s against a 3s unordered
control, for 12 reconciles against 4.

Depth is what costs, one pass per wave, which is not an implementation choice:
a hundred waves cannot be released in fewer than a hundred passes. A pass runs
from 180ms to 1.3s here, scaling with how many resources the wave applies
rather than with how many edges the graph has. `layered-500` is the clearest
case: 43s for 25 waves of 20, against `chain-100`'s 32s for 100 waves of 1.
Its 9,600 edges cost nothing; its 25 levels cost everything.

The mean reconcile is *lower* ordered than unordered in every pair, because an
ordered pass applies a slice of the resources rather than all of them. So the
graph work does not register as a per-pass cost at this scale, which is what
the microbenchmarks predict - they put a pass at about a millisecond. What
ordering costs is passes, not the cost of a pass.

## 6. Against function-sequencer

Creation only. `function-sequencer` orders deletion with Usages, which this
proposal does not propose to replace, so there is nothing to compare there.

The pipeline is `function-ordering` composing the resources and declaring no
edges, then `function-sequencer` with the same graph flattened into the
ordered list it takes.

With ConfigMaps, which are ready the instant they exist:

| chain-20 | Creation | Reconciles | Core CPU |
| --- | --- | --- | --- |
| graph | 3s | 26 | 2.9s |
| sequencer | 3s | 27 | 2.9s |

Indistinguishable, and that is the honest headline: **for creating a chain,
the graph is not faster than function-sequencer.** The case for it is not
creation throughput.

Where they diverge is width, and it only becomes visible once readiness costs
something. With NopResources at `readyAfter=2s`:

| 20 resources, `readyAfter=2s` | Creation | Reconciles |
| --- | --- | --- |
| chain, graph | 192s | 100 |
| chain, sequencer | 192s | 105 |
| fanout, graph | **3s** | 6 |
| fanout, sequencer | **192s** | 101 |

The sequencer takes exactly as long on a fanout as on a chain. A list can say
"after", and cannot say "these are unrelated", so flattening a graph into one
adds every constraint the graph deliberately left out: nineteen independent
leaves become nineteen serial steps. The chain rows are the control - where
the graph really is a list, the two mechanisms agree to within noise, which is
what makes the fanout rows worth believing.

This is the same expressiveness argument the design makes about templating
functions and about edges being data, measured rather than asserted.

## 7. Ordering does not converge against the default circuit breaker

Every measurement above raised `--circuit-breaker-burst` to 100000 to take the
realtime compositions watch circuit breaker out of the way. That was the right
call for measuring the engine and the wrong impression to leave, because
Crossplane ships with `burst=100`, `refill-rate=1/s`, a 5m cooldown and a 30s
half-open probe - and at those settings a chain does not finish.

ConfigMaps, ordered, with the breaker at its shipped defaults:

| chain | Breaker raised | Breaker at defaults |
| --- | --- | --- |
| 50 | 10s | 49/50, still going at 601s |
| 100 | 32s | 73/100, still going at 901s |

`chain-50` opened the breaker twice and dropped 133 of 339 events.
`chain-100` opened it five times and dropped 325, taking 24 half-open probes.
Neither converged; they were still making progress a wave at a time, at the
30s half-open interval, when the harness gave up.

The mechanism is that ordering converges over many reconciles by design - one
per wave - and each pass applies resources whose updates are themselves watch
events. A chain therefore generates events superlinearly in its depth while
spending exactly the budget the breaker meters. The breaker is doing what it
was built to do: an XR reconciling a hundred times in thirty seconds is the
runaway it exists to stop. It cannot currently tell that apart from a graph
converging normally.

This is a real and unresolved cost of the proposal, and it is worse than the
earlier note in `notes-comparison-function-controlled-deletion.md` suggested.
That note said deep graphs trip the breaker and each wave then takes about a
minute. The measurement says a 50-deep chain does not converge at all on a
stock Crossplane.

It bears on the earlier findings rather than invalidating them: those numbers
are what the engine costs, and remain the right answer to "what does the graph
cost". They are not what a user sees on a default install, and nothing above
should be quoted as if they were.

Wide graphs are unaffected - a fanout is two waves whatever its size - so this
is depth, like everything else that costs here. What it needs is a decision
about how the two features should interact: whether ordering-driven reconciles
should be metered at all, whether the breaker should count waves rather than
events, or whether enabling ordering should raise the burst.

## Results

NopResource runs, with `./limits.sh big` and provider-nop restarted
beforehand. `readyAfter=2s`, so 2s is the floor for any single wave - and
per finding 4, most of what these measure is the provider rather than the
engine. The ConfigMap runs in finding 5 are the ones to read for core's cost.

| Shape | Resources | Edges | Waves | Creation | Teardown | Peak Crossplane |
| --- | --- | --- | --- | --- | --- | --- |
| fanout | 100 | 99 | 2 | 5s | 63s | 310m / 66Mi |
| fanout | 250 | 249 | 2 | 5s | 25s | 265m / 76Mi |
| fanout | 500 | 499 | 2 | 8s | 47s | 298m / 106Mi |
| fanout | 1000 | 999 | 2 | 14s | 93s | 334m / 139Mi |
| chain | 50 | 49 | 50 | 158s | 55s | 241m / 82Mi |
| layered | 200 | 6400 | 5 | 82s | 43s | 467m / 113Mi |
| layered | 500 | 9600 | 25 | 895s | >900s | 694m / 151Mi |

Wide graphs are where ordering could plausibly serialize work that ought to
happen at once, and it does not: 1000 resources are released in 14s against a
2s floor, with per-wave attribution showing each wave starting within a second
of its dependencies going Ready. Growth is roughly linear and memory grows
gently. CPU never approached the 4-core limit, so none of these numbers
describe the limit rather than the engine.

Chains are dominated by their own depth, as they must be: 50 links at
`readyAfter=2s` cannot finish faster than 100s. The measured 158s is not
ordering's contribution, though - the same chain of ConfigMaps takes 10s, so
a wave costs core about 0.2s and the rest is the provider.

The layered rows look like density costing something, and they are not: run
against ConfigMaps the same two shapes take 5s and 43s rather than 82s and
895s. What is left after the provider is removed is explained by depth, not by
edges - see finding 5. The 694m CPU peak is likewise a measurement of the
provider era; ordered passes cost less CPU than unordered ones in every
ConfigMap pair.

Every NopResource row here is subject to the same correction. They are kept
because they are what was actually run, and because a composition of real
managed resources does have a provider in the loop - but they describe a
provider's reconcile rate, and none of them should be quoted as a cost of
ordering.
