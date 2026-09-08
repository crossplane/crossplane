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
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	resourcefake "github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"

	apis "github.com/crossplane/crossplane/apis/v2"
	xrdv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/definition"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/grant"
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
			c := fake.NewClientBuilder().WithScheme(scheme).WithIndex(grant.CRDMetadata(), grant.IndexOwnerUID, grant.OwnerUIDs).WithObjects(objects...).Build()
			r := newReconciler(resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)})
			if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(parent)}); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range suffixes {
				got := &rbacv1.ClusterRole{}
				if err := c.Get(ctx, client.ObjectKey{Name: prefix + suffix}, got); err != nil {
					t.Fatal(err)
				}
				var want []rbacv1.PolicyRule
				if suffix == ":system" {
					want = []rbacv1.PolicyRule{{
						APIGroups: []string{"", "coordination.k8s.io", "events.k8s.io"},
						Resources: []string{"secrets", "configmaps", "events", "leases"}, Verbs: []string{"*"},
					}}
				}
				if diff := cmp.Diff(want, got.Rules); diff != "" {
					t.Errorf("role %q rules after reconciliation (-want +got):\n%s", got.Name, diff)
				}
				if !metav1.IsControlledBy(got, parent) {
					t.Errorf("role %q lost its controller reference", got.Name)
				}
			}
		})
	}
}

// Grants follow actual CRD ownership, including inactive revisions, and
// disappear on removal.
func TestProviderGrantsFollowOwnedCRDs(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{apis.AddToScheme, rbacv1.AddToScheme, extv1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	pr := &pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "provider", UID: "revision-uid"}}
	pr.Status.ObjectRefs = []xpv2.TypedReference{{APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition", Name: "gadgets.example.org"}}
	mrd := &v1alpha1.ManagedResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org", UID: "mrd-uid", OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(pr, pkgv1.ProviderRevisionGroupVersionKind))}}}
	unowned := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "gadgets.example.org"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithIndex(grant.CRDMetadata(), grant.IndexOwnerUID, grant.OwnerUIDs).WithObjects(pr, mrd, unowned).Build()
	r := roles.NewReconciler(&resourcefake.Manager{}, roles.WithClientApplicator(resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)}))
	check := func(want []string) {
		t.Helper()
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pr)}); err != nil {
			t.Fatal(err)
		}
		role := &rbacv1.ClusterRole{}
		if err := c.Get(ctx, client.ObjectKey{Name: "crossplane:provider:provider:aggregate-to-edit"}, role); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, rule := range role.Rules {
			for _, g := range rule.APIGroups {
				for _, p := range rule.Resources {
					got = append(got, p+"."+g)
				}
			}
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("grants (-want +got):\n%s", diff)
		}
	}
	check(nil)
	crd := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: mrd.Name, OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.TypedReferenceTo(pr, pkgv1.ProviderRevisionGroupVersionKind))}}}
	if err := c.Create(ctx, crd); err != nil {
		t.Fatal(err)
	}
	check([]string{"widgets.example.org", "widgets/status.example.org"})
	for _, mutate := range []func(*metav1.OwnerReference){
		func(o *metav1.OwnerReference) { o.UID = "old-revision-uid" },
		func(o *metav1.OwnerReference) { o.Name = "another-revision" },
		func(o *metav1.OwnerReference) { o.Kind = "ConfigurationRevision" },
		func(o *metav1.OwnerReference) { o.APIVersion = "unrelated.example.org/v1" },
	} {
		saved := crd.OwnerReferences[0]
		mutate(&crd.OwnerReferences[0])
		if err := c.Update(ctx, crd); err != nil {
			t.Fatal(err)
		}
		check(nil)
		crd.OwnerReferences[0] = saved
	}
	if err := c.Update(ctx, crd); err != nil {
		t.Fatal(err)
	}
	check([]string{"widgets.example.org", "widgets/status.example.org"})
	if err := c.Delete(ctx, crd); err != nil {
		t.Fatal(err)
	}
	check(nil)
}

