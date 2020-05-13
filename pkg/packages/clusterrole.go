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

package packages

import (
	"fmt"

	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
)

// Labels used to track ownership across namespaces and scopes.
const (
	// rbac.crossplane.io/aggregate-to-{scope}-{persona}
	// {scope} is namespace or environment and may include "-default"
	// persona is one of admin, edit, or view
	LabelAggregateFmt = "rbac.crossplane.io/aggregate-to-%s-%s"

	// namespace.crossplane.io/{namespace}
	LabelNamespacePrefix = "namespace.crossplane.io/"
	LabelNamespaceFmt    = LabelNamespacePrefix + "%s"

	LabelScope = "crossplane.io/scope"

	// crossplane:ns:{namespace}:{persona}
	NamespaceClusterRoleNameFmt = "crossplane:ns:%s:%s"
)

// Crossplane ClusterRole Scopes
const (
	NamespaceScoped   = "namespace"
	EnvironmentScoped = "environment"
)

// PersonaRoleName is a helper to ensure the persona role formatting parameters
// are provided consistently
func PersonaRoleName(p *v1alpha1.Package, persona string) string {
	const clusterRoleNameFmt = "package:%s:%s:%s:%s"

	return fmt.Sprintf(clusterRoleNameFmt, p.GetNamespace(), p.GetName(), p.Spec.Version, persona)
}
