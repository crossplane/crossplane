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

package v1

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/v2/apis/common"
)

const (
	// ResourceCredentialsSecretEndpointKey is the key inside a connection secret for the connection endpoint.
	ResourceCredentialsSecretEndpointKey = "endpoint"
	// ResourceCredentialsSecretPortKey is the key inside a connection secret for the connection port.
	ResourceCredentialsSecretPortKey = "port"
	// ResourceCredentialsSecretUserKey is the key inside a connection secret for the connection user.
	ResourceCredentialsSecretUserKey = "username"
	// ResourceCredentialsSecretPasswordKey is the key inside a connection secret for the connection password.
	ResourceCredentialsSecretPasswordKey = "password"
	// ResourceCredentialsSecretCAKey is the key inside a connection secret for the server CA certificate.
	ResourceCredentialsSecretCAKey = "clusterCA"
	// ResourceCredentialsSecretClientCertKey is the key inside a connection secret for the client certificate.
	ResourceCredentialsSecretClientCertKey = "clientCert"
	// ResourceCredentialsSecretClientKeyKey is the key inside a connection secret for the client key.
	ResourceCredentialsSecretClientKeyKey = "clientKey"
	// ResourceCredentialsSecretTokenKey is the key inside a connection secret for the bearer token value.
	ResourceCredentialsSecretTokenKey = "token"
	// ResourceCredentialsSecretKubeconfigKey is the key inside a connection secret for the raw kubeconfig yaml.
	ResourceCredentialsSecretKubeconfigKey = "kubeconfig"
)

// LabelKeyProviderKind is added to ProviderConfigUsages to relate them to their
// ProviderConfig.
const LabelKeyProviderKind = common.LabelKeyProviderKind

// LabelKeyProviderName is added to ProviderConfigUsages to relate them to their
// ProviderConfig.
const LabelKeyProviderName = common.LabelKeyProviderName

// NOTE(negz): The below secret references differ from ObjectReference and
// LocalObjectReference in that they include only the fields Crossplane needs to
// reference a secret, and make those fields required. This reduces ambiguity in
// the API for resource authors.

// A LocalSecretReference is a reference to a secret in the same namespace as
// the referencer.
type LocalSecretReference = common.LocalSecretReference

// A SecretReference is a reference to a secret in an arbitrary namespace.
type SecretReference = common.SecretReference

// A SecretKeySelector is a reference to a secret key in an arbitrary namespace.
type SecretKeySelector = common.SecretKeySelector

// A LocalSecretKeySelector is a reference to a secret key
// in the same namespace with the referencing object.
type LocalSecretKeySelector = common.LocalSecretKeySelector

// Policy represents the Resolve and Resolution policies of Reference instance.
type Policy = common.Policy

// A Reference to a named object.
type Reference = common.Reference

// A NamespacedReference to a named object.
type NamespacedReference = common.NamespacedReference

// A TypedReference refers to an object by Name, Kind, and APIVersion. It is
// commonly used to reference cluster-scoped objects or objects where the
// namespace is already known.
type TypedReference = common.TypedReference

// A Selector selects an object.
type Selector = common.Selector

// NamespacedSelector selects a namespaced object.
type NamespacedSelector = common.NamespacedSelector

// ProviderConfigReference is a typed reference to a ProviderConfig
// object, with a known api group.
type ProviderConfigReference = common.ProviderConfigReference

// TODO(negz): Rename Resource* to Managed* to clarify that they enable the
// resource.Managed interface.

// A ResourceSpec defines the desired state of a managed resource.
type ResourceSpec struct {
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

// ResourceStatus represents the observed state of a managed resource.
type ResourceStatus struct {
	ConditionedStatus `json:",inline"`
	ObservedStatus    `json:",inline"`
}

// A CredentialsSource is a source from which provider credentials may be
// acquired.
type CredentialsSource = common.CredentialsSource

const (
	// CredentialsSourceNone indicates that a provider does not require
	// credentials.
	CredentialsSourceNone = common.CredentialsSourceNone

	// CredentialsSourceSecret indicates that a provider should acquire
	// credentials from a secret.
	CredentialsSourceSecret = common.CredentialsSourceSecret

	// CredentialsSourceInjectedIdentity indicates that a provider should use
	// credentials via its (pod's) identity; i.e. via IRSA for AWS,
	// Workload Identity for GCP, Pod Identity for Azure, or in-cluster
	// authentication for the Kubernetes API.
	CredentialsSourceInjectedIdentity = common.CredentialsSourceInjectedIdentity

	// CredentialsSourceEnvironment indicates that a provider should acquire
	// credentials from an environment variable.
	CredentialsSourceEnvironment = common.CredentialsSourceEnvironment

	// CredentialsSourceFilesystem indicates that a provider should acquire
	// credentials from the filesystem.
	CredentialsSourceFilesystem = common.CredentialsSourceFilesystem
)

// CommonCredentialSelectors provides common selectors for extracting
// credentials.
type CommonCredentialSelectors = common.CommonCredentialSelectors

// EnvSelector selects an environment variable.
type EnvSelector = common.EnvSelector

// FsSelector selects a filesystem location.
type FsSelector = common.FsSelector

// A ProviderConfigStatus defines the observed status of a ProviderConfig.
type ProviderConfigStatus = common.ProviderConfigStatus

// A ProviderConfigUsage is a record that a particular managed resource is using
// a particular provider configuration.
type ProviderConfigUsage = common.ProviderConfigUsage

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
