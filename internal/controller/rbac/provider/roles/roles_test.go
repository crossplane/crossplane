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

package roles

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func TestRenderClusterRoles(t *testing.T) {
	prName := "revised"
	prUID := types.UID("no-you-id")

	ctrl := true
	crCtrlr := metav1.OwnerReference{
		APIVersion:         v1.ProviderRevisionGroupVersionKind.GroupVersion().String(),
		Kind:               v1.ProviderRevisionKind,
		Name:               prName,
		UID:                prUID,
		Controller:         &ctrl,
		BlockOwnerDeletion: &ctrl,
	}

	nameEdit := namePrefix + prName + nameSuffixEdit
	nameView := namePrefix + prName + nameSuffixView
	nameSystem := SystemClusterRoleName(prName)

	groupA := "example.org"
	groupB := "example.org"
	groupC := "example.net"

	pluralA := "demonstrations"
	pluralB := "examples"
	pluralC := "examples"

	type args struct {
		pr        *v1.ProviderRevision
		resources []Resource
	}

	cases := map[string]struct {
		reason string
		args   args
		want   []rbacv1.ClusterRole
	}{
		"EmptyResources": {
			reason: "If there are no resources (yet) we should not produce any ClusterRoles.",
			args: args{
				pr:        &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: prName, UID: prUID}},
				resources: []Resource{},
			},
		},
		"MergeGroups": {
			reason: "A ProviderRevision should merge resources by group to produce the fewest rules possible.",
			args: args{
				pr: &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: prName, UID: prUID}},
				resources: []Resource{
					{
						Group:  groupA,
						Plural: pluralA,
					},
					{
						Group:  groupB,
						Plural: pluralB,
					},
					{
						Group:  groupC,
						Plural: pluralC,
					},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameEdit,
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
						Labels: map[string]string{
							keyAggregateToCrossplane: valTrue,
							keyAggregateToAdmin:      valTrue,
							keyAggregateToEdit:       valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + suffixStatus, pluralB, pluralB + suffixStatus},
							Verbs:     verbsEdit,
						},
						{
							APIGroups: []string{groupC},
							Resources: []string{pluralC, pluralC + suffixStatus},
							Verbs:     verbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameView,
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
						Labels: map[string]string{
							keyAggregateToView: valTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + suffixStatus, pluralB, pluralB + suffixStatus},
							Verbs:     verbsView,
						},
						{
							APIGroups: []string{groupC},
							Resources: []string{pluralC, pluralC + suffixStatus},
							Verbs:     verbsView,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameSystem,
						Labels:          map[string]string{keyProviderName: prName},
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
					},
					Rules: append([]rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + suffixStatus, pluralB, pluralB + suffixStatus},
							Verbs:     verbsSystem,
						},
						{
							APIGroups: []string{groupC},
							Resources: []string{pluralC, pluralC + suffixStatus},
							Verbs:     verbsSystem,
						},
						{
							APIGroups: []string{groupA, groupC},
							Resources: []string{rbacv1.ResourceAll + suffixFinalizers},
							Verbs:     verbsUpdate,
						},
					}, rulesSystemExtra...),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RenderClusterRoles(tc.args.pr, tc.args.resources)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nRenderClusterRoles(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
