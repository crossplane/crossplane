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
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xresource/unstructured/composed"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaAPIExtensions is applied to all features pertaining to API
// extensions (i.e. Composition, XRDs, etc).
const LabelAreaAPIExtensions = "apiextensions"

// Tests that should be part of the test suite for the alpha function response
// caching feature. There's no special tests for this; we just run the regular
// test suite with caching enabled.
const SuiteFunctionResponseCache = "function-response-cache"

func init() {
	environment.AddTestSuite(SuiteFunctionResponseCache,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-function-response-cache}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteFunctionResponseCache, config.TestSuiteDefault},
		}),
	)
}

var nopList = composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
	APIVersion: "nop.crossplane.io/v1alpha1",
	Kind:       "NopResource",
}))

func TestCompositionRevisionSelection(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/realtime-revision-selection"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests Crossplane's Composition functionality to react in realtime to changes in a Composition by selecting the new CompositionRevision and reconcile the XRs.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(30*time.Second, manifests, "claim.yaml", xpv1.Available()),
			).
			Assess("ClaimHasOriginalField",
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "claim.yaml", "status.coolerField", "from-original-composition"),
			).
			Assess("UpdateComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-update.yaml"),
			)).
			Assess("ClaimHasUpdatedField",
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "claim.yaml", "status.coolerField", "from-updated-composition"),
			).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestLegacyComposition(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of composition functions ensuring that the composed resources are created, conditions are met, fields are patched, and resources are properly cleaned up when deleted.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			// TODO(negz): This Assess consistently takes ~2 seconds
			// on my M1 Max MacBook, but sometimes exceeds 30
			// seconds in CI, even with larger runners. Perhaps
			// slower I/O in CI?
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(60*time.Second, manifests, "claim.yaml", xpv1.Available())).
			Assess("ClaimHasPatchedField",
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "claim.yaml", "status.coolerField", "I'M COOLER!"),
			).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestLegacyPropagateFieldsRemovalToXR(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy-propagate-field-removals"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that field removals in a claim are correctly propagated to the associated composite resource (XR), ensuring that updates and deletions are properly synchronized, and that the status from the XR is accurately reflected back to the claim.").
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("UpdateClaim", funcs.ApplyClaim(FieldManager, manifests, "claim-update.yaml")).
			Assess("FieldsRemovalPropagatedToXR", funcs.AllOf(
				// Updates and deletes are propagated claim -> XR.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo2]", "3"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo2]", "4"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[0]", "one"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[1]", "five"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[2]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[tag]", "v1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[newtag]", funcs.NotFound),
				funcs.ClaimUnderTestMustNotChangeWithin(1*time.Minute),
				funcs.CompositeUnderTestMustNotChangeWithin(1*time.Minute),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestLegacyPropagateFieldsRemovalToXRAfterUpgrade(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy-propagate-field-removals"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that field removals in a composite resource (XR) are correctly propagated after upgrading the field managers from CSA to SSA, verifying that the upgrade process does not interfere with the synchronization of fields between the claim and the XR.").
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			// SSA claims are enabled by default, so we need to explicitly
			// disable them first before we create anything.
			WithSetup("DisableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase(helm.WithArgs("--set args={--debug,--enable-ssa-claims=false}"))), // Disable our feature flag.
				funcs.ArgExistsWithin(1*time.Minute, "--enable-ssa-claims=false", namespace, "crossplane"),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
				funcs.DeploymentPodIsRunningMustNotChangeWithin(10*time.Second, namespace, "crossplane"),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			// Note that unlike TestPropagateFieldsRemovalToXR above, here we
			// enable SSA _after_ creating the claim. Our goal is to test that
			// we can still remove fields from the XR when we've upgraded the
			// field managers from CSA to SSA. If we didn't upgrade successfully
			// would end up sharing ownership with the old CSA field manager.
			Assess("EnableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ArgNotExistsWithin(1*time.Minute, "--enable-ssa-claims=false", namespace, "crossplane"),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
				funcs.DeploymentPodIsRunningMustNotChangeWithin(10*time.Second, namespace, "crossplane"),
			)).
			Assess("UpdateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim-update.yaml"),
				funcs.ClaimUnderTestMustNotChangeWithin(1*time.Minute),
			)).
			Assess("FieldsRemovalPropagatedToXR", funcs.AllOf(
				// Updates and deletes are propagated claim -> XR.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo2]", "3"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo2]", "4"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[0]", "one"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[1]", "five"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[2]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[tag]", "v1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[newtag]", funcs.NotFound),
				funcs.ClaimUnderTestMustNotChangeWithin(1*time.Minute),
				funcs.CompositeUnderTestMustNotChangeWithin(1*time.Minute),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestCompositionSelection(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/composition-selection"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that label selectors in a claim are correctly propagated to the composite resource (XR), ensuring that the appropriate composition is selected and remains consistent even after updates to the label selectors.").
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("LabelSelectorPropagatesToXR", funcs.AllOf(
				// The label selector should be propagated claim -> XR.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionSelector.matchLabels[environment]", "testing"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionSelector.matchLabels[region]", "AU"),
				// The XR should select the composition.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionRef.name", "testing-au"),
				// The selected composition should propagate XR -> claim.
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionRef.name", "testing-au"),
			)).
			// Remove the region label from the composition selector.
			Assess("UpdateClaim", funcs.ApplyClaim(FieldManager, manifests, "claim-update.yaml")).
			Assess("UpdatedLabelSelectorPropagatesToXR", funcs.AllOf(
				// The label selector should be propagated claim -> XR.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionSelector.matchLabels[environment]", "testing"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionSelector.matchLabels[region]", funcs.NotFound),
				// The XR should still have the composition selected.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionRef.name", "testing-au"),
				// The claim should still have the composition selected.
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionRef.name", "testing-au"),
				// The label selector shouldn't reappear on the claim
				// https://github.com/crossplane/crossplane/issues/3992
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionSelector.matchLabels[environment]", "testing"),
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.compositionSelector.matchLabels[region]", funcs.NotFound),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestLegacyBindToExistingXR(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy-bind-existing-xr"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a new claim can successfully bind to an existing composite resource (XR), ensuring that the XR’s fields are updated according to the claim’s specifications and that the XR is correctly managed when the claim is deleted.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			// Create an XR we'll later bind to.
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "xr.yaml", xpv1.Available()),
			)).
			// Make sure our fields are set to the XR's values.
			Assess("XRFieldHasOriginalValues", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.coolField", "Set by XR"),
			)).
			// Create a claim that explicitly asks to bind to the above XR.
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("XRIsBoundToClaim", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.claimRef.name", "bind-existing-xr"),
			)).
			Assess("XRFieldChangesToClaimValue", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.coolField", "Set by claim"),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),

				// Deleting the claim should delete the XR.
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestCompositionValidation(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/validation"

	cases := features.Table{
		{
			Name:        "ValidComposition",
			Description: "A valid Composition should pass validation",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-valid.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition-valid.yaml"),
			),
		},
		{
			Name:        "InvalidMissingPipeline",
			Description: "A Composition without a pipeline shouldn't pass validation",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "composition-invalid-missing-pipeline.yaml"),
		},
		{
			Name:        "InvalidEmptyPipeline",
			Description: "A Composition with a zero-length pipeline shouldn't pass validation",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "composition-invalid-empty-pipeline.yaml"),
		},
		{
			Name:        "InvalidDuplicatePipelinesteps",
			Description: "A Composition with duplicate pipeline step names shouldn't pass validation",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "composition-invalid-duplicate-pipeline-steps.yaml"),
		},
		{
			Name:        "InvalidFunctionMissingSecretRef",
			Description: "A Composition with a step using a Secret credential source but without a secretRef shouldn't pass validation",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "composition-invalid-missing-secretref.yaml"),
		},
	}
	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithTeardown("DeleteValidComposition", funcs.AllOf(
				funcs.DeleteResources(manifests, "composition-valid.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "composition-valid.yaml"),
			)).
			Feature(),
	)
}
