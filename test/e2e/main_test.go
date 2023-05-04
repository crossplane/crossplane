package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
)

var opts = godog.Options{
	Output: colors.Colored(os.Stdout),
	Format: "pretty",
}

func init() {
	godog.BindFlags("godog.", flag.CommandLine, &opts)
}

func TestE2E(t *testing.T) {
	o := opts
	o.TestingT = t

	status := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options:             &o,
	}.Run()

	if status == 2 {
		t.SkipNow()
	}

	if status != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		sc := &ScenarioContext{
			Cluster: &cluster{
				cli: "kubectl",
			},
			Namespace: fmt.Sprintf("test-%s", scenario.Id),
		}
		if err := sc.Cluster.createNamespace(sc.Namespace); err != nil {
			return ctx, err
		}
		ctx = context.WithValue(ctx, scenarioContextKey{}, sc)
		return ctx, nil
	})
	ctx.Step(`^claim becomes synchronized and ready$`, claimBecomesSynchronizedAndReady)
	ctx.Step(`^claim composite resource becomes synchronized and ready$`, claimCompositeResourceBecomesSynchronizedAndReady)
	ctx.Step(`^claim gets deployed$`, claimGetsDeployed)
	ctx.Step(`^composed managed resources become ready and synchronized$`, composedManagedResourcesBecomeReadyAndSynchronized)
	ctx.Step(`^CompositeResourceDefinition is present$`, clusterScopedResourceIsPresent)
	ctx.Step(`^Composition is present$`, clusterScopedResourceIsPresent)
	ctx.Step(`^Configuration is applied$`, configurationGetsDeployed)
	ctx.Step(`^configuration is marked as installed and healthy$`, configurationMarkedAsInstalledAndHealthy)
	ctx.Step(`^Crossplane is running in cluster$`, crossplaneIsRunningInCluster)
	ctx.Step(`^provider (\S+) does not get installed$`, providerNotInstalled)
	ctx.Step(`^provider (\S+) is marked as installed and healthy$`, providerMarkedAsInstalledAndHealthy)
	ctx.Step(`^provider (\S+) is running in cluster$`, providerGetsInstalled)
}
