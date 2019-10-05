# Error and event reporting

* Owner: Daniel Suskin (@suskin)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Introduction

*Why does this document exist?*

The question of debuggability and troubleshooting has started to come up
recently, as we have tried out some scenarios that include Stacks and
different providers. The intent of this document is to propose a mindset
and framework for reporting errors and other information.

## Goals

In short, a non-admin user and an admin user should both be able to
debug any issues only by inspecting logs and events. There should be no
need to rebuild the Crossplane binary or to reach out to a Crossplane
developer.

A user should be able to:

* Debug an issue without rebuilding the Crossplane binary
* Understand an issue without contacting a cluster admin
* Ask a cluster admin to check the logs for more details about the
  reason the issue happened, if the details are not part of the error
  message

A cluster admin should be able to:

* Debug an issue without rebuilding the Crossplane binary
* Debug an issue only by looking at the logs
* Debug an issue without needing to contact a Crossplane developer


## Error reporting in the logs

Error reporting in the logs is mostly intended for consumption by
cluster admins. A cluster admin should be able to debug any issue by
inspecting the logs, without needing to add more logs themselves or
contact a Crossplane developer. This means that logs should contain:

* Error messages
* Any context leading up to an error, so that the errors can be debugged
  (the context can be logged at a more verbose level)
* Ideally, a unique identifier per session so that log lines can be
  tracked as a sequence when there are multiple workers logging to the
  same file

## Error reporting as events

Error reporting as Kubernetes events is primarily aimed toward end-users
of Crossplane who are not cluster admins.
[Events](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#event-v1-core)
are available as top-level Kubernetes objects, and show up on objects
they are related to.

Events should be recorded in the following cases:

* An operation is taken on a resource
* The state of a resource is changed
* An error occurs

The events recorded in these cases can be thought of as forming an event
log of things that happen for the resources that Crossplane manages.
Each event should refer back to the relevant controller and resource,
and use other fields of the Event kind as appropriate.

More details about examples of how to interact with events can be found
in the guide to [debugging an application
cluster](https://kubernetes.io/docs/tasks/debug-application-cluster/).

## Choosing between methods of error reporting

There are many ways to report errors, such as:

* Metrics
* Events
* Logging
* Tracing

It can be confusing to figure out which one is appropriate in a given
situation. This section will try to offer advice and a mindset that can
be used to help make this decision.

Let's set the context by listing the different user scenarios where
error reporting may be consumed. Here are the typical scenarios as we
imagine them:

1. A person **using** a system needs to figure out why things aren't
   working as expected, and whether they made a mistake that they can
   correct.
2. A person **operating** a service needs to monitor the service's
   **health**, both now and historically.
3. A person **debugging** a problem which happened in a **live
   environment** (often an **operator** of the system) needs information
   to figure out what happened.
4. A person **developing** the software wants to **observe** what is
   happening.
5. A person **debugging** the software in a **development environment**
   (typically a **developer** of the system) wants to debug a problem
   (there is a lot of overlap between this and the live environment
   debugging scenario).

The goal is to satisfy the users in all of the scenarios. We'll refer to
the scenarios by number.

The short version is: we should do whatever satisfies all of the
scenarios. Logging and events are the recommendations for satisfying the
scenarios, although they don't cover scenario 2.

The longer version is:

* Scenario 1 is best served by events in the context of Crossplane,
  since the users may not have access to read logs or metrics, and even
  if they did, it would be hard to relate them back to the event the user
  is trying to understand.
* Scenario 2 is best served by metrics, because they can be aggregated
  and understood as a whole. And because they can be used to track
  things over time.
* Scenario 3 is best served by either logging that contains all the
  information about and leading up to the event. Or, if available, some
  sort of error event reporting system which captures all of the relevant
  information in one place (for example, a well-configured project in
  [Sentry](https://sentry.io/)). In my experience, the larger a fleet
  gets, the less useful logging is, and the more useful the error event
  reporting system becomes. Request-tracing systems are also useful for
  this scenario.
* Scenario 4 is usually logs, maybe at a more verbose level than normal.
  But it could be an attached debugger or some other type of tool. It
  could also be a test suite.
* Scenario 5 is usually either logs, up to the highest imaginable
  verbosity, or an attached debugging session. If there's a gap in
  reporting, it could involve adding some print statements to get more
  logging.

As for the question of how to decide whether to log or not, we believe
it helps to try to visualize which of the scenarios the error or
information in question will be used for. We recommend starting with
reporting as much information as possible, but with configurable runtime
behavior so that, for example, debugging logs don't show up in
production normally.

For the question of what constitutes an error, errors should be
actionable by a human. See the [Dave Cheney article on this
topic](https://dave.cheney.net/2015/11/05/lets-talk-about-logging) for
some more discussion.
