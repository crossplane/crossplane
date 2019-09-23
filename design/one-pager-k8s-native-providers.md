# Kubernetes-Native Providers
* Owner: Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Introduction

Currently, all infrastructure provider stacks manage resources by creating an
external resource and keeping it in sync with an instance of a CRD (managed
resource) in the Crossplane control cluster. CRUD operations are executed on
external resources using the API provided by the infrastructure provider.
However, there are a number of infrastructure providers, with more being created
everyday, that expose their API's via the Kubernetes API. This document proposes
a common pattern for stacks that utilize the Kubernetes client as their API.
Motivation for this design comes from the desire to support [Rook] as a provider
in Crossplane.

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

As an example, Rook provides the option to run [CockroachDB] in a Kubernetes
cluster. Assuming you have installed Rook and the corresponding CockroachDB
operator into a cluster, you should be able to dynamically provision a
CockroachDB cluster into the Kubernetes cluster from your Crossplane control
cluster. The experience would look as follows:

1. Create a non-portable Rook CockroachDB `ClusterClass`

```yaml
apiVersion: cockroachdb.rook.crossplane.io/v1alpha1
kind: ClusterClass
metadata:
  name: rook-cockroachdb
  namespace: rook-infra-dev
specTemplate:
  clusterSelector:
    matchLabels:
        app: my-cool-cluster
  # full documentation on all available settings can be found at:
  # https://rook.io/docs/rook/master/cockroachdb-cluster-crd.html
  scope:
    nodeCount: 3
    # You can only have one PersistentVolumeClaim in this list!
    volumeClaimTemplates:
    - metadata:
        name: rook-cockroachdb-data
      spec:
        accessModes: [ "ReadWriteOnce" ]
        # Uncomment and specify your StorageClass, otherwise
        # the cluster admin defined default StorageClass will be used.
        #storageClassName: "your-cluster-storageclass"
        resources:
          requests:
            storage: "1Gi"
  network:
    ports:
    - name: http
      port: 8080
    - name: grpc
      port: 26257
  secure: false
  cachePercent: 25
  maxSQLMemoryPercent: 25
  # A key/value list of annotations
  annotations:
  #  key: value
```

2. Create a portable `PostgreSQLInstanceClass`

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstanceClass
metadata:
  name: postgresql-standard
  namespace: app-project1-dev
classRef:
  kind: ClusterClass
  apiVersion: cockroachdb.rook.crossplane.io/v1alpha1
  name: rook-cockroachdb
  namespace: rook-infra-dev
```

3. Create a `PostgreSQLInstance` claim

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: app-postgresql
  namespace: app-project1-dev
spec:
  classRef:
    name: postgresql-standard
  writeConnectionSecretToRef:
    name: postgresqlconn
  engineVersion: "9.6"
```

On creation of the `PostgreSQLInstance` claim, the claim controller would
configure and create a `Cluster` (`cockroachdb.rook.crossplane.io/v1alpha1`)
instance. Then, the managed controller would use the Crossplane `Cluster`
instance to create a `Cluster` (`cockroachdb.rook.io/v1alpha1`) in the target
Kubernetes cluster where Rook and the CockroachDB operator are installed. This
workflow is similar to dynamic provisioning in a typical infrastructure
provider:

![K8s Native Providers](./images/k8s-native-providers.png)

## Technical Design

### Claim Reconciler

The claim controllers could use the same shared [claim reconciler] that other
existing infrastructure stacks use currently, providing their own
`ManagedConfigurators` to configure the managed resource instance using the
referenced class.

### Scheduler Reconciler

Because the scheduling behavior for most Kubernetes-native infrastructure
components will look very similar, a shared scheduler reconciler should be added
to [crossplane-runtime]. It will look almost identical to the reconciler used
for `KubernetesApplication` [scheduler controller]. In fact, the
`KubernetesApplication` scheduler controller should likely make use of the
shared `crossplane-runtime` scheduler reconciler after it is implemented.

### Managed Reconciler

Though this design is motivated by the desire to support Rook, the pattern can
be generalized to be applicable to other Kubernetes-native infrastructure
providers. While most existing infrastructure provider stacks must create a
client wrapper library to use the CRUD functions of the provider API, every
Kubernetes-native infrastructure stack will use the [client-go] SDK for CRUD
operations. Therefore, the pluggable `Observe`, `Create`, `Update`, `Delete`
functions in the [crossplane-runtime] can likely have a common implementation
for Kubernetes-native infrastructure stacks.

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

Implementation of the shared managed reconciler for the managed CockroachDB
`Cluster` controller (`cockroachdb.rook.crossplane.io/v1alpha1`) if a
`nativeExternal` type was able to be utilized could look as follows:

```go
func (c *ClusterController) SetupWithManager(mgr ctrl.Manager) error {
  r := resource.NewManagedReconciler(mgr,
    resource.ManagedKind(v1alpha1.ClusterGroupVersionKind),
    resource.WithExternalConnecter(&resource.NativeConnecter{client: mgr.GetClient(), kind: v1alpha1.Cluster}))

  name := strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.ClusterGroupKind, v1alpha1.Group))

  return ctrl.NewControllerManagedBy(mgr).
    Named(name).
    For(&v1alpha1.Cluster{}).
    Complete(r)
}
```


[Rook]: https://github.com/rook/rook
[complex workloads]: design-doc-complex-workloads.md
[CockroachDB]: https://github.com/cockroachdb/cockroach
[client-go]: https://github.com/kubernetes/client-go
[claim reconciler]: https://github.com/crossplaneio/crossplane-runtime/blob/master/pkg/resource/claim_reconciler.go
[scheduler controller]: https://github.com/crossplaneio/crossplane/blob/master/pkg/controller/workload/kubernetes/scheduler/scheduler.go
[crossplane-runtime]: https://github.com/crossplaneio/crossplane-runtime