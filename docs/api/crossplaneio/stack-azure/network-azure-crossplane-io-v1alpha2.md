# network.azure.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for Azure network services such as virtual networks.

This API group contains the following Crossplane resources:

* [Subnet](#Subnet)
* [VirtualNetwork](#VirtualNetwork)

## Subnet

A Subnet is a managed resource that represents an Azure Subnet.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `network.azure.crossplane.io/v1alpha2`
`kind` | string | `Subnet`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SubnetSpec](#SubnetSpec) | A SubnetSpec defines the desired state of a Subnet.
`status` | [SubnetStatus](#SubnetStatus) | A SubnetStatus represents the observed state of a Subnet.



## VirtualNetwork

A VirtualNetwork is a managed resource that represents an Azure Virtual Network.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `network.azure.crossplane.io/v1alpha2`
`kind` | string | `VirtualNetwork`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [VirtualNetworkSpec](#VirtualNetworkSpec) | A VirtualNetworkSpec defines the desired state of a VirtualNetwork.
`status` | [VirtualNetworkStatus](#VirtualNetworkStatus) | A VirtualNetworkStatus represents the observed state of a VirtualNetwork.



## AddressSpace

AddressSpace contains an array of IP address ranges that can be used by subnets of the virtual network.

Appears in:

* [VirtualNetworkPropertiesFormat](#VirtualNetworkPropertiesFormat)


Name | Type | Description
-----|------|------------
`addressPrefixes` | []string | AddressPrefixes - A list of address blocks reserved for this virtual network in CIDR notation.



## ServiceEndpointPropertiesFormat

ServiceEndpointPropertiesFormat defines properties of a service endpoint.

Appears in:

* [SubnetPropertiesFormat](#SubnetPropertiesFormat)


Name | Type | Description
-----|------|------------
`service` | Optional string | Service - The type of the endpoint service.
`locations` | Optional []string | Locations - A list of locations.
`provisioningState` | Optional string | ProvisioningState - The provisioning state of the resource.



## SubnetPropertiesFormat

SubnetPropertiesFormat defines properties of a Subnet.

Appears in:

* [SubnetSpec](#SubnetSpec)


Name | Type | Description
-----|------|------------
`addressPrefix` | string | AddressPrefix - The address prefix for the subnet.
`serviceEndpoints` | [[]ServiceEndpointPropertiesFormat](#ServiceEndpointPropertiesFormat) | ServiceEndpoints - An array of service endpoints.



## SubnetSpec

A SubnetSpec defines the desired state of a Subnet.

Appears in:

* [Subnet](#Subnet)


Name | Type | Description
-----|------|------------
`name` | string | Name - The name of the resource that is unique within a resource group. This name can be used to access the resource.
`virtualNetworkName` | string | VirtualNetworkName - Name of the Subnet&#39;s virtual network.
`resourceGroupName` | string | ResourceGroupName - Name of the Subnet&#39;s resource group.
`properties` | [SubnetPropertiesFormat](#SubnetPropertiesFormat) | SubnetPropertiesFormat - Properties of the subnet.


SubnetSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## SubnetStatus

A SubnetStatus represents the observed state of a Subnet.

Appears in:

* [Subnet](#Subnet)


Name | Type | Description
-----|------|------------
`state` | string | State of this Subnet.
`message` | string | A Message providing detail about the state of this Subnet, if any.
`etag` | string | Etag - A unique string that changes whenever the resource is updated.
`id` | string | ID of this Subnet.
`purpose` | string | Purpose - A string identifying the intention of use for this subnet based on delegations and other user-defined properties.


SubnetStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## VirtualNetworkPropertiesFormat

VirtualNetworkPropertiesFormat defines properties of a VirtualNetwork.

Appears in:

* [VirtualNetworkSpec](#VirtualNetworkSpec)


Name | Type | Description
-----|------|------------
`addressSpace` | Optional [AddressSpace](#AddressSpace) | AddressSpace - The AddressSpace that contains an array of IP address ranges that can be used by subnets.
`enableDdosProtection` | Optional bool | EnableDDOSProtection - Indicates if DDoS protection is enabled for all the protected resources in the virtual network. It requires a DDoS protection plan associated with the resource.
`enableVmProtection` | Optional bool | EnableVMProtection - Indicates if VM protection is enabled for all the subnets in the virtual network.



## VirtualNetworkSpec

A VirtualNetworkSpec defines the desired state of a VirtualNetwork.

Appears in:

* [VirtualNetwork](#VirtualNetwork)


Name | Type | Description
-----|------|------------
`name` | string | Name - Name of the Virtual Network.
`resourceGroupName` | string | ResourceGroupName - Name of the Virtual Network&#39;s resource group.
`properties` | [VirtualNetworkPropertiesFormat](#VirtualNetworkPropertiesFormat) | VirtualNetworkPropertiesFormat - Properties of the virtual network.
`location` | string | Location - Resource location.
`tags` | Optional map[string]string | Tags - Resource tags.


VirtualNetworkSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## VirtualNetworkStatus

A VirtualNetworkStatus represents the observed state of a VirtualNetwork.

Appears in:

* [VirtualNetwork](#VirtualNetwork)


Name | Type | Description
-----|------|------------
`state` | string | State of this VirtualNetwork.
`message` | string | A Message providing detail about the state of this VirtualNetwork, if any.
`id` | string | ID of this VirtualNetwork.
`etag` | string | Etag - A unique read-only string that changes whenever the resource is updated.
`resourceGuid` | string | ResourceGUID - The GUID of this VirtualNetwork.
`type` | string | Type of this VirtualNetwork.


VirtualNetworkStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


This API documentation was generated by `crossdocs`.