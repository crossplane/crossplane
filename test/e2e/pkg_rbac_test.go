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

package e2e

import (
	"context"
	"slices"
	"testing"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	k8sapiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// TestProviderSystemRoleGrantsOwnedResources verifies that a provider's system
// ClusterRole grants access to a resource the ProviderRevision owns even when it
// isn't recorded in the revision's status.objectRefs. We simulate that drift by
// creating a CRD with a controller owner reference to the active ProviderRevision
// without it ever being part of the package (so it never enters objectRefs). The
// RBAC manager must still grant access to it, because it derives the system role
// from the CRDs/MRDs the revision actually owns, not only from objectRefs.
func TestProviderSystemRoleGrantsOwnedResources(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/provider-rbac"

	const (
		providerName = "provider-nop-rbac"
		ownedGroup   = "rbac-e2e.crossplane.io"
		ownedPlural  = "ownedresources"
		ownedCRDName = ownedPlural + "." + ownedGroup
	)

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a provider's system ClusterRole grants access to a CRD the ProviderRevision owns even when it isn't recorded in status.objectRefs.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("InstallProvider", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider.yaml"),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreateCRDOwnedByProviderRevision",
				createCRDOwnedByActiveProviderRevision(providerName, ownedCRDName, ownedGroup, ownedPlural)).
			// Only the provider's system ClusterRole grants the (unique) owned
			// resource, so we don't need a label selector to find it.
			Assess("SystemClusterRoleGrantsOwnedResource",
				funcs.ListedResourcesValidatedWithin(2*time.Minute, &rbacv1.ClusterRoleList{}, 1,
					clusterRoleGrantsResource(ownedGroup, ownedPlural),
				)).
			WithTeardown("DeleteOwnedCRD", deleteCustomResourceDefinition(ownedCRDName)).
			WithTeardown("DeleteProvider", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "provider.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "provider.yaml"),
			)).
			Feature(),
	)
}

// clusterRoleGrantsResource returns a validator reporting whether a ClusterRole
// grants the verbs an informer needs - list and watch - on the supplied API
// group and resource. Those are the permissions whose absence causes the
// cache-sync crash this fix addresses, so we assert them rather than mere
// presence of the group/resource.
func clusterRoleGrantsResource(group, plural string) func(o k8s.Object) bool {
	return func(o k8s.Object) bool {
		cr, ok := o.(*rbacv1.ClusterRole)
		if !ok {
			return false
		}
		for _, r := range cr.Rules {
			if !slices.Contains(r.APIGroups, group) || !slices.Contains(r.Resources, plural) {
				continue
			}
			if slices.Contains(r.Verbs, rbacv1.VerbAll) {
				return true
			}
			if slices.Contains(r.Verbs, "list") && slices.Contains(r.Verbs, "watch") {
				return true
			}
		}
		return false
	}
}

// createCRDOwnedByActiveProviderRevision creates a CRD whose controller owner
// reference points at the named provider's active ProviderRevision, simulating a
// resource the revision owns but that isn't recorded in its status.objectRefs.
func createCRDOwnedByActiveProviderRevision(providerName, crdName, group, plural string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		prs := &pkgv1.ProviderRevisionList{}
		if err := c.Client().Resources().List(ctx, prs); err != nil {
			t.Fatalf("cannot list ProviderRevisions: %v", err)
			return ctx
		}

		var pr *pkgv1.ProviderRevision
		for i := range prs.Items {
			rev := &prs.Items[i]
			if rev.GetDesiredState() != pkgv1.PackageRevisionActive {
				continue
			}
			for _, or := range rev.GetOwnerReferences() {
				if or.Name == providerName {
					pr = rev
					break
				}
			}
			if pr != nil {
				break
			}
		}
		if pr == nil {
			t.Fatalf("no active ProviderRevision found for provider %q", providerName)
			return ctx
		}

		crd := &k8sapiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: crdName,
				// Controller owner reference to the ProviderRevision - this is
				// what makes the revision 'own' the CRD without it being part of
				// the package.
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         pkgv1.SchemeGroupVersion.String(),
					Kind:               pkgv1.ProviderRevisionKind,
					Name:               pr.GetName(),
					UID:                pr.GetUID(),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				}},
			},
			Spec: k8sapiextensionsv1.CustomResourceDefinitionSpec{
				Group: group,
				Names: k8sapiextensionsv1.CustomResourceDefinitionNames{
					Plural:   plural,
					Singular: "ownedresource",
					Kind:     "OwnedResource",
					ListKind: "OwnedResourceList",
				},
				Scope: k8sapiextensionsv1.ClusterScoped,
				Versions: []k8sapiextensionsv1.CustomResourceDefinitionVersion{{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
					Schema: &k8sapiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &k8sapiextensionsv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: ptr.To(true),
						},
					},
				}},
			},
		}

		if err := c.Client().Resources().Create(ctx, crd); err != nil {
			t.Fatalf("cannot create CRD %q: %v", crdName, err)
			return ctx
		}

		t.Logf("Created CRD %q owned by ProviderRevision %q", crdName, pr.GetName())
		return ctx
	}
}

// deleteCustomResourceDefinition deletes the named CRD, tolerating its absence.
func deleteCustomResourceDefinition(crdName string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		crd := &k8sapiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}
		if err := c.Client().Resources().Delete(ctx, crd); err != nil {
			if !kerrors.IsNotFound(err) {
				t.Fatalf("cannot delete CRD %q: %v", crdName, err)
			}
			t.Logf("CRD %q already absent", crdName)
		}
		return ctx
	}
}
