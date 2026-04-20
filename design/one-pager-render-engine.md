# High-Fidelity Render Engine

* Owner: Nic Cope (@negz)
* Reviewers: Adam Wolfe-Gordon (@awg)
* Status: Draft

## Background

The `crossplane render` command has become an important testing tool in the
Crossplane ecosystem. It lets you run a Composition's function pipeline locally
and see what composed resources the reconciler would produce, without a cluster.
Downstream tools build on it: the `up` CLI uses render for `up project render`
and `up project test`, and `crossplane-diff` uses it to preview changes in CI.

All of these tools depend on the same `cmd/crank/render` package, which
reimplements the XR reconciler's composition pipeline. Render shares the
mechanics of calling functions with the real reconciler, but everything the
reconciler does before and after the pipeline runs is a parallel implementation.
How composed resource metadata is rendered, how names are generated and
validated, how readiness and conditions are derived: all reimplemented.

Any time someone changes the reconciler, they must remember to update render
too. History suggests this doesn't always happen. For example, when name
validation was [added to the reconciler][name-validation] to reject composed
resources with invalid RFC 1123 names, render didn't get the same check. A user
could render successfully, deploy with confidence, and then see failures in
production.

[name-validation]: https://github.com/crossplane/crossplane/commit/015498e27

## Goals

I want render to be a high-fidelity digital twin of the real reconciler. This
means:

* **Render output should match what the real reconciler would produce.**
* **When the reconciler gains new behavior, render should automatically gain it
  too.** Adding a new validation or changing how conditions are derived
  shouldn't require a parallel change to the render implementation.
* **Callers should be able to pin the Crossplane version used for rendering.** A
  tool like `crossplane render` should support something like
  `--crossplane-version=v2.2.1`, fetching that version's render logic to test
  against exactly the reconciler behavior from that release.

It's not a goal to replace the existing `crossplane render` CLI UX. The existing
CLI handles Docker container management, file parsing, and output formatting.
Those stay where they are. I'm proposing a lower-level engine that the existing
CLI (and other tools) can build on.

It's also not a goal to auto-iterate through the reconcile loop, stepping
through successive reconciles like a debugger. That said, this design makes it
more feasible. The engine runs one reconcile loop per invocation. A harness
could call it repeatedly, feeding the composed resources from one loop back as
observed resources for the next.

## Proposal

I propose adding a hidden `crossplane internal render` subcommand to the
Crossplane binary. It runs one real `Reconcile()` call, the exact same code
path as the production reconciler, but backed by a fake in-memory client instead
of a real API server.

The fake client is simple. It satisfies reads from an in-memory store populated
with the caller's input (the XR, Composition, observed resources, secrets), and
captures writes (composed resources the reconciler would apply, resources it
would delete, status it would set). The reconciler doesn't know it's not talking
to a real API server. Every code path (pipeline execution, metadata rendering,
name generation, garbage collection, condition derivation) is the real production
code.

### Interface

The subcommand reads a protobuf `RenderRequest` from stdin and writes a protobuf
`RenderResponse` to stdout:

```
cat request.pb | crossplane internal render > response.pb
```

The expected usage is that tools like `crossplane render`, `up project render`,
and `crossplane-diff` shell out to this subcommand rather than maintaining their
own render implementations. The caller handles UX concerns (file parsing, Docker
container management, output formatting) and delegates the actual reconcile to
the Crossplane binary via stdin/stdout.

The schema is defined in `proto/render/v1alpha1/render.proto`. The request uses a
protobuf `oneof` to select the resource type to render:

```protobuf
message RenderRequest {
  RequestMeta meta = 1;
  oneof input {
    CompositeInput composite = 2;
    OperationInput operation = 3;
    CronOperationInput cron_operation = 4;
    WatchOperationInput watch_operation = 5;
  }
}
```

Each input variant contains the resource to render plus any supporting resources
(functions, observed resources, credentials). Kubernetes resources are
represented as `google.protobuf.Struct`, the same approach used by the function
protocol. For example, `CompositeInput` contains the XR, Composition, function
gRPC addresses, and optionally observed resources, credentials, and context.

The response mirrors the request with a corresponding output variant:

