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

// Package overlay implements an overlay based container store.
package overlay

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ociv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/internal/oci/layer"
	"github.com/crossplane/crossplane/internal/oci/spec"
	"github.com/crossplane/crossplane/internal/oci/store"
)

// Error strings
const (
	errMkContainerStore  = "cannot make container store directory"
	errMkLayerStore      = "cannot make layer store directory"
	errReadConfigFile    = "cannot read image config file"
	errGetLayers         = "cannot get image layers"
	errResolveLayer      = "cannot resolve layer to suitable overlayfs lower directory"
	errCreateRootFS      = "cannot create OCI rootfs"
	errCreateRuntimeSpec = "cannot create OCI runtime spec"
	errGetDigest         = "cannot get digest"
	errMkAlgoDir         = "cannot create store directory"
	errFetchLayer        = "cannot fetch and decompress layer"
	errMkWorkdir         = "cannot create temporary work directory"
	errApplyLayer        = "cannot apply (extract) uncompressed tarball layer"
	errMvWorkdir         = "cannot move temporary work directory"
	errStatLayer         = "cannot determine whether layer exists in store"
	errCleanupWorkdir    = "cannot cleanup temporary work directory"
	errMkOverlayDirTmpfs = "cannot make overlay tmpfs dir"
)

// Common overlayfs directories.
const (
	overlayDirTmpfs  = "tmpfs"
	overlayDirUpper  = "upper"
	overlayDirWork   = "work"
	overlayDirLower  = "lower"  // Only used when there are no parent layers.
	overlayDirMerged = "merged" // Only used when generating diff layers.
)

// Supported returns true if the supplied cacheRoot supports the overlay
// filesystem. Notably overlayfs was not supported in unprivileged user
// namespaces until Linux kernel 5.11. It's also not possible to create an
// overlayfs where the upper dir is itself on an overlayfs (i.e. is on a
// container's root filesystem).
// https://github.com/torvalds/linux/commit/459c7c565ac36ba09ffbf
func Supported(cacheRoot string) bool {
	// We use NewLayerWorkdir to test because it needs to create an upper dir on
	// the same filesystem as the supplied cacheRoot in order to be able to move
	// it into place as a cached layer. NewOverlayBundle creates an upper dir on
	// a tmpfs, and is thus supported in some cases where NewLayerWorkdir isn't.
	w, err := NewLayerWorkdir(cacheRoot, "supports-overlay-test", []string{})
	if err != nil {
		return false
	}
	if err := w.Cleanup(); err != nil {
		return false
	}
	return true
}

// An LayerResolver resolves the supplied layer to a path suitable for use as an
// overlayfs lower directory.
type LayerResolver interface {
	// Resolve the supplied layer to a path suitable for use as a lower dir.
	Resolve(ctx context.Context, l ociv1.Layer, parents ...ociv1.Layer) (string, error)
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

// An CachingBundler stores OCI containers, images, and layers. When asked to
// bundle a container for a new image the CachingBundler will extract and cache
// the image's layers as files on disk. The container's root filesystem is then
// created as an overlay atop the image's layers. The upper layer of this
// overlay is stored in memory on a tmpfs, and discarded once the container has
// finished running.
type CachingBundler struct {
	root  string
	layer LayerResolver
	spec  RuntimeSpecCreator
}

// NewCachingBundler returns a bundler that creates container filesystems as
// overlays on their image's layers, which are stored as extracted, overlay
// compatible directories of files.
func NewCachingBundler(root string) (*CachingBundler, error) {
	l, err := NewCachingLayerResolver(filepath.Join(root, store.DirOverlays))
	if err != nil {
		return nil, errors.Wrap(err, errMkLayerStore)
	}

	s := &CachingBundler{
		root:  filepath.Join(root, store.DirContainers),
		layer: l,
		spec:  RuntimeSpecCreatorFn(spec.Create),
	}
	return s, nil
}

// Bundle returns an OCI bundle ready for use by an OCI runtime. The supplied
// image will be fetched and cached in the store if it does not already exist.
func (c *CachingBundler) Bundle(ctx context.Context, i ociv1.Image, id string) (store.Bundle, error) {
	cfg, err := i.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, errReadConfigFile)
	}

	layers, err := i.Layers()
	if err != nil {
		return nil, errors.Wrap(err, errGetLayers)
	}

	lowerPaths := make([]string, len(layers))
	for i := range layers {
		p, err := c.layer.Resolve(ctx, layers[i], layers[:i]...)
		if err != nil {
			return nil, errors.Wrap(err, errResolveLayer)
		}
		lowerPaths[i] = p
	}

	// TODO(negz): Ideally this would be mockable. It's not really creating
	// _all_ of the bundle; just its rootfs. We could perhaps refactor it to
	// c.rootfs.Create(b, lowerPaths), but we'd need to register the mounts to
	// be unmounted when the bundle was cleaned up.
	b, err := CreateBundle(filepath.Join(c.root, id), lowerPaths)
	if err != nil {
		return nil, errors.Wrap(err, errCreateRootFS)
	}

	// Create an OCI runtime config file from our cached OCI image config file.
	// We do this every time we run the function because in future it's likely
	// that we'll want to derive the OCI runtime config file from both the OCI
	// image config file and user supplied input (i.e. from the functions array
	// of a Composition).
	if err := c.spec.Create(b, cfg); err != nil {
		_ = b.Cleanup()
		return nil, errors.Wrap(err, errCreateRuntimeSpec)
	}

	return b, nil
}

