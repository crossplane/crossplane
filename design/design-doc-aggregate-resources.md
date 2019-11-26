# Aggregate Managed Resources
* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: DRAFT

## Background

Crossplane uses a [class and claim] model to provision and manage resources in
an external system, such as a cloud provider. _External resources_ in the
provider's API are modelled as _managed resources_ in the Kubernetes API server.
Managed resources are considered the domain of _infrastructure operators_;
they're cluster scoped infrastructure like a `Node` or `PersistentVolume`.
_Application operators_ may claim a managed resource for a particular purpose by
creating a namespaced _resource claim_. Managed resources may be provisioned
explicitly before claim time (static provisioning), or automatically at claim
time (dynamic provisioning). The initial configuration of dynamically
provisioned managed resources is specified by a _resource class_.

A managed resource is a _high-fidelity_ representation of its corresponding
external resource. High-fidelity in this context means two things:

* A managed resource maps to exactly one external resource - one API object.
* A managed resource is as close to a direct translation of its corresponding
  external API object as is possible without violating [API conventions].

These properties make managed resources - Crossplane's lowest level
infrastructure primitive - flexible and self documenting. Managed resources in
and of themselves hold few opinions about _how_ they should be used, and are
easily related back to the APIs they represent. This provides a solid foundation
upon which to build Crossplane's multicloud capability.

_Application operators_ are typically prevented by [RBAC] from creating and
modifying managed resources directly; they are instead expected to dynamically
provision the managed resources they require by submitting a resource claim.
Crossplane provides claim kinds for common, widely supported resource variants
like `MySQLInstance` and `KubernetesCluster`. There is a one-to-one relationship
between claims and the managed resources they bind to; a `KubernetesCluster`
claim binds to exactly one `GKECluster` managed resource. However, a solitary
resource is often not particularly useful without supporting infrastructure, for
example:

* An RDS instance may be inaccessible without a security group.
* An Azure SQL instance may be inaccessible without a virtual network rule.
* A GKE, EKS, or AKS cluster (control plane) may not be able to run pods without
  a node group.

Crossplane stacks frequently model this supporting infrastructure (there is a
`SecurityGroup` managed resource, for example) but it cannot be dynamically
provisioned or bound to a resource claim. Instead a cluster operator must
statically provision any supporting managed resources ahead of time, then author
resource classes that reference them. This can be limiting:

* Often supporting resources must reference the managed resource they support,
  for example an Azure `MySQLServerVirtualNetworkRule` must reference the
  `MySQLServer` it applies to. Dynamically provisioned managed resources such as
  a `MySQLServer` have non-deterministic names, making it impossible to create
  a `MySQLServerVirtualNetworkRule` until the `MySQLServer` it must reference
  has been provisioned.
* When a resource class references a statically provisioned managed resource
  every managed resource that is dynamically provisioned using that class will
  reference that specific managed resource. For example if a `GKEClusterClass`
  references a `Subnetwork` then every `GKECluster` dynamically provisioned
  using said class will attempt to share said `Subnetwork`, despite it often
  being desirable to create a unique `Subnetwork` for each dynamically
  provisioned `GKECluster`.

The one-to-one relationship between resource claims and resource classes thus
weakens portability, separation of concerns, and support for [GitOps]. An
infrastructure operator can publish a resource class representing a single
managed resource that an application operator may dynamically provision, but in
the likely event that managed resource requires supporting managed resources to
function usefully the application operator must ask an infrastructure operator
to provision them.

## Goals

The goal of this proposal is to enable infrastructure operators to publish
resource classes that represent _groups_ of infrastructure, for example "an
Azure SQL server with three virtual network rules", or "a GKE cluster with two
node pools in a new subnet".

It is important that the design put forward:

* Ensures that several interdependent resources submitted to Crossplane
  simultaneously may eventually resolve to functioning infrastructure.
* Avoids unnecessary API churn, especially to `v1beta1` managed resource APIs
  which must maintain backward compatibility.
* Does not introduce the requirement that any one Crossplane controller knows
  all extant kinds of managed resource, resource class, or resource claim.
* Maintains the high-fidelity, composable nature of managed resources.

## Proposal

This document proposes Crossplane introduce two new kinds:
`AggregateResourceClass` and `AggregateResource`:

```yaml
apiVersion: aggregation.crossplane.io/v1alpha1
kind: AggregateResourceClass
metadata:
  name: cool-gke-cluster
aggregationRule:
  classSelector:
    matchLabels:
      resourceclass.crossplane.io/aggregate-to: cool-gke-cluster
specTemplate:
  writeConnectionSecretsToNamespace: crossplane-system
```

