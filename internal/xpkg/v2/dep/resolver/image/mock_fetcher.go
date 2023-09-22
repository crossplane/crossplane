/*
Copyright 2023 The Crossplane Authors.

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

package image

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// MockFetcher is an image fetcher that returns its configured values.
type MockFetcher struct {
	tags []string
	img  v1.Image
	dsc  *v1.Descriptor
	err  error
}

// NewMockFetcher constructs a new mock fetcher.
func NewMockFetcher(opts ...MockOption) *MockFetcher {
	f := &MockFetcher{}
	for _, o := range opts {
		o(f)
	}
	return f
}

// MockOption modifies the mock fetcher.
type MockOption func(*MockFetcher)

// WithTags sets the tags for the mock fetcher.
func WithTags(tags []string) MockOption {
	return func(m *MockFetcher) {
		m.tags = tags
	}
}

// WithError sets the error for the mock fetcher.
func WithError(err error) MockOption {
	return func(m *MockFetcher) {
		m.err = err
	}
}

// WithImage sets the image for the mock fetcher.
func WithImage(img v1.Image) MockOption {
	return func(m *MockFetcher) {
		m.img = img
	}
}

// WithDescriptor sets the descriptor for the mock fetcher.
func WithDescriptor(dsc *v1.Descriptor) MockOption {
	return func(m *MockFetcher) {
		m.dsc = dsc
	}
}

// Fetch returns the configured error.
func (m *MockFetcher) Fetch(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
	return m.img, m.err
}

// Head returns the configured error.
func (m *MockFetcher) Head(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
	return m.dsc, m.err
}

// Tags returns the configured tags or if none exist then error.
func (m *MockFetcher) Tags(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
	return m.tags, m.err
}
