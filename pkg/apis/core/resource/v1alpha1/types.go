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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&Class{}, &ClassList{})
	SchemeBuilder.Register(&Claim{}, &ClaimList{})
}

// ClassSpec defines the desired state of Class
type ClassSpec struct {
	metav1.TypeMeta `json:",inline"`
}

// ClassStatus defines the observed state of Class
type ClassStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Class is the Schema for the templates API
// +k8s:openapi-gen=true
type Class struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClassSpec   `json:"spec,omitempty"`
	Status ClassStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClassList contains a list of Class
type ClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Class `json:"items"`
}

// ClaimSpec defines the desired state of Claim
type ClaimSpec struct {
	ClassRef v1.LocalObjectReference `json:"classRef"`
}

// ClaimStatus defines the observed state of Claim
type ClaimStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Claim is the Schema for the templates API
// +k8s:openapi-gen=true
type Claim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClaimSpec   `json:"spec,omitempty"`
	Status ClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClaimList contains a list of Claim
type ClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Claim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Claim{}, &ClaimList{})
}
