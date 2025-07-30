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

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

func TestMRDValidation(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/mrd/validation"

	cases := features.Table{
		{
			// A valid MRD should be created.
			Name:        "ValidNewMRDIsDefaulted",
			Description: "A valid MRD should be created with defaults.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrd-defaulted.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "mrd-defaulted.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrd-defaulted.yaml", v1alpha1.InactiveManaged()),
			),
		},
		{
			// A valid MRD should be created.
			Name:        "ValidNewMRDIsAccepted",
			Description: "A valid MRD should be created.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrd-valid.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "mrd-valid.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrd-valid.yaml", v1alpha1.InactiveManaged()),
			),
		},
		{
			// An update to a valid MRD should be accepted.
			Name:        "ValidUpdatedMRDIsAccepted",
			Description: "A valid update to an MRD should be accepted.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrd-valid-updated.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "mrd-valid-updated.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrd-valid-updated.yaml", v1alpha1.EstablishedManaged()),
			),
		},
		{
			// An update to an invalid MRD should be rejected.
			Name:        "InvalidMRDUpdateIsRejected",
			Description: "An invalid update to an MRD should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "mrd-valid-updated-invalid.yaml"),
		},
		{
			// An update to immutable MRD fields should be rejected.
			Name:        "ImmutableMRDFieldUpdateIsRejected",
			Description: "An update to immutable MRD field should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "mrd-immutable-updated.yaml"),
		},
		{
			// An invalid MRD should be rejected.
			Name:        "InvalidMRDIsRejected",
			Description: "An invalid MRD should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "mrd-invalid.yaml"),
		},
		{
			// An attempt to deactivate an active MRD should be rejected.
			Name:        "ActiveMRDCannotBeDeactivated",
			Description: "An attempt to change an active MRD to inactive should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "mrd-active-to-inactive.yaml"),
		},
	}
	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelArea, LabelAreaMRAP).
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithTeardown("DeleteValidMRD", funcs.AllOf(
				funcs.DeleteResources(manifests, "*-valid.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "*-valid.yaml"),
			)).
			Feature(),
	)
}