```yaml
apiVersion: aggregation.crossplane.io/v1alpha1
kind: AggregateResource
metadata:
  name: default-coolcluster-g3bf7
spec:
  aggregationRule:
    resourceSelector:
      matchLabels:
        resourceclaim.crossplane.io/namespace: default
        resourceclaim.crossplane.io/name: coolcluster
        resourceclaim.crossplane.io/uid: eabce854-0cd7-11ea-8d71-362b9e155667
  claimRef:
    apiVersion: compute.crossplane.io/v1alpha1
    kind: KubernetesCluster
    namespace: default
    name: coolcluster
    uid: eabce854-0cd7-11ea-8d71-362b9e155667
  classRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResourceClass
    name: cool-gke-cluster
    uid: ea414c74-0cd9-11ea-8d71-362b9e155667
  writeConnectionSecretsToRef:
    namespace: crossplane-system
    name: eabce854-0cd7-11ea-8d71-362b9e155667
```

The purpose of these new resource kinds, as their names imply, is to _aggregate_
resource classes and managed resources, enabling transitive one-to-many
resource-claim-to-resource-class and resource-claim-to-managed-resource
relationships. Both new kinds use an _aggregation rule_, like an [RBAC
`ClusterRole`]. The `AggregateResourceClass` aggregates resource classes (of any
non-aggregate kind) by label, while the `AggregateResource` aggregates managed
resources (again, of any non-aggregate kind) by label.

In the above example `default-coolcluster-g3bf7` is an `AggregateResource` that
was dynamically provisioned to satisfy a `KubernetesCluster` resource claim. An
`AggregateResource` was called for because the `KubernetesCluster` specified an
`AggregateResourceClass` as its `classRef`:

```yaml
apiVersion: compute.crossplane.io/v1beta1
kind: KubernetesCluster
metadata:
  namespace: default
  name: coolcluster
spec:
  classRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResourceClass
    name: cool-gke-cluster
```

The creation of the above `KubernetesCluster` claim triggers:

1. The dynamic provisioning of the `AggregateResource`, which aggregates all
   managed resources labelled with the claim's namespace, name, and UID.
1. The dynamic provisioning of a managed resource for each resource class that
   aggregates to the `AggregateResourceClass` by matching its `aggregationRule`
   labels. These dynamically provisioned managed resources are labelled with the
   claim's namespace, name, and UID and thus aggregate to the dynamically
   provisioned `AggregateResource`.

The `AggregateResource` binds to the `KubernetesCluster` claim, while the
dynamically provisioned managed resources bind in turn to the
`AggregateResource`, thus creating a `resource claim -> aggregate resource ->
managed resource(s)` binding relationship. The below `GKECluster` is
_aggregated_. It binds to an `AggregateResource` (per its `aggregateRef` and
`bindingPhase`) rather than to a resource claim:

```yaml
apiVersion: compute.gcp.crossplane.io/v1beta1
kind: GKECluster
metadata:
  name: default-coolcluster-ffm8f
  labels:
    resourceclass.crossplane.io/aggregate-to: cool-gke-cluster
spec:
  forProvider:
    # Omitted for brevity.
  classRef:
    apiVersion: compute.gcp.crossplane.io/v1beta1
    kind: GKEClusterClass
    name: cool-gke-cluster
    uid: 64eeaf54-0cdd-11ea-8d71-362b9e155667
  aggregateRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResource
    name: default-coolcluster-g3bf7
    uid: 2d45b85e-0cdd-11ea-8d71-362b9e155667
  writeConnectionSecretsToRef:
    namespace: crossplane-system
    name: 97efa0de-0cdd-11ea-8d71-362b9e155667
  reclaimPolicy: Delete
```

An `AggregateResource` is not opinionated about which managed resource kinds it
aggregates, but this does not mean it may _bind to_ arbitrary resource claim
kinds. Crossplane is architected such that a "claim binding" controller owns
each resource claim to managed resource combination. This allows the controller
that handles the binding and dynamic provisioning logic for a particular managed
resource kind to live in that managed resource's stack, alongside the controller
that reconciles that managed resource kind with its external resource. One side
effect of this architecture is that "nonsensical" claims are ignored. A `Bucket`
claim will never automatically be allocated a `CloudSQLInstanceClass`, but
nothing prevents a `Bucket` author explicitly setting its `resourceRef` to a
`CloudSQLInstance`, or setting its `classRef` to a `CloudSQLInstanceClass`.
However, no controller owns the "nonsensical" relationship between a `Bucket`
and a `CloudSQLInstance`, so the claim is simply never reconciled. This would
hold true for aggregate kinds; each stack author would choose which claim kinds
may bind to which managed resource kinds via an `AggregateResource`. This means:

* A claim would never bind to an `AggregateResource` that did not match at least
  one managed resource that made sense for it to claim.
* A claim would never be automatically allocated to an `AggregateResourceClass`
  (via class selection or defaulting) that did not match at least one class of
  managed resource that made sense for it to claim.
* A claim would never dynamically provision anything if its `classRef` were set
  to an `AggregateResourceClass` that did not match at least one class of
  managed resource that made sense for it to claim.
* In the case of partial matches, only the supported bindings are reconciled. A
  `RedisCluster` claim may be allocated an `AggregateResourceClass` that matches
  a `CloudMemorystoreInstanceClass` and an `EKSClusterClass`, but will only
  dynamically provision and bind to (via an `AggregatedResource`) a
  `CloudMemorystoreInstance`, not an `EKSCluster`.

