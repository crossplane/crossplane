# Change Logs

* Owner: Jared Watts (@jbw976)
* Status: Proposed
* Issue: [#5477](https://github.com/crossplane/crossplane/issues/5477)

## TL;DR

* To build further trust and confidence in Crossplane, we will log every
  operation that is performed on all managed resources
* The managed reconciler will generate a record of each change made to an
  external system, as well as the state of the resource before and after the
  operation
* Each change log entry will be sent over gRPC to a sidecar container in the
  provider pod, where it will be persisted as a standard pod log
* Pod logs are a very standard location that allows many different tools to view
  and interact with the change log data
* Change log data is the only content in these logs, so it can also easily be
  sent to observability systems for further processing and analysis using a
  component like the OpenTelemetry Collector

## Background

Crossplane is deployed to manage the most critical infrastructure resources for
many of the organizations that have chosen to adopt it. Crossplane acts in a
continuously reconciling loop to drive the state of external systems and
resources to match the desired state declared by a user's platform. If changes
between the observed state of the real world and the desired state from the
users are detected, Crossplane will automatically update the resources to remove
this difference.

This means that it is possible for Crossplane to make changes to external
systems even when no user is actively interacting with the control plane.
Consider the scenario when someone updates a resource outside of Crossplane,
e.g. via the AWS console or `gcloud` CLI. When Crossplane detects this
"configuration drift", it will enforce its "source of truth" to eventually
correct this unexpected change without any user interaction with the control
plane.  Currently, there is no mechanism for users of Crossplane to have
visibility into these types of "unexpected" changes that Crossplane is making on
their behalf.

With Crossplane acting continuously and autonomously to update critical
infrastructure, it is vital for users to have insight into the operations being
performed, so they can build and maintain a strong sense of confidence and trust
in their control planes.

There are many approaches that can (and should) be taken to build trust along
the various phases of the lifecycle of control planes and the infrastructure
that they manage, but this design will focus specifically on the observability
of changes that the control plane makes to external systems at runtime. This is
not intended to facilitate review or approve the changes before they are made,
but rather to provide an auditable record of all changes that have been
performed.

This means that Crossplane (and its Providers) should start to expose data about
all changes that the control plane is making to external systems, with as much
detail as possible so the platform team can understand what is changing, for
what reason it is changing, and the outcome of the change. With this
information, Crossplane users will be able to build deeper trust and confidence
in the operation of their control planes, as well as have forensic data
available to them in the event that an unexpected change occurs.

## Proposal

### Change Log Entry Format

The change log will be a series of individual entries that capture state about
the resources involved in every operation a provider performs on its
external system. Later in this document we will explore HOW the logs are
generated and stored, but for now we will focus on WHAT will be collected.

As a general principal, we will be collecting raw observation data that has not
been processed in any significant way. A system of the users choosing that
collects the logs entries can perform opinionated calculations, analysis, or
alerting that is specific to their operating environment.

Each entry in the change log will contain the following data:

| Field Description | Example |
|-------------------| ------- |
| Timestamp | ISO 8601 format, e.g. `2023-04-01T12:34:56Z` |
| Provider name | `xpkg.upbound.io/upbound/provider-aws-ec2:v1.8.0` |
| Managed Resource identifying data (GVK/name) | `{apiVersion: ec2.aws.upbound.io/v1beta2, kind: Instance, name: dev-instance-bt66d}` |
| Operation Type | `create\|update\|delete` |
| Before operation desired/observed state of resource | `{full JSON dump of resource}` which includes desired `spec.forProvider` and observed `status.AtProvider` |
| After operation desired/observed state of resource | `{full JSON dump of resource}` which includes desired `spec.forProvider` and observed `status.AtProvider`  |
| Result of operation | `success` or error object |
| (optional) Additional information that Providers can set as they choose | `{JSON object of arbitrary data}` |

Note that we are capturing the full state of the resource both **before** and
**after** the operation is performed. We could instead just record the "before"
state during each reconciliation, but that would paint a less complete picture
compared to observing the external system and recording the "after" state
immediately after the operation as well. If we find during implementation and
testing that recording the "after" state places too much burden on the system,
we can revisit this decision.

An additional information field is available in each change log entry for
Providers to store any provider specific data they choose. Providers are not
required to store any data at all in this field, as it is optional. As an
example of data that could be stored here, `provider-aws` could store the
specific endpoints it called to perform the given operation captured in the
change log entry.

### Generating Change Logs

It is a goal that the change log data is generated as close to the source of
truth as possible. All Crossplane providers, both [classic
providers](https://github.com/crossplane/provider-template) and [Upjet generated
providers](https://github.com/crossplane/upjet-provider-template) make use of
the crossplane-runtime [managed
reconciler](https://github.com/crossplane/crossplane-runtime/blob/master/pkg/reconciler/managed/reconciler.go),
which calls into each provider to do specific CUD (Create, Update, and Delete)
operations on external resources.  Therefore, the managed reconciler is the
ideal place to generate the change log entries very close to the source of
truth.

The managed reconciler calls the provider's [`ExternalClient.Observe()`
method](https://github.com/crossplane/crossplane-runtime/blob/release-1.16/pkg/reconciler/managed/reconciler.go#L914)
before any CUD operation is performed, in order to understand the current state
of the external system before the provider acts upon it. This up to date
external state will be used to populate the "before" field in the change log
entry.

If a CUD operation is performed later on in the same `Reconcile()`, then we
will perform an additional call to this same `Observe()` method directly after
the CUD operation is performed. This second `Observe()` result will be used to
populate the "after" field in the change log entry.

Some helpful pointers to where CUD operations are performed in the managed
reconciler:

* [Create](https://github.com/crossplane/crossplane-runtime/blob/release-1.16/pkg/reconciler/managed/reconciler.go#L1058)
* [Update](https://github.com/crossplane/crossplane-runtime/blob/release-1.16/pkg/reconciler/managed/reconciler.go#L1189)
* [Delete](https://github.com/crossplane/crossplane-runtime/blob/release-1.16/pkg/reconciler/managed/reconciler.go#L954)

The code paths through the managed reconciler's `Reconcile()` method are already
a bit complex. One design choice that has helped minimize the complexity and
assist in readability is that the reconciler returns as early as it can after
performing an operation. Since we will now be adding an additional `Observe()`
call after each CUD operation, this will increase the complexity further, but
is likely unavoidable for us to obtain an immediate understanding of the "after"
state of a change, without having to wait for another reconcile.

### Storage and Durability

We consider change logs entries to be related to both security and reliability
of the control plane, and therefore we will have a fairly low tolerance for data
loss of these logs. Change logs entries will be written to a local location
immediately, so that a healthy network connection is not required at the time an
entry is generated. This local location doesn't necessarily need to be a highly
durable and persistent location itself, the key is that we don't lose any data
if we temporarily lose network connectivity while entries are being generated.

We should also minimize any effort into building a novel or specific system for
storing the change logs, so we will favor reusing built in functionality of the
control plane when possible. For example, we do not want to build any logic to
perform log rotation, compression, or other log management tasks.

Change log entries will be written to `stdout` so they can be captured by the
standard logging system of the pod they are generated from, i.e. the provider
pod, where they can benefit from all the typical conveniences of pod logs. The
change logs can then be viewed directly by entities that have read access to the
pod logs, and they can also be exported to external/centralized systems for long
term storage and further inspection and analysis. One suggested means to collect
and export the change logs is to use the [OpenTelemetry
Collector](https://opentelemetry.io/docs/collector/), but a more detailed
analysis of that setup is out of scope for this document.

#### Pod Log Benefits

Pod logs are a good option for the change log location because they:

* are very standardized, with many tools capable of reading, interacting with,
  and scraping them
* have built in management features like log rotation
* provide sufficient durability for our needs, e.g. they can survive network
  outages and pod crashes

### Flow of Change Log Entries

As mentioned in the previous section, change log entries will be written to the
pod logs. However, there is some additional structure we will build in order to
keep change logs entirely separated from the regular provider log entries, which
will aid not only with human readability, but also post processing and analysis
by the system responsible for long term storage.

Each provider pod that generates change log entries will have a sidecar
container solely responsible for receiving the entries and writing them to
`stdout` (i.e. the pod logs for the sidecar container). This sidecar container
will be listening on a gRPC connection from the main provider container. This
gRPC connection between the two containers will be established over a Unix
domain socket on a shared `emptyDir` volume mounted by both containers. This
setup allows the main provider container to send the change log entries to the
sidecar container in a fairly performant manner without needing any network
access at all.

The change log entry data will be serialized in a binary protobuf format for
transmission over the gRPC connection, while the data will be persisted to the
pod logs in a JSON serialized format.

#### Summary: Change Log Entry Flow

* The provider will have a sidecar container where each entry is sent
* Main container will communicate through gRPC with the sidecar container over a
  Unix domain socket on a shared volume
* Data is serialized in a protobuf binary format and written to the sidecar
  container logs in JSON format

### General Architecture

The general architecture of the change log system is captured in the diagram
below:

![Change logs architecture diagram](./images/change-logs-architecture.png)

### Implementation Rollout

This feature will be declared at an
[alpha](https://docs.crossplane.io/latest/learn/feature-lifecycle/#alpha-features)
level of maturity when it is first released, meaning that Crossplane users must
opt-in to using this functionality by setting an `--enable-change-logs` feature
flag. Note this flag will need to be set on each provider for which change
logs are desired, likely via a `DeploymentRuntimeConfig`.

Also note in the plan below that core Crossplane itself does not need to be updated
at all to support this feature. The key changes are in `crossplane-runtime`, the
Upjet framework, and the ecosystem of providers.

The general sequence of implementation work to roll this feature out is
described below:

* Managed reconciler is updated in `crossplane-runtime` to generate change log
  entries and send them to the gRPC server (i.e. provider sidecar container) and
  to include additional calls to `Observe()` after each CUD operation to
  capture the "after" state of the entry
* The [Upjet framework](https://github.com/crossplane/upjet) is updated to use
  this new `crossplane-runtime` logic and with additional code generation logic
  to generate providers that:
  * Create and initialize a sidecar container in the provider pod
  * Start and initialize the gRPC server in the sidecar container and the gRPC
    client in the main provider container
* The [Upjet provider
  template](https://github.com/crossplane/upjet-provider-template) is updated to
  use this new Upjet framework version, which enables new providers to have
  change log functionality
* The major upjet based providers (e.g. `provider-upjet-aws`,
  `provider-upjet-gcp`, `provider-upjet-azure`, etc.) are updated to use this
  new version of the Upjet framework, which will generate a provider capable of
  change log generation in the very next build/release of the provider.

### Assumptions

Assumptions made for this proposal include:

* Managed resources throughout the Crossplane ecosystem implement full
  `spec.forProvider` and `status.atProvider` sections so that complete desired
  and observed state is included in the change log entries simply by serializing
  the full resource.

## Future Work

The below deliverables will not be included in the initial implementation of
this feature, but should be considered in future milestones:

* The [template repo](https://github.com/crossplane/provider-template) for
  classic providers (not generated with Upjet) should be updated to include a
  sidecar container, gRPC connection, etc.
* A configuration flag for Providers that allows the user to directly send the
  change log entries to an alternate destination of their choosing besides the
  sidecar container via gRPC connection.  This alternative would also need to
  include specifying certificates and setting up mTLS to secure the connection.
* Tracing of a change log entry all the way from a Claim down to a cloud API
  call, as described in
  https://github.com/crossplane/crossplane/issues/5477#issuecomment-1996303810.

## Alternatives Considered

The follow alternative approaches have been considered for this change logs design:

### Kubernetes Events

We could generate standard Kubernetes events with that capture all of the data
we are proposing to store for each change log entry, likely stored as
annotations on the events. However, given that we propose to store the entire
object before and after a given operation is performed, this would likely be too
large for what a standard `Event` and its annotations, or the underlying etcd
storage, would allow. Relying on annotations also would diminish the portability
and ease of access of the data to a wide variety of tools.

### Kubernetes Audit Logs

The Kubernetes [audit
logs](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) capture a
similar concept to the change logs feature we are proposing, but are not a great
fit for our goals, both in the data that it captures and the scenarios for which
entries are generated. Recall the scenario where Crossplane detects drift and
corrects it in the external system without any user interaction. This discovery
of a configuration drift occurs during a regular polling `Reconcile()` call,
which is not in response to any request at the API server level. This scenario
would not be captured in the Kubernetes audit logs, as no API call was made to
the Kubernetes API server. Therefore, Kubernetes audit logs are not a great fit
for our requirements.

### OpenTelemetry Logging Events

OpenTelemetry has a concept of [logging
events](https://opentelemetry.io/docs/specs/otel/logs/event-api/#event-data-model),
but they are not a great fit either for our needs. Probably the biggest blocker
now for using otel events is that they are not very mature yet, and the Go SDK
does not yet support this concept.