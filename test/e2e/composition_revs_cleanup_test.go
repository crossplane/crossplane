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
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// TestCompositionRevsCleanup tests scenarios for composition revisions cleanup.
func TestCompositionRevsCleanup(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/composition-revs-cleanup"

	compositionRevList := composed.NewList(composed.FromReferenceToList(corev1.ObjectReference{
		APIVersion: "batch/v1",
		Kind:       "Job",
	}))

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests scenarios for compositions with realtime reconciles through MR updates.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("UpdateComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "composition-update-1.yaml"),
				funcs.ApplyResources(FieldManager, manifests, "composition-update-2.yaml"),
			)).
			Assess("CompositionRevisionCountWithin", funcs.ResourceCountWithin(compositionRevList, 70*time.Second, 1)).
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