Note that a resource class such as a `GKEClusterClass` may aggregate to more
than one `AggregateResourceClass`. This allows resource classes to be reused in
order to compose different shapes of `AggregateResourceClass`. For example more
than one `AggregateResourceClass` could share the same `GKEClusterClass` control
plane configuration, but each use different `GKENodePoolClass` configurations
for their node pools. A managed resource, conversely, may only aggregate to a
single `AggregateResource`, as it must bind to the `AggregateResource` to be
considered aggregated. A managed resource may _match_ more than one
`AggregateResource` (though the labels of dynamically provisioned managed
resources prevent this), but will bind only to the first match.

### Aggregate-internal References

Crossplane's [cross resource reference] support must be supplemented in order to
allow references between dynamically provisioned managed resources to be
specified in advance. Consider an `AggregateResourceClass` that aggregates:

* `SubnetworkClass` A
* `GKEClusterClass` A
* `ServiceAccountClass` A
* `ServiceAccountClass` B
* `GKENodePoolClass` A
* `GKENodePoolClass` B

Assume the dynamically provisioned managed resource corresponding to each class
will share its name; i.e. `SubnetworkClass` A provisions `Subnetwork` A. The
`AggregateResourceClass` author would like to configure the aggregated classes
such that:

1. `Subnetwork` A is created in an existing, statically provisioned `Network`.
1. `GKECluster` A is created in `Subnetwork` A.
1. `GKENodePool` A uses `ServiceAccount` A.
1. `GKENodePool` B uses `ServiceAccount` B.
1. Both `GKENodePool` resources join `GKECluster` A.

The author cannot use a contemporary cross resource reference for requirements
two through five. Managed resources are referenced by name, and the names of
dynamically provisioned resources are not known until they have been
provisioned. Thus the author must be able to configure rules such as:

* Managed resources that are dynamically provisioned using resource class A
  should reference a managed resource that was dynamically provisioned using
  resource class B.
* Managed resources that are dynamically provisioned using resource class A
  should reference a managed resource that is part of the same
  `AggregateResource`.

This document proposes the introduction of a reference _selector_:

```yaml
apiVersion: compute.gcp.crossplane.io/v1alpha3
kind: Subnetwork
metadata:
  name: coolsubnet
spec:
  forProvider:
    network: /projects/foo/global/networks/bar
    networkRef:
      name: foobar
    networkSelector:
      matchAggregate: true
      matchClassRef:
        name: foo
```

The above example shows a GCP `Subnetwork` managed resource that uses a
reference selector. `networkSelector` sets the `networkRef` which in turn sets
the value of `network`. The reference selector contains two optional fields:

* `matchAggregate`: Match only managed resources that are part of the same
  `AggregateResource` (i.e. have the same `aggregateRef`) as the selecting
  resource.
* `matchClassRef`: Match only managed resources that were dynamically
  provisioned using the named resource class.

If a reference field is set, its corresponding selector field is ignored. If the
selector field is unset, it is ignored. If the specified selector matches
multiple managed resources one is chosen at random. Note however that setting
both `matchAggregate` and `matchClassRef` guarantees at most one dynamically
provisioned managed resource will match the selector. At most one managed
resource dynamically provisioned by a named resource class can aggregate to an
`AggregateResource` because a named resource class may only aggregate to an
`AggregateResourceClass` once. Resource claims reference one resource class, and
a dynamically provisioned `AggregateResource` selects managed resources using
the UID of the resource claim that triggered dynamic provisioning.

It is common for Kubernetes selector objects to include a `matchLabels` stanza.
Such a stanza could be used to further limit selection, but is not immediately
useful to the design proposed by this document and thus not recommended at this
time.

### External Names

Managed resources have a name and an _external name_. The former identifies the
managed resource in the Kubernetes API, while the latter identifies the resource
in the external system. Managed resources that have reached `v1beta1` allow both
managed resource and resource claim authors to [control the name of their
external resource] using the `crossplane.io/external-name` annotation:

* The name of the external resource is set to the value of the annotation.
* If the annotation is absent it is set to the managed resource's name.
* The annotation is propagated from a resource claim to any managed resource it
  dynamically provisions.

This final point may pose a problem when one resource claim may provision many
managed resources. If a resource claim set the `crossplane.io/external-name`
annotation and referenced an `AggregateResourceClass` that aggregated two
resource classes of the same kind, both dynamically provisioned resources would
attempt to use the same external name. External names are unique at different
scopes (global, project, region) for different resources so this will not be an
issue all resources, but it may for some.

This document proposes two new external naming annotations be introduced at the
resource class level to help resource class authors avoid external name
conflicts:

* `crossplane.io/external-name-prefix` - A value prepended to the external name
  of any managed resource dynamically provisioned using the class.
