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

package cloudformation

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfiface "github.com/aws/aws-sdk-go-v2/service/cloudformation/cloudformationiface"
)

// Client interface to perform CloudFormation operations
type Client interface {
	CreateStack(stackName *string, templateBody *string, parameters map[string]string) (stackID *string, err error)
	GetStack(stackID *string) (stack *cf.Stack, err error)
	DeleteStack(stackID *string) error
}

type cloudFormationClient struct {
	cloudformation cfiface.CloudFormationAPI
}

// NewClient return new instance of the crossplane client for a specific AWS configuration
func NewClient(config *aws.Config) Client {
	return &cloudFormationClient{cf.New(*config)}
}

// CreateStack - Creates a stack
func (c *cloudFormationClient) CreateStack(stackName *string, templateBody *string, parameters map[string]string) (stackID *string, err error) {
	cfParams := make([]cf.Parameter, 0)
	for k, v := range parameters {
		if v != "" {
			cfParams = append(cfParams, cf.Parameter{ParameterKey: aws.String(k), ParameterValue: aws.String(v)})
		}
	}

	createStackResponse, err := c.cloudformation.CreateStackRequest(&cf.CreateStackInput{Capabilities: []cf.Capability{cf.CapabilityCapabilityIam}, StackName: stackName, TemplateBody: templateBody, Parameters: cfParams}).Send()
	if err != nil {
		return nil, err
	}
	return createStackResponse.StackId, nil
}

// GetStack info
func (c *cloudFormationClient) GetStack(stackID *string) (stack *cf.Stack, err error) {
	describeStackResponse, err := c.cloudformation.DescribeStacksRequest(&cf.DescribeStacksInput{StackName: stackID}).Send()
	if err != nil {
		return nil, err
	}

	// If fetching by name, then this might be a list.
	// Since we're fetching by ID, it's either not found err above, or there's an item right here.
	if len(describeStackResponse.Stacks) == 0 {
		return nil, fmt.Errorf("stack unexpectedly not in response")
	}

	return &describeStackResponse.Stacks[0], nil
}

// DeleteStack deletes a stack
func (c *cloudFormationClient) DeleteStack(stackID *string) error {
	_, err := c.cloudformation.DeleteStackRequest(&cf.DeleteStackInput{StackName: stackID}).Send()
	return err
}

// IsErrorNotFound - not found error
func IsErrorNotFound(err error) bool {
	if cloudformationErr, ok := err.(awserr.Error); ok && cloudformationErr.Code() == cf.ErrCodeStackInstanceNotFoundException {
		return true
	}
	return false
}
