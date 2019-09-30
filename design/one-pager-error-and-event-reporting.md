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
