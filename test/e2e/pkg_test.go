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
	k8sapiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
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
	// SuitePackageSignatureVerification is the value for the config.LabelTestSuite
	// label to be assigned to tests that should be part of the Signature
	// Verification test suite.
	SuitePackageSignatureVerification = "package-signature-verification"
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
	environment.AddTestSuite(SuitePackageSignatureVerification,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-signature-verification}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuitePackageSignatureVerification, config.TestSuiteDefault},
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
			Assess("LockConditionDependencyResolutionSucceeded",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "lock.yaml", v1beta1.ResolutionSucceeded())). // TODO(ezgidemirel): use ResourceHasConditionWithin instead
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
			Assess("LockConditionDependencyResolutionSucceeded",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "lock.yaml", v1beta1.ResolutionSucceeded())). // TODO(ezgidemirel): use ResourceHasConditionWithin instead
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

// TestUpgradeDependencyVersion tests that a dependency version is upgraded when the parent configuration is updated.
// The packages used in this test are built and pushed manually and the manifests must remain unchanged to ensure
// the test scenario is not broken.Corresponding meta file can be found under
// test/e2e/manifests/pkg/dependency-upgrade/version/package folder.
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
			Assess("ConfigurationIsStillHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-updated.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("LockConditionDependencyResolutionSucceeded",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "lock.yaml", v1beta1.ResolutionSucceeded())). // TODO(ezgidemirel): use ResourceHasConditionWithin instead
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

// TestUpgradeDependencyDigest tests that a dependency digest is upgraded when the parent configuration is updated.
// The packages used in this test are built and pushed manually and the manifests must remain unchanged to ensure
// the test scenario is not broken. Corresponding meta file can be found under
// test/e2e/manifests/pkg/dependency-upgrade/digest/package folder.
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
			Assess("ConfigurationIsStillHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-updated.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("LockConditionDependencyResolutionSucceeded",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "lock.yaml", v1beta1.ResolutionSucceeded())). // TODO(ezgidemirel): use ResourceHasConditionWithin instead
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

// TestUpgradeAlreadyExistsDependency tests that a previously installed dependency is upgraded to the minimal valid version when the parent configuration is updated.
// The packages used in this test are built and pushed manually and the manifests must remain unchanged to ensure
// the test scenario is not broken. Corresponding meta file can be found under
// test/e2e/manifests/pkg/dependency-upgrade/already-exists/package folder.
func TestUpgradeAlreadyExistsDependency(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/dependency-upgrade/already-exists"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a newly installed Configuration updates to existing dependency to the minimal valid version.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageDependencyUpgrades).
			WithSetup("ApplyDependency", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider.yaml"),
			)).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			Assess("ConfigurationIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("ProviderUpgradedToNewVersionAndHealthy", funcs.AllOf(
				funcs.ResourceHasFieldValueWithin(2*time.Minute, &pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "cool-provider"}}, "spec.package", "xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()))).
			Assess("ConfigurationIsStillHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("LockConditionDependencyResolutionSucceeded",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "lock.yaml", v1beta1.ResolutionSucceeded())). // TODO(ezgidemirel): use ResourceHasConditionWithin instead
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			WithTeardown("DeleteRequiredConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration-nop.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-nop.yaml"),
			)).
			WithTeardown("DeleteConfigurationRevision", funcs.AllOf(
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-nop-revision.yaml"),
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

// TestNoValidVersion tests that a Configuration will not become healthy if there is no valid version for its dependency.
// The packages used in this test are built and pushed manually and the manifests must remain unchanged to ensure
// the test scenario is not broken. Corresponding meta file can be found under
// test/e2e/manifests/pkg/dependency-upgrade/no-valid/package folder.
func TestNoValidVersion(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/dependency-upgrade/no-valid"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a Configuration will not become healthy if there is no valid version for its dependency.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageDependencyUpgrades).
			WithSetup("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("RequiredConfigurationIsUnhealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-nop.yaml", pkgv1.UnknownHealth(), pkgv1.Active())).
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			WithTeardown("DeleteRequiredConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration-nop.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-nop.yaml"),
			)).
			WithTeardown("DeleteConfigurationRevision", funcs.AllOf(
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-nop-revision.yaml"),
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

// TestNoDowngrade tests that a Configuration will not become healthy because automatic downgrades are not allowed.
// The packages used in this test are built and pushed manually and the manifests must remain unchanged to ensure
// the test scenario is not broken. Corresponding meta file can be found under
// test/e2e/manifests/pkg/dependency-upgrade/no-downgrade/package folder.
func TestNoDowngrade(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/dependency-upgrade/no-downgrade"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a Configuration will not become healthy and dependency version will not be downgraded even though there is a valid version.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageDependencyUpgrades).
			WithSetup("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("RequiredConfigurationIsUnhealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration-nop.yaml", pkgv1.UnknownHealth(), pkgv1.Active())).
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			WithTeardown("DeleteRequiredConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration-nop.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-nop.yaml"),
			)).
			WithTeardown("DeleteConfigurationRevision", funcs.AllOf(
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-nop-revision.yaml"),
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

// TestImageConfigAuth tests that we can install a private package as a dependency by providing registry pull
// credentials through ImageConfig API.
// The packages used in this test are built and pushed manually and the manifests must remain unchanged to ensure
// the test scenario is not broken. Corresponding meta file can be found at
// test/e2e/manifests/pkg/image-config/authentication/configuration-with-private-dependency/package.
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
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
				// We wait until the configuration revision is gone, otherwise
				// the provider we will be deleting next might come back as a
				// result of the configuration revision being reconciled again.
				funcs.ResourceDeletedWithin(1*time.Minute, &pkgv1.ConfigurationRevision{ObjectMeta: metav1.ObjectMeta{Name: "e2e-configuration-with-private-dependency-e5b6aa4500c3"}}),
			)).
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteProvider", funcs.AllOf(
				funcs.DeleteResources(manifests, "provider.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider.yaml"),
				// Provider is a copy of provider-nop, so waiting until nop
				// CRD is gone is sufficient to ensure the provider completely
				// deleted including all revisions.
				funcs.ResourceDeletedWithin(2*time.Minute, &k8sapiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "nopresources.nop.crossplane.io"}}),
			)).Feature(),
	)
}

