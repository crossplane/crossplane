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
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	apiextensionsv2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaLifecycle is applied to all 'features' pertaining to managing
// Crossplane's lifecycle (installing, upgrading, etc).
const LabelAreaLifecycle = "lifecycle"

// Note: First time Installation is tested as part of the environment setup,
// if not disabled explicitly.
func TestCrossplaneLifecycle(t *testing.T) {
	manifests := "test/e2e/manifests/lifecycle/upgrade"
	environment.Test(t,
		// Test that it's possible to cleanly uninstall Crossplane, even after
		// having created and deleted a claim.
		features.NewWithDescription(t.Name()+"Uninstall", "Test that it's possible to cleanly uninstall Crossplane, even after having created and deleted a claim.").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("XRDAreEstablished", funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv2.WatchingComposite())).
			WithSetup("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			WithSetup("ClaimIsAvailable", funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("DeleteClaim", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "claim.yaml"),

				// TODO(negz): We're seeing composed resources
				// sticking around after the claim is deleted,
				// even though the claim is deleting the XR (and
				// thus its composed resources) with foreground
				// deletion. How is that possible?
				funcs.ListedResourcesDeletedWithin(1*time.Minute, composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
					APIVersion: "nop.crossplane.io/v1alpha1",
					Kind:       "NopResource",
				}))),
			)).
			Assess("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),

				// TODO(negz): We're seeing revisions sticking
				// around after Crossplane is uninstalled, which
				// blocks deleting their CRDs. It's unclear how
				// that's possible given that we're deleting the
				// packages with foreground deletion, and
				// waiting for the packages to be gone.
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &pkgv1.ProviderRevisionList{}),
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &pkgv1.FunctionRevisionList{}),
			)).
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
		features.NewWithDescription(t.Name()+"Downgrade", "Test that it's possible to downgrade Crossplane to the most recent stable Helm chart from the one we're testing, even when a claim exists. This expects Crossplane not to be installed.").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			// We expect Crossplane to have been uninstalled first
			Assess("CrossplaneIsNotInstalled", funcs.AllOf(
				funcs.ResourceDeletedWithin(1*time.Minute, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}),
				funcs.ResourcesDeletedWithin(3*time.Minute, crdsDir, "*.yaml"),
			)).
			Assess("InstallCrossplane", funcs.AllOf(
				funcs.AsFeaturesFunc(envfuncs.CreateNamespace(namespace)),
				funcs.AsFeaturesFunc(environment.HelmInstallBaseCrossplane()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Assess("CreateClaimPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Assess("XRDIsEstablished",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv2.WatchingComposite())).
			Assess("ProviderIsReady",
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsAvailable", funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("DowngradeCrossplane", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradePriorCrossplane(namespace, helmReleaseName)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Assess("CoreDeploymentIsAvailable", funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane")).
			Assess("RBACManagerDeploymentIsAvailable", funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane-rbac-manager")).
			Assess("CoreCRDsAreEstablished", funcs.ResourcesHaveConditionWithin(1*time.Minute, crdsDir, "*.yaml", funcs.CRDInitialNamesAccepted())).
			Assess("ClaimIsStillAvailable", funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			// TODO(turkenh): Backport provider deployment label selector migration to the previous stable
			//  Crossplane version and remove the following step. Currently, when we downgrade from main to
			//  the previous stable version, provider stuck as unhealthy because the label selector
			//  of the provider deployment is not compatible with the previous stable version.
			//  Tracking issue https://github.com/crossplane/crossplane/issues/6506
			Assess("DeleteProviderNopDeployment", func(ctx context.Context, t *testing.T, e *envconf.Config) context.Context {
				t.Helper()
				err := e.Client().Resources("crossplane-system").Delete(ctx, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "provider-nop-lifecycle-upgrade-37f3300ebfa7",
						Namespace: "crossplane-system",
					},
				})
				if client.IgnoreNotFound(err) != nil {
					t.Errorf("Failed to delete provider-nop-lifecycle-upgrade-37f3300ebfa7 deployment: %v", err)
				}
				return ctx
			}).
			Assess("ProviderIsReady",
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active())).
			Assess("DeleteClaim", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "claim.yaml"),

				// TODO(negz): We're seeing composed resources
				// sticking around after the claim is deleted,
				// even though the claim is deleting the XR (and
				// thus its composed resources) with foreground
				// deletion. How is that possible?
				funcs.ListedResourcesDeletedWithin(1*time.Minute, composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
					APIVersion: "nop.crossplane.io/v1alpha1",
					Kind:       "NopResource",
				}))),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),

				// TODO(negz): We're seeing revisions sticking
				// around after Crossplane is uninstalled, which
				// blocks deleting their CRDs. It's unclear how
				// that's possible given that we're deleting the
				// packages with foreground deletion, and
				// waiting for the packages to be gone.
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &pkgv1.ProviderRevisionList{}),
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &pkgv1.FunctionRevisionList{}),
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
		features.NewWithDescription(t.Name()+"Upgrade", "Test that it's possible to upgrade Crossplane from the most recent stable Helm chart to the one we're testing, even when a claim exists. This expects Crossplane not to be installed.").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			// We expect Crossplane to have been uninstalled first
			Assess("CrossplaneIsNotInstalled", funcs.AllOf(
				funcs.ResourceDeletedWithin(1*time.Minute, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}),
				funcs.ResourcesDeletedWithin(3*time.Minute, crdsDir, "*.yaml"),
			)).
			Assess("InstallStableCrossplane", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmInstallPriorCrossplane(namespace, helmReleaseName)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace))).
			Assess("CreateClaimPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Assess("XRDIsEstablished",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv2.WatchingComposite())).
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
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "claim.yaml"),

				// TODO(negz): We're seeing composed resources
				// sticking around after the claim is deleted,
				// even though the claim is deleting the XR (and
				// thus its composed resources) with foreground
				// deletion. How is that possible?
				funcs.ListedResourcesDeletedWithin(1*time.Minute, composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
					APIVersion: "nop.crossplane.io/v1alpha1",
					Kind:       "NopResource",
				}))),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
