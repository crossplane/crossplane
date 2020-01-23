# database.azure.crossplane.io/v1beta1 API Reference

Package v1beta1 contains managed resources for Azure database services such as SQL server.

This API group contains the following Crossplane resources:

* [MySQLServer](#MySQLServer)
* [PostgreSQLServer](#PostgreSQLServer)
* [SQLServerClass](#SQLServerClass)

## MySQLServer

A MySQLServer is a managed resource that represents an Azure MySQL Database Server.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1beta1`
`kind` | string | `MySQLServer`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SQLServerSpec](#SQLServerSpec) | A SQLServerSpec defines the desired state of a SQLServer.
`status` | [SQLServerStatus](#SQLServerStatus) | A SQLServerStatus represents the observed state of a SQLServer.



## PostgreSQLServer

A PostgreSQLServer is a managed resource that represents an Azure PostgreSQL Database Server.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1beta1`
`kind` | string | `PostgreSQLServer`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SQLServerSpec](#SQLServerSpec) | A SQLServerSpec defines the desired state of a SQLServer.
`status` | [SQLServerStatus](#SQLServerStatus) | A SQLServerStatus represents the observed state of a SQLServer.



## SQLServerClass

A SQLServerClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1beta1`
`kind` | string | `SQLServerClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [SQLServerClassSpecTemplate](#SQLServerClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned SQLServer.



## MySQLServerNameReferencer

A MySQLServerNameReferencer returns the server name of a referenced MySQLServer.




MySQLServerNameReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## PostgreSQLServerNameReferencer

A PostgreSQLServerNameReferencer returns the server name of a referenced PostgreSQLServer.




PostgreSQLServerNameReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## ResourceGroupNameReferencerForSQLServer

ResourceGroupNameReferencerForSQLServer is an attribute referencer that resolves the name of a the ResourceGroup.

Appears in:

* [SQLServerParameters](#SQLServerParameters)




ResourceGroupNameReferencerForSQLServer supports all fields of:

* github.com/crossplaneio/stack-azure/apis/v1alpha3.ResourceGroupNameReferencer


## SKU

SKU billing information related properties of a server.

Appears in:

* [SQLServerParameters](#SQLServerParameters)


Name | Type | Description
-----|------|------------
`tier` | string | Tier - The tier of the particular SKU. Possible values include: &#39;Basic&#39;, &#39;GeneralPurpose&#39;, &#39;MemoryOptimized&#39;
`capacity` | int | Capacity - The scale up/out capacity, representing server&#39;s compute units.
`size` | Optional string | Size - The size code, to be interpreted by resource as appropriate.
`family` | string | Family - The family of hardware.



## SQLServerClassSpecTemplate

A SQLServerClassSpecTemplate is a template for the spec of a dynamically provisioned MySQLServer or PostgreSQLServer.

Appears in:

* [SQLServerClass](#SQLServerClass)


Name | Type | Description
-----|------|------------
`forProvider` | [SQLServerParameters](#SQLServerParameters) | SQLServerParameters define the desired state of an Azure SQL Database, either PostgreSQL or MySQL.


SQLServerClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)


## SQLServerObservation

SQLServerObservation represents the current state of Azure SQL resource.

Appears in:

* [SQLServerStatus](#SQLServerStatus)


Name | Type | Description
-----|------|------------
`id` | string | ID - Resource ID
`name` | string | Name - Resource name.
`type` | string | Type - Resource type.
`userVisibleState` | string | UserVisibleState - A state of a server that is visible to user.
`fullyQualifiedDomainName` | string | FullyQualifiedDomainName - The fully qualified domain name of a server.
`masterServerId` | string | MasterServerID - The master server id of a replica server.
`lastOperation` | github.com/crossplaneio/stack-azure/apis/v1alpha3.AsyncOperation | LastOperation represents the state of the last operation started by the controller.



## SQLServerParameters

SQLServerParameters define the desired state of an Azure SQL Database, either PostgreSQL or MySQL.

Appears in:

* [SQLServerClassSpecTemplate](#SQLServerClassSpecTemplate)
* [SQLServerSpec](#SQLServerSpec)


Name | Type | Description
-----|------|------------
`resourceGroupName` | string | ResourceGroupName specifies the name of the resource group that should contain this SQLServer.
`resourceGroupNameRef` | [ResourceGroupNameReferencerForSQLServer](#ResourceGroupNameReferencerForSQLServer) | ResourceGroupNameRef - A reference to a ResourceGroup object to retrieve its name
`sku` | [SKU](#SKU) | SKU is the billing information related properties of the server.
`location` | string | Location specifies the location of this SQLServer.
`administratorLogin` | string | AdministratorLogin - The administrator&#39;s login name of a server. Can only be specified when the server is being created (and is required for creation).
`tags` | Optional map[string]string | Tags - Application-specific metadata in the form of key-value pairs.
`version` | string | Version - Server version.
`sslEnforcement` | string | SSLEnforcement - Enable ssl enforcement or not when connect to server. Possible values include: &#39;Enabled&#39;, &#39;Disabled&#39;
`storageProfile` | [StorageProfile](#StorageProfile) | StorageProfile - Storage profile of a server.



## SQLServerSpec

A SQLServerSpec defines the desired state of a SQLServer.

Appears in:

* [MySQLServer](#MySQLServer)
* [PostgreSQLServer](#PostgreSQLServer)


Name | Type | Description
-----|------|------------
`forProvider` | [SQLServerParameters](#SQLServerParameters) | SQLServerParameters define the desired state of an Azure SQL Database, either PostgreSQL or MySQL.


SQLServerSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## SQLServerStatus

A SQLServerStatus represents the observed state of a SQLServer.

Appears in:

* [MySQLServer](#MySQLServer)
* [PostgreSQLServer](#PostgreSQLServer)


Name | Type | Description
-----|------|------------
`atProvider` | [SQLServerObservation](#SQLServerObservation) | SQLServerObservation represents the current state of Azure SQL resource.


SQLServerStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## StorageProfile

StorageProfile storage Profile properties of a server

Appears in:

* [SQLServerParameters](#SQLServerParameters)


Name | Type | Description
-----|------|------------
`backupRetentionDays` | Optional int | BackupRetentionDays - Backup retention days for the server.
`geoRedundantBackup` | Optional string | GeoRedundantBackup - Enable Geo-redundant or not for server backup. Possible values include: &#39;Enabled&#39;, &#39;Disabled&#39;
`storageMB` | int | StorageMB - Max storage allowed for a server.
`storageAutogrow` | Optional string | StorageAutogrow - Enable Storage Auto Grow. Possible values include: &#39;Enabled&#39;, &#39;Disabled&#39;



This API documentation was generated by `crossdocs`.