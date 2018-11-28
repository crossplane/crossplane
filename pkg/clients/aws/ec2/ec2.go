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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/ec2iface"
)

// Client defines EC2 EC2Client operations
type Client interface {
	CreateSecurityGroup(vpcID string, groupName string, description string) (*string, error)
	GetSecurityGroups(groupIDs []string) ([]ec2.SecurityGroup, error)
	DeleteSecurityGroup(groupID string) error
	CreateIngress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error
	CreateEgress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error
	RevokeEgress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error
	RevokeIngress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error
}

// EC2Client implements EC2 Client
type EC2Client struct {
	ec2 ec2iface.EC2API
}

// NewClient
func NewClient(config *aws.Config) Client {
	return &EC2Client{ec2.New(*config)}
}

// CreateSecurityGroup
func (c *EC2Client) CreateSecurityGroup(vpcID string, groupName string, description string) (*string, error) {
	response, err := c.ec2.CreateSecurityGroupRequest(&ec2.CreateSecurityGroupInput{
		VpcId: &vpcID,
		GroupName: &groupName,
		Description: &description,
	}).Send()
	return response.GroupId, err
}

func (c *EC2Client) GetSecurityGroups(groupIDs []string) ([]ec2.SecurityGroup, error) {
	response, err := c.ec2.DescribeSecurityGroupsRequest(&ec2.DescribeSecurityGroupsInput{
		GroupIds: groupIDs,
	}).Send()
	if err != nil {
		return nil, err
	}
	return response.SecurityGroups, nil
}

// DeleteSecurityGroup
func (c *EC2Client) DeleteSecurityGroup(groupID string) error {
	_, err := c.ec2.DeleteSecurityGroupRequest(&ec2.DeleteSecurityGroupInput{
		GroupId: &groupID,
	}).Send()
	return err
}

// CreateIngress
func (c *EC2Client) CreateIngress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error {
	_, err := c.ec2.AuthorizeSecurityGroupIngressRequest(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: &groupID,
		SourceSecurityGroupName: sourceSecurityGroup,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}

// CreateIngress
func (c *EC2Client) CreateEgress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error {
	_, err := c.ec2.AuthorizeSecurityGroupEgressRequest(&ec2.AuthorizeSecurityGroupEgressInput{
		GroupId: &groupID,
		SourceSecurityGroupName: sourceSecurityGroup,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}

// RevokeIngress
func (c *EC2Client) RevokeIngress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error {
	_, err := c.ec2.RevokeSecurityGroupIngressRequest(&ec2.RevokeSecurityGroupIngressInput{
		GroupId: &groupID,
		SourceSecurityGroupName: sourceSecurityGroup,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}


// CreateEgress
func (c *EC2Client) RevokeEgress(groupID string, sourceSecurityGroup *string, IpPermissions []ec2.IpPermission) error {
	_, err :=c.ec2.RevokeSecurityGroupEgressRequest(&ec2.RevokeSecurityGroupEgressInput{
		GroupId: &groupID,
		SourceSecurityGroupName: sourceSecurityGroup,
		IpPermissions: IpPermissions,
	}).Send()
	return err
}
