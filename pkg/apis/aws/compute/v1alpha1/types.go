/*
Copyright 2018 The Conductor Authors.

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
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/util"
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

type EKSClusterSpec struct {
	// RoleARN --role-arn
	// The Amazon Resource Name (ARN) of the IAM role that provides permis-
	// sions for Amazon EKS to make calls to other AWS  API  operations  on
	// your  behalf.  For more information, see Amazon EKS Service IAM Role
	// in the * Amazon EKS User Guide * .
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
	// Syntax:
	// subnetIds=string,string,
	SubnetIds []string `json:"subnetIds"`
	// Syntax:
	// securityGroupsIds=string,string,
	SecurityGroupsIds []string `json:"securityGroupIds"`

	// ClientRequestToken
	// --client-request-token (string)
	// Unique, case-sensitive identifier you provide to ensure the  idempo-
	// tency of the request.
	ClientRequestToken string `json:"clientRequestToken,omitempty"`

	// ClusterVersion --kubernetes-version (string)
	// The desired Kubernetes version for your clustee. If you do not spec-
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

	// Kubernetes object references
	ClaimRef            *corev1.ObjectReference      `json:"claimRef,omitempty"`
	ClassRef            *corev1.ObjectReference      `json:"classRef,omitempty"`
	ConnectionSecretRef *corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
	ProviderRef         corev1.LocalObjectReference  `json:"providerRef"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

type EKSClusterStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase

	// State of the cluster (see status constants above)
	State string `json:"state,omitempty"`
	// ClusterName identifier
	ClusterName string `json:"resourceName,omitempty"`
	// Endpoint for cluster
	Endpoint string `json:"endpoint,omitempty"`
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
// TODO: will be using once abstract resource support is added
func NewEKSClusterSpec(properties map[string]string) *EKSClusterSpec {
	spec := &EKSClusterSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,
	}
	// TODO: complete spec fields assignment
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

// ConnectionSecretName returns a secret name from the reference
func (e *EKSCluster) ConnectionSecretName() string {
	if e.Spec.ConnectionSecretRef == nil {
		e.Spec.ConnectionSecretRef = &corev1.LocalObjectReference{
			Name: e.Name,
		}
	} else if e.Spec.ConnectionSecretRef.Name == "" {
		e.Spec.ConnectionSecretRef.Name = e.Name
	}

	return e.Spec.ConnectionSecretRef.Name
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
