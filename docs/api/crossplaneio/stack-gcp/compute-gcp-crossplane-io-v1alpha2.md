# compute.gcp.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for GCP compute services such as GKE.

This API group contains the following Crossplane resources:

* [GKECluster](#GKECluster)
* [GKEClusterClass](#GKEClusterClass)
* [GlobalAddress](#GlobalAddress)
* [Network](#Network)
* [Subnetwork](#Subnetwork)

## GKECluster

A GKECluster is a managed resource that represents a Google Kubernetes Engine cluster.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1alpha2`
`kind` | string | `GKECluster`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [GKEClusterSpec](#GKEClusterSpec) | A GKEClusterSpec defines the desired state of a GKECluster.
`status` | [GKEClusterStatus](#GKEClusterStatus) | A GKEClusterStatus represents the observed state of a GKECluster.



## GKEClusterClass

A GKEClusterClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1alpha2`
`kind` | string | `GKEClusterClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [GKEClusterClassSpecTemplate](#GKEClusterClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned GKECluster.



## GlobalAddress

A GlobalAddress is a managed resource that represents a Google Compute Engine Global Address.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1alpha2`
`kind` | string | `GlobalAddress`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [GlobalAddressSpec](#GlobalAddressSpec) | A GlobalAddressSpec defines the desired state of a GlobalAddress.
`status` | [GlobalAddressStatus](#GlobalAddressStatus) | A GlobalAddressStatus reflects the observed state of a GlobalAddress.



## Network

A Network is a managed resource that represents a Google Compute Engine VPC Network.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1alpha2`
`kind` | string | `Network`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [NetworkSpec](#NetworkSpec) | A NetworkSpec defines the desired state of a Network.
`status` | [NetworkStatus](#NetworkStatus) | A NetworkStatus represents the observed state of a Network.



## Subnetwork

A Subnetwork is a managed resource that represents a Google Compute Engine VPC Subnetwork.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `compute.gcp.crossplane.io/v1alpha2`
`kind` | string | `Subnetwork`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SubnetworkSpec](#SubnetworkSpec) | A SubnetworkSpec defines the desired state of a Subnetwork.
`status` | [SubnetworkStatus](#SubnetworkStatus) | A SubnetworkStatus represents the observed state of a Subnetwork.



## GCPNetworkPeering

A GCPNetworkPeering represents the observed state of a Google Compute Engine VPC Network Peering.


Name | Type | Description
-----|------|------------
`autoCreateRoutes` | bool | AutoCreateRoutes: This field will be deprecated soon. Use the exchange_subnet_routes field instead. Indicates whether full mesh connectivity is created and managed automatically between peered networks. Currently this field should always be true since Google Compute Engine will automatically create and manage subnetwork routes between two networks when peering state is ACTIVE.
`exchangeSubnetRoutes` | bool | ExchangeSubnetRoutes: Indicates whether full mesh connectivity is created and managed automatically between peered networks. Currently this field should always be true since Google Compute Engine will automatically create and manage subnetwork routes between two networks when peering state is ACTIVE.
`name` | string | Name: Name of this peering. Provided by the client when the peering is created. The name must comply with RFC1035. Specifically, the name must be 1-63 characters long and match regular expression `[a-z]([-a-z0-9]*[a-z0-9])?`. The first character must be a lowercase letter, and all the following characters must be a dash, lowercase letter, or digit, except the last character, which cannot be a dash.
`network` | string | Network: The URL of the peer network. It can be either full URL or partial URL. The peer network may belong to a different project. If the partial URL does not contain project, it is assumed that the peer network is in the same project as the current network.
`state` | string | State: State for the peering, either `ACTIVE` or `INACTIVE`. The peering is `ACTIVE` when there&#39;s a matching configuration in the peer network.  Possible values:   &#34;ACTIVE&#34;   &#34;INACTIVE&#34;
`stateDetails` | string | StateDetails: Details about the current state of the peering.



## GCPNetworkRoutingConfig

A GCPNetworkRoutingConfig specifies the desired state of a Google Compute Engine VPC Network Routing configuration.

Appears in:

* [GCPNetworkStatus](#GCPNetworkStatus)
* [NetworkParameters](#NetworkParameters)


Name | Type | Description
-----|------|------------
`routingMode` | string | RoutingMode: The network-wide routing mode to use. If set to REGIONAL, this network&#39;s Cloud Routers will only advertise routes with subnets of this network in the same region as the router. If set to GLOBAL, this network&#39;s Cloud Routers will advertise routes with all subnets of this network, across regions.  Possible values:   &#34;GLOBAL&#34;   &#34;REGIONAL&#34;



## GCPNetworkStatus

A GCPNetworkStatus represents the observed state of a Google Compute Engine VPC Network.

Appears in:

* [NetworkStatus](#NetworkStatus)


Name | Type | Description
-----|------|------------
`IPv4Range` | string | IPv4Range: Deprecated in favor of subnet mode networks. The range of internal addresses that are legal on this network. This range is a CIDR specification, for example: 192.168.0.0/16. Provided by the client when the network is created.
`autoCreateSubnetworks` | bool | AutoCreateSubnetworks: When set to true, the VPC network is created in &#34;auto&#34; mode. When set to false, the VPC network is created in &#34;custom&#34; mode.  An auto mode VPC network starts with one subnet per region. Each subnet has a predetermined range as described in Auto mode VPC network IP ranges.
`creationTimestamp` | string | CreationTimestamp: Creation timestamp in RFC3339 text format.
`description` | string | Description: An optional description of this resource. Provide this field when you create the resource.
`gatewayIPv4` | string | GatewayIPv4: The gateway address for default routing out of the network, selected by GCP.
`id` | uint64 | Id: The unique identifier for the resource. This identifier is defined by the server.
`peerings` | [[]*github.com/crossplaneio/stack-gcp/gcp/apis/compute/v1alpha2.GCPNetworkPeering](#*github.com/crossplaneio/stack-gcp/gcp/apis/compute/v1alpha2.GCPNetworkPeering) | Peerings: A list of network peerings for the resource.
`routingConfig` | [GCPNetworkRoutingConfig](#GCPNetworkRoutingConfig) | RoutingConfig: The network-level routing configuration for this network. Used by Cloud Router to determine what type of network-wide routing behavior to enforce.
`selfLink` | string | SelfLink: Server-defined URL for the resource.
`subnetworks` | []string | Subnetworks: Server-defined fully-qualified URLs for all subnetworks in this VPC network.



## GCPSubnetworkSecondaryRange

A GCPSubnetworkSecondaryRange defines the state of a Google Compute Engine VPC Subnetwork secondary range.


Name | Type | Description
-----|------|------------
`ipCidrRange` | string | IPCIDRRange: The range of IP addresses belonging to this subnetwork secondary range. Provide this property when you create the subnetwork. Ranges must be unique and non-overlapping with all primary and secondary IP ranges within a network. Only IPv4 is supported.
`rangeName` | string | RangeName: The name associated with this subnetwork secondary range, used when adding an alias IP range to a VM instance. The name must be 1-63 characters long, and comply with RFC1035. The name must be unique within the subnetwork.



## GCPSubnetworkStatus

A GCPSubnetworkStatus represents the observed state of a Google Compute Engine VPC Subnetwork.

Appears in:

* [SubnetworkStatus](#SubnetworkStatus)


Name | Type | Description
-----|------|------------
`creationTimestamp` | string | CreationTimestamp: Creation timestamp in RFC3339 text format.
`description` | string | Description: An optional description of this resource. Provide this property when you create the resource. This field can be set only at resource creation time.
`enableFlowLogs` | bool | EnableFlowLogs: Whether to enable flow logging for this subnetwork. If this field is not explicitly set, it will not appear in get listings. If not set the default behavior is to disable flow logging.
`fingerprint` | string | Fingerprint: Fingerprint of this resource. A hash of the contents stored in this object. This field is used in optimistic locking. This field will be ignored when inserting a Subnetwork. An up-to-date fingerprint must be provided in order to update the Subnetwork, otherwise the request will fail with error 412 conditionNotMet.  To see the latest fingerprint, make a get() request to retrieve a Subnetwork.
`gatewayAddress` | string | GatewayAddress: The gateway address for default routes to reach destination addresses outside this subnetwork.
`id` | uint64 | Id: The unique identifier for the resource. This identifier is defined by the server.
`ipCidrRange` | string | IPCIDRRange: The range of internal addresses that are owned by this subnetwork. Provide this property when you create the subnetwork. For example, 10.0.0.0/8 or 192.168.0.0/16. Ranges must be unique and non-overlapping within a network. Only IPv4 is supported. This field can be set only at resource creation time.
`kind` | string | Kind: Type of the resource. Always compute#subnetwork for Subnetwork resources.
`name` | string | Name: The name of the resource, provided by the client when initially creating the resource. The name must be 1-63 characters long, and comply with RFC1035. Specifically, the name must be 1-63 characters long and match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?` which means the first character must be a lowercase letter, and all following characters must be a dash, lowercase letter, or digit, except the last character, which cannot be a dash.
`network` | string | Network: The URL of the network to which this subnetwork belongs, provided by the client when initially creating the subnetwork. Only networks that are in the distributed mode can have subnetworks. This field can be set only at resource creation time.
`privateIpGoogleAccess` | bool | PrivateIPGoogleAccess: Whether the VMs in this subnet can access Google services without assigned external IP addresses. This field can be both set at resource creation time and updated using setPrivateIPGoogleAccess.
`region` | string | Region: URL of the region where the Subnetwork resides. This field can be set only at resource creation time.
`secondaryIpRanges` | [[]*github.com/crossplaneio/stack-gcp/gcp/apis/compute/v1alpha2.GCPSubnetworkSecondaryRange](#*github.com/crossplaneio/stack-gcp/gcp/apis/compute/v1alpha2.GCPSubnetworkSecondaryRange) | SecondaryIPRanges: An array of configurations for secondary IP ranges for VM instances contained in this subnetwork. The primary IP of such VM must belong to the primary ipCidrRange of the subnetwork. The alias IPs may belong to either primary or secondary ranges. This field can be updated with a patch request.
`selfLink` | string | SelfLink: Server-defined URL for the resource.



## GKEClusterClassSpecTemplate

A GKEClusterClassSpecTemplate is a template for the spec of a dynamically provisioned GKECluster.

Appears in:

* [GKEClusterClass](#GKEClusterClass)




GKEClusterClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [GKEClusterParameters](#GKEClusterParameters)


## GKEClusterParameters

GKEClusterParameters define the desired state of a Google Kubernetes Engine cluster.

Appears in:

* [GKEClusterClassSpecTemplate](#GKEClusterClassSpecTemplate)
* [GKEClusterSpec](#GKEClusterSpec)


Name | Type | Description
-----|------|------------
`clusterVersion` | Optional string | ClusterVersion is the initial Kubernetes version for this cluster. Users may specify either explicit versions offered by Kubernetes Engine or version aliases, for example &#34;latest&#34;, &#34;1.X&#34;, or &#34;1.X.Y&#34;. Leave unset to use the default version.
`labels` | Optional map[string]string | Labels for the cluster to use to annotate any related Google Compute Engine resources.
`machineType` | Optional string | MachineType is the name of a Google Compute Engine machine type (e.g. n1-standard-1). If unspecified the default machine type is n1-standard-1.
`numNodes` | int64 | NumNodes is the number of nodes to create in this cluster. You must ensure that your Compute Engine resource quota is sufficient for this number of instances. You must also have available firewall and routes quota.
`zone` | Optional string | Zone specifies the name of the Google Compute Engine zone in which this cluster resides.
`scopes` | Optional []string | Scopes are the set of Google API scopes to be made available on all of the node VMs under the &#34;default&#34; service account.
`network` | Optional string | Network is the name of the Google Compute Engine network to which the cluster is connected. If left unspecified, the default network will be used.
`subnetwork` | Optional string | Subnetwork is the name of the Google Compute Engine subnetwork to which the cluster is connected.
`enableIPAlias` | Optional bool | EnableIPAlias determines whether Alias IPs will be used for pod IPs in the cluster.
`createSubnetwork` | Optional bool | CreateSubnetwork determines whether a new subnetwork will be created automatically for the cluster. Only applicable when EnableIPAlias is true.
`nodeIPV4CIDR` | Optional string | NodeIPV4CIDR specifies the IP address range of the instance IPs in this cluster. This is applicable only if CreateSubnetwork is true. Omit this field to have a range chosen with the default size. Set it to a netmask (e.g. /24) to have a range chosen with a specific netmask.
`clusterIPV4CIDR` | Optional string | ClusterIPV4CIDR specifies the IP address range of the pod IPs in this cluster. This is applicable only if EnableIPAlias is true. Omit this field to have a range chosen with the default size. Set it to a netmask (e.g. /24) to have a range chosen with a specific netmask.
`clusterSecondaryRangeName` | Optional string | ClusterSecondaryRangeName specifies the name of the secondary range to be used for the cluster CIDR block. The secondary range will be used for pod IP addresses. This must be an existing secondary range associated with the cluster subnetwork.
`serviceIPV4CIDR` | Optional string | ServiceIPV4CIDR specifies the IP address range of service IPs in this cluster. This is applicable only if EnableIPAlias is true. Omit this field to have a range chosen with the default size. Set it to a netmask (e.g. /24) to have a range chosen with a specific netmask.
`servicesSecondaryRangeName` | string | ServicesSecondaryRangeName specifies the name of the secondary range to be used as for the services CIDR block. The secondary range will be used for service ClusterIPs. This must be an existing secondary range associated with the cluster subnetwork.



## GKEClusterSpec

A GKEClusterSpec defines the desired state of a GKECluster.

Appears in:

* [GKECluster](#GKECluster)




GKEClusterSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [GKEClusterParameters](#GKEClusterParameters)


## GKEClusterStatus

A GKEClusterStatus represents the observed state of a GKECluster.

Appears in:

* [GKECluster](#GKECluster)


Name | Type | Description
-----|------|------------
`clusterName` | string | ClusterName is the name of this GKE cluster. The name is automatically generated by Crossplane.
`endpoint` | string | Endpoint of the GKE cluster used in connection strings.
`state` | string | State of this GKE cluster.


GKEClusterStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


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
`name` | string | Name of the resource. The name must be 1-63 characters long, and comply with RFC1035. Specifically, the name must be 1-63 characters long and match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?`. The first character must be a lowercase letter, and all following characters (except for the last character) must be a dash, lowercase letter, or digit. The last character must be a lowercase letter or digit.
`network` | Optional string | Network: The URL of the network in which to reserve the address. This field can only be used with INTERNAL type with the VPC_PEERING purpose.
`prefixLength` | Optional int64 | PrefixLength: The prefix length if the resource represents an IP range.
`purpose` | Optional string | Purpose: The purpose of this resource, which can be one of the following values: - `GCE_ENDPOINT` for addresses that are used by VM instances, alias IP ranges, internal load balancers, and similar resources. - `DNS_RESOLVER` for a DNS resolver address in a subnetwork - `VPC_PEERING` for addresses that are reserved for VPC peer networks. - `NAT_AUTO` for addresses that are external IP addresses automatically reserved for Cloud NAT.  Possible values:   &#34;DNS_RESOLVER&#34;   &#34;GCE_ENDPOINT&#34;   &#34;NAT_AUTO&#34;   &#34;VPC_PEERING&#34;
`subnetwork` | Optional string | Subnetwork: The URL of the subnetwork in which to reserve the address. If an IP address is specified, it must be within the subnetwork&#39;s IP range. This field can only be used with INTERNAL type with a GCE_ENDPOINT or DNS_RESOLVER purpose.



## GlobalAddressSpec

A GlobalAddressSpec defines the desired state of a GlobalAddress.

Appears in:

* [GlobalAddress](#GlobalAddress)




GlobalAddressSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [GlobalAddressParameters](#GlobalAddressParameters)


## GlobalAddressStatus

A GlobalAddressStatus reflects the observed state of a GlobalAddress.

Appears in:

* [GlobalAddress](#GlobalAddress)


Name | Type | Description
-----|------|------------
`creationTimestamp` | string | CreationTimestamp in RFC3339 text format.
`id` | uint64 | ID for the resource. This identifier is defined by the server.
`selfLink` | string | SelfLink: Server-defined URL for the resource.
`status` | string | Status of the address, which can be one of RESERVING, RESERVED, or IN_USE. An address that is RESERVING is currently in the process of being reserved. A RESERVED address is currently reserved and available to use. An IN_USE address is currently being used by another resource and is not available.  Possible values:   &#34;IN_USE&#34;   &#34;RESERVED&#34;   &#34;RESERVING&#34;
`users` | []string | Users that are using this address.


GlobalAddressStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## NetworkParameters

NetworkParameters define the desired state of a Google Compute Engine VPC Network. Most fields map directly to a Network: https://cloud.google.com/compute/docs/reference/rest/v1/networks

Appears in:

* [NetworkSpec](#NetworkSpec)


Name | Type | Description
-----|------|------------
`IPv4Range` | string | IPv4Range: Deprecated in favor of subnet mode networks. The range of internal addresses that are legal on this network. This range is a CIDR specification, for example: 192.168.0.0/16. Provided by the client when the network is created.
`autoCreateSubnetworks` | bool | AutoCreateSubnetworks: When set to true, the VPC network is created in &#34;auto&#34; mode. When set to false, the VPC network is created in &#34;custom&#34; mode. When set to nil, the VPC network is created in &#34;legacy&#34; mode which will be deprecated by GCP soon.  An auto mode VPC network starts with one subnet per region. Each subnet has a predetermined range as described in Auto mode VPC network IP ranges.
`description` | string | Description: An optional description of this resource. Provide this field when you create the resource.
`name` | string | Name: Name of the resource. Provided by the client when the resource is created. The name must be 1-63 characters long, and comply with RFC1035. Specifically, the name must be 1-63 characters long and match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?. The first character must be a lowercase letter, and all following characters (except for the last character) must be a dash, lowercase letter, or digit. The last character must be a lowercase letter or digit.
`routingConfig` | [GCPNetworkRoutingConfig](#GCPNetworkRoutingConfig) | RoutingConfig: The network-level routing configuration for this network. Used by Cloud Router to determine what type of network-wide routing behavior to enforce.



## NetworkSpec

A NetworkSpec defines the desired state of a Network.

Appears in:

* [Network](#Network)




NetworkSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [NetworkParameters](#NetworkParameters)


## NetworkStatus

A NetworkStatus represents the observed state of a Network.

Appears in:

* [Network](#Network)




NetworkStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [GCPNetworkStatus](#GCPNetworkStatus)


## SubnetworkParameters

SubnetworkParameters define the desired state of a Google Compute Engine VPC Subnetwork. Most fields map directly to a Subnetwork: https://cloud.google.com/compute/docs/reference/rest/v1/subnetworks

Appears in:

* [SubnetworkSpec](#SubnetworkSpec)


Name | Type | Description
-----|------|------------
`description` | Optional string | Description: An optional description of this resource. Provide this property when you create the resource. This field can be set only at resource creation time.
`enableFlowLogs` | Optional bool | EnableFlowLogs: Whether to enable flow logging for this subnetwork. If this field is not explicitly set, it will not appear in get listings. If not set the default behavior is to disable flow logging.
`ipCidrRange` | string | IPCIDRRange: The range of internal addresses that are owned by this subnetwork. Provide this property when you create the subnetwork. For example, 10.0.0.0/8 or 192.168.0.0/16. Ranges must be unique and non-overlapping within a network. Only IPv4 is supported. This field can be set only at resource creation time.
`name` | string | Name: The name of the resource, provided by the client when initially creating the resource. The name must be 1-63 characters long, and comply with RFC1035. Specifically, the name must be 1-63 characters long and match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?` which means the first character must be a lowercase letter, and all following characters must be a dash, lowercase letter, or digit, except the last character, which cannot be a dash.
`network` | string | Network: The URL of the network to which this subnetwork belongs, provided by the client when initially creating the subnetwork. Only networks that are in the distributed mode can have subnetworks. This field can be set only at resource creation time.
`privateIpGoogleAccess` | Optional bool | PrivateIPGoogleAccess: Whether the VMs in this subnet can access Google services without assigned external IP addresses. This field can be both set at resource creation time and updated using setPrivateIPGoogleAccess.
`region` | Optional string | Region: URL of the region where the Subnetwork resides. This field can be set only at resource creation time.
`secondaryIpRanges` | Optional [[]*github.com/crossplaneio/stack-gcp/gcp/apis/compute/v1alpha2.GCPSubnetworkSecondaryRange](#*github.com/crossplaneio/stack-gcp/gcp/apis/compute/v1alpha2.GCPSubnetworkSecondaryRange) | SecondaryIPRanges: An array of configurations for secondary IP ranges for VM instances contained in this subnetwork. The primary IP of such VM must belong to the primary ipCidrRange of the subnetwork. The alias IPs may belong to either primary or secondary ranges. This field can be updated with a patch request.



## SubnetworkSpec

A SubnetworkSpec defines the desired state of a Subnetwork.

Appears in:

* [Subnetwork](#Subnetwork)




SubnetworkSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [SubnetworkParameters](#SubnetworkParameters)


## SubnetworkStatus

A SubnetworkStatus represents the observed state of a Subnetwork.

Appears in:

* [Subnetwork](#Subnetwork)




SubnetworkStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [GCPSubnetworkStatus](#GCPSubnetworkStatus)


This API documentation was generated by `crossdocs`.