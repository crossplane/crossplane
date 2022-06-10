---
title: Managed Resources
toc: true
weight: 102
indent: true
---

# Managed Resources

## Overview

A Managed Resource (MR) is Crossplane's representation of a resource in an
external system - most commonly a cloud provider. Managed Resources are
opinionated, Crossplane Resource Model ([XRM][term-xrm]) compliant Kubernetes
Custom Resources that are installed by a Crossplane [provider].

For example, `RDSInstance` in the AWS Provider corresponds to an actual RDS
Instance in AWS. There is a one-to-one relationship and the changes on managed
resources are reflected directly on the corresponding resource in the provider.
Similarly, the `Database` types in the SQL provider represent a PostgreSQL or
MySQL database. You can browse [API Reference][api-reference] to discover all
available managed resources.

Managed Resources are the building blocks of Crossplane. They're designed to be
_composed_ into higher level, opinionated Custom Resources that Crossplane calls
Composite Resources or XRs - not used directly. See the
[Composition][composition] documentation for more information.

## Syntax

Crossplane API conventions extend the Kubernetes API conventions for the schema
of Crossplane managed resources. Following is an example of a managed resource:

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-1" data-toggle="tab">AWS</a></li>
<li><a href="#gcp-tab-1" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-1" data-toggle="tab">Azure</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="aws-tab-1" markdown="1">

The AWS provider supports provisioning an [RDS] instance via the `RDSInstance`
managed resource it adds to Crossplane.

```yaml
apiVersion: database.aws.crossplane.io/v1beta1
kind: RDSInstance
metadata:
  name: rdspostgresql
spec:
  forProvider:
    region: us-east-1
    dbInstanceClass: db.t2.small
    masterUsername: masteruser
    allocatedStorage: 20
    engine: postgres
    engineVersion: "12"
    skipFinalSnapshotBeforeDeletion: true
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: aws-rdspostgresql-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.8/docs/snippets/provision/aws.yaml
```

Creating the above instance will cause Crossplane to provision an RDS instance
on AWS. You can view the progress with the following command:

```console
kubectl get rdsinstance rdspostgresql
```

When provisioning is complete, you should see `READY: True` in the output. You
can take a look at its connection secret that is referenced under
`spec.writeConnectionSecretToRef`:

```console
kubectl describe secret aws-rdspostgresql-conn -n crossplane-system
```

You can then delete the `RDSInstance`:

```console
kubectl delete rdsinstance rdspostgresql
```

</div>
<div class="tab-pane fade" id="gcp-tab-1" markdown="1">

The GCP provider supports provisioning a [CloudSQL] instance with the
`CloudSQLInstance` managed resource it adds to Crossplane.

```yaml
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstance
metadata:
  name: cloudsqlpostgresql
spec:
  forProvider:
    databaseVersion: POSTGRES_12
    region: us-central1
    settings:
      tier: db-custom-1-3840
      dataDiskType: PD_SSD
      dataDiskSizeGb: 10
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: cloudsqlpostgresql-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.8/docs/snippets/provision/gcp.yaml
```

Creating the above instance will cause Crossplane to provision a CloudSQL
instance on GCP. You can view the progress with the following command:

```console
kubectl get cloudsqlinstance cloudsqlpostgresql
```

When provisioning is complete, you should see `READY: True` in the output. You
can take a look at its connection secret that is referenced under
`spec.writeConnectionSecretToRef`:

```console
kubectl describe secret cloudsqlpostgresql-conn -n crossplane-system
```

You can then delete the `CloudSQLInstance`:

```console
kubectl delete cloudsqlinstance cloudsqlpostgresql
```

</div>
<div class="tab-pane fade" id="azure-tab-1" markdown="1">

The Azure provider supports provisioning an [Azure Database for PostgreSQL]
instance with the `PostgreSQLServer` managed resource it adds to Crossplane.

> Note: provisioning an Azure Database for PostgreSQL requires the presence of a
> [Resource Group] in your Azure account. We go ahead and provision a new
> `ResourceGroup` here in case you do not already have a suitable one in your
> account.

```yaml
apiVersion: azure.crossplane.io/v1alpha3
kind: ResourceGroup
metadata:
  name: sqlserverpostgresql-rg
spec:
  location: West US 2
---
apiVersion: database.azure.crossplane.io/v1beta1
kind: PostgreSQLServer
metadata:
  name: sqlserverpostgresql
spec:
  forProvider:
    administratorLogin: myadmin
    resourceGroupNameRef:
      name: sqlserverpostgresql-rg
    location: West US 2
    sslEnforcement: Disabled
    version: "9.6"
    sku:
      tier: GeneralPurpose
      capacity: 2
      family: Gen5
    storageProfile:
      storageMB: 20480
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: sqlserverpostgresql-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.8/docs/snippets/provision/azure.yaml
```

Creating the above instance will cause Crossplane to provision a PostgreSQL
database instance on Azure. You can view the progress with the following
command:

```console
kubectl get postgresqlserver sqlserverpostgresql
```

When provisioning is complete, you should see `READY: True` in the output. You
can take a look at its connection secret that is referenced under
`spec.writeConnectionSecretToRef`:

