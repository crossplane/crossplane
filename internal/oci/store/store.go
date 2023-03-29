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

// Package store implements OCI container storage.
package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/internal/oci/spec"
)

// Store directories.
// Shorter is better, to avoid passing too much data to the mount syscall when
// creating an overlay mount with many layers as lower directories.
const (
	DirDigests    = "d"
	DirImages     = "i"
	DirOverlays   = "o"
	DirContainers = "c"
)

// Bundle paths.
const (
	DirRootFS  = "rootfs"
	FileConfig = "config.json"
	FileSpec   = "config.json"
)

// Error strings
const (
	errMkDigestStore    = "cannot make digest store"
	errReadDigest       = "cannot read digest"
	errParseDigest      = "cannot parse digest"
	errStoreDigest      = "cannot store digest"
	errPartial          = "cannot complete partial implementation" // This should never happen.
	errInvalidImage     = "stored image is invalid"
	errGetDigest        = "cannot get digest"
	errMkAlgoDir        = "cannot create store directory"
	errGetRawConfigFile = "cannot get image config file"
	errMkTmpfile        = "cannot create temporary layer file"
	errReadLayer        = "cannot read layer"
	errMvTmpfile        = "cannot move temporary layer file"
	errOpenConfigFile   = "cannot open image config file"
	errWriteLayers      = "cannot write image layers"
	errInvalidLayer     = "stored layer is invalid"
	errWriteConfigFile  = "cannot write image config file"
	errGetLayers        = "cannot get image layers"
	errWriteLayer       = "cannot write layer"
	errOpenLayer        = "cannot open layer"
	errStatLayer        = "cannot stat layer"
	errCheckExistence   = "cannot determine whether layer exists"
)

// A Bundler prepares OCI runtime bundles for use by an OCI runtime.
type Bundler interface {
	// Bundle returns an OCI bundle ready for use by an OCI runtime.
	Bundle(ctx context.Context, i ociv1.Image, id string, o ...spec.Option) (Bundle, error)
}

// A Bundle for use by an OCI runtime.
type Bundle interface {
	// Path of the OCI bundle.
	Path() string

	// Cleanup the OCI bundle after the container has finished running.
	Cleanup() error
}

// A Digest store is used to map OCI references to digests. Each mapping is a
// file. The filename is the SHA256 hash of the reference, and the content is
// the digest in algo:hex format.
type Digest struct{ root string }

// NewDigest returns a store used to map OCI references to digests.
func NewDigest(root string) (*Digest, error) {
	// We only use sha256 hashes. The sha256 subdirectory is for symmetry with
	// the other stores, which at least hypothetically support other hashes.
	path := filepath.Join(root, DirDigests, "sha256")
	err := os.MkdirAll(path, 0700)
	return &Digest{root: path}, errors.Wrap(err, errMkDigestStore)
}

// Hash returns the stored hash for the supplied reference.
func (d *Digest) Hash(r name.Reference) (ociv1.Hash, error) {
	b, err := os.ReadFile(d.path(r))
	if err != nil {
		return ociv1.Hash{}, errors.Wrap(err, errReadDigest)
	}
	h, err := ociv1.NewHash(string(b))
	return h, errors.Wrap(err, errParseDigest)
}

// WriteHash maps the supplied reference to the supplied hash.
func (d *Digest) WriteHash(r name.Reference, h ociv1.Hash) error {
	return errors.Wrap(os.WriteFile(d.path(r), []byte(h.String()), 0600), errStoreDigest)
}

func (d *Digest) path(r name.Reference) string {
	return filepath.Join(d.root, fmt.Sprintf("%x", sha256.Sum256([]byte(r.String()))))
}

// An Image store is used to store OCI images and their layers. It uses a
// similar disk layout to the blobs directory of an OCI image layout, but may
// contain blobs for more than one image. Layers are stored as uncompressed
// tarballs in order to speed up extraction by the uncompressed Bundler, which
// extracts a fresh root filesystem each time a container is run.
// https://github.com/opencontainers/image-spec/blob/v1.0/image-layout.md
type Image struct{ root string }

// NewImage returns a store used to store OCI images and their layers.
func NewImage(root string) *Image {
	return &Image{root: filepath.Join(root, DirImages)}
}

// Image returns the stored image with the supplied hash, if any.
func (i *Image) Image(h ociv1.Hash) (ociv1.Image, error) {
	uncompressed := image{root: i.root, h: h}

	// NOTE(negz): At the time of writing UncompressedToImage doesn't actually
	// return an error.
	oi, err := partial.UncompressedToImage(uncompressed)
	if err != nil {
		return nil, errors.Wrap(err, errPartial)
	}

	// This validates the image's manifest, config file, and layers. The
	// manifest and config file are validated fairly extensively (i.e. their
	// size, digest, etc must be correct). Layers are only validated to exist.
	return oi, errors.Wrap(validate.Image(oi, validate.Fast), errInvalidImage)
}

