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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// ResourceCredentialsSecretEndpointKey is the key inside a connection secret for the connection endpoint
	ResourceCredentialsSecretEndpointKey = "endpoint"
	// ResourceCredentialsSecretPortKey is the key inside a connection secret for the connection port
	ResourceCredentialsSecretPortKey = "port"
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
	// ResourceCredentialsSecretTokenKey is the key inside a connection secret for the bearer token value
	ResourceCredentialsSecretTokenKey = "token"
	// ResourceCredentialsSecretKubeconfigKey is the key inside a connection secret for the raw kubeconfig yaml
	ResourceCredentialsSecretKubeconfigKey = "kubeconfig"
)

// LabelKeyProviderName is added to ProviderConfigUsages to relate them to their
// ProviderConfig.
const LabelKeyProviderName = "crossplane.io/provider-config"

// NOTE(negz): The below secret references differ from ObjectReference and
// LocalObjectReference in that they include only the fields Crossplane needs to
// reference a secret, and make those fields required. This reduces ambiguity in
// the API for resource authors.

// A LocalSecretReference is a reference to a secret in the same namespace as
// the referencer.
type LocalSecretReference struct {
	// Name of the secret.
	Name string `json:"name"`
}

// A SecretReference is a reference to a secret in an arbitrary namespace.
type SecretReference struct {
	// Name of the secret.
	Name string `json:"name"`

	// Namespace of the secret.
	Namespace string `json:"namespace"`
}

// A SecretKeySelector is a reference to a secret key in an arbitrary namespace.
type SecretKeySelector struct {
	SecretReference `json:",inline"`

	// The key to select.
	Key string `json:"key"`
}

// Policy represents the Resolve and Resolution policies of Reference instance.
type Policy struct {
	// Resolve specifies when this reference should be resolved. The default
	// is 'IfNotPresent', which will attempt to resolve the reference only when
	// the corresponding field is not present. Use 'Always' to resolve the
	// reference on every reconcile.
	// +optional
	// +kubebuilder:validation:Enum=Always;IfNotPresent
	Resolve *ResolvePolicy `json:"resolve,omitempty"`

	// Resolution specifies whether resolution of this reference is required.
	// The default is 'Required', which means the reconcile will fail if the
	// reference cannot be resolved. 'Optional' means this reference will be
	// a no-op if it cannot be resolved.
	// +optional
	// +kubebuilder:default=Required
	// +kubebuilder:validation:Enum=Required;Optional
	Resolution *ResolutionPolicy `json:"resolution,omitempty"`
}

// IsResolutionPolicyOptional checks whether the resolution policy of relevant reference is Optional.
func (p *Policy) IsResolutionPolicyOptional() bool {
	if p == nil || p.Resolution == nil {
		return false
	}
	return *p.Resolution == ResolutionPolicyOptional
}

// IsResolvePolicyAlways checks whether the resolution policy of relevant reference is Always.
func (p *Policy) IsResolvePolicyAlways() bool {
	if p == nil || p.Resolve == nil {
		return false
	}
	return *p.Resolve == ResolvePolicyAlways
}

// A Reference to a named object.
type Reference struct {
	// Name of the referenced object.
	Name string `json:"name"`

	// Policies for referencing.
	// +optional
	Policy *Policy `json:"policy,omitempty"`
}

// A TypedReference refers to an object by Name, Namespace, Kind, and
// APIVersion. It is commonly used to reference cluster-scoped objects
// and can also be used for namespace-scoped objects.
type TypedReference struct {
	// APIVersion of the referenced object.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced object.
	Kind string `json:"kind"`

	// Name of the referenced object.
	Name string `json:"name"`

	// Namespace of the referenced object.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// UID of the referenced object.
	// +optional
	UID types.UID `json:"uid,omitempty"`
}

// A Selector selects an object.
type Selector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchControllerRef ensures an object with the same controller reference
	// as the selecting object is selected.
	MatchControllerRef *bool `json:"matchControllerRef,omitempty"`

	// Policies for selection.
	// +optional
	Policy *Policy `json:"policy,omitempty"`
}

// SetGroupVersionKind sets the Kind and APIVersion of a TypedReference.
func (obj *TypedReference) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	obj.APIVersion, obj.Kind = gvk.ToAPIVersionAndKind()
}

