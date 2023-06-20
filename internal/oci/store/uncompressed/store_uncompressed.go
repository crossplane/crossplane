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
	errReadConfigFile   = "cannot read image config file"
	errGetLayers        = "cannot get image layers"
	errMkRootFS         = "cannot make rootfs directory"
	errOpenLayer        = "cannot open layer tarball"
	errApplyLayer       = "cannot extract layer tarball"
	errCloseLayer       = "cannot close layer tarball"
	errWriteRuntimeSpec = "cannot write OCI runtime spec"
	errCleanupBundle    = "cannot cleanup OCI runtime bundle"
)

// A TarballApplicator applies (i.e. extracts) an OCI layer tarball.
// https://github.com/opencontainers/image-spec/blob/v1.0/layer.md
type TarballApplicator interface {
	// Apply the supplied tarball - an OCI filesystem layer - to the supplied
	// root directory. Applying all of an image's layers, in the correct order,
	// should produce the image's "flattened" filesystem.
	Apply(ctx context.Context, tb io.Reader, root string) error
}

// A RuntimeSpecWriter writes an OCI runtime spec to the supplied path.
type RuntimeSpecWriter interface {
	// Write and write an OCI runtime spec to the supplied path.
	Write(path string, o ...spec.Option) error
}

// A RuntimeSpecWriterFn allows a function to satisfy RuntimeSpecCreator.
type RuntimeSpecWriterFn func(path string, o ...spec.Option) error

// Write an OCI runtime spec to the supplied path.
func (fn RuntimeSpecWriterFn) Write(path string, o ...spec.Option) error { return fn(path, o...) }

// A Bundler prepares OCI runtime bundles for use by an OCI runtime. It creates
// the bundle's rootfs by extracting the supplied image's uncompressed layer
// tarballs.
type Bundler struct {
	root    string
	tarball TarballApplicator
	spec    RuntimeSpecWriter
}

// NewBundler returns a an OCI runtime bundler that creates a bundle's rootfs by
// extracting uncompressed layer tarballs.
func NewBundler(root string) *Bundler {
	s := &Bundler{
		root:    filepath.Join(root, store.DirContainers),
		tarball: layer.NewStackingExtractor(layer.NewWhiteoutHandler(layer.NewExtractHandler())),
		spec:    RuntimeSpecWriterFn(spec.Write),
	}
	return s
}

// Bundle returns an OCI bundle ready for use by an OCI runtime.
func (c *Bundler) Bundle(ctx context.Context, i ociv1.Image, id string, o ...spec.Option) (store.Bundle, error) {
	cfg, err := i.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, errReadConfigFile)
	}

	layers, err := i.Layers()
	if err != nil {
		return nil, errors.Wrap(err, errGetLayers)
	}

	path := filepath.Join(c.root, id)
	rootfs := filepath.Join(path, store.DirRootFS)
	if err := os.MkdirAll(rootfs, 0700); err != nil {
		return nil, errors.Wrap(err, errMkRootFS)
	}
	b := Bundle{path: path}

	if err := store.Validate(i); err != nil {
		return nil, err
	}

	for _, l := range layers {
		tb, err := l.Uncompressed()
		if err != nil {
			_ = b.Cleanup()
			return nil, errors.Wrap(err, errOpenLayer)
		}
		if err := c.tarball.Apply(ctx, tb, rootfs); err != nil {
			_ = tb.Close()
			_ = b.Cleanup()
			return nil, errors.Wrap(err, errApplyLayer)
		}
		if err := tb.Close(); err != nil {
			_ = b.Cleanup()
			return nil, errors.Wrap(err, errCloseLayer)
		}
	}

	// Inject config derived from the image first, so that any options passed in
	// by the caller will override it.
	p, g := filepath.Join(rootfs, "etc", "passwd"), filepath.Join(rootfs, "etc", "group")
	opts := append([]spec.Option{spec.WithImageConfig(cfg, p, g), spec.WithRootFS(store.DirRootFS, true)}, o...)

	if err = c.spec.Write(filepath.Join(path, store.FileSpec), opts...); err != nil {
		_ = b.Cleanup()
		return nil, errors.Wrap(err, errWriteRuntimeSpec)
	}

	return b, nil
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
