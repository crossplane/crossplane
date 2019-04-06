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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// MySQLInstanceSpec specifies the configuration of a MySQL instance.
type MySQLInstanceSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// mysql instance properties
	// +kubebuilder:validation:Enum=5.6,5.7
	EngineVersion string `json:"engineVersion"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLInstance is the CRD type for abstract MySQL database instances
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classReference.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type MySQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLInstanceSpec                `json:"spec,omitempty"`
	Status corev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLInstanceList contains a list of MySQLInstance
type MySQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLInstance `json:"items"`
}

// ObjectReference to using this object as a reference
func (m *MySQLInstance) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(m.ObjectMeta, util.IfEmptyString(m.APIVersion, APIVersion), util.IfEmptyString(m.Kind, MySQLInstanceKind))
}

// OwnerReference to use this object as an owner
func (m *MySQLInstance) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(m.ObjectReference())
}

// ClaimStatus returns the status of this resource claim.
func (m *MySQLInstance) ClaimStatus() *corev1alpha1.ResourceClaimStatus {
	return &m.Status
}

// ClassRef returns the resource class used by this resource claim.
func (m *MySQLInstance) ClassRef() *corev1.ObjectReference {
	return m.Spec.ClassRef
}

// ResourceRef returns the resource claimed by this resource claim.
func (m *MySQLInstance) ResourceRef() *corev1.ObjectReference {
	return m.Spec.ResourceRef
}

// SetResourceRef specifies the resource claimed by this resource claim.
func (m *MySQLInstance) SetResourceRef(ref *corev1.ObjectReference) {
	m.Spec.ResourceRef = ref
}

// PostgreSQLInstanceSpec specifies the configuration of this
// PostgreSQLInstance.
type PostgreSQLInstanceSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// postgresql instance properties
	// +kubebuilder:validation:Enum=9.6
	EngineVersion string `json:"engineVersion,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgreSQLInstance is the CRD type for abstract PostgreSQL database instances
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classReference.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type PostgreSQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgreSQLInstanceSpec           `json:"spec,omitempty"`
	Status corev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgreSQLInstanceList contains a list of PostgreSQLInstance
type PostgreSQLInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLInstance `json:"items"`
}

// ObjectReference to using this object as a reference
func (p *PostgreSQLInstance) ObjectReference() *corev1.ObjectReference {
	if p.Kind == "" {
		p.Kind = PostgreSQLInstanceKind
	}
	if p.APIVersion == "" {
		p.APIVersion = APIVersion
	}
	return &corev1.ObjectReference{
		APIVersion: p.APIVersion,
		Kind:       p.Kind,
		Name:       p.Name,
		Namespace:  p.Namespace,
		UID:        p.UID,
	}
}

// OwnerReference to use this object as an owner
func (p *PostgreSQLInstance) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(p.ObjectReference())
}

// ClaimStatus returns the status of this resource claim.
func (p *PostgreSQLInstance) ClaimStatus() *corev1alpha1.ResourceClaimStatus {
	return &p.Status
}

// ClassRef returns the resource class used by this resource claim.
func (p *PostgreSQLInstance) ClassRef() *corev1.ObjectReference {
	return p.Spec.ClassRef
}

// ResourceRef returns the resource claimed by this resource claim.
func (p *PostgreSQLInstance) ResourceRef() *corev1.ObjectReference {
	return p.Spec.ResourceRef
}

// SetResourceRef specifies the resource claimed by this resource claim.
func (p *PostgreSQLInstance) SetResourceRef(ref *corev1.ObjectReference) {
	p.Spec.ResourceRef = ref
}

// LocalPermissionType - Base type for LocalPermissions
type LocalPermissionType string

const (
	// ReadOnlyPermission will grant read objects in a bucket
	ReadOnlyPermission LocalPermissionType = "Read"
	// WriteOnlyPermission will grant write/delete objects in a bucket
	WriteOnlyPermission LocalPermissionType = "Write"
	// ReadWritePermission LocalPermissionType Grant both read and write permissions
	ReadWritePermission LocalPermissionType = "ReadWrite"
)

// PredefinedACL represents predefied bucket ACLs.
type PredefinedACL string

// Predefined ACLs.
const (
	ACLPrivate           PredefinedACL = "Private"
	ACLPublicRead        PredefinedACL = "PublicRead"
	ACLPublicReadWrite   PredefinedACL = "PublicReadWrite"
	ACLAuthenticatedRead PredefinedACL = "AuthenticatedRead"
)

// BucketSpec defines the desired state of Bucket
type BucketSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// Name properties
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:MinLength=3
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Enum=Private,PublicRead,PublicReadWrite,AuthenticatedRead
	// NOTE: AWS S3 and GCP Bucket values (not in Azure)
	PredefinedACL *PredefinedACL `json:"predefinedACL,omitempty"`

	// LocalPermission is the permissions granted on the bucket for the provider specific
	// bucket service account that is available in a secret after provisioning.
	// +kubebuilder:validation:Enum=Read,Write,ReadWrite
	// NOTE: AWS S3 Specific value
	LocalPermission *LocalPermissionType `json:"localPermission,omitempty"`

	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:MinLength=1
	ConnectionSecretNameOverride string `json:"connectionSecretNameOverride,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bucket is the Schema for the Bucket API
// +k8s:openapi-gen=true
// +groupName=storage
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classReference.name"
// +kubebuilder:printcolumn:name="PREDEFINED-ACL",type="string",JSONPath=".spec.predefinedACL"
// +kubebuilder:printcolumn:name="LOCAL-PERMISSION",type="string",JSONPath=".spec.localPermission"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec                       `json:"spec,omitempty"`
	Status corev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BucketList contains a list of Buckets
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

// OwnerReference to use this instance as an owner
func (b *Bucket) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(b.ObjectReference())
}

// ObjectReference to this S3Bucket
func (b *Bucket) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(b.ObjectMeta, util.IfEmptyString(b.APIVersion, APIVersion), util.IfEmptyString(b.Kind, BucketKind))
}

// ClaimStatus returns the status of this resource claim.
func (b *Bucket) ClaimStatus() *corev1alpha1.ResourceClaimStatus {
	return &b.Status
}

// ClassRef returns the resource class used by this resource claim.
func (b *Bucket) ClassRef() *corev1.ObjectReference {
	return b.Spec.ClassRef
}

// ResourceRef returns the resource claimed by this resource claim.
func (b *Bucket) ResourceRef() *corev1.ObjectReference {
	return b.Spec.ResourceRef
}

// SetResourceRef specifies the resource claimed by this resource claim.
func (b *Bucket) SetResourceRef(ref *corev1.ObjectReference) {
	b.Spec.ResourceRef = ref
}
