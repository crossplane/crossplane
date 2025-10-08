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
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/roles"
)

func TestRenderClusterRoles(t *testing.T) {
	frName := "revised"
	frUID := types.UID("no-you-id")

	ctrl := true
	crCtrlr := metav1.OwnerReference{
		APIVersion:         v1.FunctionRevisionGroupVersionKind.GroupVersion().String(),
		Kind:               v1.FunctionRevisionKind,
		Name:               frName,
		UID:                frUID,
		Controller:         &ctrl,
		BlockOwnerDeletion: &ctrl,
	}

	nameEdit := namePrefix + frName + roles.NameSuffixEdit
	nameView := namePrefix + frName + roles.NameSuffixView

	groupA := "example.org"
	groupB := "example.org"
	groupC := "example.net"

	pluralA := "demonstrations"
	pluralB := "examples"
	pluralC := "examples"

	type args struct {
		fr        *v1.FunctionRevision
		resources []roles.Resource
	}

	cases := map[string]struct {
		reason string
		args   args
		want   []rbacv1.ClusterRole
	}{
		"EmptyResources": {
			reason: "If there are no resources (yet) we should not produce any ClusterRoles.",
			args: args{
				fr:        &v1.FunctionRevision{ObjectMeta: metav1.ObjectMeta{Name: frName, UID: frUID}},
				resources: []roles.Resource{},
			},
		},
		"SingleResource": {
			reason: "A FunctionRevision with a single resource should produce edit and view ClusterRoles.",
			args: args{
				fr: &v1.FunctionRevision{ObjectMeta: metav1.ObjectMeta{Name: frName, UID: frUID}},
				resources: []roles.Resource{
					{
						Group:  groupA,
						Plural: pluralA,
					},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameEdit,
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
						Labels: map[string]string{
							roles.KeyAggregateToCrossplane: roles.ValTrue,
							roles.KeyAggregateToAdmin:      roles.ValTrue,
							roles.KeyAggregateToEdit:       roles.ValTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + roles.SuffixStatus},
							Verbs:     roles.VerbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameView,
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
						Labels: map[string]string{
							roles.KeyAggregateToView: roles.ValTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + roles.SuffixStatus},
							Verbs:     roles.VerbsView,
						},
					},
				},
			},
		},
		"MergeGroups": {
			reason: "A FunctionRevision should merge resources by group to produce the fewest rules possible.",
			args: args{
				fr: &v1.FunctionRevision{ObjectMeta: metav1.ObjectMeta{Name: frName, UID: frUID}},
				resources: []roles.Resource{
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
							roles.KeyAggregateToCrossplane: roles.ValTrue,
							roles.KeyAggregateToAdmin:      roles.ValTrue,
							roles.KeyAggregateToEdit:       roles.ValTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + roles.SuffixStatus, pluralB, pluralB + roles.SuffixStatus},
							Verbs:     roles.VerbsEdit,
						},
						{
							APIGroups: []string{groupC},
							Resources: []string{pluralC, pluralC + roles.SuffixStatus},
							Verbs:     roles.VerbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameView,
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
						Labels: map[string]string{
							roles.KeyAggregateToView: roles.ValTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + roles.SuffixStatus, pluralB, pluralB + roles.SuffixStatus},
							Verbs:     roles.VerbsView,
						},
						{
							APIGroups: []string{groupC},
							Resources: []string{pluralC, pluralC + roles.SuffixStatus},
							Verbs:     roles.VerbsView,
						},
					},
				},
			},
		},
		"SortResources": {
			reason: "Resources should be sorted deterministically to avoid spurious diffs.",
			args: args{
				fr: &v1.FunctionRevision{ObjectMeta: metav1.ObjectMeta{Name: frName, UID: frUID}},
				resources: []roles.Resource{
					{
						Group:  groupC,
						Plural: pluralC,
					},
					{
						Group:  groupA,
						Plural: pluralB,
					},
					{
						Group:  groupA,
						Plural: pluralA,
					},
				},
			},
			want: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameEdit,
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
						Labels: map[string]string{
							roles.KeyAggregateToCrossplane: roles.ValTrue,
							roles.KeyAggregateToAdmin:      roles.ValTrue,
							roles.KeyAggregateToEdit:       roles.ValTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + roles.SuffixStatus, pluralB, pluralB + roles.SuffixStatus},
							Verbs:     roles.VerbsEdit,
						},
						{
							APIGroups: []string{groupC},
							Resources: []string{pluralC, pluralC + roles.SuffixStatus},
							Verbs:     roles.VerbsEdit,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nameView,
						OwnerReferences: []metav1.OwnerReference{crCtrlr},
						Labels: map[string]string{
							roles.KeyAggregateToView: roles.ValTrue,
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{groupA},
							Resources: []string{pluralA, pluralA + roles.SuffixStatus, pluralB, pluralB + roles.SuffixStatus},
							Verbs:     roles.VerbsView,
						},
						{
							APIGroups: []string{groupC},
							Resources: []string{pluralC, pluralC + roles.SuffixStatus},
							Verbs:     roles.VerbsView,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RenderClusterRoles(tc.args.fr, tc.args.resources)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nRenderClusterRoles(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestClusterRolesDiffer(t *testing.T) {
	cases := map[string]struct {
		reason  string
		current *rbacv1.ClusterRole
		desired *rbacv1.ClusterRole
		want    bool
	}{
		"Equal": {
			reason: "Two ClusterRoles with the same labels and rules should not differ.",
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"cool": "true"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"example.org"},
						Resources: []string{"examples"},
						Verbs:     []string{"*"},
					},
				},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"cool": "true"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"example.org"},
						Resources: []string{"examples"},
						Verbs:     []string{"*"},
					},
				},
			},
			want: false,
		},
		"LabelsDiffer": {
			reason: "Two ClusterRoles with different labels should differ.",
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"cool": "true"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"example.org"},
						Resources: []string{"examples"},
						Verbs:     []string{"*"},
					},
				},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"cool": "false"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"example.org"},
						Resources: []string{"examples"},
						Verbs:     []string{"*"},
					},
				},
			},
			want: true,
		},
		"RulesDiffer": {
			reason: "Two ClusterRoles with different rules should differ.",
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"cool": "true"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"example.org"},
						Resources: []string{"examples"},
						Verbs:     []string{"*"},
					},
				},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"cool": "true"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"example.com"},
						Resources: []string{"examples"},
						Verbs:     []string{"get"},
					},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := roles.ClusterRolesDiffer(tc.current, tc.desired)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nClusterRolesDiffer(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithVerbs(t *testing.T) {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"example.org"},
			Resources: []string{"examples"},
		},
	}

	verbs := []string{"get", "list", "watch"}
	got := roles.WithVerbs(rules, verbs)

	want := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"example.org"},
			Resources: []string{"examples"},
			Verbs:     verbs,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("roles.WithVerbs(...): -want, +got:\n%s", diff)
	}

	// Ensure original rules are not modified
	if len(rules[0].Verbs) != 0 {
		t.Errorf("roles.WithVerbs should not modify original rules")
	}
}