```protobuf
message RenderResponse {
  ResponseMeta meta = 1;
  oneof output {
    CompositeOutput composite = 2;
    OperationOutput operation = 3;
    CronOperationOutput cron_operation = 4;
    WatchOperationOutput watch_operation = 5;
  }
}
```

Using protobuf gives us a well-defined, self-documenting schema that can generate
client types in any language. If we want to expose this as a gRPC service in
future, adding a service definition to the existing proto is trivial.

Function runtime management (Docker containers, development servers) is the
caller's responsibility. The render engine accepts gRPC addresses, not container
images.

### Version Pinning

Because the render engine lives in the Crossplane binary, callers can fetch a
specific version's binary (or Docker image) to get exactly that version's render
logic. A tool like `crossplane render` could support
`--crossplane-version=v2.2.1` by pulling the v2.2.1 image and shelling out to
it. This guarantees the render output matches what that specific version of
Crossplane would produce in production. This is impossible when the render logic
is a separate reimplementation.


## Alternatives Considered

### Refactor the Reconciler into Render/Apply Phases

The architecturally pure approach: restructure the reconciler into a "render"
phase (determine what to do) and an "apply" phase (do it). Both the production
reconciler and the render engine would call the same render function. The
production reconciler would additionally call apply.

I explored this in detail. The problem is that the boundary between computation
and I/O in the reconciler isn't clean. Apply results feed back into conditions.
When a composed resource fails to apply with an `IsInvalid` error, it's marked
as unsynced, which affects the XR's Synced condition. Credentials are fetched
per-pipeline-step during function execution. The result is a three-phase model
(render, apply, finalize conditions) with events and per-resource state spanning
all three phases.

More importantly, forcing the reconciler to maintain a clean computation/I/O
boundary constrains its design. The production reconciler is the important thing.
Its structure should be driven by what makes it correct, performant, and
maintainable as a controller, not by what makes it easy to split into phases for
an offline render tool. The fake client approach inverts this priority: the
render engine adapts to whatever the reconciler does, and the reconciler stays
free to evolve without considering render at all.

The fake client approach is also more robust to reconciler evolution. Any future
change that interleaves I/O and computation in new ways would break the
render/apply boundary. The fake client approach handles it automatically. New
client calls get fake responses, and new logic runs on those fake responses.

### Expose the Reconciler as a Go Library

Rather than shelling out to a binary, downstream tools could import the
reconciler (or a render wrapper around it) as a Go library. This is essentially
what the current `cmd/crank/render` package is: a library that downstream tools
like `up` and `crossplane diff` import.

I'd prefer not to do this. It would mean taking on Hyrum's law style
compatibility obligations for the reconciler's internal API. Today the
reconciler lives in `crossplane/crossplane` and its internals can change freely.
Exposing it as a library would constrain that freedom. It would also suggest
moving render-related code to `crossplane-runtime`, where shared libraries
conventionally live. That means two repos to update for every reconciler change.

The stdin/stdout binary interface avoids this entirely. Downstream tools depend
on a stable protobuf envelope, not on Go types. It also enables version pinning,
which is impossible when the render logic is compiled into the caller.

### gRPC or HashiCorp go-plugin

We could expose the render engine as a gRPC server, either directly or via
HashiCorp's [go-plugin] framework which adds version negotiation and process
lifecycle management. We already spin up functions in Docker containers and call
them via gRPC, so the machinery exists.

The difference is that functions are gRPC servers because they run as long-lived
services in production. It makes sense for their local development mode to match
the production protocol. The render engine has no production deployment. It's
only ever a plugin for other CLIs, spun up in a Docker container to handle one
request and exit.

A persistent gRPC server would be faster for batch use cases like
crossplane-diff rendering many resources in a row, since it would avoid per-call
container startup overhead. But it's unlikely to be fast enough to matter for
the kinds of UXes we imagine building around this. The cost is server lifecycle
management (startup, health checking, graceful shutdown) that every caller has to
orchestrate.

Protobuf over stdin gives us the schema benefits (generated types,
self-documenting contract, language-agnostic) without the server lifecycle
complexity. If we ever need a long-lived server, adding a gRPC service definition
to the existing proto is a one-line change.

[go-plugin]: https://github.com/hashicorp/go-plugin
