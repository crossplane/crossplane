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

// A ToFieldPathPolicy determines how to patch to a field path.
type ToFieldPathPolicy string

// ToFieldPathPatchPolicy defines the policy for the ToFieldPath in a Patch.
const (
	ToFieldPathPolicyReplace                       ToFieldPathPolicy = "Replace"
	ToFieldPathPolicyMergeObjects                  ToFieldPathPolicy = "MergeObjects"
	ToFieldPathPolicyMergeObjectsAppendArrays      ToFieldPathPolicy = "MergeObjectsAppendArrays"
	ToFieldPathPolicyForceMergeObjects             ToFieldPathPolicy = "ForceMergeObjects"
	ToFieldPathPolicyForceMergeObjectsAppendArrays ToFieldPathPolicy = "ForceMergeObjectsAppendArrays"
)

// PatchPolicy defines the policy for a patch.
type PatchPolicy struct {
	FromFieldPath *v1.FromFieldPathPolicy `json:"fromFieldPath,omitempty"`
	ToFieldPath   *ToFieldPathPolicy      `json:"toFieldPath,omitempty"`
}
