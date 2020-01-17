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
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StackConfigurationSpec defines the desired state of StackConfiguration
type StackConfigurationSpec struct {
	// Important: Run "make" to regenerate code after modifying this file
	Behaviors StackConfigurationBehaviors `json:"behaviors,omitempty"`
}

// ResourceEngineConfiguration represents a configuration for a resource engine, such as helm2 or kustomize.
type ResourceEngineConfiguration struct {
	Type string `json:"type"`
}

// StackConfigurationSource is the stack image which this stack configuration is from.
// In the future, other source types may be supported, such as a URL.
type StackConfigurationSource struct {
	// a container image id
	Image string `json:"image,omitempty"`
}

// StackConfigurationBehaviors specifies behaviors for the stack
type StackConfigurationBehaviors struct {
	CRDs   map[GVK]StackConfigurationBehavior `json:"crds,omitempty"`
	Engine ResourceEngineConfiguration        `json:"engine,omitempty"`
	// Theoretically, source and engine could be specified at a per-crd level or
	// per-hook level as well.
	Source StackConfigurationSource `json:"source,omitempty"`
}

// GVK should be in domain format, so Kind.group/version
type GVK string

// StackConfigurationBehavior specifies an individual behavior, by listing resources
// which should be processed.
type StackConfigurationBehavior struct {
	// The key for Hooks is an event name which represents the lifecycle event that the controller should respond to.
	// There are certain events that are recognized.
	// Currently, only "reoncile" is recognized.
	Hooks  map[string]HookConfigurations `json:"hooks"`
	Engine ResourceEngineConfiguration   `json:"engine,omitempty"`
}

// HookConfigurations is a list of hook configurations.
type HookConfigurations []HookConfiguration

// HookConfiguration is the configuration for an individual hook which will be
// executed in response to an event.
type HookConfiguration struct {
	Engine    ResourceEngineConfiguration `json:"engine,omitempty"`
	Directory string                      `json:"directory"`
}

// StackConfigurationStatus defines the observed state of StackConfiguration
type StackConfigurationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// StackConfiguration is the Schema for the stackconfigurations API
type StackConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackConfigurationSpec   `json:"spec,omitempty"`
	Status StackConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StackConfigurationList contains a list of StackConfiguration
type StackConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StackConfiguration `json:"items"`
}
