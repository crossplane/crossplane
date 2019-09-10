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

// IAMRoleParameters defines the desired state of an IAM role
type IAMRoleParameters struct {

	// AssumeRolePolicyDocument is the the trust relationship policy document
	// that grants an entity permission to assume the role.
	AssumeRolePolicyDocument string `json:"assumeRolePolicyDocument"`

	// Description is a description of the role.
	// +optional
	Description string `json:"description,omitempty"`

	// RoleName presents the name of the IAM role
	RoleName string `json:"roleName"`
}

// IAMRoleSpec defines the desired state of a IAM role
type IAMRoleSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	IAMRoleParameters            `json:",inline"`
}

// IAMRoleExternalStatus keeps the state for the external resource
type IAMRoleExternalStatus struct {
	// ARN is the Amazon Resource Name (ARN) specifying the role. For more information
	// about ARNs and how to use them in policies, see IAM Identifiers (http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html)
	// in the IAM User Guide guide.
	ARN string `json:"arn"`

	// RoleID is the stable and unique string identifying the role. For more information about
	// IDs, see IAM Identifiers (http://docs.aws.amazon.com/IAM/latest/UserGuide/Using_Identifiers.html)
	// in the Using IAM guide.
	RoleID string `json:"roleID"`
}

// IAMRoleStatus defines the observed state of an IAM role
type IAMRoleStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`

	IAMRoleExternalStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// IAMRole is the Schema for the IAM role API
// +kubebuilder:printcolumn:name="ROLENAME",type="string",JSONPath=".spec.roleName"
// +kubebuilder:printcolumn:name="DESCRIPTION",type="string",JSONPath=".spec.description"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type IAMRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IAMRoleSpec   `json:"spec,omitempty"`
	Status IAMRoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IAMRoleList contains a list of IAMRoles
type IAMRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMRole `json:"items"`
}
