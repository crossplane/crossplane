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
	cprt "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ComponentParameter struct {
	// Name of this parameter. OAM ApplicationConfigurations will specify
	// parameter values using this name.
	Name string `json:"name"`

	// Description of this parameter.
	Description string `json:"description"`

	// FieldPaths specifies an array of fields within this Component's workload
	// that will be overwritten by the value of this parameter. The type of the
	// parameter (e.g. int, string) is inferred from the type of these fields;
	// All fields must be of the same type. Fields are specified as JSON field
	// paths without a leading dot, for example 'spec.replicas'.
	FieldPaths []string `json:"fieldPaths"`

	// TODO(negz): Use +kubebuilder:default marker to default Required to false
	// once we're generating v1 CRDs.

	// Required specifies whether or not a value for this parameter must be
	// supplied when authoring an ApplicationConfiguration.
	// +optional
	Required *bool `json:"required,omitempty"`
}

// A ComponentSpec defines the desired state of a Component.
type ComponentSpec struct {
	// A Workload that will be created for each ApplicationConfiguration that
	// includes this Component. Workloads must be defined by a
	// WorkloadDefinition.
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Workload runtime.RawExtension `json:"workload"`

	// Parameters exposed by this component. ApplicationConfigurations that
	// reference this component may specify values for these parameters, which
	// will in turn be injected into the embedded workload.
	// +optional
	Parameters []ComponentParameter `json:"parameters,omitempty"`
}

// A ComponentStatus represents the observed state of a Component.
type ComponentStatus struct {
	cprt.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// Component is the Schema for the components API
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ComponentSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
