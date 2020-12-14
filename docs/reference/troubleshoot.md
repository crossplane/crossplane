---
title: Troubleshoot
toc: true
weight: 303
indent: true
---

# Troubleshooting

* [Resource Status and Conditions]
* [Resource Events]
* [Crossplane Logs]
* [Pausing Crossplane]
* [Deleting a Resource Hangs]

## Resource Status and Conditions

Most Crossplane resources have a `status` section that can represent the current
state of that particular resource. Running `kubectl describe` against a
Crossplane resource will frequently give insightful information about its
condition. For example, to determine the status of a MySQLInstance resource
claim, run:

```shell
kubectl describe mysqlinstance mysql
```

This should produce output that includes:

```console
Status:
  Conditions:
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Composite resource is not yet Ready
    Status:                False
    Type:                  Ready
```

Most Crossplane resources set the `Ready` condition. `Ready` represents the
availability of the resource - whether it is creating, deleting, available,
unavailable, binding, etc.

## Resource Events

Most Crossplane resources emit _events_ when something interesting happens. You
can see the events associated with a resource by running `kubectl describe` - 
e.g. `kubectl describe mysqlinstance mysql`. You can also see all events in a
particular namespace by running `kubectl get events`.

```console
Events:
  Type     Reason                   Age                From                                                   Message
  ----     ------                   ----               ----                                                   -------
  Warning  CannotConnectToProvider  16s (x4 over 46s)  managed/postgresqlserver.database.azure.crossplane.io  cannot get referenced ProviderConfig: ProviderConfig.azure.crossplane.io "default" not found
```

> Note that events are namespaced, while many Crossplane resources (XRs, etc)
> are cluster scoped. Crossplane emits events for cluster scoped resources to
> the 'default' namespace.

## Crossplane Logs

The next place to look to get more information or investigate a failure would be
in the Crossplane pod logs, which should be running in the `crossplane-system`
namespace. To get the current Crossplane logs, run the following:

```shell
kubectl -n crossplane-system logs -lapp=crossplane
```

Remember that much of Crossplane's functionality is provided by providers. You
can use `kubectl logs` to view provider logs too.

> Note that Crossplane emits few logs by default - events are typically the best
> place to look for information about what Crossplane is doing. You may need to
> restart Crossplane (or your provider) with the `--debug` flag if you can't 
> find what you're looking for.

## Pausing Crossplane

Sometimes, for example when you encounter a bug. it can be useful to pause
Crossplane if you want to stop it from actively attempting to manage your
resources. To pause Crossplane without deleting all of its resources, run the
following command to simply scale down its deployment:

```bash
kubectl -n crossplane-system scale --replicas=0 deployment/crossplane
```

Once you have been able to rectify the problem or smooth things out, you can
unpause Crossplane simply by scaling its deployment back up:

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

<!-- Named Links -->

[Using the trace command]: #using-the-trace-command
[Resource Status and Conditions]: #resource-status-and-conditions
[Crossplane Logs]: #crossplane-logs
[Pausing Crossplane]: #pausing-crossplane
[Deleting a Resource Hangs]: #deleting-a-resource-hangs
[Crossplane CLI]: https://github.com/crossplane/crossplane-cli
[the trace command documentation]: https://github.com/crossplane/crossplane-cli/tree/master/docs/trace-command.md
[Owner References]: https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents
