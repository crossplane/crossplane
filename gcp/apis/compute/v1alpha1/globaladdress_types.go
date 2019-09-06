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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// GlobalAddressSpec defines the desired state of a GlobalAddress
type GlobalAddressSpec struct {
	v1alpha1.ResourceSpec   `json:",inline"`
	GlobalAddressParameters `json:",inline"`
}

// GlobalAddressParameters specifies the configuration of a GlobalAddress.
type GlobalAddressParameters struct {
	// Address: The static IP address represented by this resource.
	// +optional
	Address *string `json:"address,omitempty"`

	// AddressType: The type of address to reserve, either INTERNAL or
	// EXTERNAL. If unspecified, defaults to EXTERNAL.
	//
	// Possible values:
	//   "EXTERNAL"
	//   "INTERNAL"
	//   "UNSPECIFIED_TYPE"
	// +optional
	AddressType *string `json:"addressType,omitempty"`

	// Description: An optional description of this resource.
	// +optional
	Description *string `json:"description,omitempty"`

	// IPVersion: The IP version that will be used by this address. Valid
	// options are IPV4 or IPV6.
	//
	// Possible values:
	//   "IPV4"
	//   "IPV6"
	//   "UNSPECIFIED_VERSION"
	// +optional
	IPVersion *string `json:"ipVersion,omitempty"`

	// Name of the resource. The name must be 1-63 characters long, and comply
	// with RFC1035. Specifically, the name must be 1-63 characters long and
	// match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?`. The first
	// character must be a lowercase letter, and all following characters
	// (except for the last character) must be a dash, lowercase letter, or
	// digit. The last character must be a lowercase letter or digit.
	Name string `json:"name"`

	// Network: The URL of the network in which to reserve the address. This
	// field can only be used with INTERNAL type with the VPC_PEERING
	// purpose.
	// +optional
	Network *string `json:"network,omitempty"`

	// PrefixLength: The prefix length if the resource represents an IP
	// range.
	// +optional
	PrefixLength *int64 `json:"prefixLength,omitempty"`

	// Purpose: The purpose of this resource, which can be one of the
	// following values:
	// - `GCE_ENDPOINT` for addresses that are used by VM instances, alias
	// IP ranges, internal load balancers, and similar resources.
	// - `DNS_RESOLVER` for a DNS resolver address in a subnetwork
	// - `VPC_PEERING` for addresses that are reserved for VPC peer
	// networks.
	// - `NAT_AUTO` for addresses that are external IP addresses
	// automatically reserved for Cloud NAT.
	//
	// Possible values:
	//   "DNS_RESOLVER"
	//   "GCE_ENDPOINT"
	//   "NAT_AUTO"
	//   "VPC_PEERING"
	// +optional
	Purpose *string `json:"purpose,omitempty"`

	// Subnetwork: The URL of the subnetwork in which to reserve the
	// address. If an IP address is specified, it must be within the
	// subnetwork's IP range. This field can only be used with INTERNAL type
	// with a GCE_ENDPOINT or DNS_RESOLVER purpose.
	// +optional
	Subnetwork *string `json:"subnetwork,omitempty"`
}

// GlobalAddressStatus reflects the state of a GlobalAddress
type GlobalAddressStatus struct {
	v1alpha1.ResourceStatus `json:",inline"`

	// CreationTimestamp in RFC3339 text format.
	CreationTimestamp string `json:"creationTimestamp,omitempty"`

	// ID for the resource. This identifier is defined by the server.
	ID uint64 `json:"id,omitempty"`

	// SelfLink: Server-defined URL for the resource.
	SelfLink string `json:"selfLink,omitempty"`

	// Status of the address, which can be one of RESERVING, RESERVED, or
	// IN_USE. An address that is RESERVING is currently in the process of being
	// reserved. A RESERVED address is currently reserved and available to use.
	// An IN_USE address is currently being used by another resource and is not
	// available.
	//
	// Possible values:
	//   "IN_USE"
	//   "RESERVED"
	//   "RESERVING"
	Status string `json:"status,omitempty"`

	// Users that are using this address.
	Users []string `json:"users,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GlobalAddress is the Schema for the GCP GlobalAddress API
type GlobalAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlobalAddressSpec   `json:"spec,omitempty"`
	Status GlobalAddressStatus `json:"status,omitempty"`
}

// SetBindingPhase of this GlobalAddress.
func (a *GlobalAddress) SetBindingPhase(p v1alpha1.BindingPhase) {
	a.Status.SetBindingPhase(p)
}

// SetConditions of this GlobalAddress.
func (a *GlobalAddress) SetConditions(c ...v1alpha1.Condition) {
	a.Status.SetConditions(c...)
}

// GetBindingPhase of this GlobalAddress.
func (a *GlobalAddress) GetBindingPhase() v1alpha1.BindingPhase {
	return a.Status.GetBindingPhase()
}

// SetClaimReference of this GlobalAddress.
func (a *GlobalAddress) SetClaimReference(r *corev1.ObjectReference) {
	a.Spec.ClaimReference = r
}

// GetClaimReference of this GlobalAddress.
func (a *GlobalAddress) GetClaimReference() *corev1.ObjectReference {
	return a.Spec.ClaimReference
}

// SetClassReference of this GlobalAddress.
func (a *GlobalAddress) SetClassReference(r *corev1.ObjectReference) {
	a.Spec.ClassReference = r
}

// GetClassReference of this GlobalAddress.
func (a *GlobalAddress) GetClassReference() *corev1.ObjectReference {
	return a.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this GlobalAddress.
func (a *GlobalAddress) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	a.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this GlobalAddress.
func (a *GlobalAddress) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return a.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this GlobalAddress.
func (a *GlobalAddress) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	return a.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this GlobalAddress.
func (a *GlobalAddress) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	a.Spec.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// GlobalAddressList contains a list of GlobalAddress
type GlobalAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalAddress `json:"items"`
}
