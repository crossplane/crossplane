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
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionscommon "github.com/crossplane/crossplane/apis/apiextensions/common"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionscommon.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
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
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "claim.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestBasicCompositionNamespaced(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/basic-namespaced"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of a namespaced XR ensuring that the composed resources are created, conditions are met, fields are patched, and resources are properly cleaned up when deleted.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionscommon.WatchingComposite()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv1.Available(), xpv1.ReconcileSuccess())).
			Assess("XRHasStatusField",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.coolerField", "I'M COOLER!"),
			).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestBasicCompositionCluster(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/basic-cluster"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of cluster-scoped XR ensuring that the composed resources are created, conditions are met, fields are patched, and resources are properly cleaned up when deleted.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionscommon.WatchingComposite()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv1.Available(), xpv1.ReconcileSuccess())).
			Assess("XRHasStatusField",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.coolerField", "I'M COOLER!"),
			).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionscommon.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "claim.yaml", xpv1.Available()),
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

func TestNamespacedXRClusterComposition(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/namespaced-xr-no-cluster-scoped-resource"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that namespaced XRs cannot compose cluster-scoped resources, ensuring proper validation and error handling when such invalid compositions are attempted.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateNamespacedXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRHasReconcileError",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv1.ReconcileError(errors.New(""))),
			).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
