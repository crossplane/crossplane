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
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaAPIExtensions is applied to all features pertaining to API
// extensions (i.e. Composition, XRDs, etc).
const LabelAreaAPIExtensions = "apiextensions"

// TestComposition tests Crossplane's Composition functionality.
func TestComposition(t *testing.T) {
	// Test that a claim using a very minimal Composition (with no patches,
	// transforms, or functions) will become available when its composed
	// resources do.
	manifests := "test/e2e/manifests/apiextensions/composition/minimal"
	minimal := features.Table{
		{
			Name: "PrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
	}

	// Test that a claim using patch-and-transform Composition will become
	// available when its composed resources do, and have a field derived from
	// the patch.
	manifests = "test/e2e/manifests/apiextensions/composition/patch-and-transform"
	pandt := features.Table{
		{
			Name: "PrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
		},
		{
			Name:       "ClaimHasPatchedField",
			Assessment: funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim.yaml", "status.coolerField", "I'M COOL!"),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
	}

	setup := funcs.ReadyToTestWithin(1*time.Minute, namespace)
	environment.Test(t,
		minimal.Build("Minimal").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			Setup(setup).Feature(),
		pandt.Build("PatchAndTransform").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			Setup(setup).Feature(),
	)
}

func TestValidation(t *testing.T) {

	// A valid Composition should be created when validated in strict mode.
	manifests := "test/e2e/manifests/apiextensions/validation/composition-schema-valid"
	valid := features.Table{
		{
			Name: "PrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{
			Name:       "ProviderIsHealthy",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
		},
		{
			Name: "CompositionIsCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition.yaml"),
			),
		},
		{
			Name: "CompositionIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "composition.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "composition.yaml"),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
	}

	// An invalid Composition should be rejected when validated in strict mode.
	manifests = "test/e2e/manifests/apiextensions/validation/composition-schema-invalid"
	invalid := features.Table{
		{
			Name: "PrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{
			Name:       "ProviderIsHealthy",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
		},
		{
			Name: "CompositionIsCreated",
			Assessment: funcs.AllOf(
				funcs.ResourcesFailToApply(FieldManager, manifests, "composition.yaml"),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
	}

	// Enable our feature flag.
	setup := funcs.AllOf(
		funcs.AsFeaturesFunc(funcs.HelmUpgrade(HelmOptions(helm.WithArgs("--set args={--debug,--enable-composition-webhook-schema-validation}"))...)),
		funcs.ReadyToTestWithin(1*time.Minute, namespace),
	)

	// Disable our feature flag.
	teardown := funcs.AllOf(
		funcs.AsFeaturesFunc(funcs.HelmUpgrade(HelmOptions()...)),
		funcs.ReadyToTestWithin(1*time.Minute, namespace),
	)

	environment.Test(t,
		valid.Build("ValidComposition").
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			Setup(setup).
			Teardown(teardown).
			Feature(),
		invalid.Build("InvalidComposition").
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			Setup(setup).
			Teardown(teardown).
			Feature(),
	)
}
