# Kubernetes-Native Providers
* Owner: Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Introduction

Currently, all infrastructure provider stacks manage resources by creating an
external resource and keeping it in sync with a Kubernetes custom resource
(managed resource) in the Crossplane control cluster. CRUD operations are
executed on external resources using the API provided by the infrastructure
provider. However, there are a number of Kubernetes-native infrastructure
providers, with more being created everyday, that only expose their API's via
the Kubernetes API. As opposed to provisioning resources externally on
provider-owned infrastructure, a Kubernetes-native provider provisions resources
in-cluster, meaning that they can run on-premises or anywhere else you may be
running a Kubernetes cluster. This document proposes a common pattern for stacks
that utilize the Kubernetes client as their API. Motivation for this design
comes from the desire to support [Rook] as a provider in Crossplane.

Using the Kubernetes API to create objects in a controller's reconcile loop is
not a foreign concept in Crossplane. Existing claim reconcilers create a
Kubernetes object to represent the external resource. However, Kubernetes-native
providers differ in that the external resource is actually a Kubernetes object
itself. This concept is also not new in Crossplane, as [complex workloads]
operate in this manner. `KubernetesApplication` instances are first scheduled to
a Kubernetes cluster by the scheduler controller, which causes them to pass the
predicates of the application controller. It uses the template(s) in the
`KubernetesApplication` instance to create one or more
`KubernetesApplicationResource` objects, which are then picked up by the
resource scheduler. It is responsible for actually creating the
Kubernetes-native (e.g. `Deployment`, `ConfigMap`, etc.) objects in the target
cluster, then managing them as a managed reconciler does for an external
resource. In this scenario, the Kubernetes-native objects are the external
resources.

## Goals

* Create a common pattern that can be used across infrastructure stacks (and
  potentially the existing complex workload controllers) that utilize the
  Kubernetes API as the infrastructure provider API
* Demonstrate the usage of this pattern with Rook

## Differences from `KubernetesApplication`

While the `KubernetesApplication` pattern detailed above provides a starting
point for this design, a key difference is that `KubernetesApplication` does not
adhere to the separation of concern structure of other Crossplane resources
because Kubernetes resources are not modeled in a `claim <-> portable class <->
non-portable class <-> managed <-> external` workflow. Kubernetes-native
infrastructure resources, on the other hand, *should* be modeled with the same
separation of concern that other cloud-provider resources expose.

## Workflow Design

The general design from a user experience perspective should differ very little
from utilizing any other infrastructure provider stack. Developers should be
able to request the creation of an abstract resource type via a claim, and
cluster operators should be able to define the classes of service available to
satisfy that claim kind.

As an example, Rook provides the option to provision [Minio] object storage in a
Kubernetes cluster. Assuming you have installed Rook and the corresponding Minio
operator into a cluster, you should be able to dynamically provision a Minio
object storage cluster into the Kubernetes cluster from your Crossplane control
cluster. The experience would look as follows:

1. Create a Kubernetes `Provider` for target cluster:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: demo-provider-rook
  namespace: rook-infra-dev
type: Opaque
data:
  endpoint: MY_K8S_ENDPOINT
  username: MY_K8S_USERNAME
  password: MY_K8S_PASSWORD
  clusterCA: MY_K8S_CLUSTER_CA
  clientCert: MY_K8S_CLIENT_CERT
  clientKey: MY_K8S_CLIENT_KEY
  token: MY_K8S_TOKEN
---
apiVersion: kubernetes.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: demo-kubernetes
  namespace: azure-infra-dev
spec:
  credentialsSecretRef:
    name: demo-provider-rook
```

2. Create a non-portable Rook Minio `ObjectStoreClass`

```yaml
apiVersion: minio.rook.crossplane.io/v1alpha1
kind: ObjectStoreClass
metadata:
  name: rook-minio
  namespace: rook-infra-dev
spec:
  scope:
    nodeCount: 4
    volumeClaimTemplates:
    - metadata:
        name: rook-minio-data1
      spec:
        accessModes: [ "ReadWriteOnce" ]
        resources:
          requests:
            storage: "8Gi"
  # A key value list of annotations
  annotations:
  #  key: value
  placement:
    tolerations:
    nodeAffinity:
    podAffinity:
    podAnyAffinity:
  credentials:
    name: minio-my-store-access-keys
    namespace: rook-minio
  providerRef:
    name: demo-kubernetes
    namespace: rook-infra-dev
```

3. Create a portable `BucketClass`

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: BucketClass
metadata:
  name: bucket-standard
  namespace: app-project1-dev
classRef:
  kind: ObjectStoreClass
  apiVersion: minio.rook.crossplane.io/v1alpha1
  name: rook-minio
  namespace: rook-infra-dev
```

4. Create a `Bucket` claim

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: Bucket
metadata:
  name: app-bucket
  namespace: app-project1-dev
spec:
  classRef:
    name: bucket-standard
  writeConnectionSecretToRef:
    name: bucketsecret
