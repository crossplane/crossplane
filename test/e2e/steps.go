package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var syncAndReadyConditions = map[string]string{"Synced": "True", "Ready": "True"}

func clusterScopedResourceIsPresent(ctx context.Context, rawYaml *godog.DocString) (context.Context, error) {
	return ctx, ScenarioContextValue(ctx).Cluster.ApplyYaml(rawYaml.Content)
}

func claimGetsDeployed(ctx context.Context, rawYaml *godog.DocString) (context.Context, error) {
	sc := ScenarioContextValue(ctx)
	claim, err := ToUnstructured(rawYaml.Content)
	if err != nil {
		return ctx, err
	}
	sc.Claim = claim
	return ctx, sc.Cluster.ApplyYamlToNamespace(sc.Namespace, rawYaml.Content)
}

func claimBecomesSynchronizedAndReady(ctx context.Context) error {
	sc := ScenarioContextValue(ctx)
	c := sc.Cluster
	claimName := sc.Claim.GetName()
	return c.WaitForResourceConditionMatch(resourceType(sc.Claim), claimName, sc.Namespace, syncAndReadyConditions)
}

func claimCompositeResourceBecomesSynchronizedAndReady(ctx context.Context) (context.Context, error) {
	sc := ScenarioContextValue(ctx)
	c := sc.Cluster
	claimName := sc.Claim.GetName()
	rawJSON, err := c.GetAndFilterResourceByJq(resourceType(sc.Claim), claimName, sc.Namespace, ".spec.resourceRef")
	if err != nil {
		return ctx, err
	}
	ref := &resourceRef{}
	if err = json.Unmarshal([]byte(rawJSON), ref); err != nil {
		return ctx, err
	}
	sc.ClaimCompositeResourceRef = ref
	return ctx, c.WaitForResourceConditionMatch(ref.Type(), ref.Name, sc.Namespace, syncAndReadyConditions)
}

func composedManagedResourcesBecomeReadyAndSynchronized(ctx context.Context) error {
	sc := ScenarioContextValue(ctx)
	c := sc.Cluster
	compositeRef := sc.ClaimCompositeResourceRef
	ns := sc.Namespace
	rawJSON, err := c.GetAndFilterResourceByJq(compositeRef.Type(), compositeRef.Name, ns, ".spec.resourceRefs")
	if err != nil {
		return err
	}
	var refs []resourceRef
	if err = json.Unmarshal([]byte(rawJSON), &refs); err != nil {
		return err
	}
	for _, r := range refs {
		return c.WaitForResourceConditionMatch(r.Type(), r.Name, ns, syncAndReadyConditions)
	}
	return nil
}

func configurationGetsDeployed(ctx context.Context, rawYaml *godog.DocString) (context.Context, error) {
	sc := ScenarioContextValue(ctx)
	xpCfg, err := ToUnstructured(rawYaml.Content)
	if err != nil {
		return ctx, err
	}
	sc.Configuration = xpCfg
	return ctx, sc.Cluster.ApplyYaml(rawYaml.Content)
}

func configurationMarkedAsInstalledAndHealthy(ctx context.Context) error {
	sc := ScenarioContextValue(ctx)
	return sc.Cluster.WaitForResourceConditionMatch("configurations.pkg.crossplane.io", sc.Configuration.GetName(), "default", map[string]string{"Installed": "True", "Healthy": "True"})
}

func crossplaneIsRunningInCluster(ctx context.Context) error {
	sc := ScenarioContextValue(ctx)
	return sc.Cluster.WaitForResourceConditionMatch("deployment", "crossplane", "crossplane-system", map[string]string{"Available": "True"})
}

func resourceType(u *unstructured.Unstructured) string {
	return fmt.Sprintf("%s.%s", strings.ToLower(u.GetKind()), strings.Split(u.GetAPIVersion(), "/")[0])
}
