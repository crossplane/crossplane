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
	googlecompute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// SubnetworkSpec defines the desired state of Network
type SubnetworkSpec struct {
	v1alpha1.ResourceSpec `json:",inline"`
	GCPSubnetworkSpec     `json:",inline"`
}

// GenerateSubnetwork creates a *googlecompute.Subnetwork object using GCPSubnetworkSpec.
func GenerateSubnetwork(in GCPSubnetworkSpec) *googlecompute.Subnetwork {
	s := &googlecompute.Subnetwork{}
	s.Name = in.Name
	s.Description = in.Description
	s.EnableFlowLogs = in.EnableFlowLogs
	s.IpCidrRange = in.IPCidrRange
	s.Network = in.Network
	s.PrivateIpGoogleAccess = in.PrivateIPGoogleAccess
	s.Region = in.Region
	for _, val := range in.SecondaryIPRanges {
		obj := &googlecompute.SubnetworkSecondaryRange{
			IpCidrRange:     val.IPCidrRange,
			RangeName:       val.RangeName,
			NullFields:      val.NullFields,
			ForceSendFields: val.ForceSendFields,
		}
		s.SecondaryIpRanges = append(s.SecondaryIpRanges, obj)
	}
	return s
}

// GenerateGCPSubnetworkStatus creates a GCPSubnetworkStatus object using *googlecompute.Subnetwork.
func GenerateGCPSubnetworkStatus(in *googlecompute.Subnetwork) GCPSubnetworkStatus {
	s := GCPSubnetworkStatus{}
	s.Name = in.Name
	s.Description = in.Description
	s.EnableFlowLogs = in.EnableFlowLogs
	s.Fingerprint = in.Fingerprint
	s.IPCidrRange = in.IpCidrRange
	s.Network = in.Network
	s.PrivateIPGoogleAccess = in.PrivateIpGoogleAccess
	s.Region = in.Region
	for _, val := range in.SecondaryIpRanges {
		obj := &GCPSubnetworkSecondaryRange{
			IPCidrRange:     val.IpCidrRange,
			RangeName:       val.RangeName,
			NullFields:      val.NullFields,
			ForceSendFields: val.ForceSendFields,
		}
		s.SecondaryIPRanges = append(s.SecondaryIPRanges, obj)
	}
	s.CreationTimestamp = in.CreationTimestamp
	s.GatewayAddress = in.GatewayAddress
	s.ID = in.Id
	s.Kind = in.Kind
	s.SelfLink = in.SelfLink
	s.GCPServerResponse = GCPServerResponse{
		HTTPStatusCode: in.ServerResponse.HTTPStatusCode,
		Header:         in.ServerResponse.Header,
	}
	return s
}

