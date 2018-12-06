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
	"context"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2018-03-31/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	. "github.com/crossplaneio/crossplane/pkg/clients/azure"
)

var (
	NotFoundError          = autorest.DetailedError{StatusCode: http.StatusNotFound}
	BadRequestError        = autorest.DetailedError{StatusCode: http.StatusBadRequest}
	InternalServerError    = autorest.DetailedError{StatusCode: http.StatusInternalServerError}
	BasicManagedCluster    = containerservice.ManagedCluster{}
	BasicCredentialsResult = containerservice.CredentialResult{}
)

type MockCreateClusterFunction func(ctx context.Context, name string, clientID string, clientSecret string, spec computev1alpha1.AKSClusterSpec) (*computev1alpha1.AKSClusterFuture, error)
type MockGetClusterFunction func(ctx context.Context, group string, name string) (containerservice.ManagedCluster, error)
type MockListCredentialsFunction func(group string, name string) (containerservice.CredentialResult, error)

type FakeAKSClient struct {
	MockCreateCluster   MockCreateClusterFunction
	MockGetCluster      MockGetClusterFunction
	MockDeleteCluster   func(ctx context.Context, group string, name string) error
	MockListCredentials MockListCredentialsFunction

	MockWaitForCompletion func(context.Context, *computev1alpha1.AKSClusterFuture) error
	MockDoneWithContext   func(context.Context, *computev1alpha1.AKSClusterFuture) (bool, error)
	MockGetResult         func(*computev1alpha1.AKSClusterFuture) (*http.Response, error)
}

func (f *FakeAKSClient) CreateCluster(ctx context.Context, name, clientID, clientSecret string, spec computev1alpha1.AKSClusterSpec) (*computev1alpha1.AKSClusterFuture, error) {
	return f.MockCreateCluster(ctx, name, clientID, clientSecret, spec)
}

func (f *FakeAKSClient) GetCluster(ctx context.Context, group, name string) (containerservice.ManagedCluster, error) {
	return f.MockGetCluster(ctx, group, name)
}

func (f *FakeAKSClient) DeleteCluster(ctx context.Context, group, name string) error {
	return f.MockDeleteCluster(ctx, group, name)
}

func (f *FakeAKSClient) ListCredentials(group, name string) (containerservice.CredentialResult, error) {
	return f.MockListCredentials(group, name)
}

func (f *FakeAKSClient) WaitForCompletion(ctx context.Context, future *computev1alpha1.AKSClusterFuture) error {
	return f.MockWaitForCompletion(ctx, future)
}

func (f *FakeAKSClient) DoneWithContext(ctx context.Context, future *computev1alpha1.AKSClusterFuture) (bool, error) {
	return f.MockDoneWithContext(ctx, future)
}

func (f *FakeAKSClient) GetResult(future *computev1alpha1.AKSClusterFuture) (*http.Response, error) {
	return f.MockGetResult(future)
}

func NewManagedClusterWithState(state string) containerservice.ManagedCluster {
	return containerservice.ManagedCluster{
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			ProvisioningState: to.StringPtr(state),
		},
	}
}

func NewMockCreateClusterFunction(f *computev1alpha1.AKSClusterFuture, e error) MockCreateClusterFunction {
	return func(ctx context.Context, name, clientID, clientSecret string, spec computev1alpha1.AKSClusterSpec) (*computev1alpha1.AKSClusterFuture, error) {
		return f, e
	}
}

func NewMockGetClusterFunction(cluster containerservice.ManagedCluster, err error) MockGetClusterFunction {
	return func(ctx context.Context, group string, name string) (containerservice.ManagedCluster, error) {
		return cluster, err
	}
}

func NewMockGetClusterFunctionWithState(state string, err error) MockGetClusterFunction {
	return NewMockGetClusterFunction(NewManagedClusterWithState(state), err)
}

func NewMockListCredentialsFunction(name string, value []byte, err error) MockListCredentialsFunction {
	return func(string, string) (credentialResult containerservice.CredentialResult, e error) {
		return containerservice.CredentialResult{
			Name:  &name,
			Value: &value,
		}, err
	}
}

