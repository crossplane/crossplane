/*
Copyright 2021 The Crossplane Authors.

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

package composite

import v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

// Returns types of patches that are from a composed resource _to_ a composite resource.
func patchTypesToXR() []v1.PatchType {
	return []v1.PatchType{v1.PatchTypeToCompositeFieldPath, v1.PatchTypeCombineToComposite}
}

// Returns types of patches that are _from_ a composite resource to a composed resource.
func patchTypesFromXR() []v1.PatchType {
	return []v1.PatchType{v1.PatchTypeFromCompositeFieldPath, v1.PatchTypeCombineFromComposite}
}

// Returns types of patches that are _from_ the environment to a composed resource
// and vice versa.
func patchTypesFromToEnvironment() []v1.PatchType {
	return []v1.PatchType{
		v1.PatchTypeFromEnvironmentFieldPath,
		v1.PatchTypeCombineFromEnvironment,
		v1.PatchTypeToEnvironmentFieldPath,
		v1.PatchTypeCombineToEnvironment,
	}
}
