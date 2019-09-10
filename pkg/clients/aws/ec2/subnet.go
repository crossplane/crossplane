package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	// SubnetIDNotFound is the code that is returned by ec2 when the given SubnetID is not valid
	SubnetIDNotFound = "InvalidSubnetID.NotFound"
)

// SubnetClient is the external client used for Subnet Custom Resource
type SubnetClient interface {
	CreateSubnetRequest(input *ec2.CreateSubnetInput) ec2.CreateSubnetRequest
	DescribeSubnetsRequest(input *ec2.DescribeSubnetsInput) ec2.DescribeSubnetsRequest
	DeleteSubnetRequest(input *ec2.DeleteSubnetInput) ec2.DeleteSubnetRequest
}

// NewSubnetClient returns a new client using AWS credentials as JSON encoded data.
func NewSubnetClient(cfg *aws.Config) (SubnetClient, error) {
	return ec2.New(*cfg), nil
}

// IsSubnetNotFoundErr returns true if the error is because the item doesn't exist
func IsSubnetNotFoundErr(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == SubnetIDNotFound {
			return true
		}
	}

	return false
}
