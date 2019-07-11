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

package fake

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"

	azurestorage "github.com/crossplaneio/crossplane/pkg/clients/azure/storage"
)

// MockAccountOperations mock implementation of AccountOperations
type MockAccountOperations struct {
	MockCreate                 func(context.Context, storage.AccountCreateParameters) (*storage.Account, error)
	MockUpdate                 func(context.Context, storage.AccountUpdateParameters) (*storage.Account, error)
	MockGet                    func(ctx context.Context) (*storage.Account, error)
	MockDelete                 func(ctx context.Context) error
	MockIsAccountNameAvailable func(context.Context, string) error
	MockListKeys               func(context.Context) ([]storage.AccountKey, error)
}

var _ azurestorage.AccountOperations = &MockAccountOperations{}

// NewMockAccountOperations returns new mock instance with default mocks
func NewMockAccountOperations() *MockAccountOperations {
	return &MockAccountOperations{
		MockCreate: func(i context.Context, parameters storage.AccountCreateParameters) (account *storage.Account, e error) {
			return nil, nil
		},
		MockUpdate: func(i context.Context, parameters storage.AccountUpdateParameters) (account *storage.Account, e error) {
			return nil, nil
		},
		MockGet: func(ctx context.Context) (account *storage.Account, e error) {
			return nil, nil
		},
		MockDelete: func(ctx context.Context) error {
			return nil
		},
		MockIsAccountNameAvailable: func(i context.Context, s string) error {
			return nil
		},
		MockListKeys: func(i context.Context) ([]storage.AccountKey, error) {
			return nil, nil
		},
	}
}

// Create mock create
func (m *MockAccountOperations) Create(ctx context.Context, params storage.AccountCreateParameters) (*storage.Account, error) {
	return m.MockCreate(ctx, params)
}

// Update mock update
func (m *MockAccountOperations) Update(ctx context.Context, params storage.AccountUpdateParameters) (*storage.Account, error) {
	return m.MockUpdate(ctx, params)
}

// Get mock get
func (m *MockAccountOperations) Get(ctx context.Context) (*storage.Account, error) {
	return m.MockGet(ctx)
}

// Delete mock delete
func (m *MockAccountOperations) Delete(ctx context.Context) error {
	return m.MockDelete(ctx)
}

// IsAccountNameAvailable mock check
func (m *MockAccountOperations) IsAccountNameAvailable(ctx context.Context, name string) error {
	return m.MockIsAccountNameAvailable(ctx, name)
}

// ListKeys mock list keys
func (m *MockAccountOperations) ListKeys(ctx context.Context) ([]storage.AccountKey, error) {
	return m.MockListKeys(ctx)
}
