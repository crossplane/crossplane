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
var _ clientset.RouteTableClient = (*MockRouteTableClient)(nil)

// MockRouteTableClient is a type that implements all the methods for RouteTableClient interface
type MockRouteTableClient struct {
	MockCreateRouteTableRequest       func(*ec2.CreateRouteTableInput) ec2.CreateRouteTableRequest
	MockDeleteRouteTableRequest       func(*ec2.DeleteRouteTableInput) ec2.DeleteRouteTableRequest
	MockDescribeRouteTablesRequest    func(*ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest
	MockCreateRouteRequest            func(*ec2.CreateRouteInput) ec2.CreateRouteRequest
	MockDeleteRouteRequest            func(*ec2.DeleteRouteInput) ec2.DeleteRouteRequest
	MockAssociateRouteTableRequest    func(*ec2.AssociateRouteTableInput) ec2.AssociateRouteTableRequest
	MockDisassociateRouteTableRequest func(*ec2.DisassociateRouteTableInput) ec2.DisassociateRouteTableRequest
}

// CreateRouteTableRequest mocks CreateRouteTableRequest method
func (m *MockRouteTableClient) CreateRouteTableRequest(input *ec2.CreateRouteTableInput) ec2.CreateRouteTableRequest {
	return m.MockCreateRouteTableRequest(input)
}

// DeleteRouteTableRequest mocks DeleteRouteTableRequest method
func (m *MockRouteTableClient) DeleteRouteTableRequest(input *ec2.DeleteRouteTableInput) ec2.DeleteRouteTableRequest {
	return m.MockDeleteRouteTableRequest(input)
}

// DescribeRouteTablesRequest mocks DescribeRouteTablesRequest method
func (m *MockRouteTableClient) DescribeRouteTablesRequest(input *ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest {
	return m.MockDescribeRouteTablesRequest(input)
}

// AssociateRouteTableRequest mocks AssociateRouteTableRequest method
func (m *MockRouteTableClient) AssociateRouteTableRequest(input *ec2.AssociateRouteTableInput) ec2.AssociateRouteTableRequest {
	return m.MockAssociateRouteTableRequest(input)
}

// DisassociateRouteTableRequest mocks DisassociateRouteTableRequest method
func (m *MockRouteTableClient) DisassociateRouteTableRequest(input *ec2.DisassociateRouteTableInput) ec2.DisassociateRouteTableRequest {
	return m.MockDisassociateRouteTableRequest(input)
}

// CreateRouteRequest mocks CreateRouteRequest method
func (m *MockRouteTableClient) CreateRouteRequest(input *ec2.CreateRouteInput) ec2.CreateRouteRequest {
	return m.MockCreateRouteRequest(input)
}

// DeleteRouteRequest mocks DeleteRouteRequest method
func (m *MockRouteTableClient) DeleteRouteRequest(input *ec2.DeleteRouteInput) ec2.DeleteRouteRequest {
	return m.MockDeleteRouteRequest(input)
}