// GroupVersionKind gets the GroupVersionKind of a TypedReference.
func (obj *TypedReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

// GetObjectKind get the ObjectKind of a TypedReference.
func (obj *TypedReference) GetObjectKind() schema.ObjectKind { return obj }

// TODO(negz): Rename Resource* to Managed* to clarify that they enable the
// resource.Managed interface.

// A ResourceSpec defines the desired state of a managed resource.
type ResourceSpec struct {
	// WriteConnectionSecretToReference specifies the namespace and name of a
	// Secret to which any connection details for this managed resource should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	// This field is planned to be replaced in a future release in favor of
	// PublishConnectionDetailsTo. Currently, both could be set independently
	// and connection details would be published to both without affecting
	// each other.
	// +optional
	WriteConnectionSecretToReference *SecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// PublishConnectionDetailsTo specifies the connection secret config which
	// contains a name, metadata and a reference to secret store config to
	// which any connection details for this managed resource should be written.
	// Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	// +optional
	PublishConnectionDetailsTo *PublishConnectionDetailsTo `json:"publishConnectionDetailsTo,omitempty"`

	// ProviderConfigReference specifies how the provider that will be used to
	// create, observe, update, and delete this managed resource should be
	// configured.
	// +kubebuilder:default={"name": "default"}
	ProviderConfigReference *Reference `json:"providerConfigRef,omitempty"`

	// THIS IS AN ALPHA FIELD. Do not use it in production. It is not honored
	// unless the relevant Crossplane feature flag is enabled, and may be
	// changed or removed without notice.
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
}

// A CredentialsSource is a source from which provider credentials may be
// acquired.
type CredentialsSource string

const (
	// CredentialsSourceNone indicates that a provider does not require
	// credentials.
	CredentialsSourceNone CredentialsSource = "None"

	// CredentialsSourceSecret indicates that a provider should acquire
	// credentials from a secret.
	CredentialsSourceSecret CredentialsSource = "Secret"

	// CredentialsSourceInjectedIdentity indicates that a provider should use
	// credentials via its (pod's) identity; i.e. via IRSA for AWS,
	// Workload Identity for GCP, Pod Identity for Azure, or in-cluster
	// authentication for the Kubernetes API.
	CredentialsSourceInjectedIdentity CredentialsSource = "InjectedIdentity"

	// CredentialsSourceEnvironment indicates that a provider should acquire
	// credentials from an environment variable.
	CredentialsSourceEnvironment CredentialsSource = "Environment"

	// CredentialsSourceFilesystem indicates that a provider should acquire
	// credentials from the filesystem.
	CredentialsSourceFilesystem CredentialsSource = "Filesystem"
)

// CommonCredentialSelectors provides common selectors for extracting
// credentials.
type CommonCredentialSelectors struct {
	// Fs is a reference to a filesystem location that contains credentials that
	// must be used to connect to the provider.
	// +optional
	Fs *FsSelector `json:"fs,omitempty"`

	// Env is a reference to an environment variable that contains credentials
	// that must be used to connect to the provider.
	// +optional
	Env *EnvSelector `json:"env,omitempty"`

	// A SecretRef is a reference to a secret key that contains the credentials
	// that must be used to connect to the provider.
	// +optional
	SecretRef *SecretKeySelector `json:"secretRef,omitempty"`
}

// EnvSelector selects an environment variable.
type EnvSelector struct {
	// Name is the name of an environment variable.
	Name string `json:"name"`
}

// FsSelector selects a filesystem location.
type FsSelector struct {
	// Path is a filesystem path.
	Path string `json:"path"`
}

// A ProviderConfigStatus defines the observed status of a ProviderConfig.
type ProviderConfigStatus struct {
	ConditionedStatus `json:",inline"`

	// Users of this provider configuration.
	Users int64 `json:"users,omitempty"`
}

// A ProviderConfigUsage is a record that a particular managed resource is using
// a particular provider configuration.
type ProviderConfigUsage struct {
	// ProviderConfigReference to the provider config being used.
	ProviderConfigReference Reference `json:"providerConfigRef"`

	// ResourceReference to the managed resource using the provider config.
	ResourceReference TypedReference `json:"resourceRef"`
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
