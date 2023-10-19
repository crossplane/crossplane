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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaLifecycle is applied to all 'features' pertaining to managing
// Crossplane's lifecycle (installing, upgrading, etc).
const LabelAreaLifecycle = "lifecycle"

const TestSuiteLifecycle = "lifecycle"

// TestCrossplaneLifecycle tests two features expecting them to be run in order:
//   - CrossplaneUninstall: Test that it's possible to cleanly uninstall Crossplane, even
//     after having created and deleted a claim.
//   - CrossplaneUpgrade: Test that it's possible to upgrade Crossplane from the most recent
//     stable Helm chart to the one we're testing, even when a claim exists. This
//     expects Crossplane not to be installed.
//
// Note: First time Installation is tested as part of the environment setup,
// if not disabled explicitly.
func TestCrossplaneLifecycle(t *testing.T) {
	manifests := "test/e2e/manifests/lifecycle/upgrade"
	environment.Test(t,
		// Test that it's possible to cleanly uninstall Crossplane, even after
		// having created and deleted a claim.
		features.New(t.Name()+"Uninstall").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithLabel(config.LabelTestSuite, TestSuiteLifecycle).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("XRDAreEstablished", funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite())).
			WithSetup("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			WithSetup("ClaimIsAvailable", funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "claim.yaml"),
			)).
			Assess("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Assess("UninstallCrossplane", funcs.AllOf(
				funcs.AsFeaturesFunc(funcs.HelmUninstall(
					helm.WithName(helmReleaseName),
					helm.WithNamespace(namespace),
				)),
			)).
			// Uninstalling the Crossplane Helm chart doesn't remove its CRDs. We
			// want to make sure they can be deleted cleanly. If they can't, it's a
			// sign something they define might have stuck around.
			WithTeardown("DeleteCrossplaneCRDs", funcs.AllOf(
				funcs.DeleteResources(crdsDir, "*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, crdsDir, "*.yaml"),
			)).
			// Uninstalling the Crossplane Helm chart doesn't remove the namespace
			// it was installed to either. We want to make sure it can be deleted
			// cleanly.
			WithTeardown("DeleteCrossplaneNamespace", funcs.AllOf(
				funcs.AsFeaturesFunc(envfuncs.DeleteNamespace(namespace)),
				funcs.ResourceDeletedWithin(3*time.Minute, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}),
			)).
			Feature(),
		features.New(t.Name()+"Upgrade").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			// We expect Crossplane to have been uninstalled first
			Assess("CrossplaneIsNotInstalled", funcs.AllOf(
				funcs.ResourceDeletedWithin(1*time.Minute, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}),
				funcs.ResourcesDeletedWithin(3*time.Minute, crdsDir, "*.yaml"),
			)).
			Assess("InstallStableCrossplane", funcs.AllOf(
				funcs.AsFeaturesFunc(funcs.HelmRepo(
					helm.WithArgs("add"),
					helm.WithArgs("crossplane-stable"),
					helm.WithArgs("https://charts.crossplane.io/stable"),
				)),
				funcs.AsFeaturesFunc(funcs.HelmInstall(
					helm.WithNamespace(namespace),
					helm.WithName(helmReleaseName),
					helm.WithChart("crossplane-stable/crossplane"),
					helm.WithArgs("--create-namespace", "--wait"),
				)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace))).
			Assess("CreateClaimPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Assess("XRDIsEstablished",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite())).
			Assess("ProviderIsReady",
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsAvailable", funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("UpgradeCrossplane", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Assess("CoreDeploymentIsAvailable", funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane")).
			Assess("RBACManagerDeploymentIsAvailable", funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane-rbac-manager")).
			Assess("CoreCRDsAreEstablished", funcs.ResourcesHaveConditionWithin(1*time.Minute, crdsDir, "*.yaml", funcs.CRDInitialNamesAccepted())).
			Assess("ClaimIsStillAvailable", funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}
