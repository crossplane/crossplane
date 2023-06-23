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
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	secretsv1alpha1 "github.com/crossplane/crossplane/apis/secrets/v1alpha1"
)

// HelmRepo manages a Helm repo.
func HelmRepo(o ...helm.Option) env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		err := helm.New(c.KubeconfigFile()).RunRepo(o...)
		return ctx, errors.Wrap(err, "cannot install Helm chart")
	}
}

// HelmInstall installs a Helm chart.
func HelmInstall(o ...helm.Option) env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		err := helm.New(c.KubeconfigFile()).RunInstall(o...)
		return ctx, errors.Wrap(err, "cannot install Helm chart")
	}
}

// HelmUpgrade upgrades a Helm chart.
func HelmUpgrade(o ...helm.Option) env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		err := helm.New(c.KubeconfigFile()).RunUpgrade(o...)
		return ctx, errors.Wrap(err, "cannot upgrade Helm chart")
	}
}

// AsFeaturesFunc converts an env.Func to a features.Func. If the env.Func
// returns an error the calling test is failed with t.Fatal(err).
func AsFeaturesFunc(fn env.Func) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		ctx, err := fn(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		return ctx
	}

}

// HelmUninstall uninstalls a Helm chart.
func HelmUninstall(o ...helm.Option) env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		err := helm.New(c.KubeconfigFile()).RunUninstall(o...)
		return ctx, errors.Wrap(err, "cannot uninstall Helm chart")
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
