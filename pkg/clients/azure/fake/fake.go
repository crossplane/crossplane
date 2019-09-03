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

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql/mysqlapi"
	"github.com/Azure/azure-sdk-for-go/services/postgresql/mgmt/2017-12-01/postgresql"
	"github.com/Azure/azure-sdk-for-go/services/postgresql/mgmt/2017-12-01/postgresql/postgresqlapi"
)

var _ mysqlapi.VirtualNetworkRulesClientAPI = &MockMySQLVirtualNetworkRulesClient{}

// MockMySQLVirtualNetworkRulesClient is a fake implementation of mysql.VirtualNetworkRulesClient.
type MockMySQLVirtualNetworkRulesClient struct {
	mysqlapi.VirtualNetworkRulesClientAPI

	MockCreateOrUpdate func(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string, parameters mysql.VirtualNetworkRule) (result mysql.VirtualNetworkRulesCreateOrUpdateFuture, err error)
	MockDelete         func(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result mysql.VirtualNetworkRulesDeleteFuture, err error)
	MockGet            func(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result mysql.VirtualNetworkRule, err error)
	MockListByServer   func(ctx context.Context, resourceGroupName string, serverName string) (result mysql.VirtualNetworkRuleListResultPage, err error)
}

// CreateOrUpdate calls the MockMySQLVirtualNetworkRulesClient's MockCreateOrUpdate method.
func (c *MockMySQLVirtualNetworkRulesClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string, parameters mysql.VirtualNetworkRule) (result mysql.VirtualNetworkRulesCreateOrUpdateFuture, err error) {
	return c.MockCreateOrUpdate(ctx, resourceGroupName, serverName, virtualNetworkRuleName, parameters)
}

// Delete calls the MockMySQLVirtualNetworkRulesClient's MockDelete method.
func (c *MockMySQLVirtualNetworkRulesClient) Delete(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result mysql.VirtualNetworkRulesDeleteFuture, err error) {
	return c.MockDelete(ctx, resourceGroupName, serverName, virtualNetworkRuleName)
}

// Get calls the MockMySQLVirtualNetworkRulesClient's MockGet method.
func (c *MockMySQLVirtualNetworkRulesClient) Get(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result mysql.VirtualNetworkRule, err error) {
	return c.MockGet(ctx, resourceGroupName, serverName, virtualNetworkRuleName)
}

// ListByServer calls the MockMySQLVirtualNetworkRulesClient's MockListByServer method.
func (c *MockMySQLVirtualNetworkRulesClient) ListByServer(ctx context.Context, resourceGroupName string, serverName string) (result mysql.VirtualNetworkRuleListResultPage, err error) {
	return c.MockListByServer(ctx, resourceGroupName, serverName)
}

var _ postgresqlapi.VirtualNetworkRulesClientAPI = &MockPostgreSQLVirtualNetworkRulesClient{}

// MockPostgreSQLVirtualNetworkRulesClient is a fake implementation of postgresql.VirtualNetworkRulesClient.
type MockPostgreSQLVirtualNetworkRulesClient struct {
	postgresqlapi.VirtualNetworkRulesClientAPI

	MockCreateOrUpdate func(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string, parameters postgresql.VirtualNetworkRule) (result postgresql.VirtualNetworkRulesCreateOrUpdateFuture, err error)
	MockDelete         func(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result postgresql.VirtualNetworkRulesDeleteFuture, err error)
	MockGet            func(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result postgresql.VirtualNetworkRule, err error)
	MockListByServer   func(ctx context.Context, resourceGroupName string, serverName string) (result postgresql.VirtualNetworkRuleListResultPage, err error)
}

// CreateOrUpdate calls the MockPostgreSQLVirtualNetworkRulesClient's MockCreateOrUpdate method.
func (c *MockPostgreSQLVirtualNetworkRulesClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string, parameters postgresql.VirtualNetworkRule) (result postgresql.VirtualNetworkRulesCreateOrUpdateFuture, err error) {
	return c.MockCreateOrUpdate(ctx, resourceGroupName, serverName, virtualNetworkRuleName, parameters)
}

// Delete calls the MockPostgreSQLVirtualNetworkRulesClient's MockDelete method.
func (c *MockPostgreSQLVirtualNetworkRulesClient) Delete(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result postgresql.VirtualNetworkRulesDeleteFuture, err error) {
	return c.MockDelete(ctx, resourceGroupName, serverName, virtualNetworkRuleName)
}

// Get calls the MockPostgreSQLVirtualNetworkRulesClient's MockGet method.
func (c *MockPostgreSQLVirtualNetworkRulesClient) Get(ctx context.Context, resourceGroupName string, serverName string, virtualNetworkRuleName string) (result postgresql.VirtualNetworkRule, err error) {
	return c.MockGet(ctx, resourceGroupName, serverName, virtualNetworkRuleName)
}

// ListByServer calls the MockPostgreSQLVirtualNetworkRulesClient's MockListByServer method.
func (c *MockPostgreSQLVirtualNetworkRulesClient) ListByServer(ctx context.Context, resourceGroupName string, serverName string) (result postgresql.VirtualNetworkRuleListResultPage, err error) {
	return c.MockListByServer(ctx, resourceGroupName, serverName)
}