// TestImageConfigVerificationWithKey tests that we can verify signature on a configuration when signed with a key.
// The providers used in this test are built and pushed manually with the necessary signatures and attestations, they
// are just a copy of the provider-nop package.
func TestImageConfigVerificationWithKey(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/image-config/signature-verification/with-key"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that we can verify signature on a configuration when signed with a key.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageSignatureVerification).
			WithSetup("ApplyImageConfig", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "image-config.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "image-config.yaml"),
			)).
			WithSetup("ApplyUnsignedPackage", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration-unsigned.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration-unsigned.yaml"),
			)).
			Assess("SignatureVerificationFailed", funcs.AllOf(
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.ConfigurationRevision{ObjectMeta: metav1.ObjectMeta{Name: "e2e-configuration-signed-with-key-e0adba255c20"}}, pkgv1.AwaitingVerification(), pkgv1.VerificationFailed("", nil).WithMessage("")),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.Configuration{ObjectMeta: metav1.ObjectMeta{Name: "e2e-configuration-signed-with-key"}}, pkgv1.Active(), pkgv1.Unhealthy()),
			)).
			Assess("SignatureVerificationSucceeded", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration-signed.yaml"),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.ConfigurationRevision{ObjectMeta: metav1.ObjectMeta{Name: "e2e-configuration-signed-with-key-1765fb139d01"}}, pkgv1.Healthy(), pkgv1.VerificationSucceeded("").WithMessage("")),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.Configuration{ObjectMeta: metav1.ObjectMeta{Name: "e2e-configuration-signed-with-key"}}, pkgv1.Active(), pkgv1.Healthy()),
			)).
			WithTeardown("DeletePackageAndImageConfig", funcs.AllOf(
				funcs.DeleteResources(manifests, "image-config.yaml"),
				funcs.DeleteResources(manifests, "configuration-signed.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration-signed.yaml"),
			)).Feature(),
	)
}

