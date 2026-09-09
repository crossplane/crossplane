# Watching Required Resources

* Owner: Nic Cope (@negz)
* Reviewers: Adam Wolfe-Gordon (@adamwg)
* Status: Draft

## Background

Composition functions can ask Crossplane for resources they don't compose. A
function returns `requirements.resources` (a set of selectors, by name or by
label) and Crossplane fetches the matching resources and passes them back to the
function on its next call. We call these required resources. They were
introduced as [extra resources][extra-resources].

Required resources are read, but not watched. The XR controller watches an XR
and the resources it composes, so a change to a composed resource reconciles its
XR promptly. If a function reads a resource it doesn't compose, and that
resource changes, the XR won't reconcile until its next poll.

[#7565][issue-7565] explains why this matters. It describes a scheduler built as
a composition function. The function requires every candidate cluster and every
existing replica, then places replicas across clusters based on free capacity.
When a replica is unschedulable and a new cluster becomes ready, nothing in the
XR's own tree changed, so the XR doesn't reconcile, and the replica stays
unschedulable. Today the only fix is to poll: set a short response TTL and
re-run the function on a timer, trading scheduling latency against wasted
reconciles.

I want a change to a required resource to reconcile the XRs that required it,
the same way a change to a composed resource does.

### Prior Art: Composed Resource Watches

The XR controller doesn't use a fixed set of watches. It starts and stops them
as XRs come and go, using a controller engine that runs one shared informer per
watched kind, no matter how many XRs watch it. When an XR reconciles, the
controller starts a watch for each kind of resource the XR composes, unless it's
already watching that kind.

Two pieces connect a composed resource's change back to its XR. An API server
field index on each XR's `spec.resourceRefs` lets Crossplane list the XRs that
reference a given resource. A handler uses that index: when a composed resource
changes, it lists the XRs that reference it and reconciles them. A garbage
collector stops watches for kinds that no XR composes anymore.

Required resources have neither piece. An XR doesn't reference the resources it
requires, and - unlike composed resources, which an XR always matches by name -
a requirement can match by label. You can't build an API server field index on
the XR for resources the XR doesn't record, and you can't index a label selector
because it's a query, not a value.

## Goals

I have the following goals:

- Watch required resources so a change to one reconciles the XRs that required it.
- Keep the plumbing simple and predictable. Its cost should be easy to reason
about.
- Use one mechanism for composed and required resources rather than two.

It's not a goal to record what an XR required on the XR itself (see
Alternatives), or to expose required resource relationships as API for audit or
governance. That's a separate discussion in [#7351][issue-7351].

## Proposal

Track which resources each XR depends on in memory, and use that to drive
watches. One mechanism covers both composed and required resources.

### Track Dependencies in Memory

I propose a dependency tracker: an in-memory index that maps a changed object
back to the XRs that depend on it. A dependency is a reference: a kind, a
namespace, and either a name or a label selector.

```go
type Reference struct {
    GVK       schema.GroupVersionKind
    Namespace string            // "" matches any namespace.
    Name      string            // Match one resource by name, or...
    Labels    map[string]string // ...match resources by label (empty matches all).
}

// A Requirement is a resource a function required. It carries the pipeline step
// and the requirement name the function used, so we can seed the resource back
// into the function's request, under the same name, next reconcile.
type Requirement struct {
    Step      string
    Name      string
    Reference Reference
}

type Tracker interface {
    // Track records what an XR depends on: the resources it composed, and the
    // resources its functions required. It replaces the XR's previous record.
    Track(xr client.ObjectKey, composed []Reference, required []Requirement)

    // Forget drops an XR's record, e.g. when it's deleted.
    Forget(xr client.ObjectKey)

    // Requirements returns what a pipeline step required last reconcile, so we
    // can seed those resources back into its request.
    Requirements(xr client.ObjectKey, step string) []Requirement

    // Dependants returns the XRs that depend on a changed object.
    Dependants(obj client.Object) []client.ObjectKey

    // GVKs returns every kind with a live reference, so we know what to watch.
    GVKs() []schema.GroupVersionKind
}
```

There's one tracker per XR controller. A small registry keyed by controller name
owns them, so tests can inject their own tracker. Bootstrap requirements, the
ones a Composition declares up front, are just requirements we know before we
run any function.

### Writing and Reading the Tracker

The Composer already owns the required resource mechanism. It fetches required
resources so it can pass them to the function. Each time it composes an XR, the
Composer calls `Track` with everything that XR depends on: the resources it
composed, by name, and the resources it required, by name or label. Recording
this in the Composer keeps requirements out of the composition result. A
requirement describes how an XR was composed, so the Composer is where it
belongs.

The controller reads the tracker for two things. It uses `Dependants` as the
handler for its watches, so when a tracked resource changes it reconciles the
XRs that depend on it. It uses `GVKs` to decide what to watch, starting a watch
for each kind that has a live reference.

This replaces the field index and the composed resource handler with one path
that also handles label selectors.

### Starting and Stopping Watches

Starting a watch is idempotent, so the controller can start watches for every
tracked kind on every reconcile. Almost every call is a no-op. Stopping is
reference counted. When the last XR that referenced a kind stops referencing it,
or is deleted, that kind has no references left and its watch can stop.

