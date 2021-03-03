---
title: Uninstall
toc: true
weight: 303
indent: true
---

# Uninstalling

Crossplane has a number of components that must be cleaned up in order to
guarantee proper removal from the cluster. When deleting objects, it is best to
consider parent-child relationships and clean up the children first to ensure
the proper action is taken externally. For instance, cleaning up `provider-aws`
before deleting an `RDSInstance` will result in the RDS instance remaining
provisioned on AWS as the controller responsible for cleaning it up will have
already been deleted. It will also result in the `RDSInstance` CRD remaining in
the cluster, which could make it difficult to re-install the same provider at a
future time.

## Deleting Resources

If you wish for all claims (XRC), composite resources (XR), and managed
resources to have deletion handled properly both in the cluster in externally,
they should be deleted and no longer exist in cluster before the package that
extended the Kubernetes API to include them is uninstalled. You can use the
following logic to clean up resources effectively:

- If an XRC exists for a given XR and set of managed resources, delete the XRC
  and both the XR and managed resources will be cleaned up.
- If only an XR exists for a given set of managed resources, delete the XR and
  each of the managed resources will be cleaned up.
- If a managed resource was provisioned directly, delete it directly.

The following commands can be used to identify existing XRC, XR, and managed
resources:

- XRC: `kubectl get claim`
- XR: `kubectl get composite`
- Managed Resources: `kubectl get managed`

Crossplane controllers add [finalizers] to resources to ensure they are handled
externally before they are fully removed from the cluster. If resource deletion
hangs it is likely due to a delay in the resource being handled externally,
causing the controller to wait to remove the finalizer. If this persists for a
long period of time, use the [troubleshooting guide] to fix the issue.

## Uninstall Packages

Once all resources are cleaned up, it is safe to uninstall packages.
`Configuration` packages can typically be deleted safely with the following
command:

```console
kubectl delete configuration.pkg <configuration-name>
```

Before deleting `Provider` packages, you will want to make sure you have deleted
all `ProviderConfig`s you created. An example command if you used AWS Provider:

```console
kubectl delete providerconfig.aws --all
```

Now you are safe to delete the `Provider` package:

```console
kubectl delete provider.pkg <provider-name>
```

## Uninstall Crossplane

When all resources and packages have been cleaned up, you are safe to uninstall
Crossplane:

```console
helm delete crossplane --namespace crossplane-system

kubectl delete namespace crossplane-system
```

Helm does not delete CRD objects. You can delete the ones Crossplane created
with the following commands:

```console
kubectl patch lock lock -p '{"metadata":{"finalizers": []}}' --type=merge
kubectl get crd -o name | grep crossplane.io | xargs kubectl delete
```

<!-- Named Links -->

[finalizers]: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#finalizers
[troubleshooting guide]: troubleshoot.md
