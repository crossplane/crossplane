# database.azure.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for Azure database services such as SQL server.

This API group contains the following Crossplane resources:

* [MysqlServer](#MysqlServer)
* [MysqlServerVirtualNetworkRule](#MysqlServerVirtualNetworkRule)
* [PostgresqlServer](#PostgresqlServer)
* [PostgresqlServerVirtualNetworkRule](#PostgresqlServerVirtualNetworkRule)
* [SQLServerClass](#SQLServerClass)

## MysqlServer

A MysqlServer is a managed resource that represents an Azure MySQL Database Server.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha2`
`kind` | string | `MysqlServer`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SQLServerSpec](#SQLServerSpec) | A SQLServerSpec defines the desired state of a SQLServer.
`status` | [SQLServerStatus](#SQLServerStatus) | A SQLServerStatus represents the observed state of a SQLServer.



## MysqlServerVirtualNetworkRule

A MysqlServerVirtualNetworkRule is a managed resource that represents an Azure MySQL Database virtual network rule.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha2`
`kind` | string | `MysqlServerVirtualNetworkRule`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [VirtualNetworkRuleSpec](#VirtualNetworkRuleSpec) | A VirtualNetworkRuleSpec defines the desired state of a VirtualNetworkRule.
`status` | [VirtualNetworkRuleStatus](#VirtualNetworkRuleStatus) | A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.



## PostgresqlServer

A PostgresqlServer is a managed resource that represents an Azure PostgreSQL Database Server.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha2`
`kind` | string | `PostgresqlServer`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SQLServerSpec](#SQLServerSpec) | A SQLServerSpec defines the desired state of a SQLServer.
`status` | [SQLServerStatus](#SQLServerStatus) | A SQLServerStatus represents the observed state of a SQLServer.



## PostgresqlServerVirtualNetworkRule

A PostgresqlServerVirtualNetworkRule is a managed resource that represents an Azure PostgreSQL Database virtual network rule.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha2`
`kind` | string | `PostgresqlServerVirtualNetworkRule`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [VirtualNetworkRuleSpec](#VirtualNetworkRuleSpec) | A VirtualNetworkRuleSpec defines the desired state of a VirtualNetworkRule.
`status` | [VirtualNetworkRuleStatus](#VirtualNetworkRuleStatus) | A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.



## SQLServerClass

A SQLServerClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha2`
`kind` | string | `SQLServerClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [SQLServerClassSpecTemplate](#SQLServerClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned SQLServer.



## PricingTierSpec

PricingTierSpec represents the performance and cost oriented properties of a SQLServer.

Appears in:

* [SQLServerParameters](#SQLServerParameters)


Name | Type | Description
-----|------|------------
`tier` | string | Tier of the particular SKU, e.g. Basic. Possible values include: &#39;Basic&#39;, &#39;GeneralPurpose&#39;, &#39;MemoryOptimized&#39;
`vcores` | int | VCores (aka Capacity) specifies how many virtual cores this SQLServer requires.
`family` | string | Family of hardware.



## SQLServer

SQLServer represents a generic Azure SQL server.


## SQLServerClassSpecTemplate

A SQLServerClassSpecTemplate is a template for the spec of a dynamically provisioned MysqlServer or PostgresqlServer.

Appears in:

* [SQLServerClass](#SQLServerClass)




SQLServerClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [SQLServerParameters](#SQLServerParameters)


## SQLServerParameters

SQLServerParameters define the desired state of an Azure SQL Database, either PostgreSQL or MySQL.

Appears in:

* [SQLServerClassSpecTemplate](#SQLServerClassSpecTemplate)
* [SQLServerSpec](#SQLServerSpec)


Name | Type | Description
-----|------|------------
`resourceGroupName` | string | ResourceGroupName specifies the name of the resource group that should contain this SQLServer.
`location` | string | Location specifies the location of this SQLServer.
`pricingTier` | [PricingTierSpec](#PricingTierSpec) | PricingTier specifies the pricing tier (aka SKU) for this SQLServer.
`storageProfile` | [StorageProfileSpec](#StorageProfileSpec) | StorageProfile configures the storage profile of this SQLServer.
`adminLoginName` | string | AdminLoginName specifies the administrator login name for this SQLServer.
`version` | string | Version specifies the version of this server, for example &#34;5.6&#34;, or &#34;9.6&#34;.
`sslEnforced` | Optional bool | SSLEnforced specifies whether SSL is required to connect to this SQLServer.



## SQLServerSpec

A SQLServerSpec defines the desired state of a SQLServer.

Appears in:

* [MysqlServer](#MysqlServer)
* [PostgresqlServer](#PostgresqlServer)




SQLServerSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [SQLServerParameters](#SQLServerParameters)


## SQLServerStatus

A SQLServerStatus represents the observed state of a SQLServer.

Appears in:

* [MysqlServer](#MysqlServer)
* [PostgresqlServer](#PostgresqlServer)


Name | Type | Description
-----|------|------------
`state` | string | State of this SQLServer.
`message` | string | A Message containing detail on the state of this SQLServer, if any.
`providerID` | string | ProviderID is the external ID to identify this resource in the cloud provider.
`endpoint` | string | Endpoint of the MySQL Server instance used in connection strings.
`runningOperation` | string | RunningOperation stores any current long running operation for this instance across reconciliation attempts.
`runningOperationType` | string | RunningOperationType is the type of the currently running operation.


SQLServerStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## StorageProfileSpec

A StorageProfileSpec represents storage related properties of a SQLServer.

Appears in:

* [SQLServerParameters](#SQLServerParameters)


Name | Type | Description
-----|------|------------
`storageGB` | int | StorageGB configures the maximum storage allowed.
`backupRetentionDays` | int | BackupRetentionDays configures how many days backups will be retained.
`geoRedundantBackup` | bool | GeoRedundantBackup enables geo-redunndant backups.



## VirtualNetworkRuleProperties

VirtualNetworkRuleProperties defines the properties of a VirtualNetworkRule.

Appears in:

* [VirtualNetworkRuleSpec](#VirtualNetworkRuleSpec)


Name | Type | Description
-----|------|------------
`virtualNetworkSubnetId` | string | VirtualNetworkSubnetID - The ARM resource id of the virtual network subnet.
`ignoreMissingVnetServiceEndpoint` | bool | IgnoreMissingVnetServiceEndpoint - Create firewall rule before the virtual network has vnet service endpoint enabled.



## VirtualNetworkRuleSpec

A VirtualNetworkRuleSpec defines the desired state of a VirtualNetworkRule.

Appears in:

* [MysqlServerVirtualNetworkRule](#MysqlServerVirtualNetworkRule)
* [PostgresqlServerVirtualNetworkRule](#PostgresqlServerVirtualNetworkRule)


Name | Type | Description
-----|------|------------
`name` | string | Name - Name of the Virtual Network Rule.
`serverName` | string | ServerName - Name of the Virtual Network Rule&#39;s server.
`resourceGroupName` | string | ResourceGroupName - Name of the Virtual Network Rule&#39;s resource group.
`properties` | [VirtualNetworkRuleProperties](#VirtualNetworkRuleProperties) | VirtualNetworkRuleProperties - Resource properties.


VirtualNetworkRuleSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## VirtualNetworkRuleStatus

A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.

Appears in:

* [MysqlServerVirtualNetworkRule](#MysqlServerVirtualNetworkRule)
* [PostgresqlServerVirtualNetworkRule](#PostgresqlServerVirtualNetworkRule)


Name | Type | Description
-----|------|------------
`state` | string | State of this virtual network rule.
`message` | string | A Message containing details about the state of this virtual network rule, if any.
`id` | string | ID - Resource ID
`type` | string | Type - Resource type.


VirtualNetworkRuleStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


This API documentation was generated by `crossdocs`.