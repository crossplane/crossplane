package storage

import (
	"context"

	"cloud.google.com/go/storage"
)

// Client bucket resource operations interface
type Client interface {
	Attrs(context.Context) (*storage.BucketAttrs, error)
	Create(context.Context, string, *storage.BucketAttrs) error
	Update(context.Context, storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error)
	Delete(context.Context) error
}

// BucketClient implements Client interface
type BucketClient struct {
	*storage.BucketHandle
}
