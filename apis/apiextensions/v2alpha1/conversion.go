/*
Copyright 2022 The Crossplane Authors.

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
