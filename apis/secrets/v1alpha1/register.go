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
	Group   = "secrets.crossplane.io"
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

// StoreConfig type metadata.
var (
	StoreConfigKind             = reflect.TypeOf(StoreConfig{}).Name()
	StoreConfigGroupKind        = schema.GroupKind{Group: Group, Kind: StoreConfigKind}.String()
	StoreConfigKindAPIVersion   = StoreConfigKind + "." + SchemeGroupVersion.String()
	StoreConfigGroupVersionKind = SchemeGroupVersion.WithKind(StoreConfigKind)
)

func init() {
	SchemeBuilder.Register(&StoreConfig{}, &StoreConfigList{})
}