* `crossplane.io/external-name-suffix` - A value appended to the external name
  of any managed resource dynamically provisioned using the class.

These annotations would only be used during dynamic provisioning when a resource
claim specified the `crossplane.io/external-name` annotation, resulting in an
external name of `{prefix}{name}{suffix}`

### Connection Secrets

Many managed resources publish _connection secrets_; Kubernetes `Secret`
resources containing the potentially sensitive details required to connect to
the managed resource such as its hostname, ports, and credentials. These secrets
are typically created in a namespace that is not accessible to application
operators. Resource claim authors may request that the connection secret of
their bound managed resource be propagated to the claim's namespace by
specifying its `writeConnectionSecretToRef` field.

This document proposes that `AggregateResource` connection secrets contain the
intersection of the connection secrets of the managed resources they aggregate. 
For example if an `AggregateResource` matched two managed resources with the
following secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: crossplane-system
  name: secret-a
stringData:
  username: cooluser
  password: verysecure
---
apiVersion: v1
kind: Secret
metadata:
  namespace: crossplane-system
  name: secret-b
stringData:
  url: https://example.org
```

The `AggregateResource` connection secret would be:

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: crossplane-system
  name: aggregate-secret
stringData:
  username: cooluser
  password: verysecure
  url: https://example.org
```

An undefined connection secret key will win in the case of a conflict. Conflicts
are expected to be rare in practice (most "supporting" managed resources do not
create a connection secret), but may be avoided by respecting a new
`aggregation.crossplane.io/connection-secret-prefix` annotation. This annotation
would be propagated from resource classes to the managed resources they
dynamically provision, specifying a string to be prepended to their connection
secret keys during aggregation.

## User Experience

Consider an infrastructure operator who wishes to enable their application
operator peers to submit `MySQLInstance` resource claims and have them be
satisfied by dynamically provisioned Azure `MySQLServer` managed resources. Each
`MySQLServer` should allow traffic from a statically provisioned subnetwork
using a `MySQLServerVirtualNetworkRule`. Each dynamically provisioned
`MySQLServer` and its associated virtual network rule should be created in its
own resource group.

First the infrastructure operator statically provisions the `Subnet` from which
all dynamically provisioned `MySQLServer` managed resources should allow
traffic:

```yaml
apiVersion: network.azure.crossplane.io/v1alpha3
kind: Subnet
metadata:
  name: private
spec:
  resourceGroupName:
    name: some-existing-group
  virtualNetworkName:
    name: some-existing-network
  properties:
    addressPrefix: 10.2.0.0/24
    serviceEndpoints:
      - service: Microsoft.Sql
  providerRef:
    name: example
```

The infrastructure operator then authors resource classes for each of the
managed resources they wish to allow application operators to dynamically
provision, and an aggregate resource class to aggregate them:

```yaml
apiVersion: aggregation.crossplane.io/v1alpha1
kind: AggregateResourceClass
metadata:
  name: private-mysql-server
  annotations:
    # This will be the default resource class for any resource claim capable of
    # binding directly to a resource of one of the classes it aggregates - in
    # this case MySQLInstance.
    resourceclass.crossplane.io/is-default-class: "true"
aggregationRule:
  classSelector:
    matchLabels:
      # Any resource class matching these labels is aggregated to this class.
      # The aggregate-to key and its value are not special, but are a
      # recommended convention.
      resourceclass.crossplane.io/aggregate-to: private-mysql-server
specTemplate:
  # This reclaim policy determines whether the AggregateResource is retained or
  # deleted when its claim is deleted. It is not propagated to the aggregated
  # managed resources; they respect the reclaimPolicy of the individual resource
  # classes used to configure them.
  reclaimPolicy: Delete
  # AggregateResources dynamically provisioned using this AggregateResourceClass
  # will write their connection secrets to this namespace.
  writeConnectionSecretsToNamespace: crossplane-system
---
apiVersion: azure.crossplane.io/v1alpha3
kind: ResourceGroupClass
metadata:
  name: private-mysql-server
  labels:
    # This ResourceGroupClass aggregates to the private-mysql-server
    # AggregateResourceClass.
    resourceclass.crossplane.io/aggregate-to: private-mysql-server
specTemplate:
  location: Central US
  providerRef:
    name: example
---
apiVersion: database.azure.crossplane.io/v1beta1
kind: SQLServerClass
metadata:
  name: private-mysql-server
  labels:
    resourceclass.crossplane.io/aggregate-to: private-mysql-server
specTemplate:
  forProvider:
    administratorLogin: my-cool-login
    # MySQLServer managed resources dynamically provisioned using this resource
    # class will select the ResourceGroup dynamically provisioned by the above
    # resource class because its classRef will be named private-mysql-server and
    # its aggregateRef will be identical to the MySQLServer's.
    resourceGroupNameSelector:
      matchAggregate: true
      matchClassRef:
        name: private-mysql-server
    location: Central US
    sslEnforcement: Disabled
    version: "5.6"
    sku:
      tier: GeneralPurpose
      capacity: 2
      family: Gen5
    storageProfile:
      storageMB: 25600
      backupRetentionDays: 7
      geoRedundantBackup: Disabled
  providerRef:
    name: example
  # MySQLServer managed resources dynamically provisioned using this class still
  # write their own unaggregated, unadulterated connection secrets.
  writeConnectionSecretsToNamespace: crossplane-system
---
apiVersion: database.azure.crossplane.io/v1alpha3
kind: MySQLServerVirtualNetworkRuleClass
metadata:
  name: private-mysql-server
  labels:
    resourceclass.crossplane.io/aggregate-to: private-mysql-server
spec:
  serverNameSelector:
    matchAggregate: true
    matchClassRef:
      name: private-mysql-server
  resourceGroupNameSelector:
    matchAggregate: true
    matchClassRef:
      name: private-mysql-server
  properties:
    # Aggregated resource classes can still reference a statically provisioned
    # managed resource as they can today. Every MySQLVirtualNetworkRule created
    # using this MySQLServerVirtualNetworkClass will allow traffic from the
    # 'private' Subnet.
    virtualNetworkSubnetIdRef:
      name: private
  providerRef:
    name: example
```

