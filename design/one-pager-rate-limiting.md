# Reconciler Rate Limiting

* Owner: Nic Cope (@negz)
* Status: Accepted

> This one pager retroactively documents a past design decision. See
> [issue #2595] for the original proposal.

## Background

Crossplane consists of a series of controllers working together. Ultimately, the
job of those controllers is to reconcile desired state with an external system.
The external system might be Amazon Web Services (AWS), GitHub, or a Kubernetes
cluster.

Crossplane and Crossplane provider reconciles are rate limited. These rate limits
attempt to ensure:

* The maximum rate at which Crossplane calls the external system is predictable.
* Crossplane doesn't overload the API server, or the external system.
* Crossplane is as performant as possible.

It's important that the rate at which Crossplane calls the external system is
predictable because some API calls may cost money. It's also important because
API calls are typically rate limited by the external system. Users may not want
Crossplane to exhaust those rate limits, for example because it must coexist
with other tools that are also subject to the same rate limits.

Each Crossplane provider exposes a `--max-reconcile-rate` flag that tunes its
rate limits. This flag allows users to make their own trade off between
increased reconcile throughput and increased external API calls.

## Controller Runtime Rate Limits

A controller built using `controller-runtime` v0.17 uses the following defaults.

### API Server Request Rate

An API server client that rate limits itself to 20 queries per second (qps),
bursting to 30 queries. This client is shared by all controllers that are part
of the same controller manager (e.g. same provider). See [`config.go`].

### Reconcile Rate

A rate limiter that rate limits reconciles triggered by _only_:

* A watched object changing.
* A previous reconcile attempt returning an error.
* A previous reconcile attempt returning `reconcile.Result{Requeue: true}`.

Importantly, a reconcile triggered by a previous reconcile attempt returning
`reconcile.Result{RequeueAfter: t}` is not subject to rate limiting. This means
reconciles triggered by `--poll-interval` are not subject to rate limiting when
using `controller-runtime` defaults.

When a reconcile is subject to rate limiting, the earliest time the controller
will process it will be the **maximum** of:

* The enqueue time plus a duration increasing exponentially from 5ms to 1000s
  (~16 minutes).
* The enqueue time plus a duration calculated to limit the controller to 10
  requeues per second on average, using a token bucket algorithm.

The exponential backoff rate limiting is per object (e.g. per managed resource)
while the token bucket rate limiter is per controller (e.g. per _kind of_
managed resource).

See [`controller.go`] and [`default_rate_limiters.go`].

### Concurrent Reconciles

Each controller may process at most one reconcile concurrently.

## Crossplane Rate Limits

The controller-runtime defaults are not suitable for Crossplane. Crossplane
wants:

* To wait more than 5ms before requeuing, but less than 16 minutes.
* To reconcile several managed resources of a particular kind at once.
* To rate limit _classes_ of managed resource (e.g. all AWS resources, or all
  EC2 resources).

Crossplane attempts to achieve this by deriving several rate limits from a
single flag - `--max-reconcile-rate`. The default value for this flag is usually
10 reconciles per second. The flag applies to an entire controller manager (e.g.
Crossplane, or a provider).

Note that provider maintainers must use the functions defined in [`default.go`]
to ensure these rate limits are applied at the client, global, and controller
levels.

### API Server Request Rate

An API server client that rate limits itself to `--max-reconcile-rate * 5` qps,
and `--max-reconcile-rate * 10` burst. With a default `--max-reconcile-rate` of
10 this is 50 qps bursting to 100 queries. This client is shared by all
controllers that are part of the same controller manager (e.g. same provider).
See [`default.go`].

### Reconcile Rate

Crossplane uses two layers of rate limiting.

A global token bucket rate limiter limits all controllers within a provider to
`--max-reconcile-rate` reconciles per second, bursting to
`--max-reconcile-rate * 10`. With a default `--max-reconcile-rate` of 10 this is
10 reconciles per second, bursting to 100.

All reconciles are subject to the global rate limiter, even those triggered by a
previous reconcile returning `reconcile.Result{RequeueAfter: t}`.

An exponential backoff rate limiter limits how frequently a particular object
may be reconciled, backing off from 1s to 60s. A reconcile triggered by a
previous reconcile returning `reconcile.Result{RequeueAfter: t}` is not subject
to this rate limiter.

Due to limitations of controller-runtime (see [issue #857]) the global rate
limiter is implemented as a middleware `Reconciler`. See [`reconciler.go`].

Reconciles may be rate limited by both layers.

Consider a reconcile that was requeued because it returned an error. First it's
subject to the controller's exponential backoff reconciler, which adds the
reconcile to the controller's work queue to be processed from 1 to 60 seconds in
the future.

When the reconcile is popped from the head of the work queue it's processed by
the middleware `Reconciler`, subject to its token bucket reconciler. If there
are sufficient tokens available in the bucket, the reconcile is passed to the
wrapped (inner) `Reconciler` immediately. If there aren't sufficient tokens
available, the reconcile is returned to the tail of the work queue by returning
`reconcile.Result{RequeueAfter: t}`.

This results in misleading work queue duration metrics. A reconcile may travel
through the work queue (at most) twice before it's processed.

### Concurrent Reconciles

Each controller may process at most `--max-reconcile-rate` reconciles
concurrently. With a default `--max-reconcile-rate` of 10 each controller may
process 10 reconciles concurrently. This means a provider will reconcile at most
10 managed resources of particular kind at once.

[issue #2595]: https://github.com/crossplane/crossplane/issues/2595
[`config.go`]: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.17.2/pkg/client/config/config.go#L96
[`controller.go`]: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.17.2/pkg/internal/controller/controller.go#L316
[`default_rate_limiters.go`]: https://github.com/kubernetes/client-go/blob/v0.29.2/util/workqueue/default_rate_limiters.go#L39o
[`default.go`]: https://github.com/crossplane/crossplane-runtime/blob/v1.15.0/pkg/ratelimiter/default.go
[issue #857]:  https://github.com/kubernetes-sigs/controller-runtime/issues/857
[`reconciler.go`]: https://github.com/crossplane/crossplane-runtime/blob/v1.15.0/pkg/ratelimiter/reconciler.go#L43