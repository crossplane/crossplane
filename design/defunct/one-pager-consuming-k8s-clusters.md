# Consuming Kubernetes Clusters
* Owner: Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Terminology

* Kubernetes cluster **managed resource**: a cluster-scoped custom resource that
  represents the existence and configuration of a specific cloud provider's
  managed Kubernetes service (e.g. GKE, AKS, EKS).
* `KubernetesCluster` claim: a namespace-scoped custom resource that can be used
  to claim a Kubernetes cluster managed resource, or dynamically provision one.
* **Workload**: a unit of work that is intended to be scheduled to a resource on
  which it can be executed. Currently, the only type of workload in Crossplane
  is a `KubernetesApplication`, which is a collection of Kubernetes resources
  that are intended to be scheduled to and executed on a target Kubernetes
  cluster.

## Background

Currently, Crossplane models portable cloud provider managed services as
*managed resources* that can be either statically or dynamically provisioned
using resource *classes* and *claims*. One such resource type is a Kubernetes
cluster, which uses a `KubernetesCluster` claim and cloud provider-specific
managed resource types (e.g. `GKECluster`, `AKSCluster`, `EKSCluster`, etc.). It
is appropriate for Kubernetes clusters to be modeled in the same manner as other
managed services due to the fact that they are presented by the provider in the
same fashion. However, in Crossplane, Kubernetes clusters are unique to other
managed service offerings in that they can also be used as a target for running
workloads.

Workloads are provisioned using the `KubernetesApplication` type. It is
namespace-scoped and is scheduled to a target cluster using labels to match a
`KubernetesCluster` claim in its namespace. Because managed resources may only
be bound to a single claim, the claim is namespaced, and `KubernetesApplication`
resources can only be scheduled to `KubernetesCluster` claims in their
namespace, any Kubernetes managed resource offering may only be consumed (i.e.
deployed to) from a single namespace.

In order to fully conceptualize the usage of Kubernetes clusters in Crossplane,
it is helpful to separate the notions of a **Kubernetes cluster as a managed
service** and a **Kubernetes cluster as a target for deploying workloads**. With
this distinction made, we can assert that this proposal in no way modifies the
managed service notion of a Kubernetes cluster, but instead formalizes this
concept of a Kubernetes cluster as a deployment target.

## Goals

Because it is unlikely that Crossplane users will always want a 1-to-1
namespace-to-cluster consumption model, it is desirable to support deploying
workloads in any namespace to any Kubernetes cluster. However, it is also
desirable to continue to treat managed Kubernetes services as a managed resource
that is supplied by cloud providers. For this reason, any solution should be
additive to the existing model.

As a secondary goal for this design, it should be possible to use existing
Kubernetes clusters (wherever they may be running) that were not provisioned by
Crossplane as targets for deploying workloads. This concept has frequently been
referred to as a "Bring Your Own Cluster" scenario.

## Proposal

This proposal can be separated into three main categories:
1. The implementation of a new `KubernetesTarget` custom resource and
   corresponding controllers.
1. The modification of the scheduling behavior for `KubernetesApplication`.
1. The implementation of a controller that automatically creates a
   `KubernetesTarget` resource when a `KubernetesClaim` is bound to a Kubernetes
   cluster managed resource. 

### KubernetesTarget