Note that the `AggregateResourceClass` was annotated as the default, and will
thus be used by any resource claim that does not specify a `resourceRef`,
`classRef`, or `classSelector`. The resource claim API is unchanged under this
proposal, so the application operator may submit a typical claim:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  namespace: default
  name: sql
  annotations:
    # The resource group, SQL server, and virtual network rule will all be named
    # "very-private-database" in the Azure API, console, etc. The above resource
    # classes need not use the external-name-prefix and suffix annotations
    # because they provision only one of each resource type, and names need not
    # by unique across types.
    crossplane.io/external-name: very-private-database
spec:
  engineVersion: "5.7"
  # The connection secret of the dynamically provisioned AggregateResource will
  # be propagated to this reference.
  writeConnectionSecretToRef:
    name: sql
```

Creation of the above resource claim would trigger dynamic provisioning of the
following resources:

```yaml
apiVersion: aggregation.crossplane.io/v1alpha1
kind: AggregateResource
metadata:
  name: default-sql-gdm0w
spec:
  aggregationRule:
    resourceSelector:
      matchLabels:
        # The dynamically provisioned AggregateResource matches all resources
        # that were also dynamically provisioned by the resource claim that
        # triggered its provisioning.
        resourceclaim.crossplane.io/namespace: default
        resourceclaim.crossplane.io/name: sql
        resourceclaim.crossplane.io/uid: 2200b0c8-0da2-11ea-8d71-362b9e155667
  # The AggregateResource maintains a claimRef back to the resource claim that
  # triggered its dynamic provisioning.
  claimRef:
    apiVersion: database.crossplane.io/v1alpha1
    kind: MySQLInstance
    namespace: default
    name: sql
    uid: eabce854-0cd7-11ea-8d71-362b9e155667
  # The AggregateResource maintains a classRef back to the aggregate resource
  # class that was used to dynamically provision it.
  classRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResourceClass
    name: private-mysql-server
    uid: ea414c74-0cd9-11ea-8d71-362b9e155667
  # The aggregated connection secret will be written to this reference. In this
  # case it will contain only the keys of the MySQLServer managed resource, as
  # it is the only aggregated resource that publishes a connection secret.
  writeConnectionSecretsToRef:
    namespace: crossplane-system
    name: ea414c74-0cd9-11ea-8d71-362b9e155667
  reclaimPolicy: Delete
status:
  # Each object under the aggregatedResource represents a managed resource that
  # matches the resourceSelector of this AggregateResource, including the
  # status of the managed resource's binding to the AggregateResource.
  aggregatedResources:
  - apiVersion: azure.crossplane.io/v1alpha3
    kind: ResourceGroup
    name: default-sql-ab4k8
    uid: bf44f148-0dd7-11ea-8d71-362b9e155667
    bindingPhase: Bound
  - apiVersion: database.azure.crossplane.io/v1beta1
    kind: SQLServer
    name: default-sql-d82nd
    uid: e3c3166c-0dd7-11ea-8d71-362b9e155667
    bindingPhase: Bound
  - apiVersion: database.azure.crossplane.io/v1alpha3
    kind: MySQLServerVirtualNetworkRule
    name: default-sql-9dm3v
    uid: e02cffd6-0dd7-11ea-8d71-362b9e155667
    bindingPhase: Bound
  # The AggregateResource's binding status represents its binding to the claim
  # that triggered its dynamic provisioning. An AggregateResource binds to its
  # claim immediately, then acts as a proxy for the claims binding to the
  # managed resources it aggregates.
  bindingPhase: Bound
---
apiVersion: azure.crossplane.io/v1alpha3
kind: ResourceGroup
metadata:
  name: default-sql-ab4k8
  labels:
    # This managed resource aggregates to the AggregateResource that was created
    # as part of its dynamic provisioning.
    resourceclaim.crossplane.io/namespace: default
    resourceclaim.crossplane.io/name: sql
    resourceclaim.crossplane.io/uid: 2200b0c8-0da2-11ea-8d71-362b9e155667
