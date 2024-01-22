// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "meta.pkg.crossplane.io"
	Version = "v1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds all registered types to scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// Provider type metadata.
var (
	ProviderKind             = reflect.TypeOf(Provider{}).Name()
	ProviderGroupKind        = schema.GroupKind{Group: Group, Kind: ProviderKind}.String()
	ProviderKindAPIVersion   = ProviderKind + "." + SchemeGroupVersion.String()
	ProviderGroupVersionKind = SchemeGroupVersion.WithKind(ProviderKind)
)

// Configuration type metadata.
var (
	ConfigurationKind             = reflect.TypeOf(Configuration{}).Name()
	ConfigurationGroupKind        = schema.GroupKind{Group: Group, Kind: ConfigurationKind}.String()
	ConfigurationKindAPIVersion   = ConfigurationKind + "." + SchemeGroupVersion.String()
	ConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(ConfigurationKind)
)

func init() {
	SchemeBuilder.Register(&Configuration{})
	SchemeBuilder.Register(&Provider{})
}
