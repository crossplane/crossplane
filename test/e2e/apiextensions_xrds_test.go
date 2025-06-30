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

	apiextensionscommon "github.com/crossplane/crossplane/apis/apiextensions/common"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
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
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd-valid.yaml", apiextensionscommon.WatchingComposite()),
			),
		},
		{
			// An update to a valid XRD should be accepted.
			Name:        "ValidUpdatedXRDIsAccepted",
			Description: "A valid update to an XRD should be accepted.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xrd-valid-updated.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xrd-valid-updated.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd-valid-updated.yaml", apiextensionscommon.WatchingComposite()),
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