// WriteImage writes the supplied image to the store.
func (i *Image) WriteImage(img ociv1.Image) error {
	d, err := img.Digest()
	if err != nil {
		return errors.Wrap(err, errGetDigest)
	}

	if _, err = i.Image(d); err == nil {
		// Image already exists in the store.
		return nil
	}

	path := filepath.Join(i.root, d.Algorithm, d.Hex)

	if err := os.MkdirAll(filepath.Join(i.root, d.Algorithm), 0700); err != nil {
		return errors.Wrap(err, errMkAlgoDir)
	}

	raw, err := img.RawConfigFile()
	if err != nil {
		return errors.Wrap(err, errGetRawConfigFile)
	}

	// CreateTemp creates a file with permission mode 0600.
	tmp, err := os.CreateTemp(filepath.Join(i.root, d.Algorithm), fmt.Sprintf("%s-", d.Hex))
	if err != nil {
		return errors.Wrap(err, errMkTmpfile)
	}

	if err := os.WriteFile(tmp.Name(), raw, 0600); err != nil {
		_ = os.Remove(tmp.Name())
		return errors.Wrap(err, errWriteConfigFile)
	}

	// TODO(negz): Ignore os.ErrExist? We might get one here if two callers race
	// to cache the same image.
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return errors.Wrap(err, errMvTmpfile)
	}

	layers, err := img.Layers()
	if err != nil {
		return errors.Wrap(err, errGetLayers)
	}

	g := &errgroup.Group{}
	for _, l := range layers {
		l := l // Pin loop var.
		g.Go(func() error {
			return i.WriteLayer(l)
		})
	}

	return errors.Wrap(g.Wait(), errWriteLayers)
}

// Layer returns the stored layer with the supplied hash, if any.
func (i *Image) Layer(h ociv1.Hash) (ociv1.Layer, error) {
	uncompressed := layer{root: i.root, h: h}

	// NOTE(negz): At the time of writing UncompressedToLayer doesn't actually
	// return an error.
	ol, err := partial.UncompressedToLayer(uncompressed)
	if err != nil {
		return nil, errors.Wrap(err, errPartial)
	}

	// This just validates that the layer exists on disk.
	return ol, errors.Wrap(validate.Layer(ol, validate.Fast), errInvalidLayer)
}

// WriteLayer writes the supplied layer to the store.
func (i *Image) WriteLayer(l ociv1.Layer) error {
	d, err := l.DiffID() // The digest of the uncompressed layer.
	if err != nil {
		return errors.Wrap(err, errGetDigest)
	}

	if _, err := i.Layer(d); err == nil {
		// Layer already exists in the store.
		return nil
	}

	if err := os.MkdirAll(filepath.Join(i.root, d.Algorithm), 0700); err != nil {
		return errors.Wrap(err, errMkAlgoDir)
	}

	// CreateTemp creates a file with permission mode 0600.
	tmp, err := os.CreateTemp(filepath.Join(i.root, d.Algorithm), fmt.Sprintf("%s-", d.Hex))
	if err != nil {
		return errors.Wrap(err, errMkTmpfile)
	}

	// This call to Uncompressed is what actually pulls the layer.
	u, err := l.Uncompressed()
	if err != nil {
		_ = os.Remove(tmp.Name())
		return errors.Wrap(err, errReadLayer)
	}

	if _, err := copyChunks(tmp, u, 1024*1024); err != nil { // Copy 1MB chunks.
		_ = os.Remove(tmp.Name())
		return errors.Wrap(err, errWriteLayer)
	}

	// TODO(negz): Ignore os.ErrExist? We might get one here if two callers race
	// to cache the same layer.
	if err := os.Rename(tmp.Name(), filepath.Join(i.root, d.Algorithm, d.Hex)); err != nil {
		_ = os.Remove(tmp.Name())
		return errors.Wrap(err, errMvTmpfile)
	}

	return nil
}

// image implements partial.UncompressedImage per
// https://pkg.go.dev/github.com/google/go-containerregistry/pkg/v1/partial
type image struct {
	root string
	h    ociv1.Hash
}

func (i image) RawConfigFile() ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(i.root, i.h.Algorithm, i.h.Hex))
	return b, errors.Wrap(err, errOpenConfigFile)
}

func (i image) MediaType() (types.MediaType, error) {
	return types.OCIManifestSchema1, nil
}

func (i image) LayerByDiffID(h ociv1.Hash) (partial.UncompressedLayer, error) {
	return layer{root: i.root, h: h}, nil
}

// layer implements partial.UncompressedLayer per
// https://pkg.go.dev/github.com/google/go-containerregistry/pkg/v1/partial
type layer struct {
	root string
	h    ociv1.Hash
}

func (l layer) DiffID() (v1.Hash, error) {
	return l.h, nil
}

func (l layer) Uncompressed() (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(l.root, l.h.Algorithm, l.h.Hex))
	return f, errors.Wrap(err, errOpenLayer)
}

func (l layer) MediaType() (types.MediaType, error) {
	return types.OCIUncompressedLayer, nil
}

// Exists satisfies partial.Exists, which is used to validate the image when
// validate.Image or validate.Layer is run with the validate.Fast option.
func (l layer) Exists() (bool, error) {
	_, err := os.Stat(filepath.Join(l.root, l.h.Algorithm, l.h.Hex))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, errStatLayer)
	}
	return true, nil
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
