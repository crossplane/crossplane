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

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

// A LocalPermissionType is a type of permission that may be granted to a
// Bucket.
type LocalPermissionType string

const (
	// ReadOnlyPermission will grant read objects in a bucket
	ReadOnlyPermission LocalPermissionType = "Read"
	// WriteOnlyPermission will grant write/delete objects in a bucket
	WriteOnlyPermission LocalPermissionType = "Write"
	// ReadWritePermission LocalPermissionType Grant both read and write permissions
	ReadWritePermission LocalPermissionType = "ReadWrite"
)

// A PredefinedACL is a predefined ACL that may be applied to a Bucket.
type PredefinedACL string

// Predefined ACLs.
const (
	ACLPrivate           PredefinedACL = "Private"
	ACLPublicRead        PredefinedACL = "PublicRead"
	ACLPublicReadWrite   PredefinedACL = "PublicReadWrite"
	ACLAuthenticatedRead PredefinedACL = "AuthenticatedRead"
)

// BucketSpec specifies the desired state of a Bucket.
type BucketSpec struct {
	runtimev1alpha1.ResourceClaimSpec `json:",inline"`

	// Name specifies the desired name of the bucket.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:MinLength=3
	Name string `json:"name,omitempty"`

	// PredefinedACL specifies a predefined ACL (e.g. Private, ReadWrite, etc)
	// to be applied to the bucket.
	// +kubebuilder:validation:Enum=Private;PublicRead;PublicReadWrite;AuthenticatedRead
	PredefinedACL *PredefinedACL `json:"predefinedACL,omitempty"`

	// LocalPermission specifies permissions granted to a provider specific
	// service account for this bucket, e.g. Read, ReadWrite, or Write.
	// +kubebuilder:validation:Enum=Read;Write;ReadWrite
	LocalPermission *LocalPermissionType `json:"localPermission,omitempty"`
}

var _ resource.Claim = &Bucket{}

// +kubebuilder:object:root=true

// A Bucket is a portable resource claim that may be satisfied by binding to a
// storage bucket PostgreSQL managed resource such as an AWS S3 bucket or Azure
// storage container.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="PREDEFINED-ACL",type="string",JSONPath=".spec.predefinedACL"
// +kubebuilder:printcolumn:name="LOCAL-PERMISSION",type="string",JSONPath=".spec.localPermission"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec                          `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// SetBindingPhase of this Bucket.
func (b *Bucket) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	b.Status.SetBindingPhase(p)
}

// GetBindingPhase of this Bucket.
func (b *Bucket) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return b.Status.GetBindingPhase()
}

// SetConditions of this Bucket.
func (b *Bucket) SetConditions(c ...runtimev1alpha1.Condition) {
	b.Status.SetConditions(c...)
}

// SetPortableClassReference of this Bucket.
func (b *Bucket) SetPortableClassReference(r *corev1.LocalObjectReference) {
	b.Spec.PortableClassReference = r
}

// GetPortableClassReference of this Bucket.
func (b *Bucket) GetPortableClassReference() *corev1.LocalObjectReference {
	return b.Spec.PortableClassReference
}

// SetResourceReference of this Bucket.
func (b *Bucket) SetResourceReference(r *corev1.ObjectReference) {
	b.Spec.ResourceReference = r
}

// GetResourceReference of this Bucket.
func (b *Bucket) GetResourceReference() *corev1.ObjectReference {
	return b.Spec.ResourceReference
}

// SetWriteConnectionSecretToReference of this Bucket.
func (b *Bucket) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	b.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this Bucket.
func (b *Bucket) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return b.Spec.WriteConnectionSecretToReference
}

// +kubebuilder:object:root=true

// BucketList contains a list of Bucket.
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

// All portable classes must satisfy the PortableClass interface
var _ resource.PortableClass = &BucketClass{}

// +kubebuilder:object:root=true

// BucketClass contains a namespace-scoped portable class for Bucket
type BucketClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	runtimev1alpha1.PortableClass `json:",inline"`
}

// All portable class lists must satisfy the PortableClassList interface
var _ resource.PortableClassList = &BucketClassList{}

// +kubebuilder:object:root=true

// BucketClassList contains a list of BucketClass.
type BucketClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BucketClass `json:"items"`
}

// SetPortableClassItems of this BucketClassList.
func (b *BucketClassList) SetPortableClassItems(r []resource.PortableClass) {
	items := make([]BucketClass, 0, len(r))
	for i := range r {
		if item, ok := r[i].(*BucketClass); ok {
			items = append(items, *item)
		}
	}
	b.Items = items
}

// GetPortableClassItems of this BucketClassList.
func (b *BucketClassList) GetPortableClassItems() []resource.PortableClass {
	items := make([]resource.PortableClass, len(b.Items))
	for i, item := range b.Items {
		item := item
		items[i] = resource.PortableClass(&item)
	}
	return items
}
