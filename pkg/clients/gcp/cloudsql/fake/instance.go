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

	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudsql"
)

// MockInstanceClient for testing purposes
type MockInstanceClient struct {
	MockGet    func(context.Context, string) (*sqladmin.DatabaseInstance, error)
	MockCreate func(context.Context, *sqladmin.DatabaseInstance) error
	MockUpdate func(context.Context, string, *sqladmin.DatabaseInstance) error
	MockDelete func(context.Context, string) error
}

var _ cloudsql.InstanceService = &MockInstanceClient{}

// Get attempts to retrieve and return cloudsql instance using provided name value
func (c *MockInstanceClient) Get(ctx context.Context, name string) (*sqladmin.DatabaseInstance, error) {
	return c.MockGet(ctx, name)
}

// Create new cloudsql instance with provided instance definition and return newly created instance
func (c *MockInstanceClient) Create(ctx context.Context, instance *sqladmin.DatabaseInstance) error {
	return c.MockCreate(ctx, instance)
}

// Update cloudsql instance with matching name with provided instance definition and return newly create instance
func (c *MockInstanceClient) Update(ctx context.Context, name string, instance *sqladmin.DatabaseInstance) error {
	return c.MockUpdate(ctx, name, instance)
}

// Delete cloudsql instance with matching name
func (c *MockInstanceClient) Delete(ctx context.Context, name string) error {
	return c.MockDelete(ctx, name)
}
