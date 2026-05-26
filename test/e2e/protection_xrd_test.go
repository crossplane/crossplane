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

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// SuiteXRDDeletionProtection is the test suite for XRD deletion
// protection, which requires the --enable-xrd-deletion-protection flag.
const SuiteXRDDeletionProtection = "xrd-deletion-protection"

func init() {
	environment.AddTestSuite(SuiteXRDDeletionProtection,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-xrd-deletion-protection}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteXRDDeletionProtection, config.TestSuiteDefault},
		}),
	)
}

func TestXRDDeletionProtection(t *testing.T) {
	manifests := "test/e2e/manifests/protection/xrd-deletion"

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that a CompositeResourceDefinition is automatically protected from deletion when it has active composite resources via ClusterUsage.").
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelSize, LabelSizeLarge).
			WithLabel(config.LabelTestSuite, SuiteXRDDeletionProtection).

			// Setup: Apply the XRD and wait for it to be established.
			WithSetup("CreateXRD", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xrd.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "xrd.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd.yaml", apiextensionsv1.WatchingComposite()),
			)).

			// Create a composite resource so the protection controller creates
			// a ClusterUsage protecting the XRD.
			Assess("CreateCompositeResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "xr.yaml"),
			)).

			// Verify that a ClusterUsage with the xrd-protection label was
			// created automatically by the protection controller.
			Assess("ClusterUsageCreatedForXRD",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					1,
					func(o k8s.Object) bool {
						cu, ok := o.(*protectionv1beta1.ClusterUsage)
						if !ok {
							return false
						}
						return cu.Spec.Of.Kind == "CompositeResourceDefinition"
					},
					resources.WithLabelSelector("crossplane.io/xrd-protection=true"),
				),
			).

			// Attempt to delete the XRD. The usage webhook should block it.
			Assess("XRDDeletionBlockedByUsage",
				funcs.DeletionBlockedByUsageWebhook(manifests, "xrd.yaml"),
			).

			// Delete the composite resource so the protection controller
			// removes the ClusterUsage.
			Assess("DeleteCompositeResource", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			)).

			// Verify that the ClusterUsage is removed after the composite
			// resource is deleted.
			Assess("ClusterUsageRemovedAfterXRDeletion",
				funcs.ListedResourcesDeletedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					resources.WithLabelSelector("crossplane.io/xrd-protection=true"),
				),
			).

			// Cleanup: Delete the XRD and ensure the CRD is removed.
			WithTeardown("DeleteXRD", funcs.AllOf(
				funcs.DeleteResources(manifests, "xrd.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xrd.yaml"),
				funcs.ResourceDeletedWithin(2*time.Minute, &k8sapiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "xprotectiontests.e2e.crossplane.io"},
				}),
			)).
			Feature(),
	)
}

func TestConfigurationDeletionProtection(t *testing.T) {
	manifests := "test/e2e/manifests/protection/configuration-deletion"

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that a Configuration is automatically protected from deletion when its XRD has active composite resources via ClusterUsage.").
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelSize, LabelSizeLarge).
			WithLabel(config.LabelTestSuite, SuiteXRDDeletionProtection).

			// Setup: Install the Configuration and wait for it to be healthy
			// and its XRD to be established.
			WithSetup("InstallConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "xrd.yaml",
					apiextensionsv1.WatchingComposite()),
			)).

			// Create a composite resource so the protection controller creates
			// ClusterUsages protecting both the XRD and the Configuration.
			Assess("CreateCompositeResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "xr.yaml"),
			)).

			// Verify that a ClusterUsage with the xrd-protection label was
			// created automatically by the protection controller.
			Assess("ClusterUsageCreatedForXRD",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					1,
					func(o k8s.Object) bool {
						cu, ok := o.(*protectionv1beta1.ClusterUsage)
						if !ok {
							return false
						}
						return cu.Spec.Of.Kind == "CompositeResourceDefinition"
					},
					resources.WithLabelSelector("crossplane.io/xrd-protection=true"),
				),
			).

			// Verify that a ClusterUsage with the configuration-protection
			// label was created automatically by the protection controller.
			Assess("ClusterUsageCreatedForConfiguration",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					1,
					func(o k8s.Object) bool {
						cu, ok := o.(*protectionv1beta1.ClusterUsage)
						if !ok {
							return false
						}
						return cu.Spec.Of.Kind == "Configuration"
					},
					resources.WithLabelSelector("crossplane.io/configuration-protection=true"),
				),
			).

			// Attempt to delete the Configuration. The usage webhook should
			// block it.
			Assess("ConfigurationDeletionBlockedByUsage",
				funcs.DeletionBlockedByUsageWebhook(manifests, "configuration.yaml"),
			).

			// Delete the composite resource so the protection controller
			// removes the ClusterUsages.
			Assess("DeleteCompositeResource", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			)).

			// Verify that the ClusterUsages are removed after the composite
			// resource is deleted.
			Assess("XRDClusterUsageRemovedAfterXRDeletion",
				funcs.ListedResourcesDeletedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					resources.WithLabelSelector("crossplane.io/xrd-protection=true,apiextensions.crossplane.io/xrd=xmockdatabases.quickstart.crossplane.io"),
				),
			).
			Assess("ConfigurationClusterUsageRemovedAfterXRDeletion",
				funcs.ListedResourcesDeletedWithin(2*time.Minute,
					&protectionv1beta1.ClusterUsageList{},
					resources.WithLabelSelector("crossplane.io/configuration-protection=true,apiextensions.crossplane.io/xrd=xmockdatabases.quickstart.crossplane.io"),
				),
			).

			// Cleanup: Delete the Configuration and ensure the CRD is removed.
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "configuration.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "configuration.yaml"),
				funcs.ResourceDeletedWithin(3*time.Minute, &k8sapiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "xmockdatabases.quickstart.crossplane.io"},
				}),
			)).
			Feature(),
	)
}
