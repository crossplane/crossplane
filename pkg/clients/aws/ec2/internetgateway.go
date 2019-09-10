package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	// InternetGatewayIDNotFound is the code that is returned by ec2 when the given SubnetID is not valid
	InternetGatewayIDNotFound = "InvalidInternetGatewayID.NotFound"
)

// InternetGatewayClient is the external client used for InternetGateway Custom Resource
type InternetGatewayClient interface {
	CreateInternetGatewayRequest(input *ec2.CreateInternetGatewayInput) ec2.CreateInternetGatewayRequest
	DeleteInternetGatewayRequest(input *ec2.DeleteInternetGatewayInput) ec2.DeleteInternetGatewayRequest
	DescribeInternetGatewaysRequest(input *ec2.DescribeInternetGatewaysInput) ec2.DescribeInternetGatewaysRequest
	AttachInternetGatewayRequest(input *ec2.AttachInternetGatewayInput) ec2.AttachInternetGatewayRequest
	DetachInternetGatewayRequest(input *ec2.DetachInternetGatewayInput) ec2.DetachInternetGatewayRequest
}

// NewInternetGatewayClient returns a new client using AWS credentials as JSON encoded data.
func NewInternetGatewayClient(cfg *aws.Config) (InternetGatewayClient, error) {
	return ec2.New(*cfg), nil
}

// IsInternetGatewayNotFoundErr returns true if the error is because the item doesn't exist
func IsInternetGatewayNotFoundErr(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == InternetGatewayIDNotFound {
			return true
		}
	}

	return false
}