// SubnetworkStatus defines the observed state of Network
type SubnetworkStatus struct {
	v1alpha1.ResourceStatus `json:",inline"`
	GCPSubnetworkStatus     `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Subnetwork is the Schema for the GCP Network API
type Subnetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetworkSpec   `json:"spec,omitempty"`
	Status SubnetworkStatus `json:"status,omitempty"`
}

// SetBindingPhase of this Subnetwork.
func (s *Subnetwork) SetBindingPhase(p v1alpha1.BindingPhase) {
	s.Status.SetBindingPhase(p)
}

// SetConditions of this Subnetwork.
func (s *Subnetwork) SetConditions(c ...v1alpha1.Condition) {
	s.Status.SetConditions(c...)
}

// GetBindingPhase of this Subnetwork.
func (s *Subnetwork) GetBindingPhase() v1alpha1.BindingPhase {
	return s.Status.GetBindingPhase()
}

// SetClaimReference of this Subnetwork.
func (s *Subnetwork) SetClaimReference(r *corev1.ObjectReference) {
	s.Spec.ClaimReference = r
}

// GetClaimReference of this Subnetwork.
func (s *Subnetwork) GetClaimReference() *corev1.ObjectReference {
	return s.Spec.ClaimReference
}

// SetClassReference of this Subnetwork.
func (s *Subnetwork) SetClassReference(r *corev1.ObjectReference) {
	s.Spec.ClassReference = r
}

// GetClassReference of this Subnetwork.
func (s *Subnetwork) GetClassReference() *corev1.ObjectReference {
	return s.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this Subnetwork.
func (s *Subnetwork) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	s.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this Subnetwork.
func (s *Subnetwork) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return s.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this Subnetwork.
func (s *Subnetwork) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	return s.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this Subnetwork.
func (s *Subnetwork) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	s.Spec.ReclaimPolicy = p
}

// GCPSubnetworkSpec contains fields of googlecompute.Subnetwork object that are
// configurable by the user, i.e. the ones that are not marked as [Output Only]
// In the future, this can be generated automatically.
type GCPSubnetworkSpec struct {
	// Description: An optional description of this resource. Provide this
	// property when you create the resource. This field can be set only at
	// resource creation time.
	Description string `json:"description,omitempty"`

	// EnableFlowLogs: Whether to enable flow logging for this subnetwork.
	// If this field is not explicitly set, it will not appear in get
	// listings. If not set the default behavior is to disable flow logging.
	EnableFlowLogs bool `json:"enableFlowLogs,omitempty"`

	// IPCidrRange: The range of internal addresses that are owned by this
	// subnetwork. Provide this property when you create the subnetwork. For
	// example, 10.0.0.0/8 or 192.168.0.0/16. Ranges must be unique and
	// non-overlapping within a network. Only IPv4 is supported. This field
	// can be set only at resource creation time.
	IPCidrRange string `json:"ipCidrRange"`

	// Name: The name of the resource, provided by the client when initially
	// creating the resource. The name must be 1-63 characters long, and
	// comply with RFC1035. Specifically, the name must be 1-63 characters
	// long and match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?`
	// which means the first character must be a lowercase letter, and all
	// following characters must be a dash, lowercase letter, or digit,
	// except the last character, which cannot be a dash.
	Name string `json:"name"`

	// Network: The URL of the network to which this subnetwork belongs,
	// provided by the client when initially creating the subnetwork. Only
	// networks that are in the distributed mode can have subnetworks. This
	// field can be set only at resource creation time.
	Network string `json:"network"`

	// PrivateIPGoogleAccess: Whether the VMs in this subnet can access
	// Google services without assigned external IP addresses. This field
	// can be both set at resource creation time and updated using
	// setPrivateIPGoogleAccess.
	PrivateIPGoogleAccess bool `json:"PrivateIPGoogleAccess,omitempty"`

	// Region: URL of the region where the Subnetwork resides. This field
	// can be set only at resource creation time.
	Region string `json:"region,omitempty"`

	// SecondaryIPRanges: An array of configurations for secondary IP ranges
	// for VM instances contained in this subnetwork. The primary IP of such
	// VM must belong to the primary ipCidrRange of the subnetwork. The
	// alias IPs may belong to either primary or secondary ranges. This
	// field can be updated with a patch request.
	SecondaryIPRanges []*GCPSubnetworkSecondaryRange `json:"secondaryIpRanges,omitempty"`
}

