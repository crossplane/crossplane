---
title: Observability Developer Guide
weight: 1002
---

## Introduction

Observability is crucial to Crossplane users; both those operating Crossplane
and those using Crossplane to operate their infrastructure. Crossplane currently
approaches observability via Kubernetes events and structured logs.

## Goals

In short, a non-admin user and an admin user should both be able to debug any
issues only by inspecting logs and events. There should be no need to rebuild
the Crossplane binary or to reach out to a Crossplane developer.

A user should be able to:

* Debug an issue without rebuilding the Crossplane binary
* Understand an issue without contacting a cluster admin
* Ask a cluster admin to check the logs for more details about the reason the
  issue happened, if the details are not part of the error message

A cluster admin should be able to:

* Debug an issue without rebuilding the Crossplane binary
* Debug an issue only by looking at the logs
* Debug an issue without needing to contact a Crossplane developer

## Error reporting in the logs

Error reporting in the logs is mostly intended for consumption by Crossplane
cluster admins. A cluster admin should be able to debug any issue by inspecting
the logs, without needing to add more logs themselves or contact a Crossplane
developer. This means that logs should contain:

* Error messages, at either the info or debug level as contextually appropriate
* Any context leading up to an error, typically at debug level, so that the
  errors can be debugged

## Error reporting as events

Error reporting as Kubernetes events is primarily aimed toward end-users of
Crossplane who are not cluster admins. Crossplane typically runs as a Kubernetes
pod, and thus it is unlikely that most users of Crossplane will have access to
its logs. [Events], on the other hand, are available as top-level Kubernetes
objects, and show up the objects they relate to when running `kubectl describe`.

Events should be recorded in the following cases:

* A significant operation is taken on a resource
* The state of a resource is changed
* An error occurs

The events recorded in these cases can be thought of as forming an event log of
things that happen for the resources that Crossplane manages. Each event should
refer back to the relevant controller and resource, and use other fields of the
Event kind as appropriate.

More details about examples of how to interact with events can be found in the
guide to [debugging an application cluster].

## Choosing between methods of error reporting

There are many ways to report errors, such as:

* Metrics
* Events
* Logging
* Tracing

It can be confusing to figure out which one is appropriate in a given situation.
This section will try to offer advice and a mindset that can be used to help
make this decision.

Let's set the context by listing the different user scenarios where error
reporting may be consumed. Here are the typical scenarios as we imagine them:

1. A person **using** a system needs to figure out why things aren't working as
   expected, and whether they made a mistake that they can correct.
2. A person **operating** a service needs to monitor the service's **health**,
   both now and historically.
3. A person **debugging** a problem which happened in a **live environment**
   (often an **operator** of the system) needs information to figure out what
   happened.
4. A person **developing** the software wants to **observe** what is happening.
5. A person **debugging** the software in a **development environment**
   (typically a **developer** of the system) wants to debug a problem (there is
   a lot of overlap between this and the live environment debugging scenario).

The goal is to satisfy the users in all of the scenarios. We'll refer to the
scenarios by number.

The short version is: we should do whatever satisfies all of the scenarios.
Logging and events are the recommendations for satisfying the scenarios,
although they don't cover scenario 2.

The longer version is:

* Scenario 1 is best served by events in the context of Crossplane, since the
  users may not have access to read logs or metrics, and even if they did, it
  would be hard to relate them back to the event the user is trying to
  understand.
* Scenario 2 is best served by metrics, because they can be aggregated and
  understood as a whole. And because they can be used to track things over time.
* Scenario 3 is best served by either logging that contains all the information
  about and leading up to the event. Request-tracing systems are also useful for
  this scenario.
* Scenario 4 is usually logs, maybe at a more verbose level than normal. But it
  could be an attached debugger or some other type of tool. It could also be a
  test suite.
* Scenario 5 is usually either logs, up to the highest imaginable verbosity, or
  an attached debugging session. If there's a gap in reporting, it could involve
  adding some print statements to get more logging.

