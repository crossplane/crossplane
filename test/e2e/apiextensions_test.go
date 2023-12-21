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
	"os"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaAPIExtensions is applied to all features pertaining to API
// extensions (i.e. Composition, XRDs, etc).
const LabelAreaAPIExtensions = "apiextensions"

var (
	nopList = composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "nop.crossplane.io/v1alpha1",
		Kind:       "NopResource",
	}))
)

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

// TODO(negz): How do we want to handle beta features? They're on by default.
// Maybe in this case add a test suite that tests P&T when Functions are
// disabled?

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
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
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
				funcs.CompositeResourceMustMatchWithin(1*time.Minute, manifests, "claim.yaml", func(xr *composite.Unstructured) bool {
					labels := xr.GetLabels()
					_, barLabelExists := labels["bar"]
					annotations := xr.GetAnnotations()
					_, barAnnotationExists := annotations["test/bar"]
					p := fieldpath.Pave(xr.Object)
					_, err := p.GetValue("spec.tags.newtag")
					n, _ := p.GetStringArray("spec.numbers")
					return labels["foo"] == "1" && !barLabelExists && labels["foo2"] == "3" &&
						annotations["test/foo"] == "1" && !barAnnotationExists && annotations["test/foo2"] == "4" &&
						reflect.DeepEqual(n, []string{"one", "five"}) &&
						err != nil
				}),
				funcs.ClaimUnderTestMustNotChangeWithin(1*time.Minute),
				funcs.ResourcesHaveFieldValueWithin(5*time.Minute, manifests, "claim-update.yaml", "status.coolerField", "I'm cool!"),
				funcs.CompositeUnderTestMustNotChangeWithin(1*time.Minute),
			)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

// TestPropagateFieldsUpdatesToMR verifies that the following reported bugs are fixed
// * https://github.com/crossplane/crossplane/issues/3335
// * https://github.com/crossplane/crossplane/issues/4162
func TestPropagateFieldUpdatesToMR(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/propagate-field-updates-mr"
	updateMR := func(manifest string) func(ctx context.Context, klient klient.Client, obj *unstructured.Unstructured) error {
		return func(ctx context.Context, klient klient.Client, obj *unstructured.Unstructured) error {
			u := &unstructured.Unstructured{}
			if err := decoder.DecodeFile(os.DirFS(manifests), manifest, u); err != nil {
				return err
			}
			u.SetName(obj.GetName())

			for i := 0; i < 10; i++ {
				err := klient.Resources().GetControllerRuntimeClient().Patch(ctx, u, client.Apply, client.FieldOwner("provider"))
				if err == nil {
					return nil
				}
				if !kerrors.IsConflict(err) {
					return err
				}
				time.Sleep(1 * time.Second)
			}
			return nil
		}
	}
	mrGK := funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"})
	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy()),
				func(ctx context.Context, t *testing.T, e *envconf.Config) context.Context {
					// wait some time so that we can be sure
					// that CRD reconciliation is over
					time.Sleep(30 * time.Second)
					return ctx
				},
				// update CRD
				funcs.UpdateResources(manifests, "crd-update.yaml"),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyClaim(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("CompositeHasLabel", funcs.CompositeResourceMustMatchWithin(1*time.Minute, manifests, "claim.yaml", func(xr *composite.Unstructured) bool {
				return xr.GetLabels()["crossplane.io/composite"] == xr.GetName()
			})).
			Assess("MRGotCreated", funcs.AllOf(
				funcs.ComposedResourcesOfClaimHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.bArr[0].targetSelector.matchLabels[key2]", "foo", mrGK),
				funcs.ComposedResourcesOfClaimHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.someField", "someValue", mrGK),
			)).
			Assess("UpdateClaim", funcs.ApplyClaim(FieldManager, manifests, "claim-update.yaml")).
			Assess("MRGotUpdated", funcs.ComposedResourcesOfClaimHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.numbers", []any{"1", "2", "3"}, mrGK)).
			Assess("ProviderUpdatesMR", funcs.VisitComposedResourcesOfClaim(manifests, "claim.yaml", updateMR("mr-update.yaml"))).
			Assess("fof", funcs.ComposedResourcesOfClaimHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.bArr[0].targetRefs[0].name", "bar", mrGK)).
			Assess("RemoveFieldFromCompositionTemplate", funcs.ApplyResources(FieldManager, manifests, "composition-update.yaml")).
			Assess("MRFieldRemoved", funcs.AllOf(
				funcs.ComposedResourcesOfClaimHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.bArr[0].targetSelector.matchLabels[key2]", "foo", mrGK),
				funcs.ComposedResourcesOfClaimHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.bArr[0].targetRefs[0].name", "bar", mrGK),
				funcs.ComposedResourcesOfClaimHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.otherField", "otherValue", mrGK),
				funcs.ComposedResourcesOfClaimHaveNotFieldWithin(1*time.Minute, manifests, "claim.yaml", "spec.forProvider.fields.someField", mrGK),
			)).
			Assess("ClaimStatusUpdated", funcs.ResourcesHaveFieldValueWithin(2*time.Minute, manifests, "claim.yaml", "status.numbers", []any{"1", "2"})).
			Assess("ProviderUpdatesMR", funcs.VisitComposedResourcesOfClaim(manifests, "claim.yaml", updateMR("mr-update2.yaml"))).
			Assess("ClaimStatusUpdated", funcs.ResourcesHaveFieldValueWithin(2*time.Minute, manifests, "claim.yaml", "status.numbers", []any{"1", "2", "3", "4"})).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}
