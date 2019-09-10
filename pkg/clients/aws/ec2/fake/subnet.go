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
var _ clientset.SubnetClient = (*MockSubnetClient)(nil)

// MockSubnetClient is a type that implements all the methods for SubnetClient interface
type MockSubnetClient struct {
	MockCreateSubnetRequest    func(*ec2.CreateSubnetInput) ec2.CreateSubnetRequest
	MockDeleteSubnetRequest    func(*ec2.DeleteSubnetInput) ec2.DeleteSubnetRequest
	MockDescribeSubnetsRequest func(*ec2.DescribeSubnetsInput) ec2.DescribeSubnetsRequest
}

// CreateSubnetRequest mocks CreateSubnetRequest method
func (m *MockSubnetClient) CreateSubnetRequest(input *ec2.CreateSubnetInput) ec2.CreateSubnetRequest {
	return m.MockCreateSubnetRequest(input)
}

// DeleteSubnetRequest mocks DeleteSubnetRequest method
func (m *MockSubnetClient) DeleteSubnetRequest(input *ec2.DeleteSubnetInput) ec2.DeleteSubnetRequest {
	return m.MockDeleteSubnetRequest(input)
}

// DescribeSubnetsRequest mocks DescribeSubnetsRequest method
func (m *MockSubnetClient) DescribeSubnetsRequest(input *ec2.DescribeSubnetsInput) ec2.DescribeSubnetsRequest {
	return m.MockDescribeSubnetsRequest(input)
}
