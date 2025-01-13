# Prevent unnecessary reconciles
* Owner: Christian Artin (@gravufo)
* Reviewers: Crossplane Maintainers
* Status: Speculative

## Introduction

Today, Crossplane providers will reconcile resources every time there is a change, regardless of the change, _and_ at pod startup _and_ when the `SyncInterval` is hit.
While this does not seem too problematic at a glance, it becomes very problematic when there are thousands of objects managed by that provider and when each reconcile can make one or multiple calls to the external provider.
Side-effects include slow performance, external rate-limiting and higher CPU usage.
A common trigger to this issue is a simple update of the provider to a newer version which causes a new pod to start which will then reconcile all its managed resources, even if they were reconciled a minute before.

## Definitions

- External reconciliation: A reconciliation which does actual work (i.e.: calls the external provider to get the resource's actual status)

## Goal

Significantly reduce the number of external reconciles done on resources to:
- Prevent unnecessary CPU usage
- Make new pods work on actual changes faster instead of being stuck behind a queue of resources that do not need to be re-validated
- Reduce load on external providers thus reducing chances of being rate limited

Reduce confusion between `SyncInterval` and `PollInterval`. With the proposed changes, we will be able to clearly define both of those properties as follows:
- `SyncInterval`: At which frequency resources should be reconciled externally to ensure the desired state is still applied.
- `PollInterval`: At which frequency the provider should re-list all its watched resources from the Kubernetes API to ensure the cache is up-to-date and no event was missed. Irrelevant when real-time compositions are enabled.

## Proposed mechanism

There are 2 facets to reaching the goal:
1. Detect when there are changes that require an external reconciliation
2. Determine if the resource is due for external reconcile based on the `SyncInterval` setting

Note: In all cases, if a resource's `Synced` or `Ready` conditions are not `True`, it should be reconciled externally to achieve a stable state.

### Detect changes requiring an external reconciliation

Here, we want to detect when changes have been done to the resource's desired state since last time it was reconciled externally.
To do so, we need to store the value of the `resourceVersion` at every external reconcile into another field which could reside inside an annotation called `crossplane.io/last-external-reconcile-resource-version`.
Then, on each reconcile we can compare the current `resourceVersion` with the value saved in the annotation. If there is a match (and all other conditions are met), we can skip the external reconcile on this resource.
Otherwise, we can run a full reconcile.

### Determine if the resource is due for an external reconcile

Crossplane providers typically expose a `SyncInterval` setting which configures how often to revalidate a resource's state.
A user which sets this setting to a value expects his resources to be reconciled at that frequency and not faster (unless changes are made to the resources' desired state of course).

An easy method of handling this is by saving a timestamp of the last external reconcile in an annotation called `crossplane.io/last-external-reconcile-timestamp`. On each subsequent reconcile, that saved value can be compared with the current time to see if the `SyncInterval` delay has passed since the last external reconcile.
If so, do a full reconcile. Otherwise, skip (assuming all other conditions are met).

## Relevant Issues

- https://github.com/crossplane/crossplane-runtime/issues/696
