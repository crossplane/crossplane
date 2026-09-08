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
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
)

func TestRenderClusterRoles(t *testing.T) {
	group := "example.org"
	pluralXR := "coolcomposites"
	pluralXRC := "coolclaims"
	name := pluralXR + "." + group
	uid := types.UID("no-you-id")

	ctrl := true
	owner := metav1.OwnerReference{
		APIVersion:         v1.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
		Kind:               v1.CompositeResourceDefinitionKind,
		Name:               name,
		UID:                uid,
		Controller:         &ctrl,
		BlockOwnerDeletion: &ctrl,
	}

	cases := map[string]struct {
		reason string
		d      *v1.CompositeResourceDefinition
		want   []rbacv1.ClusterRole
	}{
		"DoesNotOfferClaim": {
			reason: "An XRD that does not offer a claim should produce ClusterRoles that grant access to only the composite",
			d: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				Spec: v1.CompositeResourceDefinitionSpec{
					Group: group,
					Names: extv1.CustomResourceDefinitionNames{Plural: pluralXR},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixSystem,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToSystem: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixEdit,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToAdmin:   valTrue,
							keyAggregateToNSAdmin: valTrue,
							keyAggregateToEdit:    valTrue,
							keyAggregateToNSEdit:  valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixView,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToView:   valTrue,
							keyAggregateToNSView: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsView,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixBrowse,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToBrowse: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsBrowse,
						},
					},
				},
			},
		},
		"OffersClaim": {
			reason: "An XRD that offers a claim should produce ClusterRoles that grant access to that claim",
			d: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				Spec: v1.CompositeResourceDefinitionSpec{
					Group:      group,
					Names:      extv1.CustomResourceDefinitionNames{Plural: pluralXR},
					ClaimNames: &extv1.CustomResourceDefinitionNames{Plural: pluralXRC},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixSystem,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToSystem: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC, pluralXRC + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixEdit,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToAdmin:   valTrue,
							keyAggregateToNSAdmin: valTrue,
							keyAggregateToEdit:    valTrue,
							keyAggregateToNSEdit:  valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC, pluralXRC + suffixStatus},
							Verbs:     verbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixView,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToView:   valTrue,
							keyAggregateToNSView: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsView,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC, pluralXRC + suffixStatus},
							Verbs:     verbsView,
						},
					},
				},
				{
					// The browse role never includes claims.
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixBrowse,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToBrowse: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsBrowse,
						},
					},
				},
			},
		},
		"ServesScale": {
			reason: "An XRD that serves the scale subresource should produce an edit ClusterRole that grants access to the composite's scale subresource",
			d: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				Spec: v1.CompositeResourceDefinitionSpec{
					Group: group,
					Names: extv1.CustomResourceDefinitionNames{Plural: pluralXR},
					Versions: []v1.CompositeResourceDefinitionVersion{{
						Name:   "v1",
						Served: true,
						Subresources: &v1.CompositeResourceDefinitionVersionSubresources{
							Scale: &extv1.CustomResourceSubresourceScale{
								SpecReplicasPath:   ".spec.replicas",
								StatusReplicasPath: ".status.replicas",
							},
						},
					}},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixSystem,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToSystem: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixEdit,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToAdmin:   valTrue,
							keyAggregateToNSAdmin: valTrue,
							keyAggregateToEdit:    valTrue,
							keyAggregateToNSEdit:  valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR + suffixScale},
							Verbs:     verbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixView,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToView:   valTrue,
							keyAggregateToNSView: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsView,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixBrowse,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToBrowse: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsBrowse,
						},
					},
				},
			},
		},
		"UnservedVersionDeclaresScale": {
			reason: "An XRD that declares the scale subresource only on an unserved version should not grant access to the composite's scale subresource",
			d: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				Spec: v1.CompositeResourceDefinitionSpec{
					Group: group,
					Names: extv1.CustomResourceDefinitionNames{Plural: pluralXR},
					Versions: []v1.CompositeResourceDefinitionVersion{{
						Name:   "v1",
						Served: false,
						Subresources: &v1.CompositeResourceDefinitionVersionSubresources{
							Scale: &extv1.CustomResourceSubresourceScale{
								SpecReplicasPath:   ".spec.replicas",
								StatusReplicasPath: ".status.replicas",
							},
						},
					}},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixSystem,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToSystem: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixEdit,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToAdmin:   valTrue,
							keyAggregateToNSAdmin: valTrue,
							keyAggregateToEdit:    valTrue,
							keyAggregateToNSEdit:  valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixView,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToView:   valTrue,
							keyAggregateToNSView: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsView,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixBrowse,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToBrowse: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsBrowse,
						},
					},
				},
			},
		},
		"OffersClaimAndServesScale": {
			reason: "An XRD that offers a claim and serves the scale subresource should produce an edit ClusterRole that grants access to the composite's and the claim's scale subresources",
			d: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				Spec: v1.CompositeResourceDefinitionSpec{
					Group:      group,
					Names:      extv1.CustomResourceDefinitionNames{Plural: pluralXR},
					ClaimNames: &extv1.CustomResourceDefinitionNames{Plural: pluralXRC},
					Versions: []v1.CompositeResourceDefinitionVersion{{
						Name:   "v1",
						Served: true,
						Subresources: &v1.CompositeResourceDefinitionVersionSubresources{
							Scale: &extv1.CustomResourceSubresourceScale{
								SpecReplicasPath:   ".spec.replicas",
								StatusReplicasPath: ".status.replicas",
							},
						},
					}},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixSystem,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToSystem: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC, pluralXRC + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixEdit,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToAdmin:   valTrue,
							keyAggregateToNSAdmin: valTrue,
							keyAggregateToEdit:    valTrue,
							keyAggregateToNSEdit:  valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR + suffixScale},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC, pluralXRC + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC + suffixScale},
							Verbs:     verbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixView,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToView:   valTrue,
							keyAggregateToNSView: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsView,
						},
						{
							APIGroups: []string{group},
							Resources: []string{pluralXRC, pluralXRC + suffixStatus},
							Verbs:     verbsView,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namePrefix + name + nameSuffixBrowse,
						OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{
							keyAggregateToBrowse: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{group},
							Resources: []string{pluralXR, pluralXR + suffixStatus},
							Verbs:     verbsBrowse,
						},
					},
				},
			},
		},
	}

	for name, spec := range map[string]v1.CompositeResourceDefinitionSpec{
		"WildcardGroup":   {Group: "*", Names: extv1.CustomResourceDefinitionNames{Plural: pluralXR}},
		"WildcardPlural":  {Group: group, Names: extv1.CustomResourceDefinitionNames{Plural: "*"}},
		"CoreGroup":       {Group: "", Names: extv1.CustomResourceDefinitionNames{Plural: "secrets"}},
		"KubernetesGroup": {Group: "rbac.authorization.k8s.io", Names: extv1.CustomResourceDefinitionNames{Plural: "clusterroles"}},
		"CrossplaneGroup": {Group: "pkg.crossplane.io", Names: extv1.CustomResourceDefinitionNames{Plural: "providerrevisions"}},
	} {
		tc := cases["DoesNotOfferClaim"]
		tc.reason = "Refused definitions must render empty roles to revoke previously granted rules."
		tc.d = tc.d.DeepCopy()
		tc.d.Spec = spec
		tc.want = append([]rbacv1.ClusterRole(nil), tc.want...)
		for i := range tc.want {
			tc.want[i].Rules = nil
		}
		cases[name] = tc
	}

	claim := cases["DoesNotOfferClaim"]
	claim.reason = "An invalid claim plural is skipped without removing valid composite rules."
	claim.d = claim.d.DeepCopy()
	claim.d.Spec.ClaimNames = &extv1.CustomResourceDefinitionNames{Plural: "*"}
	cases["WildcardClaimPlural"] = claim

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RenderClusterRoles(tc.d)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nRenderClusterRoles(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
