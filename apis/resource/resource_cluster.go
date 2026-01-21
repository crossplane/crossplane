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

package resource

import (
	corev1 "k8s.io/api/core/v1"
)

// A ClusterManagedResourceSpec defines the desired state of a cluster-scoped
// managed resource.
type ClusterManagedResourceSpec struct {
	// WriteConnectionSecretToReference specifies the namespace and name of a
	// Secret to which any connection details for this managed resource should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	// +optional
	WriteConnectionSecretToReference *SecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// ProviderConfigReference specifies how the provider that will be used to
	// create, observe, update, and delete this managed resource should be
	// configured.
	// +kubebuilder:default={"name": "default"}
	ProviderConfigReference *Reference `json:"providerConfigRef,omitempty"`

	// THIS IS A BETA FIELD. It is on by default but can be opted out
	// through a Crossplane feature flag.
	// ManagementPolicies specify the array of actions Crossplane is allowed to
	// take on the managed and external resources.
	// This field is planned to replace the DeletionPolicy field in a future
	// release. Currently, both could be set independently and non-default
	// values would be honored if the feature flag is enabled. If both are
	// custom, the DeletionPolicy field will be ignored.
	// See the design doc for more information: https://github.com/crossplane/crossplane/blob/499895a25d1a1a0ba1604944ef98ac7a1a71f197/design/design-doc-observe-only-resources.md?plain=1#L223
	// and this one: https://github.com/crossplane/crossplane/blob/444267e84783136daa93568b364a5f01228cacbe/design/one-pager-ignore-changes.md
	// +optional
	// +kubebuilder:default={"*"}
	ManagementPolicies ManagementPolicies `json:"managementPolicies,omitempty"`

	// DeletionPolicy specifies what will happen to the underlying external
	// when this managed resource is deleted - either "Delete" or "Orphan" the
	// external resource.
	// This field is planned to be deprecated in favor of the ManagementPolicies
	// field in a future release. Currently, both could be set independently and
	// non-default values would be honored if the feature flag is enabled.
	// See the design doc for more information: https://github.com/crossplane/crossplane/blob/499895a25d1a1a0ba1604944ef98ac7a1a71f197/design/design-doc-observe-only-resources.md?plain=1#L223
	// +optional
	// +kubebuilder:default=Delete
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// A TargetSpec defines the common fields of objects used for exposing
// infrastructure to workloads that can be scheduled to.
//
// Deprecated.
type TargetSpec struct {
	// WriteConnectionSecretToReference specifies the name of a Secret, in the
	// same namespace as this target, to which any connection details for this
	// target should be written or already exist. Connection secrets referenced
	// by a target should contain information for connecting to a resource that
	// allows for scheduling of workloads.
	// +optional
	WriteConnectionSecretToReference *LocalSecretReference `json:"connectionSecretRef,omitempty"`

	// A ResourceReference specifies an existing managed resource, in any
	// namespace, which this target should attempt to propagate a connection
	// secret from.
	// +optional
	ResourceReference *corev1.ObjectReference `json:"clusterRef,omitempty"`
}

// A TargetStatus defines the observed status a target.
//
// Deprecated.
type TargetStatus struct {
	ConditionedStatus `json:",inline"`
}
