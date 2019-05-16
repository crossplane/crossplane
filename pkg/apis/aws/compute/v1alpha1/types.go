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
	"strconv"
	"strings"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	"github.com/ghodss/yaml"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cluster statuses.
const (
	// The resource is inaccessible while it is being created.
	eksAuthConfigMapName = "aws-auth"
	eksAuthMapRolesKey   = "mapRoles"
	eksAuthMapUsersKey   = "mapUsers"

	// The resource is being created. The resource is inaccessible while it is being created.
	ClusterStatusCreating = "CREATING"
	ClusterStatusActive   = "ACTIVE"

	// TODO: Deleting and Failed currently not used. Implement usage or remove
	// ClusterStatusDeleting = "DELETING"
	// ClusterStatusFailed = "FAILED"
)

type KubernetesVersion string

// EKSRegion represents an EKS enabled AWS region.
type EKSRegion string

// EKS regions.
const (
	defaultNodeAutoScalingGroupMinSize     = 1
	defaultNodeAutoScalingGroupMaxSize     = 4
	defaultNodeAutoScalingGroupDesiredSize = 3
	defaultVolumeSize                      = 20
)

// EKSClusterSpec specifies the configuration for an EKS clquster.
type EKSClusterSpec struct {
	// Configuration of this Spec is dependent on the readme as described here
	// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html

	// Region for EKS Cluster
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
	// +kubebuilder:validation:Enum=1.10,1.11
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

	// NodePools node pools that are attached to this cluster
	NodePools []corev1.LocalObjectReference `json:"nodePools"`

	// ConnectionSecretNameOverride set this override the generated name of Status.ConnectionSecretRef.Name
	ConnectionSecretNameOverride string `json:"connectionSecretNameOverride,omitempty"`

	// MapRoles - each item maps an aws role to kubernetes groups
	// Default role that allows nodes access to communicate with master is autogenerated when a node pool comes online.
	// Additional map roles configuration are documented in the link below
	// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md
	//
	//# map arn:aws:iam::000000000000:role/KubernetesAdmin to cluster admin
	// - rolearn: arn:aws:iam::000000000000:role/KubernetesAdmin
	//   username: kubernetes-admin
	//    groups:
	//      - system:masters
	MapRoles []MapRole `json:"mapRoles,omitempty"`

	// MapUsers - each item maps an aws users to kubernetes groups
	// Additional map users configuration options are documented in the link below
	// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md
	//
	//# map user IAM user Alice in 000000000000 to user "alice" in group "system:masters"
	//- userarn: arn:aws:iam::000000000000:user/Alice
	//  username: alice
	//  groups:
	//    - system:masters
	MapUsers []MapUser `json:"mapUsers,omitempty"`

	// Kubernetes object references
	ClaimRef    *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef    *corev1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef corev1.LocalObjectReference `json:"providerRef"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// generateAWSAuthConfigMap generates the configmap for configure auth
func (r *EKSCluster) GenerateAWSAuthConfigMap(clusterNodeARN map[string]string) (*v1.ConfigMap, error) {
	data := map[string]string{}
	var roles []MapRole

	// Copy spec roles
	roles = append(roles, r.Spec.MapRoles...)

	// Generate system roles needed for nodes to communicate with master
	for _, arn := range clusterNodeARN {
		defaultNodeRole := MapRole{
			RoleARN:  arn,
			Username: "system:node:{{EC2PrivateDNSName}}",
			Groups:   []string{"system:bootstrappers", "system:nodes"},
		}
		roles = append(roles, defaultNodeRole)
	}

	// Serialize mapRoles
	rolesMarshalled, err := yaml.Marshal(roles)
	if err != nil {
		return nil, err
	}

	data[eksAuthMapRolesKey] = string(rolesMarshalled)

	// Serialize mapUsers
	if len(r.Spec.MapUsers) > 0 {
		usersMarshalled, err := yaml.Marshal(r.Spec.MapUsers)
		if err != nil {
			return nil, err
		}
		data[eksAuthMapUsersKey] = string(usersMarshalled)
	}

	name := eksAuthConfigMapName
	namespace := "kube-system"
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	return &cm, nil
}

// MapRole maps an aws role to kubernetes groups
// see: https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md
type MapRole struct {
	RoleARN  string   `json:"rolearn"`
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

// MapUser maps an aws user to kubernetes groups
// see: https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md
type MapUser struct {
	UserARN  string   `json:"userarn"`
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

//EKSNodePoolSpec - Worker node spec used to define cloudformation template that provisions workers for cluster
type EKSNodePoolSpec struct {
	// ClusterName is the name of the masterNode
	ClusterName string `json:"resourceName,omitempty"`

	// Region for EKS Cluster
	Region EKSRegion `json:"region"`

	// ClusterRef reference to kubernetes master
	ClusterRef corev1.LocalObjectReference `json:"clusterRef"`

	// KeyName The EC2 Key Pair to allow SSH access to the instances
	KeyName string `json:"keyName"`

	// NodeImageId The EC2 Key Pair to allow SSH access to the instances
	// Please see https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html
	NodeImageID string `json:"nodeImageId"`

	// NodeInstanceType EC2 instance type for the node instances
	// +kubebuilder:validation:Enum=t2.small,t2.medium,t2.large,t2.xlarge,t2.2xlarge,t3.nano,t3.micro,t3.small,t3.medium,t3.large,t3.xlarge,t3.2xlarge,m3.medium,m3.large,m3.xlarge,m3.2xlarge,m4.large,m4.xlarge,m4.2xlarge,m4.4xlarge,m4.10xlarge,m5.large,m5.xlarge,m5.2xlarge,m5.4xlarge,m5.12xlarge,m5.24xlarge,c4.large,c4.xlarge,c4.2xlarge,c4.4xlarge,c4.8xlarge,c5.large,c5.xlarge,c5.2xlarge,c5.4xlarge,c5.9xlarge,c5.18xlarge,i3.large,i3.xlarge,i3.2xlarge,i3.4xlarge,i3.8xlarge,i3.16xlarge,r3.xlarge,r3.2xlarge,r3.4xlarge,r3.8xlarge,r4.large,r4.xlarge,r4.2xlarge,r4.4xlarge,r4.8xlarge,r4.16xlarge,x1.16xlarge,x1.32xlarge,p2.xlarge,p2.8xlarge,p2.16xlarge,p3.2xlarge,p3.8xlarge,p3.16xlarge,r5.large,r5.xlarge,r5.2xlarge,r5.4xlarge,r5.12xlarge,r5.24xlarge,r5d.large,r5d.xlarge,r5d.2xlarge,r5d.4xlarge,r5d.12xlarge,r5d.24xlarge,z1d.large,z1d.xlarge,z1d.2xlarge,z1d.3xlarge,z1d.6xlarge,z1d.12xlarge
	NodeInstanceType string `json:"nodeInstanceType"`

	// NodeAutoScalingGroupMinSize Minimum size of Node Group ASG.
	// default 1
	NodeAutoScalingGroupMinSize *int `json:"nodeAutoScalingGroupMinSize,omitempty"`

	// NodeAutoScalingGroupMaxSize Maximum size of Node Group ASG.
	// Default: 4
	NodeAutoScalingGroupMaxSize *int `json:"nodeAutoScalingGroupMaxSize,omitempty"`

	// NodeAutoScalingGroupDesiredCapacity Desired capacity of Node Group ASG.
	// Default: 3
	NodeAutoScalingGroupDesiredCapacity *int `json:"nodeAutoScalingGroupDesiredCapacity,omitempty"`

	// NodeVolumeSize Node volume size in GB
	// Default: 20
	NodeVolumeSize *int `json:"nodeVolumeSize,omitempty"`

	// BootstrapArguments Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami
	// Default: ""
	// TODO: --kubelet-extra-args '--node-labels=spotfleet=true --register-with-taints=spotInstance=true:PreferNoSchedule'
	BootstrapArguments string `json:"bootstrapArguments,omitempty"`

	// NodeGroupName Unique identifier for the Node Group.
	NodeGroupName string `json:"nodeGroupName,omitempty"`

	// ClusterControlPlaneSecurityGroup The security group of the cluster control plane.
	ClusterControlPlaneSecurityGroup string `json:"clusterControlPlaneSecurityGroup,omitempty"`

	// Kubernetes object references
	ClaimRef    *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef    *corev1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef corev1.LocalObjectReference `json:"providerRef"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

func (e *EKSNodePoolSpec) ApplyDefaultsIfNotSet() {
	if e.NodeAutoScalingGroupMinSize == nil {
		size := defaultNodeAutoScalingGroupMinSize
		e.NodeAutoScalingGroupMinSize = &size
	}

	if e.NodeAutoScalingGroupMaxSize == nil {
		size := defaultNodeAutoScalingGroupMaxSize
		e.NodeAutoScalingGroupMaxSize = &size
	}

	if e.NodeAutoScalingGroupDesiredCapacity == nil {
		size := defaultNodeAutoScalingGroupDesiredSize
		e.NodeAutoScalingGroupMaxSize = &size
	}

	if e.NodeVolumeSize == nil {
		size := defaultVolumeSize
		e.NodeVolumeSize = &size
	}
}

// NewEKSNodePoolSpec from properties map
func NewEKSNodePoolSpec(properties map[string]string) *EKSNodePoolSpec {
	spec := &EKSNodePoolSpec{}

	val, ok := properties["clusterRef"]
	if ok {
		spec.ClusterRef.Name = val
	}

	val, ok = properties["keyName"]
	if ok {
		spec.KeyName = val
	}

	val, ok = properties["imageId"]
	if ok {
		spec.NodeImageID = val
	}

	val, ok = properties["instanceType"]
	if ok {
		spec.NodeInstanceType = val
	}

	val, ok = properties["autoScalingGroupMinSize"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.NodeAutoScalingGroupMinSize = &size
		}
	}

	val, ok = properties["autoScalingGroupMaxSize"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.NodeAutoScalingGroupMaxSize = &size
		}
	}

	val, ok = properties["autoScalingGroupDesiredCapacity"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.NodeAutoScalingGroupDesiredCapacity = &size
		}
	}

	val, ok = properties["volumeSize"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.NodeVolumeSize = &size
		}
	}

	val, ok = properties["bootstrapArguments"]
	if ok {
		spec.BootstrapArguments = val
	}

	val, ok = properties["groupName"]
	if ok {
		spec.NodeGroupName = val
	}
	val, ok = properties["clusterControlPlaneSecurityGroup"]
	if ok {
		spec.ClusterControlPlaneSecurityGroup = val
	}

	return spec
}

