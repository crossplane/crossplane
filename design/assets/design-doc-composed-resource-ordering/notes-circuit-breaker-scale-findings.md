# Composed Resource Ordering: Scale Testing and the Circuit Breaker

Findings from testing composed resource ordering against Crossplane's watch
circuit breaker.

Ordering converges over many reconciles by design. The realtime compositions
circuit breaker rate-limits watch-driven reconciles. The question was whether
those two are compatible, and if not, whether tuning can make them so.

**In one line:** the breaker opens on graphs as small as ten resources, but
what that costs is set by a single knob, and turning it down removes almost all
of the penalty without weakening the protection.

## Hypotheses

1. The degraded wave time after the breaker opens is set either by the poll
   interval or by the breaker's half-open probe. Which one was unknown, and
   determines whether this is tunable at all.
2. Ordering either generates extra watch traffic, or merely spreads a fixed
   amount of it over a longer window. If the former, the fix belongs in the
   reconciler; if the latter, in the breaker's sizing.
3. Time to converge, and the point at which the breaker opens, scale with the
   size of the composition.

Hypotheses 1 and 3 were tested. Hypothesis 2 was not - see "Still open".

## Test setup

**Cluster.** kind, single node, 8 CPU / 8 GB. Azure was considered and rejected
for this round: nothing in these experiments touches a cloud API, so the load
is custom resources and Crossplane's reconcile loop. If it is repeated on AKS,
note that clusters default to the Free control plane tier, which does not
guarantee API server capacity - a throttled control plane would look exactly
like the breaker opening early.

**Crossplane.** Built from the `composed-resource-ordering` branch, running
in-cluster with `--enable-composed-resource-ordering` and realtime compositions
on. Defaults unless a run says otherwise.

**Composed resources.** `provider-nop` v0.5.0, using `conditionAfter` so
readiness happens on a controlled delay rather than instantly. This matters:
with ConfigMaps every wave completes inside a second and nothing is
observable.

**Function.** `index.docker.io/steve/function-ordering:dependency-test.1`,
installed through the package manager. Two graph shapes:

* *Chain* - eight resources, each waiting on all its predecessors. Depth 8,
  28 edges. Maximally serial, so wave boundaries are unambiguous.
* *Fan-out* - one root and N-1 dependents, all waiting only on the root.
  Depth 2, N-1 edges. Isolates resource count from depth.

**Method.** A fresh XR per run, because breaker state is per target and lives
for 24 hours - a reused name inherits the previous run's state. Wave boundaries
come from composed resource creation timestamps; breaker state from the XR's
`Responsive` condition; counters from `:8080/metrics`:

```text
circuit_breaker_opens_total{controller}
circuit_breaker_events_total{controller,result}   # Allowed | Dropped | HalfOpenAllowed
```

Note the metric names carry no `crossplane_` prefix.

**What can be tuned.** Four flags in `cmd/crossplane/core/core.go`. An Update
event costs two tokens, because controller-runtime invokes the MapFunc once
with the old object and once with the new, so the defaults mean a 50-Update
burst and one Update every two seconds sustained.

| Flag | Default |
| --- | --- |
| `--circuit-breaker-burst` | `100.0` |
| `--circuit-breaker-refill-rate` | `1.0` |
| `--circuit-breaker-cooldown` | `5m` |
| `--circuit-breaker-half-open-interval` | `30s` |

The fourth was not previously exposed; it was added for this work.

## Results

### What paces a convergence once the breaker is open

Chain of 8, `readyAfter: 2s`, one run per arm.

| Arm | Setting | Total | Waves while closed | Waves while open |
| --- | --- | --- | --- | --- |
| A | defaults | 120s | 10-11s | **57s** |
| B | `--poll-interval=5m` | 167s | 10-11s | **56s** |
| C | `--circuit-breaker-half-open-interval=5s` | 76s | 10-11s | **none** |

### How convergence scales

Fan-out, one XR, depth 2, defaults:

