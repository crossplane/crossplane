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

	"github.com/crossplane/crossplane/apis/daytwo/v1alpha1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaAPIDayTwo is applied to all features pertaining to API
// extensions (i.e. Composition, XRDs, etc).
const LabelAreaAPIDayTwo = "daytwo"

func TestOperationSimple(t *testing.T) {
	manifests := "test/e2e/manifests/daytwo/operation/simple"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of operation functions ensuring that the operation functions are called and the pipeline is marked succeeded.").
			WithLabel(LabelArea, LabelAreaAPIDayTwo).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Assess("CreateOperation", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "operation.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "operation.yaml"),
			)).
			Assess("OperationSucceeded",
				funcs.ResourcesHaveConditionWithin(60*time.Second, manifests, "operation.yaml", v1alpha1.Complete())).
			WithTeardown("DeleteOperation", funcs.AllOf(
				funcs.DeleteResources(manifests, "operation.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "operation.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}
