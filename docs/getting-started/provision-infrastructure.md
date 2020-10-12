---
title: Provision Infrastructure
toc: true
weight: 3
indent: true
---

# Provision Infrastructure

Crossplane allows you to provision infrastructure anywhere using the Kubernetes
API. Once you have [installed a provider] and [configured your credentials], you
can create any infrastructure currently supported by the provider. Let's start
by provisioning a database on your provider of choice.

Each provider below offers their own flavor of a managed database. When you
install a provider it extends Crossplane by adding support for several "managed
resources". A managed resource is a cluster-scoped Kubernetes custom resource
that represents an infrastructure object, such as a database instance.

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-1" data-toggle="tab">AWS</a></li>
<li><a href="#gcp-tab-1" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-1" data-toggle="tab">Azure</a></li>
<li><a href="#alibaba-tab-1" data-toggle="tab">Alibaba</a></li>
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
    dbInstanceClass: db.t2.small
    masterUsername: masteruser
    allocatedStorage: 20
    engine: postgres
    engineVersion: "9.6"
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
<div class="tab-pane fade" id="alibaba-tab-1" markdown="1">

The Alibaba provider supports provisioning an [ApsaraDB for RDS] instance with
the `RDSInstance` managed resource it adds to Crossplane.

```yaml
apiVersion: database.alibaba.crossplane.io/v1alpha1
kind: RDSInstance
metadata:
  name: rdspostgresql
spec:
  forProvider:
    engine: PostgreSQL
    engineVersion: "9.4"
    dbInstanceClass: rds.pg.s1.small
    dbInstanceStorageInGB: 20
    securityIPList: "0.0.0.0/0"
    masterUsername: "test123"
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: alibaba-rdspostgresql-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/provision/alibaba.yaml
```

Creating the above instance will cause Crossplane to provision an RDS instance
on Alibaba. You can view the progress with the following command:

```console
kubectl get rdsinstance rdspostgresql
```

When provisioning is complete, you should see `READY: True` in the output. You
can take a look at its connection secret that is referenced under `spec.writeConnectionSecretToRef`:

```console
kubectl describe secret alibaba-rdspostgresql-conn -n crossplane-system
```

You can then delete the `RDSInstance`:

```console
kubectl delete rdsinstance rdspostgresql
```

</div>
</div>

## Next Steps

Now that you have seen how to provision individual managed resources, let's take
a look at how we can compose several managed resources into new resources with
APIs of our choosing in the [next section].

<!-- Named Links -->

[installed a provider]: install-configure.md
[configured your credentials]: install-configure.md
[RDS]: https://aws.amazon.com/rds/
[CloudSQL]: https://cloud.google.com/sql
[Azure Database for PostgreSQL]: https://azure.microsoft.com/en-us/services/postgresql/
[Resource Group]: https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal#what-is-a-resource-group
[ApsaraDB for RDS]: https://www.alibabacloud.com/product/apsaradb-for-rds-postgresql
[next section]: compose-infrastructure.md
