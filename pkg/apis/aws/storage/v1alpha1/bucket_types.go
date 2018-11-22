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
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

var (
	predefinedACLMap = map[storagev1alpha1.PredefinedACL]s3.ObjectCannedACL{
		storagev1alpha1.ACLPrivate:           s3.ObjectCannedACLPrivate,
		storagev1alpha1.ACLPublicRead:        s3.ObjectCannedACLPublicRead,
		storagev1alpha1.ACLPublicReadWrite:   s3.ObjectCannedACLPublicReadWrite,
		storagev1alpha1.ACLAuthenticatedRead: s3.ObjectCannedACLAuthenticatedRead,
	}
)

func GetALCMap() map[storagev1alpha1.PredefinedACL]s3.ObjectCannedACL {
	return predefinedACLMap
}

// S3BucketSpec defines the desired state of S3Bucket
type S3BucketSpec struct {
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:MinLength=3
	Name   string `json:"name,omitempty"`
	Region string `json:"region"`
	// +kubebuilder:validation:Enum=private,public-read,public-read-write,authenticated-read,aws-exec-read,bucket-owner-read,bucket-owner-full-control
	CannedACL  s3.ObjectCannedACL `json:"cannedACL,omitempty"`
	Versioning bool               `json:"versioning"`
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:MinLength=1
	ConnectionSecretNameOverride string                  `json:"connectionSecretNameOverride,omitempty"`
	ProviderRef                  v1.LocalObjectReference `json:"providerRef"`
	// +kubebuilder:validation:Enum=read,write
	LocalPermissions []storagev1alpha1.LocalPermissionType `json:"localPermissions,omitempty"`
	ClaimRef         *v1.ObjectReference                   `json:"claimRef,omitempty"`
	ClassRef         *v1.ObjectReference                   `json:"classRef,omitempty"`
	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// S3BucketStatus defines the observed state of S3Bucket
type S3BucketStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	Message             string                  `json:"message,omitempty"`
	ProviderID          string                  `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
	IAMUsername         *string                 `json:"iamUsername,omitempty"`
}

// NewS3BucketSpec from properties map
func NewS3BucketSpec(properties map[string]string) *S3BucketSpec {
	spec := &S3BucketSpec{
		CannedACL:  s3.ObjectCannedACLPrivate,
		Versioning: false,
	}

	val, ok := properties["localPermissions"]
	if ok {
		for _, perm := range strings.Split(val, ",") {
			spec.LocalPermissions = append(spec.LocalPermissions, storagev1alpha1.LocalPermissionType(perm))
		}
	}

	val, ok = properties["predefinedACL"]
	if ok {
		acl, ok := predefinedACLMap[storagev1alpha1.PredefinedACL(val)]
		if ok {
			spec.CannedACL = acl
		}

	}

	val, ok = properties["versioning"]
	if ok {
		if versioning, err := strconv.ParseBool(val); err != nil {
			spec.Versioning = versioning
		}
	}

	val, ok = properties["connectionSecretNameOverride"]
	if ok {
		spec.ConnectionSecretNameOverride = val
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
// +k8s:openapi-gen=true
// +groupName=storage.aws
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

// ObjectReference to this S3Bucket
func (b *S3Bucket) ObjectReference() *v1.ObjectReference {
	if b.Kind == "" {
		b.Kind = S3BucketKind
	}
	if b.APIVersion == "" {
		b.APIVersion = APIVersion
	}
	return &v1.ObjectReference{
		APIVersion:      b.APIVersion,
		Kind:            b.Kind,
		Name:            b.Name,
		Namespace:       b.Namespace,
		ResourceVersion: b.ResourceVersion,
		UID:             b.UID,
	}
}

// ConnectionSecretName returns a secret name from the reference
func (b *S3Bucket) ConnectionSecretName() string {
	if b.Spec.ConnectionSecretNameOverride != "" {
		return b.Spec.ConnectionSecretNameOverride
	} else if b.Status.ConnectionSecretRef.Name != "" {
		return b.Status.ConnectionSecretRef.Name
	}

	return util.GenerateName(b.Spec.Name)
}

// OwnerReference to use this instance as an owner
func (b *S3Bucket) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(b.ObjectReference())
}

// Endpoint returns the endpoint for the bucket
func (b *S3Bucket) Endpoint() string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com", b.Spec.Region)
}

// IsAvailable for usage/binding
func (b *S3Bucket) IsAvailable() bool {
	return b.Status.IsCondition(corev1alpha1.Ready)
}

// IsBound
func (b *S3Bucket) IsBound() bool {
	return b.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound
func (r *S3Bucket) SetBound(state bool) {
	if state {
		r.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		r.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}
