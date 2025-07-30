/*
Copyright 2025 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// ManagedResourceDefinitionSpec specifies the desired state of the resource definition.
type ManagedResourceDefinitionSpec struct {
	// Inline the minor fork of upstream's CustomResourceDefinitionSpec.
	CustomResourceDefinitionSpec `json:",inline"`

	// ConnectionDetails is an array of connection detail keys and descriptions.
	ConnectionDetails []ConnectionDetail `json:"connectionDetails,omitempty"`

	// State toggles whether the underlying CRD is created or not.
	// +kubebuilder:validation:Enum=Active;Inactive
	// +kubebuilder:default=Inactive
	// +kubebuilder:validation:XValidation:rule="self == oldSelf || oldSelf != 'Active'",message="state cannot be changed once it becomes Active"
	State ManagedResourceDefinitionState `json:"state,omitempty"`
}

// ManagedResourceDefinitionState is the state of the resource definition.
type ManagedResourceDefinitionState string

const (
	// ManagedResourceDefinitionActive is an active resource definition.
	ManagedResourceDefinitionActive ManagedResourceDefinitionState = "Active"

	// ManagedResourceDefinitionInactive is an inactive resource definition.
	ManagedResourceDefinitionInactive ManagedResourceDefinitionState = "Inactive"
)

// IsActive returns if this ManagedResourceDefinitionState is "Active".
func (s ManagedResourceDefinitionState) IsActive() bool {
	return s == ManagedResourceDefinitionActive
}

// ConnectionDetail holds keys and descriptions of connection secrets.
type ConnectionDetail struct {
	// Name of the key.
	Name string `json:"name"`
	// Description of how the key is used.
	Description string `json:"description"`
}

// ManagedResourceDefinitionStatus shows the observed state of the resource definition.
type ManagedResourceDefinitionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// A ManagedResourceDefinition defines the schema for a new custom Kubernetes API.
//
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".spec.state"
// +kubebuilder:printcolumn:name="ESTABLISHED",type="string",JSONPath=".status.conditions[?(@.type=='Established')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=mrd;mrds
type ManagedResourceDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedResourceDefinitionSpec   `json:"spec,omitempty"`
	Status ManagedResourceDefinitionStatus `json:"status,omitempty"`
}

// GetCondition of this ManagedResourceDefinition.
func (p *ManagedResourceDefinition) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return p.Status.GetCondition(ct)
}

// SetConditions of this ManagedResourceDefinition.
func (p *ManagedResourceDefinition) SetConditions(c ...xpv1.Condition) {
	p.Status.SetConditions(c...)
}

// +kubebuilder:object:root=true

// ManagedResourceDefinitionList contains a list of ManagedResourceDefinitions.
type ManagedResourceDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ManagedResourceDefinition `json:"items"`
}
