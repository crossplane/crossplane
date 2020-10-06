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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
	"github.com/spf13/afero/tarfs"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/pkg/xpkg"
)

const (
	errBadReference      = "package tag is not a valid reference"
	errFetchPackage      = "failed to fetch package from remote"
	errCachePackage      = "failed to store package in cache"
	errOpenPackageStream = "failed to open package stream file"
)

// ImageBackend is a backend for parser.
type ImageBackend struct {
	pkg     string
	id      string
	cache   xpkg.Cache
	fetcher xpkg.Fetcher
	secrets []string
}

// NewImageBackend creates a new image backend.
func NewImageBackend(cache xpkg.Cache, fetcher xpkg.Fetcher) *ImageBackend {
	return &ImageBackend{
		cache:   cache,
		fetcher: fetcher,
	}
}

// Init initializes an ImageBackend.
func (i *ImageBackend) Init(ctx context.Context, bo ...parser.BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(i)
	}

	ref, err := name.ParseReference(i.pkg)
	if err != nil {
		return nil, errors.Wrap(err, errBadReference)
	}

	var img v1.Image
	// Attempt to fetch image from cache.
	img, err = i.cache.Get(i.pkg, i.id)
	if err != nil {
		img, err = i.fetcher.Fetch(ctx, ref, i.secrets)
		if err != nil {
			return nil, errors.Wrap(err, errFetchPackage)
		}
		// Cache image.
		if err := i.cache.Store(i.pkg, i.id, img); err != nil {
			return nil, errors.Wrap(err, errCachePackage)
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

// Package sets the name of the package image for ImageBackend.
func Package(name string) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.pkg = name
	}
}

// Identifier sets the name that will be used to cache the image in
// ImageBackend.
func Identifier(id string) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.id = id
	}
}

// Secrets sets the secrets that will be used to fetch the package image
// from a registry.
func Secrets(s []corev1.LocalObjectReference) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.secrets = v1alpha1.RefNames(s)
	}
}

// Cache sets the cache for the ImageBackend.
func Cache(cache xpkg.Cache) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.cache = cache
	}
}
