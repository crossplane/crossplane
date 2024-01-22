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
	Group   = "apiextensions.crossplane.io"
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

// CompositeResourceDefinition type metadata.
var (
	CompositeResourceDefinitionKind             = reflect.TypeOf(CompositeResourceDefinition{}).Name()
	CompositeResourceDefinitionGroupKind        = schema.GroupKind{Group: Group, Kind: CompositeResourceDefinitionKind}.String()
	CompositeResourceDefinitionKindAPIVersion   = CompositeResourceDefinitionKind + "." + SchemeGroupVersion.String()
	CompositeResourceDefinitionGroupVersionKind = SchemeGroupVersion.WithKind(CompositeResourceDefinitionKind)
)

// Composition type metadata.
var (
	CompositionKind             = reflect.TypeOf(Composition{}).Name()
	CompositionGroupKind        = schema.GroupKind{Group: Group, Kind: CompositionKind}.String()
	CompositionKindAPIVersion   = CompositionKind + "." + SchemeGroupVersion.String()
	CompositionGroupVersionKind = SchemeGroupVersion.WithKind(CompositionKind)
)

// CompositionRevision type metadata.
var (
	CompositionRevisionKind             = reflect.TypeOf(CompositionRevision{}).Name()
	CompositionRevisionGroupKind        = schema.GroupKind{Group: Group, Kind: CompositionRevisionKind}.String()
	CompositionRevisionKindAPIVersion   = CompositionRevisionKind + "." + SchemeGroupVersion.String()
	CompositionRevisionGroupVersionKind = SchemeGroupVersion.WithKind(CompositionRevisionKind)
)

func init() {
	SchemeBuilder.Register(&CompositeResourceDefinition{}, &CompositeResourceDefinitionList{})
	SchemeBuilder.Register(&Composition{}, &CompositionList{})
	SchemeBuilder.Register(&CompositionRevision{}, &CompositionRevisionList{})
}
