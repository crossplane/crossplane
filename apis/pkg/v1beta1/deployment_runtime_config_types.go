/*
Copyright 2023 The Crossplane Authors.

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

package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectMeta is metadata contains the configurable metadata fields for the
// runtime objects.
type ObjectMeta struct {
	// Name is the name of the object.
	// +optional
	Name *string `json:"name,omitempty"`
	// Annotations is an unstructured key value map stored with a resource that
	// may be set by external tools to store and retrieve arbitrary metadata.
	// They are not queryable and should be preserved when modifying objects.
	// More info: http:https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. Labels will be merged with internal labels
	// used by crossplane, and labels with a crossplane.io key might be
	// overwritten.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// DeploymentTemplate is the template for the Deployment object.
type DeploymentTemplate struct {
	// Metadata contains the configurable metadata fields for the Deployment.
	// +optional
	Metadata *ObjectMeta `json:"metadata,omitempty"`

	// Spec contains the configurable spec fields for the Deployment object.
	// +optional
	Spec *appsv1.DeploymentSpec `json:"spec,omitempty"`
}

// ServiceTemplate is the template for the Service object.
type ServiceTemplate struct {
	// Metadata contains the configurable metadata fields for the Service.
	// +optional
	Metadata *ObjectMeta `json:"metadata,omitempty"`
}

// ServiceAccountTemplate is the template for the ServiceAccount object.
type ServiceAccountTemplate struct {
	// Metadata contains the configurable metadata fields for the ServiceAccount.
	// +optional
	Metadata *ObjectMeta `json:"metadata,omitempty"`
}

// DeploymentRuntimeConfigSpec specifies the configuration for a packaged controller.
// Values provided will override package manager defaults. Labels and
// annotations are passed to both the controller Deployment and ServiceAccount.
type DeploymentRuntimeConfigSpec struct {
	// DeploymentTemplate is the template for the Deployment object.
	// +optional
	DeploymentTemplate *DeploymentTemplate `json:"deploymentTemplate,omitempty"`
	// ServiceTemplate is the template for the Service object.
	// +optional
	ServiceTemplate *ServiceTemplate `json:"serviceTemplate,omitempty"`
	// ServiceAccountTemplate is the template for the ServiceAccount object.
	// +optional
	ServiceAccountTemplate *ServiceAccountTemplate `json:"serviceAccountTemplate,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// The DeploymentRuntimeConfig provides settings for the Kubernetes Deployment
// of a Provider or composition function package.
//
// Read the Crossplane documentation for
// [more information about DeploymentRuntimeConfigs](https://docs.crossplane.io/latest/concepts/providers/#runtime-configuration).
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane}
type DeploymentRuntimeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DeploymentRuntimeConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// DeploymentRuntimeConfigList contains a list of DeploymentRuntimeConfig.
type DeploymentRuntimeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentRuntimeConfig `json:"items"`
}
