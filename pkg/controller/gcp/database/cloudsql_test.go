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

package database

import (
	"fmt"
	"time"

	dbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	gcpclients "github.com/crossplaneio/crossplane/pkg/clients/gcp"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// mockCloudSQLClient provides a mock implementation of the CloudSQLAPI interface for unit testing purposes
type mockCloudSQLClient struct {
	gcpclients.CloudSQLAPI
	MockGetInstance    func(project string, instance string) (*sqladmin.DatabaseInstance, error)
	MockCreateInstance func(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error)
	MockDeleteInstance func(project string, instance string) (*sqladmin.Operation, error)
	MockListUsers      func(project string, instance string) (*sqladmin.UsersListResponse, error)
	MockUpdateUser     func(project string, instance string, host string, name string, user *sqladmin.User) (*sqladmin.Operation, error)
	MockGetOperation   func(project string, operationID string) (*sqladmin.Operation, error)
}

// GetInstance retrieves details for the requested CloudSQL instance
func (m *mockCloudSQLClient) GetInstance(project string, instance string) (*sqladmin.DatabaseInstance, error) {
	if m.MockGetInstance != nil {
		return m.MockGetInstance(project, instance)
	}

	// default implementation
	return nil, nil
}

// CreateInstance creates the given CloudSQL instance
func (m *mockCloudSQLClient) CreateInstance(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error) {
	if m.MockCreateInstance != nil {
		return m.MockCreateInstance(project, databaseinstance)
	}
	return nil, nil
}

func (m *mockCloudSQLClient) DeleteInstance(project string, instance string) (*sqladmin.Operation, error) {
	if m.MockDeleteInstance != nil {
		return m.MockDeleteInstance(project, instance)
	}
	return nil, nil
}

func (m *mockCloudSQLClient) ListUsers(project string, instance string) (*sqladmin.UsersListResponse, error) {
	if m.MockListUsers != nil {
		return m.MockListUsers(project, instance)
	}
	return nil, nil
}

func (m *mockCloudSQLClient) UpdateUser(project string, instance string, host string, name string, user *sqladmin.User) (*sqladmin.Operation, error) {
	if m.MockUpdateUser != nil {
		return m.MockUpdateUser(project, instance, host, name, user)
	}
	return nil, nil
}

func (m *mockCloudSQLClient) GetOperation(project string, operationID string) (*sqladmin.Operation, error) {
	if m.MockGetOperation != nil {
		return m.MockGetOperation(project, operationID)
	}
	return nil, nil
}

func getInstanceDefault(project string, instance string) (*sqladmin.DatabaseInstance, error) {
	return createMockDatabaseInstance(project, instance, dbv1alpha1.StateRunnable), nil
}

func createInstanceDefault(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error) {
	return &sqladmin.Operation{}, nil
}

func deleteInstanceDefault(project string, instance string) (*sqladmin.Operation, error) {
	return &sqladmin.Operation{}, nil
}

func listUsersDefault(project string, instance string) (*sqladmin.UsersListResponse, error) {
	return &sqladmin.UsersListResponse{Items: []*sqladmin.User{{Name: "root"}}}, nil
}

func updateUserDefault(project string, instance string, host string, name string, user *sqladmin.User) (*sqladmin.Operation, error) {
	return &sqladmin.Operation{Name: "updateuser-op-123", Status: "RUNNING"}, nil
}

func getOperationDefault(project string, operationID string) (*sqladmin.Operation, error) {
	return &sqladmin.Operation{Name: operationID, Status: "DONE", EndTime: time.Now().String()}, nil
}

// CreateMockDatabaseInstance creates a simple test instance of a CloudSQL database instance object
func createMockDatabaseInstance(project, instance, state string) *sqladmin.DatabaseInstance {
	return &sqladmin.DatabaseInstance{
		Name:           instance,
		ConnectionName: fmt.Sprintf("%s:us-west2:%s", project, instance),
		State:          state,
		SelfLink:       fmt.Sprintf("https://www.googleapis.com/sql/v1beta4/projects/%s/instances/%s", project, instance),
	}
}

type mockCloudSQLClientFactory struct {
	mockClient *mockCloudSQLClient
}

func (m *mockCloudSQLClientFactory) CreateAPIInstance(kubernetes.Interface, string, v1.SecretKeySelector) (gcpclients.CloudSQLAPI, error) {
	return m.mockClient, nil
}
