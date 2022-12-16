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
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
)

// A RevisionSpecConverter converts a CompositionSpec to the equivalent
// CompositionRevisionSpec.
//
// goverter:converter
// goverter:name GeneratedRevisionSpecConverter
// goverter:extend ConvertRawExtension
// +k8s:deepcopy-gen=false
type RevisionSpecConverter interface {
	// goverter:ignore Revision
	ToRevisionSpec(in CompositionSpec) v1beta1.CompositionRevisionSpec
	FromRevisionSpec(in v1beta1.CompositionRevisionSpec) CompositionSpec
}

// ConvertRawExtension 'converts' a RawExtension by producing a deepcopy. This
// is necessary because goverter can't convert an embedded runtime.Object.
func ConvertRawExtension(in runtime.RawExtension) runtime.RawExtension {
	out := in.DeepCopy()
	return *out
}
