/*
Copyright 2024 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

func TestProviderDeletionProtection(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/provider"

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that provider deletion is blocked when custom resources exist for the provider's CRDs.").
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("ApplyProvider", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider-initial.yaml"),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "provider-initial.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreateManagedResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mr-initial.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "mr-initial.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mr-initial.yaml", xpv1.Available()),
			)).
			Assess("ProviderDeletionBlocked", funcs.DeletionBlockedByProviderWebhook(manifests, "provider-initial.yaml")).
			WithTeardown("DeleteManagedResource", funcs.AllOf(
				funcs.DeleteResources(manifests, "mr-initial.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "mr-initial.yaml"),
			)).
			WithTeardown("DeleteProviderAfterMRDeleted", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "provider-initial.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "provider-initial.yaml"),
			)).
			Feature(),
	)
}
