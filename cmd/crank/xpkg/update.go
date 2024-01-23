/*
Copyright 2023 The Crossplane Authors.

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
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg"

	_ "k8s.io/client-go/plugin/pkg/client/auth" // Load all the auth plugins for the cloud providers.
)

// updateCmd updates a package.
type updateCmd struct {
	// Arguments.
	Kind    string `arg:"" help:"The kind of package to update. One of \"provider\", \"configuration\", or \"function\"." enum:"provider,configuration,function"`
	Package string `arg:"" help:"The package to update to."`
	Name    string `arg:""  optional:"" help:"The name of the package to update in the Crossplane API. Derived from the package repository and tag by default."`
}

func (c *updateCmd) Help() string {
	return `
This command updates a package in a Crossplane control plane. It uses
~/.kube/config to connect to the control plane. You can override this using the
KUBECONFIG environment variable.

Examples:

  # Update the Function named function-eg
  crossplane xpkg update function upbound/function-example:v0.1.5 function-eg
`
}

// Run the package update cmd.
func (c *updateCmd) Run(k *kong.Context, logger logging.Logger) error {
	pkgName := c.Name
	if pkgName == "" {
		ref, err := name.ParseReference(c.Package, name.WithDefaultRegistry(xpkg.DefaultRegistry))
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

	var pkg v1.Package
	switch c.Kind {
	case "provider":
		pkg = &v1.Provider{}
	case "configuration":
		pkg = &v1.Configuration{}
	case "function":
		pkg = &v1beta1.Function{}
	default:
		// The enum struct tag on the Kind field should make this impossible.
		return errors.Errorf("unsupported package kind %q", c.Kind)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := kube.Get(ctx, types.NamespacedName{Name: pkgName}, pkg); err != nil {
			return errors.Wrap(warnIfNotFound(err), "cannot get package")
		}
		logger.Debug("Found existing package")

		pkg.SetSource(c.Package)

		return kube.Update(ctx, pkg)
	}); err != nil {
		return errors.Wrapf(err, "cannot update %s/%s", c.Kind, pkg.GetName())
	}

	_, err = fmt.Fprintf(k.Stdout, "%s/%s updated\n", c.Kind, pkg.GetName())
	return err
}
