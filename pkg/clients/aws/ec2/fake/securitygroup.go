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
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	clientset "github.com/crossplaneio/crossplane/pkg/clients/aws/ec2"
)

// this ensures that the mock implements the client interface
var _ clientset.SecurityGroupClient = (*MockSecurityGroupClient)(nil)

// MockSecurityGroupClient is a type that implements all the methods for SecurityGroupClient interface
type MockSecurityGroupClient struct {
	MockCreateSecurityGroupRequest           func(*ec2.CreateSecurityGroupInput) ec2.CreateSecurityGroupRequest
	MockDeleteSecurityGroupRequest           func(*ec2.DeleteSecurityGroupInput) ec2.DeleteSecurityGroupRequest
	MockDescribeSecurityGroupsRequest        func(*ec2.DescribeSecurityGroupsInput) ec2.DescribeSecurityGroupsRequest
	MockAuthorizeSecurityGroupIngressRequest func(*ec2.AuthorizeSecurityGroupIngressInput) ec2.AuthorizeSecurityGroupIngressRequest
	MockAuthorizeSecurityGroupEgressRequest  func(*ec2.AuthorizeSecurityGroupEgressInput) ec2.AuthorizeSecurityGroupEgressRequest
}

// CreateSecurityGroupRequest mocks CreateSecurityGroupRequest method
func (m *MockSecurityGroupClient) CreateSecurityGroupRequest(input *ec2.CreateSecurityGroupInput) ec2.CreateSecurityGroupRequest {
	return m.MockCreateSecurityGroupRequest(input)
}

// DeleteSecurityGroupRequest mocks DeleteSecurityGroupRequest method
func (m *MockSecurityGroupClient) DeleteSecurityGroupRequest(input *ec2.DeleteSecurityGroupInput) ec2.DeleteSecurityGroupRequest {
	return m.MockDeleteSecurityGroupRequest(input)
}

// DescribeSecurityGroupsRequest mocks DescribeSecurityGroupsRequest method
func (m *MockSecurityGroupClient) DescribeSecurityGroupsRequest(input *ec2.DescribeSecurityGroupsInput) ec2.DescribeSecurityGroupsRequest {
	return m.MockDescribeSecurityGroupsRequest(input)
}

// AuthorizeSecurityGroupIngressRequest mocks AuthorizeSecurityGroupIngressRequest method
func (m *MockSecurityGroupClient) AuthorizeSecurityGroupIngressRequest(input *ec2.AuthorizeSecurityGroupIngressInput) ec2.AuthorizeSecurityGroupIngressRequest {
	return m.MockAuthorizeSecurityGroupIngressRequest(input)
}

// AuthorizeSecurityGroupEgressRequest mocks AuthorizeSecurityGroupEgressRequest method
func (m *MockSecurityGroupClient) AuthorizeSecurityGroupEgressRequest(input *ec2.AuthorizeSecurityGroupEgressInput) ec2.AuthorizeSecurityGroupEgressRequest {
	return m.MockAuthorizeSecurityGroupEgressRequest(input)
}
