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

package v1beta1

import "github.com/crossplane/crossplane/internal/protection"

// A ResourceConverter converts a Resource to the internal implementation.
//
// goverter:converter
// goverter:name GeneratedResourceConverter
// goverter:output:file ./zz_generated.conversion.go
// +k8s:deepcopy-gen=false
type ResourceConverter interface {
	// goverter:ignore Namespace
	ToInternalResourceRef(in ResourceRef) protection.ResourceRef

	// goverter:ignore Namespace
	ToInternalResourceSelector(in ResourceSelector) protection.ResourceSelector

	ToInternal(in Resource) protection.Resource
	FromInternal(in protection.Resource) Resource
}

// A NamespacedResourceConverter converts a Resource to the internal implementation.
//
// goverter:converter
// goverter:name GeneratedNamespacedResourceConverter
// goverter:output:file ./zz_generated.conversion.go
// goverter:output:package github.com/crossplane/crossplane/apis/protection/v1beta1
// +k8s:deepcopy-gen=false
type NamespacedResourceConverter interface {
	ToInternal(in NamespacedResource) protection.Resource
	FromInternal(in protection.Resource) NamespacedResource
}
