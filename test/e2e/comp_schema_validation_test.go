package e2e

import (
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

const (
	// SuiteCompositionWebhookSchemaValidation is the value for the
	// config.LabelTestSuite label to be assigned to tests that should be part
	// of the Composition Webhook Schema Validation test suite.
	SuiteCompositionWebhookSchemaValidation = "composition-webhook-schema-validation"
)

func init() {
	environment.AddTestSuite(SuiteCompositionWebhookSchemaValidation,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-composition-webhook-schema-validation}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteCompositionWebhookSchemaValidation, config.TestSuiteDefault},
		}),
	)
}

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
	}
	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteCompositionWebhookSchemaValidation).
			// Enable our feature flag.
			WithSetup("EnableAlphaCompositionValidation", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteCompositionWebhookSchemaValidation)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithTeardown("DeleteValidComposition", funcs.AllOf(
				funcs.DeleteResources(manifests, "*-valid.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "*-valid.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaCompositionValidation", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}
