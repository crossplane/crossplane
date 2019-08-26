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
	"strconv"

	storagev1alpha1 "github.com/crossplaneio/crossplane/apis/storage/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/util"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// S3BucketParameters defines the desired state of S3Bucket
type S3BucketParameters struct {
	// NameFormat to format bucket name passing it a object UID
	// If not provided, defaults to "%s", i.e. UID value
	NameFormat string `json:"nameFormat,omitempty"`

	// Region is the aws region for the bucket
	Region string `json:"region"`
	// CannedACL applies a standard aws built-in ACL for common bucket use cases.
	// +kubebuilder:validation:Enum=private;public-read;public-read-write;authenticated-read;log-delivery-write;aws-exec-read
	CannedACL  *s3.BucketCannedACL `json:"cannedACL,omitempty"`
	Versioning bool                `json:"versioning,omitempty"`

	// LocalPermission is the permissions granted on the bucket for the provider specific
	// bucket service account that is available in a secret after provisioning.
	// +kubebuilder:validation:Enum=Read;Write;ReadWrite
	LocalPermission *storagev1alpha1.LocalPermissionType `json:"localPermission"`
}

// S3BucketSpec defines the desired state of S3Bucket
type S3BucketSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	S3BucketParameters           `json:",inline"`
}

// S3BucketStatus defines the observed state of S3Bucket
type S3BucketStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`

	Message               string                              `json:"message,omitempty"`
	ProviderID            string                              `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
	IAMUsername           string                              `json:"iamUsername,omitempty"`
	LastUserPolicyVersion int                                 `json:"lastUserPolicyVersion,omitempty"`
	LastLocalPermission   storagev1alpha1.LocalPermissionType `json:"lastLocalPermission,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// S3Bucket is the Schema for the S3Bucket API
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="PREDEFINED-ACL",type="string",JSONPath=".spec.cannedACL"
// +kubebuilder:printcolumn:name="LOCAL-PERMISSION",type="string",JSONPath=".spec.localPermission"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type S3Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   S3BucketSpec   `json:"spec,omitempty"`
	Status S3BucketStatus `json:"status,omitempty"`
}

// SetBindingPhase of this S3Bucket.
func (b *S3Bucket) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
	b.Status.SetBindingPhase(p)
}

// GetBindingPhase of this S3Bucket.
func (b *S3Bucket) GetBindingPhase() runtimev1alpha1.BindingPhase {
	return b.Status.GetBindingPhase()
}

// SetConditions of this S3Bucket.
func (b *S3Bucket) SetConditions(c ...runtimev1alpha1.Condition) {
	b.Status.SetConditions(c...)
}

// SetClaimReference of this S3Bucket.
func (b *S3Bucket) SetClaimReference(r *corev1.ObjectReference) {
	b.Spec.ClaimReference = r
}

// GetClaimReference of this S3Bucket.
func (b *S3Bucket) GetClaimReference() *corev1.ObjectReference {
	return b.Spec.ClaimReference
}

// SetClassReference of this S3Bucket.
func (b *S3Bucket) SetClassReference(r *corev1.ObjectReference) {
	b.Spec.ClassReference = r
}

// GetClassReference of this S3Bucket.
func (b *S3Bucket) GetClassReference() *corev1.ObjectReference {
	return b.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this S3Bucket.
func (b *S3Bucket) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	b.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this S3Bucket.
func (b *S3Bucket) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return b.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this S3Bucket.
func (b *S3Bucket) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return b.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this S3Bucket.
func (b *S3Bucket) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	b.Spec.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// S3BucketList contains a list of S3Buckets
type S3BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []S3Bucket `json:"items"`
}

// S3BucketClassSpecTemplate is the Schema for the resource class
type S3BucketClassSpecTemplate struct {
	runtimev1alpha1.ResourceClassSpecTemplate `json:",inline"`
	S3BucketParameters                        `json:",inline"`
}

var _ resource.Class = &S3BucketClass{}

// +kubebuilder:object:root=true

// S3BucketClass is the Schema for the resource class
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".specTemplate.providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".specTemplate.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type S3BucketClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	SpecTemplate S3BucketClassSpecTemplate `json:"specTemplate,omitempty"`
}

// GetReclaimPolicy of this S3BucketClass.
func (i *S3BucketClass) GetReclaimPolicy() runtimev1alpha1.ReclaimPolicy {
	return i.SpecTemplate.ReclaimPolicy
}

// SetReclaimPolicy of this S3BucketClass.
func (i *S3BucketClass) SetReclaimPolicy(p runtimev1alpha1.ReclaimPolicy) {
	i.SpecTemplate.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// S3BucketClassList contains a list of cloud memorystore resource classes.
type S3BucketClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []S3BucketClass `json:"items"`
}

// GetBucketName based on the NameFormat spec value,
// If name format is not provided, bucket name defaults to UID
// If name format provided with '%s' value, bucket name will result in formatted string + UID,
//   NOTE: only single %s substitution is supported
// If name format does not contain '%s' substitution, i.e. a constant string, the
// constant string value is returned back
//
// Examples:
//   For all examples assume "UID" = "test-uid"
//   1. NameFormat = "", BucketName = "test-uid"
//   2. NameFormat = "%s", BucketName = "test-uid"
//   3. NameFormat = "foo", BucketName = "foo"
//   4. NameFormat = "foo-%s", BucketName = "foo-test-uid"
//   5. NameFormat = "foo-%s-bar-%s", BucketName = "foo-test-uid-bar-%!s(MISSING)"
func (b *S3Bucket) GetBucketName() string {
	return util.ConditionalStringFormat(b.Spec.NameFormat, string(b.GetUID()))
}

// SetUserPolicyVersion specifies this bucket's policy version.
func (b *S3Bucket) SetUserPolicyVersion(policyVersion string) error {
	policyInt, err := strconv.Atoi(policyVersion[1:])
	if err != nil {
		return err
	}
	b.Status.LastUserPolicyVersion = policyInt
	b.Status.LastLocalPermission = *b.Spec.LocalPermission

	return nil
}

// HasPolicyChanged returns true if the bucket's policy is older than the
// supplied version.
func (b *S3Bucket) HasPolicyChanged(policyVersion string) (bool, error) {
	if *b.Spec.LocalPermission != b.Status.LastLocalPermission {
		return true, nil
	}
	policyInt, err := strconv.Atoi(policyVersion[1:])
	if err != nil {
		return false, err
	}
	if b.Status.LastUserPolicyVersion != policyInt && b.Status.LastUserPolicyVersion < policyInt {
		return true, nil
	}

	return false, nil
}
