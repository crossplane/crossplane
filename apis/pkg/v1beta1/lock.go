/*
Copyright 2021 The Crossplane Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// A PackageType is a type of package.
type PackageType string

// Types of packages.
const (
	ConfigurationPackageType PackageType = "Configuration"
	ProviderPackageType      PackageType = "Provider"
	FunctionPackageType      PackageType = "Function"
)

// LockPackage is a package that is in the lock.
type LockPackage struct {
	// Name corresponds to the name of the package revision for this package.
	Name string `json:"name"`

	// APIVersion of the package.
	// +optional
	APIVersion *string `json:"apiVersion,omitempty"`

	// Kind of the package (not the kind of the package revision).
	// +optional
	Kind *string `json:"kind,omitempty"`

	// Type is the type of package.
	// +kubebuilder:validation:Enum=Configuration;Provider;Function
	// +optional
	// Deprecated: Specify an apiVersion and kind instead.
	Type *PackageType `json:"type"`

	// Source is the OCI image name without a tag or digest.
	Source string `json:"source"`

	// Version is the tag or digest of the OCI image.
	Version string `json:"version"`

	// Dependencies are the list of dependencies of this package. The order of
	// the dependencies will dictate the order in which they are resolved.
	Dependencies []Dependency `json:"dependencies"`

	// ParentConstraints is a list of constraints that are passed down from the parent package to the dependency.
	ParentConstraints []string `json:"-"` // NOTE(ezgidemirel): We don't want to expose this field in the API.
}

// Identifier returns the source of a LockPackage.
func (l *LockPackage) Identifier() string {
	return l.Source
}

// GetConstraints returns the version of a LockPackage.
func (l *LockPackage) GetConstraints() string {
	return l.Version
}

// GetParentConstraints returns the parent constraints of a LockPackage.
func (l *LockPackage) GetParentConstraints() []string {
	return l.ParentConstraints
}

// AddParentConstraints appends passed constraints to the existing parent constraints.
func (l *LockPackage) AddParentConstraints(pc []string) {
	l.ParentConstraints = append(l.ParentConstraints, pc...)
}

// A Dependency is a dependency of a package in the lock.
type Dependency struct {
	// Package is the OCI image name without a tag or digest.
	Package string `json:"package"`

	// APIVersion of the package.
	// +optional
	APIVersion *string `json:"apiVersion,omitempty"`

	// Kind of the package (not the kind of the package revision).
	// +optional
	Kind *string `json:"kind,omitempty"`

	// Type is the type of package. Can be either Configuration or Provider.
	// +kubebuilder:validation:Enum=Configuration;Provider;Function
	// +optional
	// Deprecated: Specify an apiVersion and kind instead.
	Type *PackageType `json:"type"`

	// Constraints is a valid semver range or a digest, which will be used to select a valid
	// dependency version.
	Constraints string `json:"constraints"`

	// ParentConstraints is a list of constraints that are passed down from the parent package to the dependency.
	ParentConstraints []string `json:"-"` // NOTE(ezgidemirel): We don't want to expose this field in the API.
}

// Identifier returns a dependency's source.
func (d *Dependency) Identifier() string {
	return d.Package
}

// GetConstraints returns a dependency's constrain.
func (d *Dependency) GetConstraints() string {
	return d.Constraints
}

// GetParentConstraints returns a dependency's parent constraints.
func (d *Dependency) GetParentConstraints() []string {
	return d.ParentConstraints
}

// AddParentConstraints appends passed constraints to the existing parent constraints.
func (d *Dependency) AddParentConstraints(pc []string) {
	d.ParentConstraints = append(d.ParentConstraints, pc...)
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// Lock is the CRD type that tracks package dependencies.
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster
type Lock struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Packages []LockPackage `json:"packages,omitempty"`

	// Status of the Lock.
	Status LockStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LockList contains a list of Lock.
type LockList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Lock `json:"items"`
}

// LockStatus represents the status of the Lock.
type LockStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// GetCondition of this Lock.
func (l *Lock) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return l.Status.GetCondition(ct)
}

// SetConditions of this Lock.
func (l *Lock) SetConditions(c ...xpv1.Condition) {
	l.Status.SetConditions(c...)
}

// CleanConditions removes all conditions.
func (l *Lock) CleanConditions() {
	l.Status.Conditions = []xpv1.Condition{}
}
