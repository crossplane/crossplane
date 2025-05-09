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

package v1

import corev1 "k8s.io/api/core/v1"

// RevisionActivationPolicy indicates how a package should activate its
// revisions.
type RevisionActivationPolicy string

// PackageSpec specifies the desired state of a Package.
type PackageSpec struct {
	// Package is the name of the package that is being requested.
	Package string `json:"package"`

	// RevisionActivationPolicy specifies how the package controller should
	// update from one revision to the next. Options are Automatic or Manual.
	// Default is Automatic.
	// +optional
	// +kubebuilder:default=Automatic
	RevisionActivationPolicy *RevisionActivationPolicy `json:"revisionActivationPolicy,omitempty"`

	// RevisionHistoryLimit dictates how the package controller cleans up old
	// inactive package revisions.
	// Defaults to 1. Can be disabled by explicitly setting to 0.
	// +optional
	// +kubebuilder:default=1
	RevisionHistoryLimit *int64 `json:"revisionHistoryLimit,omitempty"`

	// PackagePullSecrets are named secrets in the same namespace that can be used
	// to fetch packages from private registries.
	// +optional
	PackagePullSecrets []corev1.LocalObjectReference `json:"packagePullSecrets,omitempty"`

	// PackagePullPolicy defines the pull policy for the package.
	// Default is IfNotPresent.
	// +optional
	// +kubebuilder:default=IfNotPresent
	PackagePullPolicy *corev1.PullPolicy `json:"packagePullPolicy,omitempty"`

	// IgnoreCrossplaneConstraints indicates to the package manager whether to
	// honor Crossplane version constrains specified by the package.
	// Default is false.
	// +optional
	// +kubebuilder:default=false
	IgnoreCrossplaneConstraints *bool `json:"ignoreCrossplaneConstraints,omitempty"`

	// SkipDependencyResolution indicates to the package manager whether to skip
	// resolving dependencies for a package. Setting this value to true may have
	// unintended consequences.
	// Default is false.
	// +optional
	// +kubebuilder:default=false
	SkipDependencyResolution *bool `json:"skipDependencyResolution,omitempty"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
}

// PackageStatus represents the observed state of a Package.
type PackageStatus struct {
	// CurrentRevision is the name of the current package revision. It will
	// reflect the most up to date revision, whether it has been activated or
	// not.
	CurrentRevision string `json:"currentRevision,omitempty"`

	// CurrentIdentifier is the most recent package source that was used to
	// produce a revision. The package manager uses this field to determine
	// whether to check for package updates for a given source when
	// packagePullPolicy is set to IfNotPresent. Manually removing this field
	// will cause the package manager to check that the current revision is
	// correct for the given package source.
	CurrentIdentifier string `json:"currentIdentifier,omitempty"`

	// AppliedImageConfigRefs records any image configs that were applied in
	// reconciling this package, and what they were used for.
	AppliedImageConfigRefs []ImageConfigRef `json:"appliedImageConfigRefs,omitempty"`

	// ResolvedPackage is the name of the package that was used for version
	// resolution. It may be different from spec.package if the package path was
	// rewritten using an image config.
	ResolvedPackage string `json:"resolvedPackage,omitempty"`
}

// ImageConfigRef is a reference to an image config that indicates how the
// referenced image config was used by the package manager.
type ImageConfigRef struct {
	// Name is the name of the image config.
	Name string `json:"name"`
	// Reason indicates what the image config was used for.
	Reason ImageConfigRefReason `json:"reason"`
}

// ImageConfigRefReason is a reason an image config was used.
type ImageConfigRefReason string

const (
	// ImageConfigReasonSetPullSecret indicates an image config was used to
	// configure the pull secret.
	ImageConfigReasonSetPullSecret ImageConfigRefReason = "SetImagePullSecret"
	// ImageConfigReasonVerify indicates an image config was used to configure
	// image verification.
	ImageConfigReasonVerify ImageConfigRefReason = "VerifyImage"
	// ImageConfigReasonRewrite indicates an image config was used to rewrite
	// the image's path.
	ImageConfigReasonRewrite ImageConfigRefReason = "RewriteImage"
)
