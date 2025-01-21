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

// Dependency is a dependency on another package. A dependency can be of an
// arbitrary API version and kind, but Crossplane expects package dependencies
// to behave like a Crossplane package. Specifically it expects to be able to
// create the dependency and set its spec.package field to a package OCI
// reference.
type Dependency struct {
	// APIVersion of the dependency.
	// +optional
	APIVersion *string `json:"apiVersion,omitempty"`

	// Kind of the dependency.
	// +optional
	Kind *string `json:"kind,omitempty"`

	// Package OCI reference of the dependency. Only used when apiVersion and
	// kind are set.
	// +optional
	Package *string `json:"package,omitempty"`

	// Provider is the name of a Provider package image.
	// +optional
	// Deprecated: Specify an apiVersion and kind instead.
	Provider *string `json:"provider,omitempty"`

	// Configuration is the name of a Configuration package image.
	// +optional
	// Deprecated: Specify an apiVersion, kind, and package instead.
	Configuration *string `json:"configuration,omitempty"`

	// Function is the name of a Function package image.
	// +optional
	// Deprecated: Specify an apiVersion, kind, and package instead.
	Function *string `json:"function,omitempty"`

	// Version is the semantic version constraints of the dependency image.
	Version string `json:"version"`
}
