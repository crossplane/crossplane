/*
Copyright 2026 The Crossplane Authors.

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

package grant

import "testing"

func TestPermitted(t *testing.T) {
	cases := map[string]struct {
		group  string
		plural string
		want   bool
	}{
		"CustomResource":       {group: "example.org", plural: "pinballs", want: true},
		"ProviderHelm":         {group: "helm.crossplane.io", plural: "releases", want: true},
		"ProviderKubernetes":   {group: "kubernetes.crossplane.io", plural: "objects", want: true},
		"KubernetesSIG":        {group: "cluster.x-k8s.io", plural: "clusters", want: true},
		"ThirdPartyAPI":        {group: "argoproj.io", plural: "applications", want: true},
		"WildcardGroup":        {group: "*", plural: "pinballs"},
		"WildcardPlural":       {group: "example.org", plural: "*"},
		"WildcardBoth":         {group: "*", plural: "*"},
		"CoreGroup":            {group: "", plural: "secrets"},
		"EmptyPlural":          {group: "example.org", plural: ""},
		"Apps":                 {group: "apps", plural: "deployments"},
		"Batch":                {group: "batch", plural: "jobs"},
		"Extensions":           {group: "extensions", plural: "deployments"},
		"Policy":               {group: "policy", plural: "poddisruptionbudgets"},
		"Autoscaling":          {group: "autoscaling", plural: "horizontalpodautoscalers"},
		"ClusterRoles":         {group: "rbac.authorization.k8s.io", plural: "clusterroles"},
		"ClusterRoleBindings":  {group: "rbac.authorization.k8s.io", plural: "clusterrolebindings"},
		"KubernetesDomain":     {group: "k8s.io", plural: "pinballs"},
		"KubernetesSubdomain":  {group: "future.k8s.io", plural: "pinballs"},
		"CrossplaneExtensions": {group: "apiextensions.crossplane.io", plural: "compositeresourcedefinitions"},
		"CrossplanePackages":   {group: "pkg.crossplane.io", plural: "providerrevisions"},
		"CrossplaneOperations": {group: "ops.crossplane.io", plural: "operations"},
		"CrossplaneProtection": {group: "protection.crossplane.io", plural: "usages"},
		"UppercasePlural":      {group: "example.org", plural: "Pinballs"},
		"DottedPlural":         {group: "example.org", plural: "pinballs.example"},
		"Subresource":          {group: "example.org", plural: "pinballs/status"},
		"WildcardSubresource":  {group: "example.org", plural: "*/status"},
		"UppercaseGroup":       {group: "Example.org", plural: "pinballs"},
		"TrailingDot":          {group: "example.org.", plural: "pinballs"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Permitted(tc.group, tc.plural); got != tc.want {
				t.Errorf("Permitted(%q, %q) = %t, want %t", tc.group, tc.plural, got, tc.want)
			}
		})
	}
}
