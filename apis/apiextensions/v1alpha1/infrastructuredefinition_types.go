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

// InfrastructureDefinitionSpec specifies the desired state of the definition.
type InfrastructureDefinitionSpec struct {

	// ConnectionSecretKeys is the list of keys that will be exposed to the end
	// user of the defined kind.
	ConnectionSecretKeys []string `json:"connectionSecretKeys,omitempty"`

	// CRDSpecTemplate is the base CRD template. The final CRD will have additional
	// fields to the base template to accommodate Crossplane machinery.
	CRDSpecTemplate CustomResourceDefinitionSpec `json:"crdSpecTemplate,omitempty"`
}

// InfrastructureDefinitionStatus shows the observed state of the definition.
type InfrastructureDefinitionStatus struct {
	v1alpha1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// InfrastructureDefinition is used to define a resource claim that can be
// scheduled to one of the available compatible compositions.
// +kubebuilder:resource:categories={crossplane}
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type InfrastructureDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureDefinitionSpec   `json:"spec,omitempty"`
	Status InfrastructureDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureDefinitionList contains a list of InfrastructureDefinitions.
type InfrastructureDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureDefinition `json:"items"`
}
