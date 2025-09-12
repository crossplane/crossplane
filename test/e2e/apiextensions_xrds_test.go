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

	"sigs.k8s.io/e2e-framework/pkg/features"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

func TestXRDValidation(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/xrd/validation"

	cases := features.Table{
		{
			// A valid XRD should be created.
			Name:        "ValidNewXRDIsAccepted",
			Description: "A valid XRD should be created.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xrd-valid.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xrd-valid.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd-valid.yaml", apiextensionsv1.WatchingComposite()),
			),
		},
		{
			// An update to a valid XRD should be accepted.
			Name:        "ValidUpdatedXRDIsAccepted",
			Description: "A valid update to an XRD should be accepted.",
			Assessment: funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xrd-valid-updated.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xrd-valid-updated.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xrd-valid-updated.yaml", apiextensionsv1.WatchingComposite()),
			),
		},
		{
			// An update to an invalid XRD should be rejected.
			Name:        "InvalidXRDUpdateIsRejected",
			Description: "An invalid update to an XRD should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "xrd-valid-updated-invalid.yaml"),
		},
		{
			// An update to immutable XRD fields should be rejected.
			Name:        "ImmutableXRDFieldUpdateIsRejected",
			Description: "An update to immutable XRD field should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "xrd-immutable-updated.yaml"),
		},
		{
			// An invalid XRD should be rejected.
			Name:        "InvalidXRDIsRejected",
			Description: "An invalid XRD should be rejected.",
			Assessment:  funcs.ResourcesFailToApply(FieldManager, manifests, "xrd-invalid.yaml"),
		},
	}
	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithTeardown("DeleteValidComposition", funcs.AllOf(
				funcs.DeleteResources(manifests, "*-valid.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "*-valid.yaml"),
			)).
			Feature(),
	)
}

func TestXRDScopeChange(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/xrd/scope-change"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that XRD scope changes work without requiring manual Crossplane restart. This validates the RESTMapper cache invalidation workaround.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreateNamespacedXRD", funcs.AllOf(
				// Create XRD with Namespaced scope
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition-namespaced.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("NamespacedXRCanBeCreated", funcs.AllOf(
				// Create a namespaced XR
				funcs.ApplyResources(FieldManager, manifests, "xr-namespaced.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr-namespaced.yaml"),
			)).
			Assess("NamespacedXRBecomesReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr-namespaced.yaml", xpv1.Available(), xpv1.ReconcileSuccess())).
			Assess("ScopeChangeToClusterAllowsXRCreation", funcs.AllOf(
				// Clean up the namespaced XR first
				funcs.DeleteResources(manifests, "xr-namespaced.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "xr-namespaced.yaml"),

				// Delete the namespaced XRD
				funcs.DeleteResources(manifests, "setup/definition-namespaced.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "setup/definition-namespaced.yaml"),

				// Create XRD with Cluster scope (same GVK, different scope)
				funcs.ApplyResources(FieldManager, manifests, "definition-cluster.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "definition-cluster.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "definition-cluster.yaml", apiextensionsv1.WatchingComposite()),

				// Create a cluster-scoped XR - this should work without manual restart
				// This tests that our RESTMapper cache invalidation workaround works
				funcs.ApplyResources(FieldManager, manifests, "xr-cluster.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr-cluster.yaml"),
			)).
			Assess("ClusterXRBecomesReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr-cluster.yaml", xpv1.Available(), xpv1.ReconcileSuccess())).
			WithTeardown("DeleteScopeChangeResources", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr-cluster.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "xr-cluster.yaml"),
				funcs.DeleteResources(manifests, "definition-cluster.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "definition-cluster.yaml"),
			)).
			WithTeardown("DeleteSetupResources", funcs.AllOf(
				funcs.DeleteResources(manifests, "setup/composition.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "setup/composition.yaml"),
				funcs.DeleteResources(manifests, "setup/functions.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "setup/functions.yaml"),
			)).
			Feature(),
	)
}
