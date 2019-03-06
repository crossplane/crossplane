package iam

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/iamiface"
)

const (
	policyArn = "arn:aws:iam::%s:policy/%s"
)

// Client defines IAM Client operations
type Client interface {
	CreateUser(username string) (*iam.AccessKey, error)
	DeleteUser(username string) error
	CreatePolicyAndAttach(username string, policyName string, policyDocument string) (string, error)
	GetPolicyVersion(policyName string) (string, error)
	UpdatePolicy(policyName string, policyDocument string) (string, error)
	DeletePolicyAndDetach(username string, policyName string) error
}

type iamClient struct {
	accountID *string
	iam       iamiface.IAMAPI
}

// NewClient creates new AWS Client with provided AWS Configurations/Credentials
func NewClient(config *aws.Config) Client {
	return &iamClient{iam: iam.New(*config)}
}

// CreateUser - Creates an IAM User, a policy, binds user to policy and returns an access key and policy version for the user.
func (c *iamClient) CreateUser(username string) (*iam.AccessKey, error) {
	err := c.createUser(username)
	if err != nil {
		return nil, fmt.Errorf("failed to create user, %s", err)
	}

	key, err := c.createAccessKey(username)
	if err != nil {
		return nil, fmt.Errorf("failed to create access key, %s", err)
	}

	return key, err
}

// CreatePolicyAndAttach - Creates the IAM policy and attaches it to the username
func (c *iamClient) CreatePolicyAndAttach(username string, policyName string, policyDocument string) (string, error) {
	currentVersion, err := c.createPolicy(username, policyDocument)
	if err != nil {
		return "", fmt.Errorf("failed to create policy, %s", err)
	}

	err = c.attachPolicyToUser(username, username)
	if err != nil {
		return "", fmt.Errorf("failed to attach policy, %s", err)
	}

	return currentVersion, nil
}

// GetPolicyVersion get the policy document for the IAM user
func (c *iamClient) GetPolicyVersion(username string) (string, error) {
	policyARN, err := c.getPolicyARN(username)
	if err != nil {
		return "", err
	}

	policyResponse, err := c.iam.GetPolicyRequest(&iam.GetPolicyInput{
		PolicyArn: aws.String(policyARN),
	}).Send()

	if err != nil {
		return "", err
	}

	return aws.StringValue(policyResponse.Policy.DefaultVersionId), nil
}

// UpdatePolicy - updates the policy document for the IAM user and return current policy version
func (c *iamClient) UpdatePolicy(policyName string, policyDocument string) (string, error) {
	policyARN, err := c.getPolicyARN(policyName)
	if err != nil {
		return "", err
	}
	// Create a new policy version
	policyVersionResponse, err := c.iam.CreatePolicyVersionRequest(&iam.CreatePolicyVersionInput{PolicyArn: aws.String(policyARN), PolicyDocument: aws.String(policyDocument), SetAsDefault: aws.Bool(true)}).Send()
	if err != nil {
		return "", err
	}

	currentPolicyVersion := policyVersionResponse.PolicyVersion.VersionId
	// Delete old versions of policy - Max 5 allowed
	policyVersions, err := c.iam.ListPolicyVersionsRequest(&iam.ListPolicyVersionsInput{PolicyArn: aws.String(policyARN)}).Send()
	if err != nil {
		return "", err
	}

	for _, policy := range policyVersions.Versions {
		if aws.StringValue(policy.VersionId) != aws.StringValue(currentPolicyVersion) {
			_, err := c.iam.DeletePolicyVersionRequest(&iam.DeletePolicyVersionInput{PolicyArn: aws.String(policyARN), VersionId: policy.VersionId}).Send()
			if err != nil {
				return "", err
			}
		}
	}

	return aws.StringValue(currentPolicyVersion), nil
}

