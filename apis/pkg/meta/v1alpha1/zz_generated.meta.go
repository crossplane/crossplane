// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Generated from pkg/meta/v1/meta.go by ../hack/duplicate_api_type.sh. DO NOT EDIT.

package v1alpha1

// MetaSpec are fields that every meta package type must implement.
type MetaSpec struct {
	// Semantic version constraints of Crossplane that package is compatible with.
	Crossplane *CrossplaneConstraints `json:"crossplane,omitempty"`

	// Dependencies on other packages.
	DependsOn []Dependency `json:"dependsOn,omitempty"`
}

// CrossplaneConstraints specifies a packages compatibility with Crossplane versions.
type CrossplaneConstraints struct {
	// Semantic version constraints of Crossplane that package is compatible with.
	Version string `json:"version"`
}

// Dependency is a dependency on another package. One of Provider or Configuration may be supplied.
type Dependency struct {
	// Provider is the name of a Provider package image.
	Provider *string `json:"provider,omitempty"`

	// Configuration is the name of a Configuration package image.
	Configuration *string `json:"configuration,omitempty"`

	// Function is the name of a Function package image.
	Function *string `json:"function,omitempty"`

	// Version is the semantic version constraints of the dependency image.
	Version string `json:"version"`
}
