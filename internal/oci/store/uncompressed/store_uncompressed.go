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

// Package uncompressed implemented an uncompressed layer based container store.
package uncompressed

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/internal/oci/layer"
	"github.com/crossplane/crossplane/internal/oci/spec"
	"github.com/crossplane/crossplane/internal/oci/store"
)

// Error strings
const (
	errMkContainerStore  = "cannot make container store directory"
	errMkImageStore      = "cannot make image cache directory"
	errMkLayerStore      = "cannot make layer cache directory"
	errReadConfigFile    = "cannot read image config file"
	errGetLayers         = "cannot get image layers"
	errMkRootFS          = "cannot make rootfs directory"
	errOpenLayer         = "cannot open uncompressed tarball layer"
	errApplyLayer        = "cannot apply (extract) uncompressed tarball layer"
	errCloseLayer        = "cannot close uncompressed tarball layer"
	errCreateRuntimeSpec = "cannot create OCI runtime spec"
	errGetDigest         = "cannot get digest"
	errMkWorkdir         = "cannot create temporary work directory"
	errMvWorkdir         = "cannot move temporary work directory"
	errFetchLayer        = "cannot fetch and decompress layer"
	errWriteLayer        = "cannot write layer to temporary work file"
	errMkWorkfile        = "cannot create temporary work file"
	errMvWorkfile        = "cannot move temporary work file"
	errCleanupBundle     = "cannot cleanup OCI runtime bundle"
)

// An ImageConfigReader reads OCI image configuration files.
type ImageConfigReader interface {
	ReadConfigFile(i ociv1.Image) (*ociv1.ConfigFile, error)
}

// An LayerOpener opens the supplied layer.
type LayerOpener interface {
	// Open returns an io.ReadCloser backed by the uncompressed layer tarball.
	Open(l ociv1.Layer) (io.ReadCloser, error)
}

// A TarballApplicator applies (i.e. extracts) an OCI layer tarball.
// https://github.com/opencontainers/image-spec/blob/v1.0/layer.md
type TarballApplicator interface {
	// Apply the supplied tarball - an OCI filesystem layer - to the supplied
	// root directory. Applying all of an image's layers, in the correct order,
	// should produce the image's "flattened" filesystem.
	Apply(ctx context.Context, tb io.Reader, root string) error
}

// A RuntimeSpecCreator creates (and writes) an OCI runtime spec for the
// supplied bundle.
type RuntimeSpecCreator interface {
	// Create and write an OCI runtime spec for the supplied bundle, deriving
	// configuration from the supplied OCI image config file as appropriate.
	Create(b store.Bundle, cfg *ociv1.ConfigFile) error
}

// A RuntimeSpecCreatorFn allows a function to satisfy RuntimeSpecCreator.
type RuntimeSpecCreatorFn func(b store.Bundle, cfg *ociv1.ConfigFile) error

// Create and write an OCI runtime spec for the supplied bundle, deriving
// configuration from the supplied OCI image config file as appropriate.
func (fn RuntimeSpecCreatorFn) Create(b store.Bundle, cfg *ociv1.ConfigFile) error { return fn(b, cfg) }

// A CachingBundler prepares OCI runtime bundles for use by an OCI runtime. When
// asked to 'bundle' a container it will attempt to read the OCI image's
// configuration and layers from a cache. If the cache has not been populated it
// will fetch the OCI image configuration and layers, caching them to disk.
// Layers are cached as uncompressed tarballs, and are extracted each time a
// container is bundled in order to create its root filesystem.
type CachingBundler struct {
	root    string
	image   ImageConfigReader
	layer   LayerOpener
	tarball TarballApplicator
	spec    RuntimeSpecCreator
}

// NewCachingBundler returns a an OCI runtime bundler that caches image layers
// as uncompressed tarballs.
func NewCachingBundler(root string) (*CachingBundler, error) {
	if err := os.MkdirAll(filepath.Join(root, store.DirContainers), 0700); err != nil {
		return nil, errors.Wrap(err, errMkContainerStore)
	}

	i, err := store.NewCachingImageConfigReader(filepath.Join(root, store.DirImages))
	if err != nil {
		return nil, errors.Wrap(err, errMkImageStore)
	}
	l, err := NewCachingLayerOpener(filepath.Join(root, store.DirLayers))
	if err != nil {
		return nil, errors.Wrap(err, errMkLayerStore)
	}

	s := &CachingBundler{
		root:    filepath.Join(root, store.DirContainers),
		tarball: layer.NewStackingExtractor(layer.NewWhiteoutHandler(layer.NewExtractHandler())),
		image:   i,
		layer:   l,
		spec:    RuntimeSpecCreatorFn(spec.Create),
	}
	return s, nil
}

