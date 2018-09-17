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
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	k8sclients "github.com/upbound/conductor/pkg/clients/kubernetes"
	"k8s.io/client-go/kubernetes"
)

// EC2API defines an interface for the EC2 client
type EC2API interface {
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
}

// EC2Client implements the EC2API interface to perform operations on EC2 objects
type EC2Client struct {
	*ec2.EC2
}

// NewEC2Client creates a new instance of an EC2Client
func NewEC2Client(ec2Client *ec2.EC2) *EC2Client {
	return &EC2Client{ec2Client}
}

// NewEC2ClientFromClientset creates a new instance of an EC2Client from the given Kubernetes clientset
func NewEC2ClientFromClientset(clientset kubernetes.Interface) (*EC2Client, error) {
	log.Printf("getting ec2 client from clientset...")

	nodeInfo, err := k8sclients.GetFirstNodeInfo(clientset)
	if err != nil {
		return nil, err
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %+v", err)
	}

	// Set the AWS Region that the service clients should use
	cfg.Region = nodeInfo.Region
	cfg.HTTPClient.Timeout = 5 * time.Second
	return NewEC2Client(ec2.New(cfg)), nil
}

// DescribeInstances describes the given requested EC2 instances
func (e *EC2Client) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return e.DescribeInstancesRequest(input).Send()
}

// DescribeSubnets describes the given requested EC2 subnets
func (e *EC2Client) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return e.DescribeSubnetsRequest(input).Send()
}
