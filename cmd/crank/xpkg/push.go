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
	"crypto/tls"
	"fmt"
	"net/http"
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

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/xpkg"
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
	Package string `arg:"" help:"Where to push the package. Must be a fully qualified OCI tag, including the registry, repository, and tag." placeholder:"REGISTRY/REPOSITORY:TAG"`

	// Flags. Keep sorted alphabetically.
	InsecureSkipTLSVerify bool     `help:"[INSECURE] Skip verifying TLS certificates."`
	PackageFiles          []string `help:"A comma-separated list of xpkg files to push." placeholder:"PATH" predictor:"xpkg_file" short:"f" type:"existingfile"`

	// Internal state. These aren't part of the user-exposed CLI structure.
	fs afero.Fs
}

func (c *pushCmd) Help() string {
	return `
Packages can be pushed to any OCI registry. A package's OCI tag must be a semantic
version. Credentials for the registry are automatically retrieved from xpkg login
and dockers configuration as fallback.

IMPORTANT: the package must be fully qualified, including the registry, repository, and tag.

Examples:

  # Push a multi-platform package.
  crossplane xpkg push -f function-amd64.xpkg,function-arm64.xpkg xpkg.crossplane.io/crossplane/function-example:v1.0.0

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
func (c *pushCmd) Run(logger logging.Logger) error {
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

	// load images from all the provided package files
	images := make([]packageImage, 0, len(c.PackageFiles))
	for _, p := range c.PackageFiles {
		cleanPath := filepath.Clean(p)

		img, err := tarball.ImageFromPath(cleanPath, nil)
		if err != nil {
			return err
		}

		images = append(images, packageImage{Image: img, Path: cleanPath})
	}

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipTLSVerify, //nolint:gosec // we need to support insecure connections if requested
		},
	}

	options := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithTransport(t),
	}

	return pushImages(logger, images, c.Package, options...)
}

// packageImage describes a package image that will be pushed.
type packageImage struct {
	// The OCI Image of the package to be pushed.
	Image v1.Image

	// optional path for the image (e.g. file path on disk) to help provide more
	// information about its source
	Path string
}

// pushImages pushes package images to the given URL using the provided options.
func pushImages(logger logging.Logger, images []packageImage, url string, options ...remote.Option) error {
	if len(options) == 0 {
		options = []remote.Option{
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
		}
	}

	tag, err := name.NewTag(url, name.StrictValidation)
	if err != nil {
		return errors.Wrapf(err, errFmtNewTag, url)
	}

	// If there's only one package file, handle the simple path.
	if len(images) == 1 {
		pi := images[0]

		img, err := xpkg.AnnotateLayers(pi.Image)
		if err != nil {
			return errors.Wrapf(err, errAnnotateLayers)
		}

		if err := remote.Write(tag, img, options...); err != nil {
			return errors.Wrapf(err, errFmtPushPackage, pi.Path)
		}

		logger.Debug("Pushed package", "path", pi.Path, "ref", tag.String())

		return nil
	}

	// If there's more than one package file we'll write (push) them all by
	// their digest, and create an index with the specified tag. This pattern is
	// typically used to create a multi-platform image.
	adds := make([]mutate.IndexAddendum, len(images))

	g, ctx := errgroup.WithContext(context.Background())
	for i, pi := range images {
		g.Go(func() error {
			img, err := xpkg.AnnotateLayers(pi.Image)
			if err != nil {
				return errors.Wrapf(err, errAnnotateLayers)
			}

			d, err := img.Digest()
			if err != nil {
				return errors.Wrapf(err, errFmtGetDigest, pi.Path)
			}

			n := fmt.Sprintf("%s@%s", tag.Repository.Name(), d.String())

			ref, err := name.NewDigest(n, name.StrictValidation)
			if err != nil {
				return errors.Wrapf(err, errFmtNewDigest, n, pi.Path)
			}

			mt, err := img.MediaType()
			if err != nil {
				return errors.Wrapf(err, errFmtGetMediaType, pi.Path)
			}

			conf, err := img.ConfigFile()
			if err != nil {
				return errors.Wrapf(err, errFmtGetConfigFile, pi.Path)
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
			if err := remote.Write(ref, img, append(options, remote.WithContext(ctx))...); err != nil {
				return errors.Wrapf(err, errFmtPushPackage, pi.Path)
			}

			logger.Debug("Pushed package", "path", pi.Path, "ref", ref.String())

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := remote.WriteIndex(tag, mutate.AppendManifests(empty.Index, adds...), options...); err != nil {
		return errors.Wrapf(err, errFmtWriteIndex, len(adds))
	}

	logger.Debug("Wrote OCI index", "ref", tag.String(), "manifests", len(adds))

	return nil
}
