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

import corev1 "k8s.io/api/core/v1"

// Resource defines operations supported by managed resource
type Resource interface {
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
	Resource
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