func TestXRDGrantsRequireOwnedCompositeAndClaimCRDs(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{apis.AddToScheme, rbacv1.AddToScheme, extv1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	xrd := &xrdv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "xwidgets.example.org", UID: "xrd-uid"},
		Spec:       xrdv1.CompositeResourceDefinitionSpec{Group: "example.org", Names: extv1.CustomResourceDefinitionNames{Plural: "xwidgets"}, ClaimNames: &extv1.CustomResourceDefinitionNames{Plural: "widgets"}},
	}
	xr := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: xrd.Name}}
	claim := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithIndex(grant.CRDMetadata(), grant.IndexOwnerUID, grant.OwnerUIDs).WithObjects(xrd, xr, claim).Build()
	r := definition.NewReconciler(&resourcefake.Manager{}, definition.WithClientApplicator(resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)}))
	check := func(wantXR, wantClaim bool) {
		t.Helper()
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(xrd)}); err != nil {
			t.Fatal(err)
		}
		role := &rbacv1.ClusterRole{}
		if err := c.Get(ctx, client.ObjectKey{Name: "crossplane:composite:" + xrd.Name + ":aggregate-to-edit"}, role); err != nil {
			t.Fatal(err)
		}
		var resources []string
		for _, rule := range role.Rules {
			resources = append(resources, rule.Resources...)
		}
		if got := slices.Contains(resources, "xwidgets"); got != wantXR {
			t.Errorf("composite granted = %v, want %v", got, wantXR)
		}
		if got := slices.Contains(resources, "widgets"); got != wantClaim {
			t.Errorf("claim granted = %v, want %v", got, wantClaim)
		}
	}
	check(false, false)
	owner := meta.AsController(meta.TypedReferenceTo(xrd, xrdv1.CompositeResourceDefinitionGroupVersionKind))
	xr.OwnerReferences = []metav1.OwnerReference{owner}
	if err := c.Update(ctx, xr); err != nil {
		t.Fatal(err)
	}
	check(true, false)
	claim.OwnerReferences = []metav1.OwnerReference{owner}
	if err := c.Update(ctx, claim); err != nil {
		t.Fatal(err)
	}
	check(true, true)
	if err := c.Delete(ctx, xr); err != nil {
		t.Fatal(err)
	}
	check(false, true)
	xr.ResourceVersion = ""
	if err := c.Create(ctx, xr); err != nil {
		t.Fatal(err)
	}
	check(true, true)
	if err := c.Delete(ctx, claim); err != nil {
		t.Fatal(err)
	}
	check(true, false)
	if err := c.Delete(ctx, xr); err != nil {
		t.Fatal(err)
	}
	check(false, false)
}

func TestProviderFamilyUsesOwnedCRDs(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{apis.AddToScheme, rbacv1.AddToScheme, extv1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	own := &pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "own", UID: "own-uid", Labels: map[string]string{pkgv1.LabelProviderFamily: "family"}}}
	own.Spec.Package = "registry.example.org/org/provider:v1"
	sibling := own.DeepCopy()
	sibling.Name = "sibling"
	sibling.UID = "sibling-uid"
	sibling.Spec.Package = "registry.example.org/org/other-provider:v1"
	sibling.Status.ObjectRefs = []xpv2.TypedReference{{APIVersion: extv1.SchemeGroupVersion.String(), Kind: "CustomResourceDefinition", Name: "gadgets.example.org"}}
	foreign := own.DeepCopy()
	foreign.Name = "foreign"
	foreign.UID = "foreign-uid"
	foreign.Spec.Package = "registry.example.org/other/provider:v1"
	shared := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org", OwnerReferences: []metav1.OwnerReference{
		meta.AsOwner(meta.TypedReferenceTo(own, pkgv1.ProviderRevisionGroupVersionKind)),
		meta.AsController(meta.TypedReferenceTo(sibling, pkgv1.ProviderRevisionGroupVersionKind)),
	}}}
	familyCRD := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "providerconfigs.example.org", OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(sibling, pkgv1.ProviderRevisionGroupVersionKind))}}}
	foreignCRD := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "foreign.example.org", OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(foreign, pkgv1.ProviderRevisionGroupVersionKind))}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithIndex(grant.CRDMetadata(), grant.IndexOwnerUID, grant.OwnerUIDs).WithObjects(own, sibling, foreign, shared, familyCRD, foreignCRD).Build()
	r := roles.NewReconciler(&resourcefake.Manager{}, roles.WithOrgDiffer(roles.OrgDiffer{}), roles.WithClientApplicator(resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)}))
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(own)}); err != nil {
		t.Fatal(err)
	}
	role := &rbacv1.ClusterRole{}
	if err := c.Get(ctx, client.ObjectKey{Name: "crossplane:provider:own:aggregate-to-edit"}, role); err != nil {
		t.Fatal(err)
	}
	want := []rbacv1.PolicyRule{{APIGroups: []string{"example.org"}, Resources: []string{"providerconfigs", "providerconfigs/status", "widgets", "widgets/status"}, Verbs: []string{"*"}}}
	if diff := cmp.Diff(want, role.Rules); diff != "" {
		t.Errorf("family rules (-want +got):\n%s", diff)
	}
}
