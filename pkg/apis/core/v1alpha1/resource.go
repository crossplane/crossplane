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
)

const (
	// ResourceCredentialsSecretEndpointKey is the key inside a connection secret for the connection endpoint
	ResourceCredentialsSecretEndpointKey = "endpoint"
	// ResourceCredentialsSecretUserKey is the key inside a connection secret for the connection user
	ResourceCredentialsSecretUserKey = "username"
	// ResourceCredentialsSecretPasswordKey is the key inside a connection secret for the connection password
	ResourceCredentialsSecretPasswordKey = "password"
	// ResourceCredentialsSecretCAKey is the key inside a connection secret for the server CA certificate
	ResourceCredentialsSecretCAKey = "clusterCA"
	// ResourceCredentialsSecretClientCertKey is the key inside a connection secret for the client certificate
	ResourceCredentialsSecretClientCertKey = "clientCert"
	// ResourceCredentialsSecretClientKeyKey is the key inside a connection secret for the client key
	ResourceCredentialsSecretClientKeyKey = "clientKey"
	// ResourceCredentialsTokenKey is the key inside a connection secret for the bearer token value
	ResourceCredentialsTokenKey = "token"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceClass is the Schema for the instances API
// +kubebuilder:printcolumn:name="PROVISIONER",type="string",JSONPath=".provisioner"
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ResourceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Parameters holds parameters for the provisioner.
	// These values are opaque to the  system and are passed directly
	// to the provisioner.  The only validation done on keys is that they are
	// not empty.  The maximum number of parameters is
	// 512, with a cumulative max size of 256K
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// Provisioner is the driver expected to handle this ResourceClass.
	// This is an optionally-prefixed name, like a label key.
	// For example: "RDSInstance.database.aws.crossplane.io/v1alpha1" or "CloudSQLInstance.database.gcp.crossplane.io/v1alpha1".
	// This value may not be empty.
	Provisioner string `json:"provisioner"`

	// ProviderReference is a reference to the cloud provider that will be used
	// to provision the concrete cloud resource.
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	// reclaimPolicy is the reclaim policy that dynamically provisioned
	// ResourceInstances of this resource class are created with
	// +optional
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// NOTE(hasheddan): cannot check for satisfying resource.Class interface
// because of circular imports

// GetReclaimPolicy of this ResourceClass.
func (r *ResourceClass) GetReclaimPolicy() ReclaimPolicy {
	return r.ReclaimPolicy
}

// SetReclaimPolicy of this ResourceClass.
func (r *ResourceClass) SetReclaimPolicy(p ReclaimPolicy) {
	r.ReclaimPolicy = p
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceClassList contains a list of resource classes.
type ResourceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceClass `json:"items"`
}

// ResourceClaimSpec contains standard fields that all resource claims should
// include in their spec. Unlike ResourceClaimStatus, ResourceClaimSpec should
// typically be embedded in a claim specific struct.
type ResourceClaimSpec struct {
	WriteConnectionSecretToReference corev1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`

	// TODO(negz): Make the below references immutable once set? Doing so means
	// we don't have to track what provisioner was used to create a resource.

	ClassReference    *corev1.ObjectReference `json:"classRef,omitempty"`
	ResourceReference *corev1.ObjectReference `json:"resourceRef,omitempty"`
}

// ResourceClaimStatus represents the status of a resource claim. Claims should
// typically use this struct as their status.
type ResourceClaimStatus struct {
	ConditionedStatus `json:",inline"`
	BindingStatus     `json:",inline"`
}

// ResourceSpec contains standard fields that all resources should
// include in their spec. ResourceSpec should typically be embedded in a
// resource specific struct.
type ResourceSpec struct {
	WriteConnectionSecretToReference corev1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`

	ClaimReference    *corev1.ObjectReference `json:"claimRef,omitempty"`
	ClassReference    *corev1.ObjectReference `json:"classRef,omitempty"`
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// ResourceClassSpecTemplate contains standard fields that all resource classes should
// include in their spec template. ResourceClassSpecTemplate should typically be embedded in a
// resource class specific struct.
type ResourceClassSpecTemplate struct {
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// ResourceStatus contains standard fields that all resources should
// include in their status. ResourceStatus should typically be embedded in a
// resource specific status.
type ResourceStatus struct {
	ConditionedStatus `json:",inline"`
	BindingStatus     `json:",inline"`
}

// Policy contains standard fields that all policies should include. Policy
// should typically be embedded in a specific resource claim policy.
type Policy struct {
	DefaultClassReference *corev1.ObjectReference `json:"defaultClassRef,omitempty"`
}

// SetDefaultClassReference of this Policy
func (p *Policy) SetDefaultClassReference(r *corev1.ObjectReference) {
	p.DefaultClassReference = r
}

// GetDefaultClassReference of this Policy
func (p *Policy) GetDefaultClassReference() *corev1.ObjectReference {
	return p.DefaultClassReference
}
