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

package cloudsql

import (
	"context"

	"google.golang.org/api/option"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// DefaultScope for sqladmin client
const DefaultScope = sqladmin.SqlserviceAdminScope

// InstanceService provides an interface for operations on CloudSQL instances
type InstanceService interface {
	Get(context.Context, string) (*sqladmin.DatabaseInstance, error)
	Create(context.Context, *sqladmin.DatabaseInstance) error
	Update(context.Context, string, *sqladmin.DatabaseInstance) error
	Delete(context.Context, string) error
}

// InstanceClient implements InstanceService interface
type InstanceClient struct {
	service   *sqladmin.InstancesService
	projectID string
}

// Interface validation
var _ InstanceService = &InstanceClient{}

// NewInstanceClient creates a new instance of an InstanceClient
func NewInstanceClient(ctx context.Context, creds *google.Credentials) (*InstanceClient, error) {
	service, err := sqladmin.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, creds.TokenSource)))
	if err != nil {
		return nil, err
	}

	return &InstanceClient{
		service:   service.Instances,
		projectID: creds.ProjectID,
	}, nil
}

// Get attempts to retrieve and return cloudsql instance using provided name value
func (c *InstanceClient) Get(ctx context.Context, name string) (*sqladmin.DatabaseInstance, error) {
	return c.service.Get(c.projectID, name).Context(ctx).Do()
}

// Create new cloudsql instance with provided instance definition and return newly created instance
func (c *InstanceClient) Create(ctx context.Context, instance *sqladmin.DatabaseInstance) error {
	_, err := c.service.Insert(c.projectID, instance).Context(ctx).Do()
	return err
}

// Update cloudsql instance with matching name with provided instance definition and return newly create instance
func (c *InstanceClient) Update(ctx context.Context, name string, instance *sqladmin.DatabaseInstance) error {
	_, err := c.service.Update(c.projectID, name, instance).Context(ctx).Do()
	return err
}

// Delete cloudsql instance with matching name
func (c *InstanceClient) Delete(ctx context.Context, name string) error {
	if _, err := c.Get(ctx, name); err != nil {
		return err
	}
	_, err := c.service.Delete(c.projectID, name).Context(ctx).Do()
	return err
}