spec:
  location: Central US
  providerRef:
    name: example
  # Aggregated managed resources have an aggregateRef to their AggregateResource
  # instead of a claimRef to their resource claim; this resource can still be
  # traced to and from its resource claim via the aggregateRef.
  aggregateRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResource
    name: default-sql-gdm0w
    uid: 47c7f614-0dd8-11ea-9a9f-362b9e155667
  # Aggregated managed resources have a classRef to the specific resource class
  # used to create them, not the AggregateResourceClass.
  classRef:
    apiVersion: azure.crossplane.io/v1alpha3
    kind: ResourceGroupClass
    name: private-mysql-server
    uid: 8d7c86e8-0dd8-11ea-8d71-362b9e155667
status:
  # This binding status represents the managed resource's binding to its
  # AggregateResource.
  bindingPhase: Bound
--
apiVersion: database.azure.crossplane.io/v1beta1
kind: SQLServer
metadata:
  name: default-sql-d82nd
  labels:
    resourceclaim.crossplane.io/namespace: default
    resourceclaim.crossplane.io/name: sql
    resourceclaim.crossplane.io/uid: 2200b0c8-0da2-11ea-8d71-362b9e155667
spec:
  forProvider:
    administratorLogin: my-cool-login
    # The resourceGroupName is always set by the resourceGroupNameRef. The
    # selector field sets the reference field, which sets the actual field.
    resourceGroupName: default-sql-ab4k8
    # This resourceGroupNameRef was not set by the SQLServerClass used to
    # dynamically provision this SQLServer, but was instead selected by the
    # resourceGroupNameSelector. One it is set the resourceGroupNameSelector is
    # ignored. Managed resource authors can force it to be re-resolved by
    # deleting the resourceGroupNameRef field.
    resourceGroupNameRef:
      name: default-sql-ab4k8
    resourceGroupNameSelector:
      matchAggregate: true
      matchClassRef:
        name: private-mysql-server
    location: Central US
    sslEnforcement: Disabled
    version: "5.6"
    sku:
      tier: GeneralPurpose
      capacity: 2
      family: Gen5
    storageProfile:
      storageMB: 25600
      backupRetentionDays: 7
      geoRedundantBackup: Disabled
  providerRef:
    name: example
  aggregateRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResource
    name: default-sql-gdm0w
    uid: 47c7f614-0dd8-11ea-9a9f-362b9e155667
  classRef:
    apiVersion: database.azure.crossplane.io/v1beta1
    kind: SQLServerClass
    name: private-mysql-server
    uid: 0802ef2e-0dd9-11ea-8d71-362b9e155667
  writeConnectionSecretsToNamespace: crossplane-system
status:
  bindingPhase: Bound
---
apiVersion: database.azure.crossplane.io/v1alpha3
kind: MySQLServerVirtualNetworkRule
metadata:
  name: default-sql-9dm3v
  labels:
    resourceclaim.crossplane.io/namespace: default
    resourceclaim.crossplane.io/name: sql
    resourceclaim.crossplane.io/uid: 2200b0c8-0da2-11ea-8d71-362b9e155667
spec:
  serverName: default-sql-d82nd
  serverNameRef:
    name: default-sql-d82nd
  serverNameSelector:
    matchAggregate: true
    matchClassRef:
      name: private-mysql-server
  resourceGroupName: default-sql-ab4k8
  resourceGroupNameRef:
    name: default-sql-ab4k8
  resourceGroupNameSelector:
    matchAggregate: true
    matchClassRef:
      name: private-mysql-server
  properties:
    virtualNetworkSubnetId: private
    virtualNetworkSubnetIdRef:
      name: private
  providerRef:
    name: example
  aggregateRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResource
    name: default-sql-gdm0w
    uid: 47c7f614-0dd8-11ea-9a9f-362b9e155667
  classRef:
    apiVersion: database.azure.crossplane.io/v1alpha3
    kind: MySQLServerVirtualNetworkRuleClass
    name: private-mysql-server
    uid: ce96b928-0dd8-11ea-8d71-362b9e155667
status:
  bindingPhase: Bound
```

Finally, the resource claim would be updated to reflect the dynamically
provisioned `AggregateResource` that it was bound to:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  namespace: default
  name: sql
spec:
  engineVersion: "5.7"
  writeConnectionSecretToRef:
    name: sql
  # The AggregateResourceClass that aggregates all resource classes that were
  # used to dynamically provision managed resources to satisfy this
  # MySQLInstance.
  classRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResourceClass
    name: private-mysql-server
    uid: 94c6df60-0dd9-11ea-9a9f-362b9e155667
  # The AggregateResource that aggregates all managed resources that were
  # dynamically provisioned to satisfy this MySQLInstance.
  resourceRef:
    apiVersion: aggregation.crossplane.io/v1alpha1
    kind: AggregateResource
    name: default-sql-gdm0w
    uid: 47c7f614-0dd8-11ea-9a9f-362b9e155667
```

