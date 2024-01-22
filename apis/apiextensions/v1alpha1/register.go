// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "apiextensions.crossplane.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds all registered types to scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// EnvironmentConfig type metadata.
var (
	EnvironmentConfigKind             = reflect.TypeOf(EnvironmentConfig{}).Name()
	EnvironmentConfigGroupKind        = schema.GroupKind{Group: Group, Kind: EnvironmentConfigKind}.String()
	EnvironmentConfigKindAPIVersion   = EnvironmentConfigKind + "." + SchemeGroupVersion.String()
	EnvironmentConfigGroupVersionKind = SchemeGroupVersion.WithKind(EnvironmentConfigKind)
)

// Usage type metadata.
var (
	UsageKind             = reflect.TypeOf(Usage{}).Name()
	UsageGroupKind        = schema.GroupKind{Group: Group, Kind: UsageKind}.String()
	UsageKindAPIVersion   = UsageKind + "." + SchemeGroupVersion.String()
	UsageGroupVersionKind = SchemeGroupVersion.WithKind(UsageKind)
)

func init() {
	SchemeBuilder.Register(&EnvironmentConfig{}, &EnvironmentConfigList{})
	SchemeBuilder.Register(&Usage{}, &UsageList{})
}