// DeletePolicyAndDetach delete the policy of PolicyName and detach it from the username provided
func (c *iamClient) DeletePolicyAndDetach(username string, policyName string) error {
	policyARN, err := c.getPolicyARN(username)
	if err != nil {
		return err
	}

	_, err = c.iam.DetachUserPolicyRequest(&iam.DetachUserPolicyInput{PolicyArn: aws.String(policyARN), UserName: aws.String(username)}).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}

	_, err = c.iam.DeletePolicyRequest(&iam.DeletePolicyInput{PolicyArn: aws.String(policyARN)}).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}
	return nil
}

// DeleteUser Policy and IAM User
func (c *iamClient) DeleteUser(username string) error {
	keys, err := c.iam.ListAccessKeysRequest(&iam.ListAccessKeysInput{UserName: aws.String(username)}).Send()
	if err != nil {
		return err
	}

	for _, key := range keys.AccessKeyMetadata {
		_, err = c.iam.DeleteAccessKeyRequest(&iam.DeleteAccessKeyInput{AccessKeyId: key.AccessKeyId, UserName: aws.String(username)}).Send()
		if err != nil {
			return err
		}
	}

	_, err = c.iam.DeleteUserRequest(&iam.DeleteUserInput{UserName: aws.String(username)}).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}

	return nil
}

// getAccountID - Gets the accountID of the authenticated session.
func (c *iamClient) getAccountID() (string, error) {
	if c.accountID == nil {
		user, err := c.iam.GetUserRequest(&iam.GetUserInput{}).Send()
		if err != nil {
			return "", err
		}

		arnData, err := arn.Parse(*user.User.Arn)
		if err != nil {
			return "", err
		}
		c.accountID = &arnData.AccountID
	}

	return aws.StringValue(c.accountID), nil
}

func (c *iamClient) getPolicyARN(policyName string) (string, error) {
	accountID, err := c.getAccountID()
	if err != nil {
		return "", err
	}
	policyARN := fmt.Sprintf(policyArn, accountID, policyName)
	return policyARN, nil
}

func (c *iamClient) createUser(username string) error {
	_, err := c.iam.CreateUserRequest(&iam.CreateUserInput{UserName: aws.String(username)}).Send()
	if err != nil && isErrorAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *iamClient) createAccessKey(username string) (*iam.AccessKey, error) {
	keysResponse, err := c.iam.CreateAccessKeyRequest(&iam.CreateAccessKeyInput{UserName: aws.String(username)}).Send()
	if err != nil {
		return nil, err
	}

	return keysResponse.AccessKey, nil
}

func (c *iamClient) createPolicy(policyName string, policyDocument string) (string, error) {
	response, err := c.iam.CreatePolicyRequest(&iam.CreatePolicyInput{PolicyName: aws.String(policyName), PolicyDocument: aws.String(policyDocument)}).Send()
	if err != nil {
		if isErrorAlreadyExists(err) {
			return c.UpdatePolicy(policyName, policyDocument)
		}
		return "", err
	}
	return aws.StringValue(response.Policy.DefaultVersionId), nil
}

func (c *iamClient) attachPolicyToUser(policyName string, username string) error {
	policyArn, err := c.getPolicyARN(policyName)
	if err != nil {
		return err
	}
	_, err = c.iam.AttachUserPolicyRequest(&iam.AttachUserPolicyInput{PolicyArn: aws.String(policyArn), UserName: aws.String(username)}).Send()
	return err
}

func isErrorAlreadyExists(err error) bool {
	if iamErr, ok := err.(awserr.Error); ok && iamErr.Code() == iam.ErrCodeEntityAlreadyExistsException {
		return true
	}
	return false
}

func isErrorNotFound(err error) bool {
	if iamErr, ok := err.(awserr.Error); ok && iamErr.Code() == iam.ErrCodeNoSuchEntityException {
		return true
	}
	return false
}

// PolicyDocument is the structure of IAM policy document
type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

// StatementEntry is used to define permission statements in a PolicyDocument
type StatementEntry struct {
	Sid      string
	Effect   string
	Action   []string
	Resource []string
}