// A CachingLayerResolver resolves an OCI layer to an overlay compatible
// directory on disk. The directory is created the first time a layer is
// resolved; subsequent calls return the cached directory.
type CachingLayerResolver struct {
	root    string
	tarball TarballApplicator
}

// NewCachingLayerResolver returns a LayerResolver that extracts layers upon
// first resolution, returning cached layer paths on subsequent calls.
func NewCachingLayerResolver(root string) (*CachingLayerResolver, error) {
	c := &CachingLayerResolver{
		root:    root,
		tarball: layer.NewStackingExtractor(layer.NewWhiteoutHandler(layer.NewExtractHandler())),
	}
	return c, os.MkdirAll(root, 0700)
}

// Resolve the supplied layer to a path suitable for use as an overlayfs lower
// layer directory. The first time a layer is resolved it will be extracted and
// cached as an overlayfs compatible directory of whiles, with any OCI whiteouts
// converted to overlayfs whiteouts.
func (s *CachingLayerResolver) Resolve(ctx context.Context, l ociv1.Layer, parents ...ociv1.Layer) (string, error) {
	d, err := l.DiffID() // The uncompressed layer digest.
	if err != nil {
		return "", errors.Wrap(err, errGetDigest)
	}

	path := filepath.Join(s.root, d.Algorithm, d.Hex)
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		// Doesn't exist - cache it. It's possible multiple callers may hit this
		// branch at once. This will result in multiple extractions to different
		// temporary dirs. We ignore EEXIST errors from os.Rename, so callers
		// that lose the race should return the path cached by the successful
		// caller.

		// This call to Uncompressed is what actually pulls a remote layer. In
		// most cases we'll be using an image backed by our local image store.
		tarball, err := l.Uncompressed()
		if err != nil {
			return "", errors.Wrap(err, errFetchLayer)
		}

		parentPaths := make([]string, len(parents))
		for i := range parents {
			d, err := parents[i].DiffID()
			if err != nil {
				return "", errors.Wrap(err, errGetDigest)
			}
			parentPaths[i] = filepath.Join(s.root, d.Algorithm, d.Hex)
		}

		lw, err := NewLayerWorkdir(filepath.Join(s.root, d.Algorithm), d.Hex, parentPaths)
		if err != nil {
			return "", errors.Wrap(err, errMkWorkdir)
		}

		if err := s.tarball.Apply(ctx, tarball, lw.ApplyPath()); err != nil {
			_ = lw.Cleanup()
			return "", errors.Wrap(err, errApplyLayer)
		}

		// If newpath exists now (when it didn't above) we must have lost a race
		// with another caller to cache this layer.
		if err := os.Rename(lw.ResultPath(), path); resource.Ignore(os.IsExist, err) != nil {
			_ = lw.Cleanup()
			return "", errors.Wrap(err, errMvWorkdir)
		}

		return path, errors.Wrap(lw.Cleanup(), errCleanupWorkdir)
	}
	return path, errors.Wrap(err, errStatLayer)
}

// An Bundle is an OCI runtime bundle. Its root filesystem is a temporary
// overlay atop its image's cached layers.
type Bundle struct {
	path   string
	mounts []Mount
}

// CreateBundle creates and returns an OCI runtime bundle with a root
// filesystem backed by a temporary (tmpfs) overlay atop the supplied lower
// layer paths.
func CreateBundle(path string, parentLayerPaths []string) (Bundle, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return Bundle{}, errors.Wrap(err, "cannot create bundle dir")
	}

	if err := os.Mkdir(filepath.Join(path, overlayDirTmpfs), 0700); err != nil {
		_ = os.RemoveAll(path)
		return Bundle{}, errors.Wrap(err, errMkOverlayDirTmpfs)
	}

	tm := TmpFSMount{Mountpoint: filepath.Join(path, overlayDirTmpfs)}
	if err := tm.Mount(); err != nil {
		_ = os.RemoveAll(path)
		return Bundle{}, errors.Wrap(err, "cannot mount workdir tmpfs")
	}

	for _, p := range []string{
		filepath.Join(path, overlayDirTmpfs, overlayDirUpper),
		filepath.Join(path, overlayDirTmpfs, overlayDirWork),
		filepath.Join(path, store.DirRootFS),
	} {
		if err := os.Mkdir(p, 0700); err != nil {
			_ = os.RemoveAll(path)
			return Bundle{}, errors.Wrapf(err, "cannot create %s dir", p)
		}
	}

	om := OverlayMount{
		Lower:      parentLayerPaths,
		Upper:      filepath.Join(path, overlayDirTmpfs, overlayDirUpper),
		Work:       filepath.Join(path, overlayDirTmpfs, overlayDirWork),
		Mountpoint: filepath.Join(path, store.DirRootFS),
	}
	if err := om.Mount(); err != nil {
		_ = os.RemoveAll(path)
		return Bundle{}, errors.Wrap(err, "cannot mount workdir overlayfs")
	}

	return Bundle{path: path, mounts: []Mount{om, tm}}, nil
}

