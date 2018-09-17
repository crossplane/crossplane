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
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	awsclients "github.com/upbound/conductor/pkg/clients/aws"
)

// EC2Client provides a mock implementation of the EC2API interface for unit testing purposes.
type EC2Client struct {
	awsclients.EC2API
	MockDescribeInstances func(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	MockDescribeSubnets   func(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
}

// DescribeInstances describes the given requested EC2 instances
func (m *EC2Client) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if m.MockDescribeInstances != nil {
		return m.MockDescribeInstances(input)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

// DescribeSubnets describes the given requested EC2 subnets
func (m *EC2Client) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	if m.MockDescribeSubnets != nil {
		return m.MockDescribeSubnets(input)
	}
	return &ec2.DescribeSubnetsOutput{}, nil
}
