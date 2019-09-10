package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	// LocalGatewayID is the id for local gateway
	LocalGatewayID = "local"

	// RouteTableIDNotFound is the code that is returned by ec2 when the given SubnetID is invalid
	RouteTableIDNotFound = "InvalidRouteTableID.NotFound"

	// RouteNotFound is the code that is returned when the given route is not found
	RouteNotFound = "InvalidRoute.NotFound"

	// AssociationIDNotFound is the code that is returned when then given AssociationID is invalid
	AssociationIDNotFound = "InvalidAssociationID.NotFound"
)

// RouteTableClient is the external client used for RouteTable Custom Resource
type RouteTableClient interface {
	CreateRouteTableRequest(*ec2.CreateRouteTableInput) ec2.CreateRouteTableRequest
	DeleteRouteTableRequest(*ec2.DeleteRouteTableInput) ec2.DeleteRouteTableRequest
	DescribeRouteTablesRequest(*ec2.DescribeRouteTablesInput) ec2.DescribeRouteTablesRequest

	CreateRouteRequest(*ec2.CreateRouteInput) ec2.CreateRouteRequest
	DeleteRouteRequest(*ec2.DeleteRouteInput) ec2.DeleteRouteRequest

	AssociateRouteTableRequest(*ec2.AssociateRouteTableInput) ec2.AssociateRouteTableRequest
	DisassociateRouteTableRequest(*ec2.DisassociateRouteTableInput) ec2.DisassociateRouteTableRequest
}

// NewRouteTableClient returns a new client using AWS credentials as JSON encoded data.
func NewRouteTableClient(cfg *aws.Config) (RouteTableClient, error) {
	return ec2.New(*cfg), nil
}

// IsRouteTableNotFoundErr returns true if the error is because the route table doesn't exist
func IsRouteTableNotFoundErr(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == RouteTableIDNotFound {
			return true
		}
	}
	return false
}

// IsRouteNotFoundErr returns true if the error is because the route doesn't exist
func IsRouteNotFoundErr(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == RouteNotFound {
			return true
		}
	}
	return false
}

// IsAssociationIDNotFoundErr returns true if the error is because the association doesn't exist
func IsAssociationIDNotFoundErr(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == AssociationIDNotFound {
			return true
		}
	}
	return false
}
