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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type clusterAddon string

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
	//	Shorthand Syntax:
	//
	//	subnetIds=string,string,securityGroupIds=string,string
	//
	//	JSON Syntax:
	//
	//	{
	//		"subnetIds": ["string", ...],
	//		"securityGroupIds": ["string", ...]
	//	}
	ResourcesVPCConfig string `json:"resourcesVPCConfig"`

	// ClientRequestToken
	// --client-request-token (string)
	// Unique, case-sensitive identifier you provide to ensure the  idempo-
	// tency of the request.
	ClientRequestToken string `json:"clientRequestToken"`

	// ClusterVersion --kubernetes-version (string)
	// The desired Kubernetes version for your cluster. If you do not spec-
	// ify a value here, the latest version  available  in  Amazon  EKS  is
	// used.
	ClusterVersion string `json:"clusterVersion"`

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


	// ProviderRef - reference to GCP provider object
	ProviderRef v1.LocalObjectReference `json:"providerRef"`

	// ConnectionSecretRef - reference to EKS Cluster connection secret which will be created and contain connection related data
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef"`
}

type EKSClusterStatus struct {
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EKSCluster is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=container.gcp
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

func init() {
	SchemeBuilder.Register(&EKSCluster{}, &EKSClusterList{})
}