// Bundle returns an OCI bundle ready for use by an OCI runtime. The supplied
// image will be fetched and cached in the store if it does not already exist.
func (c *CachingBundler) Bundle(ctx context.Context, i ociv1.Image, id string) (store.Bundle, error) {
	cfg, err := c.image.ReadConfigFile(i)
	if err != nil {
		return nil, errors.Wrap(err, errReadConfigFile)
	}

	layers, err := i.Layers()
	if err != nil {
		return nil, errors.Wrap(err, errGetLayers)
	}

	path := filepath.Join(c.root, id)
	if err := os.MkdirAll(filepath.Join(path, store.DirRootFS), 0700); err != nil {
		return nil, errors.Wrap(err, errMkRootFS)
	}
	b := Bundle{path: path}

	for i := range layers {
		tb, err := c.layer.Open(layers[i])
		if err != nil {
			_ = b.Cleanup()
			return nil, errors.Wrap(err, errOpenLayer)
		}
		if err := c.tarball.Apply(ctx, tb, filepath.Join(path, store.DirRootFS)); err != nil {
			_ = tb.Close()
			_ = b.Cleanup()
			return nil, errors.Wrap(err, errApplyLayer)
		}
		if err := tb.Close(); err != nil {
			_ = b.Cleanup()
			return nil, errors.Wrap(err, errCloseLayer)
		}
	}

	// Create an OCI runtime config file from the OCI image config file. We do
	// this every time we run the function because in future it's likely that
	// we'll want to derive the OCI runtime config file from both the OCI image
	// config file and user supplied input (i.e. from the functions array of a
	// Composition).
	if err := c.spec.Create(b, cfg); err != nil {
		_ = b.Cleanup()
		return nil, errors.Wrap(err, errCreateRuntimeSpec)
	}

	return b, nil
}

// A CachingLayerOpener opens an uncompressed OCI layer tarball for reading. The
// layer is cached upon first read, and read from cache on subsequent calls.
type CachingLayerOpener struct {
	root string
}

// NewCachingLayerOpener returns a LayerOpener that caches uncompressed layer
// tarballs upon first open, and opens from cache on subsequent calls.
func NewCachingLayerOpener(root string) (*CachingLayerOpener, error) {
	return &CachingLayerOpener{root: root}, os.MkdirAll(root, 0700)
}

// Open the supplied layer as an uncompressed tarball.
func (c *CachingLayerOpener) Open(l ociv1.Layer) (io.ReadCloser, error) {
	d, err := l.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errGetDigest)
	}

	// Note terr, not err, to avoid shadowing in the ErrNotExist block.
	tb, terr := os.Open(filepath.Join(c.root, d.Hex))
	if errors.Is(terr, os.ErrNotExist) {
		// Doesn't exist - cache it. It's possible multiple callers may hit this
		// branch at once. This will result in multiple calls to l.Uncompressed,
		// thus pulling the layer multiple times to multiple different temporary
		// files. Calls to os.Rename should succeed if newpath is a regular file
		// that exists, so whoever finishes last will successfully move their
		// cached layer into place.

		// This call to Uncompressed is what actually pulls the layer.
		u, err := l.Uncompressed()
		if err != nil {
			return nil, errors.Wrap(err, errFetchLayer)
		}

		// CreateTemp creates a file with permission mode 0600.
		tmp, err := os.CreateTemp(c.root, fmt.Sprintf("wrk-%s-", d.Hex))
		if err != nil {
			return nil, errors.Wrap(err, errMkWorkfile)
		}

		if _, err := copyChunks(tmp, u, 1024*1024); err != nil { // Copy 1MB chunks.
			_ = os.Remove(tmp.Name())
			return nil, errors.Wrap(err, errWriteLayer)
		}

		if err := os.Rename(tmp.Name(), filepath.Join(c.root, d.Hex)); err != nil {
			_ = os.Remove(tmp.Name())
			return nil, errors.Wrap(err, errMvWorkfile)
		}

		tb, terr = os.Open(filepath.Join(c.root, d.Hex))
	}
	return tb, errors.Wrap(terr, errOpenLayer)
}

// An Bundle is an OCI runtime bundle. Its root filesystem is a temporary
// extraction of its image's cached layers.
type Bundle struct {
	path string
}

// Path to the OCI bundle.
func (b Bundle) Path() string { return b.path }

// Cleanup the OCI bundle.
func (b Bundle) Cleanup() error {
	return errors.Wrap(os.RemoveAll(b.path), errCleanupBundle)
}

// copyChunks pleases gosec per https://github.com/securego/gosec/pull/433.
// Like Copy it reads from src until EOF, it does not treat an EOF from Read as
// an error to be reported.
//
// NOTE(negz): This rule confused me at first because io.Copy appears to use a
// buffer, but in fact it bypasses it if src/dst is an io.WriterTo/ReaderFrom.
func copyChunks(dst io.Writer, src io.Reader, chunkSize int64) (int64, error) {
	var written int64
	for {
		w, err := io.CopyN(dst, src, chunkSize)
		written += w
		if errors.Is(err, io.EOF) {
			return written, nil
		}
		if err != nil {
			return written, err
		}
	}
}
