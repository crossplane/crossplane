# compute.gcp.crossplane.io/v1beta1 API Reference

Package v1beta1 contains managed resources for GCP compute services such as GKE.

This API group contains the following Crossplane resources:

* [GlobalAddress](#GlobalAddress)
* [Network](#Network)
* [Subnetwork](#Subnetwork)

## GlobalAddress

A GlobalAddress is a managed resource that represents a Google Compute Engine Global Address.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1beta1`
`kind` | string | `GlobalAddress`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [GlobalAddressSpec](#GlobalAddressSpec) | A GlobalAddressSpec defines the desired state of a GlobalAddress.
`status` | [GlobalAddressStatus](#GlobalAddressStatus) | A GlobalAddressStatus represents the observed state of a GlobalAddress.



## Network

A Network is a managed resource that represents a Google Compute Engine VPC Network.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1beta1`
`kind` | string | `Network`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [NetworkSpec](#NetworkSpec) | A NetworkSpec defines the desired state of a Network.
`status` | [NetworkStatus](#NetworkStatus) | A NetworkStatus represents the observed state of a Network.



## Subnetwork

A Subnetwork is a managed resource that represents a Google Compute Engine VPC Subnetwork.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1beta1`
`kind` | string | `Subnetwork`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SubnetworkSpec](#SubnetworkSpec) | A SubnetworkSpec defines the desired state of a Subnetwork.
`status` | [SubnetworkStatus](#SubnetworkStatus) | A SubnetworkStatus represents the observed state of a Subnetwork.



## GlobalAddressNameReferencer

GlobalAddressNameReferencer retrieves a Name from a referenced GlobalAddress object




GlobalAddressNameReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## GlobalAddressObservation

A GlobalAddressObservation reflects the observed state of a GlobalAddress on GCP.

Appears in:

* [GlobalAddressStatus](#GlobalAddressStatus)


Name | Type | Description
-----|------|------------
`creationTimestamp` | string | CreationTimestamp in RFC3339 text format.
`id` | uint64 | ID for the resource. This identifier is defined by the server.
`selfLink` | string | SelfLink: Server-defined URL for the resource.
`status` | string | Status of the address, which can be one of RESERVING, RESERVED, or IN_USE. An address that is RESERVING is currently in the process of being reserved. A RESERVED address is currently reserved and available to use. An IN_USE address is currently being used by another resource and is not available.  Possible values:   &#34;IN_USE&#34;   &#34;RESERVED&#34;   &#34;RESERVING&#34;
`users` | []string | Users that are using this address.



## GlobalAddressParameters

GlobalAddressParameters define the desired state of a Google Compute Engine Global Address. Most fields map directly to an Address: https://cloud.google.com/compute/docs/reference/rest/v1/globalAddresses

Appears in:

* [GlobalAddressSpec](#GlobalAddressSpec)


Name | Type | Description
-----|------|------------
`address` | Optional string | Address: The static IP address represented by this resource.
`addressType` | Optional string | AddressType: The type of address to reserve, either INTERNAL or EXTERNAL. If unspecified, defaults to EXTERNAL.  Possible values:   &#34;EXTERNAL&#34;   &#34;INTERNAL&#34;   &#34;UNSPECIFIED_TYPE&#34;
`description` | Optional string | Description: An optional description of this resource.
`ipVersion` | Optional string | IPVersion: The IP version that will be used by this address. Valid options are IPV4 or IPV6.  Possible values:   &#34;IPV4&#34;   &#34;IPV6&#34;   &#34;UNSPECIFIED_VERSION&#34;
`network` | Optional string | Network: The URL of the network in which to reserve the address. This field can only be used with INTERNAL type with the VPC_PEERING purpose.
`networkRef` | Optional [NetworkURIReferencerForGlobalAddress](#NetworkURIReferencerForGlobalAddress) | NetworkRef references to a Network and retrieves its URI
`prefixLength` | Optional int64 | PrefixLength: The prefix length if the resource represents an IP range.
`purpose` | Optional string | Purpose: The purpose of this resource, which can be one of the following values: - `GCE_ENDPOINT` for addresses that are used by VM instances, alias IP ranges, internal load balancers, and similar resources. - `DNS_RESOLVER` for a DNS resolver address in a subnetwork - `VPC_PEERING` for addresses that are reserved for VPC peer networks. - `NAT_AUTO` for addresses that are external IP addresses automatically reserved for Cloud NAT.  Possible values:   &#34;DNS_RESOLVER&#34;   &#34;GCE_ENDPOINT&#34;   &#34;NAT_AUTO&#34;   &#34;VPC_PEERING&#34;
`subnetwork` | Optional string | Subnetwork: The URL of the subnetwork in which to reserve the address. If an IP address is specified, it must be within the subnetwork&#39;s IP range. This field can only be used with INTERNAL type with a GCE_ENDPOINT or DNS_RESOLVER purpose.
`subnetworkRef` | Optional [SubnetworkURIReferencerForGlobalAddress](#SubnetworkURIReferencerForGlobalAddress) | SubnetworkRef references to a Subnetwork and retrieves its URI



## GlobalAddressSpec

A GlobalAddressSpec defines the desired state of a GlobalAddress.

Appears in:

* [GlobalAddress](#GlobalAddress)


Name | Type | Description
-----|------|------------
`forProvider` | [GlobalAddressParameters](#GlobalAddressParameters) | GlobalAddressParameters define the desired state of a Google Compute Engine Global Address. Most fields map directly to an Address: https://cloud.google.com/compute/docs/reference/rest/v1/globalAddresses


GlobalAddressSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## GlobalAddressStatus

A GlobalAddressStatus represents the observed state of a GlobalAddress.

Appears in:

* [GlobalAddress](#GlobalAddress)


Name | Type | Description
-----|------|------------
`atProvider` | [GlobalAddressObservation](#GlobalAddressObservation) | A GlobalAddressObservation reflects the observed state of a GlobalAddress on GCP.


GlobalAddressStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## NetworkObservation

A NetworkObservation represents the observed state of a Google Compute Engine VPC Network.

Appears in:

* [NetworkStatus](#NetworkStatus)


Name | Type | Description
-----|------|------------
`creationTimestamp` | string | CreationTimestamp: Creation timestamp in RFC3339 text format.
`gatewayIPv4` | string | GatewayIPv4: The gateway address for default routing out of the network, selected by GCP.
`id` | uint64 | Id: The unique identifier for the resource. This identifier is defined by the server.
`peerings` | [[]*github.com/crossplaneio/stack-gcp/apis/compute/v1beta1.NetworkPeering](#*github.com/crossplaneio/stack-gcp/apis/compute/v1beta1.NetworkPeering) | Peerings: A list of network peerings for the resource.
`selfLink` | string | SelfLink: Server-defined URL for the resource.
`subnetworks` | []string | Subnetworks: Server-defined fully-qualified URLs for all subnetworks in this VPC network.



## NetworkParameters

NetworkParameters define the desired state of a Google Compute Engine VPC Network. Most fields map directly to a Network: https://cloud.google.com/compute/docs/reference/rest/v1/networks

Appears in:

* [NetworkSpec](#NetworkSpec)


Name | Type | Description
-----|------|------------
`autoCreateSubnetworks` | Optional bool | AutoCreateSubnetworks: When set to true, the VPC network is created in &#34;auto&#34; mode. When set to false, the VPC network is created in &#34;custom&#34; mode. When set to nil, the VPC network is created in &#34;legacy&#34; mode which will be deprecated by GCP soon.  An auto mode VPC network starts with one subnet per region. Each subnet has a predetermined range as described in Auto mode VPC network IP ranges.  This field can only be updated from true to false after creation using switchToCustomMode.
`description` | Optional string | Description: An optional description of this resource. Provide this field when you create the resource.
`routingConfig` | Optional [NetworkRoutingConfig](#NetworkRoutingConfig) | RoutingConfig: The network-level routing configuration for this network. Used by Cloud Router to determine what type of network-wide routing behavior to enforce.



## NetworkPeering

A NetworkPeering represents the observed state of a Google Compute Engine VPC Network Peering.


Name | Type | Description
-----|------|------------
`autoCreateRoutes` | bool | AutoCreateRoutes: This field will be deprecated soon. Use the exchange_subnet_routes field instead. Indicates whether full mesh connectivity is created and managed automatically between peered networks. Currently this field should always be true since Google Compute Engine will automatically create and manage subnetwork routes between two networks when peering state is ACTIVE.
`exchangeSubnetRoutes` | bool | ExchangeSubnetRoutes: Indicates whether full mesh connectivity is created and managed automatically between peered networks. Currently this field should always be true since Google Compute Engine will automatically create and manage subnetwork routes between two networks when peering state is ACTIVE.
`name` | string | Name: Name of this peering. Provided by the client when the peering is created. The name must comply with RFC1035. Specifically, the name must be 1-63 characters long and match regular expression `[a-z]([-a-z0-9]*[a-z0-9])?`. The first character must be a lowercase letter, and all the following characters must be a dash, lowercase letter, or digit, except the last character, which cannot be a dash.
`network` | string | Network: The URL of the peer network. It can be either full URL or partial URL. The peer network may belong to a different project. If the partial URL does not contain project, it is assumed that the peer network is in the same project as the current network.
`state` | string | State: State for the peering, either `ACTIVE` or `INACTIVE`. The peering is `ACTIVE` when there&#39;s a matching configuration in the peer network.  Possible values:   &#34;ACTIVE&#34;   &#34;INACTIVE&#34;
`stateDetails` | string | StateDetails: Details about the current state of the peering.



## NetworkRoutingConfig

A NetworkRoutingConfig specifies the desired state of a Google Compute Engine VPC Network Routing configuration.

Appears in:

* [NetworkParameters](#NetworkParameters)


Name | Type | Description
-----|------|------------
`routingMode` | string | RoutingMode: The network-wide routing mode to use. If set to REGIONAL, this network&#39;s Cloud Routers will only advertise routes with subnets of this network in the same region as the router. If set to GLOBAL, this network&#39;s Cloud Routers will advertise routes with all subnets of this network, across regions.  Possible values:   &#34;GLOBAL&#34;   &#34;REGIONAL&#34;



## NetworkSpec

A NetworkSpec defines the desired state of a Network.

Appears in:

* [Network](#Network)


Name | Type | Description
-----|------|------------
`forProvider` | [NetworkParameters](#NetworkParameters) | NetworkParameters define the desired state of a Google Compute Engine VPC Network. Most fields map directly to a Network: https://cloud.google.com/compute/docs/reference/rest/v1/networks


NetworkSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## NetworkStatus

A NetworkStatus represents the observed state of a Network.

Appears in:

* [Network](#Network)


Name | Type | Description
-----|------|------------
`atProvider` | [NetworkObservation](#NetworkObservation) | A NetworkObservation represents the observed state of a Google Compute Engine VPC Network.


NetworkStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## NetworkURIReferencer

NetworkURIReferencer retrieves a NetworkURI from a referenced Network object

Appears in:

* [NetworkURIReferencerForGlobalAddress](#NetworkURIReferencerForGlobalAddress)
* [NetworkURIReferencerForSubnetwork](#NetworkURIReferencerForSubnetwork)




NetworkURIReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## NetworkURIReferencerForGlobalAddress

NetworkURIReferencerForGlobalAddress is an attribute referencer that resolves network uri from a referenced Network and assigns it to a global address object

Appears in:

* [GlobalAddressParameters](#GlobalAddressParameters)




NetworkURIReferencerForGlobalAddress supports all fields of:

* [NetworkURIReferencer](#NetworkURIReferencer)


## NetworkURIReferencerForSubnetwork

NetworkURIReferencerForSubnetwork is an attribute referencer that resolves network uri from a referenced Network and assigns it to a subnetwork

Appears in:

* [SubnetworkParameters](#SubnetworkParameters)




NetworkURIReferencerForSubnetwork supports all fields of:

* [NetworkURIReferencer](#NetworkURIReferencer)


## SubnetworkObservation

A SubnetworkObservation represents the observed state of a Google Compute Engine VPC Subnetwork.

Appears in:

* [SubnetworkStatus](#SubnetworkStatus)


Name | Type | Description
-----|------|------------
`creationTimestamp` | string | CreationTimestamp: Creation timestamp in RFC3339 text format.
`fingerprint` | string | Fingerprint: Fingerprint of this resource. A hash of the contents stored in this object. This field is used in optimistic locking. This field will be ignored when inserting a Subnetwork. An up-to-date fingerprint must be provided in order to update the Subnetwork, otherwise the request will fail with error 412 conditionNotMet.  To see the latest fingerprint, make a get() request to retrieve a Subnetwork.
`gatewayAddress` | string | GatewayAddress: The gateway address for default routes to reach destination addresses outside this subnetwork.
`id` | uint64 | Id: The unique identifier for the resource. This identifier is defined by the server.
`selfLink` | string | SelfLink: Server-defined URL for the resource.



## SubnetworkParameters

SubnetworkParameters define the desired state of a Google Compute Engine VPC Subnetwork. Most fields map directly to a Subnetwork: https://cloud.google.com/compute/docs/reference/rest/v1/subnetworks

Appears in:

* [SubnetworkSpec](#SubnetworkSpec)


Name | Type | Description
-----|------|------------
`ipCidrRange` | string | IPCIDRRange: The range of internal addresses that are owned by this subnetwork. Provide this property when you create the subnetwork. For example, 10.0.0.0/8 or 192.168.0.0/16. Ranges must be unique and non-overlapping within a network. Only IPv4 is supported. This field can be set only at resource creation time.
`network` | string | Network: The URL of the network to which this subnetwork belongs, provided by the client when initially creating the subnetwork. Only networks that are in the distributed mode can have subnetworks. This field can be set only at resource creation time.
`networkRef` | [NetworkURIReferencerForSubnetwork](#NetworkURIReferencerForSubnetwork) | NetworkRef references to a Network and retrieves its URI
`region` | Optional string | Region: URL of the region where the Subnetwork resides. This field can be set only at resource creation time.
`description` | Optional string | Description: An optional description of this resource. Provide this property when you create the resource. This field can be set only at resource creation time.
`enableFlowLogs` | Optional bool | EnableFlowLogs: Whether to enable flow logging for this subnetwork. If this field is not explicitly set, it will not appear in get listings. If not set the default behavior is to disable flow logging.
`privateIpGoogleAccess` | Optional bool | PrivateIPGoogleAccess: Whether the VMs in this subnet can access Google services without assigned external IP addresses. This field can be both set at resource creation time and updated using setPrivateIPGoogleAccess.
`secondaryIpRanges` | Optional [[]*github.com/crossplaneio/stack-gcp/apis/compute/v1beta1.SubnetworkSecondaryRange](#*github.com/crossplaneio/stack-gcp/apis/compute/v1beta1.SubnetworkSecondaryRange) | SecondaryIPRanges: An array of configurations for secondary IP ranges for VM instances contained in this subnetwork. The primary IP of such VM must belong to the primary ipCidrRange of the subnetwork. The alias IPs may belong to either primary or secondary ranges. This field can be updated with a patch request.



## SubnetworkSecondaryRange

A SubnetworkSecondaryRange defines the state of a Google Compute Engine VPC Subnetwork secondary range.


Name | Type | Description
-----|------|------------
`ipCidrRange` | string | IPCIDRRange: The range of IP addresses belonging to this subnetwork secondary range. Provide this property when you create the subnetwork. Ranges must be unique and non-overlapping with all primary and secondary IP ranges within a network. Only IPv4 is supported.
`rangeName` | string | RangeName: The name associated with this subnetwork secondary range, used when adding an alias IP range to a VM instance. The name must be 1-63 characters long, and comply with RFC1035. The name must be unique within the subnetwork.



## SubnetworkSpec

A SubnetworkSpec defines the desired state of a Subnetwork.

Appears in:

* [Subnetwork](#Subnetwork)


Name | Type | Description
-----|------|------------
`forProvider` | [SubnetworkParameters](#SubnetworkParameters) | SubnetworkParameters define the desired state of a Google Compute Engine VPC Subnetwork. Most fields map directly to a Subnetwork: https://cloud.google.com/compute/docs/reference/rest/v1/subnetworks


SubnetworkSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## SubnetworkStatus

A SubnetworkStatus represents the observed state of a Subnetwork.

Appears in:

* [Subnetwork](#Subnetwork)


Name | Type | Description
-----|------|------------
`atProvider` | [SubnetworkObservation](#SubnetworkObservation) | A SubnetworkObservation represents the observed state of a Google Compute Engine VPC Subnetwork.


SubnetworkStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## SubnetworkURIReferencer

SubnetworkURIReferencer retrieves a SubnetworkURI from a referenced Subnetwork object

Appears in:

* [SubnetworkURIReferencerForGlobalAddress](#SubnetworkURIReferencerForGlobalAddress)




SubnetworkURIReferencer supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## SubnetworkURIReferencerForGlobalAddress

SubnetworkURIReferencerForGlobalAddress is an attribute referencer that resolves subnetwork uri from a referenced Subnetwork and assigns it to a global address object

Appears in:

* [GlobalAddressParameters](#GlobalAddressParameters)




SubnetworkURIReferencerForGlobalAddress supports all fields of:

* [SubnetworkURIReferencer](#SubnetworkURIReferencer)


This API documentation was generated by `crossdocs`.