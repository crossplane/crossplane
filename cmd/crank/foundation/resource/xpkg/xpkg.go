/*
Copyright 2023 The Crossplane Authors.

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

// Package xpkg contains the client to get a Crossplane package with all its
// dependencies as a tree of Resource.
package xpkg

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// DependencyOutput defines the output of the dependency tree.
type DependencyOutput string

const (
	// DependencyOutputUnique outputs only unique dependencies.
	DependencyOutputUnique DependencyOutput = "unique"
	// DependencyOutputAll outputs all dependencies.
	DependencyOutputAll DependencyOutput = "all"
	// DependencyOutputNone outputs no dependencies.
	DependencyOutputNone DependencyOutput = "none"
)

// RevisionOutput defines the output of the revision tree.
type RevisionOutput string

const (
	// RevisionOutputActive outputs only active revisions.
	RevisionOutputActive RevisionOutput = "active"
	// RevisionOutputAll outputs all revisions.
	RevisionOutputAll RevisionOutput = "all"
	// RevisionOutputNone outputs no revisions.
	RevisionOutputNone RevisionOutput = "none"
)

// IsPackageType returns true if the GroupKind is a Crossplane package type.
func IsPackageType(gk schema.GroupKind) bool {
	return gk == pkgv1.ProviderGroupVersionKind.GroupKind() ||
		gk == pkgv1.ConfigurationGroupVersionKind.GroupKind() ||
		gk == pkgv1.FunctionGroupVersionKind.GroupKind()
}

// IsPackageRevisionType returns true if the GroupKind is a Crossplane package
// revision type.
func IsPackageRevisionType(gk schema.GroupKind) bool {
	return gk == pkgv1.ConfigurationRevisionGroupVersionKind.GroupKind() ||
		gk == pkgv1.ProviderRevisionGroupVersionKind.GroupKind() ||
		gk == pkgv1.FunctionRevisionGroupVersionKind.GroupKind()
}

// IsPackageRuntimeConfigType returns true if the GroupKind is a Crossplane runtime
// config type.
func IsPackageRuntimeConfigType(gk schema.GroupKind) bool {
	return gk == pkgv1beta1.DeploymentRuntimeConfigGroupVersionKind.GroupKind()
}
