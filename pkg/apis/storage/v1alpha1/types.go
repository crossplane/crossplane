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

//----------------------------------------------------------------------------------------------------------------------

// MySQLInstanceSpec
type MySQLInstanceSpec struct {
	ClassRef    *corev1.ObjectReference `json:"classReference,omitempty"`
	ResourceRef *corev1.ObjectReference `json:"resourceName,omitempty"`
	Selector    metav1.LabelSelector    `json:"selector,omitempty"`

	// mysql instance properties
	EngineVersion string `json:"engineVersion"`
}

// MySQLInstanceClaimStatus
type MySQLInstanceClaimStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	// Provisioner is the driver that was used to provision the concrete resrouce
	// This is an optionally-prefixed name, like a label key.
	// For example: "RDSInstance.database.aws.crossplane.io/v1alpha1" or "CloudSQLInstance.database.gcp.crossplane.io/v1alpha1".
	Provisioner string `json:"provisioner,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLInstance is the Schema for the instances API
// +k8s:openapi-gen=true
type MySQLInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLInstanceSpec        `json:"spec,omitempty"`
	Status MySQLInstanceClaimStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLInstanceList contains a list of RDSInstance
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