```console
kubectl describe secret sqlserverpostgresql-conn -n crossplane-system
```

You can then delete the `PostgreSQLServer`:

```console
kubectl delete postgresqlserver sqlserverpostgresql
kubectl delete resourcegroup sqlserverpostgresql-rg
```

</div>
</div>

In Kubernetes, `spec` top field represents the desired state of the user.
Crossplane adheres to that and has its own conventions about how the fields
under `spec` should look like.

* `writeConnectionSecretToRef`: A reference to the secret that you want this
  managed resource to write its connection secret that you'd be able to mount to
  your pods in the same namespace. For `RDSInstance`, this secret would contain
  `endpoint`, `username` and `password`.

* `providerConfigRef`: Reference to the `ProviderConfig` resource that will
  provide information regarding authentication of Crossplane to the provider.
  `ProviderConfig` resources refer to `Secret` and potentially contain other
  information regarding authentication. The `providerConfigRef` is defaulted to
  a `ProviderConfig` named `default` if omitted.

* `deletionPolicy`: Enum to specify whether the actual cloud resource should be
  deleted when this managed resource is deleted in Kubernetes API server.
  Possible values are `Delete` (the default) and `Orphan`.

* `forProvider`: While the rest of the fields relate to how Crossplane should
  behave, the fields under `forProvider` are solely used to configure the actual
  external resource. In most of the cases, the field names correspond to the
  what exists in provider's API Reference.

  The objects under `forProvider` field can get huge depending on the provider
  API. For example, GCP `ServiceAccount` has only a few fields while GCP
  `CloudSQLInstance` has over 100 fields that you can configure.

### Versioning

Crossplane closely follows the [Kubernetes API versioning
conventions][api-versioning] for the CRDs that it deploys. In short, for
`vXbeta` and `vX` versions, you can expect that either automatic migration or
instructions for manual migration will be provided when a new version of that
CRD schema is released.

In practice, we suggest the following guidelines to provider developers:
* Every new kind has to be introduced as `v1alpha1` with no exception.
* Breaking changes require a version change, i.e. `v1alpha1` needs to become
  `v1alpha2`.
  * Alpha resources don't require automatic conversions or manual instructions
    but it's recommended that manual instructions are provided.
  * Beta resources require at least manual instructions but it's recommended
    that conversion webhooks are used so that users can upgrade without any
    hands-on operation.
  * Stable resources require conversion webhooks.
* As long as the developer feels comfortable with the guarantees above, they can
  bump the version to beta or stable given that the CRD shape adheres to the
  Crossplane Resource Model (XRM) specifications for managed resources
  [here][managed-api-patterns].
* It's suggested that the bump from Alpha to Beta or from Beta to Stable happen
  after a bake period which includes at least one release.

### Grouping

In general, managed resources are high fidelity resources meaning they will
provide parameters and behaviors that are provided by the external resource API.
This applies to grouping of resources, too. For example, `Queue` appears under
`sqs` API group in AWS,so, its `APIVersion` and `Kind` look like the following:

```yaml
apiVersion: sqs.aws.crossplane.io/v1beta1
kind: Queue
```

## Behavior

As a general rule, managed resource controllers try not to make any decision
that is not specified by the user in the desired state since managed resources
are the lowest level primitives that operate directly on the cloud provider
APIs.

### Continuous Reconciliation

Crossplane providers continuously reconcile the managed resource to achieve the
desired state. The parameters under `spec` are considered the one and only
source of truth for the external resource. This means that if someone changed a
configuration in the UI of the provider, like AWS Console, Crossplane will
change it back to what's given under `spec`.

#### Connection Details

Some Crossplane resources support writing connection details - things like URLs,
usernames, endpoints, and passwords to a Kubernetes `Secret`. You can specify
the secret to write by setting the `spec.writeConnectionSecretToRef` field. Note
that while all managed resources have a `writeConnectionSecretToRef` field, not
all managed resources actually have connection details to write - many will
write an empty `Secret`.

> Which managed resources have connection details and what connection details
> they have is currently undocumented. This is tracked in [this
> issue][issue-1143].

#### Immutable Properties

There are configuration parameters in external resources that cloud providers do
not allow to be changed. For example, in AWS, you cannot change the region of an
`RDSInstance`.

Some infrastructure tools such as Terraform delete and recreate the resource to
accommodate those changes but Crossplane does not take that route. Unless the
managed resource is deleted and its `deletionPolicy` is `Delete`, its controller
never deletes the external resource in the provider.

> Kubernetes does not yet support immutable fields for custom resources. This
> means Crossplane will allow immutable fields to be changed, but will not
> actually make the desired change. This is tracked in [this issue][issue-727].

### External Name

By default the name of the managed resource is used as the name of the external
cloud resource that will show up in your cloud console. To specify a different
external name, Crossplane has a special annotation to represent the name of the
external resource. For example, I would like to have a `CloudSQLInstance` with
an external name that is different than its managed resource name:

```yaml
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstance
metadata:
  name: foodb
  annotations:
    crossplane.io/external-name: my-special-db
spec:
  ...
```

