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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane/apis/core/v1alpha1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ProviderSpec defines the desired state of Provider
type ProviderSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// Azure service principal credentials json secret key reference
	Secret corev1.SecretKeySelector `json:"credentialsSecretRef"`
}

// +kubebuilder:object:root=true

// Provider is the Schema for the instances API
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentialsSecretRef.name",priority=1
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderList contains a list of Provider
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}

// ResourceGroupSpec defines the desired state of Resource Group
type ResourceGroupSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file
	v1alpha1.ResourceSpec `json:",inline"`

	// Name of the resource group
	Name string `json:"name,omitempty"`
	// See official list of valid regions - https://azure.microsoft.com/en-us/global-infrastructure/regions/
	Location string `json:"location,omitempty"`
}

// ResourceGroupStatus is the status for this resource group
type ResourceGroupStatus struct {
	v1alpha1.ResourceStatus `json:",inline"`

	Name string `json:"name"`
}

// +kubebuilder:object:root=true

// ResourceGroup is the Schema for the instances API
type ResourceGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceGroupSpec   `json:"spec,omitempty"`
	Status ResourceGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceGroupList contains a list of Resource Groups
type ResourceGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceGroup `json:"items"`
}
