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

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
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

	Items []ImageConfig `json:"items"`
}

// ImageConfigSpec contains the configuration for matching images.
type ImageConfigSpec struct {
	// MatchImages is a list of image matching rules. This ImageConfig will
	// match an image if any one of these rules is satisfied. In the case where
	// multiple ImageConfigs match an image for a given purpose the one with the
	// most specific match will be used. If multiple rules of equal specificity
	// match an arbitrary one will be selected.
	// +kubebuilder:validation:XValidation:rule="size(self) > 0",message="matchImages should have at least one element."
	MatchImages []ImageMatch `json:"matchImages"`
	// Registry is the configuration for the registry.
	// +optional
	Registry *RegistryConfig `json:"registry,omitempty"`
	// Verification contains the configuration for verifying the image.
	// +optional
	Verification *ImageVerification `json:"verification,omitempty"`
	// RewriteImage defines how a matched image's path should be rewritten.
	// +optional
	RewriteImage *ImageRewrite `json:"rewriteImage,omitempty"`
	// Runtime allows configuration of runtime options for the image.
	// +optional
	Runtime *ImageRuntime `json:"runtime,omitempty"`
	// PackageName defines how the package manager should name the Crossplane
	// package object (Provider, Configuration, or Function) that it creates
	// for a matched image, for example when installing a dependency. If it is
	// not specified the name is derived from the image's repository path.
	//
	// The value is a Go template, delimited by << and >> rather than the usual
	// {{ and }} so that it survives Helm chart rendering. A value with no
	// template actions is used as is, so a literal name such as "provider-aws"
	// works and must be a valid DNS label. A template is rendered with the
	// following fields, shown here for the image
	// xpkg.crossplane.io/crossplane-contrib/provider-aws:v1 matched by the
	// prefix xpkg.crossplane.io/crossplane-contrib/:
	//
	//   .Image       xpkg.crossplane.io/crossplane-contrib/provider-aws
	//   .Registry    xpkg.crossplane.io
	//   .Repository  crossplane-contrib/provider-aws
	//   .Path        crossplane-contrib
	//   .Name        provider-aws
	//   .Match       xpkg.crossplane.io/crossplane-contrib/
	//   .Remainder   provider-aws
	//   .ImageConfig the name of this ImageConfig
	//
	// The functions lower, trimPrefix, trimSuffix, and replace are available,
	// for example: << trimPrefix "provider-" .Name >>. The rendered result
	// is normalized to a valid DNS label.
	//
	// Use a literal name only when the matchImages rules select a single
	// package repository. If they match more than one repository every matched
	// package contends for that name, and only the first one installed gets
	// it; use a template instead.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:XValidation:rule="self.contains('<<') || self.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?$')",message="a packageName that is not a template must be a valid DNS label"
	PackageName string `json:"packageName,omitempty"`
}

// ImageMatch defines a rule for matching image.
type ImageMatch struct {
	// Type is the type of match.
	// +optional
	// +kubebuilder:validation:Enum=Prefix
	// +kubebuilder:default=Prefix
	Type MatchType `json:"type,omitempty"`
	// Prefix is the prefix that should be matched. When multiple prefix rules
	// match an image path, the longest one takes precedence.
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
	xpv2.LocalSecretReference `json:",inline"`

	// The key to select.
	Key string `json:"key"`
}

// ImageRewrite defines how an image's path should be rewritten.
type ImageRewrite struct {
	// Prefix is the prefix that will replace the portion of the image's path
	// matched by the prefix in the ImageMatch. If multiple prefixes matched,
	// the longest one will be replaced.
	Prefix string `json:"prefix"`
}

// ImageRuntime allows configuration of runtime options for an image.
type ImageRuntime struct {
	// ConfigReference references a RuntimeConfig resource that will be
	// used to configure the package runtime.
	// +optional
	ConfigReference *RuntimeConfigReference `json:"configRef,omitempty"`
}
