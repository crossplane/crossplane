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

package xfn

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errAdvanceTarball   = "cannot advance to next entry in tarball"
	errCloseFsTarball   = "cannot close function filesystem tarball"
	errFetchFnOCIConfig = "cannot fetch function OCI config"
	errUntarFn          = "cannot unarchive function tarball"
	errMkdir            = "cannot make directory"
	errOpenFile         = "cannot open file"
	errCopyFile         = "cannot copy file"
	errCloseFile        = "cannot close file"
	errCreateFile       = "cannot create file"
	errWriteFile        = "cannot write file"
	errMakeTmpDir       = "cannot make temporary directory"
	errParseImageConfig = "cannot parse OCI image config"
	errNewRuntimeConfig = "cannot create new OCI runtime config"
	errRemoveBundle     = "cannot remove OCI bundle from store"
	errChown            = "cannot chown path"
	errFindImage        = "cannot find OCI image in cache"
	errDirExists        = "cannot determine whether dir exists"

	errFmtSize            = "wrote %d bytes to %q; expected %d"
	errFmtInvalidPath     = "tarball contains invalid file path %q"
	errFmtUnsupportedMode = "tarball contained file %q with unknown file type: %q"
	errFmtRenameTmpDir    = "cannot move temporary directory %q to %q"
	errFmtRunExists       = "bundle for run ID %q already exists"
)

// Store paths.
const (
	cache  = "cache"
	bundle = "bundle"
	config = "config.json"
	rootfs = "rootfs"
)

type errNotCached struct{ error }

// IsNotCached indicates an OCI image was not cached in a store.
func IsNotCached(err error) bool {
	return errors.As(err, &errNotCached{})
}

// An OCIStore stores OCI runtime data.
type OCIStore interface {
	// Cache an image in the OCI store.
	Cache(ctx context.Context, imgID string, img ociv1.Image) error

	// Bundle an image from the OCI store for a runtime.
	Bundle(ctx context.Context, imgID, runID string) (Bundle, error)
}

// A ForgetfulStore does nothing. It discards requests to cache images. Bundle
// always returns an error that satisfies IsNotCached.
type ForgetfulStore struct{}

// Cache discards the supplied image.
func (s *ForgetfulStore) Cache(ctx context.Context, imgID string, img ociv1.Image) error {
	return nil
}

// Bundle always returns an error that satisfies IsNotCached.
func (s *ForgetfulStore) Bundle(ctx context.Context, imgID, runID string) (Bundle, error) {
	return Bundle{}, errNotCached{errors.New(errFindImage)}
}

// A FilesystemStore of extracted OCI images - config files and flattened filesystems.
type FilesystemStore struct {
	root string
	fs   afero.Afero
}

// An FilesystemStoreOption configures a new Store.
type FilesystemStoreOption func(*FilesystemStore)

// WithFilesystem configures the filesystem a store should use.
func WithFilesystem(fs afero.Fs) FilesystemStoreOption {
	return func(s *FilesystemStore) {
		s.fs = afero.Afero{Fs: fs}
	}
}

// NewFilesystemStore returns a new store ready for use. The store is backed by
// the OS filesystem by default.
func NewFilesystemStore(root string, o ...FilesystemStoreOption) *FilesystemStore {
	s := &FilesystemStore{root: root, fs: afero.Afero{Fs: afero.NewOsFs()}}
	for _, fn := range o {
		fn(s)
	}
	return s
}

