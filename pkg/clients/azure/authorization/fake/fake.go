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

	authorizationmgmt "github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization/authorizationapi"
)

var _ authorizationapi.RoleAssignmentsClientAPI = &MockRoleAssignmentsClient{}

// MockRoleAssignmentsClient is a fake implementation of network.RoleAssignmentsClient.
type MockRoleAssignmentsClient struct {
	MockCreate               func(ctx context.Context, scope string, roleAssignmentName string, parameters authorizationmgmt.RoleAssignmentCreateParameters) (result authorizationmgmt.RoleAssignment, err error)
	MockCreateByID           func(ctx context.Context, roleAssignmentID string, parameters authorizationmgmt.RoleAssignmentCreateParameters) (result authorizationmgmt.RoleAssignment, err error)
	MockDelete               func(ctx context.Context, scope string, roleAssignmentName string) (result authorizationmgmt.RoleAssignment, err error)
	MockDeleteByID           func(ctx context.Context, roleAssignmentID string) (result authorizationmgmt.RoleAssignment, err error)
	MockGet                  func(ctx context.Context, scope string, roleAssignmentName string) (result authorizationmgmt.RoleAssignment, err error)
	MockGetByID              func(ctx context.Context, roleAssignmentID string) (result authorizationmgmt.RoleAssignment, err error)
	MockList                 func(ctx context.Context, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error)
	MockListForResource      func(ctx context.Context, resourceGroupName string, resourceProviderNamespace string, parentResourcePath string, resourceType string, resourceName string, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error)
	MockListForResourceGroup func(ctx context.Context, resourceGroupName string, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error)
	MockListForScope         func(ctx context.Context, scope string, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error)
}

// Create calls the MockRoleAssignmentsClient's MockCreate method.
func (c *MockRoleAssignmentsClient) Create(ctx context.Context, scope string, roleAssignmentName string, parameters authorizationmgmt.RoleAssignmentCreateParameters) (result authorizationmgmt.RoleAssignment, err error) {
	return c.MockCreate(ctx, scope, roleAssignmentName, parameters)
}

// CreateByID calls the MockRoleAssignmentsClient's MockCreateByID method.
func (c *MockRoleAssignmentsClient) CreateByID(ctx context.Context, roleAssignmentID string, parameters authorizationmgmt.RoleAssignmentCreateParameters) (result authorizationmgmt.RoleAssignment, err error) {
	return c.MockCreateByID(ctx, roleAssignmentID, parameters)
}

// Delete calls the MockRoleAssignmentsClient's MockDelete method.
func (c *MockRoleAssignmentsClient) Delete(ctx context.Context, scope string, roleAssignmentName string) (result authorizationmgmt.RoleAssignment, err error) {
	return c.MockDelete(ctx, scope, roleAssignmentName)
}

// DeleteByID calls the MockRoleAssignmentsClient's MockDeleteByID method.
func (c *MockRoleAssignmentsClient) DeleteByID(ctx context.Context, roleAssignmentID string) (result authorizationmgmt.RoleAssignment, err error) {
	return c.MockDeleteByID(ctx, roleAssignmentID)
}

// Get calls the MockRoleAssignmentsClient's MockGet method.
func (c *MockRoleAssignmentsClient) Get(ctx context.Context, scope string, roleAssignmentName string) (result authorizationmgmt.RoleAssignment, err error) {
	return c.MockGet(ctx, scope, roleAssignmentName)
}

// GetByID calls the MockRoleAssignmentsClient's MockGetByID method.
func (c *MockRoleAssignmentsClient) GetByID(ctx context.Context, roleAssignmentID string) (result authorizationmgmt.RoleAssignment, err error) {
	return c.MockGetByID(ctx, roleAssignmentID)
}

// List calls the MockRoleAssignmentsClient's MockList method.
func (c *MockRoleAssignmentsClient) List(ctx context.Context, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error) {
	return c.MockList(ctx, filter)
}

// ListForResource calls the MockRoleAssignmentsClient's MockListForResource method.
func (c *MockRoleAssignmentsClient) ListForResource(ctx context.Context, resourceGroupName string, resourceProviderNamespace string, parentResourcePath string, resourceType string, resourceName string, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error) {
	return c.MockListForResource(ctx, resourceGroupName, resourceProviderNamespace, parentResourcePath, resourceType, resourceName, filter)
}

// ListForResourceGroup calls the MockRoleAssignmentsClient's MockListForResourceGroup method.
func (c *MockRoleAssignmentsClient) ListForResourceGroup(ctx context.Context, resourceGroupName string, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error) {
	return c.MockListForResourceGroup(ctx, resourceGroupName, filter)
}

// ListForScope calls the MockRoleAssignmentsClient's MockListForScope method.
func (c *MockRoleAssignmentsClient) ListForScope(ctx context.Context, scope string, filter string) (result authorizationmgmt.RoleAssignmentListResultPage, err error) {
	return c.MockListForScope(ctx, scope, filter)
}