// Path to the OCI bundle.
func (b Bundle) Path() string { return b.path }

// Cleanup the OCI bundle.
func (b Bundle) Cleanup() error {
	for _, m := range b.mounts {
		if err := m.Unmount(); err != nil {
			return errors.Wrap(err, "cannot unmount bundle filesystem")
		}
	}
	return errors.Wrap(os.RemoveAll(b.path), "cannot remove bundle")
}

// A Mount of a filesystem.
type Mount interface {
	Mount() error
	Unmount() error
}

// A TmpFSMount represents a mount of type tmpfs.
type TmpFSMount struct {
	Mountpoint string
}

// An OverlayMount represents a mount of type overlay.
type OverlayMount struct { //nolint:revive // overlay.OverlayMount makes sense given that overlay.TmpFSMount exists too.
	Mountpoint string
	Lower      []string
	Upper      string
	Work       string
}

// A LayerWorkdir is a temporary directory used to produce an overlayfs layer
// from an OCI layer by applying the OCI layer to a temporary overlay mount.
// It's not possible to _directly_ create overlay whiteout files in an
// unprivileged user namespace because doing so requires CAP_MKNOD in the 'root'
// or 'initial' user namespace - whiteout files are actually character devices
// per "whiteouts and opaque directories" at
// https://www.kernel.org/doc/Documentation/filesystems/overlayfs.txt
//
// We can however create overlay whiteout files indirectly by creating an
// overlay where the parent OCI layers are the lower overlayfs layers, and
// applying the layer to be cached to said fs. Doing so will produce an upper
// overlayfs layer that we can cache. This layer will be a valid lower layer
// (complete with overlay whiteout files) for either subsequent layers from the
// OCI image, or the final container root filesystem layer.
type LayerWorkdir struct {
	OverlayMount

	path string
}

// NewLayerWorkdir returns a temporary directory used to produce an overlayfs
// layer from an OCI layer.
func NewLayerWorkdir(dir, digest string, parentLayerPaths []string) (LayerWorkdir, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return LayerWorkdir{}, errors.Wrap(err, "cannot create temp dir")
	}
	tmp, err := os.MkdirTemp(dir, fmt.Sprintf("%s-", digest))
	if err != nil {
		return LayerWorkdir{}, errors.Wrap(err, "cannot create temp dir")
	}

	for _, d := range []string{overlayDirMerged, overlayDirUpper, overlayDirLower, overlayDirWork} {
		if err := os.Mkdir(filepath.Join(tmp, d), 0700); err != nil {
			_ = os.RemoveAll(tmp)
			return LayerWorkdir{}, errors.Wrapf(err, "cannot create %s dir", d)
		}
	}

	w := LayerWorkdir{
		OverlayMount: OverlayMount{
			Lower:      []string{filepath.Join(tmp, overlayDirLower)},
			Upper:      filepath.Join(tmp, overlayDirUpper),
			Work:       filepath.Join(tmp, overlayDirWork),
			Mountpoint: filepath.Join(tmp, overlayDirMerged),
		},
		path: tmp,
	}

	if len(parentLayerPaths) != 0 {
		w.Lower = parentLayerPaths
	}

	if err := w.Mount(); err != nil {
		_ = os.RemoveAll(tmp)
		return LayerWorkdir{}, errors.Wrap(err, "cannot mount workdir overlayfs")
	}

	return w, nil
}

// ApplyPath returns the path an OCI layer should be applied (i.e. extracted) to
// in order to create an overlayfs layer.
func (d LayerWorkdir) ApplyPath() string {
	return filepath.Join(d.path, overlayDirMerged)
}

// ResultPath returns the path of the resulting overlayfs layer.
func (d LayerWorkdir) ResultPath() string {
	return filepath.Join(d.path, overlayDirUpper)
}

// Cleanup the temporary directory.
func (d LayerWorkdir) Cleanup() error {
	if err := d.Unmount(); err != nil {
		return errors.Wrap(err, "cannot unmount workdir overlayfs")
	}
	return errors.Wrap(os.RemoveAll(d.path), "cannot remove workdir")
}
