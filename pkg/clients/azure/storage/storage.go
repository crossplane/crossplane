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

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	"github.com/Azure/azure-storage-blob-go/azblob"
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
	CreateStorageAccount(ctx context.Context, location string) (storage.Account, error)
	GetStorageAccount(ctx context.Context) (storage.Account, error)
	DeleteStorageAccount(ctx context.Context) error
	IsAccountNameAvailable(ctx context.Context) (bool, error)
	ListKeys(context.Context) ([]storage.AccountKey, error)
	Container(context.Context, string) (ContainerOperations, error)
}

// AccountHandle implements AccountOperations interface
type AccountHandle struct {
	client storage.AccountsClient

	GroupName   string
	AccountName string
}

var _ AccountOperations = &AccountHandle{}

// NewAccountHandle creates a new storage account with specific name,
func NewAccountHandle(client storage.AccountsClient, groupName, accountName string) (*AccountHandle, error) {
	return &AccountHandle{
		client:      client,
		GroupName:   groupName,
		AccountName: accountName,
	}, nil
}

// CreateStorageAccount create new storage account with given location
func (a *AccountHandle) CreateStorageAccount(ctx context.Context, location string) (storage.Account, error) {
	acct := storage.Account{}

	if ok, err := a.IsAccountNameAvailable(ctx); err != nil {
		return acct, errors.Wrapf(err, "failed to check account name availability")
	} else if !ok {
		return acct, errors.Errorf("account name: %s is not available", a.AccountName)
	}

	future, err := a.client.Create(
		ctx,
		a.GroupName,
		a.AccountName,
		storage.AccountCreateParameters{
			Sku: &storage.Sku{
				Name: storage.StandardLRS},
			Kind:                              storage.Storage,
			Location:                          to.StringPtr(location),
			AccountPropertiesCreateParameters: &storage.AccountPropertiesCreateParameters{},
		})
	if err != nil {
		return storage.Account{}, errors.Wrapf(err, "failed to start creating storage account")
	}

	err = future.WaitForCompletionRef(ctx, a.client.Client)
	if err != nil {
		return storage.Account{}, errors.Wrapf(err, "failed to finish creating storage account")
	}

	return future.Result(a.client)
}

// IsAccountNameAvailable checks if AccountHandle name is not being used (Azure requires unique storage account names)
func (a *AccountHandle) IsAccountNameAvailable(ctx context.Context) (bool, error) {
	result, err := a.client.CheckNameAvailability(
		ctx,
		storage.AccountCheckNameAvailabilityParameters{
			Name: to.StringPtr(a.AccountName),
			Type: to.StringPtr("Microsoft.Storage/storageAccounts"),
		})
	if err != nil {
		return false, err
	}

	return result.NameAvailable != nil && *result.NameAvailable, nil
}

// GetStorageAccount retrieves storage account resource
func (a *AccountHandle) GetStorageAccount(ctx context.Context) (storage.Account, error) {
	return a.client.GetProperties(ctx, a.AccountName, a.GroupName)
}

// DeleteStorageAccount deletes storage account resource
func (a *AccountHandle) DeleteStorageAccount(ctx context.Context) error {
	_, err := a.client.Delete(ctx, a.GroupName, a.AccountName)
	return err
}

// ListKeys for this storage account
func (a *AccountHandle) ListKeys(ctx context.Context) ([]storage.AccountKey, error) {
	rs, err := a.client.ListKeys(ctx, a.GroupName, a.AccountName)
	if err != nil {
		return nil, err
	}

	return *rs.Keys, nil
}

// Container creates a container handle for container (bucket) resource under this account
func (a *AccountHandle) Container(ctx context.Context, containerName string) (ContainerOperations, error) {
	keys, err := a.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.Errorf("keys not found for account: %s, group: %s", a.AccountName, a.GroupName)
	}

	return NewContainerHandle(a.AccountName, *keys[0].Value, containerName)
}

// ContainerOperations interface to perform operations on Container resources
type ContainerOperations interface {
	CreateContainer(ctx context.Context, publicAccessType azblob.PublicAccessType) (*azblob.ContainerCreateResponse, error)
	GetContainer(ctx context.Context) (*azblob.ContainerGetPropertiesResponse, error)
	DeleteContainer(ctx context.Context) (*azblob.ContainerDeleteResponse, error)
}

// ContainerHandle implements ContainerOperations
type ContainerHandle struct {
	azblob.ContainerURL
}

var _ ContainerOperations = &ContainerHandle{}

const blobFormatString = `https://%s.blob.core.windows.net`

// NewContainerHandle creates a new instance of ContainerHandle for given storage account and given container name
func NewContainerHandle(accountName, accountKey, containerName string) (*ContainerHandle, error) {
	c, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}

	p := azblob.NewPipeline(c, azblob.PipelineOptions{
		Telemetry: azblob.TelemetryOptions{Value: azure.UserAgent},
	})

	u, _ := url.Parse(fmt.Sprintf(blobFormatString, accountName))
	service := azblob.NewServiceURL(*u, p)

	return &ContainerHandle{
		ContainerURL: service.NewContainerURL(containerName),
	}, nil
}

// CreateContainer with given public access type
func (a *ContainerHandle) CreateContainer(ctx context.Context, publicAccessType azblob.PublicAccessType) (*azblob.ContainerCreateResponse, error) {
	return a.ContainerURL.Create(ctx, azblob.Metadata{}, publicAccessType)
}

// GetContainer resource information
func (a *ContainerHandle) GetContainer(ctx context.Context) (*azblob.ContainerGetPropertiesResponse, error) {
	return a.ContainerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
}

// DeleteContainer deletes the named container.
func (a *ContainerHandle) DeleteContainer(ctx context.Context) (*azblob.ContainerDeleteResponse, error) {
	return a.ContainerURL.Delete(ctx, azblob.ContainerAccessConditions{})
}
