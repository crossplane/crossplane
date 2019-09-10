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

// IPRange describes an IPv4 range.
type IPRange struct {
	// The IPv4 CIDR range. You can either specify a CIDR range or a source security
	// group, not both. To specify a single IPv4 address, use the /32 prefix length.
	CIDRIP string `json:"cidrIp"`

	// A description for the ip range
	Description string `json:"description,omitempty"`
}

// IPPermission Describes a set of permissions for a security group rule.
type IPPermission struct {
	// The start of port range for the TCP and UDP protocols, or an ICMP/ICMPv6
	// type number. A value of -1 indicates all ICMP/ICMPv6 types. If you specify
	// all ICMP/ICMPv6 types, you must specify all codes.
	FromPort int64 `json:"fromPort"`

	// The end of port range for the TCP and UDP protocols, or an ICMP/ICMPv6 code.
	// A value of -1 indicates all ICMP/ICMPv6 codes for the specified ICMP type.
	// If you specify all ICMP/ICMPv6 types, you must specify all codes.
	ToPort int64 `json:"toPort"`

	// The IP protocol name (tcp, udp, icmp) or number (see Protocol Numbers (http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml)).
	//
	// [EC2-VPC only] Use -1 to specify all protocols. When authorizing security
	// group rules, specifying -1 or a protocol number other than tcp, udp, icmp,
	// or 58 (ICMPv6) allows traffic on all ports, regardless of any port range
	// you specify. For tcp, udp, and icmp, you must specify a port range. For 58
	// (ICMPv6), you can optionally specify a port range; if you don't, traffic
	// for all types and codes is allowed when authorizing rules.
	IPProtocol string `json:"protocol"`

	// One or more IPv4 ranges.
	CIDRBlocks []IPRange `json:"cidrBlocks,omitempty"`
}

// SecurityGroupParameters defines the desired state of a SecurityGroup
type SecurityGroupParameters struct {
	// VPCID is the ID of the VPC.
	VPCID string `json:"vpcId,omitempty"`

	// A description of the security group.
	Description string `json:"description"`

	// The name of the security group.
	GroupName string `json:"groupName"`

	// One or more inbound rules associated with the security group.
	IngressPermissions []IPPermission `json:"ingress,omitempty"`

	// [EC2-VPC] One or more outbound rules associated with the security group.
	EgressPermissions []IPPermission `json:"egress,omitempty"`
}

// SecurityGroupSpec defines the desired state of a SecurityGroup
type SecurityGroupSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	SecurityGroupParameters      `json:",inline"`
}

// SecurityGroupExternalStatus keeps the state for the external resource
type SecurityGroupExternalStatus struct {
	// SecurityGroupID is the ID of the SecurityGroup.
	SecurityGroupID string `json:"securityGroupID"`

	// Tags represents to current ec2 tags.
	Tags []Tag `json:"tags,omitempty"`
}

// SecurityGroupStatus defines the observed state of an SecurityGroup
type SecurityGroupStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
	SecurityGroupExternalStatus    `json:",inline"`
}

// +kubebuilder:object:root=true

// SecurityGroup is the Schema for the SecurityGroup API
// +kubebuilder:printcolumn:name="GROUPNAME",type="string",JSONPath=".spec.groupName"
// +kubebuilder:printcolumn:name="VPCID",type="string",JSONPath=".spec.vpcId"
// +kubebuilder:printcolumn:name="DESCRIPTION",type="string",JSONPath=".spec.description"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type SecurityGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupSpec   `json:"spec,omitempty"`
	Status SecurityGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecurityGroupList contains a list of SecurityGroups
type SecurityGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroup `json:"items"`
}
