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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network/networkapi"
)

var _ networkapi.VirtualNetworksClientAPI = &MockVirtualNetworksClient{}

// MockVirtualNetworksClient is a fake implementation of network.VirtualNetworksClient.
type MockVirtualNetworksClient struct {
	networkapi.VirtualNetworksClientAPI

	MockCreateOrUpdate func(ctx context.Context, resourceGroupName string, virtualNetworkName string, parameters network.VirtualNetwork) (result network.VirtualNetworksCreateOrUpdateFuture, err error)
	MockDelete         func(ctx context.Context, resourceGroupName string, virtualNetworkName string) (result network.VirtualNetworksDeleteFuture, err error)
	MockGet            func(ctx context.Context, resourceGroupName string, virtualNetworkName string, expand string) (result network.VirtualNetwork, err error)
	MockList           func(ctx context.Context, resourceGroupName string) (result network.VirtualNetworkListResultPage, err error)
}

// CreateOrUpdate calls the MockVirtualNetworksClient's MockCreateOrUpdate method.
func (c *MockVirtualNetworksClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, virtualNetworkName string, parameters network.VirtualNetwork) (result network.VirtualNetworksCreateOrUpdateFuture, err error) {
	return c.MockCreateOrUpdate(ctx, resourceGroupName, virtualNetworkName, parameters)
}

// Delete calls the MockVirtualNetworksClient's MockDelete method.
func (c *MockVirtualNetworksClient) Delete(ctx context.Context, resourceGroupName string, virtualNetworkName string) (result network.VirtualNetworksDeleteFuture, err error) {
	return c.MockDelete(ctx, resourceGroupName, virtualNetworkName)
}

// Get calls the MockVirtualNetworksClient's MockGet method.
func (c *MockVirtualNetworksClient) Get(ctx context.Context, resourceGroupName string, virtualNetworkName string, expand string) (result network.VirtualNetwork, err error) {
	return c.MockGet(ctx, resourceGroupName, virtualNetworkName, expand)
}

// List calls the MockVirtualNetworksClient's MockListKeys method.
func (c *MockVirtualNetworksClient) List(ctx context.Context, resourceGroupName string) (result network.VirtualNetworkListResultPage, err error) {
	return c.MockList(ctx, resourceGroupName)
}

var _ networkapi.SubnetsClientAPI = &MockSubnetsClient{}

// MockSubnetsClient is a fake implementation of network.SubnetsClient.
type MockSubnetsClient struct {
	networkapi.SubnetsClientAPI

	MockCreateOrUpdate func(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, subnetParameters network.Subnet) (result network.SubnetsCreateOrUpdateFuture, err error)
	MockDelete         func(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string) (result network.SubnetsDeleteFuture, err error)
	MockGet            func(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, expand string) (result network.Subnet, err error)
	MockList           func(ctx context.Context, resourceGroupName string, virtualNetworkName string) (result network.SubnetListResultPage, err error)
}

// CreateOrUpdate calls the MockSubnetsClient's MockCreateOrUpdate method.
func (c *MockSubnetsClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, subnetParameters network.Subnet) (result network.SubnetsCreateOrUpdateFuture, err error) {
	return c.MockCreateOrUpdate(ctx, resourceGroupName, virtualNetworkName, subnetName, subnetParameters)
}

// Delete calls the MockSubnetsClient's MockDelete method.
func (c *MockSubnetsClient) Delete(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string) (result network.SubnetsDeleteFuture, err error) {
	return c.MockDelete(ctx, resourceGroupName, virtualNetworkName, subnetName)
}

// Get calls the MockSubnetsClient's MockGet method.
func (c *MockSubnetsClient) Get(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, expand string) (result network.Subnet, err error) {
	return c.MockGet(ctx, resourceGroupName, virtualNetworkName, subnetName, expand)
}

// List calls the MockSubnetsClient's MockListKeys method.
func (c *MockSubnetsClient) List(ctx context.Context, resourceGroupName string, virtualNetworkName string) (result network.SubnetListResultPage, err error) {
	return c.MockList(ctx, resourceGroupName, virtualNetworkName)
}
