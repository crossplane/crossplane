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
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BlobContainerSpec defines the desired state of S3Bucket
type BlobContainerSpec struct {
	Name     string `json:"name,omitempty"`
	Location string `json:"location,omitempty"`
	// Storage - one of
	// Locally redundant storage (LRS)
	// Geo-redundant storage (GRS)
	// Read-access geo-redundant storage (RA-GRS)
	Storage string `json:"location,omitempty"`
	// AccessTier hot or cold
	AccessTier string `json:"accessTier,omitempty"`
	// PredefinedACL
	// One of: private, full-public-read, public-read-blobs-only
	PredefinedACL string                  `json:"predefinedACL,omitempty"`
	ProviderRef   v1.LocalObjectReference `json:"providerRef"`
}

// BlobContainerStatus defines the observed state of BlobContainer
type BlobContainerStatus struct {
	corev1alpha1.ConditionedStatus
	Message    string `json:"message,omitempty"`
	ProviderID string `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BlobContainer is the Schema for the Container API
// +k8s:openapi-gen=true
type BlobContainer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              BlobContainerSpec   `json:"spec,omitempty"`
	Status            BlobContainerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BlobContainerList contains a list of BlobContainers
type BlobContainerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlobContainer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlobContainer{}, &BlobContainerList{})
}
