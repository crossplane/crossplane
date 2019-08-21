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
	"github.com/crossplaneio/crossplane/aws/apis/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks"
)

// MockEKSClient mock client for EKS
type MockEKSClient struct {
	MockCreate            func(string, v1alpha1.EKSClusterSpec) (*eks.Cluster, error)
	MockGet               func(string) (*eks.Cluster, error)
	MockDelete            func(name string) error
	MockConnectionToken   func(string) (string, error)
	MockCreateWorkerNodes func(string, string, v1alpha1.EKSClusterSpec) (*eks.ClusterWorkers, error)
	MockGetWorkerNodes    func(string) (*eks.ClusterWorkers, error)
	MockDeleteWorkerNodes func(string) error
}

// Create EKS Cluster with provided Specification
func (m *MockEKSClient) Create(name string, spec v1alpha1.EKSClusterSpec) (*eks.Cluster, error) {
	return m.MockCreate(name, spec)
}

// Get mock EKS Cluster by name
func (m *MockEKSClient) Get(name string) (*eks.Cluster, error) {
	return m.MockGet(name)
}

// Delete mock EKS Cluster
func (m *MockEKSClient) Delete(name string) error {
	return m.MockDelete(name)
}

// ConnectionToken mock
func (m *MockEKSClient) ConnectionToken(name string) (string, error) {
	return m.MockConnectionToken(name)
}

// CreateWorkerNodes mock
func (m *MockEKSClient) CreateWorkerNodes(name string, version string, spec v1alpha1.EKSClusterSpec) (*eks.ClusterWorkers, error) {
	return m.MockCreateWorkerNodes(name, version, spec)
}

// GetWorkerNodes mock
func (m *MockEKSClient) GetWorkerNodes(stackID string) (*eks.ClusterWorkers, error) {
	return m.MockGetWorkerNodes(stackID)
}

// DeleteWorkerNodes mock
func (m *MockEKSClient) DeleteWorkerNodes(stackID string) error {
	return m.MockDeleteWorkerNodes(stackID)
}
