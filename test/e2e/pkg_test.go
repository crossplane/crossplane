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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaPkg is applied to all features pertaining to packages, (i.e.
// Providers, Configurations, etc).
const LabelAreaPkg = "pkg"

// TestConfigurationPullFromPrivateRegistry tests that a Configuration can be
// installed from a private registry using a package pull secret.
func TestConfigurationPullFromPrivateRegistry(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/configuration/private"

	environment.Test(t,
		features.New(t.Name()).
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

// TestConfigurationWithDependency tests that a Configuration with a dependency
// on a Provider will become healthy when the Provider becomes healthy.
func TestConfigurationWithDependency(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/configuration/dependency"

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("ApplyConfiguration", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			Assess("ConfigurationIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("RequiredProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider-dependency.yaml", pkgv1.Healthy(), pkgv1.Active())).
			// Dependencies are not automatically deleted.
			WithTeardown("DeleteConfiguration", funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
			)).
			WithTeardown("DeleteRequiredProvider", funcs.AllOf(
				funcs.DeleteResources(manifests, "provider-dependency.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-dependency.yaml"),
			)).Feature(),
	)
}

func TestProviderUpgrade(t *testing.T) {
	// Test that we can upgrade a provider to a new version, even when a managed
	// resource has been created.
	manifests := "test/e2e/manifests/pkg/provider"

	environment.Test(t,
		features.New(t.Name()).
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
		features.New(t.Name()).
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
		features.New(t.Name()).
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
				funcs.ResourceHasFieldValueWithin(10*time.Second, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "external-sa", Namespace: namespace}}, "metadata.ownerReferences", nil),
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
