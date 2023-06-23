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
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaLifecycle is applied to all 'features' pertaining to managing
// Crossplane's lifecycle (installing, upgrading, etc).
const LabelAreaLifecycle = "lifecycle"

// TestCrossplane tests installing, uninstalling, and upgrading Crossplane.
func TestCrossplane(t *testing.T) {
	// We install Crossplane as part of setting up the test environment, so
	// we're really only validating the installation here.
	install := features.Table{
		{
			Name:       "CoreDeploymentBecomesAvailable",
			Assessment: funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane"),
		},
		{
			Name:       "RBACManagerDeploymentBecomesAvailable",
			Assessment: funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane-rbac-manager"),
		},
		{
			Name:       "CoreCRDsBecomeEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, crdsDir, "*.yaml", funcs.CRDInitialNamesAccepted()),
		},
	}

	// Test that it's possible to cleanly uninstall Crossplane, even after
	// having created and deleted a claim.
	manifests := "test/e2e/manifests/lifecycle/uninstall"
	uninstall := features.Table{
		{
			Name: "ClaimPrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "claim.yaml", xpv1.Available()),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name: "CrossplaneIsUninstalled",
			Assessment: funcs.AsFeaturesFunc(funcs.HelmUninstall(
				helm.WithName(helmReleaseName),
				helm.WithNamespace(namespace),
			)),
		},
		// Uninstalling the Crossplane Helm chart doesn't remove its CRDs. We
		// want to make sure they can be deleted cleanly. If they can't, it's a
		// sign something they define might have stuck around.
		{
			Name: "CoreCRDsAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(crdsDir, "*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, crdsDir, "*.yaml"),
			),
		},
		// Uninstalling the Crossplane Helm chart doesn't remove the namespace
		// it was installed to either. We want to make sure it can be deleted
		// cleanly.
		{
			Name: "CrossplaneNamespaceIsDeleted",
			Assessment: funcs.AllOf(
				funcs.AsFeaturesFunc(envfuncs.DeleteNamespace(namespace)),
				funcs.ResourceDeletedWithin(3*time.Minute, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}),
			),
		},
	}

	// Test that it's possible to upgrade from the most recent stable Crossplane
	// Helm chart to the one we're testing, even when a claim exists.
	manifests = "test/e2e/manifests/lifecycle/upgrade"
	upgrade := features.Table{
		{
			Name: "CrossplaneNamespaceIsCreated",
			Assessment: funcs.AllOf(
				funcs.AsFeaturesFunc(envfuncs.CreateNamespace(namespace)),
				funcs.ResourceCreatedWithin(1*time.Minute, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}),
			),
		},
		{
			Name: "CrossplaneStableIsInstalled",
			Assessment: funcs.AllOf(
				funcs.AsFeaturesFunc(funcs.HelmRepo(
					helm.WithArgs("add"),
					helm.WithArgs("crossplane-stable"),
					helm.WithArgs("https://charts.crossplane.io/stable"),
				)),
				funcs.AsFeaturesFunc(funcs.HelmInstall(
					helm.WithNamespace(namespace),
					helm.WithName(helmReleaseName),
					helm.WithChart("crossplane-stable/crossplane"),
				)),
			),
		},
		{
			Name:       "CrossplaneStableIsRunning",
			Assessment: funcs.ReadyToTestWithin(1*time.Minute, namespace),
		},
		{
			Name: "ClaimPrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "claim.yaml", xpv1.Available()),
		},
		{
			Name:       "CrossplaneIsUpgraded",
			Assessment: funcs.AsFeaturesFunc(funcs.HelmUpgrade(helmOptions...)),
		},
		{
			Name:       "CoreDeploymentBecomesAvailable",
			Assessment: funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane"),
		},
		{
			Name:       "RBACManagerDeploymentBecomesAvailable",
			Assessment: funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane-rbac-manager"),
		},
		{
			Name:       "CoreCRDsBecomeEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, crdsDir, "*.yaml", funcs.CRDInitialNamesAccepted()),
		},
		{
			Name:       "ClaimStillAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "claim.yaml", xpv1.Available()),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			),
		},
		{
			Name: "ClaimPrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
	}

	environment.Test(t,
		install.Build("Install").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeSmall).
			Feature(),
		uninstall.Build("Uninstall").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeLarge).
			Feature(),
		upgrade.Build("Upgrade").
			WithLabel(LabelArea, LabelAreaLifecycle).
			WithLabel(LabelSize, LabelSizeLarge).
			Feature(),
	)
}
