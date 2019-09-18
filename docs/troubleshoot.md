---
title: Troubleshooting
toc: true
weight: 820
indent: true
---
# Troubleshooting

* [Resource Status and Conditions](#resource-status-and-conditions)
* [Crossplane Logs](#crossplane-logs)
* [Pausing Crossplane](#pausing-crossplane)
* [Deleting a Resource Hangs](#deleting-a-resource-hangs)

## Resource Status and Conditions

Most Crossplane resources have a `status` section that can represent the current
state of that particular resource. Running `kubectl describe` against a
Crossplane resource will frequently give insightful information about its
condition. For example, to determine the status of a MySQLInstance resource
claim, run:

```shell
kubectl -n app-project1-dev describe mysqlinstance mysql-claim
```

This should produce output that includes:

```console
Status:
  Conditions:
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Managed claim is waiting for managed resource to become bindable
    Status:                False
    Type:                  Ready
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Successfully reconciled managed resource
    Status:                True
    Type:                  Synced
```

Most Crossplane resources set exactly two condition types; `Ready` and `Synced`.
`Ready` represents the availability of the resource itself - whether it is
creating, deleting, available, unavailable, binding, etc. `Synced` represents
the success of the most recent attempt to 'reconcile' the _desired_ state of the
resource with its _actual_ state. The `Synced` condition is the first place you
should look when a Crossplane resource is not behaving as expected.

## Crossplane Logs

The next place to look to get more information or investigate a failure would
be in the Crossplane pod logs, which should be running in the
`crossplane-system` namespace. To get the current Crossplane logs, run the
following:

```shell
kubectl -n crossplane-system logs -lapp=crossplane
```

Remember that much of Crossplane's functionality is provided by Stacks. You can
use `kubectl logs` to view Stack logs too, though Stacks may not run in the
`crossplane-system` namespace.

## Pausing Crossplane

Sometimes, for example when you encounter a bug. it can be useful to pause
Crossplane if you want to stop it from actively attempting to manage your
resources. To pause Crossplane without deleting all of its resources, run the
following command to simply scale down its deployment:

```bash
kubectl -n crossplane-system scale --replicas=0 deployment/crossplane
```

Once you have been able to rectify the problem or smooth things out, you can unpause Crossplane simply by scaling its deployment back up:

```bash
kubectl -n crossplane-system scale --replicas=1 deployment/crossplane
```

Remember that much of Crossplane's functionality is provided by Stacks. You can
use `kubectl scale` to pause Stack pods too, though Stacks may not run in the
`crossplane-system` namespace.

## Deleting a Resource Hangs

The resources that Crossplane manages will automatically be cleaned up so as not
to leave anything running behind. This is accomplished by using finalizers, but
in certain scenarios the finalizer can prevent the Kubernetes object from
getting deleted.

To deal with this, we essentially want to patch the object to remove its
finalizer, which will then allow it to be deleted completely. Note that this
won't necessarily delete the external resource that Crossplane was managing, so
you will want to go to your cloud provider's console and look there for any
lingering resources to clean up.

In general, a finalizer can be removed from an object with this command:

```console
kubectl patch <resource-type> <resource-name> -p '{"metadata":{"finalizers": []}}' --type=merge
```

For example, for a Workload object (`workloads.compute.crossplane.io`) named
`test-workload`, you can remove its finalizer with:

```console
kubectl patch workloads.compute.crossplane.io test-workload -p '{"metadata":{"finalizers": []}}' --type=merge
```
