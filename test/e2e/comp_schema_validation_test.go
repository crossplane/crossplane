package e2e

import (
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/features"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

func TestCompositionValidation(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/validation"

	cases := features.Table{
		{
			// A valid Composition should be created when validated in strict mode.
			Name:        "ValidCompositionIsAcceptedStrictMode",
			Description: "A valid Composition should be created when validated in strict mode.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-valid.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition-valid.yaml"),
			),
		},
		{
			// A valid Composition should be created when validated in strict mode.
			Name:        "ValidCompositionWithAToJsonTransformIsAcceptedStrictMode",
			Description: "A valid Composition defining a valid ToJson String transform should be created when validated in strict mode.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-transform-tojson-valid.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition-transform-tojson-valid.yaml"),
			),
		},
		{
			// An invalid Composition should be rejected when validated in strict mode.
			Name:       "InvalidCompositionIsRejectedStrictMode",
			Assessment: funcs.ResourcesFailToApply(FieldManager, manifests, "composition-invalid.yaml"),
		},
		{
			// An invalid Composition should be accepted when validated in warn mode.
			Name: "InvalidCompositionIsAcceptedWarnMode",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-warn-valid.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "composition-warn-valid.yaml"),
			),
		},
		{
			// A composition that updates immutable fields should be rejected when validated in strict mode.
			Name:       "ImmutableCompositionFieldUpdateIsRejectedStrictMode",
			Assessment: funcs.ResourcesFailToApply(FieldManager, manifests, "composition-invalid-immutable.yaml"),
		},
	}
	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithTeardown("DeleteValidComposition", funcs.AllOf(
				funcs.DeleteResources(manifests, "*-valid.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "*-valid.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}
