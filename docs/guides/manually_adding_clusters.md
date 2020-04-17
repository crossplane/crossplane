---
title: Manually Adding Existing Kubernetes Clusters
toc: true
weight: 201
indent: true
---

# Manually Adding Existing Kubernetes Clusters

As discussed in the section on [scheduling applications to remote clusters],
Crossplane allows you to import existing Kubernetes clusters for scheduling.
This can be done for any cluster for which you have a `kubeconfig`. Crossplane
will be given the same permissions to the remote cluster that are provided to
the user in your `kubeconfig`.

The first step is creating a `Secret` with the base64 encoded data of your
`kubeconfig`. This can be done with the following command (assumes data is in
`kubeconfig.yaml` and desired namespace is `cp-infra-ops`):

```
kubectl -n cp-infra-ops create secret generic myk8scluster --from-literal=kubeconfig=$(base64 kubeconfig.yaml -w 0)
```

In order to use this cluster as a scheduling target, you must also create a
`KubernetesTarget` resource that references the `Secret`. Make sure to include
any labels that you want to schedule your `KubernetesApplication` to:

`myk8starget.yaml`

```
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesTarget
metadata:
  name: myk8starget
  namespace: cp-infra-ops
  labels:
    guide: infra-ops
spec:
  connectionSecretRef:
    name: myk8scluster
```

```
kubectl apply -f myk8starget.yaml
```

*Note: the `Secret` and `KubernetesTarget` must be in the same namespace.*

You can now create a `KubernetesApplication` in the `cp-infra-ops` namespace and
your remote cluster will be a scheduling option.

<!-- Named Links -->
[scheduling applications to remote clusters]: ../workload.md
