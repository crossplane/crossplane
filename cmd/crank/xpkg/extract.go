/*
Copyright 2025 The Crossplane Authors.

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
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errMustProvideTag          = "must provide package tag if fetching from registry or daemon"
	errInvalidTag              = "package tag is not a valid reference"
	errFetchPackage            = "failed to fetch package from remote"
	errGetManifest             = "failed to get package image manifest from remote"
	errFetchLayer              = "failed to fetch annotated base layer from remote"
	errGetUncompressed         = "failed to get uncompressed contents from layer"
	errMultipleAnnotatedLayers = "package is invalid due to multiple annotated base layers"
	errOpenPackageStream       = "failed to open package stream file"
	errCreateOutputFile        = "failed to create output file"
	errCreateGzipWriter        = "failed to create gzip writer"
	errExtractPackageContents  = "failed to extract package contents"
	cacheContentExt            = ".gz"
)

// fetchFn fetches a package from a source.
type fetchFn func(context.Context, name.Reference) (v1.Image, error)

// registryFetch fetches a package from the registry.
func registryFetch(ctx context.Context, r name.Reference) (v1.Image, error) {
	// Use default docker auth, i.e. for private repositories.
	kc := authn.NewMultiKeychain(authn.DefaultKeychain)
	return remote.Image(r, remote.WithContext(ctx), remote.WithAuthFromKeychain(kc))
}

// daemonFetch fetches a package from the Docker daemon.
func daemonFetch(ctx context.Context, r name.Reference) (v1.Image, error) {
	return daemon.Image(r, daemon.WithContext(ctx))
}

func xpkgFetch(path string) fetchFn {
	return func(_ context.Context, _ name.Reference) (v1.Image, error) {
		return tarball.ImageFromPath(filepath.Clean(path), nil)
	}
}

// AfterApply constructs and binds context to any subcommands
// that have Run() methods that receive it.
func (c *extractCmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	c.fetch = registryFetch
	if c.FromDaemon {
		c.fetch = daemonFetch
	}
	if c.FromXpkg {
		// If package is not defined, attempt to find single package in current
		// directory.
		if c.Package == "" {
			wd, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, errGetwd)
			}
			path, err := xpkg.FindXpkgInDir(c.fs, wd)
			if err != nil {
				return errors.Wrap(err, errFindPackageinWd)
			}
			c.Package = path
		}
		c.fetch = xpkgFetch(c.Package)
	}
	if !c.FromXpkg {
		if c.Package == "" {
			return errors.New(errMustProvideTag)
		}

		name, err := name.ParseReference(c.Package, name.StrictValidation)
		if err != nil {
			return errors.Wrap(err, errInvalidTag)
		}
		c.name = name
	}
	return nil
}

// extractCmd extracts package contents into a Crossplane cache compatible
// format.
type extractCmd struct {
	fs    afero.Fs
	name  name.Reference
	fetch fetchFn

	Package    string `arg:""                                                                                                                                                     help:"Name of the package to extract. Must be a valid OCI image tag or a path if using --from-xpkg." optional:""`
	FromDaemon bool   `help:"Indicates that the image should be fetched from the Docker daemon."`
	FromXpkg   bool   `help:"Indicates that the image should be fetched from a local xpkg. If package is not specified and only one exists in current directory it will be used."`
	Output     string `default:"out.gz"                                                                                                                                           help:"Package output file path. Extension must be .gz or will be replaced."                          short:"o"`
}

// Run runs the xpkg extract cmd.
func (c *extractCmd) Run(logger logging.Logger) error { //nolint:gocyclo // xpkg extract for cli
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch package.
	img, err := c.fetch(ctx, c.name)
	if err != nil {
		return errors.Wrap(err, errFetchPackage)
	}

	// Get image manifest.
	manifest, err := img.Manifest()
	if err != nil {
		return errors.Wrap(err, errGetManifest)
	}

	// Determine if the image is using annotated layers.
	var tarc io.ReadCloser
	foundAnnotated := false
	for _, l := range manifest.Layers {
		if a, ok := l.Annotations[xpkg.AnnotationKey]; !ok || a != xpkg.PackageAnnotation {
			continue
		}
		if foundAnnotated {
			return errors.New(errMultipleAnnotatedLayers)
		}
		foundAnnotated = true
		layer, err := img.LayerByDigest(l.Digest)
		if err != nil {
			return errors.Wrap(err, errFetchLayer)
		}
		tarc, err = layer.Uncompressed()
		if err != nil {
			return errors.Wrap(err, errGetUncompressed)
		}
	}

	// If we still don't have content then we need to flatten image filesystem.
	if !foundAnnotated {
		tarc = mutate.Extract(img)
	}

	// The ReadCloser is an uncompressed tarball, either consisting of annotated
	// layer contents or flattened filesystem content. Either way, we only want
	// the package YAML stream.
	t := tar.NewReader(tarc)
	var size int64
	for {
		h, err := t.Next()
		if err != nil {
			return errors.Wrap(err, errOpenPackageStream)
		}
		if h.Name == xpkg.StreamFile {
			size = h.Size
			break
		}
	}

	out := xpkg.ReplaceExt(filepath.Clean(c.Output), cacheContentExt)
	cf, err := c.fs.Create(out)
	if err != nil {
		return errors.Wrap(err, errCreateOutputFile)
	}
	defer cf.Close() //nolint:errcheck // defer close
	w, err := gzip.NewWriterLevel(cf, gzip.BestSpeed)
	if err != nil {
		return errors.Wrap(err, errCreateGzipWriter)
	}
	if _, err = io.CopyN(w, t, size); err != nil {
		return errors.Wrap(err, errExtractPackageContents)
	}
	if err := w.Close(); err != nil {
		return errors.Wrap(err, errExtractPackageContents)
	}

	logger.Debug("xpkg contents extracted to %s", out)
	return nil
}
