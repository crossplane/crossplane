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

// InternetGatewayParameters defines the desired state of a InternetGateway
type InternetGatewayParameters struct {
	// the VPC to attach the gateway to.
	VPCID string `json:"vpcId"`
}

// InternetGatewaySpec defines the desired state of a InternetGateway
type InternetGatewaySpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	InternetGatewayParameters    `json:",inline"`
}

// InternetGatewayAttachment describes the attachment of a VPC to an internet gateway or an egress-only
// internet gateway.
type InternetGatewayAttachment struct {
	// The current state of the attachment. For an internet gateway, the state is
	// available when attached to a VPC; otherwise, this value is not returned.
	// +kubebuilder:validation:Enum=available;attaching;attached;detaching;detached
	AttachmentStatus string `json:"attachmentStatus"`

	// VPCID is the ID of the attached VPC.
	VPCID string `json:"vpcId"`
}

// InternetGatewayExternalStatus keeps the state for the external resource
type InternetGatewayExternalStatus struct {
	// Any VPCs attached to the internet gateway.
	Attachments []InternetGatewayAttachment `json:"attachments,omitempty"`

	// The ID of the internet gateway.
	InternetGatewayID string `json:"internetGatewayId"`

	// Tags represents to current ec2 tags.
	Tags []Tag `json:"tags,omitempty"`
}

// InternetGatewayStatus defines the observed state of an InternetGateway
type InternetGatewayStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	InternetGatewayExternalStatus  `json:",inline"`
}

// +kubebuilder:object:root=true

// InternetGateway is the Schema for the InternetGateway API
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.internetGatewayId"
// +kubebuilder:printcolumn:name="VPCID",type="string",JSONPath=".spec.vpcId"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type InternetGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InternetGatewaySpec   `json:"spec,omitempty"`
	Status InternetGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InternetGatewayList contains a list of InternetGateways
type InternetGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InternetGateway `json:"items"`
}
