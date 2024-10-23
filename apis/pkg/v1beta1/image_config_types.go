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
	"knative.dev/pkg/apis"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// MatchType is the method used to match the image.
type MatchType string

const (
	// Prefix is used to match the prefix of the image.
	Prefix MatchType = "Prefix"
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
	Provider string `json:"provider"`
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

// Copied from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L118

// CosignAuthority defines the rules for discovering and validating signatures.
type CosignAuthority struct {
	// Name is the name for this authority.
	// verifications.
	// If not specified, the name will be authority-<index in array>
	Name string `json:"name"`
	// Key defines the type of key to validate the image.
	// +optional
	Key *KeyRef `json:"key,omitempty"`
	// Keyless sets the configuration to verify the authority against a Fulcio
	// instance.
	// +optional
	Keyless *KeylessRef `json:"keyless,omitempty"`
	// Static specifies that signatures / attestations are not validated but
	// instead a static policy is applied against matching images.
	// +optional
	Static *StaticRef `json:"static,omitempty"`
	// Sources sets the configuration to specify the sources from where to
	// consume the signature and attestations.
	// +optional
	Sources []Source `json:"source,omitempty"`
	// CTLog sets the configuration to verify the authority against a Rekor instance.
	// +optional
	CTLog *TLog `json:"ctlog,omitempty"`
	// Attestations is a list of individual attestations for this authority,
	// once the signature for this authority has been verified.
	// +optional
	Attestations []Attestation `json:"attestations,omitempty"`
	// RFC3161Timestamp sets the configuration to verify the signature timestamp against a RFC3161 time-stamping instance.
	// +optional
	RFC3161Timestamp *RFC3161Timestamp `json:"rfc3161timestamp,omitempty"`
}

// Copied with below changes from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L152
//   - Used LocalSecretKeySelector instead of corev1.SecretReference
//     to be consistent with other secret references where we read from the
//     crossplane system namespace. It also includes the key to select to
//     avoid randomly choosing one key different from the policy controller.

// A KeyRef must specify a SecretRef and may specify a HashAlgorithm.
type KeyRef struct {
	// SecretRef sets a reference to a secret with the key.
	SecretRef *LocalSecretKeySelector `json:"secretRef"`
	// Data contains the inline public key
	// +optional
	Data string `json:"data,omitempty"`
	// KMS contains the KMS url of the public key
	// Supported formats differ based on the KMS system used.
	// +optional
	KMS string `json:"kms,omitempty"`
	// HashAlgorithm always defaults to sha256 if the algorithm hasn't been explicitly set
	// +optional
	HashAlgorithm string `json:"hashAlgorithm,omitempty"`
}

// Copied from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L210

// KeylessRef contains location of the validating certificate and the identities
// against which to verify. KeylessRef will contain either the URL to the verifying
// certificate, or it will contain the certificate data inline or in a secret.
type KeylessRef struct {
	// URL defines a url to the keyless instance.
	// +optional
	URL *apis.URL `json:"url,omitempty"`
	// Identities sets a list of identities.
	Identities []Identity `json:"identities"`
	// CACert sets a reference to CA certificate
	// +optional
	CACert *KeyRef `json:"ca-cert,omitempty"` //nolint:tagliatelle // we need to stick to policy controller's tag as it is used in the webhook internal type as well which we rely on: https://github.com/sigstore/policy-controller/blob/dc9960d8c045d360d43c8a03401f3ad7b2357258/pkg/webhook/clusterimagepolicy/clusterimagepolicy_types.go#L116
	// Use the Certificate Chain from the referred TrustRoot.CertificateAuthorities and TrustRoot.CTLog
	// +optional
	TrustRootRef string `json:"trustRootRef,omitempty"`
	// InsecureIgnoreSCT omits verifying if a certificate contains an embedded SCT
	// +optional
	InsecureIgnoreSCT *bool `json:"insecureIgnoreSCT,omitempty"` //nolint:tagliatelle // we need to stick to policy controller's tag as it is used in the webhook internal type as well which we rely on: https://github.com/sigstore/policy-controller/blob/dc9960d8c045d360d43c8a03401f3ad7b2357258/pkg/webhook/clusterimagepolicy/clusterimagepolicy_types.go#L122
}

// Copied from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L322

// Identity may contain the issuer and/or the subject found in the transparency
// log.
// Issuer/Subject uses a strict match, while IssuerRegExp and SubjectRegExp
// apply a regexp for matching.
type Identity struct {
	// Issuer defines the issuer for this identity.
	// +optional
	Issuer string `json:"issuer,omitempty"`
	// Subject defines the subject for this identity.
	// +optional
	Subject string `json:"subject,omitempty"`
	// IssuerRegExp specifies a regular expression to match the issuer for this identity.
	// +optional
	IssuerRegExp string `json:"issuerRegExp,omitempty"`
	// SubjectRegExp specifies a regular expression to match the subject for this identity.
	// +optional
	SubjectRegExp string `json:"subjectRegExp,omitempty"`
}

// Copied from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L170

// StaticRef specifies that signatures / attestations are not validated but
// instead a static policy is applied against matching images.
type StaticRef struct {
	// Action defines how to handle a matching policy.
	Action string `json:"action"`
	// For fail actions, emit an optional custom message. This only makes
	// sense for 'fail' action because on 'pass' there's no place to jot down
	// the message.
	Message string `json:"message,omitempty"`
}

// Copied from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L180

// Source specifies the location of the signature / attestations.
type Source struct {
	// OCI defines the registry from where to pull the signature / attestations.
	// +optional
	OCI string `json:"oci,omitempty"`
	// SignaturePullSecrets is an optional list of references to secrets in the
	// same namespace as the deploying resource for pulling any of the signatures
	// used by this Source.
	// +optional
	SignaturePullSecrets []corev1.LocalObjectReference `json:"signaturePullSecrets,omitempty"`
	// TagPrefix is an optional prefix that signature and attestations have.
	// This is the 'tag based discovery' and in the future once references are
	// fully supported that should likely be the preferred way to handle these.
	// +optional
	TagPrefix *string `json:"tagPrefix,omitempty"`
}

// Copied from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L198

// TLog specifies the URL to a transparency log that holds
// the signature and public key information.
type TLog struct {
	// URL sets the url to the rekor instance (by default the public rekor.sigstore.dev)
	// +optional
	URL *apis.URL `json:"url,omitempty"`
	// Use the Public Key from the referred TrustRoot.TLog
	// +optional
	TrustRootRef string `json:"trustRootRef,omitempty"`
}

// Copied with below changes from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L231
//   - Removed Policy field as it's no in the scope of first iteration.

// Attestation defines the type of attestation to validate and optionally
// apply a policy decision to it. Authority block is used to verify the
// specified attestation types, and if Policy is specified, then it's applied
// only after the validation of the Attestation signature has been verified.
type Attestation struct {
	// Name of the attestation. These can then be referenced at the CIP level
	// policy.
	Name string `json:"name"`
	// PredicateType defines which predicate type to verify. Matches cosign
	// verify-attestation options.
	PredicateType string `json:"predicateType"`
}

// Copied from https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L337-L343

// RFC3161Timestamp specifies the URL to a RFC3161 time-stamping server that holds
// the time-stamped verification for the signature.
type RFC3161Timestamp struct {
	// Use the Certificate Chain from the referred TrustRoot.TimeStampAuthorities
	// +optional
	TrustRootRef string `json:"trustRootRef,omitempty"`
}

// A LocalSecretKeySelector is a reference to a secret key in a predefined
// namespace.
type LocalSecretKeySelector struct {
	xpv1.LocalSecretReference `json:",inline"`

	// The key to select.
	Key string `json:"key"`
}
