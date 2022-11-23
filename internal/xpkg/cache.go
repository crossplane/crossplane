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
	"compress/gzip"
	"io"
	"os"
	"sync"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errGetNopCache = "cannot get content from a NopCache"
)

const cacheContentExt = ".gz"

// A PackageCache caches package content.
type PackageCache interface {
	Has(id string) bool
	Get(id string) (io.ReadCloser, error)
	Store(id string, content io.ReadCloser) error
	Delete(id string) error
}

// FsPackageCache stores and retrieves package content in a filesystem-backed
// cache in a thread-safe manner.
type FsPackageCache struct {
	dir string
	fs  afero.Fs
	mu  sync.RWMutex
}

// NewFsPackageCache creates a new FsPackageCache.
func NewFsPackageCache(dir string, fs afero.Fs) *FsPackageCache {
	return &FsPackageCache{
		dir: dir,
		fs:  fs,
	}
}

// Has indicates whether an item with the given id is in the cache.
func (c *FsPackageCache) Has(id string) bool {
	if fi, err := c.fs.Stat(BuildPath(c.dir, id, cacheContentExt)); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

// Get retrieves package contents from the cache.
func (c *FsPackageCache) Get(id string) (io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	f, err := c.fs.Open(BuildPath(c.dir, id, cacheContentExt))
	if err != nil {
		return nil, err
	}
	return GzipReadCloser(f)
}

// Store saves the package contents to the cache.
func (c *FsPackageCache) Store(id string, content io.ReadCloser) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cf, err := c.fs.Create(BuildPath(c.dir, id, cacheContentExt))
	if err != nil {
		return err
	}
	defer cf.Close() //nolint:errcheck // Error is checked in the happy path.
	w, err := gzip.NewWriterLevel(cf, gzip.BestSpeed)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, content)
	if err != nil {
		return err
	}
	// NOTE(hasheddan): gzip writer must be closed to ensure all data is flushed
	// to file.
	if err := w.Close(); err != nil {
		return err
	}
	return cf.Close()
}

// Delete removes package contents from the cache.
func (c *FsPackageCache) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.fs.Remove(BuildPath(c.dir, id, cacheContentExt))
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

// Has indicates whether content is in the NopCache.
func (c *NopCache) Has(string) bool {
	return false
}

// Get retrieves content from the NopCache.
func (c *NopCache) Get(string) (io.ReadCloser, error) {
	return nil, errors.New(errGetNopCache)
}

// Store saves content to the NopCache.
func (c *NopCache) Store(string, io.ReadCloser) error {
	return nil
}

// Delete removes content from the NopCache.
func (c *NopCache) Delete(string) error {
	return nil
}
