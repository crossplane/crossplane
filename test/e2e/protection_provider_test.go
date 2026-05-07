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

package e2e

import (
	"testing"
	"time"

	k8sapiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// SuiteProviderDeletionProtection is the test suite for provider deletion
// protection, which requires the --enable-provider-deletion-protection flag.
const SuiteProviderDeletionProtection = "provider-deletion-protection"

func init() {
	environment.AddTestSuite(SuiteProviderDeletionProtection,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-provider-deletion-protection}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteProviderDeletionProtection, config.TestSuiteDefault},
		}),
	)
}

func TestProviderDeletionProtection(t *testing.T) {
	manifests := "test/e2e/manifests/protection/provider-deletion"

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that a Provider is automatically protected from deletion when it has active managed resources via ClusterUsage.").
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelSize, LabelSizeLarge).
			WithLabel(config.LabelTestSuite, SuiteProviderDeletionProtection).

			// Setup: Install the provider and wait for it to be healthy.
			WithSetup("InstallProvider", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider.yaml"),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).

			// Create a managed resource so the protection controller creates a
			// ClusterUsage protecting the Provider.
			Assess("CreateManagedResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mr.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "mr.yaml"),
			)).

			// Verify that a ClusterUsage with the provider-protection label was
			// created automatically by the protection controller.
			Assess("ClusterUsageCreatedForProvider",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					1,
					func(o k8s.Object) bool {
						cu, ok := o.(*protectionv1beta1.ClusterUsage)
						if !ok {
							return false
						}
						return cu.Spec.Of.Kind == "Provider"
					},
					resources.WithLabelSelector("crossplane.io/provider-protection=true"),
				),
			).

			// Attempt to delete the Provider. The usage webhook should block it.
			Assess("ProviderDeletionBlockedByUsage",
				funcs.DeletionBlockedByUsageWebhook(manifests, "provider.yaml"),
			).

			// Delete the managed resource so the protection controller removes
			// the ClusterUsage.
			Assess("DeleteManagedResource", funcs.AllOf(
				funcs.DeleteResources(manifests, "mr.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "mr.yaml"),
			)).

			// Verify that the ClusterUsage is removed after the managed
			// resource is deleted.
			Assess("ClusterUsageRemovedAfterMRDeletion",
				funcs.ListedResourcesDeletedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					resources.WithLabelSelector("crossplane.io/provider-protection=true"),
				),
			).

			// Cleanup: Delete the Provider and ensure the CRD is removed.
			WithTeardown("DeleteProvider", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "provider.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "provider.yaml"),
				funcs.ResourceDeletedWithin(2*time.Minute, &k8sapiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "nopresources.nop.crossplane.io"},
				}),
			)).
			Feature(),
	)
}
