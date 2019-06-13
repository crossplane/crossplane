---
title: Troubleshooting
toc: true
weight: 360
indent: true
---
# Troubleshooting

* [Crossplane Logs](#crossplane-logs)
* [Resource Status and Conditions](#resource-status-and-conditions)
* [Pausing Crossplane](#pausing-crossplane)
* [Deleting a Resource Hangs](#deleting-a-resource-hangs)

## Crossplane Logs

The first place to look to get more information or investigate a failure would be in the Crossplane pod logs, which should be running in the `crossplane-system` namespace.
To get the current Crossplane logs, run the following:

```console
kubectl -n crossplane-system logs -lapp=crossplane
```

## Resource Status and Conditions

All of the objects that represent managed resources such as databases, clusters, etc. have a `status` section that can give good insight into the current state of that particular object.
In general, simply getting the `yaml` output of a Crossplane object will give insightful information about its condition:

```console
kubetl get <resource-type> -o yaml
```

For example, to get complete information about an Azure AKS cluster object, the following command will generate the below sample (truncated) output:

```console
> kubectl -n crossplane-system get akscluster -o yaml
...
  status:
    Conditions:
    - LastTransitionTime: 2018-12-04T08:03:01Z
      Message: 'failed to start create operation for AKS cluster aks-demo-cluster:
        containerservice.ManagedClustersClient#CreateOrUpdate: Failure sending request:
        StatusCode=400 -- Please see https://aka.ms/acs-sp-help for more details."'
      Reason: failed to create cluster
      Status: "False"
      Type: Failed
    - LastTransitionTime: 2018-12-04T08:03:14Z
      Message: ""
      Reason: ""
      Status: "False"
      Type: Creating
    - LastTransitionTime: 2018-12-04T09:59:43Z
      Message: ""
      Reason: ""
      Status: "True"
      Type: Ready
    bindingPhase: Bound
    endpoint: crossplane-aks-14af6e93.hcp.centralus.azmk8s.io
    state: Succeeded
```

We can see a few conditions in that AKS cluster's history.
It first encountered a failure, then it moved into the `Creating` state, then it finally became `Ready` later on.
Conditions that have `Status: "True"` are currently active, while conditions with `Status: "False"` happened in the past, but are no longer happening currently.

## Pausing Crossplane

Sometimes, it can be useful to pause Crossplane if you want to stop it from actively attempting to manage your resources, for instance if you have encountered a bug.
To pause Crossplane without deleting all of its resources, run the following command to simply scale down its deployment:

```console
kubectl -n crossplane-system scale --replicas=0 deployment/crossplane
```

Once you have been able to rectify the problem or smooth things out, you can unpause Crossplane simply by scaling its deployment back up:

```console
kubectl -n crossplane-system scale --replicas=1 deployment/crossplane
```

## Deleting a Resource Hangs

The resources that Crossplane manages will automatically be cleaned up so as not to leave anything running behind.
This is accomplished by using finalizers, but in certain scenarios, the finalizer can prevent the Kubernetes object from getting deleted.

To deal with this, we essentially want to patch the object to remove its finalizer, which will then allow it to be deleted completely.
Note that this won't necessarily delete the external resource that Crossplane was managing, so you will want to go to your cloud provider's console and look there for any lingering resources to clean up.

In general, a finalizer can be removed from an object with this command:

```console
kubectl patch <resource-type> <resource-name> -p '{"metadata":{"finalizers": []}}' --type=merge
```

For example, for a Workload object (`workloads.compute.crossplane.io`) named `test-workload`, you can remove its finalizer with:

```console
kubectl patch workloads.compute.crossplane.io test-workload -p '{"metadata":{"finalizers": []}}' --type=merge
```
