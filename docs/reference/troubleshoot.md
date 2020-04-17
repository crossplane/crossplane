---
title: Troubleshoot
toc: true
weight: 303
indent: true
---

# Troubleshooting

* [Using the trace command]
* [Resource Status and Conditions]
* [Crossplane Logs]
* [Pausing Crossplane]
* [Deleting a Resource Hangs]
* [Host-Aware Resource Debugging]

## Using the trace command

The [Crossplane CLI] trace command provides a holistic view for a particular
object and related ones to ease debugging and troubleshooting process. It finds
the relevant Crossplane resources for a given one and provides detailed
information as well as an overview indicating what could be wrong.

Usage:
```
kubectl crossplane trace TYPE[.GROUP] NAME [-n| --namespace NAMESPACE] [--kubeconfig KUBECONFIG] [-o| --outputFormat dot]
```

Examples:
```
# Trace a KubernetesApplication
kubectl crossplane trace KubernetesApplication wordpress-app-83f04457-0b1b-4532-9691-f55cf6c0da6e -n app-project1-dev

# Trace a MySQLInstance
kubectl crossplane trace MySQLInstance wordpress-mysql-83f04457-0b1b-4532-9691-f55cf6c0da6e -n app-project1-dev
```

For more information, see [the trace command documentation].

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

The next place to look to get more information or investigate a failure would be
in the Crossplane pod logs, which should be running in the `crossplane-system`
namespace. To get the current Crossplane logs, run the following:

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

## Host-Aware Resource Debugging

Stack resources (including the Stack, service accounts, deployments, and jobs)
are usually easy to identify by name. These resource names are based on the name
used in the StackInstall or Stack resource.

### Resource Location

In a host-aware configuration, these resources may be divided between the host
and the tenant.

The host, which runs the Stack controller, does not need (or get) the CRDs used
by the Stack controller. The Stack controller connects to the tenant Kubernetes
API to watch the owned types of the Stack (which is why the CRDs are only
installed on the Tenant).

Kind                  | Name               | Place
----                  | -----              | ------
pod                   | crossplane         | Host (ns: tenantFoo-system)
pod                   | stack-manager      | Host (ns: tenantFoo-system)
job                   | (stack installjob) | Host (ns: tenantFoo-controllers)
pod                   | (stack controller) | Host (ns: tenantFoo-controllers)

Kind                  | Name                                    | Place
----                  | -----                                   | ------
crd                   | Stack, SI, CSI                          | Tenant
Stack                 | wordpress                               | Tenant
StackInstall          | wordpress                               | Tenant
crd                   | KubernetesEngine, MysqlInstance, ...    | Tenant
crd                   | GKEInstance, CloudSQLInstance, ...      | Tenant
(rbac)                | (stack controller)                      | Tenant
(rbac)                | (workspace owner, crossplane-admin)     | Tenant
(rbac)                | (stack:namespace:1.2.3:admin)           | Tenant
crd                   | WordpressInstance                       | Tenant
WordpressInstance     | wp-instance                             | Tenant
KubernetesApplication | wp-instance                             | Tenant

Kind                  | Name                                    | Place
----                  | -----                                   | ------
pod                   | wp-instance (from KubernetesAplication) | New Cluster

### Name Truncation

In some cases, the full name of a Stack resource, which could be up to 253
characters long, can not be represented in the created resources. For example,
jobs and deployment names may not exceed 63 characters because these names are
turned into resource label values which impose a 63 character limit. Stack
created resources whose names would otherwise not be permitted in the Kubernetes
API will be truncated with a unique suffix.

When running the Stack Manager in host-aware mode, tenant stack resources
created in the host controller namespace generally reuse the Stack names:
"{tenant namespace}.{tenant name}".  In order to avoid the name length
restrictions, these resources may be truncated at either or both of the
namespace segment or over the entire name.  In these cases resource labels,
owner references, and annotations should be consulted to identify the
responsible Stack.

* [Relationship Labels]
* [Owner References]
* Annotations: `tenant.crossplane.io/{singular}-name` and
  `tenant.crossplane.io/{singular}-namespace` (_singular_ may be `stackinstall`,
  `clusterstackinstall` or `stack`)

#### Example

Long resource names may be present on the tenant.

```console
$ name=stack-with-a-really-long-resource-name-so-long-that-it-will-be-truncated
$ ns=this-is-just-a-really-long-namespace-name-at-the-character-max
$ kubectl create ns $ns
$ kubectl crossplane stack install --namespace $ns crossplane/sample-stack-wordpress:0.1.1 $name
```

When used as host resource names, the stack namespace and stack are concatenated
 to form host names, as well as labels.  These resource names and label values
 must be truncated to fit the 63 character limit on label values.

```console
$ kubectl --context=crossplane-host -n tenant-controllers get job -o yaml
apiVersion: v1
items:
- apiVersion: batch/v1
  kind: Job
  metadata:
    annotations:
      tenant.crossplane.io/stackinstall-name: stack-with-a-really-long-resource-name-so-long-that-it-will-be-truncated
      tenant.crossplane.io/stackinstall-namespace: this-is-just-a-really-long-namespace-name-at-the-character-max
    creationTimestamp: "2020-03-20T17:06:25Z"
    labels:
      core.crossplane.io/parent-group: stacks.crossplane.io
      core.crossplane.io/parent-kind: StackInstall
      core.crossplane.io/parent-name: stack-with-a-really-long-resource-name-so-long-that-it-wi-alqdw
      core.crossplane.io/parent-namespace: this-is-just-a-really-long-namespace-name-at-the-character-max
      core.crossplane.io/parent-uid: 596705e4-a28e-47c9-a907-d2732f07a85e
      core.crossplane.io/parent-version: v1alpha1
    name: this-is-just-a-really-long-namespace-name-at-the-characte-egoav
    namespace: tenant-controllers
  spec:
    backoffLimit: 0
    completions: 1
    parallelism: 1
    selector:
      matchLabels:
        controller-uid: 8f290bf2-8c91-494a-a76b-27c2ccb9e0a8
    template:
      metadata:
        creationTimestamp: null
        labels:
          controller-uid: 8f290bf2-8c91-494a-a76b-27c2ccb9e0a8
          job-name: this-is-just-a-really-long-namespace-name-at-the-characte-egoav
  ...
```

<!-- Named Links -->

[Using the trace command]: #using-the-trace-command
[Resource Status and Conditions]: #resource-status-and-conditions
[Crossplane Logs]: #crossplane-logs
[Pausing Crossplane]: #pausing-crossplane
[Deleting a Resource Hangs]: #deleting-a-resource-hangs
[Host-Aware Resource Debugging]: #host-aware-resource-debugging
[Crossplane CLI]: https://github.com/crossplane/crossplane-cli
[the trace command documentation]: https://github.com/crossplane/crossplane-cli/tree/master/docs/trace-command.md
[Relationship Labels]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-stack-relationship-labels.md
[Owner References]: https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents
