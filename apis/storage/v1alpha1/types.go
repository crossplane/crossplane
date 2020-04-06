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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
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

	// TODO(negz): Remove these class fields? Fields are almost certainly not
	// portable across all providers.

	// PredefinedACL specifies a predefined ACL (e.g. Private, ReadWrite, etc)
	// to be applied to the bucket.
	// +kubebuilder:validation:Enum=Private;PublicRead;PublicReadWrite;AuthenticatedRead
	PredefinedACL *PredefinedACL `json:"predefinedACL,omitempty"`

	// LocalPermission specifies permissions granted to a provider specific
	// service account for this bucket, e.g. Read, ReadWrite, or Write.
	// +kubebuilder:validation:Enum=Read;Write;ReadWrite
	LocalPermission *LocalPermissionType `json:"localPermission,omitempty"`
}

// +kubebuilder:object:root=true

// A Bucket is a portable resource claim that may be satisfied by binding to a
// managed resource such as an AWS S3 bucket or Azure storage container.
// +kubebuilder:resource:categories={crossplane,claim}
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="CLASS-KIND",type="string",JSONPath=".spec.classRef.kind"
// +kubebuilder:printcolumn:name="CLASS-NAME",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".spec.resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".spec.resourceRef.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec                          `json:"spec,omitempty"`
	Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BucketList contains a list of Bucket.
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}
