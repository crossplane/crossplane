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

package namespace

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCRSelector(t *testing.T) {
	xrdName := "composites.example.org"

	type fields struct {
		keyAgg  string
		keyBase string
		accepts map[string]bool
	}

	cases := map[string]struct {
		reason string
		fields fields
		cr     rbacv1.ClusterRole
		want   bool
	}{
		"MissingAggregationLabel": {
			reason: "Only ClusterRoles with the aggregation label should be selected",
			fields: fields{
				keyAgg:  keyAggToAdmin,
				keyBase: keyBaseOfAdmin,
				accepts: map[string]bool{xrdName: true},
			},
			cr: rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				keyBaseOfAdmin: valTrue,
			}}},
			want: false,
		},
		"OnlyAggregationLabel": {
			reason: "ClusterRoles must have either the base label or the label of an accepted XRD to be selected",
			fields: fields{
				keyAgg:  keyAggToAdmin,
				keyBase: keyBaseOfAdmin,
				accepts: map[string]bool{xrdName: true},
			},
			cr: rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				keyAggToAdmin: valTrue,
			}}},
			want: false,
		},
		"IsBaseRole": {
			reason: "ClusterRoles with the aggregation and base labels should be selected",
			fields: fields{
				keyAgg:  keyAggToAdmin,
				keyBase: keyBaseOfAdmin,
				accepts: map[string]bool{xrdName: true},
			},
			cr: rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				keyAggToAdmin:  valTrue,
				keyBaseOfAdmin: valTrue,
			}}},
			want: true,
		},
		"IsAcceptedXRDRole": {
			reason: "ClusterRoles with the aggregation and an accepted XRD label should be selected",
			fields: fields{
				keyAgg:  keyAggToAdmin,
				keyBase: keyBaseOfAdmin,
				accepts: map[string]bool{xrdName: true},
			},
			cr: rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				keyAggToAdmin: valTrue,
				keyXRD:        xrdName,
			}}},
			want: true,
		},
		"IsUnknownXRDRole": {
			reason: "ClusterRoles with the aggregation label but an unknown XRD label should be ignored",
			fields: fields{
				keyAgg:  keyAggToAdmin,
				keyBase: keyBaseOfAdmin,
				accepts: map[string]bool{xrdName: true},
			},
			cr: rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				keyAggToAdmin: valTrue,
				keyXRD:        "unknown.example.org", // An XRD we don't accept.
			}}},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			crs := crSelector{tc.fields.keyAgg, tc.fields.keyBase, tc.fields.accepts}
			got := crs.Select(tc.cr)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("crs.Select(...): -want, +got:\n%s\n", diff)
			}
		})
	}
}

