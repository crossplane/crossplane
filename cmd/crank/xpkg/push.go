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
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/upbound/credhelper"
)

const (
	errGetwd           = "failed to get working directory while searching for package"
	errFindPackageinWd = "failed to find a package in current working directory"
)

// DefaultRegistry for pushing Crossplane packages.
const DefaultRegistry = "xpkg.upbound.io"

// pushCmd pushes a package.
type pushCmd struct {
	// Arguments.
	Package string `arg:"" help:"Where to push the package. An OCI repository and tag, optionally prefixed with a registry."`

	// Flags. Keep sorted alphabetically.
	PackageFile string `short:"f" type:"existingfile" placeholder:"PATH" help:"The xpkg file to push."`

	// Internal state. These aren't part of the user-exposed CLI structure.
	fs afero.Fs
}

func (c *pushCmd) Help() string {
	return `
Crossplane can be extended using packages. A Crossplane package is sometimes
called an xpkg. Crossplane supports configuration, provider and function
packages. 

A package is an opinionated OCI image that contains everything needed to extend
Crossplane with new functionality. For example installing a provider package
extends Crossplane with support for new kinds of managed resource (MR).

This command pushes a package to a registry. Packages are pushed to
xpkg.upbound.io by default. You can override this by including a registry in
your OCI image name: for example index.docker.io/example/package:v1.0.0.

See https://docs.crossplane.io/latest/concepts/packages for more information.
`
}

// AfterApply sets the tag for the parent push command.
func (c *pushCmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run runs the push cmd.
func (c *pushCmd) Run(logger logging.Logger) error {
	logger = logger.WithValues("image", c.Package)
	tag, err := name.NewTag(c.Package, name.WithDefaultRegistry(DefaultRegistry))
	if err != nil {
		logger.Debug("Failed to create tag for package", "error", err)
		return err
	}

	// If package is not defined, attempt to find single package in current
	// directory.
	if c.PackageFile == "" {
		logger.Debug("Trying to find package in current directory")
		wd, err := os.Getwd()
		if err != nil {
			logger.Debug("Failed to find package in directory", "error", errors.Wrap(err, errGetwd))
			return errors.Wrap(err, errGetwd)
		}
		path, err := xpkg.FindXpkgInDir(c.fs, wd)
		if err != nil {
			logger.Debug("Failed to find package in directory", "error", errors.Wrap(err, errFindPackageinWd))
			return errors.Wrap(err, errFindPackageinWd)
		}
		c.PackageFile = path
		logger.Debug("Found package in directory", "path", path)
	}
	img, err := tarball.ImageFromPath(c.PackageFile, nil)
	if err != nil {
		logger.Debug("Failed to create image from package tarball", "error", err)
		return err
	}
	kc := authn.NewMultiKeychain(
		authn.NewKeychainFromHelper(credhelper.New()),
		authn.DefaultKeychain,
	)
	if err := remote.Write(tag, img, remote.WithAuthFromKeychain(kc)); err != nil {
		logger.Debug("Failed to push created image to remote location", "error", err)
		return err
	}
	return nil
}
