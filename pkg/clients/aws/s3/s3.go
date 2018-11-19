package s3

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3iface"
	"github.com/upbound/conductor/pkg/apis/aws/storage/v1alpha1"
	storage "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
	iamc "github.com/upbound/conductor/pkg/clients/aws/iam"
)

const (
	bucketUser      = "conductor-bucket-%s"
	bucketObjectARN = "arn:aws:s3:::%s/*"
)

// Service defines S3 Client operations
type Service interface {
	Create(spec *v1alpha1.S3BucketSpec, localPermissions []storage.LocalPermissionType) (*iam.AccessKey, error)
	UpdateBucketACL(spec *v1alpha1.S3BucketSpec) error
	UpdateVersioning(spec *v1alpha1.S3BucketSpec) error
	UpdatePolicyDocument(spec *v1alpha1.S3BucketSpec, localPermissions []storage.LocalPermissionType) error
	Delete(spec *v1alpha1.S3BucketSpec) error
}

// Client implements S3 Client
type Client struct {
	s3        s3iface.S3API
	iamClient iamc.Service
}

// NewClient creates new S3 Client with provided AWS Configurations/Credentials
func NewClient(config *aws.Config) Service {
	return &Client{iamClient: iamc.NewClient(config), s3: s3.New(*config)}
}

// Create creates s3 bucket with provided specification, and returns access keys per localPermissions
func (c *Client) Create(spec *v1alpha1.S3BucketSpec, localPermissions []storage.LocalPermissionType) (*iam.AccessKey, error) {
	input := CreateBucketInput(spec)
	_, err := c.s3.CreateBucketRequest(input).Send()
	if err != nil {
		if isErrorAlreadyExists(err) {
			err = c.UpdateBucketACL(spec)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	err = c.UpdateVersioning(spec)
	if err != nil {
		return nil, fmt.Errorf("Could not update versioning, %s", err.Error())
	}

	policyDocument, err := getPolicyDocument(spec, localPermissions)
	if err != nil {
		return nil, fmt.Errorf("Could not update policy, %s", err.Error())
	}
	return c.iamClient.Create(getBucketUsername(spec), policyDocument)
}

// UpdateBucketACL - Updated CannedACL on Bucket
func (c *Client) UpdateBucketACL(spec *v1alpha1.S3BucketSpec) error {
	input := &s3.PutBucketAclInput{
		ACL:    s3.BucketCannedACL(spec.CannedACL),
		Bucket: &spec.Name,
	}
	_, err := c.s3.PutBucketAclRequest(input).Send()
	if err != nil {
		return err
	}

	return nil
}

// UpdateVersioning configuration for Bucket
func (c *Client) UpdateVersioning(spec *v1alpha1.S3BucketSpec) error {
	input := &s3.PutBucketVersioningInput{Bucket: &spec.Name, VersioningConfiguration: &s3.VersioningConfiguration{Status: s3.BucketVersioningStatusEnabled}}
	_, err := c.s3.PutBucketVersioningRequest(input).Send()
	if err != nil {
		return err
	}
	return nil
}

// UpdatePolicyDocument based on localPermissions
func (c *Client) UpdatePolicyDocument(spec *v1alpha1.S3BucketSpec, localPermissions []storage.LocalPermissionType) error {
	policyDocument, err := getPolicyDocument(spec, localPermissions)
	if err != nil {
		return fmt.Errorf("Could not generate policy, %s", err.Error())
	}
	err = c.iamClient.UpdatePolicy(getBucketUsername(spec), policyDocument)
	if err != nil {
		return fmt.Errorf("Could not update policy, %s", err.Error())
	}
	return nil
}

// DeleteBucket deletes s3 bucket, and related IAM
func (c *Client) Delete(spec *v1alpha1.S3BucketSpec) error {
	input := &s3.DeleteBucketInput{
		Bucket: &spec.Name,
	}
	_, err := c.s3.DeleteBucketRequest(input).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}

	return c.iamClient.Delete(getBucketUsername(spec))
}

// IsErrorAlreadyExists helper function to test for ErrCodeEntityAlreadyExistsException error
func isErrorAlreadyExists(err error) bool {
	if bucketErr, ok := err.(awserr.Error); ok && bucketErr.Code() == s3.ErrCodeBucketAlreadyExists {
		return true
	}
	return false
}

// IsErrorNotFound helper function to test for ErrCodeNoSuchEntityException error
func isErrorNotFound(err error) bool {
	if bucketErr, ok := err.(awserr.Error); ok && bucketErr.Code() == s3.ErrCodeNoSuchBucket {
		return true
	}
	return false
}

// CreateS3Bucket from S3BucketSpec
func CreateBucketInput(spec *v1alpha1.S3BucketSpec) *s3.CreateBucketInput {
	return &s3.CreateBucketInput{
		ACL:                       s3.BucketCannedACL(spec.CannedACL),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{LocationConstraint: s3.BucketLocationConstraint(spec.Region)},
		Bucket:                    &spec.Name,
	}
}

func getBucketUsername(spec *v1alpha1.S3BucketSpec) *string {
	username := fmt.Sprintf(bucketUser, spec.Name)
	return &username
}

func getPolicyDocument(spec *v1alpha1.S3BucketSpec, localPermissions []storage.LocalPermissionType) (*string, error) {
	bucketARN := fmt.Sprintf(bucketObjectARN, spec.Name)
	read := iamc.StatementEntry{
		Sid:    "conductor-read",
		Effect: "Allow",
		Action: []string{
			"s3:Get*",
			"s3:List*",
		},
		Resource: bucketARN,
	}

	write := iamc.StatementEntry{
		Sid:    "conductor-write",
		Effect: "Allow",
		Action: []string{
			"s3:DeleteObject",
			"s3:Put*",
		},
		Resource: bucketARN,
	}

	policy := iamc.PolicyDocument{
		Version:   "2012-10-17",
		Statement: []iamc.StatementEntry{},
	}

	for _, perm := range localPermissions {
		if perm == storage.ReadPermission {
			policy.Statement = append(policy.Statement, read)
		} else if perm == storage.WritePermission {
			policy.Statement = append(policy.Statement, write)
		}
	}

	b, err := json.Marshal(&policy)
	if err != nil {
		return nil, fmt.Errorf("Error marshaling policy, %s", err.Error())
	}

	policyString := string(b)
	return &policyString, nil
}
