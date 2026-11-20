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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"

	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// LabelAreaNamespaceRestricted is applied to all features pertaining to namespace-restricted operations.
// This does not include any particular feature (for now), but is used to check if Crossplane keeps functioning
// correctly when the --watch-cache-namespaced argument is set
const LabelAreaNamespaceRestricted = "ns-restricted"

// Tests that should be part of the test suite for the namespace restricted features
const SuiteNamespaceRestricted = "ns-restricted"

const (
	namespaceRestrictedManifestsDir = "test/e2e/manifests/namespace-restricted"
	otherNamespaceManifestName      = "other-namespace.yaml"
)

func init() {
	environment.AddTestSuite(SuiteNamespaceRestricted,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--watch-cache-namespaced}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteNamespaceRestricted, config.TestSuiteDefault},
		}),
	)
}

func TestNamespaceRestrictedBasicCompositionNamespaced(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/basic-namespaced"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the correct functioning of a namespaced XR ensuring that the composed resources are created, conditions are met, fields are patched, and resources are properly cleaned up when deleted.").
			WithLabel(LabelArea, LabelAreaNamespaceRestricted).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteNamespaceRestricted).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ApplyResources(FieldManager, namespaceRestrictedManifestsDir, otherNamespaceManifestName),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, namespaceRestrictedManifestsDir, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, namespaceRestrictedManifestsDir, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, namespaceRestrictedManifestsDir, "xr.yaml", xpv2.Available(), xpv2.ReconcileSuccess())).
			Assess("XRHasStatusField",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, namespaceRestrictedManifestsDir, "xr.yaml", "status.coolerField", "I'M COOLER!"),
			).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(namespaceRestrictedManifestsDir, "xr.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, namespaceRestrictedManifestsDir, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
				funcs.DeleteResourcesWithPropagationPolicy(namespaceRestrictedManifestsDir, otherNamespaceManifestName, metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, namespaceRestrictedManifestsDir, otherNamespaceManifestName),
			)).
			Feature(),
	)
}
