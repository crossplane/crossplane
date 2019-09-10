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
	"github.com/aws/aws-sdk-go-v2/service/iam"

	clientset "github.com/crossplaneio/crossplane/pkg/clients/aws/iam"
)

// this ensures that the mock implements the client interface
var _ clientset.RoleClient = (*MockRoleClient)(nil)

// MockRoleClient is a type that implements all the methods for RoleClient interface
type MockRoleClient struct {
	MockGetRoleRequest    func(*iam.GetRoleInput) iam.GetRoleRequest
	MockCreateRoleRequest func(*iam.CreateRoleInput) iam.CreateRoleRequest
	MockDeleteRoleRequest func(*iam.DeleteRoleInput) iam.DeleteRoleRequest
}

// GetRoleRequest mocks GetRoleRequest method
func (m *MockRoleClient) GetRoleRequest(input *iam.GetRoleInput) iam.GetRoleRequest {
	return m.MockGetRoleRequest(input)
}

// CreateRoleRequest mocks CreateRoleRequest method
func (m *MockRoleClient) CreateRoleRequest(input *iam.CreateRoleInput) iam.CreateRoleRequest {
	return m.MockCreateRoleRequest(input)
}

// DeleteRoleRequest mocks DeleteRoleRequest method
func (m *MockRoleClient) DeleteRoleRequest(input *iam.DeleteRoleInput) iam.DeleteRoleRequest {
	return m.MockDeleteRoleRequest(input)
}
