/*
Copyright 2024 The Crossplane Authors.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// MatchType is the method used to match the image.
type MatchType string

const (
	// Prefix is used to match the prefix of the image.
	Prefix MatchType = "Prefix"
)

// ImageVerificationProvider is the provider that should be used to verify the
// image.
type ImageVerificationProvider string

const (
	// ImageVerificationProviderCosign is the cosign provider that should be
	// used to verify the image.
	ImageVerificationProviderCosign ImageVerificationProvider = "Cosign"
)

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// The ImageConfig resource is used to configure settings for package images.
//
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane}
type ImageConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ImageConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ImageConfigList contains a list of ImageConfig.
type ImageConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImageConfig `json:"items"`
}

// ImageConfigSpec contains the configuration for matching images.
type ImageConfigSpec struct {
	// MatchImages is a list of image matching rules that should be satisfied.
	// +kubebuilder:validation:XValidation:rule="size(self) > 0",message="matchImages should have at least one element."
	MatchImages []ImageMatch `json:"matchImages"`
	// Registry is the configuration for the registry.
	// +optional
	Registry *RegistryConfig `json:"registry,omitempty"`
	// Verification contains the configuration for verifying the image.
	// +optional
	Verification *ImageVerification `json:"verification,omitempty"`
}

// ImageMatch defines a rule for matching image.
type ImageMatch struct {
	// Type is the type of match.
	// +optional
	// +kubebuilder:validation:Enum=Prefix
	// +kubebuilder:default=Prefix
	Type MatchType `json:"type,omitempty"`
	// Prefix is the prefix that should be matched.
	Prefix string `json:"prefix"`
}

// RegistryAuthentication contains the authentication information for a registry.
type RegistryAuthentication struct {
	// PullSecretRef is a reference to a secret that contains the credentials for
	// the registry.
	PullSecretRef corev1.LocalObjectReference `json:"pullSecretRef"`
}

// RegistryConfig contains the configuration for the registry.
type RegistryConfig struct {
	// Authentication is the authentication information for the registry.
	// +optional
	Authentication *RegistryAuthentication `json:"authentication,omitempty"`
}

// ImageVerification contains the configuration for verifying the image.
type ImageVerification struct {
	// Provider is the provider that should be used to verify the image.
	// +kubebuilder:validation:Enum=Cosign
	Provider ImageVerificationProvider `json:"provider"`
	// Cosign is the configuration for verifying the image using cosign.
	// +optional
	Cosign *CosignVerificationConfig `json:"cosign,omitempty"`
}

// CosignVerificationConfig contains the configuration for verifying the image
// using cosign.
type CosignVerificationConfig struct {
	// Authorities defines the rules for discovering and validating signatures.
	Authorities []CosignAuthority `json:"authorities"`
}

// A LocalSecretKeySelector is a reference to a secret key in a predefined
// namespace.
type LocalSecretKeySelector struct {
	xpv1.LocalSecretReference `json:",inline"`

	// The key to select.
	Key string `json:"key"`
}
