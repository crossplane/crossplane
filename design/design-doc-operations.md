# Day Two Operations

* Owner: Nic Cope (@negz)
* Reviewer: Hasan Turken (@turkenh)
* Status: Accepted

## Background

Crossplane has three main components:

* Composition - an engine used to build opinionated self-service APIs (composite
  resources, or XRs)
* Managed resources (MRs) - a library of Kubernetes APIs for managing popular
  cloud resources
* The package manager - a tool for declaratively extending Crossplane with new
  functionality

Most organizations use these components to manage cloud infrastructure. Platform
engineering teams use Composition to build opinionated, self-service APIs that
are abstractions of one or more kinds of MR.

With [Crossplane v2][1] Crossplane will be simpler and better suited to managing
applications, not just the infrastructure they depend on. Notably XRs will be
able to compose any Kubernetes resource - not only Crossplane MRs.

This means you can use Crossplane v2 to build a great control plane for basic
lifecycle management of applications and infrastructure. By basic lifecycle
management I mean the control plane can create, update, and delete (CRUD) things.

On the other hand, you can't use Crossplane (alone) to build a control plane
that can handle "day two" operations. You can only use it to build controllers
that fit cleanly into the XR or MR paradigms. XRs are designed to turn high
level abstract desired state into low level desired state. MRs are designed to
take that low level desired state and control external systems accordingly.

Things like rolling upgrades, scheduling, backups, or misconfiguration detection
and remediation don't fit either the XR or MR paradigm well. Today to build a
controller that (for example) does a rolling version upgrade of a fleet of XRs
representing Kubernetes clusters you'd have to use a tool like [kubebuilder][2].

## Goals

The goal of this design is to enable you to use Crossplane to build a control
plane that handles your day two operations.

Examples of day two operations include:

* A rolling update of a fleet of Kubernetes cluster XRs
* Scheduling App XRs to Kubernetes cluster XRs
* Detecting and pausing MRs stuck in a tight reconcile loop
* Backing up databases when you delete an RDS instance MR

This is far from an exhaustive list - day two operations are pretty open ended.

The design should make it possible for you to:

* Use a function pipeline to build controllers that handle day two operations.
* Package and depend on day two operation controllers.
* Trigger a day two operation by watching arbitrary KRM resources.
* Trigger a day two operation on a regular interval.
* Mutate arbitrary KRM resources as part of a day two operation.

It's an explicit goal to use function pipelines because we've found them to be a
powerful way to build composition controllers. They allow you to focus on your
business logic, and in some cases to build controllers without writing code at
all. We think these benefits will extend to day two operation controllers.

It's explicitly _not_ a goal for Crossplane to handle arbitrary workflow
executions, like for example [Argo Workflows][3]. Operations aren't designed to
load and batch process gigabytes of billing data, for example.

## Proposal

I propose we add a new Operation type to Crossplane. Here's an example:

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: Operation
metadata:
  name: cluster-rolling-upgrade
spec:
  # How many times the pipeline can fail before the Operation fails.
  retryLimit: 5
  # A pipeline of functions - just like a Composition.
  mode: Pipeline
  pipeline:
  - step: rolling-upgrade
    functionRef:
      name: function-rolling-upgrade
    input:
      targets:
        apiVersion: example.org/v1
        kind: KubernetesCluster
        selector:
          matchLabels:
            ops.crossplane.io/eligible-for-rolling-update: "true"
      batches:
      - 0.01  # First upgrade 1% of clusters
      - 0.1   # Then if that works, 10%
      - 0.5   # Then 50%
      - 1.0   # Then 100%
      fromVersions:
      - "v1.29"
      toVersion: "v1.30"
      versionField: spec.version
      healthyConditions:
      - Synced
      - Ready
status:
  # An optional status output for each function.
  pipeline:
  - step: rolling-upgrade
    output:
      matchedTargets: 100
      upgradedTargets: 1
  # How many times the pipeline failed and was retried.
  failures: 1
  # What resources, if any, the Operation mutated.
  mutatedResources:
  - apiVersion: example.org/v1
    kind: KubernetesCluster
    namespace: default
    name: cluster-a
  # The status conditions of the Operation.
  conditions:
  - type: Succeeded
    status: "True"
    reason: PipelineSuccess
