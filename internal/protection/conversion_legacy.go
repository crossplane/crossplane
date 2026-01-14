/*
Copyright 2025 The Crossplane Authors.

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

package protection

import "github.com/crossplane/crossplane/v2/apis/apiextensions/v1beta1"

// A LegacyResourceConverter converts a Resource to the internal implementation.
//
// goverter:converter
// goverter:name GeneratedLegacyResourceConverter
// goverter:output:file ./zz_generated.conversion_legacy.go
// +k8s:deepcopy-gen=false
type LegacyResourceConverter interface {
	// goverter:ignore Namespace
	ToInternalResourceRef(in v1beta1.ResourceRef) ResourceRef

	// goverter:ignore Namespace
	ToInternalResourceSelector(in v1beta1.ResourceSelector) ResourceSelector

	ToInternal(in v1beta1.Resource) Resource
	FromInternal(in Resource) v1beta1.Resource
}
