# Consuming Kubernetes Clusters
* Owner: Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Terminology

* Kubernetes `Provider`: a cluster-scoped custom resource that points to any
  Kubernetes `Secret` that contains connection information for a Kubernetes
  cluster.
* Kubernetes cluster **managed resource**: a cluster-scoped custom resource that
  represents the existence and configuration of a specific cloud provider's
  managed Kubernetes service (e.g. GKE, AKS, EKS).
* `KubernetesCluster` claim: a namespace-scoped custom resource that can be used
  to claim a Kubernetes cluster managed resource, or dynamically provision one.
* **Workload**: a collection of Kubernetes resources that are intended to be
  deployed into a target Kubernetes cluster. A workload is represented by a
  `KubernetesApplication` custom resource.

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

This proposal can be separated into two main categories:
1. The implementation of a new `WorkloadTarget` custom resource.
1. The modification of the scheduling behavior for `KubernetesApplication`.

### WorkloadTarget

The purpose of the `WorkloadTarget` resource is to effectively "publish"
Kubernetes clusters for usage in a namespace. It is namespace-scoped and can
only reference Kubernetes cluster managed resources or the Kubernetes `Provider`
type, both of which are cluster-scoped. An example of a `WorkloadTarget` in
namespace `my-app-team` that is referencing a GKE cluster managed resource could
look as follows:

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: WorkloadTarget
metadata:
  namespace: my-app-team
  name: enable-dev-k8s
  labels:
    env: dev
clusterRef:
  kind: GKECluster
  apiVersion: compute.gcp.crossplane.io/v1alpha3
  name: dev-k8s-cluster
```

An example of a `WorkloadTarget` in namespace `my-app-team` that is referencing
a Kubernetes `Provider` could look as follows:

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: WorkloadTarget
metadata:
  namespace: my-app-team
  name: enable-dev-k8s
  labels:
    env: dev
clusterRef:
  kind: Provider
  apiVersion: kubernetes.crossplane.io/v1alpha1
  name: onprem-dev-k8s-cluster
```

Because `WorkloadTarget` resources can point to the Kubernetes `Provider`
resource type, any existing Kubernetes cluster that was not provisioned using
Crossplane (including on-prem clusters) can now be targeted for provisioning
workloads.

### KubernetesApplication Scheduling

As mentioned above, `KubernetesApplication` resources are currently scheduled to
`KubernetesCluster` claims in their namespace. With the introduction of the
`WorkloadTarget`, `KubernetesApplication` resources can still be scheduled via
label selectors, but will now be scheduled to a `WorkloadTarget` in their
namespace, then use its `clusterRef` to get the `Secret` referenced by the
Kubernetes cluster managed resource or Kubernetes `Provider` object.
Importantly, for a `KubernetesApplication` to be able to be scheduled to a
Kubernetes cluster, a `WorkloadTarget` that points to that cluster managed
resource or `Provider` *must* be present in the namespace of the
`KubernetesApplication`.

This change isolates the actions of *provisioning* (`KubernetesCluster` or
cloud-specific managed resource), *publishing* (`WorkloadTarget`), and
*consuming* (`KubernetesApplication`) Kubernetes clusters, actions which may be
executed by [almost disjoint] sets of users. Having these actions represented by
separate custom resources allows for more granular [RBAC] permissions around
Kubernetes cluster usage in Crossplane. 

### User Workflows

The following scenarios serve to illustrate possible workflows that would be
enabled by this model. While some of these scenarios appear less likely than
others, the primary takeaway is the flexibility of this model, which allows for
a variety of use-cases.

#### Scenario 1: Infrastructure owner provisions and publishes all Kubernetes clusters

1. Infrastructure owner statically provisions three `GKECluster` managed
   resources: `dev`, `stage`, and `prod`
1. An application owner asks for an environment to deploy their application.
1. Infrastructure owner creates a new namespace and a `WorkloadTarget` that
   points to the `dev` cluster.
1. Application owner provisions a `KubernetesApplication` with labels that
   select the `WorkloadTarget` referencing the `dev` cluster for provisioning.
1. When ready for testing, the application owner requests access to the `stage`
   cluster, which the infrastructure owner then "publishes" by creating another
   `WorkloadTarget` in the namespace that points to the `stage` cluster.
1. Application owner provisions a `KubernetesApplication` with labels that
   select the `WorkloadTarget` referencing the `stage` cluster for provisioning.

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
   cluster.
1. Application owner creates a `WorkloadTarget` to point at the newly
   provisioned `GKECluster`.
1. When ready for testing, the application owner requests access to the `stage`
   cluster, which the infrastructure owner then "publishes" by creating a
   `WorkloadTarget` in the namespace that points to the `stage` cluster.
