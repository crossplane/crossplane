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

package roles

import (
	"sort"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/roles"
)

const (
	namePrefix = "crossplane:function:"
)

// RenderClusterRoles returns ClusterRoles for the supplied FunctionRevision.
// Functions are invoked via gRPC and don't need direct Kubernetes API access,
// so we only create aggregated roles for human users to interact with the
// resources defined by the function.
func RenderClusterRoles(fr *v1.FunctionRevision, rs []roles.Resource) []rbacv1.ClusterRole {
	// Return early if we have no resources to render roles for.
	if len(rs) == 0 {
		return nil
	}

	// Our list of resources has no guaranteed order, so we sort them in order
	// to ensure we don't reorder our RBAC rules on each update.
	sort.Slice(rs, func(i, j int) bool {
		return rs[i].Plural+rs[i].Group < rs[j].Plural+rs[j].Group
	})

	groups := make([]string, 0) // Allows deterministic iteration over groups.

	resources := make(map[string][]string) // Resources by group.
	for _, r := range rs {
		if _, ok := resources[r.Group]; !ok {
			resources[r.Group] = make([]string, 0)
			groups = append(groups, r.Group)
		}

		resources[r.Group] = append(resources[r.Group], r.Plural, r.Plural+roles.SuffixStatus)
	}

	rules := []rbacv1.PolicyRule{}
	for _, g := range groups {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{g},
			Resources: resources[g],
		})
	}

	edit := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + fr.GetName() + roles.NameSuffixEdit,
			Labels: map[string]string{
				// Edit rules aggregate to the Crossplane ClusterRole too.
				// Crossplane needs access to reconcile all composite resources
				// and composite resource claims.
				roles.KeyAggregateToCrossplane: roles.ValTrue,

				// Edit rules aggregate to admin too. Currently edit and admin
				// differ only in their base roles.
				roles.KeyAggregateToAdmin: roles.ValTrue,

				roles.KeyAggregateToEdit: roles.ValTrue,
			},
		},
		Rules: roles.WithVerbs(rules, roles.VerbsEdit),
	}

	view := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + fr.GetName() + roles.NameSuffixView,
			Labels: map[string]string{
				roles.KeyAggregateToView: roles.ValTrue,
			},
		},
		Rules: roles.WithVerbs(rules, roles.VerbsView),
	}

	clusterRoles := []rbacv1.ClusterRole{*edit, *view}
	for i := range clusterRoles {
		ref := meta.AsController(meta.TypedReferenceTo(fr, v1.FunctionRevisionGroupVersionKind))
		clusterRoles[i].SetOwnerReferences([]metav1.OwnerReference{ref})
	}

	return clusterRoles
}
