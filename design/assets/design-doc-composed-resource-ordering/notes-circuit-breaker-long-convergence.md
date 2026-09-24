# Issue Draft: Watch Circuit Breaker and Long Convergences

Ready to paste into a GitHub issue against crossplane/crossplane. The `###`
headings match the project's bug report template, so they look odd read as a
document.

Kept here because the measurement came out of prototyping composed resource
ordering, and records the reproduction and suggested directions that the
proposal itself does not. Delete this file once the issue is filed.

---

## Issue body

### What happened?

An XR whose composition takes many reconciles to converge exhausts its watch
circuit breaker budget part way through, after which every subsequent step
falls back to the poll interval. Convergence degrades mid-run, and the deeper
the resource graph the worse it gets.

The breaker's budget is sized for convergence measured in seconds. Anything
that legitimately takes minutes — a deep dependency chain, slow-to-become-ready
managed resources, staged rollouts — spends its tokens before it finishes.

Measured on a ten-resource graph four levels deep, with each resource becoming
ready ten seconds after creation:

| Wave | Interval | Breaker |
| --- | --- | --- |
| 1-4 | 10-11s each | closed |
| 5-8 | 56-63s each | open |

An eight-deep chain with a two second readiness delay took 271s against an
ideal of roughly 16s. The breaker opened one second after the fourth wave.

The XR reports:

```text
Responsive=False  WatchCircuitOpen
Too many watch events from XOrdering/ordered (default). Allowing events periodically.
```

### How can we reproduce it?

Any composition that stays unconverged for more than a minute or so will do.
The reliable recipe is a chain of `provider-nop` NopResources where each one
becomes ready on a delay and each is only created after its predecessor is
ready.

Requires `--enable-realtime-compositions`. The XR's `Responsive` condition
flips to `WatchCircuitOpen` part way through, and wave times jump from the
configured readiness delay to the poll interval.

### What did you observe?

Some things worth ruling out, because they were my first guesses and both were
wrong:

**The XR is not being rewritten.** Across one wave the reconciler logged 125
reconciles at a single unchanged `resourceVersion`, and 149 at another. This
is not a self-inflicted write loop on the XR.

**The composition pipeline is not slow.** Instrumenting it showed 1-3ms per
reconcile while waves were 10-60s apart. The time is in being woken up, not in
doing the work.

What is left is the volume of legitimate watch events from composed resources
during a long convergence. Each composed resource that changes enqueues its XR
through `circuit.NewMapFunc` ([definition/reconciler.go#L606][mapfunc]), and
each Update costs two tokens, because controller-runtime invokes the MapFunc
once with the old object and once with the new.

With the defaults — capacity 100, refill 1/s, so 50 Update events of burst and
one event per two seconds sustained ([token_bucket.go#L122][config]) — ten
composed resources moving through create and ready transitions over a minute
comfortably exceeds that. Once open, the circuit stays open for five minutes
and allows a probe every 30 seconds, which is what turns 10s waves into 60s
waves.

### What did you expect to happen?

Either that the budget accommodates a convergence that legitimately takes
minutes, or that exceeding it degrades more gracefully than dropping to the
poll interval for five minutes.

The breaker exists to protect against pathological hot loops, and that is worth
keeping. The problem is that "this XR is genuinely taking a while to converge"
and "this XR is stuck in a write loop" currently look identical to it.

### Possible directions

Not sure which of these is right, and they are not mutually exclusive:

* **Don't charge for reconciles that change nothing.** A reconcile that
  produces no writes is evidence the XR is waiting, not looping. Charging only
  for reconciles that mutate something would distinguish the two cases directly.
* **Scale the budget with the composition.** A fixed 50-event burst means a
  three-resource XR and a fifty-resource XR get the same allowance.
* **Back off proportionally rather than to the poll interval.** Falling from
  10s to 60s in one step is a large cliff, and it is invisible unless you are
  watching the `Responsive` condition.
* **Reconsider the 5 minute cooldown** for an XR that is actively converging.

### Anything else?

Found while prototyping composed resource ordering, which makes this easy to
hit because it deliberately spreads composition across many reconciles. But
nothing about it is specific to that feature — any slow-converging composition
with enough composed resources should reproduce it, and larger compositions
will hit it sooner.

Happy to supply the prototype and manifests if a reproducible case is useful.

[mapfunc]: https://github.com/crossplane/crossplane/blob/main/internal/controller/apiextensions/definition/reconciler.go#L606
[config]: https://github.com/crossplane/crossplane/blob/main/internal/circuit/token_bucket.go#L122
