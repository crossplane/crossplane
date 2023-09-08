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
	"k8s.io/apimachinery/pkg/types"
)

const (
	// LabelKeyOwnerUID is the UID of the owner resource of a connection secret.
	// Kubernetes provides owner/controller references to track ownership of
	// resources including secrets, however, this would only work for in cluster
	// k8s secrets. We opted to use a label for this purpose to be consistent
	// across Secret Store implementations and expect all to support
	// setting/getting labels.
	LabelKeyOwnerUID = "secret.crossplane.io/owner-uid"
)

// PublishConnectionDetailsTo represents configuration of a connection secret.
type PublishConnectionDetailsTo struct {
	// Name is the name of the connection secret.
	Name string `json:"name"`

	// Metadata is the metadata for connection secret.
	// +optional
	Metadata *ConnectionSecretMetadata `json:"metadata,omitempty"`

	// SecretStoreConfigRef specifies which secret store config should be used
	// for this ConnectionSecret.
	// +optional
	// +kubebuilder:default={"name": "default"}
	SecretStoreConfigRef *Reference `json:"configRef,omitempty"`
}

// ConnectionSecretMetadata represents metadata of a connection secret.
// Labels are used to track ownership of connection secrets and has to be
// supported for any secret store implementation.
type ConnectionSecretMetadata struct {
	// Labels are the labels/tags to be added to connection secret.
	// - For Kubernetes secrets, this will be used as "metadata.labels".
	// - It is up to Secret Store implementation for others store types.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are the annotations to be added to connection secret.
	// - For Kubernetes secrets, this will be used as "metadata.annotations".
	// - It is up to Secret Store implementation for others store types.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// Type is the SecretType for the connection secret.
	// - Only valid for Kubernetes Secret Stores.
	// +optional
	Type *corev1.SecretType `json:"type,omitempty"`
}

// SetOwnerUID sets owner object uid label.
func (in *ConnectionSecretMetadata) SetOwnerUID(uid types.UID) {
	if in.Labels == nil {
		in.Labels = map[string]string{}
	}
	in.Labels[LabelKeyOwnerUID] = string(uid)
}

// GetOwnerUID gets owner object uid.
func (in *ConnectionSecretMetadata) GetOwnerUID() string {
	if u, ok := in.Labels[LabelKeyOwnerUID]; ok {
		return u
	}
	return ""
}

// SecretStoreType represents a secret store type.
// +kubebuilder:validation:Enum=Kubernetes;Vault;Plugin
type SecretStoreType string

const (
	// SecretStoreKubernetes indicates that secret store type is
	// Kubernetes. In other words, connection secrets will be stored as K8s
	// Secrets.
	SecretStoreKubernetes SecretStoreType = "Kubernetes"

	// SecretStorePlugin indicates that secret store type is Plugin and will be used with external secret stores.
	SecretStorePlugin SecretStoreType = "Plugin"
)

// SecretStoreConfig represents configuration of a Secret Store.
type SecretStoreConfig struct {
	// Type configures which secret store to be used. Only the configuration
	// block for this store will be used and others will be ignored if provided.
	// Default is Kubernetes.
	// +optional
	// +kubebuilder:default=Kubernetes
	Type *SecretStoreType `json:"type,omitempty"`

	// DefaultScope used for scoping secrets for "cluster-scoped" resources.
	// If store type is "Kubernetes", this would mean the default namespace to
	// store connection secrets for cluster scoped resources.
	// In case of "Vault", this would be used as the default parent path.
	// Typically, should be set as Crossplane installation namespace.
	DefaultScope string `json:"defaultScope"`

	// Kubernetes configures a Kubernetes secret store.
	// If the "type" is "Kubernetes" but no config provided, in cluster config
	// will be used.
	// +optional
	Kubernetes *KubernetesSecretStoreConfig `json:"kubernetes,omitempty"`

	// Plugin configures External secret store as a plugin.
	// +optional
	Plugin *PluginStoreConfig `json:"plugin,omitempty"`
}

// PluginStoreConfig represents configuration of an External Secret Store.
type PluginStoreConfig struct {
	// Endpoint is the endpoint of the gRPC server.
	Endpoint string `json:"endpoint,omitempty"`
	// ConfigRef contains store config reference info.
	ConfigRef Config `json:"configRef,omitempty"`
}

// Config contains store config reference info.
type Config struct {
	// APIVersion of the referenced config.
	APIVersion string `json:"apiVersion"`
	// Kind of the referenced config.
	Kind string `json:"kind"`
	// Name of the referenced config.
	Name string `json:"name"`
}

// KubernetesAuthConfig required to authenticate to a K8s API. It expects
// a "kubeconfig" file to be provided.
type KubernetesAuthConfig struct {
	// Source of the credentials.
	// +kubebuilder:validation:Enum=None;Secret;Environment;Filesystem
	Source CredentialsSource `json:"source"`

	// CommonCredentialSelectors provides common selectors for extracting
	// credentials.
	CommonCredentialSelectors `json:",inline"`
}

// KubernetesSecretStoreConfig represents the required configuration
// for a Kubernetes secret store.
type KubernetesSecretStoreConfig struct {
	// Credentials used to connect to the Kubernetes API.
	Auth KubernetesAuthConfig `json:"auth"`

	// TODO(turkenh): Support additional identities like
	// https://github.com/crossplane-contrib/provider-kubernetes/blob/4d722ef914e6964e80e190317daca9872ae98738/apis/v1alpha1/types.go#L34
}
