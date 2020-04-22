/*
Copyright 2020 The Crossplane Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// A DefinitionReference refers to a CustomResourceDefinition by name.
type DefinitionReference struct {
	// Name of the referenced CustomResourceDefinition.
	Name string `json:"name"`
}

// A WorkloadDefinitionSpec defines the desired state of a WorkloadDefinition.
type WorkloadDefinitionSpec struct {
	// Reference to the CustomResourceDefinition that defines this workload
	// kind.
	Reference DefinitionReference `json:"definitionRef"`
}

// +kubebuilder:object:root=true

// A WorkloadDefinition registers a kind of Kubernetes custom resource as a
// valid OAM workload kind by referencing its CustomResourceDefinition. The CRD
// is used to validate the schema of the workload when it is embedded in an OAM
// Component.
// +kubebuilder:printcolumn:JSONPath=".spec.definitionRef.name",name=DEFINITION-NAME,type=string
// +kubebuilder:resource:scope=Cluster,categories={crossplane,oam}
type WorkloadDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec WorkloadDefinitionSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadDefinitionList contains a list of WorkloadDefinition.
type WorkloadDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadDefinition `json:"items"`
}

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

// +kubebuilder:object:root=true

// A TraitDefinition registers a kind of Kubernetes custom resource as a valid
// OAM trait kind by referencing its CustomResourceDefinition. The CRD is used
// to validate the schema of the trait when it is embedded in an OAM
// ApplicationConfiguration.
// +kubebuilder:printcolumn:JSONPath=".spec.definitionRef.name",name=DEFINITION-NAME,type=string
// +kubebuilder:resource:scope=Cluster,categories={crossplane,oam}
type TraitDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TraitDefinitionSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// TraitDefinitionList contains a list of TraitDefinition.
type TraitDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TraitDefinition `json:"items"`
}

// A ScopeDefinitionSpec defines the desired state of a ScopeDefinition.
type ScopeDefinitionSpec struct {
	// Reference to the CustomResourceDefinition that defines this scope kind.
	Reference DefinitionReference `json:"definitionRef"`

	// AllowComponentOverlap specifies whether an OAM component may exist in
	// multiple instances of this kind of scope.
	AllowComponentOverlap bool `json:"allowComponentOverlap"`
}

// +kubebuilder:object:root=true

// A ScopeDefinition registers a kind of Kubernetes custom resource as a valid
// OAM scope kind by referencing its CustomResourceDefinition. The CRD is used
// to validate the schema of the scope when it is embedded in an OAM
// ApplicationConfiguration.
// +kubebuilder:printcolumn:JSONPath=".spec.definitionRef.name",name=DEFINITION-NAME,type=string
// +kubebuilder:resource:scope=Cluster,categories={crossplane,oam}
type ScopeDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ScopeDefinitionSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ScopeDefinitionList contains a list of ScopeDefinition.
type ScopeDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScopeDefinition `json:"items"`
}

// A ComponentParameter defines a configurable parameter of a component.
type ComponentParameter struct {
	// Name of this parameter. OAM ApplicationConfigurations will specify
	// parameter values using this name.
	Name string `json:"name"`

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

	// Description of this parameter.
	// +optional
	Description *string `json:"description,omitempty"`
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
	runtimev1alpha1.ConditionedStatus `json:",inline"`

	// TODO(negz): Maintain references to any ApplicationConfigurations that
	// reference this component? Doing so would allow us to queue a reconcile
	// for consuming ApplicationConfigurations when this Component changed.
}

// +kubebuilder:object:root=true

// A Component describes how an OAM workload kind may be instantiated.
// +kubebuilder:resource:categories={crossplane,oam}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".spec.workload.kind",name=WORKLOAD-KIND,type=string
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Component.
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
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
	ScopeReference runtimev1alpha1.TypedReference `json:"scopeRef"`
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

// An ApplicationConfigurationSpec defines the desired state of a
// ApplicationConfiguration.
type ApplicationConfigurationSpec struct {
	// Components of which this ApplicationConfiguration consists. Each
	// component will be used to instantiate a workload.
	Components []ApplicationConfigurationComponent `json:"components"`
}

// A TraitStatus represents the state of a trait.
type TraitStatus string

// A WorkloadTrait represents a trait associated with a workload.
type WorkloadTrait struct {
	// Reference to a trait created by an ApplicationConfiguration.
	Reference runtimev1alpha1.TypedReference `json:"traitRef"`
}

// A WorkloadStatus represents the status of a workload.
type WorkloadStatus struct {
	// ComponentName that produced this workload.
	ComponentName string `json:"componentName,omitempty"`

	// Reference to a workload created by an ApplicationConfiguration.
	Reference runtimev1alpha1.TypedReference `json:"workloadRef,omitempty"`

	// Traits associated with this workload.
	Traits []WorkloadTrait `json:"traits,omitempty"`
}

// An ApplicationConfigurationStatus represents the observed state of a
// ApplicationConfiguration.
type ApplicationConfigurationStatus struct {
	runtimev1alpha1.ConditionedStatus `json:",inline"`

	// Workloads created by this ApplicationConfiguration.
	Workloads []WorkloadStatus `json:"workloads,omitempty"`
}

// +kubebuilder:object:root=true

// An ApplicationConfiguration represents an OAM application.
// +kubebuilder:resource:shortName=appconfig,categories={crossplane,oam}
// +kubebuilder:subresource:status
type ApplicationConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationConfigurationSpec   `json:"spec,omitempty"`
	Status ApplicationConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationConfigurationList contains a list of ApplicationConfiguration.
type ApplicationConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationConfiguration `json:"items"`
}
