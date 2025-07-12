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

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
)

func TestRenderClusterRoles(t *testing.T) {
	group := "example.org"
	pluralXR := "coolcomposites"
	pluralXRC := "coolclaims"
	name := pluralXR + "." + group
	uid := types.UID("no-you-id")

	ctrl := true
	owner := metav1.OwnerReference{
		APIVersion:         v2.CompositeResourceDefinitionGroupVersionKind.GroupVersion().String(),
		Kind:               v2.CompositeResourceDefinitionKind,
		Name:               name,
		UID:                uid,
		Controller:         &ctrl,
		BlockOwnerDeletion: &ctrl,
	}

	cases := map[string]struct {
		reason string
		d      *v2.CompositeResourceDefinition
		want   []rbacv1.ClusterRole
	}{
		"DoesNotOfferClaim": {
			reason: "An XRD that does not offer a claim should produce ClusterRoles that grant access to only the composite",
			d: &v2.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				Spec: v2.CompositeResourceDefinitionSpec{
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
			d: &v2.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				Spec: v2.CompositeResourceDefinitionSpec{
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RenderClusterRoles(tc.d)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nRenderClusterRoles(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
