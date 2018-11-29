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

package fake

import (
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/rds"
)

type MockRDSClient struct {
	MockGetVpcId                    func(spec *v1alpha1.RDSInstanceSpec) (*string, error)
	MockDescribeInstanceSubnetGroup func(string) (*awsrds.DBSubnetGroup, error)
	MockGetInstance                 func(string) (*rds.Instance, error)
	MockCreateInstance              func(string, string, *v1alpha1.RDSInstanceSpec) (*rds.Instance, error)
	MockDeleteInstance              func(name string) (*rds.Instance, error)
}

func (m *MockRDSClient) GetVpcId(spec *v1alpha1.RDSInstanceSpec) (*string, error) {
	return m.MockGetVpcId(spec)
}
func (m *MockRDSClient) DescribeInstanceSubnetGroup(name string) (*awsrds.DBSubnetGroup, error) {
	return m.MockDescribeInstanceSubnetGroup(name)
}

// GetInstance finds RDS Instance by name
func (m *MockRDSClient) GetInstance(name string) (*rds.Instance, error) {
	return m.MockGetInstance(name)
}

// CreateInstance creates RDS Instance with provided Specification
func (m *MockRDSClient) CreateInstance(name, password string, spec *v1alpha1.RDSInstanceSpec) (*rds.Instance, error) {
	return m.MockCreateInstance(name, password, spec)
}

// DeleteInstance deletes RDS Instance
func (m *MockRDSClient) DeleteInstance(name string) (*rds.Instance, error) {
	return m.MockDeleteInstance(name)
}
