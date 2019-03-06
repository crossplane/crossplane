package s3

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3iface"

	"github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	storage "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	iamc "github.com/crossplaneio/crossplane/pkg/clients/aws/iam"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	bucketUser           = "crossplane-bucket-%s"
	bucketObjectARN      = "arn:aws:s3:::%s"
	maxIAMUsernameLength = 64
)

// Service defines S3 Client operations
type Service interface {
	CreateOrUpdateBucket(spec *v1alpha1.S3BucketSpec) error
	GetBucketInfo(username string, spec *v1alpha1.S3BucketSpec) (*Bucket, error)
	CreateUser(username string, spec *v1alpha1.S3BucketSpec) (*iam.AccessKey, string, error)
	UpdateBucketACL(spec *v1alpha1.S3BucketSpec) error
	UpdateVersioning(spec *v1alpha1.S3BucketSpec) error
	UpdatePolicyDocument(username string, spec *v1alpha1.S3BucketSpec) (string, error)
	DeleteBucket(bucket *v1alpha1.S3Bucket) error
}

// Client implements S3 Client
type Client struct {
	s3        s3iface.S3API
	iamClient iamc.Client
}

// NewClient creates new S3 Client with provided AWS Configurations/Credentials
func NewClient(config *aws.Config) Service {
	return &Client{s3: s3.New(*config), iamClient: iamc.NewClient(config)}
}

// CreateOrUpdateBucket creates or updates the supplied S3 bucket with provided
// specification, and returns access keys with permissions of localPermission
func (c *Client) CreateOrUpdateBucket(spec *v1alpha1.S3BucketSpec) error {
	input := CreateBucketInput(spec)
	_, err := c.s3.CreateBucketRequest(input).Send()
	if err != nil {
		if isErrorAlreadyExists(err) {
			return c.UpdateBucketACL(spec)
		}
	}
	return err
}

// Bucket represents crossplane metadata about the bucket
type Bucket struct {
	Versioning        bool
	UserPolicyVersion string
}

// GetBucketInfo returns the status of key bucket settings including user's policy version for permission status
func (c *Client) GetBucketInfo(username string, spec *v1alpha1.S3BucketSpec) (*Bucket, error) {
	bucket := Bucket{}
	bucketVersioning, err := c.s3.GetBucketVersioningRequest(&s3.GetBucketVersioningInput{Bucket: aws.String(spec.Name)}).Send()
	if err != nil {
		return nil, err
	}
	bucket.Versioning = bucketVersioning.Status == s3.BucketVersioningStatusEnabled
	policyVersion, err := c.iamClient.GetPolicyVersion(username)
	if err != nil {
		return nil, err
	}
	bucket.UserPolicyVersion = policyVersion

	return &bucket, err
}

// CreateUser - Create as user to access bucket per permissions in BucketSpec returing access key and policy version
func (c *Client) CreateUser(username string, spec *v1alpha1.S3BucketSpec) (*iam.AccessKey, string, error) {
	policyDocument, err := newPolicyDocument(spec)
	if err != nil {
		return nil, "", fmt.Errorf("could not update policy, %s", err.Error())
	}
	accessKeys, err := c.iamClient.CreateUser(username)
	if err != nil {
		return nil, "", fmt.Errorf("could not create user %s", err)
	}

	currentVersion, err := c.iamClient.CreatePolicyAndAttach(username, username, policyDocument)
	if err != nil {
		return nil, "", fmt.Errorf("could not create policy %s", err)
	}

	return accessKeys, currentVersion, nil
}

// UpdateBucketACL - Updated CannedACL on Bucket
func (c *Client) UpdateBucketACL(spec *v1alpha1.S3BucketSpec) error {
	var err error
	if spec.CannedACL != nil {
		input := &s3.PutBucketAclInput{
			ACL:    s3.BucketCannedACL(*spec.CannedACL),
			Bucket: &spec.Name,
		}
		_, err = c.s3.PutBucketAclRequest(input).Send()
	}

	return err
}

// UpdateVersioning configuration for Bucket
func (c *Client) UpdateVersioning(spec *v1alpha1.S3BucketSpec) error {
	versioningStatus := s3.BucketVersioningStatusSuspended
	if spec.Versioning {
		versioningStatus = s3.BucketVersioningStatusEnabled
	}

	input := &s3.PutBucketVersioningInput{Bucket: &spec.Name, VersioningConfiguration: &s3.VersioningConfiguration{Status: versioningStatus}}
	_, err := c.s3.PutBucketVersioningRequest(input).Send()
	if err != nil {
		return err
	}
	return nil
}

// UpdatePolicyDocument based on localPermissions
func (c *Client) UpdatePolicyDocument(username string, spec *v1alpha1.S3BucketSpec) (string, error) {
	policyDocument, err := newPolicyDocument(spec)
	if err != nil {
		return "", fmt.Errorf("could not generate policy, %s", err.Error())
	}
	currentVersion, err := c.iamClient.UpdatePolicy(username, policyDocument)
	if err != nil {
		return "", fmt.Errorf("could not update policy, %s", err.Error())
	}
	return currentVersion, nil
}

// DeleteBucket deletes s3 bucket, and related IAM
func (c *Client) DeleteBucket(bucket *v1alpha1.S3Bucket) error {
	input := &s3.DeleteBucketInput{
		Bucket: &bucket.Spec.Name,
	}
	_, err := c.s3.DeleteBucketRequest(input).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}

	if bucket.Status.IAMUsername != "" {
		err := c.iamClient.DeletePolicyAndDetach(bucket.Status.IAMUsername, bucket.Status.IAMUsername)
		if err != nil {
			return err
		}

		return c.iamClient.DeleteUser(bucket.Status.IAMUsername)
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

// CreateBucketInput returns a CreateBucketInput from the supplied S3BucketSpec.
func CreateBucketInput(spec *v1alpha1.S3BucketSpec) *s3.CreateBucketInput {
	bucketInput := &s3.CreateBucketInput{
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{LocationConstraint: s3.BucketLocationConstraint(spec.Region)},
		Bucket:                    &spec.Name,
	}
	if spec.CannedACL != nil {
		bucketInput.ACL = s3.BucketCannedACL(*spec.CannedACL)
	}
	return bucketInput
}

// GenerateBucketUsername - Genereates a username that is within AWS size specifications, and adds a random suffix
func GenerateBucketUsername(spec *v1alpha1.S3BucketSpec) string {
	return util.GenerateNameMaxLength(fmt.Sprintf(bucketUser, spec.Name), maxIAMUsernameLength)
}

func newPolicyDocument(spec *v1alpha1.S3BucketSpec) (string, error) {
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

	if spec.LocalPermission != nil {
		switch *spec.LocalPermission {
		case storage.ReadOnlyPermission:
			policy.Statement = append(policy.Statement, read)
		case storage.WriteOnlyPermission:
			policy.Statement = append(policy.Statement, write)
		case storage.ReadWritePermission:
			policy.Statement = append(policy.Statement, read, write)
		default:
			return "", fmt.Errorf("unknown permission, %s", *spec.LocalPermission)
		}
	}

	b, err := json.Marshal(&policy)
	if err != nil {
		return "", fmt.Errorf("error marshaling policy, %s", err.Error())
	}

	return string(b), nil
}
