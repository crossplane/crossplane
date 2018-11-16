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
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//---------------------------------------------------------------------------------------------------------------------
// MySQLInstance

// MySQLInstanceSpec
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
type MySQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLInstanceSpec                   `json:"spec,omitempty"`
	Status corev1alpha1.AbstractResourceStatus `json:"status,omitempty"`
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

func (m *MySQLInstance) ResourceStatus() *corev1alpha1.AbstractResourceStatus {
	return &m.Status
}

func (m *MySQLInstance) GetObjectMeta() *metav1.ObjectMeta {
	return &m.ObjectMeta
}

func (m *MySQLInstance) ClassRef() *corev1.ObjectReference {
	return m.Spec.ClassRef
}

func (m *MySQLInstance) ResourceRef() *corev1.ObjectReference {
	return m.Spec.ResourceRef
}

func (m *MySQLInstance) SetResourceRef(ref *corev1.ObjectReference) {
	m.Spec.ResourceRef = ref
}

//---------------------------------------------------------------------------------------------------------------------
// PostgreSQLInstance

// PostgreSQLInstanceSpec
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
type PostgreSQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgreSQLInstanceSpec              `json:"spec,omitempty"`
	Status corev1alpha1.AbstractResourceStatus `json:"status,omitempty"`
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

func (p *PostgreSQLInstance) ResourceStatus() *corev1alpha1.AbstractResourceStatus {
	return &p.Status
}

func (p *PostgreSQLInstance) GetObjectMeta() *metav1.ObjectMeta {
	return &p.ObjectMeta
}

func (p *PostgreSQLInstance) ClassRef() *corev1.ObjectReference {
	return p.Spec.ClassRef
}

func (p *PostgreSQLInstance) ResourceRef() *corev1.ObjectReference {
	return p.Spec.ResourceRef
}

func (p *PostgreSQLInstance) SetResourceRef(ref *corev1.ObjectReference) {
	p.Spec.ResourceRef = ref
}

//---------------------------------------------------------------------------------------------------------------------
// Bucket

// BucketSpec defines the desired state of Bucket
type BucketSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// Bucket properties
	Name string `json:"name,omitempty"`
	// PredefinedACL is one of
	// one of: private, publicRead, publicReadWrite(*), AuthenticatedRead(*)
	// * Not available on Azure
	PredefinedACL string `json:"predefinedACL,omitempty"`

	// LocalPermissions are the permissions granted on the bucket for the provider specific
	// bucket service account.
	// one of: read, write
	LocalPermissions []string `json:"localPermissions,omitempty"`
}

// BucketClaimStatus
type BucketClaimStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	// Provisioner is the driver that was used to provision the concrete resource
	// This is an optionally-prefixed name, like a label key.
	// For example: "S3Bucket.storage.aws.crossplane.io/v1alpha1" or "GoogleBucket.storage.gcp.crossplane.io/v1alpha1".
	Provisioner string `json:"provisioner,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bucket is the Schema for the Bucket API
// +k8s:openapi-gen=true
// +groupName=storage
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec        `json:"spec,omitempty"`
	Status BucketClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BucketList contains a list of Buckets
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}
