/*
Copyright 2018 The Conductor Authors.

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

package rds

import "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"

type MockClient struct {
	MockGetInstance    func(string) (*Instance, error)
	MockCreateInstance func(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error)
	MockDeleteInstance func(name string) (*Instance, error)
}

// GetInstance finds RDS Instance by name
func (m *MockClient) GetInstance(name string) (*Instance, error) {
	return m.MockGetInstance(name)
}

// CreateInstance creates RDS Instance with provided Specification
func (m *MockClient) CreateInstance(name, password string, spec *v1alpha1.RDSInstanceSpec) (*Instance, error) {
	return m.MockCreateInstance(name, password, spec)
}

// DeleteInstance deletes RDS Instance
func (m *MockClient) DeleteInstance(name string) (*Instance, error) {
	return m.MockDeleteInstance(name)
}