| Resources | Converged | Per resource | Breaker |
| --- | --- | --- | --- |
| 10 | 12s | 1.2s | opened once |
| 25 | 19s | 0.8s | stayed closed |
| 50 | 32s | 0.6s | stayed closed |
| 100 | 57s | 0.6s | opened once |

Depth against width, and one XR against many, at the same resource count:

| Shape | Resources | Converged | Per resource |
| --- | --- | --- | --- |
| Fan-out, 1 XR | 100 | 57s | 0.6s |
| Chain, 1 XR | 8 | 120s | **15s** |
| Fan-out, 10 XRs x 10 | 100 | **144s** | 1.4s |

## Analysis

**The half-open probe paces a degraded convergence, not the poll interval.** A
five-fold change to `--poll-interval` moved the degraded wave time by one
second. Dropping the half-open interval from 30s to 5s removed the degradation
outright: all eight waves at 10-11s, and a total of 76s against a floor of
about 80s for eight waves at a two second readiness delay. Hypothesis 1 is
answered, and the answer is the knob that wasn't exposed.

The default 30s produced 57s waves, near enough to twice the interval to
suggest each wave needs two probes - one to observe that a dependency became
ready, another to act on it. If that holds, the effective penalty is always
double whatever the interval is set to. Unconfirmed.

**Tuning does not stop the breaker opening; it makes opening cheap.** The
breaker opened in all three arms of the first experiment, including the fast
one. That is a more attractive shape than raising the budget, which would buy
throughput by weakening the guard. A low budget with a short probe keeps the
protection against hot loops and removes most of its cost to a slow
convergence.

**Depth costs roughly 25 times more per resource than width.** A resource added
to a wide graph costs about 0.6s; one added to a chain costs 15s, because it
adds a whole reconcile round trip. Convergence tracks the longest path rather
than the resource count, which is the expected shape for a design that releases
one level per reconcile - but the constant is large.

**Width is close to free.** Ten times the resources cost under five times the
time, and per-resource cost falls as the graph widens. Nothing here suggests
width is a scaling problem.

**Splitting work across XRs costs 2.5x.** The same hundred resources took 57s
as one XR and 144s as ten. The breaker is per target, so ten XRs have ten
separate budgets - this is not throttling, it is contention for the
reconciler's worker pool and the API server. In that run all ten roots appeared
immediately, then nothing moved for about 50 seconds, then the remainder
arrived at once. That stall is unexplained and is the most interesting result
here.

**Resource count alone does not predict when the breaker opens.** It opened at
10 resources and stayed closed at 25 and 50. Whatever trips it is not simply
size, and one run per configuration is too thin to say what it is.

## Still open

* **Hypothesis 2 was never tested.** Running the same graph with ordering on
  and off, and comparing `circuit_breaker_events_total`, would say whether
  ordering generates extra watch traffic or only spreads it. That determines
  whether the fix belongs in the reconciler or the breaker, and it is asserted
  both ways in the issue draft.
* **The 50 second stall** in the multi-XR run.
* **What a tuned setting costs.** Any setting that helps ordering also weakens
  what the breaker exists for. Measuring that needs a reproducer of the hot
  loop the breaker was built against, which does not exist yet. Until it does,
  no tuning recommendation is safe.
* **Repeats.** One run per configuration, on kind, with wall-clock timings that
  varied by a few seconds between identical early waves. Arm B finished slower
  than arm A despite the same degraded wave time, because its breaker opened
  earlier - real or variance, unknown. Three runs per configuration before any
  of these numbers inform a decision.

## Where this leaves the proposal

Nothing here blocks composed resource ordering, and nothing here is fixed by
changing it. The interaction is between a feature that deliberately takes many
reconciles and a rate limiter sized for convergence measured in seconds. The
candidate fixes - a different default half-open interval, a budget that scales
with the composition, or not charging for reconciles that change nothing - are
all changes to `internal/circuit`, and belong in an upstream issue rather than
in this design.
