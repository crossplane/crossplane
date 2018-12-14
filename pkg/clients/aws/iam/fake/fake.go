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

package fake

import (
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

type MockIAMClient struct {
	MockCreateUser            func(username *string) (*iam.AccessKey, error)
	MockDeleteUser            func(username *string) error
	MockGetPolicyVersion      func(username *string) (*string, error)
	MockCreatePolicyAndAttach func(username *string, policyName *string, policyDocument *string) (*string, error)
	MockUpdatePolicy          func(username *string, policyDocument *string) (*string, error)
	MockDeletePolicyAndDetach func(username *string, policyName *string) error
}

func (m *MockIAMClient) CreateUser(username *string) (*iam.AccessKey, error) {
	return m.MockCreateUser(username)
}
func (m *MockIAMClient) DeleteUser(username *string) error {
	return m.MockDeleteUser(username)
}

func (m *MockIAMClient) GetPolicyVersion(username *string) (*string, error) {
	return m.MockGetPolicyVersion(username)
}

func (m *MockIAMClient) CreatePolicyAndAttach(username *string, policyName *string, policyDocument *string) (*string, error) {
	return m.MockCreatePolicyAndAttach(username, policyName, policyDocument)
}

func (m *MockIAMClient) UpdatePolicy(username *string, policyDocument *string) (*string, error) {
	return m.MockUpdatePolicy(username, policyDocument)
}

func (m *MockIAMClient) DeletePolicyAndDetach(username *string, policyName *string) error {
	return m.MockDeletePolicyAndDetach(username, policyName)
}
