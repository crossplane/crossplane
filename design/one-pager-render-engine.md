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

The subcommand reads a structured JSON envelope from stdin and writes one to
stdout:

```
cat input.json | crossplane internal render composite > output.json
cat input.json | crossplane internal render operation > output.json
```

The expected usage is that tools like `crossplane render`, `up project render`, and `crossplane-diff` shell out to this subcommand rather than maintaining their own render implementations. The caller handles UX concerns (file parsing, Docker container management, output formatting) and delegates the actual reconcile to the Crossplane binary via stdin/stdout.

For composite resources, the input contains the XR, Composition, function gRPC
addresses, and optionally observed resources, credentials, and context:

```json
{
  "apiVersion": "render.crossplane.io/v1alpha1",
  "kind": "CompositeInput",
  "compositeResource": {
    "apiVersion": "example.org/v1",
    "kind": "XBucket",
    "metadata": {"name": "my-bucket"},
    "spec": {"region": "us-east-2"}
  },
  "composition": {
    "metadata": {"name": "bucket-composition"},
    "spec": {
      "compositeTypeRef": {"apiVersion": "example.org/v1", "kind": "XBucket"},
      "pipeline": [
        {
          "step": "create-bucket",
          "functionRef": {"name": "function-auto-ready"}
        }
      ]
    }
  },
  "functions": [
    {"name": "function-auto-ready", "address": "localhost:9443"}
  ],
  "observedResources": [],
  "credentials": [],
  "context": {}
}
```

The output contains the rendered XR with status and conditions, composed
resources the reconciler would apply, deleted resources it would garbage collect,
and events it would emit:

```json
{
  "apiVersion": "render.crossplane.io/v1alpha1",
  "kind": "CompositeOutput",
  "compositeResource": {
    "apiVersion": "example.org/v1",
    "kind": "XBucket",
    "metadata": {"name": "my-bucket"},
    "spec": {
      "region": "us-east-2",
      "crossplane": {
        "resourceRefs": [
          {"apiVersion": "s3.aws.upbound.io/v1beta2", "kind": "Bucket", "name": "my-bucket-a8b3c1d0e2f4"}
        ]
      }
    },
    "status": {
      "conditions": [
        {"type": "Ready", "status": "True", "reason": "Available"},
        {"type": "Synced", "status": "True", "reason": "ReconcileSuccess"}
      ]
    }
  },
  "composedResources": [
    {
      "apiVersion": "s3.aws.upbound.io/v1beta2",
      "kind": "Bucket",
      "metadata": {
        "name": "my-bucket-a8b3c1d0e2f4",
        "generateName": "my-bucket-",
        "labels": {"crossplane.io/composite": "my-bucket"},
        "annotations": {"crossplane.io/composition-resource-name": "bucket"},
        "ownerReferences": [{"apiVersion": "example.org/v1", "kind": "XBucket", "name": "my-bucket", "controller": true}]
      },
      "spec": {
        "forProvider": {"region": "us-east-2"}
      }
    }
  ],
  "deletedResources": [],
  "events": [
    {"type": "Normal", "reason": "SelectComposition", "message": "Successfully selected composition: bucket-composition"}
  ]
}
```

Function runtime management (Docker containers, development servers) is the
caller's responsibility. The render engine accepts gRPC addresses, not container
images. The Operation render interface follows the same pattern.

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
on a stable JSON envelope, not on Go types. It also enables version pinning,
which is impossible when the render logic is compiled into the caller.
