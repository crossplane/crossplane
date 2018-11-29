/*
Copyright 2018 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"strconv"
	"strings"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (

	// The resource is being created. The resource is inaccessible while it is being created.
	ClusterStatusCreating = "CREATING"
	// The resource is created and in active state
	ClusterStatusActive = "ACTIVE"

	// TODO: Deleting and Failed currently not used. Implement usage or remove
	// The resource is being deleted
	// ClusterStatusDeleting = "DELETING"
	// The resource is in failed state
	// ClusterStatusFailed = "FAILED"
)

type EKSRegion string

const (
	// EKSRegionUSWest2 - us-west-2 (Oregon) region for eks cluster
	EKSRegionUSWest2 EKSRegion = "us-west-2"
	// EKSRegionUSEast1 - us-east-1 (N. Virginia) region for eks cluster
	EKSRegionUSEast1 EKSRegion = "us-east-1"
	// EKSRegionUSEast2 - us-east-2 (Ohio) region for eks worker only
	EKSRegionUSEast2 EKSRegion = "us-east-2"
	// EKSRegionEUWest1 - eu-west-1 (Ireland) region for eks cluster
	EKSRegionEUWest1 EKSRegion = "eu-west-1"
)

var (
	workerNodeRegionAMI = map[EKSRegion]string{
		EKSRegionUSWest2: "ami-0f54a2f7d2e9c88b3",
		EKSRegionUSEast1: "ami-0a0b913ef3249b655",
		EKSRegionUSEast2: "ami-0958a76db2d150238",
		EKSRegionEUWest1: "ami-00c3b2d35bddd4f5c",
	}
)

type SecurityGroupSpec struct {
	// The name of the security group.
	Name string `json:"name"`

	// Description A description of the security group.
	Description string `json:"groupDescription"`

	// IpPermissions One or more inbound rules associated with the security group.
	IpPermissions []IpPermission `json:"ipPermissions"`

	// IpPermissionsEgress [EC2-VPC] One or more outbound rules associated with the security group.
	IpPermissionsEgress []IpPermission `json:"ipPermissionsEgress"`

	// Tags Any tags assigned to the security group.
	Tags []Tag `json:"tags"`

	// VpcID  [EC2-VPC] The ID of the VPC for the security group.
	VpcID string `json:"vpcId"`

	// Kubernetes object references
	ClaimRef    *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef    *corev1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef corev1.LocalObjectReference `json:"providerRef"`
	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

type SecurityGroupStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase

	// SecurityGroupID The ID of the security group.
	SecurityGroupID string `json:"groupId"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecurityGroup is the Schema for the resources API
// +k8s:openapi-gen=true
// +groupName=compute.aws
type SecurityGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupSpec   `json:"spec,omitempty"`
	Status SecurityGroupStatus `json:"status,omitempty"`
}

// Describes a tag.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/Tag
type Tag struct {
	// Key of the tag.
	//
	// Constraints: Tag keys are case-sensitive and accept a maximum of 127 Unicode
	// characters. May not begin with aws:.
	Key *string `json:"key"`

	// Value of the tag.
	//
	// Constraints: Tag values are case-sensitive and accept a maximum of 255 Unicode
	// characters.
	Value *string `json:"value"`
}

type IpPermission struct {
	// FromPort The start of port range for the TCP and UDP protocols, or an ICMP/ICMPv6
	// type number. A value of -1 indicates all ICMP/ICMPv6 types. If you specify
	// all ICMP/ICMPv6 types, you must specify all codes.
	FromPort *int64 `json:"fromPort"`

	// IpProtocol name (tcp, udp, icmp) or number (see Protocol Numbers (http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml)).
	//
	// Use -1 to specify all protocols. When authorizing security
	// group rules, specifying -1 or a protocol number other than tcp, udp, icmp,
	// or 58 (ICMPv6) allows traffic on all ports, regardless of any port range
	// you specify. For tcp, udp, and icmp, you must specify a port range. For 58
	// (ICMPv6), you can optionally specify a port range; if you don't, traffic
	// for all types and codes is allowed when authorizing rules.
	IpProtocol *string `json:"ipProtocol"`

	// One or more IPv4 ranges.
	IpRanges []IpRange `json:"ipRanges"`

	// Ipv6Ranges One or more IPv6 ranges.
	Ipv6Ranges []Ipv6Range `json:"ipv6Ranges"`

	// ToPort The end of port range for the TCP and UDP protocols, or an ICMP/ICMPv6 code.
	// A value of -1 indicates all ICMP/ICMPv6 codes for the specified ICMP type.
	// If you specify all ICMP/ICMPv6 types, you must specify all codes.
	ToPort *int64 `json:"toPort" type:"integer"`
}

type IpRange struct {
	// CidrIp IPv4 CIDR range. You can either specify a CIDR range or a source security
	// group, not both. To specify a single IPv4 address, use the /32 prefix length.
	CidrIp *string `json:"cidrIp"`

	// Description for the security group rule that references this IPv4 address
	// range.
	//
	// Constraints: Up to 255 characters in length. Allowed characters are a-z,
	// A-Z, 0-9, spaces, and ._-:/()#,@[]+=;{}!$*
	Description *string `json:"description"`
}

// [EC2-VPC only] Describes an IPv6 range.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/Ipv6Range
type Ipv6Range struct {

	// CidrIpv6 IPv6 CIDR range. You can either specify a CIDR range or a source security
	// group, not both. To specify a single IPv6 address, use the /128 prefix length.
	CidrIpv6 *string `json:"cidrIpv6"`

	// Description for the security group rule that references this IPv6 address
	// range.
	//
	// Constraints: Up to 255 characters in length. Allowed characters are a-z,
	// A-Z, 0-9, spaces, and ._-:/()#,@[]+=;{}!$*
	Description *string `locationName:"description" type:"string"`
}

type EKSClusterSpec struct {
	// Configuration of this Spec is dependent on the readme as described here
	// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html

	// Region for EKS Cluster
	// +kubebuilder:validation:Enum=us-west-2,us-east-1,eu-west-1
	Region EKSRegion `json:"region"`

	// RoleARN --role-arn
	// The Amazon Resource Name (ARN) of the IAM role that provides permis-
	// sions for Amazon EKS to make calls to other AWS  API  operations  on
	// your  behalf.  For more information, see Amazon EKS Service IAM Role
	// in the * Amazon EKS User Guide * .
	// TODO: we could simplify this to roleName.
	RoleARN string `json:"roleARN"`

	// ResourcesVPCConfig --resources-vpc-config (structure)
	// The VPC subnets and security groups  used  by  the  cluster  control
	// plane.  Amazon  EKS VPC resources have specific requirements to work
	// properly with Kubernetes. For more information, see Cluster VPC Con-
	// siderations  and Cluster Security Group Considerations in the Amazon
	// EKS User Guide . You must specify at  least  two  subnets.  You  may
	// specify  up  to  5  security groups, but we recommend that you use a
	// dedicated security group for your cluster control plane.
	//
	// VpcID of EKS cluster
	VpcID string `json:"vpcId"`
	// SubnetIds
	// Syntax:
	// subnetIds=string,string,
	SubnetIds []string `json:"subnetIds"`
	// SecurityGroupIds
	// Syntax:
	// securityGroupIds=string,string,
	SecurityGroupIds []string `json:"securityGroupIds"`

	// ClientRequestToken
	// --client-request-token (string)
	// Unique, case-sensitive identifier you provide to ensure the  idempo-
	// tency of the request.
	ClientRequestToken string `json:"clientRequestToken,omitempty"`

	// ClusterVersion --kubernetes-version (string)
	// The desired Kubernetes version for your cluster. If you do not spec-
	// ify a value here, the latest version  available  in  Amazon  EKS  is
	// used.
	ClusterVersion string `json:"clusterVersion,omitempty"`

	// CLIInput --cli-input-json  (string) Performs service operation based on the JSON
	// string provided. The JSON string follows the format provided by  --gen-
	// erate-cli-skeleton.  If  other  arguments  are  provided on the command
	// line, the CLI values will override the JSON-provided values. It is  not
	// possible to pass arbitrary binary values using a JSON-provided value as
	// the string will be taken literally.
	CLIInput string `json:"cliInput,omitempty"`

	// GenerateCLISkeleton --generate-cli-skeleton (string) Prints a  JSON  skeleton  to  standard
	// output without sending an API request. If provided with no value or the
	// value input, prints a sample input JSON that can be used as an argument
	// for  --cli-input-json.  If provided with the value output, it validates
	// the command inputs and returns a sample output JSON for that command.
	GenerateCLISkeleton string `json:"generateCLISkeleton,omitempty"`

	// WorkerNodes configuration for cloudformation
	WorkerNodes WorkerNodesSpec `json:"workerNodes"`

	// ConnectionSecretNameOverride set this override the generated name of Status.ConnectionSecretRef.Name
	ConnectionSecretNameOverride string `json:"connectionSecretNameOverride,omitempty"`

	// Kubernetes object references
	ClaimRef    *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef    *corev1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef corev1.LocalObjectReference `json:"providerRef"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

//WorkerNodesSpec - Worker node spec used to define cloudformation template that provisions workers for cluster
type WorkerNodesSpec struct {
	// KeyName The EC2 Key Pair to allow SSH access to the instances
	KeyName string `json:"keyName"`

	// NodeImageId The EC2 Key Pair to allow SSH access to the instances
	// defaults to region standard AMI
	NodeImageID string `json:"nodeImageId,omitempty"`

	// NodeInstanceType EC2 instance type for the node instances
	// +kubebuilder:validation:Enum=t2.small,t2.medium,t2.large,t2.xlarge,t2.2xlarge,t3.nano,t3.micro,t3.small,t3.medium,t3.large,t3.xlarge,t3.2xlarge,m3.medium,m3.large,m3.xlarge,m3.2xlarge,m4.large,m4.xlarge,m4.2xlarge,m4.4xlarge,m4.10xlarge,m5.large,m5.xlarge,m5.2xlarge,m5.4xlarge,m5.12xlarge,m5.24xlarge,c4.large,c4.xlarge,c4.2xlarge,c4.4xlarge,c4.8xlarge,c5.large,c5.xlarge,c5.2xlarge,c5.4xlarge,c5.9xlarge,c5.18xlarge,i3.large,i3.xlarge,i3.2xlarge,i3.4xlarge,i3.8xlarge,i3.16xlarge,r3.xlarge,r3.2xlarge,r3.4xlarge,r3.8xlarge,r4.large,r4.xlarge,r4.2xlarge,r4.4xlarge,r4.8xlarge,r4.16xlarge,x1.16xlarge,x1.32xlarge,p2.xlarge,p2.8xlarge,p2.16xlarge,p3.2xlarge,p3.8xlarge,p3.16xlarge,r5.large,r5.xlarge,r5.2xlarge,r5.4xlarge,r5.12xlarge,r5.24xlarge,r5d.large,r5d.xlarge,r5d.2xlarge,r5d.4xlarge,r5d.12xlarge,r5d.24xlarge,z1d.large,z1d.xlarge,z1d.2xlarge,z1d.3xlarge,z1d.6xlarge,z1d.12xlarge
	NodeInstanceType string `json:"nodeInstanceType"`

	// NodeAutoScalingGroupMinSize Minimum size of Node Group ASG.
	// default 1
	NodeAutoScalingGroupMinSize *int `json:"nodeAutoScalingGroupMinSize,omitempty"`

	// NodeAutoScalingGroupMaxSize Maximum size of Node Group ASG.
	// Default: 3
	NodeAutoScalingGroupMaxSize *int `json:"nodeAutoScalingGroupMaxSize,omitempty"`

	// NodeVolumeSize Node volume size in GB
	// Default: 20
	NodeVolumeSize *int `json:"nodeVolumeSize,omitempty"`

	// BootstrapArguments Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami
	// Default: ""
	BootstrapArguments string `json:"bootstrapArguments,omitempty"`

	// NodeGroupName Unique identifier for the Node Group.
	NodeGroupName string `json:"nodeGroupName,omitempty"`

	// ClusterControlPlaneSecurityGroup The security group of the cluster control plane.
	ClusterControlPlaneSecurityGroup string `json:"clusterControlPlaneSecurityGroup,omitempty"`
}

// EKSClusterStatus schema of the status of eks cluster
type EKSClusterStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase

	// State of the cluster (see status constants above)
	State string `json:"state,omitempty"`
	// ClusterName identifier
	ClusterName string `json:"resourceName,omitempty"`
	// Endpoint for cluster
	Endpoint string `json:"endpoint,omitempty"`
	// CloudFormationStackID Stack-id
	CloudFormationStackID string `json:"cloudformationStackId,omitempty"`

	ConnectionSecretRef corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EKSCluster is the Schema for the resources API
// +k8s:openapi-gen=true
// +groupName=compute.aws
type EKSCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EKSClusterSpec   `json:"spec,omitempty"`
	Status EKSClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EKSClusterList contains a list of EKSCluster items
type EKSClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EKSCluster `json:"items"`
}

// NewEKSClusterSpec from properties map
func NewEKSClusterSpec(properties map[string]string) *EKSClusterSpec {
	spec := &EKSClusterSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,
	}

	val, ok := properties["region"]
	if ok {
		spec.Region = EKSRegion(val)
	}

	val, ok = properties["roleARN"]
	if ok {
		spec.RoleARN = val
	}

	val, ok = properties["vpcId"]
	if ok {
		spec.VpcID = val
	}

	val, ok = properties["subnetIds"]
	if ok {
		spec.SubnetIds = append(spec.SubnetIds, strings.Split(val, ",")...)
	}

	val, ok = properties["securityGroupIds"]
	if ok {
		spec.SecurityGroupIds = append(spec.SecurityGroupIds, strings.Split(val, ",")...)
	}

	val, ok = properties["clusterVersion"]
	if ok {
		spec.ClusterVersion = val
	}

	val, ok = properties["workerKeyName"]
	if ok {
		spec.WorkerNodes.KeyName = val
	}

	val, ok = properties["workerNodeImageId"]
	if ok {
		spec.WorkerNodes.NodeImageID = val
	}

	val, ok = properties["workerNodeInstanceType"]
	if ok {
		spec.WorkerNodes.NodeInstanceType = val
	}

	val, ok = properties["workerNodeAutoScalingGroupMinSize"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.WorkerNodes.NodeAutoScalingGroupMinSize = &size
		}
	}

	val, ok = properties["workerNodeAutoScalingGroupMaxSize"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.WorkerNodes.NodeAutoScalingGroupMaxSize = &size
		}
	}

	val, ok = properties["workerNodeVolumeSize"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.WorkerNodes.NodeVolumeSize = &size
		}
	}

	val, ok = properties["workerBootstrapArguments"]
	if ok {
		spec.WorkerNodes.BootstrapArguments = val
	}

	val, ok = properties["workerNodeGroupName"]
	if ok {
		spec.WorkerNodes.NodeGroupName = val
	}
	val, ok = properties["workerClusterControlPlaneSecurityGroup"]
	if ok {
		spec.WorkerNodes.ClusterControlPlaneSecurityGroup = val
	}

	val, ok = properties["connectionSecretNameOverride"]
	if ok {
		spec.ConnectionSecretNameOverride = val
	}

	return spec
}

// ConnectionSecret with this cluster owner reference
func (e *EKSCluster) ConnectionSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       e.Namespace,
			Name:            e.ConnectionSecretName(),
			OwnerReferences: []metav1.OwnerReference{e.OwnerReference()},
		},
	}
}

// ConnectionSecretName returns the name of the connection secret
func (e *EKSCluster) ConnectionSecretName() string {
	if e.Status.ConnectionSecretRef.Name != "" {
		return e.Status.ConnectionSecretRef.Name
	} else if e.Spec.ConnectionSecretNameOverride != "" {
		return e.Spec.ConnectionSecretNameOverride
	}
	return e.Name
}

// SetConnectionSecretReference sets a local object reference to this secret in Status.ConnectionSecretRef
func (e *EKSCluster) SetConnectionSecretReference(secret *corev1.Secret) {
	e.Status.ConnectionSecretRef.Name = secret.Name
}

// Endpoint returns rds resource endpoint value saved in the status (could be empty)
func (e *EKSCluster) Endpoint() string {
	return e.Status.Endpoint
}

// SetEndpoint sets status endpoint field
func (e *EKSCluster) SetEndpoint(s string) {
	e.Status.Endpoint = s
}

// ObjectReference to this EKSCluster
func (e *EKSCluster) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(e.ObjectMeta, util.IfEmptyString(e.APIVersion, APIVersion), util.IfEmptyString(e.Kind, EKSClusterKind))
}

// OwnerReference to use this resource as an owner
func (e *EKSCluster) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(e.ObjectReference())
}

// State returns rds resource state value saved in the status (could be empty)
func (e *EKSCluster) State() string {
	return e.Status.State
}

// SetState sets status state field
func (e *EKSCluster) SetState(s string) {
	e.Status.State = s
}

// IsAvailable for usage/binding
func (e *EKSCluster) IsAvailable() bool {
	return e.State() == ClusterStatusActive
}

// IsBound
func (e *EKSCluster) IsBound() bool {
	return e.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound
func (e *EKSCluster) SetBound(state bool) {
	if state {
		e.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		e.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}

// GetAMIByRegion returns the default ami id for a given EKS region
func GetRegionAMI(region EKSRegion) (string, error) {
	if val, ok := workerNodeRegionAMI[region]; ok {
		return val, nil
	}
	return "", fmt.Errorf("not a valid EKS region, %s", string(region))
}
