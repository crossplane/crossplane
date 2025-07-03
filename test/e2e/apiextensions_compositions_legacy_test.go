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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsshared "github.com/crossplane/crossplane/apis/apiextensions/shared"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaAPIExtensions is applied to all features pertaining to legacy v1
// style API extensions (i.e. Composition, XRDs, etc).
const LabelAreaAPIExtensionsLegacy = "apiextensions-legacy"

func TestLegacyComposition(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of composition functions ensuring that the composed resources are created, conditions are met, fields are patched, and resources are properly cleaned up when deleted.").
			WithLabel(LabelArea, LabelAreaAPIExtensionsLegacy).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsshared.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("ClaimHasPatchedField",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "status.coolerField", "I'M COOLER!"),
			).
			Assess("ConnectionSecretCreated",
				funcs.ResourceHasFieldValueWithin(30*time.Second, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "basic-secret"}}, "data[super]", "c2VjcmV0Cg=="),
			).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestLegacyPropagateFieldsRemovalToXR(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy-propagate-field-removals"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that field removals in a claim are correctly propagated to the associated composite resource (XR), ensuring that updates and deletions are properly synchronized, and that the status from the XR is accurately reflected back to the claim.").
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaAPIExtensionsLegacy).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsshared.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "claim.yaml", xpv1.Available()),
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
				funcs.ClaimUnderTestMustNotChangeWithin(30*time.Second),
				funcs.CompositeUnderTestMustNotChangeWithin(30*time.Second),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestLegacyPropagateFieldsRemovalToXRAfterUpgrade(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy-propagate-field-removals"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that field removals in a composite resource (XR) are correctly propagated after upgrading the field managers from CSA to SSA, verifying that the upgrade process does not interfere with the synchronization of fields between the claim and the XR.").
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaAPIExtensionsLegacy).
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsshared.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "claim.yaml", xpv1.Available()),
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
				funcs.ClaimUnderTestMustNotChangeWithin(30*time.Second),
				funcs.CompositeUnderTestMustNotChangeWithin(30*time.Second),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestLegacyBindToExistingXR(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/legacy-bind-existing-xr"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a new claim can successfully bind to an existing composite resource (XR), ensuring that the XR’s fields are updated according to the claim’s specifications and that the XR is correctly managed when the claim is deleted.").
			WithLabel(LabelArea, LabelAreaAPIExtensionsLegacy).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsshared.WatchingComposite()),
			)).
			// Create an XR we'll later bind to.
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv1.Available()),
			)).
			// Make sure our fields are set to the XR's values.
			Assess("XRFieldHasOriginalValues", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.coolField", "Set by XR"),
			)).
			// Create a claim that explicitly asks to bind to the above XR.
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("XRIsBoundToClaim", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.claimRef.name", "bind-existing-xr"),
			)).
			Assess("XRFieldChangesToClaimValue", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.coolField", "Set by claim"),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),

				// Deleting the claim should delete the XR.
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
