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
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2018-03-31/containerservice"
	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
)

type FakeAKSClient struct {
	MockCreate          func(string, computev1alpha1.AKSClusterSpec) (containerservice.ManagedCluster, error)
	MockGet             func(string, string) (containerservice.ManagedCluster, error)
	MockDelete          func(string, string) error
	MockListCredentials func(string, string) (containerservice.CredentialResult, error)
}

func (f *FakeAKSClient) Create(name string, spec computev1alpha1.AKSClusterSpec) (containerservice.ManagedCluster, error) {
	return f.MockCreate(name, spec)
}

func (f *FakeAKSClient) Get(group, name string) (containerservice.ManagedCluster, error) {
	return f.MockGet(group, name)
}

func (f *FakeAKSClient) Delete(group, name string) error {
	return f.MockDelete(group, name)
}

func (f *FakeAKSClient) ListCredentials(group, name string) (containerservice.CredentialResult, error) {
	return f.MockListCredentials(group, name)
}
