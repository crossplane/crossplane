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

package v1alpha1

import corev1 "k8s.io/api/core/v1"

// PackageSpec specifies the desired state of a Package.
type PackageSpec struct {
	// Package is the name of the package that is being requested.
	Package string `json:"package"`

	// RevisionActivationPolicy specifies how the package controller should
	// update from one revision to the next. Options are Automatic or Manual.
	// Default is Automatic.
	// +optional
	RevisionActivationPolicy *RevisionActivationPolicy `json:"revisionActivationPolicy,omitempty"`

	// RevisionHistoryLimit dictates how the package controller cleans up old
	// inactive package revisions.
	// Defaults to 1. Can be disabled by explicitly setting to 0.
	// +optional
	RevisionHistoryLimit *int64 `json:"revisionHistoryLimit,omitempty"`

	// PackagePullSecrets are named secrets in the same namespace that can be used
	// to fetch packages from private registries.
	// +optional
	PackagePullSecrets []corev1.LocalObjectReference `json:"packagePullSecrets,omitempty"`

	// PackagePullPolicy defines the pull policy for the package.
	// Default is IfNotPresent.
	// +optional
	PackagePullPolicy *corev1.PullPolicy `json:"packagePullPolicy,omitempty"`
}

// PackageStatus represents the observed state of a Package.
type PackageStatus struct {
	CurrentRevision string `json:"currentRevision,omitempty"`
}
