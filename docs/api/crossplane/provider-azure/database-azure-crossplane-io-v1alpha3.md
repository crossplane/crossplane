# database.azure.crossplane.io/v1alpha3 API Reference

Package v1alpha3 contains managed resources for Azure database services.

This API group contains the following Crossplane resources:

* [MySQLServerVirtualNetworkRule](#MySQLServerVirtualNetworkRule)
* [PostgreSQLServerVirtualNetworkRule](#PostgreSQLServerVirtualNetworkRule)

## MySQLServerVirtualNetworkRule

A MySQLServerVirtualNetworkRule is a managed resource that represents an Azure MySQL Database virtual network rule.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha3`
`kind` | string | `MySQLServerVirtualNetworkRule`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [MySQLVirtualNetworkRuleSpec](#MySQLVirtualNetworkRuleSpec) | A MySQLVirtualNetworkRuleSpec defines the desired state of a MySQLVirtualNetworkRule.
`status` | [VirtualNetworkRuleStatus](#VirtualNetworkRuleStatus) | A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.



## PostgreSQLServerVirtualNetworkRule

A PostgreSQLServerVirtualNetworkRule is a managed resource that represents an Azure PostgreSQL Database virtual network rule.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.azure.crossplane.io/v1alpha3`
`kind` | string | `PostgreSQLServerVirtualNetworkRule`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [PostgreSQLVirtualNetworkRuleSpec](#PostgreSQLVirtualNetworkRuleSpec) | A PostgreSQLVirtualNetworkRuleSpec defines the desired state of a PostgreSQLVirtualNetworkRule.
`status` | [VirtualNetworkRuleStatus](#VirtualNetworkRuleStatus) | A VirtualNetworkRuleStatus represents the observed state of a VirtualNetworkRule.



## MySQLVirtualNetworkRuleSpec

A MySQLVirtualNetworkRuleSpec defines the desired state of a MySQLVirtualNetworkRule.

Appears in:

* [MySQLServerVirtualNetworkRule](#MySQLServerVirtualNetworkRule)


Name | Type | Description
-----|------|------------
`name` | string | Name - Name of the Virtual Network Rule.
`serverName` | string | ServerName - Name of the Virtual Network Rule&#39;s server.
`serverNameRef` | github.com/crossplane/provider-azure/apis/database/v1beta1.MySQLServerNameReferencer | ServerNameRef - A reference to the Virtual Network Rule&#39;s MySQLServer.
`resourceGroupName` | string | ResourceGroupName - Name of the Virtual Network Rule&#39;s resource group.
`resourceGroupNameRef` | [ResourceGroupNameReferencerForVirtualNetworkRule](#ResourceGroupNameReferencerForVirtualNetworkRule) | ResourceGroupNameRef - A reference to a ResourceGroup object to retrieve its name
`properties` | [VirtualNetworkRuleProperties](#VirtualNetworkRuleProperties) | VirtualNetworkRuleProperties - Resource properties.


MySQLVirtualNetworkRuleSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## PostgreSQLVirtualNetworkRuleSpec

A PostgreSQLVirtualNetworkRuleSpec defines the desired state of a PostgreSQLVirtualNetworkRule.

Appears in:

* [PostgreSQLServerVirtualNetworkRule](#PostgreSQLServerVirtualNetworkRule)


Name | Type | Description
-----|------|------------
`name` | string | Name - Name of the Virtual Network Rule.
`serverName` | string | ServerName - Name of the Virtual Network Rule&#39;s PostgreSQLServer.
`serverNameRef` | github.com/crossplane/provider-azure/apis/database/v1beta1.PostgreSQLServerNameReferencer | ServerNameRef - A reference to the Virtual Network Rule&#39;s PostgreSQLServer.
`resourceGroupName` | string | ResourceGroupName - Name of the Virtual Network Rule&#39;s resource group.
`resourceGroupNameRef` | [ResourceGroupNameReferencerForVirtualNetworkRule](#ResourceGroupNameReferencerForVirtualNetworkRule) | ResourceGroupNameRef - A reference to a ResourceGroup object to retrieve its name
`properties` | [VirtualNetworkRuleProperties](#VirtualNetworkRuleProperties) | VirtualNetworkRuleProperties - Resource properties.


PostgreSQLVirtualNetworkRuleSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## ResourceGroupNameReferencerForVirtualNetworkRule

ResourceGroupNameReferencerForVirtualNetworkRule is an attribute referencer that resolves the name of a the ResourceGroup.

Appears in:

* [MySQLVirtualNetworkRuleSpec](#MySQLVirtualNetworkRuleSpec)
* [PostgreSQLVirtualNetworkRuleSpec](#PostgreSQLVirtualNetworkRuleSpec)




ResourceGroupNameReferencerForVirtualNetworkRule supports all fields of:

* github.com/crossplane/provider-azure/apis/v1alpha3.ResourceGroupNameReferencer


## ServerNameReferencerForMySQLServerVirtualNetworkRule

ServerNameReferencerForMySQLServerVirtualNetworkRule is an attribute referencer that resolves the name of a MySQLServer.




ServerNameReferencerForMySQLServerVirtualNetworkRule supports all fields of:

* github.com/crossplane/provider-azure/apis/database/v1beta1.MySQLServerNameReferencer


## ServerNameReferencerForPostgreSQLServerVirtualNetworkRule

ServerNameReferencerForPostgreSQLServerVirtualNetworkRule is an attribute referencer that resolves the name of a PostgreSQLServer.




ServerNameReferencerForPostgreSQLServerVirtualNetworkRule supports all fields of:

* github.com/crossplane/provider-azure/apis/database/v1beta1.PostgreSQLServerNameReferencer


## SubnetIDReferencerForVirtualNetworkRule

SubnetIDReferencerForVirtualNetworkRule is an attribute referencer that resolves id from a referenced Subnet and assigns it to a PostgreSQLServer or MySQL server object

Appears in:

* [VirtualNetworkRuleProperties](#VirtualNetworkRuleProperties)




SubnetIDReferencerForVirtualNetworkRule supports all fields of:

* github.com/crossplane/provider-azure/apis/network/v1alpha3.SubnetIDReferencer


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