The purpose of the `KubernetesTarget` resource is to effectively "publish"
Kubernetes clusters for usage in a namespace. It is namespace-scoped and can
only reference a cluster-scoped Kubernetes cluster managed resource in its
`clusterRef` field, which will result in the setting of its
`connectionSecretRef` field, or a local namespace-scoped `Secret` directly in
the `connectionSecretRef` field. If both the `clusterRef` field and
`connectionSecretRef` field are populated at time of creation, the
`connectionSecretRef` field will take precedence. The creation of a
`KubernetesTarget` resources will be watched by controllers in stacks that
provide Kubernetes cluster managed resources (i.e. a specific controller must be
implemented in each stack that wishes to provide a Kubernetes cluster managed
resource that `KubernetesApplication` resources can be scheduled to). These
controllers will check to see if the `KubernetesTarget` references their cluster
type n its `clusterRef` and, if so, will propagate the cluster's connection
`Secret` to the namespace of the `KubernetesTarget`. A local object reference to
the `Secret` will then be set on the `KubernetesTarget` object in the
`connectionSecretRef` field. If a `KubernetesTarget` is created with a reference
to a local `Secret`, it will be ignored by these controllers as the required
`Secret` must already be present in the namespace. An example of a
`KubernetesTarget` in namespace `my-app-team` that is referencing a GKE cluster
managed resource could look as follows:

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesTarget
metadata:
  namespace: my-app-team
  name: enable-dev-k8s
  labels:
    env: dev
spec:
  clusterRef:
    kind: GKECluster
    apiVersion: compute.gcp.crossplane.io/v1alpha3
    name: dev-k8s-cluster
```

Upon creation, the connection `Secret` associated with `dev-k8s-cluster` will be
propagated to the `my-app-team` namespace and a reference to it will be set in
the `connectionSecretRef` field of the `KubernetesTarget`.

Note that `kind` and `apiVersion` are required for a `KubernetesTarget` that
utilizes the `clusterRef` field because Crossplane cannot be knowledgeable of
every Kubernetes cluster managed resource type that may have been installed into
the cluster.

An example of a `KubernetesTarget` in namespace `my-app-team` that is
referencing a local `Secret` directly could look as follows:

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesTarget
metadata:
  namespace: my-app-team
  name: enable-dev-k8s
  labels:
    env: dev
spec:
  connectionSecretRef:
    name: my-k8s-connection
```

Because `KubernetesTarget` resources can point directly to a local `Secret`, any
existing Kubernetes cluster that was not provisioned using Crossplane (including
on-prem clusters) can now be targeted for provisioning workloads. To enable this
functionality, a user would manually create a `Secret` containing the
`kubeconfig` information for their cluster in the `my-app-team` namespace, then
create a `KubernetesTarget` that references it.

### KubernetesApplication Scheduling

As mentioned above, `KubernetesApplication` resources are currently scheduled to
`KubernetesCluster` claims in their namespace. With the introduction of the
`KubernetesTarget`, `KubernetesApplication` resources can still be scheduled via
label selectors, but will now be scheduled to a `KubernetesTarget` in their
namespace, then use its `connectionSecretRef` to get the `Secret` that was
propagated to the namespace from the Kubernetes cluster managed resource or
already existed due to manual creation. Importantly, for a
`KubernetesApplication` to be able to be scheduled to a Kubernetes cluster, a
`KubernetesTarget` that points to that cluster managed resource or local
`Secret` *must* be present in the namespace of the `KubernetesApplication`.

This change isolates the actions of *provisioning* (`KubernetesCluster` or
cloud-specific managed resource), *publishing* (`KubernetesTarget`), and
*consuming* (`KubernetesApplication`) Kubernetes clusters, actions which may be
executed by [almost disjoint] sets of users. Having these actions represented by
separate custom resources allows for more granular [RBAC] permissions around
Kubernetes cluster usage in Crossplane.

### Automatic KubernetesTarget Creation Controller

Because the ability to *publish* a Kubernetes cluster provides immense power in
a Crossplane environment, it is desirable for Kubernetes cluster managed
resources to be able to be consumed in the namespace that they are claimed
without a user requiring the ability to create a `KubernetesTarget`. For
instance, if an infrastructure owner wants to allow for an application team to
provision some clusters dynamically by creating a `KubernetesCluster` claim in
their namespace that references a `GKEClusterClass`, that application team
should be able to then schedule `KubernetesApplication` resources to that
cluster without requiring the permissions to create a `KubernetesTarget` (which
would allow them to enable to consumption of any Kubernetes cluster managed
resource) or asking the infrastructure owner to create a `KubernetesTarget` for
them (which would break the self-service model).

