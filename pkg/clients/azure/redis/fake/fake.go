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

	"github.com/Azure/azure-sdk-for-go/profiles/latest/redis/mgmt/redis/redisapi"
	"github.com/Azure/azure-sdk-for-go/services/redis/mgmt/2018-03-01/redis"
)

var _ redisapi.ClientAPI = &MockClient{}

// MockClient is a fake implementation of cloudmemorystore.Client.
type MockClient struct {
	redisapi.ClientAPI

	MockCreate   func(ctx context.Context, resourceGroupName string, name string, parameters redis.CreateParameters) (result redis.CreateFuture, err error)
	MockDelete   func(ctx context.Context, resourceGroupName string, name string) (result redis.DeleteFuture, err error)
	MockGet      func(ctx context.Context, resourceGroupName string, name string) (result redis.ResourceType, err error)
	MockListKeys func(ctx context.Context, resourceGroupName string, name string) (result redis.AccessKeys, err error)
	MockUpdate   func(ctx context.Context, resourceGroupName string, name string, parameters redis.UpdateParameters) (result redis.ResourceType, err error)
}

// Create calls the MockClient's MockCreate method.
func (c *MockClient) Create(ctx context.Context, resourceGroupName string, name string, parameters redis.CreateParameters) (result redis.CreateFuture, err error) {
	return c.MockCreate(ctx, resourceGroupName, name, parameters)
}

// Delete calls the MockClient's MockDelete method.
func (c *MockClient) Delete(ctx context.Context, resourceGroupName string, name string) (result redis.DeleteFuture, err error) {
	return c.MockDelete(ctx, resourceGroupName, name)
}

// Get calls the MockClient's MockGet method.
func (c *MockClient) Get(ctx context.Context, resourceGroupName string, name string) (result redis.ResourceType, err error) {
	return c.MockGet(ctx, resourceGroupName, name)
}

// ListKeys calls the MockClient's MockListKeys method.
func (c *MockClient) ListKeys(ctx context.Context, resourceGroupName string, name string) (result redis.AccessKeys, err error) {
	return c.MockListKeys(ctx, resourceGroupName, name)
}

// Update calls the MockClient's MockUpdate method.
func (c *MockClient) Update(ctx context.Context, resourceGroupName string, name string, parameters redis.UpdateParameters) (result redis.ResourceType, err error) {
	return c.MockUpdate(ctx, resourceGroupName, name, parameters)
}
