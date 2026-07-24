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
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sapiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// SuiteProviderRuntimeActivation is the test suite for provider runtime
// activation, which is part of the CRD to MRD conversion feature. The chart's
// default activation policy is disabled so that the safe-start provider's
// MRDs stay inactive until the test activates them.
const SuiteProviderRuntimeActivation = "provider-runtime-activation"

func init() {
	environment.AddTestSuite(SuiteProviderRuntimeActivation,
		config.WithHelmInstallOpts(
			helm.WithArgs(
				"--set args={--debug}",
				"--set provider.defaultActivations=null",
			),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteProviderRuntimeActivation, config.TestSuiteDefault},
		}),
	)
}

func TestProviderRuntimeActivation(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/runtime-activation"

	providerName := "provider-nop-runtime-activation"
	parentPackageSelector := resources.WithLabelSelector(pkgv1.LabelParentPackage + "=" + providerName)

	environment.Test(t,
		features.NewWithDescription(t.Name(),
			"Tests that the runtime of a provider with the safe-start capability is scaled to zero until its first ManagedResourceDefinition is activated.").
			WithLabel(LabelArea, LabelAreaPkg).
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelSize, LabelSizeLarge).
			WithLabel(config.LabelTestSuite, SuiteProviderRuntimeActivation).

			// Install a provider with the safe-start capability. With the
			// chart's default activations disabled, all of its MRDs are
			// created inactive, so the runtime must not run.
			WithSetup("InstallSafeStartProvider", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "provider.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "provider.yaml"),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "provider.yaml", pkgv1.HealthyAwaitingActivation(), pkgv1.Active()),
			)).

			// The provider's MRDs exist but are inactive.
			Assess("MRDsAreInactive",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&apiextensionsv1alpha1.ManagedResourceDefinitionList{},
					1,
					func(o k8s.Object) bool {
						mrd, ok := o.(*apiextensionsv1alpha1.ManagedResourceDefinition)
						if !ok || mrd.Spec.State.IsActive() {
							return false
						}
						for _, ref := range mrd.GetOwnerReferences() {
							if ref.Kind == pkgv1.ProviderRevisionKind && strings.HasPrefix(ref.Name, providerName) {
								return true
							}
						}
						return false
					},
				),
			).

			// The provider revision is intentionally awaiting activation
			// rather than unhealthy.
			Assess("RevisionAwaitsActivation",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&pkgv1.ProviderRevisionList{},
					1,
					func(o k8s.Object) bool {
						pr, ok := o.(*pkgv1.ProviderRevision)
						if !ok {
							return false
						}
						healthy := pr.GetCondition(pkgv1.TypeRuntimeHealthy)
						active := pr.GetCondition(pkgv1.TypeRuntimeActive)
						return healthy.Status == corev1.ConditionTrue &&
							active.Status == corev1.ConditionFalse && active.Reason == pkgv1.ReasonAwaitingActivation
					},
					parentPackageSelector,
				),
			).

			// The runtime deployment exists but is scaled to zero.
			Assess("RuntimeScaledToZero",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&appsv1.DeploymentList{},
					1,
					func(o k8s.Object) bool {
						d, ok := o.(*appsv1.Deployment)
						if !ok || !strings.HasPrefix(d.GetName(), providerName) {
							return false
						}
						return d.Spec.Replicas != nil && *d.Spec.Replicas == 0
					},
				),
			).

			// Activate the provider's MRDs.
			Assess("ActivateMRDs", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "mrap.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "mrap.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "mrap.yaml", apiextensionsv1alpha1.Healthy()),
			)).

			// The runtime scales up and becomes available.
			Assess("RuntimeScalesUp",
				funcs.ListedResourcesValidatedWithin(3*time.Minute,
					&appsv1.DeploymentList{},
					1,
					func(o k8s.Object) bool {
						d, ok := o.(*appsv1.Deployment)
						if !ok || !strings.HasPrefix(d.GetName(), providerName) {
							return false
						}
						return d.Spec.Replicas != nil && *d.Spec.Replicas == 1 && d.Status.AvailableReplicas == 1
					},
				),
			).

			// The revision reports a healthy, active runtime and the provider
			// stays healthy throughout.
			Assess("RevisionRuntimeIsHealthyAndActive",
				funcs.ListedResourcesValidatedWithin(2*time.Minute,
					&pkgv1.ProviderRevisionList{},
					1,
					func(o k8s.Object) bool {
						pr, ok := o.(*pkgv1.ProviderRevision)
						if !ok {
							return false
						}
						healthy := pr.GetCondition(pkgv1.TypeRuntimeHealthy)
						active := pr.GetCondition(pkgv1.TypeRuntimeActive)
						return healthy.Status == corev1.ConditionTrue && healthy.Reason == pkgv1.ReasonHealthy &&
							active.Status == corev1.ConditionTrue
					},
					parentPackageSelector,
				),
			).
			Assess("ProviderIsHealthy",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			).

			// Cleanup.
			WithTeardown("DeleteMRAP", funcs.AllOf(
				funcs.DeleteResources(manifests, "mrap.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "mrap.yaml"),
			)).
			WithTeardown("DeleteProvider", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "provider.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "provider.yaml"),
				funcs.ResourceDeletedWithin(2*time.Minute, &k8sapiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "nopresources.nop.crossplane.io"},
				}),
			)).
			Feature(),
	)
}
