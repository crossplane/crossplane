package e2e

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
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
	// SuiteUsage is the value for the config.LabelTestSuite label to be
	// assigned to tests that should be part of the Usage test suite.
	SuiteUsage = "usage"
)

func init() {
	environment.AddTestSuite(SuiteUsage,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-usages}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteUsage, config.TestSuiteDefault},
		}),
	)
}

// TestUsageStandalone tests scenarios for Crossplane's `Usage` resource without
// a composition involved.
func TestUsageStandalone(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/usage/standalone"

	cases := features.Table{
		{
			// Deletion of a (used) resource should be blocked if there is a Usage relation with a using resource defined.
			Name: "UsageBlockedByUsingResource",
			Assessment: funcs.AllOf(
				// Create using and used managed resources together with a usage.
				funcs.ApplyResources(FieldManager, manifests, "with-by/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-by/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-by/usage.yaml", xpv1.Available()),

				// Deletion of used resource should be blocked by usage.
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-by/used.yaml"),

				// Deletion of using resource should clear usage.
				funcs.DeleteResources(manifests, "with-by/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by/usage.yaml"),
				// We have "replayDeletion: true" on the usage, deletion of used resource should be replayed after usage is cleared.
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "with-by/used.yaml"),
			),
		},
		{
			// Deletion of a (protected) resource should be blocked if there is a Usage with a reason defined.
			Name: "UsageBlockedWithReason",
			Assessment: funcs.AllOf(
				// Create protected managed resources together with a usage.
				funcs.ApplyResources(FieldManager, manifests, "with-reason/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-reason/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-reason/usage.yaml", xpv1.Available()),

				// Deletion of protected resource should be blocked by usage.
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-reason/used.yaml"),

				// Deletion of usage should clear usage.
				funcs.DeleteResources(manifests, "with-reason/usage.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/usage.yaml"),

				// Deletion of protected resource should be allowed after usage is cleared.
				funcs.DeleteResources(manifests, "with-reason/used.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/used.yaml"),
			),
		},
	}

	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteUsage).
			// Enable the usage feature flag.
			WithSetup("EnableAlphaUsages", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteUsage)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaUsages", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}

// TestUsageComposition tests scenarios for Crossplane's `Usage` resource as part
// of a composition.
func TestUsageComposition(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/usage/composition"

	usageList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "apiextensions.crossplane.io/v1alpha1",
		Kind:       "Usage",
	}))

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteUsage).
			// Enable the usage feature flag.
			WithSetup("EnableAlphaUsages", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteUsage)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("ClaimCreatedAndReady", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("UsedResourceHasInUseLabel", funcs.AllOf(
				funcs.ComposedResourcesHaveFieldValueWithin(1*time.Minute, manifests, "claim.yaml", "metadata.labels[crossplane.io/in-use]", "true", func(object k8s.Object) bool {
					return object.GetLabels()["usage"] == "used"
				}),
			)).
			Assess("ClaimDeleted", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "claim.yaml"),
			)).
			// NOTE(turkenh): At this point, the claim is deleted and hence the
			// garbage collector started attempting to delete all composed
			// resources. With the help of a finalizer (namely
			// `delay-deletion-of-using-resource`, see in the composition),
			// we know that the using resource is still there and hence the
			// deletion of the used resource should be blocked. We will assess
			// that below.
			Assess("OthersDeletedExceptUsed", funcs.AllOf(
				// Using resource should have a deletion timestamp (i.e. deleted by the garbage collector).
				funcs.ListedResourcesValidatedWithin(1*time.Minute, nopList, 1, func(object k8s.Object) bool {
					return object.GetDeletionTimestamp() != nil
				}, resources.WithLabelSelector(labels.FormatLabels(map[string]string{"usage": "using"}))),
				// Usage resource should not have a deletion timestamp since it is owned by the using resource.
				funcs.ListedResourcesValidatedWithin(1*time.Minute, usageList, 1, func(object k8s.Object) bool {
					return object.GetDeletionTimestamp() == nil
				}),
				// Used resource should not have a deletion timestamp since it is still in use.
				funcs.ListedResourcesValidatedWithin(1*time.Minute, nopList, 1, func(object k8s.Object) bool {
					return object.GetDeletionTimestamp() == nil
				}, resources.WithLabelSelector(labels.FormatLabels(map[string]string{"usage": "used"}))),
			)).
			Assess("UsingDeletedAllGone", funcs.AllOf(
				// Remove the finalizer from the using resource.
				funcs.ListedResourcesModifiedWith(nopList, 1, func(object k8s.Object) {
					object.SetFinalizers(nil)
				}, resources.WithLabelSelector(labels.FormatLabels(map[string]string{"usage": "using"}))),
				// All composed resources should now be deleted including the Usage itself.
				funcs.ListedResourcesDeletedWithin(2*time.Minute, nopList),
				funcs.ListedResourcesDeletedWithin(2*time.Minute, usageList),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaUsages", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}
