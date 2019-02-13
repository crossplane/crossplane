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

package provider

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockResourceGroupClient struct {
	MockCreateOrUpdateGroup    func(client *client.Client, name string, location string) error
	MockCheckResourceGroupName func(name string) error
}

func (m *mockResourceGroupClient) CreateOrUpdateGroup(client *client.Client, name string, location string) error {
	if m.MockCreateOrUpdateGroup != nil {
		return m.MockCreateOrUpdateGroup(client, name, location)
	}
	return nil
}

func (m *mockResourceGroupClient) CheckResourceGroupName(name string) error {
	if m.MockCheckResourceGroupName != nil {
		return m.MockCheckResourceGroupName(name)
	}
	return nil
}
