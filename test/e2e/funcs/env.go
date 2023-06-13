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

package funcs

import (
	"context"
	"strings"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	secretsv1alpha1 "github.com/crossplane/crossplane/apis/secrets/v1alpha1"
)

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

const (
	crdsDir = "cluster/crds"
)

// HelmInstallCrossplane installs Crossplane by executing helm install.
func HelmInstallCrossplane(release, namespace, chartDir string, set ...string) env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		args := make([]string, len(set))
		for i := range set {
			args[i] = "--set " + set[i]
		}
		err := helm.New(c.KubeconfigFile()).RunInstall(
			helm.WithName(release),
			helm.WithNamespace(namespace),
			helm.WithChart(chartDir),
			helm.WithArgs(args...),
		)
		return ctx, errors.Wrap(err, "cannot install Crossplane Helm chart")
	}
}

// AddCrossplaneTypesToScheme adds Crossplane's core custom resource's to the
// environment's scheme. This allows the environment's client to work with said
// types.
func AddCrossplaneTypesToScheme() env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		_ = apiextensionsv1.AddToScheme(c.Client().Resources().GetScheme())
		_ = pkgv1.AddToScheme(c.Client().Resources().GetScheme())
		_ = secretsv1alpha1.AddToScheme(c.Client().Resources().GetScheme())
		return ctx, nil
	}
}

// EnvFuncs runs the supplied functions in order, returning the first error.
func EnvFuncs(fns ...env.Func) env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		for _, fn := range fns {
			var err error
			ctx, err = fn(ctx, c)
			if err != nil {
				return ctx, err
			}
		}
		return ctx, nil
	}
}

// SetupControlPlane creates a kind cluster and installs the local build of
// Crossplane (produced by `make build`) using its Helm chart.
func SetupControlPlane(clusterName, crossplaneNamespace string) env.Func {
	return EnvFuncs(
		envfuncs.CreateKindCluster(clusterName),
		envfuncs.LoadDockerImageToCluster(clusterName, imgcore),
		envfuncs.LoadDockerImageToCluster(clusterName, imgxfn),
		envfuncs.CreateNamespace(crossplaneNamespace),
		HelmInstallCrossplane(
			helmReleaseName,
			crossplaneNamespace,
			helmChartDir,
			"image.repository="+strings.Split(imgcore, ":")[0],
			"image.tag="+strings.Split(imgcore, ":")[1],
			"xfn.image.repository="+strings.Split(imgxfn, ":")[0],
			"xfn.image.tag="+strings.Split(imgxfn, ":")[1],
		),
		AddCrossplaneTypesToScheme(),
	)
}

// DestroyControlPlane destroys a control plane by deleting its kind cluster.
func DestroyControlPlane(clusterName string) env.Func {
	return envfuncs.DestroyKindCluster(clusterName)
}
