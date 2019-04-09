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

	"github.com/Azure/azure-storage-blob-go/azblob"

	azurestorage "github.com/crossplaneio/crossplane/pkg/clients/azure/storage"
)

// MockContainerOperations mock implementation of ContainerOperations
type MockContainerOperations struct {
	MockCreate func(context.Context, azblob.PublicAccessType, azblob.Metadata) error
	MockUpdate func(context.Context, azblob.PublicAccessType, azblob.Metadata) error
	MockGet    func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error)
	MockDelete func(ctx context.Context) error
}

var _ azurestorage.ContainerOperations = &MockContainerOperations{}

// NewMockContainerOperations create new mock instance with default mocks
func NewMockContainerOperations() *MockContainerOperations {
	return &MockContainerOperations{
		MockCreate: func(ctx context.Context, pat azblob.PublicAccessType, meta azblob.Metadata) error {
			return nil
		},
		MockUpdate: func(ctx context.Context, pat azblob.PublicAccessType, meta azblob.Metadata) error {
			return nil
		},
		MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
			return nil, nil, nil
		},
		MockDelete: func(ctx context.Context) error {
			return nil
		},
	}
}

// Create mock create function
func (m *MockContainerOperations) Create(ctx context.Context, pat azblob.PublicAccessType, meta azblob.Metadata) error {
	return m.MockCreate(ctx, pat, meta)
}

// Update mock update function
func (m *MockContainerOperations) Update(ctx context.Context, pat azblob.PublicAccessType, meta azblob.Metadata) error {
	return m.MockUpdate(ctx, pat, meta)
}

// Get mock get function
func (m *MockContainerOperations) Get(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
	return m.MockGet(ctx)
}

// Delete mock delete function
func (m *MockContainerOperations) Delete(ctx context.Context) error {
	return m.MockDelete(ctx)
}

// PublicAccessTypePtr returns pointer of the PublicAccessType value
func PublicAccessTypePtr(pab azblob.PublicAccessType) *azblob.PublicAccessType {
	return &pab
}
