/*
Copyright 2022 The Crossplane Authors.

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

const (
	// LabelAreaPkg is applied to all features pertaining to packages, (i.e.
	// Providers, Configurations, etc).
	LabelAreaPkg = "pkg"
	// SuitePackageDependencyUpgrades is the value for the config.LabelTestSuite
	// label to be assigned to tests that should be part of the Package Upgrade
	// test suite.
	SuitePackageDependencyUpgrades = "package-dependency-upgrades"
)

func init() {
	environment.AddTestSuite(SuitePackageDependencyUpgrades,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-dependency-version-upgrades}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuitePackageDependencyUpgrades, config.TestSuiteDefault},
		}),
	)
}

func TestConfigurationPullFromPrivateRegistry(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/configuration/private"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a Configuration can be installed from a private registry using a package pull secret.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreateConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "*.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "*.yaml"),
			)).
			Assess("ConfigurationIsHealthy", funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active())).
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "*.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "*.yaml"),
			)).Feature(),
	)
}

func TestConfigurationWithDependency(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/configuration/dependency"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a Configuration with a dependency on a Provider will become healthy when the Provider becomes healthy.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider-dependency.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("ConfigurationIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active())).
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			WithTeardown("DeleteRequiredProvider", funcs.AllOf(
				funcs.DeleteResources(manifests, "provider-dependency.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-dependency.yaml"),
			)).
			WithTeardown("DeleteProviderRevision", funcs.AllOf(
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-revision-dependency.yaml"),
			)).Feature(),
	)
}

func TestProviderUpgrade(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/provider"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that we can upgrade a provider to a new version, even when a managed resource has been created.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("ApplyInitialProvider", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider-initial.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider-initial.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("InitialManagedResourceIsReady", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mr-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "mr-initial.yaml"),
			)).
			Assess("UpgradeProvider", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-upgrade.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider-upgrade.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("UpgradeManagedResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mr-upgrade.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mr-upgrade.yaml", xpv1.Available()),
			)).
			WithTeardown("DeleteUpgradedManagedResource", funcs.AllOf(
				funcs.DeleteResources(manifests, "mr-upgrade.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "mr-upgrade.yaml"),
			)).WithTeardown("DeleteUpgradedProvider", funcs.ResourcesDeletedAfterListedAreGone(1*time.Minute, manifests, "provider-upgrade.yaml", nopList)).Feature(),
	)
}

func TestDeploymentRuntimeConfig(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/deployment-runtime-config"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that custom configurations in the deployment runtime do not disrupt the functionality of the resources, ensuring that deployments, services, and service accounts are created and configured correctly according to the specified runtime settings.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			// Ensure that none of the custom configurations we have made in the
			// deployment runtime configuration are causing any disruptions to
			// the functionality.
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("ClaimHasPatchedField",
				funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim.yaml", "status.coolerField", "I'M COOLER!"),
			).
			Assess("ServiceAccountNamedProperly",
				funcs.ResourceCreatedWithin(10*time.Second, &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ibelieveicanfly",
						Namespace: namespace,
					},
				})).
			Assess("ServiceNamedProperly",
				funcs.ResourceCreatedWithin(10*time.Second, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "letscomplicateitfurther",
						Namespace: namespace,
					},
				})).
			Assess("DeploymentNamedProperly",
				funcs.ResourceCreatedWithin(10*time.Second, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "iamfreetochoose",
						Namespace: namespace,
					},
				})).
			Assess("DeploymentHasSpecFromDeploymentRuntimeConfig", funcs.AllOf(
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.replicas", int64(2)),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.metadata.labels.some-pod-labels", "cool-label"),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.metadata.annotations.some-pod-annotations", "cool-annotation"),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.spec.containers[0].resources.limits.memory", "2Gi"),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.spec.containers[0].resources.limits.memory", "2Gi"),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.spec.containers[0].resources.requests.cpu", "100m"),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.spec.containers[0].volumeMounts[0].name", "shared-volume"),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.spec.containers[1].name", "sidecar"),
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "iamfreetochoose", Namespace: namespace}}, "spec.template.spec.volumes[0].name", "shared-volume"),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestExternallyManagedServiceAccount(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/externally-managed-service-account"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that an externally managed service account is not owned by the deployment while verifying that the deployment correctly references the service account as specified in the runtime configuration.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			// Ensure that none of the custom configurations we have made in the
			// deployment runtime configuration are causing any disruptions to
			// the functionality.
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("ClaimHasPatchedField",
				funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim.yaml", "status.coolerField", "I'M COOLER!"),
			).
			Assess("ExternalServiceAccountIsNotOwned",
				funcs.ResourceHasFieldValueWithin(10*time.Second, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "external-sa", Namespace: namespace}}, "metadata.ownerReferences", funcs.NotFound),
			).
			Assess("DeploymentHasSpecFromDeploymentRuntimeConfig",
				funcs.ResourceHasFieldValueWithin(10*time.Second, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "provider-runtime", Namespace: namespace}}, "spec.template.spec.serviceAccountName", "external-sa"),
			).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestConfigurationWithDigest(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/configuration/digest"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a Configuration with digest which depends on a Provider with digest will become healthy when the Provider becomes healthy").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider-dependency.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("ConfigurationIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active())).
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
				// We wait until the configuration revision is gone, otherwise
				// the provider we will be deleting next might come back as a
				// result of the configuration revision being reconciled again.
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-revision.yaml"),
			)).
			WithTeardown("DeleteRequiredProvider", funcs.AllOf(
				funcs.DeleteResources(manifests, "provider-dependency.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-dependency.yaml"),
			)).
			WithTeardown("DeleteProviderRevision", funcs.AllOf(
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-revision-dependency.yaml"),
			)).Feature(),
	)
}

func TestImageConfigAuth(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/image-config/authentication/configuration-with-private-dependency"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that we can install a private package as a dependency by providing registry pull credentials through ImageConfig API.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("ApplyImageConfig", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "pull-secret.yaml"),
				funcs.ApplyResources(FieldManager, manifests, "image-config.yaml"),
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			Assess("ProviderInstalledAndHealthy", funcs.AllOf(
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("ConfigurationInstalledAndHealthy", funcs.AllOf(
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
				// We wait until the configuration revision is gone, otherwise
				// the provider we will be deleting next might come back as a
				// result of the configuration revision being reconciled again.
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-revision.yaml"),
			)).
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteProvider", funcs.AllOf(
				funcs.DeleteResources(manifests, "provider.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider.yaml"),
			)).Feature(),
	)
}

func TestUpgradeDependencyVersion(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/dependency-upgrade/version"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a Configuration with a dependency on provider with version upgrades when dependency changes to another version.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageDependencyUpgrades).
			WithSetup("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration-initial.yaml"),
			)).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("ConfigurationIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-initial.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("UpdateConfiguration",
				funcs.ApplyResources(FieldManager, manifests, "configuration-updated.yaml")).
			Assess("ProviderUpgradedToNewVersionAndHealthy", funcs.AllOf(
				funcs.ResourceHasFieldValueWithin(2*time.Minute, &pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "crossplane-contrib-provider-nop"}}, "spec.package", "xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()))).
			Assess("ConfigurationStillHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-updated.yaml", pkgv1.Healthy(), pkgv1.Active())).
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration-updated.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-updated.yaml"),
			)).
			WithTeardown("DeleteRequiredProvider", funcs.AllOf(
				funcs.DeleteResources(manifests, "provider.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider.yaml"),
			)).
			WithTeardown("DeleteProviderRevision", funcs.AllOf(
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-revision.yaml"),
			)).Feature(),
	)
}

func TestUpgradeDependencyDigest(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/dependency-upgrade/digest"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a Configuration with a dependency on provider with digest upgrades when dependency changes to another digest.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageDependencyUpgrades).
			WithSetup("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration-initial.yaml"),
			)).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("ConfigurationIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-initial.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("UpdateConfiguration",
				funcs.ApplyResources(FieldManager, manifests, "configuration-updated.yaml")).
			Assess("ProviderUpgradedToNewDigestAndHealthy", funcs.AllOf(
				funcs.ResourceHasFieldValueWithin(2*time.Minute, &pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "crossplane-contrib-provider-nop"}}, "spec.package", "xpkg.upbound.io/crossplane-contrib/provider-nop@sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()))).
			Assess("ConfigurationStillHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-updated.yaml", pkgv1.Healthy(), pkgv1.Active())).
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration-updated.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-updated.yaml"),
			)).
			WithTeardown("DeleteRequiredProvider", funcs.AllOf(
				funcs.DeleteResources(manifests, "provider.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider.yaml"),
			)).
			WithTeardown("DeleteProviderRevision", funcs.AllOf(
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-revision.yaml"),
			)).Feature(),
	)
}
