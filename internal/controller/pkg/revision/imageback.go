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
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
	"github.com/spf13/afero/tarfs"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errPullPolicyNever   = "failed to get pre-cached package with pull policy Never"
	errBadReference      = "package tag is not a valid reference"
	errFetchPackage      = "failed to fetch package from remote"
	errCachePackage      = "failed to store package in cache"
	errOpenPackageStream = "failed to open package stream file"
)

// ImageBackend is a backend for parser.
type ImageBackend struct {
	pr       v1.PackageRevision
	registry string
	cache    xpkg.Cache
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
func NewImageBackend(cache xpkg.Cache, fetcher xpkg.Fetcher, opts ...ImageBackendOption) *ImageBackend {
	i := &ImageBackend{
		cache:   cache,
		fetcher: fetcher,
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// Init initializes an ImageBackend.
func (i *ImageBackend) Init(ctx context.Context, bo ...parser.BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(i)
	}
	var img regv1.Image
	var err error

	pullPolicy := i.pr.GetPackagePullPolicy()
	if pullPolicy != nil && *pullPolicy == corev1.PullNever {
		// If package is pre-cached we assume there are never multiple tags in
		// the same image.
		img, err = i.cache.Get("", i.pr.GetSource())
		if err != nil {
			return nil, errors.Wrap(err, errPullPolicyNever)
		}
	} else {
		// Ensure source is a valid image reference.
		ref, err := name.ParseReference(i.pr.GetSource(), name.WithDefaultRegistry(i.registry))
		if err != nil {
			return nil, errors.Wrap(err, errBadReference)
		}
		// Attempt to fetch image from cache.
		img, err = i.cache.Get(i.pr.GetSource(), i.pr.GetName())
		if err != nil {
			img, err = i.fetcher.Fetch(ctx, ref, v1.RefNames(i.pr.GetPackagePullSecrets())...)
			if err != nil {
				return nil, errors.Wrap(err, errFetchPackage)
			}
			// Cache image.
			if err := i.cache.Store(i.pr.GetSource(), i.pr.GetName(), img); err != nil {
				return nil, errors.Wrap(err, errCachePackage)
			}
		}
	}

	// Extract package contents from image.
	r := mutate.Extract(img)
	fs := tarfs.New(tar.NewReader(r))
	f, err := fs.Open(xpkg.StreamFile)
	if err != nil {
		return nil, errors.Wrap(err, errOpenPackageStream)
	}
	return f, nil
}

// PackageRevision sets the package revision for ImageBackend.
func PackageRevision(pr v1.PackageRevision) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.pr = pr
	}
}