```

An Operation is a bit like a Kubernetes Job, except it runs a Crossplane
function pipeline (like a Composition), not a set of pods. It runs once to
completion, though it will retry up to its `spec.retryLimit` if the function
pipeline returns an error.

The Operation controller's logic will be very similar to the XR controller's
logic. It'll call a pipeline of _operation functions_ instead of composition
functions.

The example shows a hypothetical operation function designed to perform a
rolling upgrade of a fleet of KubernetesCluster XRs from Kubernetes v1.29 to
v1.30 by updating the `spec.version` field in increasingly larger batches. It's
important to note this is just example of a potential operation function. I
expect a broad ecosystem of operation functions to appear, just like composition
functions.

An operation function will serve the same [`FunctionRunnerService`][4] RPC as a
composition function. In fact a function could act as both a composition
function and an operation function. We'll allow functions to advertise their
capabilities by adding a new field to their package metadata:

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-python
spec:
  # A new optional field - defaults to composition for backward compatibility.
  capabilities:
  - composition
  - operation
```

These capabilities will be reflected in an installed Function package's status.
This'll allow Crossplane to avoid trying to use a composition (only) function
for operations and vice versa, e.g. due to a misconfiguration.

Using the same `FunctionRunnerService` RPC means you'll be able to use existing
Crossplane function SDKs like [function-sdk-go][5] and [function-sdk-python][6]
to build operation functions.

We'll add a new field to `run_function.proto` - `rsp.output`:

```proto
message RunFunctionResponse {
    // Existing fields omitted.

    // An arbitrary output object, to be written to the Operation's
    // status.pipeline field.
    optional google.protobuf.Struct output = 7;
}
```

By convention Crossplane will never set `req.observed` when calling an operation
function. However an operation function can return `rsp.requirements` to request
Crossplane call it again immediately with a set of extra resources it's
interested in, per the [extra resources design][7].

'Extra' resources is a misnomer in the context of an Operation. There's no
observed resources for them to be extra to. I propose we rename extra resources
to 'required' resources in the function protobuf and SDKs:

```proto
message Requirements {
  map<string, ResourceSelector> required_resources = 1;
}

message RunFunctionRequest {
    // All other fields omitted.

    map<string, Resources> required_resources = 6;
}

```

Renaming a protobuf message field isn't a breaking change, so older function
SDKs that use e.g. `GetExtraResources()` will continue to work.

An operation function can instruct Crossplane to create or update[^1] arbitary
resources by including server-side apply [fully-specified intent][8] (FSI)
patches in `rsp.desired.resources`, just like a composition function.

Each unique Operation will be the server-side apply field manager of any applied
fields, and the Operation controller will force conflicts. This means the
Operation controller will assume management of a field that's already set, and
overwrite its value.

This has a few implications:

1. Operations must take care not to fight over fields with XRs and other
   Operations. For example if an XR and an Operation both try to own an MR's
   `spec.forProvider.version` field, they'll enter an endless loop fighting over
   it.
1. Unlike an XR, an Operation can't delete a field simply by omitting it from
   its desired state (i.e. SSA FSI). XRs delete fields by first setting them in
   FSI, then later omitting them from FSI. Operations will only run once.
   They'll need to delete fields by explicitly setting them to `null`.

An Operation's pipeline will run to completion once, as soon as you create it.
I propose we support three ways - in addition to manual creation - to create an
Operation:

1. Compose an Operation using an XR
1. Create an Operation on a regular schedule using a CronOperation
1. Create an Operation when a resource changes using a WatchOperation