### Technical Implementation

Support for `AggregateResourceClass` and `AggregateResource` would be
implemented by introducing a variant of the contemporary
[`resource.ClaimReconciler`] that is used to dynamically provision and bind
resource claims. Little to no change would be necessary to existing managed
resources, claims, or classes either at the API or logic level, except to make
them aware of `aggregateRef` and add support for cross resource selection. Both
changes are strictly additive and thus deemed [backward compatible].

Crossplane infrastructure stacks are designed such that each stack need only be
aware of the resource claim, resource class, and managed resource kinds that it
is concerned with. No one Crossplane controller, including those in the core
Crossplane controller manager, need be aware of the complete set of claims,
classes, or resources. This design allows a stack to add support for new kinds
of managed resources, claims, and classes without coordinating with any other
system. All resource claim controllers use a `resource.ClaimReconciler` and are
concerned with a particular kind of managed resource, claim, and class. For
example the `GKECluster` claim controller watches for `KubernetesCluster`
resource claims that either reference a specific `GKECluster`, or reference a
`GKEClusterClass` for dynamic provisioning. Binding logic is standard across
controllers, but each controller must supply one or more types that satisfy
[`resource.ManagedConfigurator`] in order to specify how a dynamically
provisioned managed resource should be configured, given a resource claim and
class.

An aggregate-aware equivalent to `resource.ClaimReconciler` would be similarly
concerned with a particular kind of managed resource, claim, and class; it would
reconcile only the subset of aggregated classes and/or resources that were of
the kind it was concerned with, via their aggregate kinds. For example an
aggregate-aware `GKECluster` reconciler would:

1. Watch for `KubernetesCluster` managed resources that either reference a
   specific `AggregateResource` or reference an `AggregateResourceClass`.
1. Use a watch predicate to filter out any claims whose aggregate resource or
   class does not match at least one managed resource or class of the kind it is
   concerned with, i.e. `GKECluster` or `GKEClusterClass`.
1. If the resource claim had no `resourceRef` the reconciler would create an
   `AggregateResource` with an `aggregateRule` selecting on labels derived from
   the resource claim's namespace, name, and UID then immediately bind it to the
   `KubernetesCluster` claim.
1. For each `GKEClusterClass` matching the `AggregateResourceClass` referenced
   by the claim, check whether a `GKECluster` referencing said class was already
   referenced by the `aggregatedResources` of the `AggregateResource.` If no
   such `GKECluster` was referenced, dynamically provision a new one using the
   `GKEClusterClass` and add it to the `aggregatedResources`. The existing
   `ConfigureGKECluster` configurator could be reused for this.
1. Process any `GKECluster` managed resources bound to the `AggregateResource`,
   updating their binding status and propagating their connection secrets.

No controller is watching for `AggregateResource` specifically; only resource
claims that use or would use (due to their class) an `AggregateResource`. This
means that a statically provisioned `AggregateResource` will not bind to any of
the managed resources it would aggregate until it is claimed.

This design implies the introduction of resource classes for managed resources
that do not currently support them; there is no `ResourceGroupClass`, for
example, because `ResourceGroup` has never had a corresponding resource claim to
bind to. Furthermore, each infrastructure stack must instantiate an
aggregation-aware resource claim controller for each possible resource claim to
managed resource combination. Put otherwise, if it makes _any_ sense for a
particular managed resource to aggregate to a particular claim kind a controller
must own that relationship. Many supporting managed resources map only to a
single resource claim kind. A `MySQLServerVirtualNetworkRuleClass` is unlikely
to be useful in the context of a `RedisCluster` resource claim; it likely need
only apply to `MySQLServer` claims. Several resources however, including
`ResourceGroup` and networking constructs like `Subnetwork` could make sense to
aggregate to almost all resource claim kinds.

Care must be taken when naming a dynamically provisioned `AggregateResource`,
because creation of the `AggregateResource` could be handled any aggregate-aware
resource claim controller concerned with one of the aggregated resource classes.
Dynamic provisioning [typically uses] the API server's built in `GenerateName`
support to generate a non-deterministic name for the newly created managed
resource. If two controllers were to use this approach there would be a race;
each controller would create a differently named `AggregateResource`, but only
one would succeed in updating the `resourceRef` of the resource claim, leaving
an orphaned `AggregateResource`. This can be avoided by emulating `GenerateName`
locally, and using a name-then-create approach:

1. If the resource claim has a `resourceRef.name`, but no `resourceRef.uid`, all
   concerned controllers race to create the named `AggregateResource`. The
   create will fail if another controller created it first. If the create
   succeeds, the controller writes the newly created `AggregateResource` UID to
   `.resourceRef.uid`.
1. If the claim has no `resourceRef.name` all concerned controllers generate a
   name using the typical managed resource naming scheme and attempt to set it
   as the claim's `resourceRef.name`. The write will fail if another controller
   has set the name since the controller read the claim.

