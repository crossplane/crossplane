/*
Copyright 2020 The Crossplane Authors.

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

package provider

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

const (
	namePrefix       = "crossplane:provider:"
	nameSuffixEdit   = ":aggregate-to-edit"
	nameSuffixView   = ":aggregate-to-view"
	nameSuffixSystem = ":system"

	keyAggregateToCrossplane = "rbac.crossplane.io/aggregate-to-crossplane"
	keyAggregateToAdmin      = "rbac.crossplane.io/aggregate-to-admin"
	keyAggregateToEdit       = "rbac.crossplane.io/aggregate-to-edit"
	keyAggregateToView       = "rbac.crossplane.io/aggregate-to-view"

	valTrue = "true"

	suffixStatus = "/status"

	pluralSecrets = "secrets"
)

var (
	verbsEdit   = []string{rbacv1.VerbAll}
	verbsView   = []string{"get", "list", "watch"}
	verbsSystem = []string{"get", "list", "watch", "update", "patch"}
)

// Extra rules that are granted to all provider pods.
// TODO(negz): Extra rules should be requested via a ProviderRevision's (as yet
// unimplemented) PermissionRequests field. In order to do so we must allow the
// RBAC manager to be configured such that it knows which rules may and may not
// be requested.
var rulesSystemExtra = []rbacv1.PolicyRule{
	{
		APIGroups: []string{""},
		Resources: []string{pluralSecrets},
		Verbs:     verbsEdit,
	},
}

// SystemClusterRoleName returns the name of the 'system' cluster role - i.e.
// the role that a provider's ServiceAccount should be bound to.
func SystemClusterRoleName(providerName string) string {
	return namePrefix + providerName + nameSuffixSystem
}

// RenderClusterRoles returns ClusterRoles for the supplied ProviderRevision.
func RenderClusterRoles(pr *v1alpha1.ProviderRevision, crds []v1beta1.CustomResourceDefinition) []rbacv1.ClusterRole {
	c := metav1.GetControllerOf(pr)
	if c == nil {
		// All ProviderRevisions should be controlled by a Provider. If this one
		// is not it should be adopted by a Provider on a future reconcile.
		return nil
	}

	// We're actually reconciling ProviderRevisions, but we want the roles to
	// appear to be for the Provider so we extract its name from our controller
	// reference.
	name := c.Name

	groups := make([]string, 0)            // Allows deterministic iteration over groups.
	resources := make(map[string][]string) // Resources by group.
	for _, crd := range crds {
		if _, ok := resources[crd.Spec.Group]; !ok {
			resources[crd.Spec.Group] = make([]string, 0)
			groups = append(groups, crd.Spec.Group)
		}
		resources[crd.Spec.Group] = append(resources[crd.Spec.Group],
			crd.Spec.Names.Plural,
			crd.Spec.Names.Plural+suffixStatus,
		)
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
			Name: namePrefix + name + nameSuffixEdit,
			Labels: map[string]string{
				// Edit rules aggregate to the Crossplane ClusterRole too.
				// Crossplane needs access to reconcile all composite resources
				// and composite resource claims.
				keyAggregateToCrossplane: valTrue,

				// Edit rules aggregate to admin too. Currently edit and admin
				// differ only in their base roles.
				keyAggregateToAdmin: valTrue,

				keyAggregateToEdit: valTrue,
			},
		},
		Rules: withVerbs(rules, verbsEdit),
	}

	view := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + name + nameSuffixView,
			Labels: map[string]string{
				keyAggregateToView: valTrue,
			},
		},
		Rules: withVerbs(rules, verbsView),
	}

	// The 'system' RBAC role does not aggregate; it is intended to be bound
	// directly to the service account tha provider runs as.
	system := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: SystemClusterRoleName(name)},
		// TODO(negz): Require providers to explicitly ask for access to Secrets
		// via their permissionRequests.
		Rules: append(withVerbs(rules, verbsSystem), rulesSystemExtra...),
	}

	roles := []rbacv1.ClusterRole{*edit, *view, *system}
	for i := range roles {
		// If we're an inactive PackageRevision we should relinquish control of
		// our ClusterRoles by downgrading our controller reference to an owner
		// reference. This ensures that the newly activated revision can gain
		// control of this role. It also ensures that the role is not orphaned
		// if no other revision takes control of it.
		ref := meta.AsOwner(meta.TypedReferenceTo(pr, v1alpha1.ProviderRevisionGroupVersionKind))
		if pr.Spec.DesiredState == v1alpha1.PackageRevisionActive {
			ref = meta.AsController(meta.TypedReferenceTo(pr, v1alpha1.ProviderRevisionGroupVersionKind))
		}
		roles[i].SetOwnerReferences([]metav1.OwnerReference{ref})
	}
	return roles
}

func withVerbs(r []rbacv1.PolicyRule, verbs []string) []rbacv1.PolicyRule {
	verbal := make([]rbacv1.PolicyRule, len(r))
	for i := range r {
		verbal[i] = r[i]
		verbal[i].Verbs = verbs
	}
	return verbal
}
