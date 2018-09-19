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
	"github.com/aws/aws-sdk-go-v2/service/rds"
	awsclients "github.com/upbound/conductor/pkg/clients/aws"
)

// mockRDSClient provides a mock implementation of the RDSAPI interface for unit testing purposes.
type mockRDSClient struct {
	awsclients.RDSAPI
	MockDescribeDBInstances          func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error)
	MockCreateDBInstance             func(*rds.CreateDBInstanceInput) (*rds.CreateDBInstanceOutput, error)
	MockWaitUntilDBInstanceAvailable func(input *rds.DescribeDBInstancesInput) error
	MockDescribeDBSecurityGroups     func(*rds.DescribeDBSecurityGroupsInput) (*rds.DescribeDBSecurityGroupsOutput, error)
	MockCreateDBSubnetGroup          func(*rds.CreateDBSubnetGroupInput) (*rds.CreateDBSubnetGroupOutput, error)
}

// DescribeDBInstances describes the given requested RDS instances
func (m *mockRDSClient) DescribeDBInstances(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	if m.MockDescribeDBInstances != nil {
		return m.MockDescribeDBInstances(input)
	}
	return &rds.DescribeDBInstancesOutput{DBInstances: []rds.DBInstance{}}, nil
}

// CreateDBInstance creates the given requested RDS instance
func (m *mockRDSClient) CreateDBInstance(input *rds.CreateDBInstanceInput) (*rds.CreateDBInstanceOutput, error) {
	if m.MockCreateDBInstance != nil {
		return m.MockCreateDBInstance(input)
	}
	return &rds.CreateDBInstanceOutput{}, nil
}

// WaitUntilDBInstanceAvailable will wait until the requested RDS instance becomes available
func (m *mockRDSClient) WaitUntilDBInstanceAvailable(input *rds.DescribeDBInstancesInput) error {
	if m.MockWaitUntilDBInstanceAvailable != nil {
		return m.MockWaitUntilDBInstanceAvailable(input)
	}
	return nil
}

// DescribeDBSecurityGroups describes the given requested RDS instance security groups
func (m *mockRDSClient) DescribeDBSecurityGroups(input *rds.DescribeDBSecurityGroupsInput) (*rds.DescribeDBSecurityGroupsOutput, error) {
	if m.MockDescribeDBSecurityGroups != nil {
		return m.MockDescribeDBSecurityGroups(input)
	}
	return &rds.DescribeDBSecurityGroupsOutput{}, nil
}

// CreateDBSubnetGroup creates the given requested RDS instance subnet group
func (m *mockRDSClient) CreateDBSubnetGroup(input *rds.CreateDBSubnetGroupInput) (*rds.CreateDBSubnetGroupOutput, error) {
	if m.MockCreateDBSubnetGroup != nil {
		return m.MockCreateDBSubnetGroup(input)
	}
	return &rds.CreateDBSubnetGroupOutput{}, nil
}
