package rds

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rds/rdsiface"

	"github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type DBInstanceStatus int

const (
	// The instance is healthy and available
	DBInstanceStatusAvailable DBInstanceStatus = iota
	// The instance is being created. The instance is inaccessible while it is being created.
	DBInstanceStatusCreating
	// The instance is being deleted.
	DBInstanceStatusDeleting
	// The instance has failed and Amazon RDS can't recover it. Perform a point-in-time restore to the latest restorable time of the instance to recover the data.
	DBInstanceStatusFailed
)

func (s DBInstanceStatus) String() string {
	return [...]string{"available", "creating", "deleting", "failed"}[s]
}

// Instance conductor repsentation of the to AWS DBInstance
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

// Service defines RDS Client operations
type Service interface {
	CreateInstance(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error)
	GetInstance(name string) (*Instance, error)
	DeleteInstance(name string) (*Instance, error)
}

// Client implements RDS Client
type Client struct {
	rds rdsiface.RDSAPI
}

// NewClient creates new RDS Client with provided AWS Configurations/Credentials
func NewClient(config *aws.Config) Service {
	return &Client{rds.New(*config)}
}

// GetInstance finds RDS Instance by name
func (c *Client) GetInstance(name string) (*Instance, error) {
	input := &rds.DescribeDBInstancesInput{DBInstanceIdentifier: &name}
	output, err := c.rds.DescribeDBInstancesRequest(input).Send()
	if err != nil {
		if IsErrNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	outputCount := len(output.DBInstances)
	if outputCount == 0 || outputCount > 1 {
		return nil, nil // TODO: or maybe error
	}

	return NewInstance(&output.DBInstances[0]), nil
}

// CreateInstance creates RDS Instance with provided Specification
func (c *Client) CreateInstance(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error) {
	input := CreateDBInstanceInput(name, password, spec)
	output, err := c.rds.CreateDBInstanceRequest(input).Send()
	if err != nil {
		return nil, err
	}
	return NewInstance(output.DBInstance), nil
}

// DeleteInstance deletes RDS Instance
func (c *Client) DeleteInstance(name string) (*Instance, error) {
	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: &name,
		SkipFinalSnapshot:    aws.Bool(true),
	}
	output, err := c.rds.DeleteDBInstanceRequest(input).Send()
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
		MasterUsername:        aws.String(spec.MasterUsername),
		MasterUserPassword:    aws.String(password),
		BackupRetentionPeriod: aws.Int64(0),
		VpcSecurityGroupIds:   spec.SecurityGroups,
		PubliclyAccessible:    aws.Bool(true),
	}
}

// ConditionType based on DBInstance status
func ConditionType(status string) v1alpha1.RDSInstanceConditionType {
	switch status {
	case DBInstanceStatusAvailable.String():
		return v1alpha1.Running
	case DBInstanceStatusCreating.String():
		return v1alpha1.Creating
	case DBInstanceStatusDeleting.String():
		return v1alpha1.Deleting
	case DBInstanceStatusFailed.String():
		return v1alpha1.Failed
	default:
		return v1alpha1.Pending
	}
}
