package e2e

import (
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

const (
	// SuiteEnvironmentConfig is the value for the
	// config.LabelTestSuite label to be assigned to tests that should be part
	// of the EnvironmentConfig test suite.
	SuiteEnvironmentConfigs = "environment-configs"

	manifestsFolderEnvironmentConfigs = "test/e2e/manifests/apiextensions/environment"
)

func init() {
	environment.AddTestSuite(SuiteEnvironmentConfigs,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-environment-configs}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{
				SuiteEnvironmentConfigs,
				// disabled default tests because we don't get any interaction
				// between environment configs and basic functionalities
				// config.TestSuiteDefault,
				// We only keep the lifecycle tests because they are the only
				// ones that are relevant for environment configs.
				TestSuiteLifecycle,
			},
		}),
	)
}

func TestEnvironmentConfigDefault(t *testing.T) {
	subfolder := "default"

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteEnvironmentConfigs).
			// Enable our feature flag.
			WithSetup("EnableAlphaEnvironmentConfigs", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteEnvironmentConfigs)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("CreateGlobalPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifestsFolderEnvironmentConfigs, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), xpv1.Available()),
			)).
			Assess("MRHasAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "2",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			WithTeardown("DeleteCreatedResources", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml"), nopList)).
			WithTeardown("DeleteGlobalPrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaEnvironmentConfig", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestEnvironmentResolutionOptional(t *testing.T) {
	subfolder := "resolutionOptional"

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteEnvironmentConfigs).
			// Enable our feature flag.
			WithSetup("EnableAlphaEnvironmentConfigs", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteEnvironmentConfigs)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("CreateGlobalPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifestsFolderEnvironmentConfigs, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), xpv1.Available()),
			)).
			Assess("MRHasAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "1",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			WithTeardown("DeleteCreatedResources", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml"), nopList)).
			WithTeardown("DeleteGlobalPrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaEnvironmentConfig", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestEnvironmentResolveIfNotPresent(t *testing.T) {
	subfolder := "resolveIfNotPresent"

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteEnvironmentConfigs).
			// Enable our feature flag.
			WithSetup("EnableAlphaEnvironmentConfigs", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteEnvironmentConfigs)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("CreateGlobalPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifestsFolderEnvironmentConfigs, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), xpv1.Available()),
			)).
			Assess("MRHasAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "2",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			Assess("CreateAdditionalEnvironmentConfigMatchingSelector", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "01-addedEnvironmentConfig.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "01-addedEnvironmentConfig.yaml")),
			)).
			Assess("SetAnnotationOnClaimToForceReconcile",
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), funcs.SetAnnotationMutateOption("e2e-reconcile-plz", time.Now().String()))).
			Assess("MRHasStillAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(5*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "2",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			WithTeardown("DeleteCreatedResources", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml"), nopList)).
			WithTeardown("DeleteGlobalPrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaEnvironmentConfig", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestEnvironmentResolveAlways(t *testing.T) {
	subfolder := "resolveAlways"

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteEnvironmentConfigs).
			// Enable our feature flag.
			WithSetup("EnableAlphaEnvironmentConfigs", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteEnvironmentConfigs)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("CreateGlobalPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifestsFolderEnvironmentConfigs, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), xpv1.Available()),
			)).
			Assess("MRHasAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "2",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			Assess("CreateAdditionalEnvironmentConfigMatchingSelector", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "01-addedEnvironmentConfig.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "01-addedEnvironmentConfig.yaml")),
			)).
			Assess("SetAnnotationOnClaimToForceReconcile",
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), funcs.SetAnnotationMutateOption("e2e-reconcile-plz", time.Now().String()))).
			Assess("MRHasUpdatedAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(5*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "3",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			WithTeardown("DeleteCreatedResources", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml"), nopList)).
			WithTeardown("DeleteGlobalPrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaEnvironmentConfig", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestEnvironmentConfigMultipleMaxMatchNil(t *testing.T) {
	subfolder := "multipleModeMaxMatchNil"

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteEnvironmentConfigs).
			// Enable our feature flag.
			WithSetup("EnableAlphaEnvironmentConfigs", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteEnvironmentConfigs)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("CreateGlobalPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifestsFolderEnvironmentConfigs, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), xpv1.Available()),
			)).
			Assess("MRHasAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "3",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			WithTeardown("DeleteCreatedResources", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml"), nopList)).
			WithTeardown("DeleteGlobalPrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaEnvironmentConfig", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

func TestEnvironmentConfigMultipleMaxMatch1(t *testing.T) {
	subfolder := "multipleModeMaxMatch1"

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteEnvironmentConfigs).
			// Enable our feature flag.
			WithSetup("EnableAlphaEnvironmentConfigs", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteEnvironmentConfigs)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("CreateGlobalPrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifestsFolderEnvironmentConfigs, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml")),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesCreatedWithin(30*time.Second, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml")),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"), xpv1.Available()),
			)).
			Assess("MRHasAnnotation",
				funcs.ComposedResourcesHaveFieldValueWithin(2*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "00-claim.yaml"),
					"metadata.annotations[valueFromEnv]", "2",
					funcs.FilterByGK(schema.GroupKind{Group: "nop.crossplane.io", Kind: "NopResource"}))).
			WithTeardown("DeleteCreatedResources", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "*.yaml")),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifestsFolderEnvironmentConfigs, filepath.Join(subfolder, "setup/*.yaml"), nopList)).
			WithTeardown("DeleteGlobalPrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifestsFolderEnvironmentConfigs, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaEnvironmentConfig", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}
