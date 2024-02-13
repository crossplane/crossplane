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

// Package fake contains mock Crossplane package implementations.
package fake

import (
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/crossplane/crossplane/internal/xpkg"
)

var _ xpkg.PackageCache = &MockCache{}

// MockCache is a mock Cache.
type MockCache struct {
	MockHas    func() bool
	MockGet    func() (io.ReadCloser, error)
	MockStore  func(s string, rc io.ReadCloser) error
	MockDelete func() error
}

// NewMockCacheHasFn creates a new MockGet function for MockCache.
func NewMockCacheHasFn(has bool) func() bool {
	return func() bool { return has }
}

// NewMockCacheGetFn creates a new MockGet function for MockCache.
func NewMockCacheGetFn(rc io.ReadCloser, err error) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) { return rc, err }
}

// NewMockCacheStoreFn creates a new MockStore function for MockCache.
func NewMockCacheStoreFn(err error) func(s string, rc io.ReadCloser) error {
	return func(_ string, _ io.ReadCloser) error { return err }
}

// NewMockCacheDeleteFn creates a new MockDelete function for MockCache.
func NewMockCacheDeleteFn(err error) func() error {
	return func() error { return err }
}

// Has calls the underlying MockHas.
func (c *MockCache) Has(string) bool {
	return c.MockHas()
}

// Get calls the underlying MockGet.
func (c *MockCache) Get(string) (io.ReadCloser, error) {
	return c.MockGet()
}

// Store calls the underlying MockStore.
func (c *MockCache) Store(s string, rc io.ReadCloser) error {
	return c.MockStore(s, rc)
}

// Delete calls the underlying MockDelete.
func (c *MockCache) Delete(string) error {
	return c.MockDelete()
}

var _ xpkg.Fetcher = &MockFetcher{}

// MockFetcher is a mock fetcher.
type MockFetcher struct {
	MockFetch func() (v1.Image, error)
	MockHead  func() (*v1.Descriptor, error)
	MockTags  func() ([]string, error)
}

// NewMockFetchFn creates a new MockFetch function for MockFetcher.
func NewMockFetchFn(img v1.Image, err error) func() (v1.Image, error) {
	return func() (v1.Image, error) { return img, err }
}

// Fetch calls the underlying MockFetch.
func (m *MockFetcher) Fetch(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
	return m.MockFetch()
}

// NewMockHeadFn creates a new MockHead function for MockFetcher.
func NewMockHeadFn(d *v1.Descriptor, err error) func() (*v1.Descriptor, error) {
	return func() (*v1.Descriptor, error) { return d, err }
}

// Head calls the underlying MockHead.
func (m *MockFetcher) Head(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
	return m.MockHead()
}

// NewMockTagsFn creates a new MockTags function for MockFetcher.
func NewMockTagsFn(tags []string, err error) func() ([]string, error) {
	return func() ([]string, error) { return tags, err }
}

// Tags calls the underlying MockTags.
func (m *MockFetcher) Tags(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
	return m.MockTags()
}
