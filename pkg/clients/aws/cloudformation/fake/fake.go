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
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
)

// MockCloudFormationClient mock
type MockCloudFormationClient struct {
	MockCreateStack func(stackName *string, templateBody *string, parameters map[string]string) (stackID *string, err error)
	MockGetStack    func(stackID *string) (status *cloudformation.Stack, err error)
	MockDeleteStack func(stackID *string) error
}

// CreateStack mock
func (m *MockCloudFormationClient) CreateStack(stackName *string, templateBody *string, parameters map[string]string) (stackID *string, err error) {
	return m.MockCreateStack(stackName, templateBody, parameters)
}

// GetStack mock
func (m *MockCloudFormationClient) GetStack(stackID *string) (status *cloudformation.Stack, err error) {
	return m.MockGetStack(stackID)
}

// DeleteStack mock
func (m *MockCloudFormationClient) DeleteStack(stackID *string) error {
	return m.MockDeleteStack(stackID)
}