```

On creation of the `Bucket` claim, the claim controller would configure and
create an `ObjectStore` (`minio.rook.crossplane.io/v1alpha1`) instance. Then,
the managed controller would use the Crossplane `ObjectStore` instance to create
an `ObjectStore` (`minio.rook.io/v1alpha1`) in the target Kubernetes cluster
where Rook and the Minio operator are installed. This workflow is similar to
dynamic provisioning in a typical infrastructure provider:

![K8s Native Providers](./images/k8s-native-providers.png)

## Technical Design

### Claim Reconciler

The claim controllers could use the same shared [claim reconciler] that other
existing infrastructure stacks use currently, providing their own
`ManagedConfigurators` to configure the managed resource instance using the
referenced class.

### Managed Reconciler

Kubernetes-native infrastructure providers can make use of the shared managed
reconciler in `crossplane-runtime` in the same manner as traditional
infrastructure providers. However, instead of creating a new client for a cloud
provider API, Kubernetes-native infrastructure providers will return a
Kubernetes client that is configured to talk to the target cluster using the
`providerRef`. This is similar to the client configuration for the
`KubernetesApplicationResource` [managed reconciler], except that it obtains
Kubernetes credentials using a `Provider` object rather than a
`KubernetesClusterObject`. This `Provider` type
(`provider.kubernetes.crossplane.io`) should likely exist outside of the Rook
stack (i.e. in core Crossplane) due to its generic applicability. 

## Future Considerations

### More Sophisticated Scheduling

This initial proposal embeds a `proivderRef` field in the non-portable classes
of a Kubernetes-native provider that specifies the cluster in which resources
should be provisioned. This mitigates the need for a scheduler, but requires
that a `Provider` object be created in order to talk to any Kubernetes cluster,
even if that cluster was provisioned by Crossplane. In the case that the cluster
was provisioned by Kubernetes, the `Secret` the the `Provider` references should
already exist in the control cluster.

It is possible that we will want to be able to schedule Kubernetes-native
infrastructure resources in a more intelligent manner. This could include
implementing scheduling policies that allow for resources to be provisioned
based on geography, resource requirements, etc. An initial scheduler
implementation may look similar to the `KubernetesApplication` [scheduler
controller], but instead of matching `KubernetesCluster` objects by label, it
would use `provider.kubernetes.crossplane.io` objects. Because the scheduling
behavior for most Kubernetes-native infrastructure components will look very
similar, a shared scheduler reconciler should be added to [crossplane-runtime].
In fact, the `KubernetesApplication` scheduler controller may also be able to
make use of the shared `crossplane-runtime` scheduler reconciler after it is
implemented.

### Generalized Managed Reconciler

Though this design is motivated by the desire to support Rook, the pattern can
be generalized to be applicable to other Kubernetes-native infrastructure
providers. While most existing infrastructure provider stacks must create a
client wrapper library to use the CRUD functions of the provider API, every
Kubernetes-native infrastructure stack will use the [client-go] SDK for CRUD
operations. Therefore, the pluggable `Observe`, `Create`, `Update`, `Delete`
functions in the [crossplane-runtime] can likely have a common implementation
for some Kubernetes-native infrastructure stacks.

This can be done via generic `NativeConnecter` and `nativeExternal` types in
`crossplane-runtime`. Similar to the `clusterConnecter` in the
`KubernetesApplicationResource` controller, it would get the cluster connection
information of the Kubernetes cluster that the managed resource was scheduled to
by the scheduler controller, then return a `nativeExternal` type that implements
the `ExternalClient` interface. Because external client functions will
essentially be making sure the "external resource" in the target cluster matches
the "managed resource" (i.e. they should be nearly identical Kubernetes
resources), it should be possible to have generic implementations of the
`ExternalClient` functions. However, if it proves difficult to do so, or it just
does not work for a specific resource type, manual implementation of the
`connecter` and `external` types would always be possible, just as they are for
existing infrastructure stacks.

Implementation of the shared managed reconciler for the managed Minio
`ObjectStore` controller (`minio.rook.crossplane.io/v1alpha1`) if a
`nativeExternal` type was able to be utilized could look as follows:

```go
func (c *ObjectStoreController) SetupWithManager(mgr ctrl.Manager) error {
  r := resource.NewManagedReconciler(mgr,
    resource.ManagedKind(v1alpha1.ObjectStoreGroupVersionKind),
    resource.WithExternalConnecter(&resource.NativeConnecter{client: mgr.GetClient(), kind: v1alpha1.ObjectStore}))

  name := strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.ObjectStoreGroupKind, v1alpha1.Group))

  return ctrl.NewControllerManagedBy(mgr).
    Named(name).
    For(&v1alpha1.ObjectStore{}).
    Complete(r)
}
```

### Automatic Operator Installation

For any Kubernetes-native provider, there must be an operator installed in the
target cluster for the resources to be provisioned. However, it would be nice to
be able to provision resources in a cluster where the operator is not currently
installed. This could be implemented by checking if the operator exists in the
target cluster in the managed reconciler's `Create` method, and installing it if
not. This may not be part of initial implementation, but should definitely be
tracked and supported.

## Relevant Issues

- [crossplane-runtime #22]: Allow existing resources to be "imported" into Crossplane to be managed
- [crossplane-runtime #34]: Automatically install required operators in target cluster if not present

[Rook]: https://github.com/rook/rook
[complex workloads]: design-doc-complex-workloads.md
[Minio]: https://min.io/
[client-go]: https://github.com/kubernetes/client-go
[managed reconciler]: https://github.com/crossplaneio/crossplane/blob/14fa6dda6a3e91d5f1ac98d1020a151b02311cb1/pkg/controller/workload/kubernetes/resource/resource.go#L401
[claim reconciler]: https://github.com/crossplaneio/crossplane-runtime/blob/master/pkg/resource/claim_reconciler.go
[scheduler controller]: https://github.com/crossplaneio/crossplane/blob/master/pkg/controller/workload/kubernetes/scheduler/scheduler.go
[crossplane-runtime]: https://github.com/crossplaneio/crossplane-runtime
[crossplane-runtime #22]: https://github.com/crossplaneio/crossplane-runtime/issues/22
[crossplane-runtime #34]: https://github.com/crossplaneio/crossplane-runtime/issues/34