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
	"flag"
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelArea represents the 'area' of a feature. For example 'apiextensions',
// 'pkg', etc. Assessments roll up to features, which roll up to feature areas.
// Features within an area may be split across different test functions.
const LabelArea = "area"

// LabelSize represents the 'size' (i.e. duration) of a test.
const LabelSize = "size"

const (
	// LabelSizeSmall is used for tests that usually complete in a minute.
	LabelSizeSmall = "small"

	// LabelSizeLarge is used for test that usually complete in over a minute.
	LabelSizeLarge = "large"
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

// FieldManager is the server-side apply field manager used when applying
// manifests.
const FieldManager = "crossplane-e2e-tests"

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
	destroy := flag.Bool("destroy-kind-cluster", true, "destroy the kind cluster when tests complete")

	clusterName := envconf.RandomName("crossplane-e2e", 32)
	environment, _ = env.NewFromFlags()
	environment.Setup(funcs.EnvFuncs(
		// TODO(negz): If we want to support running tests on existing clusters
		// (e.g. GKE, EKS, your own kind cluster) we could wrap most of this in
		// an env.Func that skips everything but adding types to the scheme when
		// the tests are run with the --kubeconfig flag. I'm not aware of any
		// way to load an image to a non-kind cluster, so we'd have to leave it
		// to the caller to ensure the right build of Crossplane was installed.
		// https://github.com/cilium/tetragon/blob/v0.9.0/tests/e2e/helpers/cluster.go#L136
		envfuncs.CreateKindCluster(clusterName),
		envfuncs.LoadDockerImageToCluster(clusterName, imgcore),
		envfuncs.LoadDockerImageToCluster(clusterName, imgxfn),
		envfuncs.CreateNamespace(namespace),
		funcs.HelmInstall(helmOptions...),
		funcs.AddCrossplaneTypesToScheme(),
	))
	if *destroy {
		environment.Finish(envfuncs.DestroyKindCluster(clusterName))
	}
	os.Exit(environment.Run(m))
}
