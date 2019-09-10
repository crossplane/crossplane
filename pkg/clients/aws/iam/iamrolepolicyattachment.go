package iam

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// RolePolicyAttachmentClient is the external client used for IAMRolePolicyAttachment Custom Resource
type RolePolicyAttachmentClient interface {
	AttachRolePolicyRequest(*iam.AttachRolePolicyInput) iam.AttachRolePolicyRequest
	ListAttachedRolePoliciesRequest(*iam.ListAttachedRolePoliciesInput) iam.ListAttachedRolePoliciesRequest
	DetachRolePolicyRequest(*iam.DetachRolePolicyInput) iam.DetachRolePolicyRequest
}

// NewRolePolicyAttachmentClient returns a new client given an aws config
func NewRolePolicyAttachmentClient(conf *aws.Config) (RolePolicyAttachmentClient, error) {
	return iam.New(*conf), nil
}
