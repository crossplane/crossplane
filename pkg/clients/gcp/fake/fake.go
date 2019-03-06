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
	"google.golang.org/api/container/v1"

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
)

// FakeGKEClient for mocking.
type FakeGKEClient struct {
	MockCreateCluster func(string, computev1alpha1.GKEClusterSpec) (*container.Cluster, error)
	MockGetCluster    func(string, string) (*container.Cluster, error)
	MockDeleteCluster func(string, string) error
}

// CreateCluster calls the underlying MockCreateCluster method.
func (f *FakeGKEClient) CreateCluster(name string, spec computev1alpha1.GKEClusterSpec) (*container.Cluster, error) {
	return f.MockCreateCluster(name, spec)
}

// GetCluster calls the underlying MockGetCluster method.
func (f *FakeGKEClient) GetCluster(zone, name string) (*container.Cluster, error) {
	return f.MockGetCluster(zone, name)
}

// DeleteCluster calls the underlying MockDeleteCluster method.
func (f *FakeGKEClient) DeleteCluster(zone, name string) error {
	return f.MockDeleteCluster(zone, name)
}

// NewGKEClient returns a fake GKE client for testing.
func NewGKEClient() *FakeGKEClient {
	return &FakeGKEClient{}
}
