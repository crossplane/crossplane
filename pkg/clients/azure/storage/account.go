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
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

// NewStorageAccountClient create Azure storage.AccountClient using provided credentials data
func NewStorageAccountClient(data []byte) (*storage.AccountsClient, error) {
	creds := &azure.Credentials{}
	if err := json.Unmarshal(data, creds); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}

	config := auth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	config.AADEndpoint = creds.ActiveDirectoryEndpointURL
	config.Resource = creds.ResourceManagerEndpointURL

	authorizer, err := config.Authorizer()
	if err != nil {
		return nil, fmt.Errorf("failed to get authorizer from config: %+v", err)
	}

	client := storage.NewAccountsClient(creds.SubscriptionID)
	client.Authorizer = authorizer

	if err := client.AddToUserAgent(azure.UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}
	return &client, nil
}

// AccountOperations Azure storate account interface
type AccountOperations interface {
	Create(context.Context, storage.AccountCreateParameters) (*storage.Account, error)
	Update(context.Context, storage.AccountUpdateParameters) (*storage.Account, error)
	Get(ctx context.Context) (*storage.Account, error)
	Delete(ctx context.Context) error
	IsAccountNameAvailable(context.Context, string) error
	ListKeys(context.Context) ([]storage.AccountKey, error)
}

// AccountHandle implements AccountOperations interface
type AccountHandle struct {
	client      *storage.AccountsClient
	groupName   string
	accountName string
}

var _ AccountOperations = &AccountHandle{}

// NewAccountHandle creates a new storage account with specific name,
func NewAccountHandle(client *storage.AccountsClient, groupName, accountName string) *AccountHandle {
	return &AccountHandle{
		client:      client,
		groupName:   groupName,
		accountName: accountName,
	}
}

// Create create new storage account with given location
func (a *AccountHandle) Create(ctx context.Context, params storage.AccountCreateParameters) (*storage.Account, error) {
	if err := a.IsAccountNameAvailable(ctx, a.accountName); err != nil {
		return nil, errors.Wrapf(err, "failed to check account name availability")
	}

	future, err := a.client.Create(ctx, a.groupName, a.accountName, params)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start creating storage account")
	}

	err = future.WaitForCompletionRef(ctx, a.client.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to finish creating storage account")
	}

	acct, err := future.Result(*a.client)
	if err != nil {
		return nil, err
	}
	return &acct, nil
}

// Update create new storage account with given location
func (a *AccountHandle) Update(ctx context.Context, params storage.AccountUpdateParameters) (*storage.Account, error) {
	acct, err := a.client.Update(ctx, a.groupName, a.accountName, params)
	if err != nil {
		return nil, err
	}
	return &acct, nil
}

// Get retrieves storage account resource
func (a *AccountHandle) Get(ctx context.Context) (*storage.Account, error) {
	acct, err := a.client.GetProperties(ctx, a.groupName, a.accountName)
	if err != nil {
		return nil, err
	}
	return &acct, nil
}

// Delete deletes storage account resource
func (a *AccountHandle) Delete(ctx context.Context) error {
	_, err := a.client.Delete(ctx, a.groupName, a.accountName)
	return err
}

// IsAccountNameAvailable checks if AccountHandle name is not being used (Azure requires unique storage account names)
func (a *AccountHandle) IsAccountNameAvailable(ctx context.Context, name string) error {
	result, err := a.client.CheckNameAvailability(
		ctx,
		storage.AccountCheckNameAvailabilityParameters{
			Name: to.StringPtr(name),
			Type: to.StringPtr("Microsoft.Storage/storageAccounts"),
		})
	if err != nil {
		return err
	}

	if result.NameAvailable == nil || !*result.NameAvailable {
		return errors.Errorf("%s - %s", result.Reason, to.String(result.Message))
	}

	return nil
}

// ListKeys for this storage account
func (a *AccountHandle) ListKeys(ctx context.Context) ([]storage.AccountKey, error) {
	rs, err := a.client.ListKeys(ctx, a.groupName, a.accountName)
	if err != nil {
		return nil, err
	}

	return *rs.Keys, nil
}
