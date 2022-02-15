---
title: Troubleshoot
toc: true
weight: 305
indent: true
---

# Troubleshooting

* [Requested Resource Not Found]
* [Resource Status and Conditions]
* [Resource Events]
* [Crossplane Logs]
* [Provider Logs]
* [Pausing Crossplane]
* [Pausing Providers]
* [Deleting When a Resource Hangs]
* [Installing Crossplane Package]
* [Handling Crossplane Package Dependency]

## Requested Resource Not Found

If you use the kubectl Crossplane plugin to install a `Provider` or `Configuration`
(e.g. `kubectl crossplane install provider crossplane/provider-aws:v1.6.3`) and
get `the server could not find the requested resource` error, more often than
not, that is an indicator that the kubectl Crossplane you're using is outdated.
In other words some Crossplane API has been graduated from alpha to beta or stable
and the old plugin is not aware of this change.

You can follow the [install Crossplane CLI] instructions to upgrade the plugin.

## Resource Status and Conditions

Most Crossplane resources have a `status` section that can represent the current
state of that particular resource. Running `kubectl describe` against a
Crossplane resource will frequently give insightful information about its
condition. For example, to determine the status of a GCP `CloudSQLInstance`
managed resource, run:

```shell
kubectl describe cloudsqlinstance my-db
```

This should produce output that includes:

```console
Status:
  Conditions:
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Creating
    Status:                False
    Type:                  Ready
```

Most Crossplane resources set the `Ready` condition. `Ready` represents the
availability of the resource - whether it is creating, deleting, available,
unavailable, binding, etc.

## Resource Events

Most Crossplane resources emit _events_ when something interesting happens. You
can see the events associated with a resource by running `kubectl describe` -
e.g. `kubectl describe cloudsqlinstance my-db`. You can also see all events in a
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

> Note that Crossplane emits few logs by default - events are typically the best
> place to look for information about what Crossplane is doing. You may need to
> restart Crossplane with the `--debug` flag if you can't find what you're
> looking for.

## Provider Logs

Remember that much of Crossplane's functionality is provided by providers. You
can use `kubectl logs` to view provider logs too. By convention, they also emit
few logs by default.

```shell
kubectl -n crossplane-system logs <name-of-provider-pod>
```

All providers maintained by the Crossplane community mirror Crossplane's support
of the `--debug` flag. The easiest way to set flags on a provider is to create a
`ControllerConfig` and reference it from the `Provider`:

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: ControllerConfig
metadata:
  name: debug-config
spec:
  args:
    - --debug
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: crossplane/provider-aws:v0.18.1
  controllerConfigRef:
    name: debug-config
```

> Note that a reference to a `ControllerConfig` can be added to an already
> installed `Provider` and it will update its `Deployment` accordingly.

## Pausing Crossplane

Sometimes, for example when you encounter a bug, it can be useful to pause
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

## Pausing Providers

Providers can also be paused when troubleshooting an issue or orchestrating a
complex migration of resources. Creating and referencing a `ControllerConfig` is
the easiest way to scale down a provider, and the `ControllerConfig` can be
modified or the reference can be removed to scale it back up:

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: ControllerConfig
metadata:
  name: scale-config
spec:
  replicas: 0
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: crossplane/provider-aws:v0.18.1
  controllerConfigRef:
    name: scale-config
```

> Note that a reference to a `ControllerConfig` can be added to an already
> installed `Provider` and it will update its `Deployment` accordingly.

## Deleting When a Resource Hangs

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

For example, for a `CloudSQLInstance` managed resource (`database.gcp.crossplane.io`) named
`my-db`, you can remove its finalizer with:

```console
kubectl patch cloudsqlinstance my-db -p '{"metadata":{"finalizers": []}}' --type=merge
```

## Installing Crossplane Package

After installing [Crossplane package], to verify the install results or 
troubleshoot any issue spotted during the installation, there are a few things 
you can do.

Run below command to list all Crossplane resources available on your cluster:

```console
kubectl get crossplane
```

If you installed a Provider package, pay attention to the `Provider` and 
`ProviderRevision` resource. Especially the `INSTALLED` and `HEALTHY` column. 
They all need to be `TRUE`. Otherwise, there must be some errors that occurred
during the installation.

If you installed a Configuration package, pay attention to the `Configuration` 
and `ConfigurationRevision` resource. Again, the `INSTALLED` and `HEALTHY` 
column for these resources need to be `TRUE`. Besides that, you should also see 
the `CompositeResourceDefinition` and `Composition` resources included in this 
package are listed if the package is installed successfully.

If you only care about the installed packages, you can also run below command
which will show you all installed Configuration and Provider packages:

```console
kubectl get pkg
```

When there are errors, you can run below command to check detailed information 
for the packages that are getting installed.

```console
kubectl get lock -o yaml
```

To inspect a particular package for troubleshooting, you can run 
`kubectl describe` against the corresponding resources, e.g. the `Provider` and 
`ProviderRevision` resource for Provider package, or the `Configuration` and 
`ConfigurationRevision` resource for Configuration package. Usually, you should 
be able to know the error reason by checking the `Status` and `Events` field for 
these resources.

## Handling Crossplane Package Dependency

When using `crossplane.yaml` to define a Crossplane Configuration package, you 
can specify packages that it depends on by including `spec.dependsOn`. You can 
also specify version constraints for dependency packages.

When you define a dependency package, please make sure you provide the fully 
qualified address to the dependency package, but do not append the package 
version (i.e. the OCI image tag) after the package name. This may lead to the 
missing dependency error when Crossplane tries to install the dependency.

When specifying the version constraint, you should strictly follow the 
[semver spec]. Otherwise, it may not be able to find the appropriate version for 
the dependency package even it says the dependency is found. This may lead to an 
incompatible dependency error during the installation.

Below is an example where a Configuration package depends on a provider pulled 
from `crossplane/provider-aws`. It defines `">v0.16.0-0` as the version 
constraint which means all versions after `v0.16.0` including all prerelease 
versions, in the form of `-xyz` after the normal version string, will be 
considered when Crossplane tries to find the best match.

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: test-configuration
  annotations:
    provider: aws
spec:
  crossplane:
    version: ">=v1.0.0-0"
  dependsOn:
    - provider: crossplane/provider-aws
      version: ">v0.16.0-0"
```

<!-- Named Links -->

[Requested Resource Not Found]: #requested-resource-not-found
[install Crossplane CLI]: ../getting-started/install-configure.md#install-crossplane-cli
[Resource Status and Conditions]: #resource-status-and-conditions
[Resource Events]: #resource-events
[Crossplane Logs]: #crossplane-logs
[Provider Logs]: #provider-logs
[Pausing Crossplane]: #pausing-crossplane
[Pausing Providers]: #pausing-providers
[Deleting When a Resource Hangs]: #deleting-when-a-resource-hangs
[Installing Crossplane Package]: #installing-crossplane-package
[Crossplane package]: https://crossplane.io/docs/v1.3/concepts/packages.html
[Handling Crossplane Package Dependency]: #handling-crossplane-package-dependency
[semver spec]: https://github.com/Masterminds/semver#basic-comparisons
