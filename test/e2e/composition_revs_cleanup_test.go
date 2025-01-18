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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

const (
	// composition-revs-cleanup is the value for the config.LabelTestSuite
	// label to be assigned to tests that should be part of the
	// Composition revs cleanup test suite.
	SuiteCompositionRevsCleanup = "composition-revs-cleanup"
)

func init() {
	environment.AddTestSuite(SuiteCompositionRevsCleanup,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set job.removeUnusedCompositionRevision.keepTopNItems=1"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteCompositionRevsCleanup, config.TestSuiteDefault},
		}),
	)
}

// TestCompositionRevsCleanup tests scenarios for composition revisions cleanup.
func TestCompositionRevsCleanup(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/" + SuiteCompositionRevsCleanup

	compositionRevList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "apiextensions.crossplane.io/v1",
		Kind:       "CompositionRevision",
	}))
	withTestLabels := resources.WithLabelSelector(labels.FormatLabels(map[string]string{SuiteCompositionRevsCleanup: "true"}))

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests scenarios for compositions with realtime reconciles through MR updates.").
			WithLabel(LabelStage, LabelStageAlpha).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteCompositionRevsCleanup).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("UpdateComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-update-1.yaml"),
				funcs.ApplyResources(FieldManager, manifests, "composition-update-2.yaml"),
				funcs.InBackground(funcs.LogResources(compositionRevList, withTestLabels)),
			)).
			Assess("CompositionRevisionCountWithin", funcs.ResourceCountWithin(compositionRevList, 90*time.Second, 1)).
			WithTeardown("DeleteClaim", funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			WithTeardown("DisableAlphaRealtimeCompositions", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()), // Disable our feature flag.
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
			)).
			Feature(),
	)
}
