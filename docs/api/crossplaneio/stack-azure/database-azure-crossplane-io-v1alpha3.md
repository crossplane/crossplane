# database.azure.crossplane.io/v1alpha3 API Reference

Package v1alpha3 contains managed resources for Azure database services such as SQL server.

This API group contains the following Crossplane resources:

* [MySQLServer](#MySQLServer)
* [MySQLServerVirtualNetworkRule](#MySQLServerVirtualNetworkRule)
* [PostgreSQLServer](#PostgreSQLServer)
* [PostgreSQLServerVirtualNetworkRule](#PostgreSQLServerVirtualNetworkRule)
* [SQLServerClass](#SQLServerClass)

## MySQLServer

A MySQLServer is a managed resource that represents an Azure MySQL Database Server.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha3`
`kind` | string | `MySQLServer`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SQLServerSpec](#SQLServerSpec) | A SQLServerSpec defines the desired state of a SQLServer.
`status` | [SQLServerStatus](#SQLServerStatus) | A SQLServerStatus represents the observed state of a SQLServer.



## MySQLServerVirtualNetworkRule

A MySQLServerVirtualNetworkRule is a managed resource that represents an Azure MySQL Database virtual network rule.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha3`
`kind` | string | `MySQLServerVirtualNetworkRule`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [MySQLVirtualNetworkRuleSpec](#MySQLVirtualNetworkRuleSpec) | A MySQLVirtualNetworkRuleSpec defines the desired state of a MySQLVirtualNetworkRule.
`status` | [VirtualNetworkRuleStatus](#VirtualNetworkRuleStatus) | A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.



## PostgreSQLServer

A PostgreSQLServer is a managed resource that represents an Azure PostgreSQL Database Server.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha3`
`kind` | string | `PostgreSQLServer`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SQLServerSpec](#SQLServerSpec) | A SQLServerSpec defines the desired state of a SQLServer.
`status` | [SQLServerStatus](#SQLServerStatus) | A SQLServerStatus represents the observed state of a SQLServer.



## PostgreSQLServerVirtualNetworkRule

A PostgreSQLServerVirtualNetworkRule is a managed resource that represents an Azure PostgreSQL Database virtual network rule.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha3`
`kind` | string | `PostgreSQLServerVirtualNetworkRule`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [PostgreSQLVirtualNetworkRuleSpec](#PostgreSQLVirtualNetworkRuleSpec) | A PostgreSQLVirtualNetworkRuleSpec defines the desired state of a PostgreSQLVirtualNetworkRule.
`status` | [VirtualNetworkRuleStatus](#VirtualNetworkRuleStatus) | A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.



## SQLServerClass

A SQLServerClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha3`
`kind` | string | `SQLServerClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [SQLServerClassSpecTemplate](#SQLServerClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned SQLServer.



## MySQLServerNameReferencer

A MySQLServerNameReferencer returns the server name of a referenced MySQLServer.

Appears in:

* [MySQLVirtualNetworkRuleSpec](#MySQLVirtualNetworkRuleSpec)
* [ServerNameReferencerForMySQLServerVirtualNetworkRule](#ServerNameReferencerForMySQLServerVirtualNetworkRule)




MySQLServerNameReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## MySQLVirtualNetworkRuleSpec

A MySQLVirtualNetworkRuleSpec defines the desired state of a MySQLVirtualNetworkRule.

Appears in:

* [MySQLServerVirtualNetworkRule](#MySQLServerVirtualNetworkRule)


Name | Type | Description
-----|------|------------
`name` | string | Name - Name of the Virtual Network Rule.
`serverName` | string | ServerName - Name of the Virtual Network Rule&#39;s server.
`serverNameRef` | [MySQLServerNameReferencer](#MySQLServerNameReferencer) | ServerNameRef - A reference to the Virtual Network Rule&#39;s MySQLServer.
`resourceGroupName` | string | ResourceGroupName - Name of the Virtual Network Rule&#39;s resource group.
`resourceGroupNameRef` | [ResourceGroupNameReferencerForVirtualNetworkRule](#ResourceGroupNameReferencerForVirtualNetworkRule) | ResourceGroupNameRef - A reference to a ResourceGroup object to retrieve its name
`properties` | [VirtualNetworkRuleProperties](#VirtualNetworkRuleProperties) | VirtualNetworkRuleProperties - Resource properties.


MySQLVirtualNetworkRuleSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## PostgreSQLServerNameReferencer

A PostgreSQLServerNameReferencer returns the server name of a referenced PostgreSQLServer.

Appears in:

* [PostgreSQLVirtualNetworkRuleSpec](#PostgreSQLVirtualNetworkRuleSpec)
* [ServerNameReferencerForPostgreSQLServerVirtualNetworkRule](#ServerNameReferencerForPostgreSQLServerVirtualNetworkRule)




PostgreSQLServerNameReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## PostgreSQLVirtualNetworkRuleSpec

A PostgreSQLVirtualNetworkRuleSpec defines the desired state of a PostgreSQLVirtualNetworkRule.

Appears in:

* [PostgreSQLServerVirtualNetworkRule](#PostgreSQLServerVirtualNetworkRule)


Name | Type | Description
-----|------|------------
`name` | string | Name - Name of the Virtual Network Rule.
`serverName` | string | ServerName - Name of the Virtual Network Rule&#39;s PostgreSQLServer.
`serverNameRef` | [PostgreSQLServerNameReferencer](#PostgreSQLServerNameReferencer) | ServerNameRef - A reference to the Virtual Network Rule&#39;s PostgreSQLServer.
`resourceGroupName` | string | ResourceGroupName - Name of the Virtual Network Rule&#39;s resource group.
`resourceGroupNameRef` | [ResourceGroupNameReferencerForVirtualNetworkRule](#ResourceGroupNameReferencerForVirtualNetworkRule) | ResourceGroupNameRef - A reference to a ResourceGroup object to retrieve its name
`properties` | [VirtualNetworkRuleProperties](#VirtualNetworkRuleProperties) | VirtualNetworkRuleProperties - Resource properties.


PostgreSQLVirtualNetworkRuleSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## PricingTierSpec

PricingTierSpec represents the performance and cost oriented properties of a SQLServer.

Appears in:

* [SQLServerParameters](#SQLServerParameters)


Name | Type | Description
-----|------|------------
`tier` | string | Tier of the particular SKU, e.g. Basic. Possible values include: &#39;Basic&#39;, &#39;GeneralPurpose&#39;, &#39;MemoryOptimized&#39;
`vcores` | int | VCores (aka Capacity) specifies how many virtual cores this SQLServer requires.
`family` | string | Family of hardware.



## ResourceGroupNameReferencerForSQLServer

ResourceGroupNameReferencerForSQLServer is an attribute referencer that resolves the name of a the ResourceGroup.

Appears in:

* [SQLServerParameters](#SQLServerParameters)




ResourceGroupNameReferencerForSQLServer supports all fields of:

* github.com/crossplaneio/stack-azure/apis/v1alpha3.ResourceGroupNameReferencer


## ResourceGroupNameReferencerForVirtualNetworkRule

ResourceGroupNameReferencerForVirtualNetworkRule is an attribute referencer that resolves the name of a the ResourceGroup.

Appears in:

* [MySQLVirtualNetworkRuleSpec](#MySQLVirtualNetworkRuleSpec)
* [PostgreSQLVirtualNetworkRuleSpec](#PostgreSQLVirtualNetworkRuleSpec)




ResourceGroupNameReferencerForVirtualNetworkRule supports all fields of:

* github.com/crossplaneio/stack-azure/apis/v1alpha3.ResourceGroupNameReferencer


## SQLServerClassSpecTemplate

A SQLServerClassSpecTemplate is a template for the spec of a dynamically provisioned MySQLServer or PostgreSQLServer.

Appears in:

* [SQLServerClass](#SQLServerClass)




SQLServerClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)
* [SQLServerParameters](#SQLServerParameters)


## SQLServerParameters

SQLServerParameters define the desired state of an Azure SQL Database, either PostgreSQL or MySQL.

Appears in:

* [SQLServerClassSpecTemplate](#SQLServerClassSpecTemplate)
* [SQLServerSpec](#SQLServerSpec)


Name | Type | Description
-----|------|------------
`resourceGroupName` | string | ResourceGroupName specifies the name of the resource group that should contain this SQLServer.
`resourceGroupNameRef` | [ResourceGroupNameReferencerForSQLServer](#ResourceGroupNameReferencerForSQLServer) | ResourceGroupNameRef - A reference to a ResourceGroup object to retrieve its name
`location` | string | Location specifies the location of this SQLServer.
`pricingTier` | [PricingTierSpec](#PricingTierSpec) | PricingTier specifies the pricing tier (aka SKU) for this SQLServer.
`storageProfile` | [StorageProfileSpec](#StorageProfileSpec) | StorageProfile configures the storage profile of this SQLServer.
`adminLoginName` | string | AdminLoginName specifies the administrator login name for this SQLServer.
`version` | string | Version specifies the version of this server, for example &#34;5.6&#34;, or &#34;9.6&#34;.
`sslEnforced` | Optional bool | SSLEnforced specifies whether SSL is required to connect to this SQLServer.



## SQLServerSpec

A SQLServerSpec defines the desired state of a SQLServer.

Appears in:

* [MySQLServer](#MySQLServer)
* [PostgreSQLServer](#PostgreSQLServer)




SQLServerSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [SQLServerParameters](#SQLServerParameters)


## SQLServerStatus

A SQLServerStatus represents the observed state of a SQLServer.

Appears in:

* [MySQLServer](#MySQLServer)
* [PostgreSQLServer](#PostgreSQLServer)


Name | Type | Description
-----|------|------------
`state` | string | State of this SQLServer.
`message` | string | A Message containing detail on the state of this SQLServer, if any.
`providerID` | string | ProviderID is the external ID to identify this resource in the cloud provider.
`endpoint` | string | Endpoint of the MySQL Server instance used in connection strings.


SQLServerStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## ServerNameReferencerForMySQLServerVirtualNetworkRule

ServerNameReferencerForMySQLServerVirtualNetworkRule is an attribute referencer that resolves the name of a MySQLServer.




ServerNameReferencerForMySQLServerVirtualNetworkRule supports all fields of:

* [MySQLServerNameReferencer](#MySQLServerNameReferencer)


## ServerNameReferencerForPostgreSQLServerVirtualNetworkRule

ServerNameReferencerForPostgreSQLServerVirtualNetworkRule is an attribute referencer that resolves the name of a PostgreSQLServer.




ServerNameReferencerForPostgreSQLServerVirtualNetworkRule supports all fields of:

* [PostgreSQLServerNameReferencer](#PostgreSQLServerNameReferencer)


## StorageProfileSpec

A StorageProfileSpec represents storage related properties of a SQLServer.

Appears in:

* [SQLServerParameters](#SQLServerParameters)


Name | Type | Description
-----|------|------------
`storageGB` | int | StorageGB configures the maximum storage allowed.
`backupRetentionDays` | int | BackupRetentionDays configures how many days backups will be retained.
`geoRedundantBackup` | bool | GeoRedundantBackup enables geo-redunndant backups.



## SubnetIDReferencerForVirtualNetworkRule

SubnetIDReferencerForVirtualNetworkRule is an attribute referencer that resolves id from a referenced Subnet and assigns it to a PostgreSQLServer or MySQL server object

Appears in:

* [VirtualNetworkRuleProperties](#VirtualNetworkRuleProperties)




SubnetIDReferencerForVirtualNetworkRule supports all fields of:

* github.com/crossplaneio/stack-azure/apis/network/v1alpha3.SubnetIDReferencer


## VirtualNetworkRuleProperties

VirtualNetworkRuleProperties defines the properties of a VirtualNetworkRule.

Appears in:

* [MySQLVirtualNetworkRuleSpec](#MySQLVirtualNetworkRuleSpec)
* [PostgreSQLVirtualNetworkRuleSpec](#PostgreSQLVirtualNetworkRuleSpec)


Name | Type | Description
-----|------|------------
`virtualNetworkSubnetId` | string | VirtualNetworkSubnetID - The ARM resource id of the virtual network subnet.
`virtualNetworkSubnetIdRef` | [SubnetIDReferencerForVirtualNetworkRule](#SubnetIDReferencerForVirtualNetworkRule) | VirtualNetworkSubnetIDRef - A reference to a Subnet to retrieve its ID
`ignoreMissingVnetServiceEndpoint` | bool | IgnoreMissingVnetServiceEndpoint - Create firewall rule before the virtual network has vnet service endpoint enabled.



## VirtualNetworkRuleStatus

A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.

Appears in:

* [MySQLServerVirtualNetworkRule](#MySQLServerVirtualNetworkRule)
* [PostgreSQLServerVirtualNetworkRule](#PostgreSQLServerVirtualNetworkRule)


Name | Type | Description
-----|------|------------
`state` | string | State of this virtual network rule.
`message` | string | A Message containing details about the state of this virtual network rule, if any.
`id` | string | ID - Resource ID
`type` | string | Type - Resource type.


VirtualNetworkRuleStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


This API documentation was generated by `crossdocs`.