func TestDefinedResources(t *testing.T) {
	cases := map[string]struct {
		reason string
		refs   []runtime.Object
		want   []roles.Resource
	}{
		"CRD": {
			reason: "A CRD reference should be converted to a Resource.",
			refs: []runtime.Object{
				&metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.k8s.io/v1",
						Kind:       "CustomResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "examples.example.org",
					},
				},
			},
			want: []roles.Resource{
				{
					Group:  "example.org",
					Plural: "examples",
				},
			},
		},
		"InvalidName": {
			reason: "A CRD reference with an invalid name should be ignored.",
			refs: []runtime.Object{
				&metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.k8s.io/v1",
						Kind:       "CustomResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid",
					},
				},
			},
			want: []roles.Resource{},
		},
		"NonCRD": {
			reason: "A non-CRD reference should be ignored.",
			refs: []runtime.Object{
				&metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			want: []roles.Resource{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Convert runtime.Objects to TypedReferences
			refs := make([]metav1.PartialObjectMetadata, 0, len(tc.refs))
			for _, obj := range tc.refs {
				refs = append(refs, *obj.(*metav1.PartialObjectMetadata))
			}

			got := roles.DefinedResources(convertToTypedRefs(refs))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nDefinedResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// Helper function to convert PartialObjectMetadata to TypedReferences.
func convertToTypedRefs(objs []metav1.PartialObjectMetadata) []xpv1.TypedReference {
	refs := make([]xpv1.TypedReference, 0, len(objs))
	for _, obj := range objs {
		refs = append(refs, xpv1.TypedReference{
			APIVersion: obj.APIVersion,
			Kind:       obj.Kind,
			Name:       obj.Name,
		})
	}
	return refs
}
