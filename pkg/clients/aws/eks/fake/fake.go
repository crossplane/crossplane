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
	"github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks"
)

type MockEKSClient struct {
	MockCreate          func(string, v1alpha1.EKSClusterSpec) (*eks.Cluster, error)
	MockGet             func(string) (*eks.Cluster, error)
	MockDelete          func(name string) error
	MockConnectionToken func(string) (string, error)
}

// Create EKS Cluster with provided Specification
func (m *MockEKSClient) Create(name string, spec v1alpha1.EKSClusterSpec) (*eks.Cluster, error) {
	return m.MockCreate(name, spec)
}

// Get EKS Cluster by name
func (m *MockEKSClient) Get(name string) (*eks.Cluster, error) {
	return m.MockGet(name)
}

// Delete EKS Cluster
func (m *MockEKSClient) Delete(name string) error {
	return m.MockDelete(name)
}

func (m *MockEKSClient) ConnectionToken(name string) (string, error) {
	return m.MockConnectionToken(name)
}
