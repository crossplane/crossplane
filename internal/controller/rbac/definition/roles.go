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

package definition

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
)

const (
	namePrefix       = "crossplane:composite:"
	nameSuffixSystem = ":aggregate-to-crossplane"
	nameSuffixEdit   = ":aggregate-to-edit"
	nameSuffixView   = ":aggregate-to-view"
	nameSuffixBrowse = ":aggregate-to-browse"

	keyAggregateToSystem = "rbac.crossplane.io/aggregate-to-crossplane"

	keyAggregateToAdmin   = "rbac.crossplane.io/aggregate-to-admin"
	keyAggregateToNSAdmin = "rbac.crossplane.io/aggregate-to-ns-admin"

	keyAggregateToEdit   = "rbac.crossplane.io/aggregate-to-edit"
	keyAggregateToNSEdit = "rbac.crossplane.io/aggregate-to-ns-edit"

	keyAggregateToView   = "rbac.crossplane.io/aggregate-to-view"
	keyAggregateToNSView = "rbac.crossplane.io/aggregate-to-ns-view"

	keyAggregateToBrowse = "rbac.crossplane.io/aggregate-to-browse"

	valTrue = "true"

	suffixStatus     = "/status"
	suffixFinalizers = "/finalizers"
)

//nolint:gochecknoglobals // We treat these as constants.
var (
	verbsAll    = []string{rbacv1.VerbAll}
	verbsEdit   = []string{"create", "update", "patch", "delete", "deletecollection"}
	verbsView   = []string{"get", "list", "watch"}
	verbsBrowse = []string{"get", "list", "watch"}
	verbsUpdate = []string{"update"}
)

// RenderClusterRoles returns ClusterRoles for the supplied XRD.
func RenderClusterRoles(d *v1.CompositeResourceDefinition) []rbacv1.ClusterRole {
	system := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + d.GetName() + nameSuffixSystem,
			Labels: map[string]string{
				keyAggregateToSystem: valTrue,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{d.Spec.Group},
				Resources: []string{
					d.Spec.Names.Plural,
					d.Spec.Names.Plural + suffixStatus,
				},
				Verbs: verbsAll,
			},
			{
				// Crossplane reconciles an XR by creating one or more composed resources.
				// These composed resources are controlled (in the owner reference sense) by the XR.
				// Crossplane needs permission to set finalizers on XRs in order to create resources
				// that block their deletion when the OwnerReferencesPermissionEnforcement admission controller is enabled.
				APIGroups: []string{d.Spec.Group},
				Resources: []string{
					d.Spec.Names.Plural + suffixFinalizers,
				},
				Verbs: verbsUpdate,
			},
		},
	}

	edit := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + d.GetName() + nameSuffixEdit,
			Labels: map[string]string{
				// Cluster-scoped admin inherits edit transitively: the crossplane-admin
				// aggregationRule selects aggregate-to-edit (set in the Helm chart), so
				// the cluster-scoped aggregate-to-admin label is intentionally omitted here.
				keyAggregateToEdit: valTrue,

				// Edit rules still aggregate directly to the namespaced admin role.
				keyAggregateToNSEdit:  valTrue,
				keyAggregateToNSAdmin: valTrue,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{d.Spec.Group},
				Resources: []string{
					d.Spec.Names.Plural,
					d.Spec.Names.Plural + suffixStatus,
				},
				Verbs: verbsEdit,
			},
		},
	}

	view := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + d.GetName() + nameSuffixView,
			Labels: map[string]string{
				// Cluster-scoped edit and admin inherit view transitively via the view role.
				keyAggregateToView: valTrue,

				// View rules still aggregate directly to the namespaced edit and admin roles.
				keyAggregateToNSView:  valTrue,
				keyAggregateToNSEdit:  valTrue,
				keyAggregateToNSAdmin: valTrue,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{d.Spec.Group},
				Resources: []string{
					d.Spec.Names.Plural,
					d.Spec.Names.Plural + suffixStatus,
				},
				Verbs: verbsView,
			},
		},
	}

	browse := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + d.GetName() + nameSuffixBrowse,
			Labels: map[string]string{
				keyAggregateToBrowse: valTrue,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{d.Spec.Group},
				Resources: []string{
					d.Spec.Names.Plural,
					d.Spec.Names.Plural + suffixStatus,
				},
				Verbs: verbsBrowse,
			},
		},
	}

	if d.Spec.ClaimNames != nil {
		system.Rules = append(system.Rules, rbacv1.PolicyRule{
			APIGroups: []string{d.Spec.Group},
			Resources: []string{
				d.Spec.ClaimNames.Plural,
				d.Spec.ClaimNames.Plural + suffixStatus,
			},
			Verbs: verbsAll,
		},
			rbacv1.PolicyRule{
				// Crossplane needs permission to set finalizers on Claims in order to create resources
				// that block their deletion when the OwnerReferencesPermissionEnforcement admission controller is enabled.
				APIGroups: []string{d.Spec.Group},
				Resources: []string{
					d.Spec.ClaimNames.Plural + suffixFinalizers,
				},
				Verbs: verbsUpdate,
			},
		)

		edit.Rules = append(edit.Rules, rbacv1.PolicyRule{
			APIGroups: []string{d.Spec.Group},
			Resources: []string{
				d.Spec.ClaimNames.Plural,
				d.Spec.ClaimNames.Plural + suffixStatus,
			},
			Verbs: verbsEdit,
		})

		view.Rules = append(view.Rules, rbacv1.PolicyRule{
			APIGroups: []string{d.Spec.Group},
			Resources: []string{
				d.Spec.ClaimNames.Plural,
				d.Spec.ClaimNames.Plural + suffixStatus,
			},
			Verbs: verbsView,
		})

		// The browse role only includes composite resources; not claims.
	}

	for _, o := range []metav1.Object{system, edit, view, browse} {
		meta.AddOwnerReference(o, meta.AsController(meta.TypedReferenceTo(d, v1.CompositeResourceDefinitionGroupVersionKind)))
	}

	return []rbacv1.ClusterRole{*system, *edit, *view, *browse}
}
