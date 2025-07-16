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

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
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

func TestBasicOperation(t *testing.T) {
	cm := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cool-map"}}

	manifests := "test/e2e/manifests/ops/operations/basic"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of a basic Operation that creates a ConfigMap.").
			WithLabel(LabelArea, LabelAreaOps).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteOps).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Assess("CreateOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "operation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "operation.yaml"),
			)).
			Assess("OperationSucceeded", funcs.AllOf(
				// This takes a little longer because it needs
				// the FunctionRevision above to become ready,
				// but it strictly retries with exponential
				// backoff. It doesn't watch FunctionRevisions.
				funcs.ResourcesHaveConditionWithin(60*time.Second, manifests, "operation.yaml", v1alpha1.Complete()),
				funcs.ResourcesHaveFieldValueWithin(30*time.Second, manifests, "operation.yaml", "status.appliedResourceRefs[0].name", "cool-map"),
				funcs.ResourceHasFieldValueWithin(30*time.Second, cm, "data[coolData]", "I'm cool!"),
				// TODO(negz): Test function output when we have
				// a function-dummy release containing this PR.
				// https://github.com/crossplane-contrib/function-dummy/pull/42
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

func TestBasicCronOperation(t *testing.T) {
	manifests := "test/e2e/manifests/ops/cronoperations/basic"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of a basic CronOperation that creates Operations on schedule.").
			WithLabel(LabelArea, LabelAreaOps).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteOps).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Assess("CreateCronOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "cronoperation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "cronoperation.yaml"),
			)).
			Assess("CronOperationCreatesOperation", funcs.AllOf(
				// Wait for CronOperation to create its first Operation
				// Since the schedule is every minute, this should happen quickly
				funcs.ResourcesHaveFieldValueWithin(90*time.Second, manifests, "cronoperation.yaml", "status.lastScheduleTime", funcs.Any),
				// Verify the CronOperation has the correct status
				funcs.ResourcesHaveConditionWithin(30*time.Second, manifests, "cronoperation.yaml", xpv1.ReconcileSuccess()),
			)).
			Assess("OperationSucceeds", funcs.AllOf(
				// Verify the CronOperation's lastSuccessfulTime gets updated
				funcs.ResourcesHaveFieldValueWithin(60*time.Second, manifests, "cronoperation.yaml", "status.lastSuccessfulTime", funcs.Any),
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
