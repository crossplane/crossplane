/*
Copyright 2020 The Crossplane Authors.

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

package instance

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// A ResourceInstanceCommonSpec defines the desired state of a resource instance.
type ResourceInstanceCommonSpec struct {
	// WriteConnectionSecretToReference specifies the namespace and name of a
	// Secret to which any connection details for this instance should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the bound managed resources.
	// +optional
	WriteConnectionSecretToReference *runtimev1alpha1.SecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// TODO(negz): Make the below references immutable once set? Doing so means
	// we don't have to track what provisioner was used to create a resource.

	// A CompositionSelector specifies labels that will be used to select a
	// composition for this instance. If multiple compositions match the labels
	// one will be chosen at random.
	// +optional
	CompositionSelector *v1.LabelSelector `json:"compositionSelector,omitempty"`

	// A CompositionReference specifies a composition that will be used to
	// dynamically provision the managed resources when the instance is
	// created.
	// +optional
	CompositionReference *corev1.ObjectReference `json:"compositionRef,omitempty"`

	// ResourceReferences array lists existing set of managed resources, in any
	// namespace, to which this resource instance should attempt to bind. Omit this
	// field to enable dynamic provisioning using a composition; the resource
	// references will be automatically populated by Crossplane.
	// +optional
	ResourceReferences []corev1.ObjectReference `json:"resourceRefs,omitempty"`
}
