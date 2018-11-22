package s3

import (
	"encoding/json"
	"fmt"

	"github.com/crossplaneio/crossplane/pkg/util"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3iface"
	"github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	storage "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	iamc "github.com/crossplaneio/crossplane/pkg/clients/aws/iam"
)

const (
	bucketUser           = "crossplane-bucket-%s"
	bucketObjectARN      = "arn:aws:s3:::%s"
	maxIAMUsernameLength = 64
)

// Service defines S3 Client operations
type Service interface {
	CreateBucket(spec *v1alpha1.S3BucketSpec) error
	CreateUser(username *string, spec *v1alpha1.S3BucketSpec) (*iam.AccessKey, error)
	UpdateBucketACL(spec *v1alpha1.S3BucketSpec) error
	UpdateVersioning(spec *v1alpha1.S3BucketSpec) error
	UpdatePolicyDocument(username *string, spec *v1alpha1.S3BucketSpec) error
	Delete(bucket *v1alpha1.S3Bucket) error
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
func (c *Client) CreateBucket(spec *v1alpha1.S3BucketSpec) error {
	input := CreateBucketInput(spec)
	_, err := c.s3.CreateBucketRequest(input).Send()
	if err != nil {
		if isErrorAlreadyExists(err) {
			err = c.UpdateBucketACL(spec)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	err = c.UpdateVersioning(spec)
	if err != nil {
		return fmt.Errorf("could not update versioning, %s", err.Error())
	}

	return nil
}

// CreateUser - Create as user to access bucket per permissions in BucketSpec
func (c *Client) CreateUser(username *string, spec *v1alpha1.S3BucketSpec) (*iam.AccessKey, error) {
	policyDocument, err := getPolicyDocument(spec)
	if err != nil {
		return nil, fmt.Errorf("could not update policy, %s", err.Error())
	}
	accessKeys, err := c.iamClient.Create(username, policyDocument)
	if err != nil {
		return nil, fmt.Errorf("could not create user %s", err)
	}

	return accessKeys, nil
}

// UpdateBucketACL - Updated CannedACL on Bucket
func (c *Client) UpdateBucketACL(spec *v1alpha1.S3BucketSpec) error {
	input := &s3.PutBucketAclInput{
		ACL:    s3.BucketCannedACL(spec.CannedACL),
		Bucket: &spec.Name,
	}
	_, err := c.s3.PutBucketAclRequest(input).Send()
	return err
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
func (c *Client) UpdatePolicyDocument(username *string, spec *v1alpha1.S3BucketSpec) error {
	policyDocument, err := getPolicyDocument(spec)
	if err != nil {
		return fmt.Errorf("could not generate policy, %s", err.Error())
	}
	err = c.iamClient.UpdatePolicy(username, policyDocument)
	if err != nil {
		return fmt.Errorf("could not update policy, %s", err.Error())
	}
	return nil
}

// Delete deletes s3 bucket, and related IAM
func (c *Client) Delete(bucket *v1alpha1.S3Bucket) error {
	input := &s3.DeleteBucketInput{
		Bucket: &bucket.Spec.Name,
	}
	_, err := c.s3.DeleteBucketRequest(input).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}

	if bucket.Status.IAMUsername != nil {
		return c.iamClient.Delete(bucket.Status.IAMUsername)
	}

	return nil
}

// isErrorAlreadyExists helper function to test for ErrCodeBucketAlreadyOwnedByYou error
func isErrorAlreadyExists(err error) bool {
	if bucketErr, ok := err.(awserr.Error); ok && bucketErr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
		return true
	}
	return false
}

// isErrorNotFound helper function to test for ErrCodeNoSuchEntityException error
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

// GenerateBucketUsername - Genereates a username that is within AWS size specifications, and adds a random suffix
func GenerateBucketUsername(spec *v1alpha1.S3BucketSpec) *string {
	username := util.GenerateNameMaxLength(fmt.Sprintf(bucketUser, spec.Name), maxIAMUsernameLength)
	return &username
}

func getPolicyDocument(spec *v1alpha1.S3BucketSpec) (*string, error) {
	bucketARN := fmt.Sprintf(bucketObjectARN, spec.Name)
	read := iamc.StatementEntry{
		Sid:    "crossplaneRead",
		Effect: "Allow",
		Action: []string{
			"s3:Get*",
			"s3:List*",
		},
		Resource: []string{bucketARN, bucketARN + "/*"},
	}

	write := iamc.StatementEntry{
		Sid:    "crossplaneWrite",
		Effect: "Allow",
		Action: []string{
			"s3:DeleteObject",
			"s3:Put*",
		},
		Resource: []string{bucketARN + "/*"},
	}

	policy := iamc.PolicyDocument{
		Version:   "2012-10-17",
		Statement: []iamc.StatementEntry{},
	}

	for _, perm := range spec.LocalPermissions {
		if perm == storage.ReadPermission {
			policy.Statement = append(policy.Statement, read)
		} else if perm == storage.WritePermission {
			policy.Statement = append(policy.Statement, write)
		} else {
			return nil, fmt.Errorf("unknown permission, %s", perm)
		}
	}

	b, err := json.Marshal(&policy)
	if err != nil {
		return nil, fmt.Errorf("error marshaling policy, %s", err.Error())
	}

	policyString := string(b)
	return &policyString, nil
}
