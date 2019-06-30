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

	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudsql"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// MockUserClient implements UserService interface
type MockUserClient struct {
	MockList   func(context.Context, string) ([]*sqladmin.User, error)
	MockCreate func(context.Context, string, *sqladmin.User) error
	MockUpdate func(context.Context, string, string, *sqladmin.User) error
	MockDelete func(context.Context, string, string, string) error
}

// Interface validation
var _ cloudsql.UserService = &MockUserClient{}

// List and return all users for a provided instance (name)
func (c *MockUserClient) List(ctx context.Context, instance string) ([]*sqladmin.User, error) {
	return c.MockList(ctx, instance)
}

// Create new user for a given instance with provided user definition
func (c *MockUserClient) Create(ctx context.Context, instance string, user *sqladmin.User) error {
	return c.MockCreate(ctx, instance, user)
}

// Update existing user for a given instance with provided user definition
func (c *MockUserClient) Update(ctx context.Context, instance, userName string, user *sqladmin.User) error {
	return c.MockUpdate(ctx, instance, userName, user)
}

// Delete existing user from a given instance database with matching name
func (c *MockUserClient) Delete(ctx context.Context, instance, database, user string) error {
	return c.MockDelete(ctx, instance, database, user)
}
