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
	runtime "k8s.io/apimachinery/pkg/runtime"
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
	// ResourceCredentialsToken
	ResourceCredentialsToken = "token"
)

// AbstractResource defines an abstract resource that can be provisioned and bound to a concrete resource.
type AbstractResource interface {
	runtime.Object
	ResourceStatus() *AbstractResourceStatus
	GetObjectMeta() *metav1.ObjectMeta
	OwnerReference() metav1.OwnerReference
	ObjectReference() *corev1.ObjectReference
	ClassRef() *corev1.ObjectReference
	ResourceRef() *corev1.ObjectReference
	SetResourceRef(*corev1.ObjectReference)
}

// AbstractResourceStatus represents the status of an abstract resource
type AbstractResourceStatus struct {
	ConditionedStatus
	BindingStatusPhase
	// Provisioner is the driver that was used to provision the concrete resource
	// This is an optionally-prefixed name, like a label key.
	// For example: "RDSInstance.database.aws.crossplane.io/v1alpha1" or "CloudSQLInstance.database.gcp.crossplane.io/v1alpha1".
	Provisioner string `json:"provisioner,omitempty"`
}

// ConcreteResource defines a concrete resource that can be provisioned and bound to an abstract resource.
type ConcreteResource interface {
	// Resource connection secret name
	ConnectionSecretName() string
	// Resource endpoint for connection
	Endpoint() string
	// Kubernetes object reference to this resource
	ObjectReference() *corev1.ObjectReference
	// Is resource available for finding
	IsAvailable() bool
	// IsBound() bool
	IsBound() bool
	// Update bound status of the resource
	SetBound(bool)
}

// BasicResource base structure that implements Resource interface
// +k8s:deepcopy-gen=false
type BasicResource struct {
	ConcreteResource
	connectionSecretName string
	endpoint             string
	namespace            string
	state                string
	bound                bool
	objectReference      *corev1.ObjectReference
}

// ConnectionSecretName referenced by this resource
func (br *BasicResource) ConnectionSecretName() string {
	return br.connectionSecretName
}

// Endpoint to establish connection to this resource
func (br *BasicResource) Endpoint() string {
	return br.endpoint
}

// ObjectReference to this resource
func (br *BasicResource) ObjectReference() *corev1.ObjectReference {
	return br.objectReference
}

// IsAvailable
func (br *BasicResource) IsAvailable() bool {
	return br.state == "available"
}

// SetBound
func (br *BasicResource) SetBound(isBound bool) {
	br.bound = isBound
}

// IsBound
func (br *BasicResource) IsBound() bool {
	return br.bound
}

// NewBasicResource new instance of base resource
func NewBasicResource(ref *corev1.ObjectReference, secretName, endpoint, state string) *BasicResource {
	return &BasicResource{
		connectionSecretName: secretName,
		endpoint:             endpoint,
		state:                state,
		objectReference:      ref,
	}
}
