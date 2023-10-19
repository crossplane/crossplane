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
	Kind string `arg:"" help:"Kind of package to install. One of \"provider\", \"configuration\", or \"function\"." enum:"provider,configuration,function"`
	Ref  string `arg:"" help:"The package's OCI image reference (e.g. tag)."`
	Name string `arg:"" optional:"" help:"Name of the package to update. Will be derived from the ref if omitted."`
}

func (c *updateCmd) Help() string {
	return `
Crossplane can be extended using packages. A Crossplane package is sometimes
called an xpkg. Crossplane supports configuration, provider and function
packages. 

A package is an opinionated OCI image that contains everything needed to extend
Crossplane with new functionality. For example installing a provider package
extends Crossplane with support for new kinds of managed resource (MR).

This command tells the Crossplane package manager to update an installed package
to a new version, pulled from a package registry. It uses ~/.kube/config to know
how to connect to the package manager. You can override this using the
KUBECONFIG environment variable.

See https://docs.crossplane.io/latest/concepts/packages for more information.
`
}

// Run the package update cmd.
func (c *updateCmd) Run(k *kong.Context, logger logging.Logger) error {
	pkgName := c.Name
	if pkgName == "" {
		ref, err := name.ParseReference(c.Ref, name.WithDefaultRegistry(DefaultRegistry))
		if err != nil {
			logger.Debug(errPkgIdentifier, "error", err)
			return errors.Wrap(err, errPkgIdentifier)
		}
		pkgName = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}

	logger = logger.WithValues(
		"kind", c.Kind,
		"ref", c.Ref,
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

		pkg.SetSource(c.Ref)

		return kube.Update(ctx, pkg)
	}); err != nil {
		return errors.Wrapf(err, "cannot update %s/%s", c.Kind, pkg.GetName())
	}

	_, err = fmt.Fprintf(k.Stdout, "%s/%s updated\n", c.Kind, pkg.GetName())
	return err
}
