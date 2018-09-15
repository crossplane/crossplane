/*
Copyright 2018 The Conductor Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ProviderSpec defines the desired state of Provider
type ProviderSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// GCP ServiceAccount json secret key reference
	SecretKey corev1.SecretKeySelector `json:"credentialsSecretRef"`

	// GCP ProjectID (name)
	ProjectID string `json:"projectID"`

	// Permissions  - list of granted CCP permissions this provider's service account expect to have
	Permissions []string `json:"permissions,omitempty"`
}

type ProviderConditionType string

const (
	// Valid means that provider's credentials has been processed and validated
	Valid ProviderConditionType = "Valid"
	// Invalid means that provider's credentials has been processed and deemed invalid
	Invalid ProviderConditionType = "Invalid"
)

// ProviderCondition contains details for the current condition of this pod.
type ProviderCondition struct {
	Type               ProviderConditionType
	Status             corev1.ConditionStatus
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
}

// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
	// Conditions indicate state for particular aspects of a CustomResourceDefinition
	Conditions []ProviderCondition
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Provider is the Schema for the instances API
// +k8s:openapi-gen=true
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProviderList contains a list of Provider
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}
