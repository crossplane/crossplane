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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(30*time.Second, manifests, "claim.yaml", xpv2.Available()),
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available(), xpv2.ReconcileSuccess())).
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

func TestLackOfRightsNamespaced(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/lack-of-rights-namespaced"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Test that when attempting to compose a resource the controller has insufficient rights to manage, we get correct messaging.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRHasStatusField", funcs.AllOf(
				// A blank error is as good as we can do at the moment. This validates we get into a reconciliation error, which is better than nothing.
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "xr.yaml", xpv2.ReconcileError(errors.New(""))),
			)).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available(), xpv2.ReconcileSuccess()),
			).
			Assess("ComposedResourceHasGenerateName",
				funcs.ComposedResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "metadata.generateName", "basic-xr-cluster-", func(object k8s.Object) bool {
					return object.GetObjectKind().GroupVersionKind().Kind == "ConfigMap"
				}),
			).
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "claim.yaml", xpv2.Available()),
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.ReconcileError(errors.New(""))),
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

func TestCircuitBreaker(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/circuit-breaker"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests circuit breaker functionality when XR reconciliation is triggered too frequently due to rapid ConfigMap updates, ensuring the circuit opens with proper status conditions.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRBecomesReady",
				funcs.ResourcesHaveConditionWithin(30*time.Second, manifests, "xr.yaml", xpv2.Available()),
			).
			Assess("WaitForNormalOperation", funcs.SleepFor(45*time.Second)).
			// Verify the circuit stays closed under normal operation
			Assess("VerifyCircuitStaysClosedUnderNormalOperation",
				funcs.ResourcesHaveConditionWithin(10*time.Second, manifests, "xr.yaml", apiextensionsv1.WatchCircuitClosed()),
			).
			Assess("TriggerCircuitBreakerWithRapidUpdates", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				t.Helper()

				t.Log("Starting rapid ConfigMap updates using Server-Side Apply to trigger circuit breaker...")

				// Rapidly apply ConfigMap updates using SSA
				// We'll do 60 updates over 3 seconds (20 updates/second)
				// This should exhaust the 50-token bucket quickly
				for i := range 60 {
					cm := &unstructured.Unstructured{}
					cm.SetAPIVersion("v1")
					cm.SetKind("ConfigMap")
					cm.SetNamespace("default")
					cm.SetName("circuit-breaker-configmap")
					fieldpath.Pave(cm.Object).SetString("data.counter", fmt.Sprintf("%d", i))

					//nolint:staticcheck // TODO(adamwg) Stop using client.Apply after the v2.2 release.
					if err := cfg.Client().Resources().GetControllerRuntimeClient().Patch(ctx, cm, client.Apply, client.FieldOwner(FieldManager), client.ForceOwnership); err != nil {
						t.Logf("SSA update %d failed: %v", i, err)
					}

					// 50ms between updates = 20 updates/second
					time.Sleep(50 * time.Millisecond)
				}

				t.Log("Completed rapid ConfigMap updates using SSA")
				return ctx
			}).
			Assess("CircuitBreakerOpens",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "xr.yaml",
					xpv2.Condition{
						Type:   apiextensionsv1.TypeResponsive,
						Status: corev1.ConditionFalse,
						Reason: apiextensionsv1.ReasonWatchCircuitOpen,
						// Note: Message is intentionally omitted as E2E framework ignores messages during comparison
					}),
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

