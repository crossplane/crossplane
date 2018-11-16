/*
Copyright 2018 The Conductor Authors.

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
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// S3BucketSpec defines the desired state of S3Bucket
type S3BucketSpec struct {
	Name   string `json:"name,omitempty"`
	Region string `json:"region,omitempty"`
	// CannedACL is one of:
	// private, public-read, public-read-write, authenticated-read bucket-owner-read
	// bucket-owner-full-control, aws-exec-read, log-delivery-write
	CannedACL   string                  `json:"cannedACL,omitempty"`
	Versioning  bool                    `json:"versioning"`
	ProviderRef v1.LocalObjectReference `json:"providerRef"`
}

// S3BucketStatus defines the observed state of S3Bucket
type S3BucketStatus struct {
	corev1alpha1.ConditionedStatus
	Message    string `json:"message,omitempty"`
	ProviderID string `json:"providerID,omitempty"` // the external ID to identify this resource in the cloud provider
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// S3Bucket is the Schema for the S3Bucket API
// +k8s:openapi-gen=true
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

func init() {
	SchemeBuilder.Register(&S3Bucket{}, &S3BucketList{})
}
