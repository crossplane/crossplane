/*
Copyright 2021 The Crossplane Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/crossplane/crossplane/internal/client/clientset/versioned/typed/pkg/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakePkgV1alpha1 struct {
	*testing.Fake
}

func (c *FakePkgV1alpha1) Configurations() v1alpha1.ConfigurationInterface {
	return &FakeConfigurations{c}
}

func (c *FakePkgV1alpha1) ConfigurationRevisions() v1alpha1.ConfigurationRevisionInterface {
	return &FakeConfigurationRevisions{c}
}

func (c *FakePkgV1alpha1) ControllerConfigs() v1alpha1.ControllerConfigInterface {
	return &FakeControllerConfigs{c}
}

func (c *FakePkgV1alpha1) Providers() v1alpha1.ProviderInterface {
	return &FakeProviders{c}
}

func (c *FakePkgV1alpha1) ProviderRevisions() v1alpha1.ProviderRevisionInterface {
	return &FakeProviderRevisions{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakePkgV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
