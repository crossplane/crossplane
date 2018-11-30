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

package ec2

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/ec2iface"
)

// Client defines EC2 EC2Client operations
type Client interface {
	GetDefaultVpcID() (*string, error)
	CreateSecurityGroup(vpcID string, groupName string, description string) (groupID *string, err error)
	GetSecurityGroup(groupName string, vpcID string) (*ec2.SecurityGroup, error)
	DeleteSecurityGroup(groupID string) error
	CreateIngress(groupID string, IpPermissions []ec2.IpPermission) error
	CreateEgress(groupID string, IpPermissions []ec2.IpPermission) error
	RevokeEgress(groupID string, IpPermissions []ec2.IpPermission) error
	RevokeIngress(groupID string, IpPermissions []ec2.IpPermission) error
}

// EC2Client implements EC2 Client
type EC2Client struct {
	ec2 ec2iface.EC2API
}

// NewClient
func NewClient(config *aws.Config) Client {
	return &EC2Client{ec2.New(*config)}
}

// GetDefaultVpcID
func (c *EC2Client) GetDefaultVpcID() (*string, error) {
	response, err := c.ec2.DescribeAccountAttributesRequest(&ec2.DescribeAccountAttributesInput{
		AttributeNames: []ec2.AccountAttributeName{ec2.AccountAttributeNameDefaultVpc},
	}).Send()
	if err != nil {
		return nil, err
	}

	for _, attr := range response.AccountAttributes {
		if *attr.AttributeName == string(ec2.AccountAttributeNameDefaultVpc) {
			if len(attr.AttributeValues) == 1 && attr.AttributeValues[0].AttributeValue != nil {
				vpcID := attr.AttributeValues[0].AttributeValue
				return vpcID, nil
			}
		}
	}

	return nil, fmt.Errorf("no default vpc found")
}

// CreateSecurityGroup
func (c *EC2Client) CreateSecurityGroup(vpcID string, groupName string, description string) (*string, error) {
	response, err := c.ec2.CreateSecurityGroupRequest(&ec2.CreateSecurityGroupInput{
		VpcId:       &vpcID,
		GroupName:   &groupName,
		Description: &description,
	}).Send()
	if err != nil {
		return nil, err
	}

	return response.GroupId, nil
}

func (c *EC2Client) GetSecurityGroup(groupName string, vpcID string) (*ec2.SecurityGroup, error) {
	vpcFilter := "vpc-id"
	groupNameFilter := "group-name"
	response, err := c.ec2.DescribeSecurityGroupsRequest(&ec2.DescribeSecurityGroupsInput{
		Filters: []ec2.Filter{
			{
				Name: &vpcFilter,
				Values: []string{vpcID},
			},
			{
				Name: &groupNameFilter,
				Values: []string{groupName},
			},
		},
	}).Send()
	if err != nil {
		return nil, err
	}

	if len(response.SecurityGroups) != 1 {
		return nil, fmt.Errorf("security group not found")
	}

	return &response.SecurityGroups[0], nil
}

// DeleteSecurityGroup
func (c *EC2Client) DeleteSecurityGroup(groupID string) error {
	_, err := c.ec2.DeleteSecurityGroupRequest(&ec2.DeleteSecurityGroupInput{
		GroupId: &groupID,
	}).Send()
	return err
}

// CreateIngress
func (c *EC2Client) CreateIngress(groupID string, IpPermissions []ec2.IpPermission) error {
	_, err := c.ec2.AuthorizeSecurityGroupIngressRequest(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       &groupID,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}

// CreateIngress
func (c *EC2Client) CreateEgress(groupID string, IpPermissions []ec2.IpPermission) error {
	_, err := c.ec2.AuthorizeSecurityGroupEgressRequest(&ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       &groupID,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}

// RevokeIngress
func (c *EC2Client) RevokeIngress(groupID string, IpPermissions []ec2.IpPermission) error {
	_, err := c.ec2.RevokeSecurityGroupIngressRequest(&ec2.RevokeSecurityGroupIngressInput{
		GroupId:       &groupID,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}

// CreateEgress
func (c *EC2Client) RevokeEgress(groupID string, IpPermissions []ec2.IpPermission) error {
	_, err := c.ec2.RevokeSecurityGroupEgressRequest(&ec2.RevokeSecurityGroupEgressInput{
		GroupId:       &groupID,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}

// IsErrorAlreadyExists helper function
func IsErrorSecurityGroupAlreadyExists(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "InvalidGroup.Duplicate" {
		return true
	}
	return false
}
