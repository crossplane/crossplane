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
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaAPIExtensions is applied to all features pertaining to ops (e.g.
// Operations, CronOperations, etc).
const LabelAreaOps = "ops"

// Tests that should be part of the test suite for the alpha Operations
// feature.
const SuiteOps = "ops"

func init() {
	environment.AddTestSuite(SuiteOps,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-operations}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteOps, config.TestSuiteDefault},
		}),
	)
}

func TestOperation(t *testing.T) {
	cm := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cool-map"}}

	manifests := "test/e2e/manifests/ops/operations/simple"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of a basic Operation that creates a ConfigMap.").
			WithLabel(LabelArea, LabelAreaOps).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteOps).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				// Wait for function to be ready with capabilities before creating Operation
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "operation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "operation.yaml"),
			)).
			Assess("OperationSucceeded", funcs.AllOf(
				funcs.ResourcesHaveConditionWithin(60*time.Second, manifests, "operation.yaml", v1alpha1.Complete()),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.appliedResourceRefs[0].name", "cool-map"),
				funcs.ResourceHasFieldValueWithin(30*time.Second, cm, "data[coolData]", "I'm cool!"),
			)).
			WithTeardown("DeleteOperation", funcs.AllOf(
				funcs.DeleteResources(manifests, "operation.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "operation.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestOperationRetryLogic(t *testing.T) {
	manifests := "test/e2e/manifests/ops/operations/retry"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the retry logic of Operations when function pipelines fail.").
			WithLabel(LabelArea, LabelAreaOps).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteOps).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				// Wait for function to be ready with capabilities before creating Operation
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/*.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "operation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "operation.yaml"),
			)).
			Assess("OperationRetriesAndEventuallyFails", funcs.AllOf(
				// Wait for Operation to start retrying - it should start with 0 failures
				funcs.ResourcesHaveConditionWithin(30*time.Second, manifests, "operation.yaml", xpv1.ReconcileSuccess()),
				// Wait for Operation to accumulate failures as it retries (exponential backoff means this takes time)
				// retryLimit is 3, so it should eventually reach 3 failures and be marked as failed
				funcs.ResourcesHaveFieldValueWithin(3*time.Minute, manifests, "operation.yaml", "status.failures", int64(3)),
				// Operation should eventually be marked as failed after reaching retry limit
				funcs.ResourcesHaveConditionWithin(30*time.Second, manifests, "operation.yaml", v1alpha1.Failed("")),
			)).
			WithTeardown("DeleteOperation", funcs.AllOf(
				funcs.DeleteResources(manifests, "operation.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "operation.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestOperationMultiStepPipeline(t *testing.T) {
	cmA := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "multi-step-configmap-a"}}
	cmB := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "multi-step-configmap-b"}}

	manifests := "test/e2e/manifests/ops/operations/pipeline"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests multi-step pipeline execution with function output capture.").
			WithLabel(LabelArea, LabelAreaOps).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteOps).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				// Wait for function to be ready with capabilities before creating Operation
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/*.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "operation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "operation.yaml"),
			)).
			Assess("OperationExecutesPipelineSuccessfully", funcs.AllOf(
				// Wait for Operation to become ready and succeed
				funcs.ResourcesHaveConditionWithin(60*time.Second, manifests, "operation.yaml", v1alpha1.Complete()),
				// Verify both ConfigMaps were created by the pipeline
				funcs.ResourceHasFieldValueWithin(30*time.Second, cmA, "data[step]", "1"),
				funcs.ResourceHasFieldValueWithin(30*time.Second, cmA, "data[content]", "Created by step 1"),
				funcs.ResourceHasFieldValueWithin(30*time.Second, cmB, "data[step]", "2"),
				funcs.ResourceHasFieldValueWithin(30*time.Second, cmB, "data[content]", "Created by step 2"),
				// Verify appliedResourceRefs contains both ConfigMaps
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.appliedResourceRefs[0].name", "multi-step-configmap-a"),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.appliedResourceRefs[1].name", "multi-step-configmap-b"),
			)).
			Assess("OperationCapturesFunctionOutputs", funcs.AllOf(
				// Verify function outputs are captured in Operation status
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.pipeline[0].step", "create-configmap-a"),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.pipeline[0].output.stepName", "create-configmap-a"),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.pipeline[0].output.resourcesCreated", int64(1)),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.pipeline[1].step", "create-configmap-b"),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.pipeline[1].output.stepName", "create-configmap-b"),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.pipeline[1].output.totalPipelineResources", int64(2)),
			)).
			WithTeardown("DeleteOperation", funcs.AllOf(
				funcs.DeleteResources(manifests, "operation.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "operation.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestCronOperationScheduling(t *testing.T) {
	var firstScheduleTime, firstSuccessTime time.Time

	manifests := "test/e2e/manifests/ops/cronoperations/scheduling"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests CronOperation scheduling behavior, validating that it creates multiple Operations on schedule over time.").
			WithLabel(LabelArea, LabelAreaOps).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteOps).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				// Wait for function to be ready with capabilities before creating Operation
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/*.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateCronOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "cronoperation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "cronoperation.yaml"),
			)).
			Assess("CronOperationCreatesFirstOperation", funcs.AllOf(
				// Capture the first lastScheduleTime when CronOperation creates its first Operation
				// Since the schedule is every minute, this should happen quickly
				funcs.ResourcesHaveFieldValueWithin(90*time.Second, manifests, "cronoperation.yaml", "status.lastScheduleTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								firstScheduleTime = parsed
								return true
							}
						}
						return false
					})),
				// Verify the CronOperation has the correct status and schedule is active
				funcs.ResourcesHaveConditionWithin(30*time.Second, manifests, "cronoperation.yaml", xpv1.ReconcileSuccess(), v1alpha1.ScheduleActive()),
			)).
			Assess("FirstOperationSucceeds", funcs.AllOf(
				// Capture the first lastSuccessfulTime when the first Operation completes
				funcs.ResourcesHaveFieldValueWithin(60*time.Second, manifests, "cronoperation.yaml", "status.lastSuccessfulTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								firstSuccessTime = parsed
								return true
							}
						}
						return false
					})),
			)).
			Assess("CronOperationCreatesSecondOperation", funcs.AllOf(
				// Wait for the second cron trigger to happen and create another Operation
				// Verify the lastScheduleTime advances, proving a new Operation was created
				// Since the schedule is every minute, we wait up to 2 minutes for the next trigger
				funcs.ResourcesHaveFieldValueWithin(2*time.Minute, manifests, "cronoperation.yaml", "status.lastScheduleTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								return parsed.After(firstScheduleTime)
							}
						}
						return false
					})),
			)).
			Assess("SecondOperationSucceeds", funcs.AllOf(
				// Verify the second Operation also completes successfully
				// Check that lastSuccessfulTime advances from the first success time
				funcs.ResourcesHaveFieldValueWithin(90*time.Second, manifests, "cronoperation.yaml", "status.lastSuccessfulTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								return parsed.After(firstSuccessTime)
							}
						}
						return false
					})),
			)).
			Assess("ValidateMultipleOperations", funcs.AllOf(
				// As a final validation, verify that we have the expected number of Operations
				// We expect at least 2 Operations: one from each cron trigger
				funcs.ListedResourcesValidatedWithin(30*time.Second,
					&v1alpha1.OperationList{},
					2, // At least 2 Operations should exist
					func(o k8s.Object) bool {
						// Check if this Operation was created by our CronOperation
						return o.GetLabels()[v1alpha1.LabelCronOperationName] == "basic-cronop"
					}),
			)).
			WithTeardown("DeleteCronOperation", funcs.AllOf(
				funcs.DeleteResources(manifests, "cronoperation.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "cronoperation.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestWatchOperationResourceChanges(t *testing.T) {
	var firstScheduleTime, secondScheduleTime, thirdScheduleTime time.Time

	manifests := "test/e2e/manifests/ops/watchoperations/resource-changes"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests WatchOperation resource change detection, validating that it creates Operations when watched resources are created, updated, deleted, or when multiple resources match the selector.").
			WithLabel(LabelArea, LabelAreaOps).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteOps).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				// Wait for function to be ready with capabilities before creating Operation
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/*.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateWatchOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "watchoperation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "watchoperation.yaml"),
			)).
			Assess("WatchOperationEstablishesWatching", funcs.AllOf(
				// Wait for WatchOperation to become ready (controller started) and actively watching
				funcs.ResourcesHaveConditionWithin(60*time.Second, manifests, "watchoperation.yaml", xpv1.ReconcileSuccess(), v1alpha1.WatchActive()),
			)).
			Assess("CreateWatchedResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "test-configmap.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "test-configmap.yaml"),
			)).
			Assess("WatchOperationCreatesFirstOperation", funcs.AllOf(
				// Capture the first lastScheduleTime when WatchOperation creates its first Operation
				funcs.ResourcesHaveFieldValueWithin(60*time.Second, manifests, "watchoperation.yaml", "status.lastScheduleTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								firstScheduleTime = parsed
								return true
							}
						}
						return false
					})),
				// Verify watching resources count is updated
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "watchoperation.yaml", "status.watchingResources", int64(1)),
			)).
			Assess("FirstOperationSucceeds", funcs.AllOf(
				// Verify the WatchOperation's lastSuccessfulTime gets updated when Operation completes
				funcs.ResourcesHaveFieldValueWithin(90*time.Second, manifests, "watchoperation.yaml", "status.lastSuccessfulTime", funcs.Any),
			)).
			Assess("UpdateWatchedResource", funcs.AllOf(
				// Update the ConfigMap to trigger another Operation
				funcs.ApplyResources(FieldManager, manifests, "test-configmap-updated.yaml"),
				// Wait for the update to be processed
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "test-configmap-updated.yaml", "data.testData", "I trigger WatchOperations again!"),
			)).
			Assess("WatchOperationDetectsUpdate", funcs.AllOf(
				// Wait for WatchOperation to detect the ConfigMap update and create another Operation
				// Verify the lastScheduleTime advances, proving a new Operation was created
				funcs.ResourcesHaveFieldValueWithin(60*time.Second, manifests, "watchoperation.yaml", "status.lastScheduleTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								if parsed.After(firstScheduleTime) {
									secondScheduleTime = parsed
									return true
								}
							}
						}
						return false
					})),
			)).
			Assess("CreateSecondWatchedResource", funcs.AllOf(
				// Create a second ConfigMap that matches the label selector
				funcs.ApplyResources(FieldManager, manifests, "test-configmap-second.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "test-configmap-second.yaml"),
			)).
			Assess("WatchOperationDetectsSecondResource", funcs.AllOf(
				// Wait for WatchOperation to detect the second ConfigMap and create another Operation
				// Verify the lastScheduleTime advances again, proving a third Operation was created
				funcs.ResourcesHaveFieldValueWithin(60*time.Second, manifests, "watchoperation.yaml", "status.lastScheduleTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								if parsed.After(secondScheduleTime) {
									thirdScheduleTime = parsed
									return true
								}
							}
						}
						return false
					})),
				// Verify watching resources count is updated to 2
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "watchoperation.yaml", "status.watchingResources", int64(2)),
			)).
			Assess("DeleteFirstWatchedResource", funcs.AllOf(
				// Delete the first ConfigMap to trigger another Operation
				funcs.DeleteResources(manifests, "test-configmap.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "test-configmap.yaml"),
			)).
			Assess("WatchOperationDetectsDeletion", funcs.AllOf(
				// Wait for WatchOperation to detect the ConfigMap deletion and create another Operation
				// Verify the lastScheduleTime advances once more, proving a fourth Operation was created
				funcs.ResourcesHaveFieldValueWithin(60*time.Second, manifests, "watchoperation.yaml", "status.lastScheduleTime",
					funcs.FieldValueChecker(func(got any) bool {
						if timeStr, ok := got.(string); ok {
							if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
								return parsed.After(thirdScheduleTime)
							}
						}
						return false
					})),
				// As a final validation, verify that we have the expected number of Operations
				// We expect at least 4 Operations: create for first ConfigMap, update for first ConfigMap,
				// create for second ConfigMap, and delete for first ConfigMap
				funcs.ListedResourcesValidatedWithin(30*time.Second,
					&v1alpha1.OperationList{},
					4, // At least 4 Operations should exist
					func(o k8s.Object) bool {
						// Check if this Operation was created by our WatchOperation
						return o.GetLabels()[v1alpha1.LabelWatchOperationName] == "basic-watchop"
					}),
			)).
			WithTeardown("DeleteSecondWatchedResource", funcs.AllOf(
				funcs.DeleteResources(manifests, "test-configmap-second.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "test-configmap-second.yaml"),
			)).
			WithTeardown("DeleteWatchOperation", funcs.AllOf(
				funcs.DeleteResources(manifests, "watchoperation.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "watchoperation.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