When you create this managed resource, you will see that the name of
`CloudSQLInstance` in GCP console will be `my-special-db`.

If the annotation is not given, Crossplane will fill it with the name of the
managed resource by default. In cases where provider doesn't allow you to name
the resource, like AWS VPC, the controller creates the resource and sets
external annotation to be the name that the cloud provider chose. So, you would
see something like `vpc-28dsnh3` as the value of `crossplane.io/external-name`
annotation of your AWS `VPC` resource even if you added your own custom external
name during creation.

### Late Initialization

For some of the optional fields, users rely on the default that the cloud
provider chooses for them. Since Crossplane treats the managed resource as the
source of the truth, values of those fields need to exist in `spec` of the
managed resource. So, in each reconciliation, Crossplane will fill the value of
a field that is left empty by the user but is assigned a value by the provider.
For example, there could be two fields like `region` and `availabilityZone` and
you might want to give only `region` and leave the availability zone to be
chosen by the cloud provider. In that case, if the provider assigns an
availability zone, Crossplane gets that value and fills `availabilityZone`. Note
that if the field is already filled, the controller won't override its value.

### Deletion

When a deletion request is made for a managed resource, its controller starts
the deletion process immediately. However, the managed resource is kept in the
Kubernetes API (via a finalizer) until the controller confirms the external
resource in the cloud is gone. So you can be sure that if the managed resource
is deleted, then the external cloud resource is also deleted. Any errors that
happen during deletion will be added to the `status` of the managed resource, so
you can troubleshoot any issues.

## Dependencies

In many cases, an external resource refers to another one for a specific
configuration. For example, you could want your Azure Kubernetes cluster in a
specific Virtual Network. External resources have specific fields for these
relations, however, they usually require the information to be supplied in
different formats. In Azure MySQL, you might be required to enter only the name
of the Virtual Network while in Azure Kubernetes, it could be required to enter
a string in a specific format that includes other information such as resource
group name.

In Crossplane, users have 3 fields to refer to another resource. Here is an
example from Azure MySQL managed resource referring to an Azure Resource Group:

```yaml
spec:
  forProvider:
    resourceGroupName: foo-res-group
    resourceGroupNameRef:
      name: resourcegroup
    resourceGroupNameSelector:
      matchLabels:
        app: prod
```

In this example, the user provided only a set of labels to select a
`ResourceGroup` managed resource that already exists in the cluster via
`resourceGroupNameSelector`. Then after a specific `ResourceGroup` is selected,
`resourceGroupNameRef` is filled with the name of that `ResourceGroup` managed
resource. Then in the last step, Crossplane fills the actual `resourceGroupName`
field with whatever format Azure accepts it. Once a dependency is resolved, the
controller never changes it.

Users are able to specify any of these three fields:

- Selector to select via labels
- Reference to point to a determined managed resource
- Actual value that will be submitted to the provider

It's important to note that in case a reference exists, the managed resource
does not create the external resource until the referenced object is ready. In
this example, creation call of Azure MySQL Server will not be made until
referenced `ResourceGroup` has its `status.condition` named `Ready` to be true.

## Importing Existing Resources

If you have some resources that are already provisioned in the cloud provider,
you can import them as managed resources and let Crossplane manage them. What
you need to do is to enter the name of the external resource as well as the
required fields on the managed resource. For example, let's say I have a GCP
Network provisioned from GCP console and I would like to migrate it to
Crossplane. Here is the YAML that I need to create:

```yaml
apiVersion: compute.gcp.crossplane.io/v1beta1
kind: Network
metadata:
  name: foo-network
  annotations:
    crossplane.io/external-name: existing-network
spec:
  forProvider: {}
  providerConfigRef:
    name: default
```

Crossplane will check whether a GCP Network called `existing-network` exists,
and if it does, then the optional fields under `forProvider` will be filled with
the values that are fetched from the provider.

Note that if a resource has required fields, you must fill those fields or the
creation of the managed resource will be rejected. So, in those cases, you will
need to enter the name of the resource as well as the required fields as
indicated in the [API Reference][api-reference] documentation.

## Backup and Restore

Crossplane adheres to Kubernetes conventions as much as possible and one of the
advantages we gain is backup & restore ability with tools that work with native
Kubernetes types, like [Velero][velero].

If you'd like to backup and restore manually, you can simply export them and
save YAMLs in your file system. When you reload them, as we've discovered in
import section, their `crossplane.io/external-name` annotation and required
fields are there and those are enough to import a resource. The tool you're
using needs to store `annotations` and `spec` fields, which most tools do
including Velero.

[term-xrm]: terminology.md#crossplane-resource-model
[composition]: composition.md
[api-versioning]: https://kubernetes.io/docs/reference/using-api/api-overview/#api-versioning
[velero]: https://velero.io/
[api-reference]: ../api-docs/overview.md
[provider]: providers.md
[issue-727]: https://github.com/crossplane/crossplane/issues/727
[issue-1143]: https://github.com/crossplane/crossplane/issues/1143
[managed-api-patterns]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-managed-resource-api-design.md
