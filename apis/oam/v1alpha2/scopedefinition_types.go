/*

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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A ScopeDefinitionSpec defines the desired state of a ScopeDefinition.
type ScopeDefinitionSpec struct {
	// Reference to the CustomResourceDefinition that defines this scope kind.
	Reference DefinitionReference `json:"definitionRef"`

	// AllowComponentOverlap specifies whether an OAM component may exist in
	// multiple instances of this kind of scope.
	AllowComponentOverlap bool `json:"allowComponentOverlap"`
}

// ScopeDefinitionStatus defines the observed state of ScopeDefinition
type ScopeDefinitionStatus struct {
	DefinitionStatus DefinitionStatus `json:"definitionStatus"`
}

// +kubebuilder:object:root=true

// ScopeDefinition is the Schema for the scopedefinitions API
type ScopeDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScopeDefinitionSpec   `json:"spec,omitempty"`
	Status ScopeDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScopeDefinitionList contains a list of ScopeDefinition
type ScopeDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScopeDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScopeDefinition{}, &ScopeDefinitionList{})
}