func NewMockFutureFromResponse(response *http.Response) (*computev1alpha1.AKSClusterFuture, error) {
	f, err := azure.NewFutureFromResponse(response)
	if err != nil {
		return nil, err
	}
	af := computev1alpha1.AKSClusterFuture(containerservice.ManagedClustersCreateOrUpdateFuture{Future: f})
	return &af, err
}

func NewMockFutureFromResponseValues(host, method string, statusCode int) (*computev1alpha1.AKSClusterFuture, error) {
	// test future
	resp := &http.Response{
		Status:     http.StatusText(statusCode),
		StatusCode: statusCode,
		Request: &http.Request{
			Method: method,
			URL: &url.URL{
				Host: host,
			},
		},
	}
	return NewMockFutureFromResponse(resp)
}

// ---------------------------------------------------------------------------------------------------------------------
type MockCreateApplicationFunction func(ctx context.Context, name, url string, password graphrbac.PasswordCredential) (*graphrbac.Application, error)

type FakeApplicationClient struct {
	MockCreateApplication MockCreateApplicationFunction
	MockGetApplication    func(ctx context.Context, objectID string) (*graphrbac.Application, error)
	MockDeleteApplication func(ctx context.Context, objectID string) error
}

func (f *FakeApplicationClient) CreateApplication(ctx context.Context, name, url string, password graphrbac.PasswordCredential) (*graphrbac.Application, error) {
	return f.MockCreateApplication(ctx, name, url, password)
}

func (f *FakeApplicationClient) GetApplication(ctx context.Context, objectID string) (*graphrbac.Application, error) {
	return f.MockGetApplication(ctx, objectID)
}

func (f *FakeApplicationClient) DeleteApplication(ctx context.Context, objectID string) error {
	return f.MockDeleteApplication(ctx, objectID)
}

func NewMockCreateApplicationFunction(app *graphrbac.Application, err error) MockCreateApplicationFunction {
	return func(ctx context.Context, name, url string, password graphrbac.PasswordCredential) (*graphrbac.Application, error) {
		return app, err
	}
}

// ---------------------------------------------------------------------------------------------------------------------
type FakeServicePrincipalClient struct {
	MockCreateServicePrincipal func(ctx context.Context, appID string) (*graphrbac.ServicePrincipal, error)
	MockGetServicePrincipal    func(ctx context.Context, objectID string) (*graphrbac.ServicePrincipal, error)
	MockDeleteServicePrincipal func(ctx context.Context, objectID string) error
}

func (f *FakeServicePrincipalClient) CreateServicePrincipal(ctx context.Context, appID string) (*graphrbac.ServicePrincipal, error) {
	return f.MockCreateServicePrincipal(ctx, appID)
}

func (f *FakeServicePrincipalClient) GetServicePrincipal(ctx context.Context, objectID string) (*graphrbac.ServicePrincipal, error) {
	return f.MockGetServicePrincipal(ctx, objectID)
}

func (f *FakeServicePrincipalClient) DeleteServicePrincipal(ctx context.Context, objectID string) error {
	return f.MockDeleteServicePrincipal(ctx, objectID)
}

// ---------------------------------------------------------------------------------------------------------------------
type FakeAKSClientset struct {
	*FakeApplicationClient
	*FakeServicePrincipalClient
	*FakeAKSClient

	MockDelete func(ctx context.Context, group, name, appID string) error
}

func NewFakeAKSClientset() *FakeAKSClientset {
	return &FakeAKSClientset{
		FakeApplicationClient:      &FakeApplicationClient{},
		FakeServicePrincipalClient: &FakeServicePrincipalClient{},
		FakeAKSClient:              &FakeAKSClient{},
	}
}

func (f *FakeAKSClientset) Delete(ctx context.Context, group, name, appID string) error {
	return f.MockDelete(ctx, group, name, appID)
}

//------------------------------------------------------------------------------------------------------------
type FakeAKSClientsetFactory struct {
	MockClientset *FakeAKSClientset
	MockError     error
}

func NewMockAKSClientsetFactory(clientset *FakeAKSClientset, err error) *FakeAKSClientsetFactory {
	return &FakeAKSClientsetFactory{
		MockClientset: clientset,
		MockError:     err,
	}
}

func (a *FakeAKSClientsetFactory) NewAKSClientset(config *ClientCredentialsConfig) (AKSClientsetAPI, error) {
	return a.MockClientset, a.MockError
}
