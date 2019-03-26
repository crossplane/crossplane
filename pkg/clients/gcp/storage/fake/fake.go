package fake

import (
	"context"

	"cloud.google.com/go/storage"

	gcpstorage "github.com/crossplaneio/crossplane/pkg/clients/gcp/storage"
)

// MockBucketClient
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