// EKSClusterStatus schema of the status of eks cluster
type EKSNodePoolStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	// State of the NodePool (see status constants above)
	State string `json:"state,omitempty"`
	// CloudFormationStackID Stack-id
	CloudFormationStackID string `json:"cloudformationStackId,omitempty"`

	// NodeInstanceRole - This role must be added to mapRoles aws-auth configmap in cluster.
	NodeInstanceRoleARN string `json:"NodeInstanceRoleARN,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EKSNodePool is the Schema for the resources API
// +k8s:openapi-gen=true
// +groupName=compute.aws
type EKSNodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EKSNodePoolSpec   `json:"spec,omitempty"`
	Status EKSNodePoolStatus `json:"status,omitempty"`
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

	ConnectionSecretRef corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EKSCluster is the Schema for the resources API
// +k8s:openapi-gen=true
// +groupName=compute.aws
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLUSTER-NAME",type="string",JSONPath=".status.clusterName"
// +kubebuilder:printcolumn:name="ENDPOINT",type="string",JSONPath=".status.endpoint"
// +kubebuilder:printcolumn:name="CLUSTER-CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="LOCATION",type="string",JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
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
		ReclaimPolicy:                corev1alpha1.ReclaimRetain,
		Region:                       EKSRegion(properties["region"]),
		RoleARN:                      properties["roleARN"],
		VpcID:                        properties["vpcId"],
		ClusterVersion:               properties["clusterVersion"],
		SubnetIds:                    parseSlice(properties["subnetIds"]),
		SecurityGroupIds:             parseSlice(properties["securityGroupIds"]),
		ConnectionSecretNameOverride: properties["connectionSecretNameOverride"],
	}

	return spec
}

// parseSlice parses a string of comma separated strings, for example
// "value1, value2", into a string slice.
func parseSlice(s string) []string {
	if s == "" {
		return nil
	}
	sl := make([]string, 0, strings.Count(s, ",")+1)
	for _, sub := range strings.Split(s, ",") {
		sl = append(sl, strings.TrimSpace(sub))
	}
	return sl
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

// IsBound returns true if this cluster is bound to a resource claim.
func (e *EKSCluster) IsBound() bool {
	return e.Status.IsBound()
}

// SetBound specifies whether this cluster is bound to a resource claim.
func (e *EKSCluster) SetBound(bound bool) {
	e.Status.SetBound(bound)
}
