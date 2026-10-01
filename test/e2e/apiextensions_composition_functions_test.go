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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/features"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// TestCompositionFunctionByOCIRef exercises referencing a composition pipeline
// function by OCI ref rather than by the name of a pre-installed Function.
func TestCompositionFunctionByOCIRef(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/function-oci-ref"

	const functionName = "crossplane-e2e-function-revision-id"

	parentFunction := &pkgv1.Function{ObjectMeta: metav1.ObjectMeta{Name: functionName}}
	revisions := &pkgv1.FunctionRevisionList{}
	revisionSelector := resources.WithLabelSelector(pkgv1.LabelFunction + "=" + functionName)

	// revisionHealthy validates that a listed FunctionRevision is established
	// and healthy.
	revisionHealthy := func(o k8s.Object) bool {
		fr, ok := o.(*pkgv1.FunctionRevision)
		if !ok {
			return false
		}
		return fr.Status.GetCondition(pkgv1.TypeRevisionHealthy).Status == corev1.ConditionTrue
	}

	// externalRefsLen returns a checker that passes when
	// spec.externalRevisionRefs has exactly n entries.
	externalRefsLen := func(n int) funcs.FieldValueChecker {
		return func(got any) bool {
			s, ok := got.([]any)
			return ok && len(s) == n
		}
	}

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a composition pipeline step can reference a function by OCI ref.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
			)).
			Assess("FunctionInstalledFromOCIRef", funcs.AllOf(
				funcs.ResourceHasConditionWithin(2*time.Minute, parentFunction, pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourceHasFieldValueWithin(2*time.Minute, parentFunction, "spec.externalRevisionRefs", externalRefsLen(1)),
				// An empty (omitted) spec.package is what tells the package
				// manager these revisions are externally managed.
				funcs.ResourceHasFieldValueWithin(2*time.Minute, parentFunction, "spec.package", funcs.NotFound),
				funcs.ListedResourcesValidatedWithin(2*time.Minute, revisions, 1, revisionHealthy, revisionSelector),
			)).
			Assess("XRIsReadyOnV1", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "xr.yaml", xpv2.Available()),
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.servedBy", "revision-one"),
			)).
			Assess("PinnedXRKeepsOldFunctionRevision", funcs.AllOf(
				// Update the composition to create a new composition revision
				// with an updated function reference. Ensure the new function
				// revision is created and becomes healthy.
				funcs.ApplyResources(FieldManager, manifests, "composition-update.yaml"),
				funcs.ListedResourcesValidatedWithin(2*time.Minute, revisions, 2, revisionHealthy, revisionSelector),
				funcs.ResourceHasFieldValueWithin(2*time.Minute, parentFunction, "spec.externalRevisionRefs", externalRefsLen(2)),
				// Touch the pinned composite's spec to force a re-reconcile and
				// confirm it's still using the original function revision.
				funcs.ApplyResources(FieldManager, manifests, "xr-updated.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "xr.yaml", xpv2.Available()),
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr.yaml", "status.servedBy", "revision-one"),
			)).
			Assess("NewXRUsesNewFunctionRevision", funcs.AllOf(
				// Create another XR, which should use the new composition
				// revision and thus the new function revision, to ensure that
				// both revisions work simultaneuosly.
				funcs.ApplyResources(FieldManager, manifests, "xr2.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr2.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "xr2.yaml", xpv2.Available()),
				funcs.ResourcesHaveFieldValueWithin(1*time.Minute, manifests, "xr2.yaml", "status.servedBy", "revision-two"),
			)).
			Assess("RevisionsGarbageCollectedOnCompositionDelete", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr2.yaml"),
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "composition-update.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "composition-update.yaml"),
				funcs.ResourceDeletedWithin(2*time.Minute, parentFunction),
				funcs.ListedResourcesDeletedWithin(2*time.Minute, revisions, revisionSelector),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/definition.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "setup/definition.yaml"),
			)).
			Feature(),
	)
}
