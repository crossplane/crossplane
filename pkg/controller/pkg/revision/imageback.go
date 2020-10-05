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

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/afero/tarfs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/pkg/xpkg"
)

// ImageBackend is a backend for parser.
type ImageBackend struct {
	pkg         string
	id          string
	cache       xpkg.Cache
	client      kubernetes.Interface
	pullSecrets []string
	namespace   string
}

// NewImageBackend creates a new image backend.
func NewImageBackend(bo ...parser.BackendOption) *ImageBackend {
	i := &ImageBackend{
		cache: xpkg.NewNopCache(),
	}
	for _, o := range bo {
		o(i)
	}
	return i
}

// Init initializes an ImageBackend.
func (i *ImageBackend) Init(ctx context.Context, bo ...parser.BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(i)
	}

	ref, err := name.ParseReference(i.pkg)
	if err != nil {
		return nil, err
	}

	var img v1.Image

	// Attempt to fetch image from cache.
	img, err = i.cache.Get(i.pkg, i.id)
	if err != nil {
		// Image is not cached, acquire it from registry
		auth, err := k8schain.New(ctx, i.client, k8schain.Options{
			Namespace:        i.namespace,
			ImagePullSecrets: i.pullSecrets,
		})
		if err != nil {
			return nil, err
		}
		img, err = remote.Image(ref, remote.WithAuthFromKeychain(auth))
		if err != nil {
			return nil, err
		}
		// Cache image.
		if err := i.cache.Store(i.pkg, i.id, img); err != nil {
			return nil, err
		}
	}

	// Extract package contents from image.
	r := mutate.Extract(img)
	fs := tarfs.New(tar.NewReader(r))
	f, err := fs.Open(xpkg.StreamFile)
	if err != nil {
		return nil, err
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

// Namespace sets the namespace where any image pull secrets will exist for
// ImageBackend.
func Namespace(ns string) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.namespace = ns
	}
}

// PullSecrets sets the secrets that will be used to fetch the package image
// from a registry.
func PullSecrets(s []corev1.LocalObjectReference) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.pullSecrets = v1alpha1.RefNames(s)
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

// Client sets the Kubernetes client for ImageBackend.
func Client(client kubernetes.Interface) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*ImageBackend)
		if !ok {
			return
		}
		i.client = client
	}
}