// Cache the OCI image config file and flattened 'rootfs' filesystem of the
// supplied OCI image to the store. Cache simulates a transaction in that it
// will attempt to unwind a partial write if an error occurs. The resulting
// entry has the following layout:
//
//   /store/image-id/cache/config.json - OCI image config.
//   /store/image-id/cache/rootfs/     - Flattened root filesystem.
//
// Consumers must not modify the cached rootfs.
func (s *FilesystemStore) Cache(ctx context.Context, imgID string, img ociv1.Image) error {
	imgPath := filepath.Join(s.root, imgID)
	if err := s.fs.MkdirAll(imgPath, 0750); err != nil {
		return errors.Wrap(err, errMkdir)
	}

	// We unarchive to a temporary directory first, then move it to its
	// 'real' path only if the unarchive worked. This lets us simulate an
	// 'atomic' unarchive that we can unwind if it fails.
	tmp, err := s.fs.TempDir(imgPath, cache)
	if err != nil {
		return errors.Wrap(err, errMakeTmpDir)
	}

	// RemoveAll doesn't return an error if the supplied directory doesn't
	// exist, so this should be a no-op if we successfully called Rename to
	// move our tmp directory to its 'real' path.
	defer s.fs.RemoveAll(tmp) //nolint:errcheck // There's not much we can do if this fails.

	flattened := mutate.Extract(img)
	if err := untar(ctx, tar.NewReader(flattened), s.fs, filepath.Join(tmp, rootfs)); err != nil {
		_ = flattened.Close()
		return errors.Wrap(err, errUntarFn)
	}
	if err := flattened.Close(); err != nil {
		return errors.Wrap(err, errCloseFsTarball)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return errors.Wrap(err, errFetchFnOCIConfig)
	}

	cfgPath := filepath.Join(tmp, config)
	f, err := s.fs.Create(cfgPath)
	if err != nil {
		return errors.Wrap(err, errCreateFile)
	}

	if err := json.NewEncoder(f).Encode(cfg); err != nil {
		_ = f.Close()
		return errors.Wrap(err, errWriteFile)
	}

	if err := f.Close(); err != nil {
		return errors.Wrap(err, errCloseFile)
	}

	// We successfully wrote our root filesystem and config file. Time to move
	// our temporary working directory to the 'real' cache location.
	cachePath := filepath.Join(imgPath, cache)
	return errors.Wrapf(s.fs.Rename(tmp, cachePath), errFmtRenameTmpDir, tmp, cachePath)
}

// Bundle prepares and returns a new OCI runtime bundle for a single run of the
// supplied image ID. The bundle has the following layout:
//
//  /store/image-id/bundle/run-id/config.json - OCI runtime config.
//  /store/image-id/bundle/run-id/rootfs/     - An empty directory.
//
// The returned bundle includes a reference to a cached rootfs that may be used
// to seed the bundle's rootfs directory, e.g. by creating an overlay filesystem
// or simply making a copy of the cached rootfs.
func (s *FilesystemStore) Bundle(_ context.Context, imgID, runID string) (Bundle, error) { //nolint:gocyclo
	// NOTE(negz): This function is a little over our complexity limit but I
	// can't immediately see a way to break it up that is equally readable.

	// Fail early if there's no cache from which to build this Bundle.
	cachePath := filepath.Join(s.root, imgID, cache)
	icf, err := s.fs.Open(filepath.Join(cachePath, config))
	if errors.Is(err, fs.ErrNotExist) {
		return Bundle{}, errNotCached{err}
	}
	if err != nil {
		return Bundle{}, errors.Wrap(err, errOpenFile)
	}
	defer icf.Close() //nolint:errcheck // File was only open for reading.

	bundlePath := filepath.Join(s.root, imgID, bundle)
	if err := s.fs.MkdirAll(bundlePath, 0750); err != nil {
		return Bundle{}, errors.Wrap(err, errMkdir)
	}

	runPath := filepath.Join(bundlePath, runID)
	exists, err := s.fs.DirExists(runPath)
	if err != nil {
		return Bundle{}, errors.Wrap(err, errDirExists)
	}
	if exists {
		return Bundle{}, errors.Errorf(errFmtRunExists, runID)
	}

	tmp, err := s.fs.TempDir(bundlePath, runID)
	if err != nil {
		return Bundle{}, errors.Wrap(err, errMakeTmpDir)
	}

	// RemoveAll doesn't return an error if the supplied directory doesn't
	// exist, so this should be a no-op if we successfully called Rename to
	// move our tmp directory to its 'real' path.
	defer s.fs.RemoveAll(tmp) //nolint:errcheck // There's not much we can do if this fails.

	// Create an OCI runtime config file from our cached OCI image config file.
	// We do this every time we run the function because in future it's likely
	// that we'll want to derive the OCI runtime config file from both the OCI
	// image config file and user supplied input (i.e. from the functions array
	// of a Composition).
	icfg, err := ociv1.ParseConfigFile(icf)
	if err != nil {
		return Bundle{}, errors.Wrap(err, errParseImageConfig)
	}
	rcfg, err := NewRuntimeConfig(icfg)
	if err != nil {
		return Bundle{}, errors.Wrap(err, errNewRuntimeConfig)
	}

	cfgPath := filepath.Join(tmp, config)
	rcf, err := s.fs.Create(cfgPath)
	if err != nil {
		return Bundle{}, errors.Wrap(err, errCreateFile)
	}

	if err := json.NewEncoder(rcf).Encode(rcfg); err != nil {
		_ = rcf.Close()
		return Bundle{}, errors.Wrap(err, errWriteFile)
	}

	if err := rcf.Close(); err != nil {
		return Bundle{}, errors.Wrap(err, errCloseFile)
	}

	if err := s.fs.Mkdir(filepath.Join(tmp, rootfs), 0755); err != nil {
		return Bundle{}, errors.Wrap(err, errMkdir)
	}

	// We successfully wrote our OCI runtime config file. Time to move our
	// temporary working directory to the 'real' bundle location.
	b := Bundle{Path: runPath, CachedRootFS: filepath.Join(cachePath, rootfs), fs: s.fs}
	return b, errors.Wrapf(s.fs.Rename(tmp, runPath), errFmtRenameTmpDir, tmp, runPath)
}