1. Application owner provisions a `KubernetesApplication` with labels that
   select the `WorkloadTarget` referencing the `stage` cluster for provisioning.

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
   existing on-premises cluster (for some important reason). The infrastructure
   owner creates a Kubernetes `Secret` with connection information to that
   cluster in an infrastructure namespace (secrets are namespace-scoped). They
   then create a cluster-scoped Kubernetes `Provider` that references the
   `Secret`.
1. Application owner says that they need to run a sensitive workload on that
   cluster. Infrastructure owner creates a new namespace called
   `super-secret-workloads` and creates a `WorkloadTarget` in that namespace
   that references the previously created Kubernetes `Provider`.
1. Application owner provisions a `KubernetesApplication` with labels that
   select that `WorkloadTarget` for provisioning.

This example shows the power of this model, but also one of the notable
shortcomings: if a user can publish clusters to a namespace with a
`WorkloadTarget`, then they can consume any cluster. For the application owners
that are doing self-service provisioning in this scenario, they will likely need
to be able to create `WorkloadTarget` resources for their dynamically
provisioned Kubernetes cluster managed resources. However, if they can do so,
they could just as easily create a `WorkloadTarget` for the Kubernetes
`Provider` that is only to be used for sensitive workloads. While it would be
nice to restrict the clusters that could be published by users with the ability
to create `WorkloadTarget` resources, there is not an immediately clear and
elegant way to do so. It could be argued that only trusted users should have
this permission level anyway, so it may be less of an issue than expressed here.
Nevertheless, a potential solution is outlined [below](#future-considerations).

## Technical Implementation

The implementation of this proposal is relatively lightweight and could
potentially be implemented without churn on any existing exposed APIs. The first
step would be implementing the `WorkloadTarget` type in [core Crossplane]. It
would not require any additional controllers.

The next step would be to modify the `KubernetesApplication` [scheduling
controller] to list `WorkloadTarget` resources in the namespace that match
labels, instead of the current implementation that lists `KubernetesCluster`
claims. Currently, this controller sets a reference to a `KubernetesCluster`
claim, which is then later propagated to the child
`KubernetesApplicationResource` objects, which use it to get a `Secret` with
[remote cluster connection information].

Because a `WorkloadTarget` may reference either a Kubernetes cluster managed
resource or a Kubernetes `Provider` resource, both of which ultimately lead to a
connection `Secret`, it would likely be cleanest to go ahead and get that
`Secret` and set a reference to it on the `KubernetesApplication`, then
propagate to the `KubernetesApplicationResource` objects. The alternative option
would be to set a reference to either the managed resource or the `Provider`,
then follow the references to the corresponding `Secret` in the remote cluster
connection step in the `KubernetesApplicationResource` object controller. While
setting the reference to the `Secret` is potentially a cross-namespace
reference, it is one that only exists in the status of the resource, and a valid
`WorkloadTarget` would have to exist in the `KubernetesApplication` namespace
for that cross-namespace reference to be set by the controller.

## Future Considerations

One feature that would eliminate a step for self-service dynamic cluster
provisioning and also fix the issue described above of users with permissions to
publish *a* cluster (i.e. create `WorkloadTarget` resources) being able to
publish *any* cluster would be to automatically create a `WorkloadTarget` as a
side effect for creating a `KubernetesCluster` claim. In scenario 3
[above](#user-workflows), this would mean that application owners would only
need permissions to create `KubernetesCluster` claims, and thus only be able to
consume their dynamically provisioned clusters because a `WorkloadTarget` is
created by the controller automatically. For clusters created by the
infrastructure owner, the claims could just be created in a
infrastructure-specific namespace as to not accidentally publish a cluster to an
application team namespace. Importantly, all Kubernetes cluster managed
resources created by an infrastructure owner in this scenario, whether
provisioned statically or dynamically, should be claimed immediately such that
application owners with permissions to create `KubernetesCluster` claims are not
able to use a cluster that they are not intended to. Implementation of this
functionality would increase the scope of this design to include implementing a
new claim controller exclusively for `KubernetesCluster` claims, so it has been
deferred to future implementation for the time being.

<!-- Named Links -->
[almost disjoint]: https://en.wikipedia.org/wiki/Almost_disjoint_sets
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[core Crossplane]: https://github.com/crossplaneio/crossplane
[scheduling controller]: https://github.com/crossplaneio/crossplane/blob/2cab9826a1d5088a87b5d24693336d228279a3a7/pkg/controller/workload/kubernetes/scheduler/scheduler.go#L57
[remote cluster connection information]: https://github.com/crossplaneio/crossplane/blob/2cab9826a1d5088a87b5d24693336d228279a3a7/pkg/controller/workload/kubernetes/resource/resource.go#L401