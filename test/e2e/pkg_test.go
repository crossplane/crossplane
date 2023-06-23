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

	"sigs.k8s.io/e2e-framework/pkg/features"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaPkg is applied to all features pertaining to packages, (i.e.
// Providers, Configurations, etc).
const LabelAreaPkg = "pkg"

func TestConfiguration(t *testing.T) {
	// Test that we can install a Configuration from a private repository using
	// a package pull secret.
	manifests := "test/e2e/manifests/pkg/configuration/private"
	private := features.Table{
		{
			Name: "ConfigurationIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "*.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "*.yaml"),
			),
		},
		{
			Name:       "ConfigurationIsHealthy",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active()),
		},
		{
			Name: "ConfigurationIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "*.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "*.yaml"),
			),
		},
	}

	manifests = "test/e2e/manifests/pkg/configuration/dependency"
	dependency := features.Table{
		{
			Name: "ConfigurationIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "configuration.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "configuration.yaml"),
			),
		},
		{
			Name:       "ConfigurationIsHealthy",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "configuration.yaml", pkgv1.Healthy(), pkgv1.Active()),
		},
		{
			// TODO(negz): This is failing because the nopresource CRD still
			// exists and is owned by a ProviderRevision that no longer does.

			// The last thing that installed a Provider was
			// TestCrossplane/Upgrade. It seems to successfully delete its
			// claim, and its prereqs, then an entire other test runs before
			// this one starts. What gives?
			Name:       "ProviderIsHealthy",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "provider-dependency.yaml", pkgv1.Healthy(), pkgv1.Active()),
		},
		{
			Name: "ConfigurationIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "configuration.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "configuration.yaml"),
			),
		},
		{
			// Dependencies are not automatically deleted.
			Name: "ProviderIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "provider-dependency.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-dependency.yaml"),
			),
		},
	}

	setup := funcs.ReadyToTestWithin(1*time.Minute, namespace)
	environment.Test(t,
		private.Build("PullFromPrivateRegistry").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			Setup(setup).Feature(),
		dependency.Build("WithDependency").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			Setup(setup).Feature(),
	)
}

func TestProvider(t *testing.T) {
	// Test that we can upgrade a provider to a new version, even when a managed
	// resource has been created.
	manifests := "test/e2e/manifests/pkg/provider"
	upgrade := features.Table{
		{
			Name: "ProviderIsInstalled",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider-initial.yaml"),
			),
		},
		{
			Name:       "ProviderBecomesHealthy",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "provider-initial.yaml", pkgv1.Healthy(), pkgv1.Active()),
		},
		{
			Name: "ManagedResourceIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mr-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "mr-initial.yaml"),
			),
		},
		{
			Name:       "ProviderIsUpgraded",
			Assessment: funcs.ApplyResources(FieldManager, manifests, "provider-upgrade.yaml"),
		},
		{
			Name:       "UpgradedProviderBecomesHealthy",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "provider-upgrade.yaml", pkgv1.Healthy(), pkgv1.Active()),
		},
		{
			Name: "ManagedResourceIsUpdated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mr-upgrade.yaml"),
			),
		},
		{
			Name:       "ManagedResourceBecomesAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mr.yaml", xpv1.Available()),
		},
		{
			Name: "ManagedResourceIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "mr-upgrade.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "mr-upgrade.yaml"),
			),
		},
		{
			Name: "ProviderIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "provider-upgrade.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-upgrade.yaml"),
			),
		},
	}

	setup := funcs.ReadyToTestWithin(1*time.Minute, namespace)
	environment.Test(t,
		upgrade.Build("Upgrade").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelSize, LabelSizeSmall).
			Setup(setup).Feature(),
	)
}
