/*
Copyright 2018 The Crossplane Authors.

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

package fake

import (
	"context"

	"cloud.google.com/go/storage"

	gcpstorage "github.com/crossplaneio/crossplane/pkg/clients/gcp/storage"
)

// MockBucketClient Client interface implementation
type MockBucketClient struct {
	MockAttrs  func(context.Context) (*storage.BucketAttrs, error)
	MockCreate func(context.Context, string, *storage.BucketAttrs) error
	MockUpdate func(context.Context, storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error)
	MockDelete func(context.Context) error
}

// NewMockBucketClient returns new MockBucketClient with default mock implementations
func NewMockBucketClient() *MockBucketClient {
	return &MockBucketClient{
		MockAttrs:  func(i context.Context) (attrs *storage.BucketAttrs, e error) { return nil, nil },
		MockCreate: func(i context.Context, s string, attrs *storage.BucketAttrs) error { return nil },
		MockUpdate: func(i context.Context, update storage.BucketAttrsToUpdate) (attrs *storage.BucketAttrs, e error) {
			return nil, nil
		},
		MockDelete: func(i context.Context) error { return nil },
	}
}

// Attrs retrieves bucket attributes
func (m *MockBucketClient) Attrs(ctx context.Context) (*storage.BucketAttrs, error) {
	return m.MockAttrs(ctx)
}

// Create new bucket resource
func (m *MockBucketClient) Create(ctx context.Context, projectID string, attrs *storage.BucketAttrs) error {
	return m.MockCreate(ctx, projectID, attrs)
}

// Update existing bucket resource
func (m *MockBucketClient) Update(ctx context.Context, attrs storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error) {
	return m.MockUpdate(ctx, attrs)
}

// Delete existing bucket resource
func (m *MockBucketClient) Delete(ctx context.Context) error {
	return m.MockDelete(ctx)
}

// assert interface
var _ gcpstorage.Client = &MockBucketClient{}
