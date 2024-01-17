// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

// A RevisionSpecConverter converts a CompositionSpec to the equivalent
// CompositionRevisionSpec.
//
// goverter:converter
// goverter:name GeneratedRevisionSpecConverter
// goverter:extend ConvertRawExtension ConvertResourceQuantity
// goverter:output:file ./zz_generated.conversion.go
// goverter:output:package github.com/crossplane/crossplane/apis/apiextensions/v1
// +k8s:deepcopy-gen=false
type RevisionSpecConverter interface {
	// goverter:ignore Revision
	ToRevisionSpec(in CompositionSpec) CompositionRevisionSpec
	FromRevisionSpec(in CompositionRevisionSpec) CompositionSpec
}

// ConvertRawExtension 'converts' a RawExtension by producing a deepcopy. This
// is necessary because goverter can't convert an embedded runtime.Object.
func ConvertRawExtension(in runtime.RawExtension) runtime.RawExtension {
	out := in.DeepCopy()
	return *out
}

// ConvertResourceQuantity 'converts' a Quantity by producing a deepcopy. This
// is necessary because goverter can't convert a Quantity's unexported fields.
func ConvertResourceQuantity(in *resource.Quantity) *resource.Quantity {
	out := in.DeepCopy()
	return &out
}
