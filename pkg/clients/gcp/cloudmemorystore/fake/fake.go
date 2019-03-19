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

	redisv1 "cloud.google.com/go/redis/apiv1"
	gax "github.com/googleapis/gax-go"
	redisv1pb "google.golang.org/genproto/googleapis/cloud/redis/v1"

	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudmemorystore"
)

var _ cloudmemorystore.Client = &MockClient{}

// MockClient is a fake implementation of cloudmemorystore.Client.
type MockClient struct {
	MockCreateInstance func(context.Context, *redisv1pb.CreateInstanceRequest, ...gax.CallOption) (*redisv1.CreateInstanceOperation, error)
	MockUpdateInstance func(context.Context, *redisv1pb.UpdateInstanceRequest, ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error)
	MockDeleteInstance func(context.Context, *redisv1pb.DeleteInstanceRequest, ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error)
	MockGetInstance    func(context.Context, *redisv1pb.GetInstanceRequest, ...gax.CallOption) (*redisv1pb.Instance, error)
}

// CreateInstance calls the MockClient's MockCreateInstance function.
func (c *MockClient) CreateInstance(ctx context.Context, req *redisv1pb.CreateInstanceRequest, opts ...gax.CallOption) (*redisv1.CreateInstanceOperation, error) {
	return c.MockCreateInstance(ctx, req, opts...)
}

// UpdateInstance calls the MockClient's MockUpdateInstance function.
func (c *MockClient) UpdateInstance(ctx context.Context, req *redisv1pb.UpdateInstanceRequest, opts ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error) {
	return c.MockUpdateInstance(ctx, req, opts...)
}

// DeleteInstance calls the MockClient's MockDeleteInstance function.
func (c *MockClient) DeleteInstance(ctx context.Context, req *redisv1pb.DeleteInstanceRequest, opts ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
	return c.MockDeleteInstance(ctx, req, opts...)
}

// GetInstance calls the MockClient's MockGetInstance function.
func (c *MockClient) GetInstance(ctx context.Context, req *redisv1pb.GetInstanceRequest, opts ...gax.CallOption) (*redisv1pb.Instance, error) {
	return c.MockGetInstance(ctx, req, opts...)
}
