/*
Copyright 2019 The Crossplane Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// VPCParameters defines the desired state of a VPC
type VPCParameters struct {
	// CIDRBlock is the IPv4 network range for the VPC, in CIDR notation. For example, 10.0.0.0/16.
	// +kubebuilder:validation:Required
	CIDRBlock string `json:"cidrBlock"`
}

// VPCSpec defines the desired state of a VPC
type VPCSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	VPCParameters                `json:",inline"`
}

// VPCExternalStatus keeps the state for the external resource
type VPCExternalStatus struct {
	// VPCState is the current state of the VPC.
	// +kubebuilder:validation:Enum=pending;available
	VPCState string `json:"vpcState"`

	// Tags represents to current ec2 tags.
	Tags []Tag `json:"tags,omitempty"`

	// VPCID is the ID of the VPC.
	VPCID string `json:"vpcId"`
}

// VPCStatus defines the observed state of an VPC
type VPCStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	VPCExternalStatus              `json:",inline"`
}

// +kubebuilder:object:root=true

// VPC is the Schema for the VPC API
// +kubebuilder:printcolumn:name="VPCID",type="string",JSONPath=".status.vpcId"
// +kubebuilder:printcolumn:name="CIDRBLOCK",type="string",JSONPath=".spec.cidrBlock"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.vpcState"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type VPC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCSpec   `json:"spec,omitempty"`
	Status VPCStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCList contains a list of VPCs
type VPCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPC `json:"items"`
}
