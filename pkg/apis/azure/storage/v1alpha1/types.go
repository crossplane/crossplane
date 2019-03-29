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
	"github.com/Azure/azure-storage-blob-go/azblob"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// AccountSpec defines the desired state of StorageAccountStatus
type AccountSpec struct {
	// StorageAccountName for azure blob storage
	StorageAccountName string `json:"storageAccountName"`

	// StorageAccountSpec the parameters used when creating a storage account.
	StorageAccountSpec *StorageAccountSpec `json:"storageAccountSpec"`

	// ConnectionSecretNameOverride to generate connection secret with specific name
	ConnectionSecretNameOverride string `json:"connectionSecretNameOverride,omitempty"`

	ProviderRef corev1.LocalObjectReference `json:"providerRef"`
	ClaimRef    *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef    *corev1.ObjectReference     `json:"classRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// AccountStatus defines the observed state of StorageAccountStatus
type AccountStatus struct {
	AccountProperties StorageAccountStatusProperties `json:"properties"`

	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	ConnectionSecretRef corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Account is the Schema for the Account API
// +k8s:openapi-gen=true
// +groupName=storage.azure
type Account struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AccountSpec   `json:"spec,omitempty"`
	Status            AccountStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccountList contains a list of AzureBuckets
type AccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Account `json:"items"`
}

// ContainerSpec is the schema for ContainerSpec object
type ContainerSpec struct {
	// NameFormat to format container name passing it a object UID
	// If not provided, defaults to "%s", i.e. UID value
	NameFormat string `json:"nameFormat,omitempty"`

	PublicAccessType azblob.PublicAccessType `json:"publicAccessType,omitempty"`

	AccountRef corev1.LocalObjectReference `json:"accountRef"`

	// ConnectionSecretNameOverride to generate connection secret with specific name
	ConnectionSecretNameOverride string `json:"connectionSecretNameOverride,omitempty"`

	ProviderRef corev1.LocalObjectReference `json:"providerRef"`
	ClaimRef    *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef    *corev1.ObjectReference     `json:"classRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// ContainerStatus sub-resource for Container object
type ContainerStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	ConnectionSecretRef corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
	Name                string                      `json:"name,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Container is the Schema for the Container
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STORAGE_ACCOUNT",type="string",JSONPath=".spec.accountRef.name"
// +kubebuilder:printcolumn:name="PUBLIC_ACCESS_TYPE",type="string",JSONPath=".spec.publicAccessType"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RECLAIM_POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Container struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ContainerSpec   `json:"spec,omitempty"`
	Status            ContainerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerList - list of the container objects
type ContainerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Container `json:"items"`
}
