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

// Tag defines a tag
type Tag struct {
	// Key is the name of the tag.
	Key string `json:"key"`
	// Value is the value of the tag.
	Value string `json:"value"`
}

// Subnet represents a aws subnet
type Subnet struct {
	// Specifies the identifier of the subnet.
	SubnetID string `json:"subnetID"`

	// Specifies the status of the subnet.
	SubnetStatus string `json:"subnetStatus"`
}

// DBSubnetGroupParameters defines the desired state of a DBSubnetGroup
type DBSubnetGroupParameters struct {
	// The description for the DB subnet group.
	DBSubnetGroupDescription string `json:"description"`

	// The name for the DB subnet group. This value is stored as a lowercase string.
	DBSubnetGroupName string `json:"groupName"`

	// The EC2 Subnet IDs for the DB subnet group.
	SubnetIDs []string `json:"subnetIds"`

	// A list of tags. For more information, see Tagging Amazon RDS Resources (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Tagging.html)
	// in the Amazon RDS User Guide.
	Tags []Tag `json:"tags,omitempty"`
}

// DBSubnetGroupSpec defines the desired state of a DBSubnetGroup
type DBSubnetGroupSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	DBSubnetGroupParameters      `json:",inline"`
}

// DBSubnetGroupExternalStatus keeps the state for the external resource
type DBSubnetGroupExternalStatus struct {
	// The Amazon Resource Name (ARN) for the DB subnet group.
	DBSubnetGroupARN string `json:"groupArn"`

	// Provides the status of the DB subnet group.
	SubnetGroupStatus string `json:"groupStatus"`

	// Contains a list of Subnet elements.
	Subnets []Subnet `json:"subnets"`

	// Provides the VpcId of the DB subnet group.
	VPCID string `json:"vpcId"`
}

// DBSubnetGroupStatus defines the observed state of an DBSubnetGroup
type DBSubnetGroupStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	DBSubnetGroupExternalStatus    `json:",inline"`
}

// +kubebuilder:object:root=true

// DBSubnetGroup is the Schema for the DBSubnetGroup API
// +kubebuilder:printcolumn:name="GROUPNAME",type="string",JSONPath=".spec.groupName"
// +kubebuilder:printcolumn:name="DESCRIPTION",type="string",JSONPath=".spec.description"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.groupStatus"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type DBSubnetGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DBSubnetGroupSpec   `json:"spec,omitempty"`
	Status DBSubnetGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DBSubnetGroupList contains a list of DBSubnetGroups
type DBSubnetGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DBSubnetGroup `json:"items"`
}
