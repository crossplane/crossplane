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
	Group   = "pkg.crossplane.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds all registered types to the scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// ControllerConfig type metadata.
var (
	ControllerConfigKind             = reflect.TypeOf(ControllerConfig{}).Name()
	ControllerConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ControllerConfigKind}.String()
	ControllerConfigKindAPIVersion   = ControllerConfigKind + "." + SchemeGroupVersion.String()
	ControllerConfigGroupVersionKind = SchemeGroupVersion.WithKind(ControllerConfigKind)
)

func init() {
	SchemeBuilder.Register(&ControllerConfig{}, &ControllerConfigList{})
}
