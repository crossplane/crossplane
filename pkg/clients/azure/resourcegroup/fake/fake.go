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

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources/resourcesapi"
	"github.com/Azure/go-autorest/autorest"
)

var _ resourcesapi.GroupsClientAPI = &MockClient{}

// MockClient is a fake implementation of the azure groups client.
type MockClient struct {
	resourcesapi.GroupsClientAPI

	MockCreateOrUpdate func(ctx context.Context, resourceGroupName string, parameters resources.Group) (result resources.Group, err error)
	MockCheckExistence func(ctx context.Context, resourceGroupName string) (result autorest.Response, err error)
	MockDelete         func(ctx context.Context, resourceGroupName string) (result resources.GroupsDeleteFuture, err error)
}

// CreateOrUpdate calls the underlying MockCreateOrUpdate method.
func (m *MockClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, parameters resources.Group) (result resources.Group, err error) {
	return m.MockCreateOrUpdate(ctx, resourceGroupName, parameters)
}

// CheckExistence calls the underlying MockCheckExistence method.
func (m *MockClient) CheckExistence(ctx context.Context, resourceGroupName string) (result autorest.Response, err error) {
	return m.MockCheckExistence(ctx, resourceGroupName)
}

// Delete calls the underlying MockDeleteGroup method.
func (m *MockClient) Delete(ctx context.Context, resourceGroupName string) (result resources.GroupsDeleteFuture, err error) {
	return m.MockDelete(ctx, resourceGroupName)
}