## Alternatives Considered

The following alternatives were considered in arriving at the proposal put
forward by this document.

### Complex Managed Resources

One alternative to aggregate managed resources would be to model tightly coupled
external resources as a single managed resource. For example the `MySQLServer`
managed resource might allow the author to configure an array of virtual network
rules in its spec. Such "complex" managed resources are not uncommon in
Crossplane's older controllers; at the time of writing creating an `S3Bucket`
managed resource for example creates both an S3 bucket and an IAM user in the
AWS API.

This design compromises on the "granularity" aspect of high-fidelity managed
resources, as discussed in the [Background] section of this document. While this
may seem innocuous, it has undesirable properties:

* It forces Crossplane's assumptions on how resources will be used; i.e. that a
  Crossplane user would never want to create a GKE node pool without also
  managing its control plane in Crossplane, or would never want to create a
  virtual network rule for an Azure SQL server that Crossplane was unaware of.
* It violates the [principle of least astonishment]. In the `S3Bucket` example
  above it may be surprising for a Crossplane user to be provisioned an IAM user
  when they only requested an S3 bucket.
* The mapping from Crossplane managed resource to underlying API becomes less
  obvious - it's not clear that virtual network rules are actually a distinct
  API rather than an inherent part of a MySQL server. This makes it harder for
  Crossplane users to fall back to the underlying provider's documentation when
  Crossplane's is unclear.
* Each managed resource reconciler becomes more complex, as it must reconcile
  each managed resource with multiple external resources.

Supporting complex managed resources would remove (or less generously, move) the
need to add complexity to the Crossplane API and resource claim logic, but the
cost would be a complex mental model of how Crossplane managed resources relate
to cloud resources, and limitations placed on how Crossplane could be used.

### Cross-claim References

Cross-claim references would allow resource claims to reference other resource
claims, similarly to how managed resources can reference other managed
resources. Rather than a `MySQLInstance` claim binding to an `AggregateResource`
of a `MySQLServer` and a `MySQLVirtualNetworkRule` each managed resource would
be claimed separately - the `MySQLServer` by the `MySQLInstance` and the
`MySQLVirtualNetworkRule` by a new claim: perhaps `FirewallRule`. The
`FirewallRule` claim would state via a cross-claim reference that it applies to
the `MySQLInstance` claim.

This design shifts much of the burden of designing infrastructure from
infrastructure operators to application operators. Instead of an application
operator requesting a `MySQLInstance` and trusting that the infrastructure
operator has published a resource class that will ensure it is securely
configured, they must also request a `FirewallRule` be applied to that
`MySQLInstance`. Furthermore, the "lowest common denominator" problem affects
resource claims, in that they can only expose configuration fields that
translate to _every_ managed resource that could satisfy the claim. A
`FirewallRule` for example could only expose configuration fields that apply to
`MySQLVirtualNetworkRule`, `SecurityGroup`, and any other firewall-like managed
resource. This means application owners would simultaneously be saddled with the
burden of designing appropriate infrastructure for their needs and a limited
language in which to do so.

### In-line Equivalents of AggregateResource and AggregateResourceClass

Both `AggregateResource` and `AggregateResourceClass` _select_ the resources and
classes they aggregate. An alternative to this approach would be to have the
resources specified (or templated) in-line in one large managed resource or
class, similar to a Crossplane `KubernetesApplication`. Such a design would
require Crossplane to support arbitrary nested managed resources or classes
in-line of their parent. This would require the nested resources to be
schemaless, weakening the user experience by making it difficult to document and
validate the values of fields when configuring aggregate resources or classes.

### Multiple-binding Resource Claims

`AggregateResourceClass` could be introduced without `AggregateResource` if the
functionality of `AggregateResource` were built directly into resource claims.
This would reduce the number of new API types and concepts, but would require
broad changes to resource claim API types and reconciler logic.

[class and claim]: https://static.sched.com/hosted_files/kccncna19/2d/kcconna19-eric-tune.pdf
[API conventions]: https://github.com/kubernetes/community/blob/862de062acf8bbd84f7a655914fa08972498819a/contributors/devel/sig-architecture/api-conventions.md
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[GitOps]: https://www.weave.works/technologies/gitops/
[RBAC `ClusterRole`]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles
[cross resource reference]: one-pager-cross-resource-referencing.md
[control the name of their external resource]: one-pager-managed-resource-api-design.md#external-resource-name
[`resource.ClaimReconciler`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ClaimReconciler
[backward compatible]: https://github.com/kubernetes/community/blob/862de062acf8bbd84f7a655914fa08972498819a/contributors/devel/sig-architecture/api_changes.md#on-compatibility
[`resource.ManagedConfigurator`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ManagedConfigurator
[typically uses]: one-pager-managed-resource-api-design.md#custom-resource-instance-name
[Background]: #background
[principle of least astonishment]: https://en.wikipedia.org/wiki/Principle_of_least_astonishment