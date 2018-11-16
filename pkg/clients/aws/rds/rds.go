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

package rds

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/rdsiface"
	"github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
)

// Instance conductor representation of the to AWS DBInstance
type Instance struct {
	Name     string
	ARN      string
	Status   string
	Endpoint string
}

// CreateInstance returns new Instance structure
func NewInstance(instance *rds.DBInstance) *Instance {
	endpoint := ""
	if instance.Endpoint != nil {
		endpoint = aws.StringValue(instance.Endpoint.Address)
	}

	return &Instance{
		Name:     aws.StringValue(instance.DBInstanceIdentifier),
		ARN:      aws.StringValue(instance.DBInstanceArn),
		Status:   aws.StringValue(instance.DBInstanceStatus),
		Endpoint: endpoint,
	}
}

// Client defines RDS RDSClient operations
type Client interface {
	CreateInstance(string, string, *v1alpha1.RDSInstanceSpec) (*Instance, error)
	GetInstance(name string) (*Instance, error)
	DeleteInstance(name string) (*Instance, error)
}

// RDSClient implements RDS RDSClient
type RDSClient struct {
	rds rdsiface.RDSAPI
}

// NewClient creates new RDS RDSClient with provided AWS Configurations/Credentials
func NewClient(config *aws.Config) Client {
	return &RDSClient{rds.New(*config)}
}

// TranslateDatabaseEngineVersion returns a default database version for a given database engine, version combination,
// For example: (the actual return values may be different)
// - mysql, 5.7         - returns 5.7.23
// - mysql, 5.6         - returns 5.6.40
// - mysql, 5.6.35      - returns 5.6.35
// - postgres, 9.6      - returns 9.6.9
// - postgres, 9.6.1    - returns 9.6.1
// - postgres, 10       - returns 10.4
func (r *RDSClient) TranslateDatabaseEngineVersion(spec *v1alpha1.RDSInstanceSpec, defaultOnly bool) (*string, error) {
	input := rds.DescribeDBEngineVersionsInput{
		Engine:        aws.String(spec.Engine),
		EngineVersion: aws.String(spec.EngineVersion),
		DefaultOnly:   aws.Bool(defaultOnly),
	}
	output, err := r.rds.DescribeDBEngineVersionsRequest(&input).Send()
	if err != nil {
		return nil, err
	}
	if len(output.DBEngineVersions) == 0 {
		return nil, fmt.Errorf("error translating database engin version")
	} else if len(output.DBEngineVersions) > 1 {
		return r.TranslateDatabaseEngineVersion(spec, true)
	}
	return output.DBEngineVersions[0].EngineVersion, nil
}

// CreateInstance creates RDS Instance with provided Specification
func (r *RDSClient) CreateInstance(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error) {
	input := CreateDBInstanceInput(name, password, spec)

	// attempt to translate database engine version
	if version, err := r.TranslateDatabaseEngineVersion(spec, false); err != nil {
		return nil, err
	} else {
		input.EngineVersion = version
	}

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
		if IsErrNotFound(err) {
			return nil, nil
		}
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
		if IsErrNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return NewInstance(output.DBInstance), nil
}

// IsErrorNotFound helper function to test for ErrCodeDBInstanceNotFoundFault error
func IsErrNotFound(err error) bool {
	return strings.Contains(err.Error(), rds.ErrCodeDBInstanceNotFoundFault)
}

// CreateDBInstanceInput from RDSInstanceSpec
func CreateDBInstanceInput(name, password string, spec *v1alpha1.RDSInstanceSpec) *rds.CreateDBInstanceInput {
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
		PubliclyAccessible:    aws.Bool(true),
	}
}