Composing an Operation is hopefully self-explanatory. Operation doesn't have an
abstraction resource (like an XR) by design - see [Alternatives
Considered][#alternatives-considered] for more details.

CronOperation and WatchOperation will be to Operation as CronJob is to Job in
Kubernetes. They'll contain a template for an Operation, and create one as
needed. Either on a cron schedule, or when a watched resource changes.

Here's an example CronOperation:

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: CronOperation
metadata:
  name: cluster-rolling-upgrade
spec:
  # Crontab schedule
  schedule: "0 12 * * *"
  # How long after the scheduled time to wait before considering it too late to
  # create the Operation.
  startDeadline: 10m
  # How many completed Operations to keep around.
  successfulHistoryLimit: 3
  failedHistoryLimit: 3
  # Specifies how to treat concurrent executions of an operation - Allow
  # (default), Forbid, or Replace.
  concurrencyPolicy: Forbid
  operationTemplate:
    spec:
      # How many times the pipeline can fail before the Operation fails.
      retryLimit: 5
      # A pipeline of functions - just like a Composition.
      mode: Pipeline
      pipeline:
      - step: rolling-upgrade
        functionRef:
          name: function-rolling-upgrade
        input:
          targets:
            apiVersion: example.org/v1
            kind: KubernetesCluster
            selector:
              matchLabels:
                ops.crossplane.io/eligible-for-rolling-update: "true"
          batches:
          - 0.01  # First upgrade 1% of clusters
          - 0.1   # Then if that works, 10%
          - 0.5   # Then 50%
          - 1.0   # Then 100%
          fromVersions:
          - "v1.29"
          toVersion: "v1.30"
          versionField: spec.version
          healthyConditions:
          - Synced
          - Ready
status:
  # Operations that're currently running
  active:
  - name: cluster-rolling-upgrade-amfka
  lastScheduleTime: "2024-04-18T12:00:37+00:00"
  lastSuccessfulTime: "2024-04-18T12:00:37+00:00"
```

This CronOperation makes a lot more sense than a bare Operation for the
hypothetical rolling upgrade scenario. An Operation isn't long-running - it's
akin to a single reconcile loop. So to upgrade a fleet of clusters in four
batches you'd want the Operation to run (at least) four times, with each
Operation handling the next largest batch.

Here's an example of a WatchOperation:

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: WatchOperation
metadata:
  name: schedule-app-to-cluster
spec:
  # Watch for all App XRs
  watch:
    apiVersion: example.org/v1
    kind: App
    # Optional. Defaults to all resources.
    matchLabels:
      ops.crossplane.io/auto-schedule: "true"
  # WatchOperation also supports all the top-level spec fields shown in
  # CronOperation, except for schedule.
  operationTemplate:
    # Omitted for brevity.
status:
  # Operations that're currently running.
  active:
  - name: schedule-app-to-cluster-anjda
  - name: schedule-app-to-cluster-f0d92
  # Number of resources the WatchOperation is watching.
  watchingResources: 42
```

The WatchOperation needs a way to tell the Operation it creates what watched
resource changed. Without this information the Operation's function pipeline
can't know what resource it was created to act on - e.g. what App to schedule.

I propose we address this by allowing a function pipeline step to be
'bootstrapped' with a set of required resources, like this:

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: Operation
metadata:
  name: example
spec:
  mode: Pipeline
  pipeline:
  - step: example
    functionRef: function-example
    requirements:
      requiredResources:
      - requirementName: function-needs-these-resources
        apiVersion: example.org/v1
        kind: App
        namespace: default # Namespace is optional.
        name: example-xr   # One of name or matchLabels is required.
```

Pipeline steps in Compositions and Operations would both support these explicit
'bootstrapped' requirements. Crossplane would handle these requirements as if a
function had returned them in a RunFunctionResponse. They allow Crossplane to
avoid calling a function to learn its requirements if they're known in advance.

The WatchOperation controller can then use this functionality to inject the
watched resource that changed using a special, well-known requirement name. For
example:

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: Operation
metadata:
  name: example
spec:
  mode: Pipeline
  pipeline:
  - step: rolling-upgrade
    functionRef:
      name: function-rolling-upgrade
    input:
      # Omitted for brevity
    requirements:
      requiredResources:
      - requirementName: ops.crossplane.io/watched-resource-changed
        apiVersion: example.org/v1
        kind: App
        namespace: default
        name: rip-db1
```

Note this requirement wouldn't explicitly appear in the WatchOperation's
operation template. It would be injected automatically.

With this in place, a function designed to operate on a watched resource would
always be called with the watched resource pre-populated in the RunFunctionRequest.

I propose Operations, CronOperations, and WatchOperations be valid payloads for
a Configuration package. A Configuration that delivers an Operation would cause
that Operation to run once at Configuration install time. This could be used as
an alternative to the [init XRs][10] proposal.

## Future Improvements

The following ideas aren't in scope for the first release of this feature, but
could be added in future.

### Track Specific Fields

Under the proposed design, a WatchOperation will create an Operation any time a
watched resource's `metadata.resourceVersion` changes. The `resourceVersion`
changes whenever the resource changes in any way - e.g. something updates its
metadata, spec, or status.

This may result in too many Operations. Perhaps for example the Operation should
only be created when the watched resource's status conditions change, or
`spec.size` changes.

If this turns out to be the case, we could support watching specific fields:

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: WatchOperation
metadata:
  name: schedule-app-to-cluster
spec:
  watch:
    apiVersion: example.org/v1
    kind: App
    # Optional. Defaults to any change.
    fields:
    - fieldPath: metadata.generation
  operationTemplate:
    # Omitted for brevity.
status:
  active:
  - name: schedule-app-to-cluster-anjda
  - name: schedule-app-to-cluster-f0d92
  watchingResources: 42
  lastScheduleTime: "2024-04-18T12:00:37+00:00"
  lastSuccessfulTime: "2024-04-18T12:00:37+00:00"
```

In this example the WatchOperation would only produce an Operation if a App's
`metadata.generation` changed.

To do this the WatchOperation would need to track the current value of fields.
This isn't needed with resource versions, because Kubernetes watches are
natively based on resource versions. You get a watch even each time the version
changes. To watch only certain fields Crossplane would need to filter those
watch events - e.g. by checking the new field value against its last known
value.

## Alternatives Considered

I considered the following alternatives before arriving at this proposal.


### Require Operations to Define a Custom Resource

In this alternative an Operation would have an abstraction resource like an XR.

XRs exist so that one team (a platform team) can build abstractions like 'an
app' or 'a cluster' to expose to other teams. I expect that kind of abstraction
would be overkill for most Operations.

Consider for example a platform team who wants to ensure all RDS instances are
automatically backed up before they're deleted. They probably don't need to
create an abstract RDSAutoBackup resource to configure this. They'd be defining
an abstraction only they would use.

On the other hand if they _did_ want to create an abstraction for other teams to
use, they could do that by defining an XR that composes an Operation. For
example a RDSBackup XR that composes an Operation derived from the XR.


### Just use Compositions

In this alternative we wouldn't add a new Operation type, and would use XRs to
implement all day two operations.

There's a couple of challenges with using XRs and Compositions as they exist
today:

* Conceptually, many day two operations aren't "API extensions" or "composing"
  resources. So names like XR, Composition, etc would become misnomers.
* XRs can't mutate arbitrary resources - ones they don't compose.
* XRs can't run on a cron schedule, or as one-shot reconciles.
* XRs don't run a function pipeline when the XR is deleted, and it's
  [unclear][9] what the UX for function authors should be if they did.

We could make changes to XRs and Compositions to address these, but the changes
would be significant. It'd be difficult to adapt XRs to cover the day two use
case while maintaining backward compatibility with existing Compositions, for
example.

### Use an Existing Workflow Engine

In this alternative Crossplane wouldn't address day two scenarios, and users
would use existing tools like CronJobs or [Argo Workflows][3] to handle them.

While the Operation design looks a lot like a generic Job or workflow on the
surface, it helps to think of it as a way to extend Crossplane with new
controllers. Like XRs and Compositions.

A key difference compared to Jobs or Workflows is that Operations and functions
just tell _Crossplane_ what to do. It's the core Crossplane pod that'll fetch
any needed resources, and mutate them. Being able to read and mutate KRM
resources is important for the kind of day two operations this proposal
addresses.

To read and mutate KRM resources using Jobs or Argo Workflows[^2] you'd need to
spawn pods that acted as Kubernetes clients with their own identity and RBAC
permissions. You'd need to write whatever logic these pods needed without the
help of an SDK like Crossplane's function SDKS.

Jobs and Workflows also lack some of the functionality proposed by this design,
like watch-triggered reconciles.

[^1]: I'm unsure about delete. Composition functions delete resources by
    omitting them from `rsp.desired.resources`. That won't work for operation
    functions - they'd need to set an explicit tombstone to delete a resource. I
    propose we don't support deletes until there's a clear demand.
[^2]: Workflows can actually do basic (fixed) KRM resource templating, but can't
    run a function produce a template of a resource to change.

[1]: https://github.com/crossplane/crossplane/blob/c98ccb3/design/proposal-crossplane-v2.md
[2]: https://book.kubebuilder.io
[3]: https://argoproj.github.io/workflows/
[4]: https://github.com/crossplane/crossplane/blob/c98ccb3/apis/apiextensions/fn/proto/v1/run_function.proto
[5]: https://github.com/crossplane/function-sdk-go
[6]: https://github.com/crossplane/function-sdk-python
[7]: https://github.com/crossplane/crossplane/blob/c98ccb3/design/design-doc-composition-functions-extra-resources.md
[8]: https://kubernetes.io/docs/reference/using-api/server-side-apply/
[9]: https://github.com/crossplane/crossplane/issues/5092#issuecomment-1874728842
[10]: https://github.com/crossplane/crossplane/issues/5259