func TestRequiredSchemas(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/required-schemas"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that functions can request and receive OpenAPI schemas for resource kinds via the required schemas mechanism, with referenced schemas like ObjectMeta flattened inline.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available(), xpv2.ReconcileSuccess()),
			).
			Assess("SchemaWasReceived",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.schemaReceived", true),
			).
			Assess("SchemaHasExpectedProperties",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.schemaHasProperties", true),
			).
			Assess("SchemaHasFlattenedRefs",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.schemaHasFlattenedRefs", true),
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

func TestRequiredResources(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/required-resources"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that functions can request and receive existing cluster resources via the required resources mechanism.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available(), xpv2.ReconcileSuccess()),
			).
			Assess("ResourceWasReceived",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.resourceReceived", true),
			).
			Assess("ResourceHasExpectedData",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.resourceHasExpectedData", true),
			).
			Assess("XRReflectsInitialValue",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.receivedValue", "test-value"),
			).
			// Changing the required ConfigMap must reconcile the XR, so its
			// status reflects the new value. The XR doesn't compose the
			// ConfigMap, so no composed resource watch observes it. function-python
			// applies a default 60s response TTL, but we assert the change within
			// 30s - before that requeue could fire - so the reconcile is
			// attributable to the required resource watch, not to polling.
			Assess("UpdateRequiredResource",
				funcs.ApplyResources(FieldManager, manifests, "configmap-update.yaml"),
			).
			Assess("XRReactsToRequiredResourceChange",
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "xr.yaml", "status.receivedValue", "updated-value"),
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

func TestCompositionRevisionGarbageCollection(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/revision-gc"

	// Each update changes an annotation, which creates a new revision. We wait
	// for each revision before the next update. The controller only sees the
	// latest state, so updating faster could skip revisions.
	update := func(n int) features.Func {
		return funcs.ApplyResources(FieldManager, manifests, "composition.yaml",
			funcs.SetAnnotationMutateOption("revision-gc.e2e.crossplane.io/update", strconv.Itoa(n)))
	}

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that Crossplane deletes unused CompositionRevisions beyond the Composition's revision history limit.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition.yaml"),
				compositionRevisionsWithin(30*time.Second, "revision-gc", 1),
			)).
			// Create an XR with the manual update policy. It will be pinned to
			// revision 1, so revision 1 is in use.
			Assess("CreateXRPinnedToFirstRevision", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "xr.yaml", "spec.crossplane.compositionRevisionRef.name", funcs.Any),
			)).
			// No GC on the first update: all revisions are in use.
			Assess("UpdateCompositionOnce", funcs.AllOf(
				update(1),
				compositionRevisionsWithin(30*time.Second, "revision-gc", 1, 2),
			)).
			// GC on the second update: revision 2 is unused and
			// revisionHistoryLimit is 1.
			Assess("UpdateCompositionTwice", funcs.AllOf(
				update(2),
				compositionRevisionsWithin(30*time.Second, "revision-gc", 1, 3),
			)).
			// GC on the third update: revision 3 is unused.
			Assess("UpdateCompositionThreeTimes", funcs.AllOf(
				update(3),
				compositionRevisionsWithin(30*time.Second, "revision-gc", 1, 4),
			)).
			// Delete the XR to make revision 1 unused.
			Assess("DeleteXR", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			)).
			// GC on the fourth update: revision 1 is now unused, so we should
			// keep 4 and 5.
			Assess("UpdateCompositionAfterXRIsDeleted", funcs.AllOf(
				update(4),
				compositionRevisionsWithin(30*time.Second, "revision-gc", 4, 5),
			)).
			// Deleting the XRD deletes the XR too, if an earlier step failed
			// before deleting it.
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifests, "composition.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "composition.yaml"),
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

// compositionRevisionsWithin fails a test if the revision numbers of the named
// Composition's CompositionRevisions aren't exactly the supplied revisions
// within the supplied duration.
func compositionRevisionsWithin(d time.Duration, comp string, want ...int64) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		var got []int64

		err := wait.For(func(ctx context.Context) (bool, error) {
			l := &apiextensionsv1.CompositionRevisionList{}
			if err := c.Client().Resources().List(ctx, l, resources.WithLabelSelector(apiextensionsv1.LabelCompositionName+"="+comp)); err != nil {
				t.Logf("cannot list CompositionRevisions: %v", err)
				return false, nil
			}

			got = make([]int64, 0, len(l.Items))
			for _, rev := range l.Items {
				got = append(got, rev.Spec.Revision)
			}

			slices.Sort(got)

			return slices.Equal(got, want), nil
		}, wait.WithTimeout(d), wait.WithInterval(funcs.DefaultPollInterval))
		if err != nil {
			t.Errorf("Composition %q has revisions %v, want %v: %v", comp, got, want, err)
			return ctx
		}

		t.Logf("Composition %q has revisions %v", comp, want)

		return ctx
	}
}
