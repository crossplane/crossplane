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
	"net/http"

	googlecompute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// NetworkSpec defines the desired state of Network
type NetworkSpec struct {
	v1alpha1.ResourceSpec `json:",inline"`
	GCPNetworkSpec        `json:",inline"`
}

// NetworkStatus defines the observed state of Network
type NetworkStatus struct {
	v1alpha1.ResourceStatus `json:",inline"`
	GCPNetworkStatus        `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Network is the Schema for the GCP Network API
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

// SetBindingPhase of this ReplicationGroup.
func (n *Network) SetBindingPhase(p v1alpha1.BindingPhase) {
	n.Status.SetBindingPhase(p)
}

// SetConditions of this ReplicationGroup.
func (n *Network) SetConditions(c ...v1alpha1.Condition) {
	n.Status.SetConditions(c...)
}

// GetBindingPhase of this ReplicationGroup.
func (n *Network) GetBindingPhase() v1alpha1.BindingPhase {
	return n.Status.GetBindingPhase()
}

// SetClaimReference of this ReplicationGroup.
func (n *Network) SetClaimReference(r *corev1.ObjectReference) {
	n.Spec.ClaimReference = r
}

// GetClaimReference of this ReplicationGroup.
func (n *Network) GetClaimReference() *corev1.ObjectReference {
	return n.Spec.ClaimReference
}

// SetClassReference of this ReplicationGroup.
func (n *Network) SetClassReference(r *corev1.ObjectReference) {
	n.Spec.ClassReference = r
}

// GetClassReference of this ReplicationGroup.
func (n *Network) GetClassReference() *corev1.ObjectReference {
	return n.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this ReplicationGroup.
func (n *Network) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	n.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this ReplicationGroup.
func (n *Network) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return n.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this ReplicationGroup.
func (n *Network) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	return n.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this ReplicationGroup.
func (n *Network) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	n.Spec.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

// GCPNetworkSpec contains fields of googlecompute.Network object that are
// configurable by the user, i.e. the ones that are not marked as [Output Only]
// In the future, this can be generated automatically.
type GCPNetworkSpec struct {
	// IPv4Range: Deprecated in favor of subnet mode networks. The range of
	// internal addresses that are legal on this network. This range is a
	// CIDR specification, for example: 192.168.0.0/16. Provided by the
	// client when the network is created.
	IPv4Range string `json:"IPv4Range,omitempty"`

	// AutoCreateSubnetworks: When set to true, the VPC network is created
	// in "auto" mode. When set to false, the VPC network is created in
	// "custom" mode.
	//
	// An auto mode VPC network starts with one subnet per region. Each
	// subnet has a predetermined range as described in Auto mode VPC
	// network IP ranges.
	AutoCreateSubnetworks bool `json:"autoCreateSubnetworks,omitempty"`

	// Description: An optional description of this resource. Provide this
	// field when you create the resource.
	Description string `json:"description,omitempty"`

	// Name: Name of the resource. Provided by the client when the resource
	// is created. The name must be 1-63 characters long, and comply with
	// RFC1035. Specifically, the name must be 1-63 characters long and
	// match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?. The first
	// character must be a lowercase letter, and all following characters
	// (except for the last character) must be a dash, lowercase letter, or
	// digit. The last character must be a lowercase letter or digit.
	Name string `json:"name,omitempty"`

	// RoutingConfig: The network-level routing configuration for this
	// network. Used by Cloud Router to determine what type of network-wide
	// routing behavior to enforce.
	RoutingConfig *GCPNetworkRoutingConfig `json:"routingConfig,omitempty"`
}

// GCPNetworkStatus is the complete mirror of googlecompute.Network but
// with deepcopy functions. In the future, this can be generated automatically.
type GCPNetworkStatus struct {
	// IPv4Range: Deprecated in favor of subnet mode networks. The range of
	// internal addresses that are legal on this network. This range is a
	// CIDR specification, for example: 192.168.0.0/16. Provided by the
	// client when the network is created.
	IPv4Range string `json:"IPv4Range,omitempty"`

	// AutoCreateSubnetworks: When set to true, the VPC network is created
	// in "auto" mode. When set to false, the VPC network is created in
	// "custom" mode.
	//
	// An auto mode VPC network starts with one subnet per region. Each
	// subnet has a predetermined range as described in Auto mode VPC
	// network IP ranges.
	AutoCreateSubnetworks bool `json:"autoCreateSubnetworks,omitempty"`

	// CreationTimestamp: [Output Only] Creation timestamp in RFC3339 text
	// format.
	CreationTimestamp string `json:"creationTimestamp,omitempty"`

	// Description: An optional description of this resource. Provide this
	// field when you create the resource.
	Description string `json:"description,omitempty"`

	// GatewayIPv4: [Output Only] The gateway address for default routing
	// out of the network, selected by GCP.
	GatewayIPv4 string `json:"gatewayIPv4,omitempty"`

	// Id: [Output Only] The unique identifier for the resource. This
	// identifier is defined by the server.
	ID uint64 `json:"id,omitempty"`

	// Kind: [Output Only] Type of the resource. Always compute#network for
	// networks.
	Kind string `json:"kind,omitempty"`

	// Name: Name of the resource. Provided by the client when the resource
	// is created. The name must be 1-63 characters long, and comply with
	// RFC1035. Specifically, the name must be 1-63 characters long and
	// match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?. The first
	// character must be a lowercase letter, and all following characters
	// (except for the last character) must be a dash, lowercase letter, or
	// digit. The last character must be a lowercase letter or digit.
	Name string `json:"name,omitempty"`

	// Peerings: [Output Only] A list of network peerings for the resource.
	Peerings []*GCPNetworkPeering `json:"peerings,omitempty"`

	// RoutingConfig: The network-level routing configuration for this
	// network. Used by Cloud Router to determine what type of network-wide
	// routing behavior to enforce.
	RoutingConfig *GCPNetworkRoutingConfig `json:"routingConfig,omitempty"`

	// SelfLink: [Output Only] Server-defined URL for the resource.
	SelfLink string `json:"selfLink,omitempty"`

	// Subnetworks: [Output Only] Server-defined fully-qualified URLs for
	// all subnetworks in this VPC network.
	Subnetworks []string `json:"subnetworks,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	GCPServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "IPv4Range") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "IPv4Range") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

// GenerateGCPNetworkSpec takes a *GCPNetworkStatus and returns *googlecompute.Network.
// It assigns only the fields that are writable, i.e. not labelled as [Output Only]
// in Google's reference.
func GenerateGCPNetworkSpec(in GCPNetworkSpec) *googlecompute.Network {
	n := &googlecompute.Network{}
	n.IPv4Range = in.IPv4Range
	n.AutoCreateSubnetworks = in.AutoCreateSubnetworks
	n.Description = in.Description
	n.Name = in.Name
	if in.RoutingConfig != nil {
		n.RoutingConfig = &googlecompute.NetworkRoutingConfig{
			RoutingMode: in.RoutingConfig.RoutingMode,
		}
	}
	return n
}

// GenerateGCPNetworkStatus takes a *googlecompute.Network and returns *GCPNetworkStatus
// It assings all the fields.
func GenerateGCPNetworkStatus(in googlecompute.Network) *GCPNetworkStatus {
	gn := &GCPNetworkStatus{
		IPv4Range:             in.IPv4Range,
		AutoCreateSubnetworks: in.AutoCreateSubnetworks,
		CreationTimestamp:     in.CreationTimestamp,
		Description:           in.Description,
		GatewayIPv4:           in.GatewayIPv4,
		ID:                    in.Id,
		Kind:                  in.Kind,
		Name:                  in.Name,
		RoutingConfig: &GCPNetworkRoutingConfig{
			RoutingMode: in.RoutingConfig.RoutingMode,
		},
		SelfLink:    in.SelfLink,
		Subnetworks: in.Subnetworks,
		GCPServerResponse: GCPServerResponse{
			HTTPStatusCode: in.ServerResponse.HTTPStatusCode,
			Header:         in.ServerResponse.Header,
		},
	}
	for _, p := range in.Peerings {
		gp := &GCPNetworkPeering{
			Name:                 p.Name,
			Network:              p.Network,
			State:                p.State,
			AutoCreateRoutes:     p.AutoCreateRoutes,
			ExchangeSubnetRoutes: p.ExchangeSubnetRoutes,
			StateDetails:         p.StateDetails,
		}
		gn.Peerings = append(gn.Peerings, gp)
	}
	return gn
}

// GCPNetworkPeering is the mirror of googlecompute.NetworkPeering but with deepcopy functions.
type GCPNetworkPeering struct {
	// AutoCreateRoutes: This field will be deprecated soon. Use the
	// exchange_subnet_routes field instead. Indicates whether full mesh
	// connectivity is created and managed automatically between peered
	// networks. Currently this field should always be true since Google
	// Compute Engine will automatically create and manage subnetwork routes
	// between two networks when peering state is ACTIVE.
	AutoCreateRoutes bool `json:"autoCreateRoutes,omitempty"`

	// ExchangeSubnetRoutes: Indicates whether full mesh connectivity is
	// created and managed automatically between peered networks. Currently
	// this field should always be true since Google Compute Engine will
	// automatically create and manage subnetwork routes between two
	// networks when peering state is ACTIVE.
	ExchangeSubnetRoutes bool `json:"exchangeSubnetRoutes,omitempty"`

	// Name: Name of this peering. Provided by the client when the peering
	// is created. The name must comply with RFC1035. Specifically, the name
	// must be 1-63 characters long and match regular expression
	// `[a-z]([-a-z0-9]*[a-z0-9])?`. The first character must be a lowercase
	// letter, and all the following characters must be a dash, lowercase
	// letter, or digit, except the last character, which cannot be a dash.
	Name string `json:"name,omitempty"`

	// Network: The URL of the peer network. It can be either full URL or
	// partial URL. The peer network may belong to a different project. If
	// the partial URL does not contain project, it is assumed that the peer
	// network is in the same project as the current network.
	Network string `json:"network,omitempty"`

	// State: [Output Only] State for the peering, either `ACTIVE` or
	// `INACTIVE`. The peering is `ACTIVE` when there's a matching
	// configuration in the peer network.
	//
	// Possible values:
	//   "ACTIVE"
	//   "INACTIVE"
	State string `json:"state,omitempty"`

	// StateDetails: [Output Only] Details about the current state of the
	// peering.
	StateDetails string `json:"stateDetails,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AutoCreateRoutes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AutoCreateRoutes") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

// GCPNetworkRoutingConfig is the mirror of googlecompute.NetworkRoutingConfig but with deepcopy functions.
type GCPNetworkRoutingConfig struct {
	// RoutingMode: The network-wide routing mode to use. If set to
	// REGIONAL, this network's Cloud Routers will only advertise routes
	// with subnets of this network in the same region as the router. If set
	// to GLOBAL, this network's Cloud Routers will advertise routes with
	// all subnets of this network, across regions.
	//
	// Possible values:
	//   "GLOBAL"
	//   "REGIONAL"
	RoutingMode string `json:"routingMode,omitempty"`

	// ForceSendFields is a list of field names (e.g. "RoutingMode") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "RoutingMode") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

// GCPServerResponse is the mirror of googleapi.ServerResponse but with deepcopy functions.
type GCPServerResponse struct {
	// HTTPStatusCode is the server's response status code. When using a
	// resource method's Do call, this will always be in the 2xx range.
	HTTPStatusCode int
	// Header contains the response header fields from the server.
	Header http.Header
}
