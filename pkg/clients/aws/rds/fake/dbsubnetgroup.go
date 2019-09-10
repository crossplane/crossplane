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
	"github.com/aws/aws-sdk-go-v2/service/rds"

	clientset "github.com/crossplaneio/crossplane/pkg/clients/aws/rds"
)

// this ensures that the mock implements the client interface
var _ clientset.DBSubnetGroupClient = (*MockDBSubnetGroupClient)(nil)

// MockDBSubnetGroupClient is a type that implements all the methods for DBSubnetGroupClient interface
type MockDBSubnetGroupClient struct {
	MockCreateDBSubnetGroupRequest    func(*rds.CreateDBSubnetGroupInput) rds.CreateDBSubnetGroupRequest
	MockDeleteDBSubnetGroupRequest    func(*rds.DeleteDBSubnetGroupInput) rds.DeleteDBSubnetGroupRequest
	MockDescribeDBSubnetGroupsRequest func(*rds.DescribeDBSubnetGroupsInput) rds.DescribeDBSubnetGroupsRequest
}

// CreateDBSubnetGroupRequest mocks CreateDBSubnetGroupRequest method
func (m *MockDBSubnetGroupClient) CreateDBSubnetGroupRequest(input *rds.CreateDBSubnetGroupInput) rds.CreateDBSubnetGroupRequest {
	return m.MockCreateDBSubnetGroupRequest(input)
}

// DeleteDBSubnetGroupRequest mocks DeleteDBSubnetGroupRequest method
func (m *MockDBSubnetGroupClient) DeleteDBSubnetGroupRequest(input *rds.DeleteDBSubnetGroupInput) rds.DeleteDBSubnetGroupRequest {
	return m.MockDeleteDBSubnetGroupRequest(input)
}

// DescribeDBSubnetGroupsRequest mocks DescribeDBSubnetGroupsRequest method
func (m *MockDBSubnetGroupClient) DescribeDBSubnetGroupsRequest(input *rds.DescribeDBSubnetGroupsInput) rds.DescribeDBSubnetGroupsRequest {
	return m.MockDescribeDBSubnetGroupsRequest(input)
}
