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
	conductorcorev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&RDSInstance{}, &RDSInstanceList{})
	SchemeBuilder.Register(&RDSInstanceClass{}, &RDSInstanceClassList{})
	SchemeBuilder.Register(&RDSInstanceClaim{}, &RDSInstanceClaimList{})
}

// RDSInstanceConfiguration contains properties needed to stand up RDS Database Instance
type RDSInstanceConfiguration struct {
	MasterUsername string   `json:"masterUsername"`
	Engine         string   `json:"engine"`                   // "postgres"
	Class          string   `json:"class"`                    // like "db.t2.micro"
	Size           int64    `json:"size"`                     // size in gb
	SecurityGroups []string `json:"securityGroups,omitempty"` // VPC Security groups
}

// RDSInstanceConnectionSecret name of the secret generated as a result of RDSInstnace creation and which contains
// RDS DB Instance connection information
type RDSInstanceConnectionSecret struct {
	ConnectionSecret string `json:"connectionSecretName"`
}

//----------------------------------------------------------------------------------------------------------------------

// RDSInstanceSpec defines the desired state of RDSInstance
type RDSInstanceSpec struct {
	conductorcorev1alpha1.ProviderReference `json:",inline"`
	RDSInstanceConfiguration                `json:",inline"`
	RDSInstanceConnectionSecret             `json:",inline"`
}

// RDSInstanceStatus defines the observed state of RDSInstance
type RDSInstanceStatus struct {
	conductorcorev1alpha1.ConditionedStatus
	State        string `json:"state,omitempty"`
	Message      string `json:"message,omitempty"`
	ProviderID   string `json:"providerID,omitempty"`   // the external ID to identify this resource in the cloud provider
	InstanceName string `json:"instanceName,omitempty"` // the generated DB Instance name
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstance is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.aws
type RDSInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RDSInstanceSpec   `json:"spec,omitempty"`
	Status RDSInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstanceList contains a list of RDSInstance
type RDSInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSInstance `json:"items"`
}

//----------------------------------------------------------------------------------------------------------------------

// RDSInstanceClassSpec
type RDSInstanceClassSpec struct {
	conductorcorev1alpha1.ProviderReference `json:",inline"`
	RDSInstanceConfiguration                `json:",inline"`
}

// RDSInstanceClassStatus
type RDSInstanceClassStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstanceClass is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.aws
type RDSInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RDSInstanceClassSpec   `json:"spec,omitempty"`
	Status RDSInstanceClassStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstanceClassList contains a list of RDSInstance
type RDSInstanceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSInstanceClass `json:"items"`
}

//----------------------------------------------------------------------------------------------------------------------

// RDSInstanceClaimSpec
type RDSInstanceClaimSpec struct {
	RDSInstanceConfiguration    `json:",inline"`
	RDSInstanceConnectionSecret `json:",inline"`

	ClassRef corev1.LocalObjectReference `json:"classRef"`
}

// RDSInstanceClaimStatus
type RDSInstanceClaimStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstanceClaim is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=database.aws
type RDSInstanceClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RDSInstanceClaimSpec   `json:"spec,omitempty"`
	Status RDSInstanceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RDSInstanceClaimList contains a list of RDSInstance
type RDSInstanceClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSInstanceClaim `json:"items"`
}
