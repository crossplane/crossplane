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
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// S3BucketSpec defines the desired state of S3Bucket
type S3BucketSpec struct {
	// NameFormat to format bucket name passing it a object UID
	// If not provided, defaults to "%s", i.e. UID value
	NameFormat string `json:"nameFormat,omitempty"`

	// Region is the aws region for the bucket
	Region string `json:"region"`
	// CannedACL applies a standard aws built-in ACL for common bucket use cases.
	// +kubebuilder:validation:Enum=private,public-read,public-read-write,authenticated-read,log-delivery-write,aws-exec-read
	CannedACL  *s3.BucketCannedACL `json:"cannedACL,omitempty"`
	Versioning bool                `json:"versioning,omitempty"`
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:MinLength=1
	ConnectionSecretNameOverride string `json:"connectionSecretNameOverride,omitempty"`

	// LocalPermission is the permissions granted on the bucket for the provider specific
	// bucket service account that is available in a secret after provisioning.
	// +kubebuilder:validation:Enum=Read,Write,ReadWrite
	LocalPermission *storagev1alpha1.LocalPermissionType `json:"localPermission"`

	ProviderRef v1.LocalObjectReference `json:"providerRef"`
	ClaimRef    *v1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef    *v1.ObjectReference     `json:"classRef,omitempty"`
	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// S3BucketStatus defines the observed state of S3Bucket
type S3BucketStatus struct {
	corev1alpha1.DeprecatedConditionedStatus
	corev1alpha1.BindingStatusPhase
	Message               string                              `json:"message,omitempty"`
	ProviderID            string                              `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
	ConnectionSecretRef   v1.LocalObjectReference             `json:"connectionSecretRef,omitempty"`
	IAMUsername           string                              `json:"iamUsername,omitempty"`
	LastUserPolicyVersion int                                 `json:"lastUserPolicyVersion,omitempty"`
	LastLocalPermission   storagev1alpha1.LocalPermissionType `json:"lastLocalPermission,omitempty"`
}

// NewS3BucketSpec from properties map
func NewS3BucketSpec(properties map[string]string) *S3BucketSpec {
	spec := &S3BucketSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,
	}

	val, ok := properties["localPermission"]
	if ok {
		perm := storagev1alpha1.LocalPermissionType(val)
		spec.LocalPermission = &perm
	}

	val, ok = properties["cannedACL"]
	if ok {
		perm := s3.BucketCannedACL(val)
		spec.CannedACL = &perm
	}

	val, ok = properties["versioning"]
	if ok {
		if versioning, err := strconv.ParseBool(val); err == nil {
			spec.Versioning = versioning
		}
	}

	val, ok = properties["region"]
	if ok {
		spec.Region = val
	}

	return spec
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// S3BucketList contains a list of S3Buckets
type S3BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []S3Bucket `json:"items"`
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

// ConnectionSecretName returns the name of the connection secret
func (b *S3Bucket) ConnectionSecretName() string {
	if b.Status.ConnectionSecretRef.Name != "" {
		return b.Status.ConnectionSecretRef.Name
	} else if b.Spec.ConnectionSecretNameOverride != "" {
		return b.Spec.ConnectionSecretNameOverride
	}
	return b.Name
}

// IsAvailable for usage/binding
func (b *S3Bucket) IsAvailable() bool {
	return b.Status.IsDeprecatedCondition(corev1alpha1.DeprecatedReady)
}

// IsBound returns true if this bucket is bound to a resource claim.
func (b *S3Bucket) IsBound() bool {
	return b.Status.IsBound()
}

// SetBound specifies whether this bucket is bound to a resource claim.
func (b *S3Bucket) SetBound(bound bool) {
	b.Status.SetBound(bound)
}
