/*
Copyright 2018 The Crossplane Authors.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AzureBucketSpec defines the desired state of AzureBucket
type AzureBucketSpec struct {
	Name     string `json:"name,omitempty"`
	Location string `json:"location,omitempty"`
	// Storage
	// One of:
	// Locally redundant storage (LRS)
	// Geo-redundant storage (GRS)
	// Read-access geo-redundant storage (RA-GRS)
	Storage string `json:"storage,omitempty"`
	// AccessTier hot or cold
	AccessTier string `json:"accessTier,omitempty"`
	// PredefinedACL
	// One of: private, full-public-read, public-read-blobs-only
	PredefinedACL                string                  `json:"predefinedACL,omitempty"`
	ConnectionSecretNameOverride string                  `json:"connectionSecretNameOverride,omitempty"`
	ProviderRef                  v1.LocalObjectReference `json:"providerRef"`
}

// AzureBucketStatus defines the observed state of AzureBucket
type AzureBucketStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	Message             string                  `json:"message,omitempty"`
	ProviderID          string                  `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureBucket is the Schema for the Bucket API
// +k8s:openapi-gen=true
// +groupName=storage.azure
type AzureBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AzureBucketSpec   `json:"spec,omitempty"`
	Status            AzureBucketStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureBucketList contains a list of AzureBuckets
type AzureBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureBucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureBucket{}, &AzureBucketList{})
}