Stops don't happen inline, because one XR dropping a kind at the same moment
another picks it up would race. Instead a periodic sweep stops watches for kinds
the tracker no longer references. This is simpler than today's garbage
collector, which lists every XR from the API server and double checks against an
uncached client. The tracker already knows the answer.

The cost is what happens after a restart. The tracker is in memory, so it starts
empty, and watches don't exist until XRs reconcile and repopulate it. In
practice the controller reconciles every XR on startup, so the tracker fills
right away.

### Backstop

Watches are the fast path. The informer cache behind every watch also resyncs on
a fixed interval (`--sync-interval`, one hour by default), re-delivering every
object it holds. That reconciles every XR from scratch, which re-fetches its
required resources and re-registers its references. So the fast path can miss an
event: one that lands between fetching a requirement and recording it, or a
watch a sweep stopped just as an XR came to need it. Both are recovered within
one sync interval. This is the same backstop composed resource watches rely on
today.

Because missed events are rare and self-heal, the tracker only needs to converge,
not catch every event as it happens. The resync also frees the response TTL.
Functions default to a 60 second TTL, which requeues every XR every minute; that
default is deliberately low while realtime compositions are in beta. Because the
resync is the real backstop, not the TTL, we can raise that default, to a few
hours, without weakening freshness for required resources.

### Seeding Requirements

Because the tracker remembers what an XR required last time, we can send those
resources to the function before it asks for them. Resolving a function's
requirements takes at least two calls today: one to learn the requirements, and
another with the resources populated. We seed each step's request with the
resources it required last reconcile, under the function's own requirement
names, alongside the Composition's bootstrap requirements. When the seed already
satisfies the function, the function runner returns after the first call instead
of a second, confirming one. If the function asks for something different, the
runner fetches and re-runs. The seed is only a hint, and a function whose
requirements never settle still errors.

### The Function Response Cache

The alpha function response cache keys each response on a hash of the function
request. It's tempting to assume this makes watching safe for free: change a
required resource, change the hash, miss the cache, re-run the function. That
holds only for bootstrap requirements, which are fetched and hashed before the
function runs. A function's dynamic requirements are fetched after the request
is hashed, so they aren't in the key. A required resource could change, the XR
could reconcile, and the cache could return the same response because the key
didn't move. Watching would do nothing whenever the cache is on.

Seeding closes this. Because we seed the request with the last reconcile's
required resources before hashing it, the hash reflects their current content.
When a required resource changes, so does the seeded content, and with it the
request hash, so the cache misses and the function re-runs. We hash the seeded
resources but still let the function resolve its own requirements, so a stale
hint only costs a cache miss.

This leans on what the cache already assumes: that a function is a pure function
of its request. An impure function - one that grows a requirement with no change
to its inputs, from a network call or the time of day - could be served a cached
response when it wanted to run again. The cache has always worked this way. Such
a function opts out by setting a zero TTL, which is never cached. Tracking and
watching don't depend on the TTL, so even an uncached function's required
resources are still watched.

## Predictability

The tracker's memory is linear in the number of XRs times the references each XR
has. It's independent of how many resources each reference matches. A
match-everything requirement, every replica in the fleet, is one reference, not
one per matched resource.

Some rough numbers. A reference costs a few hundred bytes. A control plane with
1,000 XRs, each with a handful of references, is a few megabytes. A large one
with 100,000 XRs, each with a dozen references, is a few hundred megabytes. Most
of that is the composed references that replace the existing field index, which
already costs about the same.

Either way it's dwarfed by the informer caches the watches need, which hold
every object of each watched kind at a few kilobytes apiece.

## Alternatives Considered

### Record Required Resources on the XR

We could record what an XR required in its status and index that the way we
index composed resources. This is [#7351][issue-7351]. I don't think we should,
for three reasons. The volume can be large: post-[#7241][pr-7241] a single
requirement can match hundreds of resources, and a scheduler that requires the
whole fleet would record the whole fleet on every XR. A list of what was
required last reconcile is a hint, not a guarantee of what'll be required next,
so it's misleading for the drift detection people want it for. And it exposes
implementation detail, how an XR was composed, as part of the XR's own API.
Tracking in memory avoids all three, and a reference is a selector, not a list
of everything it matched.

### An API Server Field Index for Required Resources

Composed resource watches use an API server field index on the XR. We can't
reuse that for required resources. An XR doesn't record what it requires, and a
field index needs a value to index, whereas a requirement can be a label
selector, which has no single value to index.

### Let the Tracker Own the Engine

The tracker could start and stop watches itself, instead of the controller
reading `GVKs` and driving the engine. That removes the periodic sweep, but it
couples the tracker to the controller engine and makes it harder to test on its
own. I'd rather keep the tracker a plain data structure and let the controller
drive the engine, as it does for composed resources today.

[extra-resources]: design-doc-composition-functions-extra-resources.md
[pr-7241]: https://github.com/crossplane/crossplane/pull/7241
[issue-7565]: https://github.com/crossplane/crossplane/issues/7565
[issue-7351]: https://github.com/crossplane/crossplane/issues/7351
