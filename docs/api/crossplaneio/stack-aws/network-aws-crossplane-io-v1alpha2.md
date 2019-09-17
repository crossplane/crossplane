# network.aws.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for AWS network services such as VPC and Subnet.

This API group contains the following Crossplane resources:

* [InternetGateway](#InternetGateway)
* [RouteTable](#RouteTable)
* [SecurityGroup](#SecurityGroup)
* [Subnet](#Subnet)
* [VPC](#VPC)

## InternetGateway

An InternetGateway is a managed resource that represents an AWS VPC Internet Gateway.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `network.aws.crossplane.io/v1alpha2`
`kind` | string | `InternetGateway`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [InternetGatewaySpec](#InternetGatewaySpec) | An InternetGatewaySpec defines the desired state of an InternetGateway.
`status` | [InternetGatewayStatus](#InternetGatewayStatus) | An InternetGatewayStatus represents the observed state of an InternetGateway.



## RouteTable

A RouteTable is a managed resource that represents an AWS VPC Route Table.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `network.aws.crossplane.io/v1alpha2`
`kind` | string | `RouteTable`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [RouteTableSpec](#RouteTableSpec) | A RouteTableSpec defines the desired state of a RouteTable.
`status` | [RouteTableStatus](#RouteTableStatus) | A RouteTableStatus represents the observed state of a RouteTable.



## SecurityGroup

A SecurityGroup is a managed resource that represents an AWS VPC Security Group.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `network.aws.crossplane.io/v1alpha2`
`kind` | string | `SecurityGroup`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SecurityGroupSpec](#SecurityGroupSpec) | A SecurityGroupSpec defines the desired state of a SecurityGroup.
`status` | [SecurityGroupStatus](#SecurityGroupStatus) | A SecurityGroupStatus represents the observed state of a SecurityGroup.



## Subnet

A Subnet is a managed resource that represents an AWS VPC Subnet.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `network.aws.crossplane.io/v1alpha2`
`kind` | string | `Subnet`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [SubnetSpec](#SubnetSpec) | A SubnetSpec defines the desired state of a Subnet.
`status` | [SubnetStatus](#SubnetStatus) | A SubnetStatus represents the observed state of a Subnet.



## VPC

A VPC is a managed resource that represents an AWS Virtual Private Cloud.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `network.aws.crossplane.io/v1alpha2`
`kind` | string | `VPC`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [VPCSpec](#VPCSpec) | A VPCSpec defines the desired state of a VPC.
`status` | [VPCStatus](#VPCStatus) | A VPCStatus represents the observed state of a VPC.



## Association

Association describes an association between a route table and a subnet.

Appears in:

* [AssociationState](#AssociationState)
* [RouteTableParameters](#RouteTableParameters)


Name | Type | Description
-----|------|------------
`subnetId` | string | The ID of the subnet. A subnet ID is not returned for an implicit association.



## AssociationState

AssociationState describes an association state in the route table.

Appears in:

* [RouteTableExternalStatus](#RouteTableExternalStatus)


Name | Type | Description
-----|------|------------
`main` | bool | Indicates whether this is the main route table.
`associationId` | string | The ID of the association between a route table and a subnet.


AssociationState supports all fields of:

* [Association](#Association)


## IPPermission

IPPermission Describes a set of permissions for a security group rule.

Appears in:

* [SecurityGroupParameters](#SecurityGroupParameters)


Name | Type | Description
-----|------|------------
`fromPort` | int64 | The start of port range for the TCP and UDP protocols, or an ICMP/ICMPv6 type number. A value of -1 indicates all ICMP/ICMPv6 types. If you specify all ICMP/ICMPv6 types, you must specify all codes.
`toPort` | int64 | The end of port range for the TCP and UDP protocols, or an ICMP/ICMPv6 code. A value of -1 indicates all ICMP/ICMPv6 codes for the specified ICMP type. If you specify all ICMP/ICMPv6 types, you must specify all codes.
`protocol` | string | The IP protocol name (tcp, udp, icmp) or number (see Protocol Numbers (http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml)).  [EC2-VPC only] Use -1 to specify all protocols. When authorizing security group rules, specifying -1 or a protocol number other than tcp, udp, icmp, or 58 (ICMPv6) allows traffic on all ports, regardless of any port range you specify. For tcp, udp, and icmp, you must specify a port range. For 58 (ICMPv6), you can optionally specify a port range; if you don&#39;t, traffic for all types and codes is allowed when authorizing rules.
`cidrBlocks` | [[]IPRange](#IPRange) | One or more IPv4 ranges.



## IPRange

IPRange describes an IPv4 range.

Appears in:

* [IPPermission](#IPPermission)


Name | Type | Description
-----|------|------------
`cidrIp` | string | The IPv4 CIDR range. You can either specify a CIDR range or a source security group, not both. To specify a single IPv4 address, use the /32 prefix length.
`description` | string | A description for the ip range



## InternetGatewayAttachment

InternetGatewayAttachment describes the attachment of a VPC to an internet gateway or an egress-only internet gateway.

Appears in:

* [InternetGatewayExternalStatus](#InternetGatewayExternalStatus)


Name | Type | Description
-----|------|------------
`attachmentStatus` | string | The current state of the attachment. For an internet gateway, the state is available when attached to a VPC; otherwise, this value is not returned.
`vpcId` | string | VPCID is the ID of the attached VPC.



## InternetGatewayExternalStatus

InternetGatewayExternalStatus keeps the state for the external resource

Appears in:

* [InternetGatewayStatus](#InternetGatewayStatus)


Name | Type | Description
-----|------|------------
`attachments` | [[]InternetGatewayAttachment](#InternetGatewayAttachment) | Any VPCs attached to the internet gateway.
`internetGatewayId` | string | The ID of the internet gateway.
`tags` | [[]Tag](#Tag) | Tags represents to current ec2 tags.



## InternetGatewayParameters

InternetGatewayParameters define the desired state of an AWS VPC Internet Gateway.

Appears in:

* [InternetGatewaySpec](#InternetGatewaySpec)


Name | Type | Description
-----|------|------------
`vpcId` | string | the VPC to attach the gateway to.



## InternetGatewaySpec

An InternetGatewaySpec defines the desired state of an InternetGateway.

Appears in:

* [InternetGateway](#InternetGateway)




InternetGatewaySpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [InternetGatewayParameters](#InternetGatewayParameters)


## InternetGatewayStatus

An InternetGatewayStatus represents the observed state of an InternetGateway.

Appears in:

* [InternetGateway](#InternetGateway)




InternetGatewayStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [InternetGatewayExternalStatus](#InternetGatewayExternalStatus)


## Route

Route describes a route in a route table.

Appears in:

* [RouteState](#RouteState)
* [RouteTableParameters](#RouteTableParameters)


Name | Type | Description
-----|------|------------
`destinationCidrBlock` | string | The IPv4 CIDR address block used for the destination match. Routing decisions are based on the most specific match.
`gatewayId` | string | The ID of an internet gateway or virtual private gateway attached to your VPC.



## RouteState

RouteState describes a route state in the route table.

Appears in:

* [RouteTableExternalStatus](#RouteTableExternalStatus)


Name | Type | Description
-----|------|------------
`routeState` | string | The state of the route. The blackhole state indicates that the route&#39;s target isn&#39;t available (for example, the specified gateway isn&#39;t attached to the VPC, or the specified NAT instance has been terminated).


RouteState supports all fields of:

* [Route](#Route)


## RouteTableExternalStatus

RouteTableExternalStatus keeps the state for the external resource

Appears in:

* [RouteTableStatus](#RouteTableStatus)


Name | Type | Description
-----|------|------------
`routeTableId` | string | RouteTableID is the ID of the RouteTable.
`routes` | [[]RouteState](#RouteState) | The actual routes created for the route table.
`associations` | [[]AssociationState](#AssociationState) | The actual associations created for the route table.



## RouteTableParameters

RouteTableParameters define the desired state of an AWS VPC Route Table.

Appears in:

* [RouteTableSpec](#RouteTableSpec)


Name | Type | Description
-----|------|------------
`vpcId` | string | VPCID is the ID of the VPC.
`routes` | [[]Route](#Route) | the routes in the route table
`associations` | [[]Association](#Association) | The associations between the route table and one or more subnets.



## RouteTableSpec

A RouteTableSpec defines the desired state of a RouteTable.

Appears in:

* [RouteTable](#RouteTable)




RouteTableSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [RouteTableParameters](#RouteTableParameters)


## RouteTableStatus

A RouteTableStatus represents the observed state of a RouteTable.

Appears in:

* [RouteTable](#RouteTable)




RouteTableStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [RouteTableExternalStatus](#RouteTableExternalStatus)


## SecurityGroupExternalStatus

SecurityGroupExternalStatus keeps the state for the external resource

Appears in:

* [SecurityGroupStatus](#SecurityGroupStatus)


Name | Type | Description
-----|------|------------
`securityGroupID` | string | SecurityGroupID is the ID of the SecurityGroup.
`tags` | [[]Tag](#Tag) | Tags represents to current ec2 tags.



## SecurityGroupParameters

SecurityGroupParameters define the desired state of an AWS VPC Security Group.

Appears in:

* [SecurityGroupSpec](#SecurityGroupSpec)


Name | Type | Description
-----|------|------------
`vpcId` | string | VPCID is the ID of the VPC.
`description` | string | A description of the security group.
`groupName` | string | The name of the security group.
`ingress` | [[]IPPermission](#IPPermission) | One or more inbound rules associated with the security group.
`egress` | [[]IPPermission](#IPPermission) | [EC2-VPC] One or more outbound rules associated with the security group.



## SecurityGroupSpec

A SecurityGroupSpec defines the desired state of a SecurityGroup.

Appears in:

* [SecurityGroup](#SecurityGroup)




SecurityGroupSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [SecurityGroupParameters](#SecurityGroupParameters)


## SecurityGroupStatus

A SecurityGroupStatus represents the observed state of a SecurityGroup.

Appears in:

* [SecurityGroup](#SecurityGroup)




SecurityGroupStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [SecurityGroupExternalStatus](#SecurityGroupExternalStatus)


## SubnetExternalStatus

SubnetExternalStatus keeps the state for the external resource

Appears in:

* [SubnetStatus](#SubnetStatus)


Name | Type | Description
-----|------|------------
`subnetState` | string | SubnetState is the current state of the Subnet.
`tags` | [[]Tag](#Tag) | Tags represents to current ec2 tags.
`subnetId` | string | SubnetID is the ID of the Subnet.



## SubnetParameters

SubnetParameters define the desired state of an AWS VPC Subnet.

Appears in:

* [SubnetSpec](#SubnetSpec)


Name | Type | Description
-----|------|------------
`cidrBlock` | string | CIDRBlock is the IPv4 network range for the Subnet, in CIDR notation. For example, 10.0.0.0/18.
`availabilityZone` | string | The Availability Zone for the subnet. Default: AWS selects one for you. If you create more than one subnet in your VPC, we may not necessarily select a different zone for each subnet.
`vpcId` | string | VPCID is the ID of the VPC.



## SubnetSpec

A SubnetSpec defines the desired state of a Subnet.

Appears in:

* [Subnet](#Subnet)




SubnetSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [SubnetParameters](#SubnetParameters)


## SubnetStatus

A SubnetStatus represents the observed state of a Subnet.

Appears in:

* [Subnet](#Subnet)




SubnetStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [SubnetExternalStatus](#SubnetExternalStatus)


## Tag

Tag defines a tag

Appears in:

* [InternetGatewayExternalStatus](#InternetGatewayExternalStatus)
* [SecurityGroupExternalStatus](#SecurityGroupExternalStatus)
* [SubnetExternalStatus](#SubnetExternalStatus)
* [VPCExternalStatus](#VPCExternalStatus)


Name | Type | Description
-----|------|------------
`key` | string | Key is the name of the tag.
`value` | string | Value is the value of the tag.



## VPCExternalStatus

VPCExternalStatus keeps the state for the external resource

Appears in:

* [VPCStatus](#VPCStatus)


Name | Type | Description
-----|------|------------
`vpcState` | string | VPCState is the current state of the VPC.
`tags` | [[]Tag](#Tag) | Tags represents to current ec2 tags.
`vpcId` | string | VPCID is the ID of the VPC.



## VPCParameters

VPCParameters define the desired state of an AWS Virtual Private Cloud.

Appears in:

* [VPCSpec](#VPCSpec)


Name | Type | Description
-----|------|------------
`cidrBlock` | string | CIDRBlock is the IPv4 network range for the VPC, in CIDR notation. For example, 10.0.0.0/16.
`enableDnsSupport` | bool | A boolean flag to enable/disable DNS support in the VPC
`enableDnsHostNames` | bool | A boolean flag to enable/disable DNS hostnames in the VPC



## VPCSpec

A VPCSpec defines the desired state of a VPC.

Appears in:

* [VPC](#VPC)




VPCSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [VPCParameters](#VPCParameters)


## VPCStatus

A VPCStatus represents the observed state of a VPC.

Appears in:

* [VPC](#VPC)




VPCStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [VPCExternalStatus](#VPCExternalStatus)


This API documentation was generated by `crossdocs`.