As for the question of how to decide whether to log or not, we believe it helps
to try to visualize which of the scenarios the error or information in question
will be used for. We recommend starting with reporting as much information as
possible, but with configurable runtime behavior so that, for example, debugging
logs don't show up in production normally.

For the question of what constitutes an error, errors should be actionable by a
human. See the [Dave Cheney article] on this topic for some more discussion.

## In Practice

Crossplane provides two observability libraries as part of crossplane-runtime:

* [`event`] emits Kubernetes events.
* [`logging`] produces structured logs. Refer to its package documentation for
  additional context on its API choices.

Keep the following in mind when using the above libraries:

* [Do] [not] use package level loggers or event recorders. Instantiate them in
  `main()` and plumb them down to where they're needed.
* Each [`Reconciler`] implementation should use its own `logging.Logger` and
  `event.Recorder`. Implementations are strongly encouraged to default to using
  `logging.NewNopLogger()` and `event.NewNopRecorder()`, and accept a functional
  loggers and recorder via variadic options. See for example the [managed
  resource reconciler].
* Each controller should use its name as its event recorder's name, and include
  its name under the `controller` structured logging key. The controllers name
  should be of the form `controllertype/resourcekind`, for example
  `managed/cloudsqlinstance` or `stacks/stackdefinition`. Controller names
  should always be lowercase.
* Logs and events should typically be emitted by the `Reconcile` method of the
  `Reconciler` implementation; not by functions called by `Reconcile`. Author
  the methods orchestrated by `Reconcile` as if they were a library; prefer
  surfacing useful information for the `Reconciler` to log (for example by
  [wrapping errors]) over plumbing loggers and event recorders down to
  increasingly deeper layers of code.
* Almost nothing is worth logging at info level. When deciding which logging
  level to use, consider a production deployment of Crossplane reconciling tens
  or hundreds of managed resources. If in doubt, pick debug. You can easily
  increase the log level later if it proves warranted.
* The above is true even for errors; consider the audience. Is this an error
  only the Crossplane cluster operator can fix? Does it indicate a significant
  degradation of Crossplane's functionality? If so, log it at info. If the error
  pertains to a single Crossplane resource emit an event instead.
* Always log errors under the structured logging key `error` (e.g.
  `log.Debug("boom!, "error", err)`). Many logging implementations (including
  Crossplane's) add context like stack traces for this key.
* Emit events liberally; they're rate limited and deduplicated.
* Follow [API conventions] when emitting events; ensure event reasons are unique
  and `CamelCase`.
* Consider emitting events and logs when a terminal condition is encountered
  (e.g. `Reconcile` returns) over logging logic flow. i.e. Prefer one log line
  that reads "encountered an error fooing the bar" over two log lines that read
  "about to foo the bar" and "encountered an error". Recall that if the audience
  is a developer debugging Crossplane they will be provided a stack trace with
  file and line context when an error is logged.
* Consider including the `reconcile.Request`, and the resource's UID and
  resource version (not API version) under the keys `request`, `uid`, and
  `version`. Doing so allows log readers to determine what specific version of a
  resource the log pertains to.

Finally, when in doubt, aim for consistency with existing Crossplane controller
implementations.

<!-- Named Links -->

[Events]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#event-v1-core
[debugging an application cluster]: https://kubernetes.io/docs/tasks/debug-application-cluster/
[Dave Cheney article]: https://dave.cheney.net/2015/11/05/lets-talk-about-logging
[`event`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/event
[`logging`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/logging
[Do]: https://peter.bourgon.org/go-best-practices-2016/#logging-and-instrumentation
[not]: https://dave.cheney.net/2017/01/23/the-package-level-logger-anti-pattern
[`Reconciler`]: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
[managed resource reconciler]: https://github.com/crossplane/crossplane-runtime/blob/a6bb0/pkg/reconciler/managed/reconciler.go#L436
[wrapping errors]: https://godoc.org/github.com/pkg/errors#Wrap
[API conventions]: https://github.com/kubernetes/community/blob/09f55c6/contributors/devel/sig-architecture/api-conventions.md#events
