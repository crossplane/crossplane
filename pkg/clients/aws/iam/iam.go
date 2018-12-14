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

// Service defines IAM Client operations
type Service interface {
	CreateUser(username *string) (*iam.AccessKey, error)
	DeleteUser(username *string) error
	CreatePolicyAndAttach(username *string, policyName *string, policyDocument *string) (*string, error)
	GetPolicyVersion(policyName *string) (*string, error)
	UpdatePolicy(policyName *string, policyDocument *string) (*string, error)
	DeletePolicyAndDetach(username *string, policyName *string) error
}

// Client implements IAM Client
type Client struct {
	accountID *string
	iam       iamiface.IAMAPI
}

// NewClient creates new AWS Client with provided AWS Configurations/Credentials
func NewClient(config *aws.Config) Service {
	return &Client{iam: iam.New(*config)}
}

// Create - Creates an IAM User, a policy, binds user to policy and returns an access key and policy version for the user.
func (c *Client) CreateUser(username *string) (*iam.AccessKey, error) {
	err := c.createUser(username)
	if err != nil {
		return nil, fmt.Errorf("Failed to create user, %s", err)
	}

	key, err := c.createAccessKey(username)
	if err != nil {
		return nil, fmt.Errorf("Failed to create access key, %s", err)
	}

	return key, err
}

func (c *Client) CreatePolicyAndAttach(username *string, policyName *string, policyDocument *string) (*string, error) {
	currentVersion, err := c.createPolicy(username, policyDocument)
	if err != nil {
		return nil, fmt.Errorf("Failed to create policy, %s", err)
	}

	err = c.attachPolicyToUser(username, username)
	if err != nil {
		return nil, fmt.Errorf("Failed to attach policy, %s", err)
	}

	return currentVersion, nil
}

// GetPolicy get the policy document for the IAM user
func (c *Client) GetPolicyVersion(username *string) (*string, error) {
	policyARN, err := c.getPolicyARN(username)
	if err != nil {
		return nil, err
	}

	policyResponse, err := c.iam.GetPolicyRequest(&iam.GetPolicyInput{
		PolicyArn: policyARN,
	}).Send()

	if err != nil {
		return nil, err
	}

	return policyResponse.Policy.DefaultVersionId, nil
}

// Update the policy document for the IAM user and return current policy version
func (c *Client) UpdatePolicy(policyName *string, policyDocument *string) (*string, error) {
	policyARN, err := c.getPolicyARN(policyName)
	if err != nil {
		return nil, err
	}
	// Create a new policy version
	policyVersionResponse, err := c.iam.CreatePolicyVersionRequest(&iam.CreatePolicyVersionInput{PolicyArn: policyARN, PolicyDocument: policyDocument, SetAsDefault: aws.Bool(true)}).Send()
	if err != nil {
		return nil, err
	}

	currentPolicyVersion := policyVersionResponse.PolicyVersion.VersionId
	// Delete old versions of policy - Max 5 allowed
	policyVersions, err := c.iam.ListPolicyVersionsRequest(&iam.ListPolicyVersionsInput{PolicyArn: policyARN}).Send()
	if err != nil {
		return nil, err
	}

	for _, policy := range policyVersions.Versions {
		if *policy.VersionId != *currentPolicyVersion {
			_, err := c.iam.DeletePolicyVersionRequest(&iam.DeletePolicyVersionInput{PolicyArn: policyARN, VersionId: policy.VersionId}).Send()
			if err != nil {
				return nil, err
			}
		}
	}

	return currentPolicyVersion, nil
}

func (c *Client) DeletePolicyAndDetach(username *string, policyName *string) error {
	policyARN, err := c.getPolicyARN(username)
	if err != nil {
		return err
	}

	_, err = c.iam.DetachUserPolicyRequest(&iam.DetachUserPolicyInput{PolicyArn: policyARN, UserName: username}).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}

	_, err = c.iam.DeletePolicyRequest(&iam.DeletePolicyInput{PolicyArn: policyARN}).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}
	return nil
}

// Delete Policy and IAM User
func (c *Client) DeleteUser(username *string) error {
	keys, err := c.iam.ListAccessKeysRequest(&iam.ListAccessKeysInput{UserName: username}).Send()
	if err != nil {
		return err
	}

	for _, key := range keys.AccessKeyMetadata {
		_, err = c.iam.DeleteAccessKeyRequest(&iam.DeleteAccessKeyInput{AccessKeyId: key.AccessKeyId, UserName: username}).Send()
		if err != nil {
			return err
		}
	}

	_, err = c.iam.DeleteUserRequest(&iam.DeleteUserInput{UserName: username}).Send()
	if err != nil && !isErrorNotFound(err) {
		return err
	}

	return nil
}

// getAccountID - Gets the accountID of the authenticated session.
func (c *Client) getAccountID() (*string, error) {
	if c.accountID == nil {
		user, err := c.iam.GetUserRequest(&iam.GetUserInput{}).Send()
		if err != nil {
			return nil, err
		}

		arnData, err := arn.Parse(*user.User.Arn)
		if err != nil {
			return nil, err
		}
		c.accountID = &arnData.AccountID
	}

	return c.accountID, nil
}

func (c *Client) getPolicyARN(policyName *string) (*string, error) {
	accountID, err := c.getAccountID()
	if err != nil {
		return nil, err
	}
	policyARN := fmt.Sprintf(policyArn, *accountID, *policyName)
	return &policyARN, nil
}

func (c *Client) createUser(username *string) error {
	_, err := c.iam.CreateUserRequest(&iam.CreateUserInput{UserName: username}).Send()
	if err != nil && isErrorAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *Client) createAccessKey(username *string) (*iam.AccessKey, error) {
	keysResponse, err := c.iam.CreateAccessKeyRequest(&iam.CreateAccessKeyInput{UserName: username}).Send()
	if err != nil {
		return nil, err
	}

	return keysResponse.AccessKey, nil
}

func (c *Client) createPolicy(policyName *string, policyDocument *string) (*string, error) {
	var currentVersion *string
	response, err := c.iam.CreatePolicyRequest(&iam.CreatePolicyInput{PolicyName: policyName, PolicyDocument: policyDocument}).Send()
	if err == nil {
		currentVersion = response.Policy.DefaultVersionId
	} else {
		if isErrorAlreadyExists(err) {
			currentVersion, err = c.UpdatePolicy(policyName, policyDocument)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return currentVersion, nil
}

func (c *Client) attachPolicyToUser(policyName *string, username *string) error {
	policyArn, err := c.getPolicyARN(policyName)
	if err != nil {
		return err
	}
	_, err = c.iam.AttachUserPolicyRequest(&iam.AttachUserPolicyInput{PolicyArn: policyArn, UserName: username}).Send()
	if err != nil {
		return err
	}
	return nil
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

type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

type StatementEntry struct {
	Sid      string
	Effect   string
	Action   []string
	Resource []string
}
