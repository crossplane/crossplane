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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

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
			Assess("OperationIsComplete",
				funcs.ResourcesHaveConditionWithin(30*time.Minute, manifests, "operation.yaml", v1alpha1.Complete())).
			// TODO(negz): Test that the ConfigMap shows up in
			// status.appliedResourceRefs.
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
