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

// A TraitDefinitionSpec defines the desired state of a TraitDefinition.
type TraitDefinitionSpec struct {
	// Reference to the CustomResourceDefinition that defines this trait kind.
	Reference DefinitionReference `json:"definitionRef"`

	// AppliesToWorkloads specifies the list of workload kinds this trait
	// applies to. Workload kinds are specified in kind.group/version format,
	// e.g. server.core.oam.dev/v1alpha2. Traits that omit this field apply to
	// all workload kinds.
	// +optional
	AppliesToWorkloads []string `json:"appliesToWorkloads,omitempty"`
}

// TraitDefinitionStatus defines the observed state of TraitDefinition
type TraitDefinitionStatus struct {
	DefinitionStatus DefinitionStatus `json:"definitionStatus"`
}

// +kubebuilder:object:root=true

// TraitDefinition is the Schema for the traitdefinitions API
type TraitDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TraitDefinitionSpec   `json:"spec,omitempty"`
	Status TraitDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TraitDefinitionList contains a list of TraitDefinition
type TraitDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TraitDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TraitDefinition{}, &TraitDefinitionList{})
}
