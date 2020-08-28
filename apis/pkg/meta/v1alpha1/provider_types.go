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

import (
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderSpec specifies the configuration of a Provider.
type ProviderSpec struct {
	// Configuration for the packaged Provider's controller.
	Controller ControllerSpec `json:"controller"`

	// Semantic version of Crossplane that Provider is compatible with.
	Crossplane *string `json:"crossplane,omitempty"`

	// Dependencies on other packages.
	DependsOn []Dependency `json:"dependsOn,omitempty"`

	// Requests for additional permissions other than those automatically
	// supplied by the CRDs that the Provider installs.
	PermissionRequests []rbac.PolicyRule `json:"permissionRequests,omitempty"`

	// Paths and types to ignore when building package image.
	Ignore []Ignore `json:"ignore,omitempty"`
}

// ControllerSpec specifies the configuration for the packaged Provider
// controller.
type ControllerSpec struct {
	// Image is the packaged Provider controller image.
	Image string `json:"image"`
}

// Dependency is a dependency on another package. One of Provider or Configuration may be supplied.
type Dependency struct {
	// Provider is the name of a Provider package image.
	Provider *string `json:"provider,omitempty"`

	// Configuration is the name of a Configuration package image.
	Configuration *string `json:"configuration,omitempty"`

	// Version is the semantic version of the dependency image.
	Version string `json:"version"`
}

// Ignore contains paths and types that should be ignored during package build.
type Ignore struct {
	// Path is a filesystem path.
	Path string `json:"path"`
}

// +kubebuilder:object:root=true

// A Provider is the description of a Crossplane Provider package.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec"`
}
