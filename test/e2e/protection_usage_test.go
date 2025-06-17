package e2e

import (
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/features"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// TODO(negz): Add composition-focused test here? We have one for the legacy
// Usage, which is powered by the same webhook and controller. Maybe refactor
// it and move it here when we remove the legacy implementation?

// LabelAreaProtection is applied to all features pertaining to protection.
const LabelAreaProtection = "protection"

func TestUsageStandaloneNamespaced(t *testing.T) {
	manifests := "test/e2e/manifests/protection/usage/standalone-namespaced"

	cases := features.Table{
		{
			// Deletion of a (used) resource should be blocked if there is a Usage relation with a using resource defined.
			Name: "UsageBlockedByUsingResource",
			Assessment: funcs.AllOf(
				// Create using and used resources together with a usage.
				funcs.ApplyResources(FieldManager, manifests, "with-by/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-by/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-by/usage.yaml", xpv1.Available()),

				// Deletion of used resource should be blocked by usage.
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-by/used.yaml"),

				// Deletion of using resource should clear usage.
				funcs.DeleteResources(manifests, "with-by/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by/usage.yaml"),
				// We have "replayDeletion: true" on the usage, deletion of used resource should be replayed after usage is cleared.
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "with-by/used.yaml"),
			),
		},
		{
			// Deletion of a (used) resource in another namespace should be blocked if there is a Usage relation with a using resource defined.
			Name: "CrossNamespaceUsageBlockedByUsingResource",
			Assessment: funcs.AllOf(
				// Create using and used resources together with a usage.
				funcs.ApplyResources(FieldManager, manifests, "with-by-across-namespaces/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-by-across-namespaces/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-by-across-namespaces/usage.yaml", xpv1.Available()),

				// Deletion of used resource should be blocked by usage.
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-by-across-namespaces/used.yaml"),

				// Deletion of using resource should clear usage.
				funcs.DeleteResources(manifests, "with-by-across-namespaces/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by-across-namespaces/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by-across-namespaces/usage.yaml"),
				// We have "replayDeletion: true" on the usage, deletion of used resource should be replayed after usage is cleared.
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "with-by-across-namespaces/used.yaml"),
			),
		},
		{
			// Deletion of a (protected) resource should be blocked if there is a Usage with a reason defined.
			Name: "UsageBlockedWithReason",
			Assessment: funcs.AllOf(
				// Create protected resources together with a usage.
				funcs.ApplyResources(FieldManager, manifests, "with-reason/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-reason/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-reason/usage.yaml", xpv1.Available()),

				// Deletion of protected resource should be blocked by usage.
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-reason/used.yaml"),

				// Deletion of usage should clear usage.
				funcs.DeleteResources(manifests, "with-reason/usage.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/usage.yaml"),

				// Deletion of protected resource should be allowed after usage is cleared.
				funcs.DeleteResources(manifests, "with-reason/used.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/used.yaml"),
			),
		},
		{
			// Usages across namespaces should be blocked if the by resource is not specified, using resourceRef.
			Name: "CrossNamespaceUsageBlockedWithoutByResourceRef",
			Assessment: funcs.AllOf(
				funcs.ResourcesFailToApply(FieldManager, manifests, "ref-without-by-cross-namespace-invalid/*.yaml"),
			),
		},
		{
			// Usages across namespaces should be blocked if the by resource is not specified, using resourceSelector.
			Name: "CrossNamespaceUsageBlockedWithoutByResourceSelector",
			Assessment: funcs.AllOf(
				funcs.ResourcesFailToApply(FieldManager, manifests, "selector-without-by-cross-namespace-invalid/*.yaml"),
			),
		},
	}

	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			Feature(),
	)
}

func TestUsageStandaloneCluster(t *testing.T) {
	manifests := "test/e2e/manifests/protection/usage/standalone-cluster"

	cases := features.Table{
		{
			// Deletion of a (used) resource should be blocked if there is a Usage relation with a using resource defined.
			Name: "UsageBlockedByUsingResource",
			Assessment: funcs.AllOf(
				// Create using and used resources together with a usage.
				funcs.ApplyResources(FieldManager, manifests, "with-by/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-by/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-by/usage.yaml", xpv1.Available()),

				// Deletion of used resource should be blocked by usage.
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-by/used.yaml"),

				// Deletion of using resource should clear usage.
				funcs.DeleteResources(manifests, "with-by/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by/using.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-by/usage.yaml"),
				// We have "replayDeletion: true" on the usage, deletion of used resource should be replayed after usage is cleared.
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "with-by/used.yaml"),
			),
		},
		{
			// Deletion of a (protected) resource should be blocked if there is a Usage with a reason defined.
			Name: "UsageBlockedWithReason",
			Assessment: funcs.AllOf(
				// Create protected resources together with a usage.
				funcs.ApplyResources(FieldManager, manifests, "with-reason/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "with-reason/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "with-reason/usage.yaml", xpv1.Available()),

				// Deletion of protected resource should be blocked by usage.
				funcs.DeletionBlockedByUsageWebhook(manifests, "with-reason/used.yaml"),

				// Deletion of usage should clear usage.
				funcs.DeleteResources(manifests, "with-reason/usage.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/usage.yaml"),

				// Deletion of protected resource should be allowed after usage is cleared.
				funcs.DeleteResources(manifests, "with-reason/used.yaml"),
				funcs.ResourcesDeletedWithin(30*time.Second, manifests, "with-reason/used.yaml"),
			),
		},
	}

	environment.Test(t,
		cases.Build(t.Name()).
			WithLabel(LabelStage, LabelStageBeta).
			WithLabel(LabelArea, LabelAreaProtection).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			Feature(),
	)
}
