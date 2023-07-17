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
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/e2e-framework/klient/conf"
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

// LabelModifyCrossplaneInstallation is used to mark tests that are going to
// modify Crossplane's installation, e.g. installing, uninstalling or upgrading
// it.
const LabelModifyCrossplaneInstallation = "modify-crossplane-installation"

// LabelModifyCrossplaneInstallationTrue is used to mark tests that are going to
// modify Crossplane's installation.
const LabelModifyCrossplaneInstallationTrue = "true"

// LabelStage represents the 'stage' of a feature - alpha, beta, etc. Generally
// available features have no stage label.
const LabelStage = "stage"

const (
	// LabelStageAlpha is used for tests of alpha features.
	LabelStageAlpha = "alpha"

	// LabelStageBeta is used for tests of beta features.
	LabelStageBeta = "beta"
)

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

// HelmOptions valid for installing and upgrading the Crossplane Helm chart.
// Used to install Crossplane before any test starts, but some tests also use
// these options - for example to reinstall Crossplane with a feature flag
// enabled.
func HelmOptions(extra ...helm.Option) []helm.Option {
	o := []helm.Option{
		helm.WithName(helmReleaseName),
		helm.WithNamespace(namespace),
		helm.WithChart(helmChartDir),
		// wait for the deployment to be ready for up to 5 minutes before returning
		helm.WithWait(),
		helm.WithTimeout("5m"),
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
	return append(o, extra...)
}

var (
	// The test environment, shared by all E2E test functions.
	environment env.Environment
	clusterName string
)

func TestMain(m *testing.M) {
	// TODO(negz): Global loggers are dumb and klog is dumb. Remove this when
	// e2e-framework is running controller-runtime v0.15.x per
	// https://github.com/kubernetes-sigs/e2e-framework/issues/270
	log.SetLogger(klog.NewKlogr())

	kindClusterName := flag.String("kind-cluster-name", "", "name of the kind cluster to use")
	create := flag.Bool("create-kind-cluster", true, "create a kind cluster (and deploy Crossplane) before running tests, if the cluster does not already exist with the same name")
	destroy := flag.Bool("destroy-kind-cluster", true, "destroy the kind cluster when tests complete")
	install := flag.Bool("install-crossplane", true, "install Crossplane before running tests")
	load := flag.Bool("load-images-kind-cluster", true, "load Crossplane images into the kind cluster before running tests")

	var setup []env.Func
	var finish []env.Func

	cfg, _ := envconf.NewFromFlags()

	clusterName = envconf.RandomName("crossplane-e2e", 32)
	if *kindClusterName != "" {
		clusterName = *kindClusterName
	}

	// we want to create the cluster if it doesn't exist, but only if we're
	isKindCluster := *create || *kindClusterName != ""
	if isKindCluster {
		kindCfg, err := filepath.Abs(filepath.Join("test", "e2e", "testdata", "kindConfig.yaml"))
		if err != nil {
			log.Log.Error(err, "error getting kind config file")
			os.Exit(1)
		}
		setup = []env.Func{
			funcs.CreateKindClusterWithConfig(clusterName, kindCfg),
		}
	} else {
		cfg.WithKubeconfigFile(conf.ResolveKubeConfigFile())
	}
	environment = env.NewWithConfig(cfg)

	if *load && isKindCluster {
		setup = append(setup,
			envfuncs.LoadDockerImageToCluster(clusterName, imgcore),
			envfuncs.LoadDockerImageToCluster(clusterName, imgxfn),
		)
	}
	if *install {
		setup = append(setup,
			envfuncs.CreateNamespace(namespace),
			funcs.HelmInstall(HelmOptions()...),
		)
	}

	// We always want to add our types to the scheme.
	setup = append(setup, funcs.AddCrossplaneTypesToScheme())

	// We want to destroy the cluster if we created it, but only if we created it,
	// otherwise the random name will be meaningless.
	if *destroy && isKindCluster {
		finish = []env.Func{envfuncs.DestroyKindCluster(clusterName)}
	}

	environment.Setup(setup...)
	environment.Finish(finish...)
	os.Exit(environment.Run(m))
}
