/*
Copyright 2020 The Crossplane Authors.

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

package xpkg

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/version"
	"github.com/crossplane/crossplane/internal/xpkg"

	// Load all the auth plugins for the cloud providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	errPkgIdentifier = "invalid package image identifier"
	errKubeConfig    = "failed to get kubeconfig"
	errKubeClient    = "failed to create kube client"
)

// installCmd installs a package.
type installCmd struct {
	// Arguments.
	Kind    string `arg:"" help:"The kind of package to install. One of \"provider\", \"configuration\", or \"function\"." enum:"provider,configuration,function"`
	Package string `arg:"" help:"The package to install."`
	Name    string `arg:""  optional:"" help:"The name of the new package in the Crossplane API. Derived from the package repository and tag by default."`

	// Flags. Keep sorted alphabetically.
	RuntimeConfig        string        `placeholder:"NAME" help:"Install the package with a runtime configuration (for example a DeploymentRuntimeConfig)."`
	ManualActivation     bool          `short:"m" help:"Require the new package's first revision to be manually activated."`
	PackagePullSecrets   []string      `placeholder:"NAME" help:"A comma-separated list of secrets the package manager should use to pull the package from the registry."`
	RevisionHistoryLimit int64         `short:"r" placeholder:"LIMIT" help:"How many package revisions may exist before the oldest revisions are deleted."`
	Wait                 time.Duration `short:"w" default:"0s" help:"How long to wait for the package to install before returning. The command does not wait by default."`
}

func (c *installCmd) Help() string {
	return `
This command installs a package in a Crossplane control plane. It uses
~/.kube/config to connect to the control plane. You can override this using the
KUBECONFIG environment variable.

Examples:

  # Wait 1 minute for the package to finish installing before returning.
  crossplane xpkg install provider upbound/provider-aws-eks:v0.41.0 --wait=1m

  # Install a Function named function-eg that uses a runtime config named
  # customconfig.
  crossplane xpkg install function upbound/function-example:v0.1.4 function-eg \
    --runtime-config=customconfig
`
}

// Run the package install cmd.
func (c *installCmd) Run(k *kong.Context, logger logging.Logger) error { //nolint:gocyclo // TODO(negz): Can anything be broken out here?
	pkgName := c.Name
	if pkgName == "" {
		ref, err := name.ParseReference(c.Package, name.WithDefaultRegistry(DefaultRegistry))
		if err != nil {
			logger.Debug(errPkgIdentifier, "error", err)
			return errors.Wrap(err, errPkgIdentifier)
		}
		pkgName = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}

	logger = logger.WithValues(
		"kind", c.Kind,
		"ref", c.Package,
		"name", pkgName,
	)

	rap := v1.AutomaticActivation
	if c.ManualActivation {
		rap = v1.ManualActivation
	}
	secrets := make([]corev1.LocalObjectReference, len(c.PackagePullSecrets))
	for i, s := range c.PackagePullSecrets {
		secrets[i] = corev1.LocalObjectReference{
			Name: s,
		}
	}

	spec := v1.PackageSpec{
		Package:                  c.Package,
		RevisionActivationPolicy: &rap,
		RevisionHistoryLimit:     &c.RevisionHistoryLimit,
		PackagePullSecrets:       secrets,
	}

	var pkg v1.Package
	switch c.Kind {
	case "provider":
		pkg = &v1.Provider{
			ObjectMeta: metav1.ObjectMeta{Name: pkgName},
			Spec:       v1.ProviderSpec{PackageSpec: spec},
		}
	case "configuration":
		pkg = &v1.Configuration{
			ObjectMeta: metav1.ObjectMeta{Name: pkgName},
			Spec:       v1.ConfigurationSpec{PackageSpec: spec},
		}
	case "function":
		pkg = &v1beta1.Function{
			ObjectMeta: metav1.ObjectMeta{Name: pkgName},
			Spec:       v1beta1.FunctionSpec{PackageSpec: spec},
		}
	default:
		// The enum struct tag on the Kind field should make this impossible.
		return errors.Errorf("unsupported package kind %q", c.Kind)
	}

	if c.RuntimeConfig != "" {
		rpkg, ok := pkg.(v1.PackageWithRuntime)
		if !ok {
			return errors.Errorf("package kind %T does not support runtime configuration", pkg)
		}
		rpkg.SetRuntimeConfigRef(&v1.RuntimeConfigReference{Name: c.RuntimeConfig})
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")

	s := runtime.NewScheme()
	_ = v1.AddToScheme(s)
	_ = v1beta1.AddToScheme(s)

	kube, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		return errors.Wrap(err, errKubeClient)
	}
	logger.Debug("Created kubernetes client")

	timeout := 10 * time.Second
	if c.Wait > 0 {
		timeout = c.Wait
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := kube.Create(ctx, pkg); err != nil {
		return errors.Wrap(warnIfNotFound(err), "cannot create package")
	}

	if c.Wait > 0 {
		// Poll every 2 seconds to see whether the package is ready.
		logger.Debug("Waiting for package to be ready", "timeout", timeout)
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			if err := kube.Get(ctx, client.ObjectKeyFromObject(pkg), pkg); err != nil {
				logger.Debug("Cannot get package", "error", err)
				return
			}

			// Our package is ready, cancel the context to stop our wait loop.
			if pkg.GetCondition(v1.TypeHealthy).Status == corev1.ConditionTrue {
				logger.Debug("Package is ready")
				cancel()
				return
			}

			logger.Debug("Package is not yet ready")
		}, 2*time.Second)
	}

	_, err = fmt.Fprintf(k.Stdout, "%s/%s created\n", c.Kind, pkg.GetName())
	return err
}

// TODO(negz): What is this trying to do? My guess is its trying to handle the
// case where the CRD of the package kind isn't installed. Perhaps we could be
// clearer in the error?

func warnIfNotFound(err error) error {
	serr := &kerrors.StatusError{}
	if !errors.As(err, &serr) {
		return err
	}
	if serr.ErrStatus.Code != http.StatusNotFound {
		return err
	}
	return errors.WithMessagef(err, "crossplane CLI (version %s) might be out of date", version.New().GetVersionString())
}
