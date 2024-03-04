/*
Copyright 2020 The Crossiane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in comiiance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by apiicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or imiied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package revision

import (
	"archive/tar"
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/validate"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errBadReference            = "package tag is not a valid reference"
	errFetchPackage            = "failed to fetch package from remote"
	errGetManifest             = "failed to get package image manifest from remote"
	errFetchLayer              = "failed to fetch annotated base layer from remote"
	errGetUncompressed         = "failed to get uncompressed contents from layer"
	errMultipleAnnotatedLayers = "package is invalid due to multiple annotated base layers"
	errFmtNoPackageFileFound   = "couldn't find \"" + xpkg.StreamFile + "\" file after checking %d files in the archive (annotated layer: %v)"
	errFmtMaxManifestLayers    = "package has %d layers, but only %d are allowed"
	errValidateLayer           = "invalid package layer"
	errValidateImage           = "invalid package image"
)

const (
	layerAnnotation     = "io.crossplane.xpkg"
	baseAnnotationValue = "base"
	// maxLayers is the maximum number of layers an image can have.
	maxLayers = 256
)

// ImageBackend is a backend for parser.
type ImageBackend struct {
	registry string
	fetcher  xpkg.Fetcher
}

// An ImageBackendOption sets configuration for an image backend.
type ImageBackendOption func(i *ImageBackend)

// WithDefaultRegistry sets the default registry that an image backend will use.
func WithDefaultRegistry(registry string) ImageBackendOption {
	return func(i *ImageBackend) {
		i.registry = registry
	}
}

// NewImageBackend creates a new image backend.
func NewImageBackend(fetcher xpkg.Fetcher, opts ...ImageBackendOption) *ImageBackend {
	i := &ImageBackend{
		fetcher: fetcher,
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// Init initializes an ImageBackend.
func (i *ImageBackend) Init(ctx context.Context, bo ...parser.BackendOption) (io.ReadCloser, error) {
	// NOTE(hasheddan): we use nestedBackend here because simultaneous
	// reconciles of providers or configurations can lead to the package
	// revision being overwritten mid-execution in the shared image backend when
	// it is a member of the image backend struct. We could introduce a lock
	// here, but there is no reason why a given reconcile should require
	// exclusive access to the image backend other than its poor design. We
	// should consider restructuring the parser backend interface to better
	// accommodate for shared, thread-safe backends.
	n := &nestedBackend{}
	for _, o := range bo {
		o(n)
	}
	ref, err := name.ParseReference(n.pr.GetSource(), name.WithDefaultRegistry(i.registry))
	if err != nil {
		return nil, errors.Wrap(err, errBadReference)
	}
	// Fetch image from registry.
	img, err := i.fetcher.Fetch(ctx, ref, v1.RefNames(n.pr.GetPackagePullSecrets())...)
	if err != nil {
		return nil, errors.Wrap(err, errFetchPackage)
	}
	// Get image manifest.
	manifest, err := img.Manifest()
	if err != nil {
		return nil, errors.Wrap(err, errGetManifest)
	}

	// Check that the image has less than the maximum allowed number of layers.
	if nLayers := len(manifest.Layers); nLayers > maxLayers {
		return nil, errors.Errorf(errFmtMaxManifestLayers, nLayers, maxLayers)
	}

	// Determine if the image is using annotated layers.
	var tarc io.ReadCloser
	foundAnnotated := false
	for _, l := range manifest.Layers {
		if a, ok := l.Annotations[layerAnnotation]; !ok || a != baseAnnotationValue {
			continue
		}
		// NOTE(hasheddan): the xpkg specification dictates that only one layer
		// descriptor may be annotated as xpkg base. Since iterating through all
		// descriptors is relatively inexpensive, we opt to do so in order to
		// verify that we aren't just using the first layer annotated as xpkg
		// base.
		if foundAnnotated {
			return nil, errors.New(errMultipleAnnotatedLayers)
		}
		foundAnnotated = true
		layer, err := img.LayerByDigest(l.Digest)
		if err != nil {
			return nil, errors.Wrap(err, errFetchLayer)
		}
		if err := validate.Layer(layer); err != nil {
			return nil, errors.Wrap(err, errValidateLayer)
		}
		tarc, err = layer.Uncompressed()
		if err != nil {
			return nil, errors.Wrap(err, errGetUncompressed)
		}
	}

	// If we still don't have content then we need to flatten image filesystem.
	if !foundAnnotated {
		if err := validate.Image(img); err != nil {
			return nil, errors.Wrap(err, errValidateImage)
		}
		tarc = mutate.Extract(img)
	}

	// The ReadCloser is an uncompressed tarball, either consisting of annotated
	// layer contents or flattened filesystem content. Either way, we only want
	// the package YAML stream.
	t := tar.NewReader(tarc)
	var read int
	for {
		h, err := t.Next()
		if err != nil {
			return nil, errors.Wrapf(err, errFmtNoPackageFileFound, read, foundAnnotated)
		}
		if h.Name == xpkg.StreamFile {
			break
		}
		read++
	}

	// NOTE(hasheddan): we return a JoinedReadCloser such that closing will free
	// resources allocated to the underlying ReadCloser. See
	// https://github.com/google/go-containerregistry/blob/329563766ce8131011c25fd8758a25d94d9ad81b/pkg/v1/mutate/mutate.go#L222
	// for more info.
	return xpkg.JoinedReadCloser(t, tarc), nil
}

// nestedBackend is a nop parser backend that conforms to the parser backend
// interface to allow holding intermediate data passed via parser backend
// options.
// NOTE(hasheddan): see usage in ImageBackend Init() for reasoning.
type nestedBackend struct {
	pr v1.PackageRevision
}

// Init is a nop because nestedBackend does not actually meant to act as a
// parser backend.
func (n *nestedBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return nil, nil
}

// PackageRevision sets the package revision for ImageBackend.
func PackageRevision(pr v1.PackageRevision) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*nestedBackend)
		if !ok {
			return
		}
		i.pr = pr
	}
}
