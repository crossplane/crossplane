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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
	fnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errInvalidInput     = "invalid function input"
	errInvalidOutput    = "invalid function output"
	errBadReference     = "OCI tag is not a valid reference"
	errHeadImg          = "cannot fetch OCI image descriptor"
	errExecFn           = "cannot execute function"
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

	errRemoveBundle = "cannot remove OCI bundle from store"
	errCloseBundle  = "cannot close OCI bundle"

	// TODO(negz): Make these errFmt, with the image string.
	errFetchFn  = "cannot fetch function from registry"
	errLookupFn = "cannot lookup function in store"
	errWriteFn  = "cannot write function to store"

	errFmtSize            = "wrote %d bytes to %q; expected %d"
	errFmtInvalidPath     = "tarball contains invalid file path %q"
	errFmtUnsupportedMode = "tarball contained file %q with unknown file type: %q"
	errFmtRenameTmpDir    = "cannot move temporary directory %q to %q"
)

const (
	// Store paths.
	cache  = "cache"
	bundle = "bundle"
	config = "config.json"
	rootfs = "rootfs"
)

// An OCIRunner runs an XRM function packaged as an OCI image by extracting it
// and running it in a chroot.
type OCIRunner struct {
	image string

	// TODO(negz): Break fetch-ey bits out of xpkg.
	defaultRegistry string
	registry        xpkg.Fetcher
	store           *Store
}

// A OCIRunnerOption configures a new OCIRunner.
type OCIRunnerOption func(*OCIRunner)

// NewOCIRunner returns a new Runner that runs functions packaged as OCI images.
func NewOCIRunner(image string, o ...OCIRunnerOption) *OCIRunner {
	r := &OCIRunner{
		image:           image,
		defaultRegistry: name.DefaultRegistry,
		registry:        xpkg.NewNopFetcher(),
		store:           NewStore("/xfn/store"),
	}
	for _, fn := range o {
		fn(r)
	}
	return r
}

// Run a function packaged as an OCI image. Functions are not run as containers,
// but rather by unarchiving them and executing their entrypoint and/or cmd in a
// chroot with their supplied environment variables set. This allows them to be
// run from inside an existing, unprivileged container. Functions that write to
// stderr, return non-zero, or that cannot be executed in the first place (e.g.
// because they cannot be fetched from the registry) will return an error.
func (r *OCIRunner) Run(ctx context.Context, in *fnv1alpha1.ResourceList) (*fnv1alpha1.ResourceList, error) {
	// Parse the input early, before we potentially pull and write the image.
	y, err := yaml.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, errInvalidInput)
	}
	stdin := bytes.NewReader(y)

	ref, err := name.ParseReference(r.image, name.WithDefaultRegistry(r.defaultRegistry))
	if err != nil {
		return nil, errors.Wrap(err, errBadReference)
	}

	d, err := r.registry.Head(ctx, ref)
	if err != nil {
		return nil, errors.Wrap(err, errHeadImg)
	}

	imgID := d.Digest.Hex
	runID := uuid.NewString()

	// TODO(negz): Wrap store in a variant that automatically fetches and writes
	// missing refs. Presumably we'd need to take the ref name (not hex digest)
	// as an ID.
	b, err := r.store.Bundle(ctx, imgID, runID)
	if IsNotFound(err) {
		// If the function isn't found in the store we fetch it, write it, and
		// try to look it up again.
		img, ferr := r.registry.Fetch(ctx, ref)
		if ferr != nil {
			return nil, errors.Wrap(ferr, errFetchFn)
		}

		if err := r.store.Cache(ctx, d.Digest.Hex, img); err != nil {
			return nil, errors.Wrap(err, errWriteFn)
		}

		// Note we're setting the outer err that satisfied IsNotFound.
		b, err = r.store.Bundle(ctx, imgID, runID)
	}
	if err != nil {
		return nil, errors.Wrap(err, errLookupFn)
	}

	// TODO(negz): Invoke ignition
	cmd := exec.CommandContext(ctx, "")
	cmd.Stdin = stdin

	stdout, err := cmd.Output()
	if err != nil {
		// TODO(negz): Don't swallow stderr if this is an *exec.ExitError?
		return nil, errors.Wrap(err, errExecFn)
	}

	if err := b.Delete(); err != nil {
		return nil, errors.Wrap(err, errCloseBundle)
	}

	out := &v1alpha1.ResourceList{}
	return out, errors.Wrap(yaml.Unmarshal(stdout, out), errInvalidOutput)
}

type errNotFound struct{ error }

// IsNotFound indicates a config file and/or filesystem were not found in the
// store.
func IsNotFound(err error) bool {
	return errors.As(err, &errNotFound{})
}

// We'll get an error that we want to wrap.
// We want to decorate that error to 'be' something else.

// A Store of extracted OCI images - config files and flattened filesystems.
type Store struct {
	root string
	fs   afero.Afero
}

// A StoreOption configures a new Store.
type StoreOption func(*Store)

// WithFS configures the filesystem a store should use.
func WithFS(fs afero.Fs) StoreOption {
	return func(s *Store) {
		s.fs = afero.Afero{Fs: fs}
	}
}

// NewStore returns a new store ready for use. The store is backed by the OS
// filesystem unless the WithFS option is supplied.
func NewStore(root string, o ...StoreOption) *Store {
	s := &Store{root: root, fs: afero.Afero{Fs: afero.NewOsFs()}}
	for _, fn := range o {
		fn(s)
	}
	return s
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

// Bundle prepares and returns a new OCI runtime bundle for the supplied image
// ID. The bundle has the following layout:
//
//  /store/image-id/bundle/run-id/config.json - OCI runtime config.
//  /store/image-id/bundle/run-id/rootfs/     - An empty directory.
//
// The returned bundle includes a reference to a cached rootfs that may be used
// to seed the bundle's rootfs directory, e.g. by creating an overlay filesystem
// or simply making a copy of the cached rootfs.
func (s *Store) Bundle(_ context.Context, imgID, runID string) (Bundle, error) {
	tmp, err := s.fs.TempDir(s.root, imgID)
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
	cachePath := filepath.Join(s.root, imgID, cache)
	icf, err := s.fs.Open(filepath.Join(cachePath, config))
	if err != nil {
		return Bundle{}, errors.Wrap(err, errOpenFile)
	}
	defer icf.Close() //nolint:errcheck // File was only open for reading.

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
	b := Bundle{Path: filepath.Join(s.root, imgID, bundle, runID), CachedRootFS: filepath.Join(cachePath, rootfs), fs: s.fs}
	return b, errors.Wrapf(s.fs.Rename(tmp, b.Path), errFmtRenameTmpDir, tmp, b.Path)
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
func (s *Store) Cache(ctx context.Context, imgID string, img ociv1.Image) error {
	// We unarchive to a temporary directory first, then move it to its
	// 'real' path only if the unarchive worked. This lets us simulate an
	// 'atomic' unarchive that we can unwind if it fails.
	tmp, err := s.fs.TempDir(s.root, imgID)
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
	cachePath := filepath.Join(s.root, imgID, cache)
	return errors.Wrapf(s.fs.Rename(tmp, cachePath), errFmtRenameTmpDir, tmp, cachePath)
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
			if err := fs.MkdirAll(path, 0755); err != nil {
				return errors.Wrap(err, errMkdir)
			}
		case mode.IsRegular():
			if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
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
