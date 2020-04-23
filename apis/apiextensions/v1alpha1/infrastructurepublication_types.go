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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// InfrastructurePublicationSpec specifies the desired state of the definition.
type InfrastructurePublicationSpec struct {
	// InfrastructureDefinitionReference references the InfrastructureDefinition
	// that should be published.
	InfrastructureDefinitionReference v1alpha1.Reference `json:"infrastructureDefinitionRef"`
}

// InfrastructurePublicationStatus shows the observed state of the definition.
type InfrastructurePublicationStatus struct {
	v1alpha1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// An InfrastructurePublication publishes a defined kind of composite
// infrastructure resource. Published infrastructure resources may be bound to
// an application via an infrastructure requirement.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane
type InfrastructurePublication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructurePublicationSpec   `json:"spec,omitempty"`
	Status InfrastructurePublicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructurePublicationList contains a list of InfrastructurePublications.
type InfrastructurePublicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructurePublication `json:"items"`
}
