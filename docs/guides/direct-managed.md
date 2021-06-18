---
title: Managed Resources
toc: true
weight: 250
indent: true
---

# Using Managed Resources Directly

Crossplane allows you to provision infrastructure anywhere using the Kubernetes
API. While users are encouraged to make use of [composition] to expose
infrastructure resources, you may opt to use managed resources directly. Once
you have [installed a provider] and [configured your credentials], you can
create any infrastructure currently supported by the provider. Let's start by
provisioning a database on your provider of choice.

Each provider below offers their own flavor of a managed database. When you
install a provider it extends Crossplane by adding support for several "managed
resources". A managed resource is a cluster-scoped Kubernetes custom resource
that represents an infrastructure object, such as a database instance. Managed
resources are cluster-scoped because they are only intended to be used directly
when an infrastructure admin is creating a single resource that is intended to
be shared across teams and namespaces. Infrastructure consumers, such as
application teams, are expected to _always_ provision and interact with
infrastructure via claims (XRCs).

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
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/provision/aws.yaml
```

Creating the above instance will cause Crossplane to provision an RDS instance
on AWS. You can view the progress with the following command:

```console
kubectl get rdsinstance rdspostgresql
```

When provisioning is complete, you should see `READY: True` in the output. You
can take a look at its connection secret that is referenced under `spec.writeConnectionSecretToRef`:

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
    databaseVersion: POSTGRES_9_6
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
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/provision/gcp.yaml
```

Creating the above instance will cause Crossplane to provision a CloudSQL
instance on GCP. You can view the progress with the following command:

```console
kubectl get cloudsqlinstance cloudsqlpostgresql
```

When provisioning is complete, you should see `READY: True` in the output. You
can take a look at its connection secret that is referenced under `spec.writeConnectionSecretToRef`:

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
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/provision/azure.yaml
```

Creating the above instance will cause Crossplane to provision a PostgreSQL
database instance on Azure. You can view the progress with the following
command:

```console
kubectl get postgresqlserver sqlserverpostgresql
```

When provisioning is complete, you should see `READY: True` in the output. You
can take a look at its connection secret that is referenced under `spec.writeConnectionSecretToRef`:

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

## Clean Up

Let's check whether there are any managed resources before deleting the
provider.

```console
kubectl get managed
```

If there are any, please delete them first, so you don't lose the track of them.
Then delete all the `ProviderConfig`s you created. An example command if you used
AWS Provider:
```
kubectl delete providerconfig.aws --all
```

List installed providers:
```console
kubectl get provider.pkg
```

Delete the one you want to delete:
```
kubectl delete provider.pkg <provider-name>
```

<!-- Named Links -->

[composition]: ../concepts/composition.md
[installed a provider]: ../concepts/providers.md
[configured your credentials]: ../concepts/providers.md
[RDS]: https://aws.amazon.com/rds/
[CloudSQL]: https://cloud.google.com/sql
[Azure Database for PostgreSQL]: https://azure.microsoft.com/en-us/services/postgresql/
[Resource Group]: https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal#what-is-a-resource-group
[ApsaraDB for RDS]: https://www.alibabacloud.com/product/apsaradb-for-rds-postgresql
