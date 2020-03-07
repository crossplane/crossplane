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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// An ApplicationConfigurationSpec defines the desired state of a
// ApplicationConfiguration.
type ApplicationConfigurationSpec struct {
	// Components of which this ApplicationConfiguration consists. Each
	// component will be used to instantiate a workload.
	Components []ApplicationConfigurationComponent `json:"components"`
}

// A ComponentParameterValue specifies a value for a named parameter. The
// associated component must publish a parameter with this name.
type ComponentParameterValue struct {
	// Name of the component parameter to set.
	Name string `json:"name"`

	// Value to set.
	Value intstr.IntOrString `json:"value"`
}

// A ComponentTrait specifies a trait that should be applied to a component.
type ComponentTrait struct {
	// A Trait that will be created for the component
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Trait runtime.RawExtension `json:"trait"`
}

// A ComponentScope specifies a scope in which a component should exist.
type ComponentScope struct {
	// A ScopeReference must refer to an OAM scope resource.
	ScopeReference OAMReference `json:"scopeRef"`
}

// An ApplicationConfigurationComponent specifies a component of an
// ApplicationConfiguration. Each component is used to instantiate a workload.
type ApplicationConfigurationComponent struct {
	// ComponentName specifies a component of which an ApplicationConfiguration
	// should consist. The named component must exist.
	ComponentName string `json:"componentName"`

	// ParameterValues specify values for the the specified component's
	// parameters. Any parameter required by the component must be specified.
	// +optional
	ParameterValues []ComponentParameterValue `json:"parameterValues,omitempty"`

	// Traits of the specified component.
	// +optional
	Traits []ComponentTrait `json:"traits,omitempty"`

	// Scopes in which the specified component should exist.
	// +optional
	Scopes []ComponentScope `json:"scopes,omitempty"`
}

// A WorkloadTrait represents a trait associated with a workload.
type WorkloadTrait struct {
	// Reference to a trait created by an ApplicationConfiguration.
	Reference OAMReference `json:"traitRef"`

	// Status of this trait.
	// +kubebuilder:validation:Enum=Unknown;Error;Created
	Status TraitStatus `json:"status,omitempty"`
}

// An ApplicationConfigurationWorkload represents a workload associated with an
// ApplicationConfiguration.
type ApplicationConfigurationWorkload struct {
	// Reference to a workload created by an ApplicationConfiguration.
	Reference OAMReference `json:"workloadRef,omitempty"`

	// Traits associated with this workload.
	Traits []WorkloadTrait `json:"traits,omitempty"`

	// Status of this workload.
	// +kubebuilder:validation:Enum=Unknown;Error;Created
	Status WorkloadStatus `json:"status,omitempty"`
}

// An ApplicationConfigurationStatus represents the observed state of a
// ApplicationConfiguration.
type ApplicationConfigurationStatus struct {
	cprt.ConditionedStatus `json:",inline"`

	// Workloads created by this ApplicationConfiguration.
	Workloads []ApplicationConfigurationWorkload `json:"workloads,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationConfiguration is the Schema for the applicationconfigurations API
type ApplicationConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationConfigurationSpec   `json:"spec,omitempty"`
	Status ApplicationConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationConfigurationList contains a list of ApplicationConfiguration
type ApplicationConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationConfiguration{}, &ApplicationConfigurationList{})
}
