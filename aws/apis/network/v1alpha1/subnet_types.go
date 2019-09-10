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

// SubnetParameters defines the desired state of a Subnet
type SubnetParameters struct {
	// CIDRBlock is the IPv4 network range for the Subnet, in CIDR notation. For example, 10.0.0.0/18.
	CIDRBlock string `json:"cidrBlock"`

	// The Availability Zone for the subnet.
	// Default: AWS selects one for you. If you create more than one subnet in your
	// VPC, we may not necessarily select a different zone for each subnet.
	AvailabilityZone string `json:"availabilityZone"`

	// VPCID is the ID of the VPC.
	VPCID string `json:"vpcId"`
}

// SubnetSpec defines the desired state of a Subnet
type SubnetSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	SubnetParameters             `json:",inline"`
}

// SubnetExternalStatus keeps the state for the external resource
type SubnetExternalStatus struct {
	// SubnetState is the current state of the Subnet.
	// +kubebuilder:validation:Enum=pending;available
	SubnetState string `json:"subnetState"`

	// Tags represents to current ec2 tags.
	Tags []Tag `json:"tags,omitempty"`

	// SubnetID is the ID of the Subnet.
	SubnetID string `json:"subnetId"`
}

// SubnetStatus defines the observed state of an Subnet
type SubnetStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	SubnetExternalStatus           `json:",inline"`
}

// +kubebuilder:object:root=true

// Subnet is the Schema for the Subnet API
// +kubebuilder:printcolumn:name="SUBNETID",type="string",JSONPath=".status.subnetId"
// +kubebuilder:printcolumn:name="VPCID",type="string",JSONPath=".spec.vpcId"
// +kubebuilder:printcolumn:name="CIDRBLOCK",type="string",JSONPath=".spec.cidrBlock"
// +kubebuilder:printcolumn:name="AZ",type="string",JSONPath=".spec.availabilityZone"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.subnetState"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type Subnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetSpec   `json:"spec,omitempty"`
	Status SubnetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SubnetList contains a list of Subnets
type SubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnet `json:"items"`
}
