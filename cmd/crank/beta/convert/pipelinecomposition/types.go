/*
Copyright 2024 The Crossplane Authors.

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

package pipelinecomposition

import v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

// Input represents the input to the patch-and-transform function
// This struct is copied from function patch and transform, as we can't import it directly
// https://github.com/crossplane-contrib/function-patch-and-transform/blob/main/input/v1beta1/resources.go
type Input struct {
	// PatchSets define a named set of patches that may be included by any
	// resource in this Composition. PatchSets cannot themselves refer to other
	// PatchSets.
	//
	// PatchSets are only used by the "Resources" mode of Composition. They
	// are ignored by other modes.
	// +optional
	PatchSets []v1.PatchSet `json:"patchSets,omitempty"`

	// Environment configures the environment in which resources are rendered.
	//
	// THIS IS AN ALPHA FIELD. Do not use it in production. It is not honored
	// unless the relevant Crossplane feature flag is enabled, and may be
	// changed or removed without notice.
	// +optional
	Environment *v1.EnvironmentConfiguration `json:"environment,omitempty"`

	// Resources is a list of resource templates that will be used when a
	// composite resource referring to this composition is created.
	//
	// Resources are only used by the "Resources" mode of Composition. They are
	// ignored by other modes.
	// +optional
	Resources []v1.ComposedTemplate `json:"resources,omitempty"`
}
