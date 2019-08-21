/*
Copyright 2019 The Crossplane Authors.

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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/eksiface"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	awscomputev1alpha1 "github.com/crossplaneio/crossplane/aws/apis/compute/v1alpha1"
	cfc "github.com/crossplaneio/crossplane/pkg/clients/aws/cloudformation"
)

const (
	clusterIDHeader                = "x-k8s-aws-id"
	v1Prefix                       = "k8s-aws-v1."
	cloudFormationNodeInstanceRole = "NodeInstanceRole"
)

// Cluster crossplane representation of the AWS EKS Cluster
type Cluster struct {
	Name     string
	Version  string
	ARN      string
	Status   string
	Endpoint string
	CA       string
}

// NewCluster returns crossplane representation AWS EKS cluster
func NewCluster(c *eks.Cluster) *Cluster {
	cluster := &Cluster{
		Name:     aws.StringValue(c.Name),
		Version:  aws.StringValue(c.Version),
		ARN:      aws.StringValue(c.Arn),
		Status:   string(c.Status),
		Endpoint: aws.StringValue(c.Endpoint),
	}

	if c.CertificateAuthority != nil {
		cluster.CA = aws.StringValue(c.CertificateAuthority.Data)
	}

	return cluster
}

// ClusterWorkers crossplane representation of the AWS EKS cluster worker nodes
type ClusterWorkers struct {
	WorkersStatus cloudformation.StackStatus
	WorkerReason  string
	WorkerStackID string
	WorkerARN     string
}

// NewClusterWorkers returns crossplane representation of the AWS EKS cluster worker nodes
func NewClusterWorkers(workerStackID string, workerStatus cloudformation.StackStatus, workerReason string, workerARN string) *ClusterWorkers {
	return &ClusterWorkers{
		WorkerStackID: workerStackID,
		WorkersStatus: workerStatus,
		WorkerReason:  workerReason,
		WorkerARN:     workerARN,
	}
}

// Client interface to perform cluster operations
type Client interface {
	Create(string, awscomputev1alpha1.EKSClusterSpec) (*Cluster, error)
	Get(string) (*Cluster, error)
	Delete(string) error
	CreateWorkerNodes(name string, version string, spec awscomputev1alpha1.EKSClusterSpec) (*ClusterWorkers, error)
	GetWorkerNodes(stackID string) (*ClusterWorkers, error)
	DeleteWorkerNodes(stackID string) error
	ConnectionToken(string) (string, error)
}

// AMIClient the interface for getting AMI images information
type AMIClient interface {
	DescribeImagesRequest(*ec2.DescribeImagesInput) ec2.DescribeImagesRequest
}

type eksClient struct {
	eks            eksiface.EKSAPI
	amiClient      AMIClient
	sts            *sts.STS
	cloudformation cfc.Client
}

// NewClient return new instance of the crossplane client for a specific AWS configuration
func NewClient(config *aws.Config) Client {
	return &eksClient{eks.New(*config),
		ec2.New(*config), sts.New(*config), cfc.NewClient(config)}
}

// Create new EKS cluster
func (e *eksClient) Create(name string, spec awscomputev1alpha1.EKSClusterSpec) (*Cluster, error) {
	input := &eks.CreateClusterInput{
		Name:    aws.String(name),
		RoleArn: aws.String(spec.RoleARN),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			SubnetIds:        spec.SubnetIds,
			SecurityGroupIds: spec.SecurityGroupIds,
		},
	}
	if spec.ClusterVersion != "" {
		input.Version = aws.String(spec.ClusterVersion)
	}

	output, err := e.eks.CreateClusterRequest(input).Send()
	if err != nil {
		return nil, err
	}
	return NewCluster(output.Cluster), nil
}

// CreateWorkerNodes new EKS cluster workers nodes
func (e *eksClient) CreateWorkerNodes(name string, clusterVersion string, spec awscomputev1alpha1.EKSClusterSpec) (*ClusterWorkers, error) {
	// Cloud formation create workers
	ami, err := e.getAMIImage(spec.WorkerNodes.NodeImageID, clusterVersion)
	if err != nil {
		return nil, err
	}

	subnetIds := strings.Join(spec.SubnetIds, ",")
	parameters := map[string]string{
		"ClusterName":                      name,
		"VpcId":                            spec.VpcID,
		"Subnets":                          subnetIds,
		"KeyName":                          spec.WorkerNodes.KeyName,
		"NodeImageId":                      aws.StringValue(ami.ImageId),
		"NodeInstanceType":                 spec.WorkerNodes.NodeInstanceType,
		"BootstrapArguments":               spec.WorkerNodes.BootstrapArguments,
		"NodeGroupName":                    spec.WorkerNodes.NodeGroupName,
		"ClusterControlPlaneSecurityGroup": spec.WorkerNodes.ClusterControlPlaneSecurityGroup,
	}

	if spec.WorkerNodes.NodeAutoScalingGroupMinSize != nil {
		nodeAutoScalingGroupMinSize := strconv.Itoa(*spec.WorkerNodes.NodeAutoScalingGroupMinSize)
		parameters["NodeAutoScalingGroupMinSize"] = nodeAutoScalingGroupMinSize
	}

	if spec.WorkerNodes.NodeAutoScalingGroupMaxSize != nil {
		nodeAutoScalingGroupMaxSize := strconv.Itoa(*spec.WorkerNodes.NodeAutoScalingGroupMaxSize)
		parameters["NodeAutoScalingGroupMaxSize"] = nodeAutoScalingGroupMaxSize
	}

	if spec.WorkerNodes.NodeVolumeSize != nil {
		nodeVolumeSize := strconv.Itoa(*spec.WorkerNodes.NodeVolumeSize)
		parameters["NodeVolumeSize"] = nodeVolumeSize
	}

	stackID, err := e.cloudformation.CreateStack(aws.String(name), aws.String(workerCloudFormationTemplate), parameters)
	if err != nil {
		return nil, err
	}

	return NewClusterWorkers(*stackID, cloudformation.StackStatusCreateInProgress, "", ""), nil
}

// Get an existing EKS cluster
func (e *eksClient) Get(name string) (*Cluster, error) {
	input := &eks.DescribeClusterInput{Name: aws.String(name)}
	output, err := e.eks.DescribeClusterRequest(input).Send()
	if err != nil {
		return nil, err
	}

	return NewCluster(output.Cluster), err
}

// GetWorkerNodes information about existing cloud formation stack
func (e *eksClient) GetWorkerNodes(stackID string) (*ClusterWorkers, error) {
	stack, err := e.cloudformation.GetStack(&stackID)
	if err != nil {
		return nil, err
	}

	nodeARN := ""
	if stack.Outputs != nil {
		for _, item := range stack.Outputs {
			if aws.StringValue(item.OutputKey) == cloudFormationNodeInstanceRole {
				nodeARN = aws.StringValue(item.OutputValue)
				break
			}
		}
	}

	return NewClusterWorkers(stackID, stack.StackStatus, aws.StringValue(stack.StackStatusReason), nodeARN), nil
}

// Delete a EKS cluster
func (e *eksClient) Delete(name string) error {
	input := &eks.DeleteClusterInput{Name: aws.String(name)}
	_, err := e.eks.DeleteClusterRequest(input).Send()
	return err
}

// DeleteWorkerNodes deletes the cloud formation for this stack.
func (e *eksClient) DeleteWorkerNodes(stackID string) error {
	return e.cloudformation.DeleteStack(&stackID)
}

// ConnectionToken to a cluster
func (e *eksClient) ConnectionToken(name string) (string, error) {
	request := e.sts.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	request.HTTPRequest.Header.Add(clusterIDHeader, name)

	// sign the request
	presignedURLString, err := request.Presign(60 * time.Second)
	if err != nil {
		return "", err
	}

	return v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLString)), nil
}

// getAMIImage checks to see if the requested image ID is compatible with the
// AMI images available for the cluster. If no imageID is provided, it picks the most
// recent available image.
func (e *eksClient) getAMIImage(requestedImageID, clusterVersion string) (*ec2.Image, error) {
	images, err := e.getAvailableImages(clusterVersion)
	if err != nil {
		return nil, err
	}

	if len(images) == 0 {
		return nil, errors.New("No available AMI images were found for the cluster version in this region")
	}

	if requestedImageID != "" {
		return getImageWithID(requestedImageID, images)
	}

	return getMostRecentImage(images), nil
}

// getAvailableImages retrieves AMI images available for the cluster.
func (e *eksClient) getAvailableImages(clusterVersion string) ([]*ec2.Image, error) {

	// make sure the provided cluster version is valid
	r := regexp.MustCompile(`v?(\d+)\.(\d+)`)
	versionParts := r.FindStringSubmatch(clusterVersion)
	if len(versionParts) < 3 {
		return nil, errors.New("Cluster version is empty or invalid")
	}
	versionMajor, versionMinor := versionParts[1], versionParts[2]

	// reconstruct the cluster version off of major and minor
	version := fmt.Sprintf("%v.%v", versionMajor, versionMinor)

	// ami query
	filters := map[string][]string{
		"name":  {fmt.Sprintf("*amazon-eks-node-%v*", version)},
		"state": {"available"},
	}

	ec2Filters := []ec2.Filter{}
	for name, values := range filters {
		ec2Filters = append(ec2Filters, ec2.Filter{Name: aws.String(name), Values: values})
	}

	request := e.amiClient.DescribeImagesRequest(&ec2.DescribeImagesInput{
		Filters: ec2Filters,
	})
	out, err := request.Send()

	if err != nil {
		return nil, err
	}

	result := make([]*ec2.Image, len(out.Images))
	for i := 0; i < len(out.Images); i++ {
		result[i] = &out.Images[i]
	}

	return result, nil
}

func getMostRecentImage(images []*ec2.Image) *ec2.Image {
	var result *ec2.Image
	for i := 0; i < len(images); i++ {
		if result == nil || aws.StringValue(result.CreationDate) < aws.StringValue(images[i].CreationDate) {
			result = images[i]
		}
	}

	return result
}

func getImageWithID(imgName string, images []*ec2.Image) (*ec2.Image, error) {
	for _, img := range images {
		if imgName == aws.StringValue(img.ImageId) {
			return img, nil
		}
	}

	return nil, errors.New("The specified AMI image name is either invalid or is not available for this cluster version and region")
}

// IsErrorAlreadyExists helper function
func IsErrorAlreadyExists(err error) bool {
	return strings.Contains(err.Error(), eks.ErrCodeResourceInUseException)
}

// IsErrorBadRequest helper function
func IsErrorBadRequest(err error) bool {
	return strings.Contains(err.Error(), eks.ErrCodeInvalidParameterException) ||
		strings.Contains(err.Error(), eks.ErrCodeUnsupportedAvailabilityZoneException)
}

// IsErrorNotFound helper function
func IsErrorNotFound(err error) bool {
	return strings.Contains(err.Error(), eks.ErrCodeResourceNotFoundException)
}

const (
	// workerCloudFormationTemplate taken from aws README
	// https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
	// Specifically: https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-11-07/amazon-eks-nodegroup.yaml
	workerCloudFormationTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS - Node Group - Released 2018-08-30'

Parameters:

  KeyName:
    Description: The EC2 Key Pair to allow SSH access to the instances
    Type: AWS::EC2::KeyPair::KeyName

  NodeImageId:
    Type: AWS::EC2::Image::Id
    Description: AMI id for the node instances.

  NodeInstanceType:
    Description: EC2 instance type for the node instances
    Type: String
    Default: t2.medium
    AllowedValues:
    - t2.small
    - t2.medium
    - t2.large
    - t2.xlarge
    - t2.2xlarge
    - t3.nano
    - t3.micro
    - t3.small
    - t3.medium
    - t3.large
    - t3.xlarge
    - t3.2xlarge
    - m3.medium
    - m3.large
    - m3.xlarge
    - m3.2xlarge
    - m4.large
    - m4.xlarge
    - m4.2xlarge
    - m4.4xlarge
    - m4.10xlarge
    - m5.large
    - m5.xlarge
    - m5.2xlarge
    - m5.4xlarge
    - m5.12xlarge
    - m5.24xlarge
    - c4.large
    - c4.xlarge
    - c4.2xlarge
    - c4.4xlarge
    - c4.8xlarge
    - c5.large
    - c5.xlarge
    - c5.2xlarge
    - c5.4xlarge
    - c5.9xlarge
    - c5.18xlarge
    - i3.large
    - i3.xlarge
    - i3.2xlarge
    - i3.4xlarge
    - i3.8xlarge
    - i3.16xlarge
    - r3.xlarge
    - r3.2xlarge
    - r3.4xlarge
    - r3.8xlarge
    - r4.large
    - r4.xlarge
    - r4.2xlarge
    - r4.4xlarge
    - r4.8xlarge
    - r4.16xlarge
    - x1.16xlarge
    - x1.32xlarge
    - p2.xlarge
    - p2.8xlarge
    - p2.16xlarge
    - p3.2xlarge
    - p3.8xlarge
    - p3.16xlarge
    - r5.large
    - r5.xlarge
    - r5.2xlarge
    - r5.4xlarge
    - r5.12xlarge
    - r5.24xlarge
    - r5d.large
    - r5d.xlarge
    - r5d.2xlarge
    - r5d.4xlarge
    - r5d.12xlarge
    - r5d.24xlarge
    - z1d.large
    - z1d.xlarge
    - z1d.2xlarge
    - z1d.3xlarge
    - z1d.6xlarge
    - z1d.12xlarge
    ConstraintDescription: Must be a valid EC2 instance type

  NodeAutoScalingGroupMinSize:
    Type: Number
    Description: Minimum size of Node Group ASG.
    Default: 1

  NodeAutoScalingGroupMaxSize:
    Type: Number
    Description: Maximum size of Node Group ASG.
    Default: 3

  NodeVolumeSize:
    Type: Number
    Description: Node volume size
    Default: 20

  ClusterName:
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.
    Type: String

  BootstrapArguments:
    Description: Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami
    Default: ""
    Type: String

  NodeGroupName:
    Description: Unique identifier for the Node Group.
    Type: String

  ClusterControlPlaneSecurityGroup:
    Description: The security group of the cluster control plane.
    Type: AWS::EC2::SecurityGroup::Id

  VpcId:
    Description: The VPC of the worker instances
    Type: AWS::EC2::VPC::Id

  Subnets:
    Description: The subnets where workers can be created.
    Type: List<AWS::EC2::Subnet::Id>

Metadata:
  AWS::CloudFormation::Interface:
    ParameterGroups:
      -
        Label:
          default: "EKS Cluster"
        Parameters:
          - ClusterName
          - ClusterControlPlaneSecurityGroup
      -
        Label:
          default: "Worker Node Configuration"
        Parameters:
          - NodeGroupName
          - NodeAutoScalingGroupMinSize
          - NodeAutoScalingGroupMaxSize
          - NodeInstanceType
          - NodeImageId
          - NodeVolumeSize
          - KeyName
          - BootstrapArguments
      -
        Label:
          default: "Worker Network Configuration"
        Parameters:
          - VpcId
          - Subnets

Resources:

  NodeInstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: "/"
      Roles:
      - !Ref NodeInstanceRole

  NodeInstanceRole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service:
            - ec2.amazonaws.com
          Action:
          - sts:AssumeRole
      Path: "/"
      ManagedPolicyArns:
        - arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
        - arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy
        - arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly

  Route53NodeInstancePolicy:
    Type: AWS::IAM::Policy
    DependsOn: NodeInstanceRole
    Properties:
      PolicyName: !Ref AWS::StackName
      PolicyDocument:
        Version: "2012-10-17"
        Statement:
          -
            Effect: "Allow"
            Action:
              - "route53:ChangeResourceRecordSets"
            Resource:
              - "arn:aws:route53:::hostedzone/*"
          -
            Effect: "Allow"
            Action:
              - "route53:ListHostedZones"
              - "route53:ListResourceRecordSets"
            Resource:
              - "*"
      Roles:
        - !Ref NodeInstanceRole

  NodeSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for all nodes in the cluster
      VpcId:
        !Ref VpcId
      Tags:
      - Key: !Sub "kubernetes.io/cluster/${ClusterName}"
        Value: 'owned'

  NodeSecurityGroupIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow node to communicate with each other
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: '-1'
      FromPort: 0
      ToPort: 65535

  NodeSecurityGroupFromControlPlaneIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow worker Kubelets and pods to receive communication from the cluster control plane
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  ControlPlaneEgressToNodeSecurityGroup:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with worker Kubelet and pods
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  NodeSecurityGroupFromControlPlaneOn443Ingress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods running extension API servers on port 443 to receive communication from cluster control plane
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  ControlPlaneEgressToNodeSecurityGroupOn443:
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods running extension API servers on port 443
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  ClusterControlPlaneSecurityGroupIngress:
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods to communicate with the cluster API Server
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      ToPort: 443
      FromPort: 443

  NodeGroup:
    Type: AWS::AutoScaling::AutoScalingGroup
    Properties:
      DesiredCapacity: !Ref NodeAutoScalingGroupMaxSize
      LaunchConfigurationName: !Ref NodeLaunchConfig
      MinSize: !Ref NodeAutoScalingGroupMinSize
      MaxSize: !Ref NodeAutoScalingGroupMaxSize
      VPCZoneIdentifier:
        !Ref Subnets
      Tags:
      - Key: Name
        Value: !Sub "${ClusterName}-${NodeGroupName}-Node"
        PropagateAtLaunch: 'true'
      - Key: !Sub 'kubernetes.io/cluster/${ClusterName}'
        Value: 'owned'
        PropagateAtLaunch: 'true'
    UpdatePolicy:
      AutoScalingRollingUpdate:
        MinInstancesInService: '1'
        MaxBatchSize: '1'

  NodeLaunchConfig:
    Type: AWS::AutoScaling::LaunchConfiguration
    Properties:
      AssociatePublicIpAddress: 'true'
      IamInstanceProfile: !Ref NodeInstanceProfile
      ImageId: !Ref NodeImageId
      InstanceType: !Ref NodeInstanceType
      KeyName: !Ref KeyName
      SecurityGroups:
      - !Ref NodeSecurityGroup
      BlockDeviceMappings:
        - DeviceName: /dev/xvda
          Ebs:
            VolumeSize: !Ref NodeVolumeSize
            VolumeType: gp2
            DeleteOnTermination: true
      UserData:
        Fn::Base64:
          !Sub |
            #!/bin/bash
            set -o xtrace
            /etc/eks/bootstrap.sh ${ClusterName} ${BootstrapArguments}
            /opt/aws/bin/cfn-signal --exit-code $? \
                     --stack  ${AWS::StackName} \
                     --resource NodeGroup  \
                     --region ${AWS::Region}

Outputs:
  NodeInstanceRole:
    Description: The node instance role
    Value: !GetAtt NodeInstanceRole.Arn
  NodeSecurityGroup:
    Description: The security group for the node group
    Value: !Ref NodeSecurityGroup
`
)
