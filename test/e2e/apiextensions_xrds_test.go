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

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

func TestXRDValidation(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/xrd/validation"

	cases := features.Table{
		{
			// A valid XRD should be created.
			Name:        "ValidNewXRDIsAccepted",
			Description: "A valid XRD should be created.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xrd-valid.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xrd-valid.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd-valid.yaml", apiextensionsv1.WatchingComposite()),
			),
		},
		{
			// An update to a valid XRD should be accepted.
			Name:        "ValidUpdatedXRDIsAccepted",
			Description: "A valid update to an XRD should be accepted.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xrd-valid-updated.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xrd-valid-updated.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd-valid-updated.yaml", apiextensionsv1.WatchingComposite()),
			),
		},
		{
			// An update to an invalid XRD should be rejected.
			Name:        "InvalidXRDUpdateIsRejected",
			Description: "An invalid update to an XRD should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "xrd-valid-updated-invalid.yaml"),
		},
		{
			// An update to immutable XRD fields should be rejected.
			Name:        "ImmutableXRDFieldUpdateIsRejected",
			Description: "An update to immutable XRD field should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "xrd-immutable-updated.yaml"),
		},
		{
			// An invalid XRD should be rejected.
			Name:        "InvalidXRDIsRejected",
			Description: "An invalid XRD should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "xrd-invalid.yaml"),
		},
	}
	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithTeardown("DeleteValidComposition", funcs.AllOf(
				funcs.DeleteResources(manifests, "*-valid.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "*-valid.yaml"),
			)).
			Feature(),
	)
}

func TestXRDReferenceableVersionChange(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/xrd/referenceable-version-change"

	// TODO(negz): https://github.com/crossplane/crossplane/issues/6805
	// This test would be more robust if it could verify that the WatchingComposite
	// condition's observedGeneration changes from 1 to 2 when the XRD is updated.
	// Currently we can only see this in the test logs due to the observedGeneration
	// workaround in ResourcesHaveConditionWithin.

	environment.Test(t,
		features.NewWithDescription(
			"XRDReferenceableVersionChange",
			"Controller restarts on XRD generation change; composition selection switches from v1 to v2.",
		).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreateXRDWithV1Referenceable", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/xrd-v1-referenceable.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXRBeforeChange", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr-before-change.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr-before-change.yaml"),
				// XR should select v1 composition because referenceable version is v1
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr-before-change.yaml", "spec.crossplane.compositionRef.name", "test-composition-v1"),
			)).
			Assess("UpdateXRDToV2Referenceable", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xrd-v2-referenceable.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xrd-v2-referenceable.yaml"),
				// Wait for controller to restart with new generation
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd-v2-referenceable.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXRAfterChange", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr-after-change.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr-after-change.yaml"),
				// XR should select v2 composition because referenceable version is v2
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr-after-change.yaml", "spec.crossplane.compositionRef.name", "test-composition-v2"),
			)).
			WithTeardown("DeleteResources", funcs.AllOf(
				// Delete XRs first, then XRD and compositions
				funcs.DeleteResources(manifests, "xr-*.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "xr-*.yaml"),
				funcs.DeleteResources(manifests, "xrd-*.yaml"),
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "xrd-*.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
