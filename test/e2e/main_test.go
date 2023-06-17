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

package e2e

import (
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane/test/e2e/funcs"
)

const namespace = "crossplane-system"

const crdsDir = "cluster/crds"

// The caller (e.g. make e2e) must ensure these exists.
// Run `make build e2e-tag-images` to produce them
const (
	imgcore = "crossplane-e2e/crossplane:latest"
	imgxfn  = "crossplane-e2e/xfn:latest"
)

const (
	helmChartDir    = "cluster/charts/crossplane"
	helmReleaseName = "crossplane"
)

// We reuse these options in TestCrossplane, which uninstalls Crossplane,
// installs the stable chart, then upgrades back to this chart.
var helmOptions = []helm.Option{
	helm.WithName(helmReleaseName),
	helm.WithNamespace(namespace),
	helm.WithChart(helmChartDir),
	helm.WithArgs(
		// Run with debug logging to ensure all log statements are run.
		"--set args={--debug}",
		"--set image.repository="+strings.Split(imgcore, ":")[0],
		"--set image.tag="+strings.Split(imgcore, ":")[1],

		"--set xfn.args={--debug}",
		"--set xfn.image.repository="+strings.Split(imgxfn, ":")[0],
		"--set xfn.image.tag="+strings.Split(imgxfn, ":")[1],
	),
}

// The test environment, shared by all E2E test functions.
var environment env.Environment

func TestMain(m *testing.M) {
	clusterName := envconf.RandomName("crossplane-e2e", 32)
	environment, _ = env.NewFromFlags()
	environment.Setup(funcs.EnvFuncs(
		envfuncs.CreateKindCluster(clusterName),
		envfuncs.LoadDockerImageToCluster(clusterName, imgcore),
		envfuncs.LoadDockerImageToCluster(clusterName, imgxfn),
		envfuncs.CreateNamespace(namespace),
		funcs.HelmInstall(helmOptions...),
		funcs.AddCrossplaneTypesToScheme(),
	))
	environment.Finish(envfuncs.DestroyKindCluster(clusterName))
	os.Exit(environment.Run(m))
}
