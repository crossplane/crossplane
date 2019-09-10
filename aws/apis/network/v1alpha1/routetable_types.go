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

// Route describes a route in a route table.
type Route struct {
	// The IPv4 CIDR address block used for the destination match. Routing decisions
	// are based on the most specific match.
	DestinationCIDRBlock string `json:"destinationCidrBlock"`

	// The ID of an internet gateway or virtual private gateway attached to your
	// VPC.
	GatewayID string `json:"gatewayId"`
}

// RouteState describes a route state in the route table.
type RouteState struct {
	// The state of the route. The blackhole state indicates that the route's target
	// isn't available (for example, the specified gateway isn't attached to the
	// VPC, or the specified NAT instance has been terminated).
	RouteState string `json:"routeState,omitempty"`

	Route `json:",inline"`
}

// Association describes an association between a route table and a subnet.
type Association struct {
	// The ID of the subnet. A subnet ID is not returned for an implicit association.
	SubnetID string `json:"subnetId"`
}

// AssociationState describes an association state in the route table.
type AssociationState struct {
	// Indicates whether this is the main route table.
	Main bool `json:"main"`

	// The ID of the association between a route table and a subnet.
	AssociationID string `json:"associationId"`

	Association `json:",inline"`
}

// RouteTableParameters defines the desired state of a RouteTable
type RouteTableParameters struct {
	// VPCID is the ID of the VPC.
	VPCID string `json:"vpcId"`

	// the routes in the route table
	Routes []Route `json:"routes,omitempty"`

	// The associations between the route table and one or more subnets.
	Associations []Association `json:"associations,omitempty"`
}

// RouteTableSpec defines the desired state of a RouteTable
type RouteTableSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	RouteTableParameters         `json:",inline"`
}

// RouteTableExternalStatus keeps the state for the external resource
type RouteTableExternalStatus struct {

	// RouteTableID is the ID of the RouteTable.
	RouteTableID string `json:"routeTableId"`

	// The actual routes created for the route table.
	Routes []RouteState `json:"routes,omitempty"`

	// The actual associations created for the route table.
	Associations []AssociationState `json:"associations,omitempty"`
}

// RouteTableStatus defines the observed state of an RouteTable
type RouteTableStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	RouteTableExternalStatus       `json:",inline"`
}

// +kubebuilder:object:root=true

// RouteTable is the Schema for the RouteTable API
// +kubebuilder:printcolumn:name="TABLEID",type="string",JSONPath=".status.routeTableId"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type RouteTable struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteTableSpec   `json:"spec,omitempty"`
	Status RouteTableStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RouteTableList contains a list of RouteTables
type RouteTableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RouteTable `json:"items"`
}
