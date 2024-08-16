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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

const (
	// SuiteSSAClaims is the value for the config.LabelTestSuite label to be
	// assigned to tests that should be part of the SSAClaims  test suite.
	SuiteSSAClaims = "ssa-claims"
)

func init() {
	environment.AddTestSuite(SuiteSSAClaims,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-ssa-claims}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteSSAClaims, config.TestSuiteDefault},
		}),
	)
}

// LabelAreaAPIExtensions is applied to all features pertaining to API
// extensions (i.e. Composition, XRDs, etc).
const LabelAreaAPIExtensions = "apiextensions"

var nopList = composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
	APIVersion: "nop.crossplane.io/v1alpha1",
	Kind:       "NopResource",
}))

// TestCompositionMinimal tests Crossplane's Composition functionality,
// checking that a claim using a very minimal Composition (with no patches,
// transforms, or functions) will become available when its composed
// resources do.
func TestCompositionMinimal(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/minimal"

	claimList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "nop.example.org/v1alpha1",
		Kind:       "NopResource",
	}))
	xrList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "nop.example.org/v1alpha1",
		Kind:       "XNopResource",
	}))

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.InBackground(funcs.LogResources(claimList)),
				funcs.InBackground(funcs.LogResources(xrList)),
				funcs.InBackground(funcs.LogResources(nopList)),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

// TestCompositionInvalidComposed tests Crossplane's Composition functionality,
// checking that although a composed resource is invalid, i.e. it didn't apply
// successfully.
func TestCompositionInvalidComposed(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/invalid-composed"

	xrList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "example.org/v1alpha1",
		Kind:       "XParent",
	}), composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "example.org/v1alpha1",
		Kind:       "XChild",
	}))

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.InBackground(funcs.LogResources(xrList)),
				funcs.InBackground(funcs.LogResources(nopList)),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRStillAnnotated", funcs.AllOf(
				// Check the XR it has metadata.annotations set
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "metadata.annotations[exampleVal]", "foo"),
			)).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

// TestCompositionPatchAndTransform tests Crossplane's Composition functionality,
// checking that a claim using patch-and-transform Composition will become
// available when its composed resources do, and have a field derived from
// the patch.
func TestCompositionPatchAndTransform(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/patch-and-transform"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("ClaimHasPatchedField",
				funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim.yaml", "status.coolerField", "I'M COOL!"),
			).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

// TestCompositionRealtimeRevisionSelection tests Crossplane's Composition
// functionality to react in realtime to changes in a Composition by selecting
// the new CompositionRevision and reconcile the XRs.
func TestCompositionRealtimeRevisionSelection(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/realtime-revision-selection"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			).
			Assess("UpdateComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-update.yaml"),
			)).
			Assess("ClaimHasPatchedField",
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "claim.yaml", "status.coolerField", "I'M COOL!"),
			).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestCompositionFunctions(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/functions"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			)).
			Assess("ClaimIsReady",
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available())).
			Assess("ClaimHasPatchedField",
				funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim.yaml", "status.coolerField", "I'M COOLER!"),
			).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

func TestPropagateFieldsRemovalToXR(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/propagate-field-removals"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteSSAClaims).
			WithSetup("EnableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteSSAClaims)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("UpdateClaim", funcs.ApplyClaim(FieldManager, manifests, "claim-update.yaml")).
			Assess("FieldsRemovalPropagatedToXR", funcs.AllOf(
				// Updates and deletes are propagated claim -> XR.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo2]", "3"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo2]", "4"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[0]", "one"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[1]", "five"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[2]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[tag]", "v1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[newtag]", funcs.NotFound),
				funcs.ClaimUnderTestMustNotChangeWithin(1*time.Minute),
				// Status is propagated XR -> claim.
				funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim-update.yaml", "status.coolerField", "I'm cool!"),
				funcs.CompositeUnderTestMustNotChangeWithin(1*time.Minute),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			WithTeardown("DisableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()), // Disable our feature flag.
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestPropagateFieldsRemovalToXRAfterUpgrade(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/propagate-field-removals"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteSSAClaims).
			// SSA claims are always enabled in this test suite, so we need to
			// explicitly disable them first before we create anything.
			WithSetup("DisableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(config.TestSuiteDefault)), // Disable our feature flag.
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			// Note that unlike TestPropagateFieldsRemovalToXR above, here we
			// enable SSA _after_ creating the claim. Our goal is to test that
			// we can still remove fields from the XR when we've upgraded the
			// field managers from CSA to SSA. If we didn't upgrade successfully
			// would end up sharing ownership with the old CSA field manager.
			Assess("EnableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteSSAClaims)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Assess("UpdateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim-update.yaml"),
				funcs.ClaimUnderTestMustNotChangeWithin(1*time.Minute),
			)).
			Assess("FieldsRemovalPropagatedToXR", funcs.AllOf(
				// Updates and deletes are propagated claim -> XR.
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[foo2]", "3"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo]", "1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/bar]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.annotations[test/foo2]", "4"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[0]", "one"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[1]", "five"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.numbers[2]", funcs.NotFound),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[tag]", "v1"),
				funcs.CompositeResourceHasFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.parameters.tags[newtag]", funcs.NotFound),
				funcs.ClaimUnderTestMustNotChangeWithin(1*time.Minute),
				// Status is propagated XR -> claim.
				funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim-update.yaml", "status.coolerField", "I'm cool!"),
				funcs.CompositeUnderTestMustNotChangeWithin(1*time.Minute),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			WithTeardown("DisableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()), // Disable our feature flag.
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

// TestPropagateFieldsRemovalToComposed tests Crossplane's end-to-end SSA syncing
// functionality of clear propagation of fields from claim->XR->MR, when existing
// composition and resources are migrated from native P-and-T to functions pipeline mode.
func TestPropagateFieldsRemovalToComposed(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/propagate-field-removals"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteSSAClaims).
			WithSetup("EnableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteSSAClaims)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("ConvertToPipelineCompositionUpgrade", funcs.ApplyResources(FieldManager, manifests, "composition-xfn.yaml")).
			Assess("UpdateClaim", funcs.ApplyClaim(FieldManager, manifests, "claim-update.yaml")).
			Assess("FieldsRemovalPropagatedToMR", funcs.AllOf(
				// field removals and updates are propagated claim -> XR -> MR, after converting composition from native to pipeline mode
				funcs.ComposedResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml",
					"spec.forProvider.fields.tags[newtag]", funcs.NotFound,
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"})),
				funcs.ComposedResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml",
					"spec.forProvider.fields.tags[tag]", "v1",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"})),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			WithTeardown("DisableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()), // Disable our feature flag.
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestCompositionSelection(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/composition-selection"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteSSAClaims).
			WithSetup("EnableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteSSAClaims)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
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
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			WithTeardown("DisableSSAClaims", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()), // Disable our feature flag.
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestBindToExistingXR(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/bind-existing-xr"
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			// Create an XR we'll later bind to.
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "xr.yaml", xpv1.Available()),
			)).
			// Make sure our fields are set to the XR's values.
			Assess("XRFieldHasOriginalValues", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.coolField", "Set by XR"),
			)).
			// Create a claim that explicitly asks to bind to the above XR.
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("XRIsBoundToClaim", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.claimRef.name", "bind-existing-xr"),
			)).
			Assess("XRFieldChangesToClaimValue", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "spec.coolField", "Set by claim"),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),

				// Deleting the claim should delete the XR.
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}
