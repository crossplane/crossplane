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

package rbac_test

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	resourcefake "github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"

	apis "github.com/crossplane/crossplane/v2/apis"
	xrdv1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/definition"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/provider/roles"
)

func TestReconcileReplacesStaleRules(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := apis.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pr := &pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "unowned", UID: "unowned"}}
	pr.Status.ObjectRefs = []xpv2.TypedReference{{
		APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition",
		Name: "widgets.example.org",
	}}
	mrd := pr.DeepCopy()
	mrd.Status.ObjectRefs[0].APIVersion = v1alpha1.SchemeGroupVersion.String()
	mrd.Status.ObjectRefs[0].Kind = v1alpha1.ManagedResourceDefinitionKind
	empty := pr.DeepCopy()
	empty.Status.ObjectRefs = nil
	xrd := &xrdv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org", UID: "xrd-uid"},
		Spec: xrdv1.CompositeResourceDefinitionSpec{
			Group: "example.org", Names: extv1.CustomResourceDefinitionNames{Plural: "widgets", Kind: "Widget"},
		},
	}

	for _, parent := range []client.Object{pr, mrd, empty, xrd} {
		name := "XRD"
		prefix := "crossplane:composite:" + parent.GetName()
		suffixes := []string{":aggregate-to-crossplane", ":aggregate-to-edit", ":aggregate-to-view", ":aggregate-to-browse"}
		owner := meta.AsController(meta.TypedReferenceTo(parent, xrdv1.CompositeResourceDefinitionGroupVersionKind))
		newReconciler := func(c resource.ClientApplicator) reconcile.Reconciler {
			return definition.NewReconciler(&resourcefake.Manager{}, definition.WithClientApplicator(c))
		}
		if revision, ok := parent.(*pkgv1.ProviderRevision); ok {
			name = "EmptyRevision"
			if len(revision.Status.ObjectRefs) > 0 {
				name = revision.Status.ObjectRefs[0].Kind
			}
			prefix = "crossplane:provider:" + parent.GetName()
			suffixes = []string{":aggregate-to-edit", ":aggregate-to-view", ":system"}
			owner = meta.AsController(meta.TypedReferenceTo(parent, pkgv1.ProviderRevisionGroupVersionKind))
			newReconciler = func(c resource.ClientApplicator) reconcile.Reconciler {
				return roles.NewReconciler(&resourcefake.Manager{}, roles.WithClientApplicator(c))
			}
		}

		t.Run(name, func(t *testing.T) {
			objects := make([]client.Object, 0, 1+len(suffixes))
			objects = append(objects, parent)
			for _, suffix := range suffixes {
				// Seed roles with rules left by an earlier reconcile, including
				// aggregated and provider system roles.
				objects = append(objects, &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: prefix + suffix, OwnerReferences: []metav1.OwnerReference{owner},
						Labels: map[string]string{"rbac.crossplane.io/aggregate-to-edit": "true"},
					},
					Rules: []rbacv1.PolicyRule{{
						APIGroups: []string{"example.org"},
						Resources: []string{"widgets", "widgets/status"},
						Verbs:     []string{"get", "list", "watch"},
					}},
				})
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			r := newReconciler(resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)})
			if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(parent)}); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range suffixes {
				got := &rbacv1.ClusterRole{}
				if err := c.Get(ctx, client.ObjectKey{Name: prefix + suffix}, got); err != nil {
					t.Fatal(err)
				}
				if len(got.Rules) != 0 {
					t.Errorf("role %q retains rules after reconciliation: %v", got.Name, got.Rules)
				}
				if !metav1.IsControlledBy(got, parent) {
					t.Errorf("role %q lost its controller reference", got.Name)
				}
			}
		})
	}
}
