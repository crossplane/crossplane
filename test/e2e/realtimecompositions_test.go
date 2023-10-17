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
	// SuiteRealtimeCompositions is the value for the config.LabelTestSuite
	// label to be assigned to tests that should be part of the Usage test
	// suite.
	SuiteRealtimeCompositions = "realtime-compositions"
)

func init() {
	environment.AddTestSuite(SuiteRealtimeCompositions,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-realtime-compositions}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteRealtimeCompositions, config.TestSuiteDefault},
		}),
	)
}

// TestRealtimeCompositions tests scenarios for compositions with realtime
// reconciles through MR updates.
func TestRealtimeCompositions(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/realtime-compositions"

	nopCrossplaneList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "nop.crossplane.io/v1alpha1",
		Kind:       "NopResource",
	}))
	nopList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "realtime-revision-selection.e2e.crossplane.io/v1alpha1",
		Kind:       "NopResource",
	}))
	xnopList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "realtime-revision-selection.e2e.crossplane.io/v1alpha1",
		Kind:       "XNopResource",
	}))
	withTestLabels := resources.WithLabelSelector(labels.FormatLabels(map[string]string{"realtime-compositions": "true"}))

	environment.Test(t,
		features.New(t.Name()).
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteRealtimeCompositions).
			WithSetup("EnableAlphaRealtimeCompositions", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteRealtimeCompositions)),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("CreateClaim", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "claim.yaml"),
				funcs.InBackground(funcs.LogResources(nopList, withTestLabels)),
				funcs.InBackground(funcs.LogResources(xnopList, withTestLabels)),
				funcs.InBackground(funcs.LogResources(nopCrossplaneList, withTestLabels)),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "claim.yaml", xpv1.Available()),
			)).
			Assess("UpdateMR", funcs.AllOf(
				funcs.ListedResourcesModifiedWith(nopCrossplaneList, 1, func(object k8s.Object) {
					anns := object.GetAnnotations()
					if anns == nil {
						anns = make(map[string]string)
					}
					anns["cool-field"] = "I'M COOL!"
					object.SetAnnotations(anns)
				}, withTestLabels),
			)).
			Assess("ClaimHasPatchedField",
				// 10 seconds is a long time for a realtime composition, but
				// considerably below the normal reconcile time for a XR.
				funcs.ResourcesHaveFieldValueWithin(10*time.Second, manifests, "claim.yaml", "status.coolerField", "I'M COOL!"),
			).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResources(manifests, "setup/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			// Disable our feature flag.
			WithTeardown("DisableAlphaRealtimeCompositions", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}
