/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICEE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIO OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespace

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

const (
	nameAdmin = "crossplane-admin"
	nameEdit  = "crossplane-edit"
	nameView  = "crossplane-view"

	keyPrefix = "rbac.crossplane.io/"

	keyAggToAdmin = keyPrefix + "aggregate-to-ns-admin"
	keyAggToEdit  = keyPrefix + "aggregate-to-ns-edit"
	keyAggToView  = keyPrefix + "aggregate-to-ns-view"

	keyBaseOfAdmin = keyPrefix + "base-of-ns-admin"
	keyBaseOfEdit  = keyPrefix + "base-of-ns-edit"
	keyBaseOfView  = keyPrefix + "base-of-ns-view"

	keyXRD = keyPrefix + "xrd"

	keyAggregated = "aggregated-by-crossplane"

	valTrue   = "true"
	valAccept = "xrd-claim-accepted"
)

// RenderRoles for the supplied namespace by aggregating rules from the supplied
// cluster roles.
func RenderRoles(ns *corev1.Namespace, crs []rbacv1.ClusterRole) []rbacv1.Role {
	// Our list of CRs has no guaranteed order, so we sort them in order to
	// ensure we don't reorder our RBAC rules on each update.
	sort.Slice(crs, func(i, j int) bool { return crs[i].GetName() < crs[j].GetName() })

	admin := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns.GetName(),
			Name:        nameAdmin,
			Annotations: map[string]string{keyPrefix + keyAggregated: valTrue},
		},
	}
	edit := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns.GetName(),
			Name:        nameEdit,
			Annotations: map[string]string{keyPrefix + keyAggregated: valTrue},
		},
	}
	view := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns.GetName(),
			Name:        nameView,
			Annotations: map[string]string{keyPrefix + keyAggregated: valTrue},
		},
	}

	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	meta.AddOwnerReference(admin, meta.AsController(meta.TypedReferenceTo(ns, gvk)))
	meta.AddOwnerReference(edit, meta.AsController(meta.TypedReferenceTo(ns, gvk)))
	meta.AddOwnerReference(view, meta.AsController(meta.TypedReferenceTo(ns, gvk)))

	accepts := map[string]bool{}
	for k, v := range ns.GetAnnotations() {
		if strings.HasPrefix(k, keyPrefix) && v == valAccept {
			accepts[strings.TrimPrefix(k, keyPrefix)] = true
		}
	}

	acrs := crSelector{keyAggToAdmin, keyBaseOfAdmin, accepts}
	ecrs := crSelector{keyAggToEdit, keyBaseOfEdit, accepts}
	vcrs := crSelector{keyAggToView, keyBaseOfView, accepts}

	// TODO(negz): Annotate rendered Roles to indicate which ClusterRoles they
	// are aggregating rules from? This aggregation is likely to be surprising
	// to the uninitiated.
	for _, cr := range crs {
		if acrs.Select(cr) {
			admin.Rules = append(admin.Rules, cr.Rules...)
		}

		if ecrs.Select(cr) {
			edit.Rules = append(edit.Rules, cr.Rules...)
		}

		if vcrs.Select(cr) {
			view.Rules = append(view.Rules, cr.Rules...)
		}
	}

	return []rbacv1.Role{*admin, *edit, *view}
}

type crSelector struct {
	keyAgg  string
	keyBase string
	accepts map[string]bool
}

func (s crSelector) Select(cr rbacv1.ClusterRole) bool {
	l := cr.GetLabels()

	// All cluster roles must have an aggregation key to be selected.
	if l[s.keyAgg] != valTrue {
		return false
	}

	// Cluster roles must either be the base of this role, or pertain to an XRD
	// that this namespace accepts a claim from.
	return l[s.keyBase] == valTrue || s.accepts[l[keyXRD]]
}
