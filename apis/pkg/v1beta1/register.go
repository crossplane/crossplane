// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "pkg.crossplane.io"
	Version = "v1beta1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds all registered types to the scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// Lock type metadata.
var (
	LockKind             = reflect.TypeOf(Lock{}).Name()
	LockGroupKind        = schema.GroupKind{Group: Group, Kind: LockKind}.String()
	LockKindAPIVersion   = LockKind + "." + SchemeGroupVersion.String()
	LockGroupVersionKind = SchemeGroupVersion.WithKind(LockKind)
)

// Function type metadata.
var (
	FunctionKind             = reflect.TypeOf(Function{}).Name()
	FunctionGroupKind        = schema.GroupKind{Group: Group, Kind: FunctionKind}.String()
	FunctionKindAPIVersion   = FunctionKind + "." + SchemeGroupVersion.String()
	FunctionGroupVersionKind = SchemeGroupVersion.WithKind(FunctionKind)
)

// FunctionRevision type metadata.
var (
	FunctionRevisionKind             = reflect.TypeOf(FunctionRevision{}).Name()
	FunctionRevisionGroupKind        = schema.GroupKind{Group: Group, Kind: FunctionRevisionKind}.String()
	FunctionRevisionKindAPIVersion   = FunctionRevisionKind + "." + SchemeGroupVersion.String()
	FunctionRevisionGroupVersionKind = SchemeGroupVersion.WithKind(FunctionRevisionKind)
)

// DeploymentRuntimeConfig type metadata.
var (
	DeploymentRuntimeConfigKind             = reflect.TypeOf(DeploymentRuntimeConfig{}).Name()
	DeploymentRuntimeConfigGroupKind        = schema.GroupKind{Group: Group, Kind: DeploymentRuntimeConfigKind}.String()
	DeploymentRuntimeConfigKindAPIVersion   = DeploymentRuntimeConfigKind + "." + SchemeGroupVersion.String()
	DeploymentRuntimeConfigGroupVersionKind = SchemeGroupVersion.WithKind(DeploymentRuntimeConfigKind)
)

func init() {
	SchemeBuilder.Register(&Lock{}, &LockList{})
	SchemeBuilder.Register(&Function{}, &FunctionList{})
	SchemeBuilder.Register(&FunctionRevision{}, &FunctionRevisionList{})
	SchemeBuilder.Register(&DeploymentRuntimeConfig{}, &DeploymentRuntimeConfigList{})
}