To enable this functionality, a single controller should be implemented in core
Crossplane that watches for the creation of `KubernetesCluster` claim resources
and automatically creates a `KubernetesTarget` with the `connectionSecretRef`
set to the connection `Secret` that was propagated to the namespace.
Importantly, this controller should also clean up the `KubernetesTarget` when
the `KubernetesCluster` (and corresponding) `Secret` are deleted. This can be
accomplished by setting an `ownerReference` to the `KubernetesCluster` claim on
the `KubernetesTarget`. In addition, because this `KubernetesTarget` resource
may be scheduled to by a `KubernetesApplication` using labels, any labels that
are present on the `KubernetesCluster` claim should be propagated to the
`KubernetesTarget` resource and the name of the resource should match that of
the claim.

### User Workflows

The following scenarios serve to illustrate possible workflows that would be
enabled by this model. While some of these scenarios appear less likely than
others, the primary takeaway is the flexibility of this model, which allows for
a variety of use-cases.

#### Scenario 1: Infrastructure owner provisions and publishes all Kubernetes clusters

1. Infrastructure owner statically provisions three `GKECluster` managed
   resources: `dev`, `stage`, and `prod`
1. An application owner asks for an environment to deploy their application.
1. Infrastructure owner creates a new namespace and a `KubernetesTarget` that
   points to the `dev` cluster.
1. Application owner provisions a `KubernetesApplication` in that namespace with
   labels that select the `KubernetesTarget` referencing the `dev` cluster for
   provisioning.
1. When ready for testing, the application owner requests access to the `stage`
   cluster, which the infrastructure owner then "publishes" by creating another
   `KubernetesTarget` in the namespace that points to the `stage` cluster.
1. Application owner provisions a `KubernetesApplication` with labels that
   select the `KubernetesTarget` referencing the `stage` cluster for
   provisioning.

In this scenario, there is a strict separation of concern between infrastructure
owner and application owner.

#### Scenario 2: Infrastructure owner provisions and publishes some Kubernetes, but also enables self-service

1. Infrastructure owner statically provisions two `GKECluster` managed
   resources: `stage`, and `prod`. They also create a `GKEClusterClass` with
   configuration for a `dev` cluster.
1. An application owner asks for an environment to deploy their application.
1. Infrastructure owner creates a new namespace.
1. Application owner creates a `KubernetesCluster` claim in the namespace that
   references the `dev` `GKEClusterClass` and provisions a new Kubernetes
   cluster. This causes the creation of a `KubernetesTarget` in the namespace
   when the connection `KubernetesCluster` claim becomes `Bound`.
1. Application owner provisions a `KubernetesApplication` in that namespace with
   labels that select the automatically created `KubernetesTarget`.
1. When ready for testing, the application owner requests access to the `stage`
   cluster, which the infrastructure owner then "publishes" by creating a
   `KubernetesTarget` in the namespace that points to the `stage` cluster.
1. Application owner provisions a `KubernetesApplication` with labels that
   select the `KubernetesTarget` referencing the `stage` cluster for
   provisioning.

In this scenario, long-lived "pet" clusters that may be more vital to the
organization are completely managed and published by the infrastructure owner.
However, for development work, application owners are provided the ability to
create and delete small clusters as needed using configuration supplied by the
infrastructure owner (i.e. "clusters as cattle").

#### Scenario 3: Infrastructure owner enables self-service, except for one on-prem cluster with sensitive data

1. Infrastructure owner creates a variety of Kubernetes cluster classes with
   configuration for different environments and different cloud providers.
   Application owners are given permissions to create clusters using these
   classes as they see fit.
1. There is one particular set of workloads that is required to be run in an
   existing on-premises cluster (for some important reason).
