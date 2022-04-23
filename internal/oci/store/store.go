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
	"fmt"
	"os"
	"path/filepath"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Store directories.
// Shorter is better, to avoid passing too much data to the mount syscall when
// creating an overlay mount with many layers as lower directories.
const (
	DirLayers     = "l"
	DirImages     = "i"
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
	errMkImageStore     = "cannot make image cache directory"
	errGetDigest        = "cannot get digest"
	errMkWorkdir        = "cannot create temporary work directory"
	errMvWorkdir        = "cannot move temporary work directory"
	errGetRawConfigFile = "cannot get raw image config file"
	errWriteConfigFile  = "cannot write image config file"
	errGetConfigFile    = "cannot get image config file"
	errOpenConfigFile   = "cannot open image config file"
	errParseConfigFile  = "cannot parse image config file"
	errCloseConfigFile  = "cannot close image config file"
)

// A Bundler prepares OCI runtime bundles for use by an OCI runtime.
type Bundler interface {
	// Bundle returns an OCI bundle ready for use by an OCI runtime.
	Bundle(ctx context.Context, i ociv1.Image, id string) (Bundle, error)
}

// A Bundle for use by an OCI runtime.
type Bundle interface {
	// Path of the OCI bundle.
	Path() string

	// Cleanup the OCI bundle after the container has finished running.
	Cleanup() error
}

// RootFSPath returns the path to the supplied bundle's rootfs.
func RootFSPath(b Bundle) string {
	return filepath.Join(b.Path(), DirRootFS)
}

// SpecPath returns the path to the supplied Bundle's OCI runtime spec.
func SpecPath(b Bundle) string {
	return filepath.Join(b.Path(), FileSpec)
}

// A CachingImageConfigReader reads an image's ConfigFile. The file is cached
// upon first read, and read from cache on subsequent calls.
type CachingImageConfigReader struct {
	root string
}

// NewCachingImageConfigReader returns an ImageConfigReader that caches config
// files upon first read, and reads them from cache on subsequent calls.
func NewCachingImageConfigReader(root string) (*CachingImageConfigReader, error) {
	return &CachingImageConfigReader{root: root}, os.MkdirAll(root, 0700)
}

// ReadConfigFile of the supplied OCI image.
func (c *CachingImageConfigReader) ReadConfigFile(i ociv1.Image) (*ociv1.ConfigFile, error) {
	d, err := i.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errGetDigest)
	}

	// Note ferr, not err, to avoid shadowing in the ErrNotExist block.
	f, ferr := os.Open(filepath.Join(c.root, d.Hex, FileConfig))
	if errors.Is(ferr, os.ErrNotExist) {
		tmp, err := os.MkdirTemp(c.root, fmt.Sprintf("xfn-wrk-%q-", d.Hex))
		if err != nil {
			return nil, errors.Wrap(err, errMkWorkdir)
		}
		defer os.RemoveAll(tmp) //nolint:errcheck // Not much we can do if this fails.

		raw, err := i.RawConfigFile()
		if err != nil {
			return nil, errors.Wrap(err, errGetRawConfigFile)
		}

		if err := os.WriteFile(filepath.Join(tmp, FileConfig), raw, 0600); err != nil {
			return nil, errors.Wrap(err, errWriteConfigFile)
		}

		if err := os.Rename(tmp, filepath.Join(c.root, d.Hex)); err != nil {
			return nil, errors.Wrap(err, errMvWorkdir)
		}

		// It would be slightly cheaper to avoid reading the file when we have
		// it cached, but this provides a little validation that we actually
		// cached something we'll be able to read next time.
		f, ferr = os.Open(filepath.Join(c.root, d.Hex, FileConfig))
	}
	if ferr != nil {
		return nil, errors.Wrap(err, errOpenConfigFile)
	}

	cfg, err := ociv1.ParseConfigFile(f)
	if err != nil {
		_ = f.Close()
		return nil, errors.Wrap(err, errParseConfigFile)
	}
	return cfg, errors.Wrap(f.Close(), errCloseConfigFile)
}
