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

package rds

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/ec2"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/rdsiface"
	"github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
)

// Instance crossplane representation of the to AWS DBInstance
type Instance struct {
	Name     string
	ARN      string
	Status   string
	Endpoint string
	VpcID    string
}

const (
	defaultSubgroupName = "default"
)

// NewInstance returns new Instance structure
func NewInstance(instance *rds.DBInstance) *Instance {
	endpoint := ""
	if instance.Endpoint != nil {
		endpoint = aws.StringValue(instance.Endpoint.Address)
	}

	VpcID := ""
	if instance.DBSubnetGroup != nil {
		VpcID = aws.StringValue(instance.DBSubnetGroup.VpcId)
	}

	return &Instance{
		Name:     aws.StringValue(instance.DBInstanceIdentifier),
		ARN:      aws.StringValue(instance.DBInstanceArn),
		Status:   aws.StringValue(instance.DBInstanceStatus),
		Endpoint: endpoint,
		VpcID:    VpcID,
	}
}

// Client defines RDS RDSClient operations
type Client interface {
	GetVpcId(spec *v1alpha1.RDSInstanceSpec) (*string, error)
	CreateInstance(string, string, *v1alpha1.RDSInstanceSpec) (*Instance, error)
	DescribeInstanceSubnetGroup(name string) (*rds.DBSubnetGroup, error)
	GetInstance(name string) (*Instance, error)
	DeleteInstance(name string) (*Instance, error)
}

// RDSClient implements RDS RDSClient
type RDSClient struct {
	rds rdsiface.RDSAPI
	ec2 ec2.Client
}

// NewClient creates new RDS RDSClient with provided AWS Configurations/Credentials
func NewClient(config *aws.Config) Client {
	return &RDSClient{rds.New(*config), ec2.NewClient(config)}
}

// GetVpcId - Detect the VPC ID where an RDS instance will be provisioned based on
// 1. User provided DBSubnetGroup's VPC
// 2. Lookup DBSubnetGroup of name default and check it's VPC
// 3. Lookup default vpc of this account.
func (r *RDSClient) GetVpcId(spec *v1alpha1.RDSInstanceSpec) (*string, error) {
	// Create default security groups
	//
	// Detect VPC where database will be created.
	var vpcID *string
	if spec.DBSubnetGroupName != "" {
		response, err := r.DescribeInstanceSubnetGroup(spec.DBSubnetGroupName)
		if err != nil {
			return nil, err
		}
		vpcID = response.VpcId
	} else {
		response, err := r.DescribeInstanceSubnetGroup(defaultSubgroupName)
		if err != nil {
			if !IsErrorDBSubnetNotFound(err) {
				return nil, err
			}
			// try to fetch default vpc
			vpcID, err = r.ec2.GetDefaultVpcID()
			if err != nil {
				return nil, err
			}

		}
		vpcID = response.VpcId
	}
	return vpcID, nil
}

// DescribeInstanceSubnetGroup get details about a dbsubnet group
func (r *RDSClient) DescribeInstanceSubnetGroup(name string) (*rds.DBSubnetGroup, error) {
	output, err := r.rds.DescribeDBSubnetGroupsRequest(&rds.DescribeDBSubnetGroupsInput{
		DBSubnetGroupName: aws.String(name),
	}).Send()

	if err != nil {
		return nil, err
	}

	if len(output.DBSubnetGroups) == 0 || len(output.DBSubnetGroups) > 1 {
		return nil, fmt.Errorf("unexpected response")
	}

	return &output.DBSubnetGroups[0], nil
}

// CreateInstance creates RDS Instance with provided Specification
func (r *RDSClient) CreateInstance(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error) {
	vpcID, err := r.GetVpcId(spec)
	if err != nil {
		return nil, err
	}

	spec.vpcID = vpcID

	if len(spec.SecurityGroups) == 0 {

		groupID, err := r.ec2.CreateSecurityGroup(*vpcID, name, "Default crossplane security group for RDS Database")
		if err != nil && !ec2.IsErrorSecurityGroupAlreadyExists(err) {
			return nil, err
		}
		spec.SecurityGroups = append(spec.SecurityGroups, aws.StringValue(groupID))
	}

	input := CreateDBInstanceInput(name, password, spec)

	output, err := r.rds.CreateDBInstanceRequest(input).Send()
	if err != nil {
		return nil, err
	}
	return NewInstance(output.DBInstance), nil
}

// GetInstance finds RDS Instance by name
func (r *RDSClient) GetInstance(name string) (*Instance, error) {
	input := rds.DescribeDBInstancesInput{DBInstanceIdentifier: &name}
	output, err := r.rds.DescribeDBInstancesRequest(&input).Send()
	if err != nil {
		return nil, err
	}

	outputCount := len(output.DBInstances)
	if outputCount == 0 || outputCount > 1 {
		return nil, fmt.Errorf("rds instance [%s] is not found", name)
	}

	return NewInstance(&output.DBInstances[0]), nil
}

// DeleteInstance deletes RDS Instance
func (r *RDSClient) DeleteInstance(name string) (*Instance, error) {
	input := rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: &name,
		SkipFinalSnapshot:    aws.Bool(true),
	}
	output, err := r.rds.DeleteDBInstanceRequest(&input).Send()
	if err != nil {
		return nil, err
	}

	return NewInstance(output.DBInstance), nil
}

func IsErrorAlreadyExists(err error) bool {
	return strings.Contains(err.Error(), rds.ErrCodeDBClusterAlreadyExistsFault)
}

// IsErrorNotFound helper function to test for ErrCodeDBInstanceNotFoundFault error
func IsErrorNotFound(err error) bool {
	return strings.Contains(err.Error(), rds.ErrCodeDBInstanceNotFoundFault)
}

func IsErrorDBSubnetNotFound(err error) bool {
	if cloudformationErr, ok := err.(awserr.Error); ok && cloudformationErr.Code() == rds.ErrCodeDBSubnetGroupNotFoundFault {
		return true
	}
	return false
}

// CreateDBInstanceInput from RDSInstanceSpec
func CreateDBInstanceInput(name, password string, spec *v1alpha1.RDSInstanceSpec) *rds.CreateDBInstanceInput {
	publicallyAccessible := aws.Bool(true)
	if spec.PubliclyAccessible != nil {
		publicallyAccessible = spec.PubliclyAccessible
	}

	var dbSubnetGroupName *string
	if spec.DBSubnetGroupName != "" {
		dbSubnetGroupName = &spec.DBSubnetGroupName
	}

	return &rds.CreateDBInstanceInput{
		DBInstanceIdentifier:  aws.String(name),
		AllocatedStorage:      aws.Int64(spec.Size),
		DBInstanceClass:       aws.String(spec.Class),
		Engine:                aws.String(spec.Engine),
		EngineVersion:         aws.String(spec.EngineVersion),
		MasterUsername:        aws.String(spec.MasterUsername),
		MasterUserPassword:    aws.String(password),
		BackupRetentionPeriod: aws.Int64(0),
		VpcSecurityGroupIds:   spec.SecurityGroups,
		PubliclyAccessible:    publicallyAccessible,
		DBSubnetGroupName:     dbSubnetGroupName,
	}
}