// TestImageConfigVerificationKeyless tests that we can verify signature on a provider when signed keyless.
// The providers used in this test are built and pushed manually with the necessary signatures and attestations, they
// are just a copy of the provider-nop package.
func TestImageConfigVerificationKeyless(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/image-config/signature-verification/keyless"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that we can verify signature on a provider when signed keyless.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageSignatureVerification).
			WithSetup("ApplyImageConfig", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "image-config.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "image-config.yaml"),
			)).
			WithSetup("ApplyUnsignedPackage", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-unsigned.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider-unsigned.yaml"),
			)).
			Assess("SignatureVerificationFailed", funcs.AllOf(
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "e2e-provider-signed-keyless-552a394a8acc"}}, pkgv1.AwaitingVerification(), pkgv1.VerificationFailed("", nil).WithMessage("")),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "e2e-provider-signed-keyless"}}, pkgv1.Active(), pkgv1.Unhealthy()),
			)).
			Assess("SignatureVerificationSucceeded", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-signed.yaml"),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "e2e-provider-signed-keyless-37f3300ebfa7"}}, pkgv1.Healthy(), pkgv1.VerificationSucceeded("").WithMessage("")),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "e2e-provider-signed-keyless"}}, pkgv1.Active(), pkgv1.Healthy()),
			)).
			WithTeardown("DeletePackageAndImageConfig", funcs.AllOf(
				funcs.DeleteResources(manifests, "image-config.yaml"),
				funcs.DeleteResources(manifests, "provider-signed.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-signed.yaml"),
				// Providers are a copy of provider-nop, so waiting until nop
				// CRD is gone is sufficient to ensure the provider completely
				// deleted including all revisions.
				funcs.ResourceDeletedWithin(2*time.Minute, &k8sapiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "nopresources.nop.crossplane.io"}}),
			)).Feature(),
	)
}

// TestImageConfigAttestationVerificationPrivateKeyless tests that we can verify signature and attestations on a private
// provider when signed keyless.
// The providers used in this test are built and pushed manually with the necessary signatures and attestations, they
// are just a copy of the provider-nop package.
func TestImageConfigAttestationVerificationPrivateKeyless(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/image-config/signature-verification/keyless-private-with-attestation"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that we can verify signature and attestations on a private provider when signed keyless.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuitePackageSignatureVerification).
			WithSetup("ApplyImageConfig", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "image-config.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "image-config.yaml"),
			)).
			WithSetup("ApplyUnsignedPackage", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-unsigned.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider-unsigned.yaml"),
			)).
			Assess("SignatureVerificationFailed", funcs.AllOf(
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "e2e-private-provider-signed-keyless-552a394a8acc"}}, pkgv1.AwaitingVerification(), pkgv1.VerificationFailed("", nil).WithMessage("")),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "e2e-private-provider-signed-keyless"}}, pkgv1.Active(), pkgv1.Unhealthy()),
			)).
			Assess("SignatureVerificationSucceeded", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-signed.yaml"),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "e2e-private-provider-signed-keyless-37f3300ebfa7"}}, pkgv1.Healthy(), pkgv1.VerificationSucceeded("").WithMessage("")),
				funcs.ResourceHasConditionWithin(2*time.Minute, &pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "e2e-private-provider-signed-keyless"}}, pkgv1.Active(), pkgv1.Healthy()),
			)).
			WithTeardown("DeletePackageAndImageConfig", funcs.AllOf(
				funcs.DeleteResources(manifests, "image-config.yaml"),
				funcs.DeleteResources(manifests, "provider-signed.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-signed.yaml"),
				// Providers are a copy of provider-nop, so waiting until nop
				// CRD is gone is sufficient to ensure the provider completely
				// deleted including all revisions.
				funcs.ResourceDeletedWithin(2*time.Minute, &k8sapiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "nopresources.nop.crossplane.io"}}),
			)).Feature(),
	)
}
