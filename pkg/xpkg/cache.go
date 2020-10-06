/*
Copyright 2020 The Crossplane Authors.

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

package xpkg

import (
	"os"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (
	errGetNopCache = "cannot get an image from a NopCache"
)

// A Cache caches OCI images.
type Cache interface {
	Get(tag string, id string) (v1.Image, error)
	Store(tag string, id string, img v1.Image) error
	Delete(id string) error
}

// ImageCache stores and retrieves OCI images in a filesystem-backed cache in a
// thread-safe manner.
type ImageCache struct {
	dir string
	fs  afero.Fs
	mu  sync.RWMutex
}

// NewImageCache creates a new ImageCache.
func NewImageCache(dir string, fs afero.Fs) *ImageCache {
	return &ImageCache{
		dir: dir,
		fs:  fs,
	}
}

// Get retrieves an image from the ImageCache.
func (c *ImageCache) Get(tag, id string) (v1.Image, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var t *name.Tag
	if tag != "" {
		nt, err := name.NewTag(tag)
		if err != nil {
			return nil, err
		}
		t = &nt
	}
	return tarball.ImageFromPath(BuildPath(c.dir, id), t)
}

// Store saves an image to the ImageCache.
func (c *ImageCache) Store(tag, id string, img v1.Image) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ref, err := name.ParseReference(tag)
	if err != nil {
		return err
	}
	cf, err := c.fs.Create(BuildPath(c.dir, id))
	if err != nil {
		return err
	}
	return tarball.Write(ref, img, cf)
}

// Delete removes an image from the ImageCache.
func (c *ImageCache) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.fs.Remove(BuildPath(c.dir, id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// NopCache is a cache implementation that does not store anything and always
// returns an error on get.
type NopCache struct{}

// NewNopCache creates a new NopCache.
func NewNopCache() *NopCache {
	return &NopCache{}
}

// Get retrieves an image from the NopCache.
func (c *NopCache) Get(tag, id string) (v1.Image, error) {
	return nil, errors.New(errGetNopCache)
}

// Store saves an image to the NopCache.
func (c *NopCache) Store(tag, id string, img v1.Image) error {
	return nil
}

// Delete removes an image from the NopCache.
func (c *NopCache) Delete(id string) error {
	return nil
}
