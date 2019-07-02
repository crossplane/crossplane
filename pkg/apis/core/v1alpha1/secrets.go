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
)

// PublishConnectionTo specifies how a managed resource or claim publishes its
// connection details for consumption by other resources.
type PublishConnectionTo struct {
	// ServiceReference specifies a local Secret to which sensitive connection
	// details such as passwords and certificates will be published. The Secret
	// should not exist; it will be created by Crossplane.
	SecretReference *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// ConfigMapReference specifies a local ConfigMap to which non-sensitive
	// connection details such as usernames, addresses, and ports will be
	// published. The ConfigMap should not exist; it will be created by
	// Crossplane.
	ConfigMapReference *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// ServiceReference specifies a local ExternalName type Service that will
	// publish this resource's connection endpoint. The Service should not
	// exist; it will be created by Crossplane.
	ServiceReference *corev1.LocalObjectReference `json:"serviceRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// A SecretTransform requests a standard Crossplane connection secret be
// transformed into a form required by a particular application.
type SecretTransform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SourceReference is the source secret, presumed to be an existing standard
	// Crossplane connection secret.
	SourceReference corev1.LocalObjectReference `json:"sourceRef"`

	// DestinationReference is the destination secret, written according to
	// this SecretTransform. It should not exist; it will be created by
	// Crossplane.
	DestinationReference corev1.LocalObjectReference `json:"destinationRef"`

	// DataTemplate is a map of keys to value templates. Keys correspond to the
	// keys of the DestinationReference's Data map. Templates are Go templates,
	// executed against the DestinationReference's data map.
	DataTemplate map[string]string `json:"template"`
}
