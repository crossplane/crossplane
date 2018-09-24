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

package database

import (
	"fmt"

	gcpclients "github.com/upbound/conductor/pkg/clients/gcp"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// mockCloudSQLClient provides a mock implementation of the CloudSQLAPI interface for unit testing purposes
type mockCloudSQLClient struct {
	gcpclients.CloudSQLAPI
	MockGetInstance    func(project string, instance string) (*sqladmin.DatabaseInstance, error)
	MockCreateInstance func(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error)
}

// GetInstance retrieves details for the requested CloudSQL instance
func (m *mockCloudSQLClient) GetInstance(project string, instance string) (*sqladmin.DatabaseInstance, error) {
	if m.MockGetInstance != nil {
		return m.MockGetInstance(project, instance)
	}

	// default implementation
	return createMockDatabaseInstance(project, instance, "RUNNABLE"), nil
}

// CreateInstance creates the given CloudSQL instance
func (m *mockCloudSQLClient) CreateInstance(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error) {
	if m.MockCreateInstance != nil {
		return m.MockCreateInstance(project, databaseinstance)
	}
	return &sqladmin.Operation{}, nil
}

func (m *mockCloudSQLClient) ListUsers(project string, instance string) (*sqladmin.UsersListResponse, error) {
	return &sqladmin.UsersListResponse{}, nil
}

// CreateMockDatabaseInstance creates a simple test instance of a CloudSQL database instance object
func createMockDatabaseInstance(project, instance, state string) *sqladmin.DatabaseInstance {
	return &sqladmin.DatabaseInstance{
		Name:     instance,
		State:    state,
		SelfLink: fmt.Sprintf("https://www.googleapis.com/sql/v1beta4/projects/%s/instances/%s", project, instance),
	}
}

type mockCloudSQLClientFactory struct {
	mockClient *mockCloudSQLClient
}

func (m *mockCloudSQLClientFactory) CreateAPIInstance(kubernetes.Interface, string, v1.SecretKeySelector) (gcpclients.CloudSQLAPI, error) {
	return m.mockClient, nil
}
