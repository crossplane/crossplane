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

// A WorkloadDefinitionSpec defines the desired state of a WorkloadDefinition.
type WorkloadDefinitionSpec struct {
	// Reference to the CustomResourceDefinition that defines this workload kind.
	DefinitionRef DefinitionReference `json:"definitionRef"`
}

// WorkloadDefinitionStatus defines the observed state of WorkloadDefinition
type WorkloadDefinitionStatus struct {
	DefinitionStatus DefinitionStatus `json:"definitionStatus"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:JSONPath=".spec.definitionRef.name",name=DEFINITION-NAME,type=string

// WorkloadDefinition is the Schema for the workloadDefinitions API
type WorkloadDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadDefinitionSpec   `json:"spec,omitempty"`
	Status WorkloadDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadDefinitionList contains a list of WorkloadDefinition
type WorkloadDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkloadDefinition{}, &WorkloadDefinitionList{})
}