// A Bundle for use by an OCI runtime.
type Bundle struct {
	// Path to the Bundle within the filesystem that backs the store.
	Path string

	// Path to a cached rootfs that may be used to populate the bundle's rootfs.
	// The caller must not modify this cache.
	CachedRootFS string

	fs afero.Fs
}

// Delete the Bundle from the store.
func (b Bundle) Delete() error {
	// We shouldn't have to cleanup any mounts inside this bundle. They should
	// be unmounted automatically when the mount namespace they were in no
	// longer exists (i.e. due to no processes remaining inside it).
	return errors.Wrap(b.fs.RemoveAll(b.Path), errRemoveBundle)
}

// untar an uncompressed tarball to dir in the supplied filesystem.
// Adapted from https://github.com/golang/build/blob/5aee8e/internal/untar/untar.go
func untar(ctx context.Context, tb io.Reader, fs afero.Fs, dir string) error { //nolint:gocyclo
	// NOTE(negz): This function is a little over our gocyclo target. I can't
	// see an immediate way to simplify/break it up that would be equally easy
	// to read.

	tr := tar.NewReader(tb)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return errors.Wrap(err, errAdvanceTarball)
		}
		if !validPath(hdr.Name) {
			return errors.Errorf(errFmtInvalidPath, hdr.Name)
		}

		path := filepath.Join(dir, filepath.Clean(filepath.FromSlash(hdr.Name)))
		mode := hdr.FileInfo().Mode()

		switch {
		case mode.IsDir():
			// TODO(negz): Will this potentially create parent directories with
			// the child's permissions if we encounter a parent before a child?
			if err := fs.MkdirAll(path, mode.Perm()); err != nil {
				return errors.Wrap(err, errMkdir)
			}
		case mode.IsRegular():
			if err := fs.MkdirAll(filepath.Dir(path), mode.Perm()); err != nil {
				return errors.Wrap(err, errMkdir)
			}

			dst, err := fs.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return errors.Wrap(err, errOpenFile)
			}
			n, err := copyChunks(dst, tr, 1024*1024) // Copy in 1MB chunks.
			if err != nil {
				_ = dst.Close()
				return errors.Wrap(err, errCopyFile)
			}
			if err := dst.Close(); err != nil {
				return errors.Wrap(err, errCloseFile)
			}
			if n != hdr.Size {
				return errors.Errorf(errFmtSize, n, path, hdr.Size)
			}
		default:
			return errors.Errorf(errFmtUnsupportedMode, hdr.Name, mode)
		}

		if err := fs.Chown(path, hdr.Uid, hdr.Gid); err != nil {
			return errors.Wrap(err, errChown)
		}

	}
}

func validPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.Contains(p, "../") {
		return false
	}
	return true
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
