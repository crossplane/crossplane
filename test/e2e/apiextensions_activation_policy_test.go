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

	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaMRAP is applied to all features pertaining to MRAP testing.
const LabelAreaMRAP = "mrap"

// Tests that should be part of the test suite for the MRAP feature.
const SuiteMRAP = "mrap"

func init() {
	environment.AddTestSuite(SuiteMRAP,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set provider.defaultActivations=null"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteMRAP, config.TestSuiteDefault},
		}),
	)
}

func TestMRAPActivatesSingleMRD(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/activation-policy/single-activation"

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that ManagedResourceActivationPolicy can activate a single ManagedResourceDefinition.").
			WithLabel(LabelArea, LabelAreaMRAP).
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).

			// Setup Phase: Create MRD in Inactive state and disabled MRAP
			WithSetup("CreateInactiveMRDAndDisabledMRAP", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/mrd.yaml",
					v1alpha1.InactiveManaged()),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/mrap-disabled.yaml",
					v1alpha1.Healthy()),
			)).

			// Verify MRD is inactive (no need to check CRD as it should never be created)
			Assess("MRDIsInactive", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "setup/mrd.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionInactive)),
			)).

			// Update MRAP to activate the MRD
			Assess("UpdateMRAPToActivateMRD", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrap.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrap.yaml",
					v1alpha1.Healthy()),
			)).

			// Verify MRD becomes active
			Assess("MRDBecomesActive", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "setup/mrd.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionActive)),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/mrd.yaml",
					v1alpha1.EstablishedManaged()),
			)).

			// Verify CRD is created
			Assess("CRDIsCreated", funcs.AllOf(
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "expected-crd.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "expected-crd.yaml",
					funcs.CRDInitialNamesAccepted()),
			)).

			// Verify MRAP status shows activated MRD
			Assess("MRAPStatusShowsActivatedMRD",
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "mrap.yaml",
					"status.activated", []any{"buckets.single.activation-e2e.crossplane.io"}),
			).

			// Cleanup
			WithTeardown("DeleteMRD", funcs.AllOf(
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestMRAPWildcardActivation(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/activation-policy/wildcard-activation"

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that ManagedResourceActivationPolicy can activate multiple ManagedResourceDefinitions using wildcard patterns.").
			WithLabel(LabelArea, LabelAreaMRAP).
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).

			// Setup Phase: Create multiple MRDs in Inactive state
			WithSetup("CreateMultipleInactiveMRDs", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/mrd-bucket.yaml",
					v1alpha1.InactiveManaged()),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/mrd-instance.yaml",
					v1alpha1.InactiveManaged()),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/mrd-database.yaml",
					v1alpha1.InactiveManaged()),
			)).

			// Verify all MRDs are inactive
			Assess("AllMRDsAreInactive", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "setup/mrd-bucket.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionInactive)),
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "setup/mrd-instance.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionInactive)),
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "setup/mrd-database.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionInactive)),
			)).

			// Create MRAP with wildcard pattern
			Assess("CreateWildcardMRAPToActivateMRDs", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrap-wildcard.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "mrap-wildcard.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrap-wildcard.yaml",
					v1alpha1.Healthy()),
			)).

			// Verify all matching MRDs become active
			Assess("AllMatchingMRDsBecomeActive", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "setup/mrd-bucket.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionActive)),
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "setup/mrd-instance.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionActive)),
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "setup/mrd-database.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionActive)),
			)).

			// Verify all corresponding CRDs are created
			Assess("AllCRDsAreCreated", funcs.AllOf(
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "expected-crds/bucket-crd.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "expected-crds/instance-crd.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "expected-crds/database-crd.yaml"),
			)).

			// Verify MRAP status shows all activated MRDs
			Assess("WildcardMRAPStatusShowsAllActivatedMRDs",
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "mrap-wildcard.yaml",
					"status.activated", []any{
						"buckets.wildcard.activation-e2e.crossplane.io",
						"databases.wildcard.activation-e2e.crossplane.io",
						"instances.wildcard.activation-e2e.crossplane.io",
					}),
			).

			// Cleanup
			WithTeardown("DeleteWildcardMRAPAndMRDs", funcs.AllOf(
				funcs.DeleteResources(manifests, "mrap-wildcard.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "mrap-wildcard.yaml"),
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestMultipleMRAPsOneMRD(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/activation-policy/multiple-mraps"

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that multiple ManagedResourceActivationPolicies can manage the same ManagedResourceDefinition.").
			WithLabel(LabelArea, LabelAreaMRAP).
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).

			// Setup Phase: Create MRD in Inactive state
			WithSetup("CreateMultiPolicyTestMRD", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/mrd.yaml",
					v1alpha1.InactiveManaged()),
			)).

			// Create first MRAP with specific name
			Assess("CreateSpecificMRAP", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrap-specific.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "mrap-specific.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrap-specific.yaml",
					v1alpha1.Healthy()),
			)).

			// Verify MRD becomes active
			Assess("MRDActivatedBySpecificMRAP", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "setup/mrd.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionActive)),
			)).

			// Create second MRAP with wildcard pattern
			Assess("CreateWildcardMRAP", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrap-wildcard.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "mrap-wildcard.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrap-wildcard.yaml",
					v1alpha1.Healthy()),
			)).

			// Verify both MRAPs show the MRD as activated
			Assess("BothMRAPsShowMRDAsActivated", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "mrap-specific.yaml",
					"status.activated", []any{"storages.multiple.activation-e2e.crossplane.io"}),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "mrap-wildcard.yaml",
					"status.activated", []any{"storages.multiple.activation-e2e.crossplane.io"}),
			)).

			// Delete first MRAP, MRD should stay active (still matched by wildcard)
			Assess("DeleteSpecificMRAPMRDStaysActive", funcs.AllOf(
				funcs.DeleteResources(manifests, "mrap-specific.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "mrap-specific.yaml"),
				// MRD should still be active because wildcard MRAP still matches
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "setup/mrd.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionActive)),
			)).

			// Delete second MRAP, MRD should stay active.
			Assess("DeleteWildcardMRAPMRDBecomesInactive", funcs.AllOf(
				funcs.DeleteResources(manifests, "mrap-wildcard.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "mrap-wildcard.yaml"),
				// MRD should continue being active
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "setup/mrd.yaml",
					"spec.state", string(v1alpha1.ManagedResourceDefinitionActive)),
			)).

			// Cleanup
			WithTeardown("DeleteMultiPolicyTestMRD", funcs.AllOf(
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
