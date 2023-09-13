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

// Generated from pkg/meta/v1/meta.go by ../hack/duplicate_api_type.sh. DO NOT EDIT.

package v1beta1

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
