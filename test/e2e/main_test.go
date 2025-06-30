/*
Copyright 2022 The Crossplane Authors.

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

// Package e2e implements end-to-end tests for Crossplane.
package e2e

import (
	"context"
	"os"
	"strings"
	"testing"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// TODO(phisco): make it configurable.
const namespace = "crossplane-system"

// TODO(phisco): make it configurable.
const crdsDir = "cluster/crds"

const (
	// TODO(phisco): make it configurable.
	helmChartDir = "cluster/charts/crossplane"
	// TODO(phisco): make it configurable.
	helmReleaseName = "crossplane"
)

var environment = config.NewEnvironmentFromFlags()

func TestMain(m *testing.M) {
	// TODO(negz): Global loggers are dumb and klog is dumb. Remove this when
	// e2e-framework is running controller-runtime v0.15.x per
	// https://github.com/kubernetes-sigs/e2e-framework/issues/270
	log.SetLogger(klog.NewKlogr())

	// Parse flags to ensure we have the environment configured
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		panic(err)
	}

	// Set the default suite, to be used as base for all the other suites.
	environment.AddDefaultTestSuite(
		config.WithoutBaseDefaultTestSuite(),
		config.WithHelmInstallOpts(
			helm.WithName(helmReleaseName),
			helm.WithNamespace(namespace),
			helm.WithChart(helmChartDir),
			// wait for the deployment to be ready for up to 5 minutes before returning
			helm.WithWait(),
			helm.WithTimeout("5m"),
			helm.WithArgs(
				// Run with debug logging to ensure all log statements are run.
				"--set args={--debug}",
				"--set image.repository="+strings.Split(environment.GetCrossplaneImage(), ":")[0],
				"--set image.tag="+strings.Split(environment.GetCrossplaneImage(), ":")[1],
				"--set metrics.enabled=true",
			),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{config.TestSuiteDefault},
		}),
	)

	var setup []env.Func
	var finish []env.Func

	if environment.IsKindCluster() {
		setup = append(setup, envfuncs.CreateClusterWithConfig(
			kind.NewProvider(),
			environment.GetKindClusterName(),
			"./test/e2e/manifests/kind/kind-config.yaml",
		))
	} else {
		cfg.WithKubeconfigFile(conf.ResolveKubeConfigFile())
	}

	// Enrich the selected labels with the ones from the suite.
	// Not replacing the user provided ones if any.
	cfg.WithLabels(environment.EnrichLabels(cfg.Labels()))

	environment.SetEnvironment(env.NewWithConfig(cfg))

	if environment.ShouldLoadImages() {
		clusterName := environment.GetKindClusterName()
		setup = append(setup,
			envfuncs.LoadDockerImageToCluster(clusterName, environment.GetCrossplaneImage()),
		)
	}

	// Add the setup functions defined by the suite being used
	setup = append(setup,
		environment.GetSelectedSuiteAdditionalEnvSetup()...,
	)

	if environment.ShouldInstallCrossplane() {
		setup = append(setup,
			envfuncs.CreateNamespace(namespace),
			environment.HelmInstallBaseCrossplane(),
		)
	}

	// We always want to add our types to the scheme.
	setup = append(setup, funcs.AddCrossplaneTypesToScheme(), funcs.AddCRDsToScheme())

	if environment.ShouldCollectKindLogsOnFailure() {
		finish = append(finish, envfuncs.ExportClusterLogs(environment.GetKindClusterName(), environment.GetKindClusterLogsLocation()))
	}

	// We want to destroy the cluster if we created it, but only if we created it,
	// otherwise the random name will be meaningless.
	if environment.ShouldDestroyKindCluster() {
		finish = append(finish, envfuncs.DestroyCluster(environment.GetKindClusterName()))
	}

	// Check that all features are specifying a suite they belong to via LabelTestSuite.
	//nolint:thelper // We can't make testing.T the second argument because we want to satisfy types.FeatureEnvFunc.
	environment.BeforeEachFeature(func(ctx context.Context, _ *envconf.Config, t *testing.T, feature features.Feature) (context.Context, error) {
		t.Helper()

		if _, exists := feature.Labels()[config.LabelTestSuite]; !exists {
			t.Fatalf("Feature %q does not have the required %q label set", feature.Name(), config.LabelTestSuite)
		}
		return ctx, nil
	})

	environment.Setup(setup...)
	environment.Finish(finish...)
	os.Exit(environment.Run(m))
}
