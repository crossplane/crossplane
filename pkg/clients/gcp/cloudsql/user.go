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

package cloudsql

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// UserService provides an interface for operations on cloudsql users
type UserService interface {
	List(context.Context, string) ([]*sqladmin.User, error)
	Create(context.Context, string, *sqladmin.User) error
	Update(context.Context, string, string, *sqladmin.User) error
	Delete(context.Context, string, string, string) error
}

// UserClient implements UserService interface
type UserClient struct {
	service   *sqladmin.UsersService
	projectID string
}

// Interface validation
var _ UserService = &UserClient{}

// NewUserClient creates new instance of UserClient
func NewUserClient(ctx context.Context, creds *google.Credentials) (*UserClient, error) {
	service, err := sqladmin.New(oauth2.NewClient(ctx, creds.TokenSource))
	if err != nil {
		return nil, err
	}

	return &UserClient{
		service:   service.Users,
		projectID: creds.ProjectID,
	}, nil
}

// List and return all users for a provided instance (name)
func (c *UserClient) List(ctx context.Context, instance string) ([]*sqladmin.User, error) {
	res, err := c.service.List(c.projectID, instance).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

// Create new user for a given instance with provided user definition
func (c *UserClient) Create(ctx context.Context, instance string, user *sqladmin.User) error {
	_, err := c.service.Insert(c.projectID, instance, user).Context(ctx).Do()
	return err
}

// Update existing user for a given instance with provided user definition
func (c *UserClient) Update(ctx context.Context, instance, userName string, user *sqladmin.User) error {
	_, err := c.service.Update(c.projectID, instance, userName, user).Host(user.Host).Context(ctx).Do()
	return err
}

// Delete existing user from a given instance database with matching name
func (c *UserClient) Delete(ctx context.Context, instance, database, user string) error {
	_, err := c.service.Delete(c.projectID, instance, database, user).Context(ctx).Do()
	return err
}
