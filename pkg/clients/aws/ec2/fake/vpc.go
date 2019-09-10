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
var _ clientset.VPCClient = (*MockVPCClient)(nil)

// MockVPCClient is a type that implements all the methods for VPCClient interface
type MockVPCClient struct {
	MockCreateVpcRequest    func(*ec2.CreateVpcInput) ec2.CreateVpcRequest
	MockDeleteVpcRequest    func(*ec2.DeleteVpcInput) ec2.DeleteVpcRequest
	MockDescribeVpcsRequest func(*ec2.DescribeVpcsInput) ec2.DescribeVpcsRequest
}

// CreateVpcRequest mocks CreateVpcRequest method
func (m *MockVPCClient) CreateVpcRequest(input *ec2.CreateVpcInput) ec2.CreateVpcRequest {
	return m.MockCreateVpcRequest(input)
}

// DeleteVpcRequest mocks DeleteVpcRequest method
func (m *MockVPCClient) DeleteVpcRequest(input *ec2.DeleteVpcInput) ec2.DeleteVpcRequest {
	return m.MockDeleteVpcRequest(input)
}

// DescribeVpcsRequest mocks DescribeVpcsRequest method
func (m *MockVPCClient) DescribeVpcsRequest(input *ec2.DescribeVpcsInput) ec2.DescribeVpcsRequest {
	return m.MockDescribeVpcsRequest(input)
}