// GCPSubnetworkStatus is the complete mirror of googlecompute.Subnetwork but
// with deepcopy functions. In the future, this can be generated automatically.
type GCPSubnetworkStatus struct {
	// CreationTimestamp: [Output Only] Creation timestamp in RFC3339 text
	// format.
	CreationTimestamp string `json:"creationTimestamp,omitempty"`

	// Description: An optional description of this resource. Provide this
	// property when you create the resource. This field can be set only at
	// resource creation time.
	Description string `json:"description,omitempty"`

	// EnableFlowLogs: Whether to enable flow logging for this subnetwork.
	// If this field is not explicitly set, it will not appear in get
	// listings. If not set the default behavior is to disable flow logging.
	EnableFlowLogs bool `json:"enableFlowLogs,omitempty"`

	// Fingerprint: Fingerprint of this resource. A hash of the contents
	// stored in this object. This field is used in optimistic locking. This
	// field will be ignored when inserting a Subnetwork. An up-to-date
	// fingerprint must be provided in order to update the Subnetwork,
	// otherwise the request will fail with error 412 conditionNotMet.
	//
	// To see the latest fingerprint, make a get() request to retrieve a
	// Subnetwork.
	Fingerprint string `json:"fingerprint,omitempty"`

	// GatewayAddress: [Output Only] The gateway address for default routes
	// to reach destination addresses outside this subnetwork.
	GatewayAddress string `json:"gatewayAddress,omitempty"`

	// Id: [Output Only] The unique identifier for the resource. This
	// identifier is defined by the server.
	ID uint64 `json:"id,omitempty"`

	// IPCidrRange: The range of internal addresses that are owned by this
	// subnetwork. Provide this property when you create the subnetwork. For
	// example, 10.0.0.0/8 or 192.168.0.0/16. Ranges must be unique and
	// non-overlapping within a network. Only IPv4 is supported. This field
	// can be set only at resource creation time.
	IPCidrRange string `json:"ipCidrRange,omitempty"`

	// Kind: [Output Only] Type of the resource. Always compute#subnetwork
	// for Subnetwork resources.
	Kind string `json:"kind,omitempty"`

	// Name: The name of the resource, provided by the client when initially
	// creating the resource. The name must be 1-63 characters long, and
	// comply with RFC1035. Specifically, the name must be 1-63 characters
	// long and match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?`
	// which means the first character must be a lowercase letter, and all
	// following characters must be a dash, lowercase letter, or digit,
	// except the last character, which cannot be a dash.
	Name string `json:"name,omitempty"`

	// Network: The URL of the network to which this subnetwork belongs,
	// provided by the client when initially creating the subnetwork. Only
	// networks that are in the distributed mode can have subnetworks. This
	// field can be set only at resource creation time.
	Network string `json:"network,omitempty"`

	// PrivateIPGoogleAccess: Whether the VMs in this subnet can access
	// Google services without assigned external IP addresses. This field
	// can be both set at resource creation time and updated using
	// setPrivateIPGoogleAccess.
	PrivateIPGoogleAccess bool `json:"PrivateIPGoogleAccess,omitempty"`

	// Region: URL of the region where the Subnetwork resides. This field
	// can be set only at resource creation time.
	Region string `json:"region,omitempty"`

	// SecondaryIPRanges: An array of configurations for secondary IP ranges
	// for VM instances contained in this subnetwork. The primary IP of such
	// VM must belong to the primary ipCidrRange of the subnetwork. The
	// alias IPs may belong to either primary or secondary ranges. This
	// field can be updated with a patch request.
	SecondaryIPRanges []*GCPSubnetworkSecondaryRange `json:"secondaryIpRanges,omitempty"`

	// SelfLink: [Output Only] Server-defined URL for the resource.
	SelfLink string `json:"selfLink,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	GCPServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "CreationTimestamp")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CreationTimestamp") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

// GCPSubnetworkSecondaryRange is the mirror of googlecompute.SubnetworkSecondaryRange but with deepcopy functions.
type GCPSubnetworkSecondaryRange struct {
	// IPCidrRange: The range of IP addresses belonging to this subnetwork
	// secondary range. Provide this property when you create the
	// subnetwork. Ranges must be unique and non-overlapping with all
	// primary and secondary IP ranges within a network. Only IPv4 is
	// supported.
	IPCidrRange string `json:"ipCidrRange,omitempty"`

	// RangeName: The name associated with this subnetwork secondary range,
	// used when adding an alias IP range to a VM instance. The name must be
	// 1-63 characters long, and comply with RFC1035. The name must be
	// unique within the subnetwork.
	RangeName string `json:"rangeName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "IPCidrRange") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "IPCidrRange") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

// +kubebuilder:object:root=true

// SubnetworkList contains a list of Network
type SubnetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnetwork `json:"items"`
}
