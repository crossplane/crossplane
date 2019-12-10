/*
Copyright 2019 The Crossplane Authors.

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

package stacks

import (
	"fmt"

	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

// Labels used to track ownership across namespaces and scopes.
const (
	// namespace.crossplane.io/{namespace}
	LabelNamespaceFmt = "namespace.crossplane.io/%s"
)

// PersonaRoleName is a helper to ensure the persona role formatting parameters
// are provided consistently
func PersonaRoleName(stack *v1alpha1.Stack, persona string) string {
	const clusterRoleNameFmt = "stack:%s:%s:%s:%s"

	return fmt.Sprintf(clusterRoleNameFmt, stack.GetNamespace(), stack.GetName(), stack.Spec.Version, persona)
}
