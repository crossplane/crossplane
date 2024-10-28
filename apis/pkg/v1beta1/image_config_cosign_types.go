// Copyright 2022 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Note (turkenh): The types in this file are largely derived from
// https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L118
// with the modifications listed below. This approach ensures compatibility with
// the policy controller's API, leveraging their expertise in API design while
// maintaining familiarity for existing users.

package v1beta1

// Modifications over the original policy controller "Authority" type: https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L118
// - Removed the Static, Sources, CTLog and RFC3161Timestamp fields as they are
//   not supported in the initial implementation.
// - Tweaked field descriptions to remove references to unsupported functionality.

// CosignAuthority defines the rules for discovering and validating signatures.
type CosignAuthority struct {
	// Name is the name for this authority.
	Name string `json:"name"`
	// Key defines the type of key to validate the image.
	// +optional
	Key *KeyRef `json:"key,omitempty"`
	// Keyless sets the configuration to verify the authority against a Fulcio
	// instance.
	// +optional
	Keyless *KeylessRef `json:"keyless,omitempty"`
	// Attestations is a list of individual attestations for this authority,
	// once the signature for this authority has been verified.
	// +optional
	Attestations []Attestation `json:"attestations,omitempty"`
}

// Modifications over the original policy controller "KeyRef" type: https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L152
// - Removed the Data and KMS fields as they are not supported in the initial
//   implementation.
// - SecretRef is now a required LocalSecretKeySelector instead of an optional
//   SecretRef to be consistent with other secret references where we read from
//   the crossplane system namespace. It also includes the key to select to
//   avoid randomly choosing one key different from the policy controller.
// - HashAlgorithm is now required and defaults to sha256 if not explicitly set.

// A KeyRef must specify a SecretRef and may specify a HashAlgorithm.
type KeyRef struct {
	// SecretRef sets a reference to a secret with the key.
	SecretRef LocalSecretKeySelector `json:"secretRef"`
	// HashAlgorithm always defaults to sha256 if the algorithm hasn't been explicitly set
	// +kubebuilder:default="sha256"
	HashAlgorithm string `json:"hashAlgorithm"`
}

// Modifications over the original policy controller "KeylessRef" type: https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L210
// - Removed the URL and CACert as they are not supported in the initial
//   implementation.
// - Remove the TrustRootRef field as it is a reference to another resource
//   that policy controller has and not applicable in this context.

// KeylessRef contains location of the validating certificate and the identities
// against which to verify. KeylessRef will contain either the URL to the verifying
// certificate, or it will contain the certificate data inline or in a secret.
type KeylessRef struct {
	// Identities sets a list of identities.
	Identities []Identity `json:"identities"`
	// InsecureIgnoreSCT omits verifying if a certificate contains an embedded SCT
	// +optional
	InsecureIgnoreSCT *bool `json:"insecureIgnoreSCT,omitempty"` //nolint:tagliatelle // to be compatible with policy controller's API
}

// Modifications over the original policy controller "Identity" type: https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L322
// - Enhanced the descriptions for regexp fields to clarify their precedence
//   over the non-regexp fields.

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
	// This has precedence over the Issuer field.
	// +optional
	IssuerRegExp string `json:"issuerRegExp,omitempty"`
	// SubjectRegExp specifies a regular expression to match the subject for this identity.
	// This has precedence over the Subject field.
	// +optional
	SubjectRegExp string `json:"subjectRegExp,omitempty"`
}

// Modifications over the original policy controller "Attestation" type: https://github.com/sigstore/policy-controller/blob/d73e188a4669780af82d3d168f40a6fff438345a/pkg/apis/policy/v1alpha1/clusterimagepolicy_types.go#L210
// - Removed the Policy field as it is not supported in the initial implementation.

// Attestation defines the type of attestation to validate and optionally
// apply a policy decision to it. Authority block is used to verify the
// specified attestation types, and if Policy is specified, then it's applied
// only after the validation of the Attestation signature has been verified.
type Attestation struct {
	// Name of the attestation.
	Name string `json:"name"`
	// PredicateType defines which predicate type to verify. Matches cosign
	// verify-attestation options.
	PredicateType string `json:"predicateType"`
}
