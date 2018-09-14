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

package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type RDSAPI interface {
	DescribeDBInstances(*rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error)
	CreateDBInstance(*rds.CreateDBInstanceInput) (*rds.CreateDBInstanceOutput, error)
	WaitUntilDBInstanceAvailable(*rds.DescribeDBInstancesInput) error
	DescribeDBSecurityGroups(*rds.DescribeDBSecurityGroupsInput) (*rds.DescribeDBSecurityGroupsOutput, error)
	CreateDBSubnetGroup(*rds.CreateDBSubnetGroupInput) (*rds.CreateDBSubnetGroupOutput, error)
}

type RDSClient struct {
	*rds.RDS
}

func NewRDSClient(rdsClient *rds.RDS) *RDSClient {
	return &RDSClient{rdsClient}
}

func (r *RDSClient) DescribeDBInstances(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	return r.DescribeDBInstancesRequest(input).Send()
}

func (r *RDSClient) CreateDBInstance(input *rds.CreateDBInstanceInput) (*rds.CreateDBInstanceOutput, error) {
	return r.CreateDBInstanceRequest(input).Send()
}

func (r *RDSClient) DescribeDBSecurityGroups(input *rds.DescribeDBSecurityGroupsInput) (*rds.DescribeDBSecurityGroupsOutput, error) {
	return r.DescribeDBSecurityGroupsRequest(input).Send()
}

func (r *RDSClient) CreateDBSubnetGroup(input *rds.CreateDBSubnetGroupInput) (*rds.CreateDBSubnetGroupOutput, error) {
	return r.CreateDBSubnetGroupRequest(input).Send()
}
