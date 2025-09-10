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
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
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

// contextKey is a type used for context keys to avoid context key collisions
type contextKey string

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

func TestRealtimeCompositionPerformanceNamespaced(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/realtime-namespaced-performance"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests realtime composition performance for namespaced XRs. Verifies Crossplane detects external changes to composed resources within 2-3 seconds via watches, not polling (~60s). Catches performance regressions in realtime composition.").
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv1.Available(), xpv1.ReconcileSuccess())).
			Assess("WaitForComposedResources",
				// Give some time for composed resources to be created and initial reconciliation to complete
				funcs.SleepFor(5*time.Second),
			).
			Assess("CaptureBaselineState", CaptureReconciliationState(manifests, "xr.yaml", "baseline")).
			Assess("ModifyComposedResource", ModifyComposedConfigMap("default")).
			Assess("XRReconcilesInRealtime",
				// This is the critical test - XR should be reconciled within 2-3 seconds if realtime composition is working
				// If it takes ~60 seconds, then we're falling back to polling which is a performance regression
				VerifyReconciledWithin(3*time.Second, manifests, "xr.yaml", "baseline"),
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

// CaptureReconciliationState captures the current reconciliation state of an XR.
func CaptureReconciliationState(dir, pattern, ctxKey string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		// Parse the XR manifest to get namespace and name
		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Fatalf("Failed to decode manifests: %v", err)
		}

		if len(rs) == 0 {
			t.Fatal("No manifests found")
		}

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(rs[0].GetObjectKind().GroupVersionKind())
		obj.SetName(rs[0].GetName())
		obj.SetNamespace(rs[0].GetNamespace())

		// Get the current XR to capture its reconciliation state
		if err := cfg.Client().Resources().Get(ctx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
			t.Fatalf("Failed to get XR: %v", err)
		}

		// Capture the current reconciliation baseline
		baselineState := map[string]interface{}{
			"captureTime": metav1.Now(),
			"object":      obj,
		}

		// Get the ReconcileSuccess condition timestamp as baseline
		status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "conditions")
		if err == nil && found {
			if conditions, ok := status.([]interface{}); ok {
				for _, c := range conditions {
					if condition, ok := c.(map[string]interface{}); ok {
						if condition["type"] == "ReconcileSuccess" {
							if lastTransitionStr, ok := condition["lastTransitionTime"].(string); ok {
								baselineState["lastReconcileTime"] = lastTransitionStr
								break
							}
						}
					}
				}
			}
		}

		ctx = context.WithValue(ctx, contextKey(ctxKey), baselineState)
		t.Logf("Captured baseline reconciliation state")

		return ctx
	}
}

// VerifyReconciledWithin verifies that an XR was reconciled within the specified duration
// after a composed resource change, testing realtime composition performance.
func VerifyReconciledWithin(d time.Duration, _, pattern, ctxKey string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		// Get the baseline state from context
		baselineState, ok := ctx.Value(contextKey(ctxKey)).(map[string]interface{})
		if !ok {
			t.Fatal("Failed to get baseline state from context")
		}

		baselineObj := baselineState["object"].(*unstructured.Unstructured)
		baselineReconcileTime := ""
		if rt, exists := baselineState["lastReconcileTime"]; exists {
			baselineReconcileTime = rt.(string)
		}

		// Create a new object to poll
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(baselineObj.GroupVersionKind())
		obj.SetName(baselineObj.GetName())
		obj.SetNamespace(baselineObj.GetNamespace())

		// Poll for reconciliation within the timeout
		deadline := time.Now().Add(d)
		startTime := time.Now()

		t.Logf("Waiting up to %v for XR to reconcile after composed resource change...", d)

		for time.Now().Before(deadline) {
			if err := cfg.Client().Resources().Get(ctx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
				t.Fatalf("Failed to get XR: %v", err)
			}

			// Check if reconciliation occurred by comparing condition timestamp
			status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "conditions")
			if err != nil || !found {
				time.Sleep(200 * time.Millisecond)
				continue
			}

			conditions, ok := status.([]interface{})
			if !ok {
				time.Sleep(200 * time.Millisecond)
				continue
			}

			for _, c := range conditions {
				condition, ok := c.(map[string]interface{})
				if !ok {
					continue
				}

				if condition["type"] == "ReconcileSuccess" {
					currentReconcileTimeStr, ok := condition["lastTransitionTime"].(string)
					if !ok {
						continue
					}

					// If we have a baseline, check if reconciliation happened after it
					if baselineReconcileTime != "" && currentReconcileTimeStr != baselineReconcileTime {
						elapsed := time.Since(startTime)
						t.Logf("SUCCESS: XR reconciled in %v (well within %v limit)", elapsed, d)
						t.Logf("Realtime composition is working - Crossplane detected composed resource change quickly!")
						return ctx
					}

					// If no baseline, check if reconciliation happened after we started waiting
					if baselineReconcileTime == "" {
						currentReconcileTime, err := time.Parse(time.RFC3339, currentReconcileTimeStr)
						if err != nil {
							continue
						}

						if currentReconcileTime.After(startTime.Add(-1 * time.Second)) { // small buffer
							elapsed := time.Since(startTime)
							t.Logf("SUCCESS: XR reconciled in %v (well within %v limit)", elapsed, d)
							t.Logf("Realtime composition is working!")
							return ctx
						}
					}
				}
			}

			time.Sleep(200 * time.Millisecond)
		}

		elapsed := time.Since(startTime)
		t.Fatalf("PERFORMANCE REGRESSION DETECTED: XR was not reconciled within %v\n"+
			"This suggests realtime composition is broken and falling back to polling (~60s)\n"+
			"Expected: Resource watch triggers immediate reconciliation (<%v)\n"+
			"Actual: Took >%v, likely waiting for polling interval", d, d, elapsed)
		return ctx
	}
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
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "xr.yaml", xpv1.ReconcileError(errors.New(""))),
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv1.Available(), xpv1.ReconcileSuccess()),
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

// ModifyComposedConfigMap returns a function that directly modifies a composed ConfigMap.
// This simulates an external change to a composed resource to test realtime composition.
func ModifyComposedConfigMap(namespace string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		// Get the specific ConfigMap created by our composition (predictable name)
		cm := &corev1.ConfigMap{}
		cm.SetNamespace(namespace)
		cm.SetName("perftest-composed-configmap")

		if err := cfg.Client().Resources().Get(ctx, cm.GetName(), cm.GetNamespace(), cm); err != nil {
			t.Fatalf("Failed to get composed ConfigMap %s/%s: %v", namespace, cm.GetName(), err)
		}

		t.Logf("Found composed ConfigMap: %s/%s", namespace, cm.GetName())

		// Modify the ConfigMap data to simulate an external change
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data["testData"] = "modified-by-external-system"
		cm.Data["modifiedAt"] = time.Now().Format(time.RFC3339)

		// Update the ConfigMap
		if err := cfg.Client().Resources().Update(ctx, cm); err != nil {
			t.Fatalf("Failed to update ConfigMap %s/%s: %v", namespace, cm.GetName(), err)
		}

		t.Logf("Modified composed ConfigMap %s/%s externally", namespace, cm.GetName())
		return ctx
	}
}
