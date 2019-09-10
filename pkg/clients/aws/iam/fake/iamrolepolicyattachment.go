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

package fake

import (
	"github.com/aws/aws-sdk-go-v2/service/iam"

	clientset "github.com/crossplaneio/crossplane/pkg/clients/aws/iam"
)

// this ensures that the mock implements the client interface
var _ clientset.RolePolicyAttachmentClient = (*MockRolePolicyAttachmentClient)(nil)

// MockRolePolicyAttachmentClient is a type that implements all the methods for RolePolicyAttachmentClient interface
type MockRolePolicyAttachmentClient struct {
	MockAttachRolePolicyRequest         func(*iam.AttachRolePolicyInput) iam.AttachRolePolicyRequest
	MockListAttachedRolePoliciesRequest func(*iam.ListAttachedRolePoliciesInput) iam.ListAttachedRolePoliciesRequest
	MockDetachRolePolicyRequest         func(*iam.DetachRolePolicyInput) iam.DetachRolePolicyRequest
}

// AttachRolePolicyRequest mocks AttachRolePolicyRequest method
func (m *MockRolePolicyAttachmentClient) AttachRolePolicyRequest(input *iam.AttachRolePolicyInput) iam.AttachRolePolicyRequest {
	return m.MockAttachRolePolicyRequest(input)
}

// ListAttachedRolePoliciesRequest mocks ListAttachedRolePoliciesRequest method
func (m *MockRolePolicyAttachmentClient) ListAttachedRolePoliciesRequest(input *iam.ListAttachedRolePoliciesInput) iam.ListAttachedRolePoliciesRequest {
	return m.MockListAttachedRolePoliciesRequest(input)
}

// DetachRolePolicyRequest mocks DetachRolePolicyRequest method
func (m *MockRolePolicyAttachmentClient) DetachRolePolicyRequest(input *iam.DetachRolePolicyInput) iam.DetachRolePolicyRequest {
	return m.MockDetachRolePolicyRequest(input)
}
