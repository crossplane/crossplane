/*
Copyright 2019 The Crossplane Authors.

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
