/*
Copyright 2018 The Conductor Authors.

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

package gcp

import (
	"fmt"

	"google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/client-go/kubernetes"
)

// CloudSQLAPI provides an interface for operations on CloudSQL instances
type CloudSQLAPI interface {
	GetInstance(project string, instance string) (*sqladmin.DatabaseInstance, error)
	CreateInstance(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error)
}

// CloudSQLClient implements the CloudSQLAPI interface for real CloudSQL instances
type CloudSQLClient struct {
	*sqladmin.Service
}

// NewCloudSQLClient creates a new instance of a CloudSQLClient
func NewCloudSQLClient(clientset kubernetes.Interface) (*CloudSQLClient, error) {
	hc, err := GetGoogleClient(clientset, sqladmin.SqlserviceAdminScope)
	if err != nil {
		return nil, err
	}

	service, err := sqladmin.New(hc)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqladmin client: %+v", err)
	}

	return &CloudSQLClient{service}, nil
}

// GetInstance retrieves details for the requested CloudSQL instance
func (c *CloudSQLClient) GetInstance(project string, instance string) (*sqladmin.DatabaseInstance, error) {
	return c.Instances.Get(project, instance).Do()
}

// CreateInstance creates the given CloudSQL instance
func (c *CloudSQLClient) CreateInstance(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error) {
	return c.Instances.Insert(project, databaseinstance).Do()
}
