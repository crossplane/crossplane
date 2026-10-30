/*
Copyright 2026 The Crossplane Authors.

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
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// Tests that should be part of the test suite for the alpha function-controlled
// deletion feature, which adds a deletionPolicy to Compositions.
const SuiteFunctionControlledDeletion = "function-controlled-deletion"

func init() {
	environment.AddTestSuite(SuiteFunctionControlledDeletion,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-function-controlled-deletion}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteFunctionControlledDeletion, config.TestSuiteDefault},
		}),
	)
}

// The composed ConfigMaps carry this finalizer so the tests can see when they're
// deleted.
const deletionTestFinalizer = "e2e.crossplane.io/hold"

// heldForDeletion returns true if the supplied object is being deleted, and
// only the test finalizer is holding it.
func heldForDeletion(o k8s.Object) bool {
	return o.GetDeletionTimestamp() != nil && slices.Equal(o.GetFinalizers(), []string{deletionTestFinalizer})
}

// release removes the test finalizer, letting Kubernetes delete the object.
func release(o k8s.Object) {
	o.SetFinalizers(slices.DeleteFunc(o.GetFinalizers(), func(f string) bool { return f == deletionTestFinalizer }))
}

func TestCompositionDeletionPolicyForeground(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/deletion-policy-foreground"

	// Crossplane removes crossplane.io/composite before it deletes a composed
	// resource, so select the composed ConfigMaps by test labels.
	foreground := resources.WithLabelSelector("e2e.crossplane.io/deletion-policy=foreground")
	background := resources.WithLabelSelector("e2e.crossplane.io/deletion-policy=background")

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that an XR whose Composition has the Foreground deletion policy is kept until Crossplane has deleted its composed resources, and that one with the Background policy is deleted immediately.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteFunctionControlledDeletion).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXRs", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr-*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr-*.yaml"),
			)).
			Assess("XRsAreReady",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "xr-*.yaml", xpv2.Available(), xpv2.ReconcileSuccess()),
			).
			Assess("XRsRecordTheirDeletionPolicies", funcs.AllOf(
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr-foreground.yaml", "status.crossplane.deletionPolicy", "Foreground"),
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr-background.yaml", "status.crossplane.deletionPolicy", "Background"),
			)).
			Assess("ComposedResourcesAreCreated", funcs.AllOf(
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 2, func(k8s.Object) bool { return true }, foreground),
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 2, func(k8s.Object) bool { return true }, background),
			)).
			// With the Background policy Crossplane doesn't wait for the XR's
			// composed resources. Kubernetes deletes them once the XR is gone.
			Assess("DeleteBackgroundXR", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr-background.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr-background.yaml"),
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 2, heldForDeletion, background),
			)).
			Assess("ReleaseBackgroundComposedResources", funcs.AllOf(
				funcs.ListedResourcesModifiedWith(&corev1.ConfigMapList{}, 2, release, background),
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &corev1.ConfigMapList{}, background),
			)).
			// With the Foreground policy Crossplane deletes the XR's composed
			// resources, and keeps the XR until they're gone.
			Assess("DeleteForegroundXR", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr-foreground.yaml"),
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 2, heldForDeletion, foreground),
			)).
			Assess("ForegroundXRWaitsForComposedResources", funcs.AllOf(
				funcs.SleepFor(10*time.Second),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr-foreground.yaml", xpv2.Deleting()),
			)).
			Assess("ReleaseForegroundComposedResources", funcs.AllOf(
				funcs.ListedResourcesModifiedWith(&corev1.ConfigMapList{}, 2, release, foreground),
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &corev1.ConfigMapList{}, foreground),
			)).
			Assess("ForegroundXRIsDeleted",
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr-foreground.yaml"),
			).
			// The assessments delete the XRs. If one failed, don't leave
			// composed resources stuck behind the test finalizer.
			WithTeardown("ReleaseComposedResources", funcs.AllOf(
				funcs.ListedResourcesModifiedWith(&corev1.ConfigMapList{}, 0, release, foreground),
				funcs.ListedResourcesModifiedWith(&corev1.ConfigMapList{}, 0, release, background),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestCompositionDeletionPolicyPipeline(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/deletion-policy-pipeline"

	// Crossplane removes crossplane.io/composite before it deletes a composed
	// resource, so select the composed ConfigMaps by test labels.
	composed := resources.WithLabelSelector("e2e.crossplane.io/deletion-policy=pipeline")
	first := resources.WithLabelSelector("e2e.crossplane.io/deletion-policy=pipeline,e2e.crossplane.io/resource=first")
	second := resources.WithLabelSelector("e2e.crossplane.io/deletion-policy=pipeline,e2e.crossplane.io/resource=second")

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that an XR whose Composition has the Pipeline deletion policy runs its function pipeline while it's deleted, so the function decides the order its composed resources are deleted in.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteFunctionControlledDeletion).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "xr.yaml", xpv2.Available(), xpv2.ReconcileSuccess()),
			).
			Assess("XRRecordsItsDeletionPolicy",
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.crossplane.deletionPolicy", "Pipeline"),
			).
			Assess("ComposedResourcesAreCreated",
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 2, func(k8s.Object) bool { return true }, composed),
			).
			// The function deletes the second ConfigMap first, and keeps the
			// first until the second is gone.
			Assess("DeleteXR", funcs.AllOf(
				funcs.DeleteResources(manifests, "xr.yaml"),
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 1, heldForDeletion, second),
			)).
			Assess("FirstWaitsForSecond", funcs.AllOf(
				funcs.SleepFor(10*time.Second),
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 1, func(o k8s.Object) bool { return o.GetDeletionTimestamp() == nil }, first),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Deleting()),
			)).
			Assess("ReleaseSecond", funcs.AllOf(
				funcs.ListedResourcesModifiedWith(&corev1.ConfigMapList{}, 1, release, second),
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &corev1.ConfigMapList{}, second),
			)).
			Assess("FirstIsDeleted", funcs.AllOf(
				funcs.ListedResourcesValidatedWithin(1*time.Minute, &corev1.ConfigMapList{}, 1, heldForDeletion, first),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Deleting()),
			)).
			Assess("ReleaseFirst", funcs.AllOf(
				funcs.ListedResourcesModifiedWith(&corev1.ConfigMapList{}, 1, release, first),
				funcs.ListedResourcesDeletedWithin(1*time.Minute, &corev1.ConfigMapList{}, first),
			)).
			Assess("XRIsDeleted",
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			).
			// The assessments delete the XR. If one failed, don't leave
			// composed resources stuck behind the test finalizer.
			WithTeardown("ReleaseComposedResources",
				funcs.ListedResourcesModifiedWith(&corev1.ConfigMapList{}, 0, release, composed),
			).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