func TestRenderClusterRoles(t *testing.T) {
	name := "spacename"
	uid := types.UID("no-you-id")

	ctrl := true
	owner := metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Namespace",
		Name:               name,
		UID:                uid,
		Controller:         &ctrl,
		BlockOwnerDeletion: &ctrl,
	}

	crNameA := "A"
	crNameB := "B"
	crNameC := "C"

	ruleA := rbacv1.PolicyRule{APIGroups: []string{"A"}}
	ruleB := rbacv1.PolicyRule{APIGroups: []string{"B"}}
	ruleC := rbacv1.PolicyRule{APIGroups: []string{"C"}}

	xrdName := "guilty-gear-xrd"

	type args struct {
		ns  *corev1.Namespace
		crs []rbacv1.ClusterRole
	}

	cases := map[string]struct {
		reason string
		args   args
		want   []rbacv1.Role
	}{
		"APlainOldNamespace": {
			reason: "A namespace with no annotations should get admin, edit, and view roles with only base rules, if any exist.",
			args: args{
				ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid}},
				crs: []rbacv1.ClusterRole{
					{
						// This role's rules should be aggregated to the admin role.
						ObjectMeta: metav1.ObjectMeta{
							Name: crNameA,
							Labels: map[string]string{
								keyAggToAdmin:  valTrue,
								keyBaseOfAdmin: valTrue,
							},
						},
						Rules: []rbacv1.PolicyRule{ruleA},
					},
					{
						// This role's rules should also be aggregated to the admin role.
						ObjectMeta: metav1.ObjectMeta{
							Name: crNameB,
							Labels: map[string]string{
								keyAggToAdmin:  valTrue,
								keyBaseOfAdmin: valTrue,
							},
						},
						Rules: []rbacv1.PolicyRule{ruleB},
					},
					{
						// This role doesn't have any interesting labels. It should not be aggregated.
						ObjectMeta: metav1.ObjectMeta{
							Name:   crNameC,
							Labels: map[string]string{},
						},
						Rules: []rbacv1.PolicyRule{ruleC},
					},
				},
			},
			want: []rbacv1.Role{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       name,
						Name:            nameAdmin,
						OwnerReferences: []metav1.OwnerReference{owner},
						Annotations:     map[string]string{keyPrefix + keyAggregated: valTrue},
					},
					Rules: []rbacv1.PolicyRule{ruleA, ruleB},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       name,
						Name:            nameEdit,
						OwnerReferences: []metav1.OwnerReference{owner},
						Annotations:     map[string]string{keyPrefix + keyAggregated: valTrue},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       name,
						Name:            nameView,
						OwnerReferences: []metav1.OwnerReference{owner},
						Annotations:     map[string]string{keyPrefix + keyAggregated: valTrue},
					},
				},
			},
		},
		"ANamespaceThatAcceptsClaims": {
			reason: "A namespace that is annotated to accept claims should get admin, edit, and view roles with base and XRD rules, if they exist.",
			args: args{
				ns: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						UID:         uid,
						Annotations: map[string]string{keyPrefix + xrdName: valAccept},
					},
				},
				crs: []rbacv1.ClusterRole{
					{
						// This role's rules should be aggregated to the admin and edit roles.
						ObjectMeta: metav1.ObjectMeta{
							Name: crNameA,
							Labels: map[string]string{
								keyAggToAdmin:  valTrue,
								keyBaseOfAdmin: valTrue,
								keyAggToEdit:   valTrue,
								keyBaseOfEdit:  valTrue,
							},
						},
						Rules: []rbacv1.PolicyRule{ruleA},
					},
					{
						// This role's rules should also be aggregated to the admin and edit roles.
						ObjectMeta: metav1.ObjectMeta{
							Name: crNameB,
							Labels: map[string]string{
								keyAggToAdmin: valTrue,
								keyAggToEdit:  valTrue,
								keyXRD:        xrdName, // The namespace accepts the claim this XRD offers.
							},
						},
						Rules: []rbacv1.PolicyRule{ruleB},
					},
					{
						// This role's rules should be aggregated to the view role.
						ObjectMeta: metav1.ObjectMeta{
							Name: crNameC,
							Labels: map[string]string{
								keyAggToView: valTrue,
								keyXRD:       xrdName, // The namespace accepts the claim this XRD offers.
							},
						},
						Rules: []rbacv1.PolicyRule{ruleC},
					},
				},
			},
			want: []rbacv1.Role{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       name,
						Name:            nameAdmin,
						OwnerReferences: []metav1.OwnerReference{owner},
						Annotations:     map[string]string{keyPrefix + keyAggregated: valTrue},
					},
					Rules: []rbacv1.PolicyRule{ruleA, ruleB},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       name,
						Name:            nameEdit,
						OwnerReferences: []metav1.OwnerReference{owner},
						Annotations:     map[string]string{keyPrefix + keyAggregated: valTrue},
					},
					Rules: []rbacv1.PolicyRule{ruleA, ruleB},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       name,
						Name:            nameView,
						OwnerReferences: []metav1.OwnerReference{owner},
						Annotations:     map[string]string{keyPrefix + keyAggregated: valTrue},
					},
					Rules: []rbacv1.PolicyRule{ruleC},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RenderRoles(tc.args.ns, tc.args.crs)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nRenderRoles(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
