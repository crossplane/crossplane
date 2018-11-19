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

package eks

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/eksiface"
	awscomputev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/compute/v1alpha1"
)

const (
	clusterIDHeader = "x-k8s-aws-id"
	v1Prefix        = "k8s-aws-v1."
)

// Cluster conductor representation of the to AWS EKS Cluster
type Cluster struct {
	Name     string
	ARN      string
	Status   string
	Endpoint string
	CA       string
}

func NewCluster(c *eks.Cluster) *Cluster {
	return &Cluster{
		Name:     aws.StringValue(c.Name),
		ARN:      aws.StringValue(c.Arn),
		Status:   string(c.Status),
		Endpoint: aws.StringValue(c.Endpoint),
		CA:       c.CertificateAuthority.GoString(),
	}
}

// Client interface to perform cluster operations
type Client interface {
	Create(string, awscomputev1alpha1.EKSClusterSpec) (*Cluster, error)
	Get(string) (*Cluster, error)
	Delete(string) error
	ConnectionToken(string) (string, error)
}

// EKSClient conductor eks client
type EKSClient struct {
	eks eksiface.EKSAPI
	sts *sts.STS
}

// NewClient return new instance of the conductor client for a specific AWS configuration
func NewClient(config *aws.Config) Client {
	return &EKSClient{eks.New(*config), sts.New(*config)}
}

// Create new EKS cluster
func (e *EKSClient) Create(name string, spec awscomputev1alpha1.EKSClusterSpec) (*Cluster, error) {
	input := &eks.CreateClusterInput{
		Name:    aws.String(name),
		Version: aws.String(spec.ClusterVersion),
		RoleArn: aws.String(spec.RoleARN),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			SubnetIds:        spec.SubnetIds,
			SecurityGroupIds: spec.SecurityGroupsIds,
		},
	}
	output, err := e.eks.CreateClusterRequest(input).Send()
	if err != nil {
		return nil, err
	}
	return NewCluster(output.Cluster), nil
}

// Get an existing EKS cluster
func (e *EKSClient) Get(name string) (*Cluster, error) {
	input := &eks.DescribeClusterInput{Name: aws.String(name)}
	output, err := e.eks.DescribeClusterRequest(input).Send()
	if err != nil {
		return nil, err
	}
	return NewCluster(output.Cluster), err
}

// Delete a EKS cluster
func (e *EKSClient) Delete(name string) error {
	input := &eks.DeleteClusterInput{Name: aws.String(name)}
	_, err := e.eks.DeleteClusterRequest(input).Send()
	return err
}

// ConnectionToken to a cluster
func (e *EKSClient) ConnectionToken(name string) (string, error) {
	request := e.sts.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	request.HTTPRequest.Header.Add(clusterIDHeader, name)

	// sign the request
	presignedURLString, err := request.Presign(60 * time.Second)
	if err != nil {
		return "", err
	}

	return v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLString)), nil
}

// IsErrorNotFound helper function
func IsErrorAlreadyExists(err error) bool {
	return strings.Contains(err.Error(), eks.ErrCodeResourceInUseException)
}

// IsErrorNotFound helper function
func IsErrorBadRequest(err error) bool {
	return strings.Contains(err.Error(), eks.ErrCodeInvalidParameterException) ||
		strings.Contains(err.Error(), eks.ErrCodeUnsupportedAvailabilityZoneException)
}

// IsErrorNotFound helper function
func IsErrorNotFound(err error) bool {
	return strings.Contains(err.Error(), eks.ErrCodeResourceNotFoundException)
}
