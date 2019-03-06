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

// GCPBucketSpec defines the desired state of GCPBucket
type GCPBucketSpec struct {
	Name string `json:"name,omitempty"`

	// Location - See authoritative list https://developers.google.com/storage/docs/bucket-locations
	// Which you use is dependent on whether it's multi_region or not.
	Location string `json:"location,omitempty"`

	// PredefinedACL
	// One of: private, authenticatedRead, projectPrivate, publicRead, publicReadWrite
	PredefinedACL *string `json:"predefinedACL,omitempty"`

	//StorageClass one of
	// MULTI_REGIONAL, REGIONAL, STANDARD, NEARLINE, COLDLINE, and DURABLE_REDUCED_AVAILABILITY.
	// If this value is not specified when the bucket is created, it will default to STANDARD
	StorageClass                 *string                 `json:"storageClass,omitempty"`
	Versioning                   bool                    `json:"versioning,omitempty"`
	ConnectionSecretNameOverride string                  `json:"connectionSecretNameOverride,omitempty"`
	ProviderRef                  v1.LocalObjectReference `json:"providerRef"`
	ClaimRef                     *v1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef                     *v1.ObjectReference     `json:"classRef,omitempty"`
}

// GCPBucketStatus defines the observed state of GoogleBucket
type GCPBucketStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	Message             string                  `json:"message,omitempty"`
	ProviderID          string                  `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCPBucket is the Schema for the GCPBucket API
// +k8s:openapi-gen=true
// +groupName=storage.gcp
type GCPBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPBucketSpec   `json:"spec,omitempty"`
	Status GCPBucketStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCPBucketList contains a list of GCPBuckets
type GCPBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPBucket `json:"items"`
}
