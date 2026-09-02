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

	admv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// SuiteUsageWebhookIgnoreFailurePolicy is the test suite for the configurable
// failurePolicy of the Usage deletion protection webhook, which requires
// installing the chart with webhooks.usage.failurePolicy=Ignore. Some managed
// Kubernetes operations (e.g. Azure AKS cluster stop) reject Fail-policy
// webhooks with wildcard rules, so users need to be able to relax the policy
// without disabling the Usages feature.
const SuiteUsageWebhookIgnoreFailurePolicy = "usage-webhook-ignore-failure-policy"

func init() {
	environment.AddTestSuite(SuiteUsageWebhookIgnoreFailurePolicy,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set webhooks.usage.failurePolicy=Ignore"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteUsageWebhookIgnoreFailurePolicy},
		}),
	)
}

func TestUsageWebhookIgnoreFailurePolicy(t *testing.T) {
	manifests := "test/e2e/manifests/protection/usage/standalone-cluster"

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that the failurePolicy of the Usage deletion protection webhook is configurable via Helm, and that deletion protection still works with failurePolicy Ignore while the webhook is available.").
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteUsageWebhookIgnoreFailurePolicy).

			// The init container should have installed the webhook configuration
			// with the failurePolicy set via webhooks.usage.failurePolicy.
			Assess("WebhookHasIgnoreFailurePolicy",
				funcs.ResourceHasFieldValueWithin(1*time.Minute,
					&admv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "crossplane-no-usages"}},
					"webhooks[0].failurePolicy", "Ignore"),
			).

			// While the webhook is available the failurePolicy is irrelevant, so
			// deletion protection should still work.
			Assess("DeletionStillBlockedByUsage", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "with-reason/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-reason/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-reason/usage.yaml", xpv2.Available()),
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-reason/used.yaml"),
			)).
			WithTeardown("DeleteResources", funcs.AllOf(
				funcs.DeleteResources(manifests, "with-reason/usage.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/usage.yaml"),
				funcs.DeleteResources(manifests, "with-reason/used.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/used.yaml"),
			)).
			Feature(),
	)
}

func TestUsageWebhookGatedByUsagesFeature(t *testing.T) {
	usageWebhookConfig := func() *admv1.ValidatingWebhookConfiguration {
		return &admv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "crossplane-no-usages"}}
	}

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that the Usage deletion protection webhook configuration is removed by the init container when the Usages feature is disabled, and reinstalled with the configured failurePolicy when it is re-enabled.").
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(LabelModifyCrossplaneInstallation, LabelModifyCrossplaneInstallationTrue).
			WithLabel(config.LabelTestSuite, SuiteUsageWebhookIgnoreFailurePolicy).

			// The webhook configuration exists because the suite installs with
			// Usages enabled (the default).
			WithSetup("WebhookConfigurationExists",
				funcs.ResourceCreatedWithin(1*time.Minute, usageWebhookConfig()),
			).
			Assess("DisableUsages", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToSuite(SuiteUsageWebhookIgnoreFailurePolicy,
					helm.WithArgs("--set usages.enabled=false"))),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
				funcs.DeploymentBecomesAvailableWithin(2*time.Minute, namespace, "crossplane"),
			)).

			// The init container should have removed the pre-existing webhook
			// configuration, so nothing blocks DELETEs while the feature is off.
			Assess("WebhookConfigurationRemoved",
				funcs.ResourceDeletedWithin(2*time.Minute, usageWebhookConfig()),
			).
			WithTeardown("EnableUsages", funcs.AllOf(
				funcs.AsFeaturesFunc(environment.HelmUpgradeCrossplaneToBase()),
				funcs.ReadyToTestWithin(1*time.Minute, namespace),
				funcs.DeploymentBecomesAvailableWithin(2*time.Minute, namespace, "crossplane"),
				// The webhook configuration should be reinstalled with the
				// failurePolicy configured for this suite, not the packaged
				// default.
				funcs.ResourceHasFieldValueWithin(2*time.Minute, usageWebhookConfig(), "webhooks[0].failurePolicy", "Ignore"),
			)).
			Feature(),
	)
}
