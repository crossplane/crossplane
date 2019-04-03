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
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

// MockRGClient is a fake implementation of the azure groups client.
type MockRGClient struct {
	MockCreateOrUpdateGroup func(client *azure.Client, name string, location string) error
	MockCheckExistence      func(client *azure.Client, name string, location string) (bool, error)
	MockDeleteGroup         func(client *azure.Client, name string, location string) (resources.GroupsDeleteFuture, error)
}

// CreateOrUpdateGroup calls the underlying MockCreateOrUpdateGroup method.
func (m *MockRGClient) CreateOrUpdateGroup(client *azure.Client, name string, location string) error {
	return m.MockCreateOrUpdateGroup(client, name, location)
}

// CheckExistence calls the underlying MockCheckExistence method.
func (m *MockRGClient) CheckExistence(client *azure.Client, name string, location string) (bool, error) {
	return m.MockCheckExistence(client, name, location)
}

// DeleteGroup calls the underlying MockDeleteGroup method.
func (m *MockRGClient) DeleteGroup(client *azure.Client, name string, location string) (resources.GroupsDeleteFuture, error) {
	return m.MockDeleteGroup(client, name, location)
}