1. Application owner says that they need to run a sensitive workload on that
   cluster. Infrastructure owner creates a new namespace called
   `super-secret-workloads` and creates a `Secret` with the on-prem cluster's
   `kubeconfig` information. They also create a `KubernetesTarget` in that
   namespace that references the `Secret`.
1. Application owner provisions a `KubernetesApplication` with labels that
   select that `KubernetesTarget` for provisioning.

This example shows the power of this model, as users are now able to consume
clusters that were provisioned with Crossplane as well as those that may be
pre-existing or cannot be managed by Crossplane.

## Technical Implementation

The implementation of this proposal is relatively lightweight and requires
minimal churn on any existing exposed APIs. The first step would be implementing
the `KubernetesTarget` type in [core Crossplane]. As mentioned above, in
addition to the creation of the custom resource, a new controller would also be
required in each stack to watch `KubernetesTarget` object creation and propagate
`Secret` objects to its namespace if its `clusterRef` references a Kubernetes
cluster managed resource type supplied by that stack. These controllers should
use [predicates] to only reconcile `KubernetesTarget` objects that reference
their resource type and do not already have a `connectionSecretRef` set, and
should refuse to propagate a `Secret` if the `KubernetesTarget` references a
managed resource that is not `Bound`. The majority of this logic can be shared
across controllers by adding a new reconciler to [crossplane-runtime].

The next step would be to modify the `KubernetesApplication` [scheduling
controller] to list `KubernetesTarget` resources in the namespace that match
labels, instead of the current implementation that lists `KubernetesCluster`
claims. Currently, this controller sets a reference to a `KubernetesCluster`
claim, which is then later propagated to the child
`KubernetesApplicationResource` objects, which use it to get a `Secret` with
[remote cluster connection information]. The same process can be achieved when
referencing a `KubernetesTarget` resource with a reference to a local `Secret`
with connection information. Because a `KubernetesApplication` is now being
scheduled to something other than a `KubernetesCluster` claim (and may be
scheduled to other resource types in the future), updating the `clusterRef`
field to be named `targetRef` and `clusterSelector` to `targetSelector` would be
more clear. This is the only API change proposed by this design and will result
in the increment of the `KubernetesApplication` `apiVersion` from `v1alpha1` to
`v1alpha2`.

Lastly, a single controller must be added to core Crossplane that watches for
the creation of `KubernetesCluster` claims and automatically creates a
`KubernetesTarget` resource in the namespace when the `KubernetesCluster` claim
becomes `Bound`. As mentioned above, the created `KubernetesTarget` should have
an `ownerReference` to the `KubernetesCluster` claim to ensure it is cleaned up
by [garbage collection] when the claim is deleted.

## Future Considerations

Because the scheduling of `KubernetesApplication` resources is now isolated to
target the `KubernetesTarget` resource, more intelligent scheduling can be
enabled without touching other parts of the Crossplane ecosystem. Previously, a
`KubernetesCluster` claim was used for claiming, consuming, and dynamically
provisioning Kubernetes cluster resources so changes to the API type related
to scheduling (i.e. consuming) could unintentionally affect those other
capabilities as well. Potential future scheduling improvements could involve
price, latency, and geographic optimization by surfacing additional fields or
labels on the `KubernetesTarget` type.

<!-- Named Links -->
[almost disjoint]: https://en.wikipedia.org/wiki/Almost_disjoint_sets
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[core Crossplane]: https://github.com/crossplane/crossplane
[scheduling controller]: https://github.com/crossplane/crossplane/blob/2cab9826a1d5088a87b5d24693336d228279a3a7/pkg/controller/workload/kubernetes/scheduler/scheduler.go#L57
[remote cluster connection information]: https://github.com/crossplane/crossplane/blob/2cab9826a1d5088a87b5d24693336d228279a3a7/pkg/controller/workload/kubernetes/resource/resource.go#L401
[predicates]: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/predicate
[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime
[garbage collection]: https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/