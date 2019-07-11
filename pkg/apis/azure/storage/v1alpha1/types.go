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
	"github.com/Azure/azure-storage-blob-go/azblob"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// AccountSpec is the schema for Account object
type AccountSpec struct {
	corev1alpha1.ResourceSpec `json:",inline"`

	// ResourceGroupName azure group name
	ResourceGroupName string `json:"resourceGroupName"`

	// StorageAccountName for azure blob storage
	// +kubebuilder:validation:MaxLength=24
	StorageAccountName string `json:"storageAccountName"`

	// StorageAccountSpec the parameters used when creating a storage account.
	StorageAccountSpec *StorageAccountSpec `json:"storageAccountSpec"`
}

// AccountStatus defines the observed state of StorageAccountStatus
type AccountStatus struct {
	corev1alpha1.ResourceStatus `json:",inline"`

	*StorageAccountStatus `json:"accountStatus,inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Account is the Schema for the Account API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="RESOURCE_GROUP",type="string",JSONPath=".spec.resourceGroupName"
// +kubebuilder:printcolumn:name="ACCOUNT_NAME",type="string",JSONPath=".spec.storageAccountName"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RECLAIM_POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Account struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AccountSpec   `json:"spec,omitempty"`
	Status            AccountStatus `json:"status,omitempty"`
}

// SetBindingPhase of this Account.
func (a *Account) SetBindingPhase(p corev1alpha1.BindingPhase) {
	a.Status.SetBindingPhase(p)
}

// GetBindingPhase of this Account.
func (a *Account) GetBindingPhase() corev1alpha1.BindingPhase {
	return a.Status.GetBindingPhase()
}

// SetClaimReference of this Account.
func (a *Account) SetClaimReference(r *corev1.ObjectReference) {
	a.Spec.ClaimReference = r
}

// GetClaimReference of this Account.
func (a *Account) GetClaimReference() *corev1.ObjectReference {
	return a.Spec.ClaimReference
}

// SetClassReference of this Account.
func (a *Account) SetClassReference(r *corev1.ObjectReference) {
	a.Spec.ClassReference = r
}

// GetClassReference of this Account.
func (a *Account) GetClassReference() *corev1.ObjectReference {
	return a.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this Account.
func (a *Account) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	a.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this Account.
func (a *Account) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return a.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this Account.
func (a *Account) GetReclaimPolicy() corev1alpha1.ReclaimPolicy {
	return a.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this Account.
func (a *Account) SetReclaimPolicy(p corev1alpha1.ReclaimPolicy) {
	a.Spec.ReclaimPolicy = p
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccountList contains a list of AzureBuckets
type AccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Account `json:"items"`
}

// ParseAccountSpec from properties map key/values
func ParseAccountSpec(p map[string]string) *AccountSpec {
	return &AccountSpec{
		ResourceSpec: corev1alpha1.ResourceSpec{
			ReclaimPolicy: corev1alpha1.ReclaimRetain,
		},
		ResourceGroupName:  p["resourceGroupName"],
		StorageAccountName: p["storageAccountName"],
		StorageAccountSpec: parseStorageAccountSpec(p["storageAccountSpec"]),
	}
}

// ContainerSpec is the schema for ContainerSpec object
type ContainerSpec struct {
	// NameFormat to format container name passing it a object UID
	// If not provided, defaults to "%s", i.e. UID value
	NameFormat string `json:"nameFormat,omitempty"`

	// Container metadata
	Metadata azblob.Metadata `json:"metadata,omitempty"`

	// PublicAccessType
	PublicAccessType azblob.PublicAccessType `json:"publicAccessType,omitempty"`

	// AccountReference to azure storage account object
	AccountReference corev1.LocalObjectReference `json:"accountReference"`

	// NOTE(negz): Container is the only Crossplane type that does not use a
	// Provider (it reads credentials from its associated Account instead). This
	// means we can't embed a corev1alpha1.ResourceSpec, as doing so would
	// require a redundant providerRef be specified. Instead we duplicate
	// most of that struct here; the below values should be kept in sync with
	// corev1alpha1.ResourceSpec.

	WriteConnectionSecretToReference corev1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
	ClaimReference                   *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassReference                   *corev1.ObjectReference     `json:"classRef,omitempty"`
	ReclaimPolicy                    corev1alpha1.ReclaimPolicy  `json:"reclaimPolicy,omitempty"`
}

// ContainerStatus sub-resource for Container object
type ContainerStatus struct {
	corev1alpha1.ResourceStatus `json:",inline"`

	Name string `json:"name,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Container is the Schema for the Container
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

// SetBindingPhase of this Container.
func (c *Container) SetBindingPhase(p corev1alpha1.BindingPhase) {
	c.Status.SetBindingPhase(p)
}

// GetBindingPhase of this Container.
func (c *Container) GetBindingPhase() corev1alpha1.BindingPhase {
	return c.Status.GetBindingPhase()
}

// SetClaimReference of this Container.
func (c *Container) SetClaimReference(r *corev1.ObjectReference) {
	c.Spec.ClaimReference = r
}

// GetClaimReference of this Container.
func (c *Container) GetClaimReference() *corev1.ObjectReference {
	return c.Spec.ClaimReference
}

// SetClassReference of this Container.
func (c *Container) SetClassReference(r *corev1.ObjectReference) {
	c.Spec.ClassReference = r
}

// GetClassReference of this Container.
func (c *Container) GetClassReference() *corev1.ObjectReference {
	return c.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this Container.
func (c *Container) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	c.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this Container.
func (c *Container) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return c.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this Container.
func (c *Container) GetReclaimPolicy() corev1alpha1.ReclaimPolicy {
	return c.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this Container.
func (c *Container) SetReclaimPolicy(p corev1alpha1.ReclaimPolicy) {
	c.Spec.ReclaimPolicy = p
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerList - list of the container objects
type ContainerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Container `json:"items"`
}

// GetContainerName based on the NameFormat spec value,
// If name format is not provided, container name defaults to UID
// If name format provided with '%s' value, container name will result in formatted string + UID,
//   NOTE: only single %s substitution is supported
// If name format does not contain '%s' substitution, i.e. a constant string, the
// constant string value is returned back
//
// Examples:
//   For all examples assume "UID" = "test-uid"
//   1. NameFormat = "", ContainerName = "test-uid"
//   2. NameFormat = "%s", ContainerName = "test-uid"
//   3. NameFormat = "foo", ContainerName = "foo"
//   4. NameFormat = "foo-%s", ContainerName = "foo-test-uid"
//   5. NameFormat = "foo-%s-bar-%s", ContainerName = "foo-test-uid-bar-%!s(MISSING)"
func (c *Container) GetContainerName() string {
	return util.ConditionalStringFormat(c.Spec.NameFormat, string(c.GetUID()))
}

// ParseContainerSpec from properties map key/values
func ParseContainerSpec(p map[string]string) *ContainerSpec {
	return &ContainerSpec{
		ReclaimPolicy:    corev1alpha1.ReclaimRetain,
		Metadata:         util.ParseMap(p["metadata"]),
		NameFormat:       p["nameFormat"],
		PublicAccessType: parsePublicAccessType(p["publicAccessType"]),
	}
}

func parsePublicAccessType(s string) azblob.PublicAccessType {
	if s == "" {
		return azblob.PublicAccessNone
	}
	return azblob.PublicAccessType(s)
}
