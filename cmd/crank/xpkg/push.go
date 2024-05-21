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
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/upbound"
	"github.com/crossplane/crossplane/internal/xpkg/upbound/credhelper"
)

const (
	errGetwd           = "failed to get working directory while searching for package"
	errFindPackageinWd = "failed to find a package in current working directory"
	errAnnotateLayers  = "failed to propagate xpkg annotations from OCI image config file to image layers"

	errFmtNewTag        = "failed to parse package tag %q"
	errFmtReadPackage   = "failed to read package file %s"
	errFmtPushPackage   = "failed to push package file %s"
	errFmtGetDigest     = "failed to get digest of package file %s"
	errFmtNewDigest     = "failed to parse digest %q for package file %s"
	errFmtGetMediaType  = "failed to get media type of package file %s"
	errFmtGetConfigFile = "failed to get OCI config file of package file %s"
	errFmtWriteIndex    = "failed to push an OCI image index of %d packages"
)

// pushCmd pushes a package.
type pushCmd struct {
	// Arguments.
	Package string `arg:"" help:"Where to push the package."`

	// Flags. Keep sorted alphabetically.
	PackageFiles []string `help:"A comma-separated list of xpkg files to push." placeholder:"PATH" short:"f" type:"existingfile"`

	// Common Upbound API configuration.
	upbound.Flags `embed:""`

	// Internal state. These aren't part of the user-exposed CLI structure.
	fs afero.Fs
}

func (c *pushCmd) Help() string {
	return `
Packages can be pushed to any OCI registry. Packages are pushed to the
xpkg.upbound.io registry by default. A package's OCI tag must be a semantic
version. Credentials for the registry are automatically retrieved from xpkg login 
and dockers configuration as fallback.

Examples:

  # Push a multi-platform package.
  crossplane xpkg push -f function-amd64.xpkg,function-arm64.xpkg crossplane/function-example:v1.0.0

  # Push the xpkg file in the current directory to a different registry.
  crossplane xpkg push index.docker.io/crossplane/function-example:v1.0.0
`
}

// AfterApply sets the tag for the parent push command.
func (c *pushCmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run runs the push cmd.
func (c *pushCmd) Run(logger logging.Logger) error { //nolint:gocognit // This feels easier to read as-is.
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}

	tag, err := name.NewTag(c.Package, name.WithDefaultRegistry(xpkg.DefaultRegistry))
	if err != nil {
		return errors.Wrapf(err, errFmtNewTag, c.Package)
	}

	// If package is not defined, attempt to find single package in current
	// directory.
	if len(c.PackageFiles) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, errGetwd)
		}
		path, err := xpkg.FindXpkgInDir(c.fs, wd)
		if err != nil {
			return errors.Wrap(err, errFindPackageinWd)
		}
		c.PackageFiles = []string{path}
		logger.Debug("Found package in directory", "path", path)
	}

	kc := authn.NewMultiKeychain(
		authn.NewKeychainFromHelper(credhelper.New(
			credhelper.WithLogger(logger),
			credhelper.WithProfile(upCtx.ProfileName),
			credhelper.WithDomain(upCtx.Domain.Hostname()),
		)),
		authn.DefaultKeychain,
	)

	// If there's only one package file, handle the simple path.
	if len(c.PackageFiles) == 1 {
		img, err := tarball.ImageFromPath(c.PackageFiles[0], nil)
		if err != nil {
			return errors.Wrapf(err, errFmtReadPackage, c.PackageFiles[0])
		}
		img, err = xpkg.AnnotateLayers(img)
		if err != nil {
			return errors.Wrapf(err, errAnnotateLayers)
		}
		if err := remote.Write(tag, img, remote.WithAuthFromKeychain(kc)); err != nil {
			return errors.Wrapf(err, errFmtPushPackage, c.PackageFiles[0])
		}
		logger.Debug("Pushed package", "path", c.PackageFiles[0], "ref", tag.String())
		return nil
	}

	// If there's more than one package file we'll write (push) them all by
	// their digest, and create an index with the specified tag. This pattern is
	// typically used to create a multi-platform image.
	adds := make([]mutate.IndexAddendum, len(c.PackageFiles))
	g, ctx := errgroup.WithContext(context.Background())
	for i, file := range c.PackageFiles {
		g.Go(func() error {
			img, err := tarball.ImageFromPath(filepath.Clean(file), nil)
			if err != nil {
				return errors.Wrapf(err, errFmtReadPackage, file)
			}

			img, err = xpkg.AnnotateLayers(img)
			if err != nil {
				return errors.Wrapf(err, errAnnotateLayers)
			}

			d, err := img.Digest()
			if err != nil {
				return errors.Wrapf(err, errFmtGetDigest, file)
			}
			n := fmt.Sprintf("%s@%s", tag.Repository.Name(), d.String())
			ref, err := name.NewDigest(n, name.WithDefaultRegistry(xpkg.DefaultRegistry))
			if err != nil {
				return errors.Wrapf(err, errFmtNewDigest, n, file)
			}

			mt, err := img.MediaType()
			if err != nil {
				return errors.Wrapf(err, errFmtGetMediaType, file)
			}

			conf, err := img.ConfigFile()
			if err != nil {
				return errors.Wrapf(err, errFmtGetConfigFile, file)
			}

			adds[i] = mutate.IndexAddendum{
				Add: img,
				Descriptor: v1.Descriptor{
					MediaType: mt,
					Platform: &v1.Platform{
						Architecture: conf.Architecture,
						OS:           conf.OS,
						OSVersion:    conf.OSVersion,
					},
				},
			}
			if err := remote.Write(ref, img, remote.WithAuthFromKeychain(kc), remote.WithContext(ctx)); err != nil {
				return errors.Wrapf(err, errFmtPushPackage, file)
			}
			logger.Debug("Pushed package", "path", file, "ref", ref.String())
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := remote.WriteIndex(tag, mutate.AppendManifests(empty.Index, adds...), remote.WithAuthFromKeychain(kc)); err != nil {
		return errors.Wrapf(err, errFmtWriteIndex, len(adds))
	}
	logger.Debug("Wrote OCI index", "ref", tag.String(), "manifests", len(adds))
	return nil
}
