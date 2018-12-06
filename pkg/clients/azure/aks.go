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

package azure

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2018-03-31/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	AgentPoolProfileNameFmt = "%s-nodepool"
	MaxClusterNameLength    = 31
)

// AKSClientAPI interface for AKS Cluster client
type AKSClientAPI interface {
	CreateCluster(ctx context.Context, name string, clientID string, clientSecret string, spec v1alpha1.AKSClusterSpec) (*v1alpha1.AKSClusterFuture, error)
	GetCluster(ctx context.Context, group string, name string) (containerservice.ManagedCluster, error)
	DeleteCluster(ctx context.Context, group string, name string) error
	ListCredentials(group string, name string) (containerservice.CredentialResult, error)

	WaitForCompletion(context.Context, *v1alpha1.AKSClusterFuture) error
	DoneWithContext(context.Context, *v1alpha1.AKSClusterFuture) (bool, error)
	GetResult(future *v1alpha1.AKSClusterFuture) (*http.Response, error)
}

// AKSClient implementing AKSClientAPI interface
type AKSClient struct {
	*containerservice.ManagedClustersClient
	*ClientCredentialsConfig
}

// NewAKSClient return AKS client implementation
func NewAKSClient(config *ClientCredentialsConfig) (AKSClientAPI, error) {
	authorizer, err := config.Authorizer()
	if err != nil {
		return nil, err
	}
	aksClient := containerservice.NewManagedClustersClient(config.SubscriptionID)
	aksClient.Authorizer = authorizer
	if err := aksClient.AddToUserAgent(UserAgent); err != nil {
		return nil, err
	}

	return &AKSClient{
		ManagedClustersClient:   &aksClient,
		ClientCredentialsConfig: config,
	}, nil
}

// CreateCluster new AKS Cluster with given name and specs
func (a *AKSClient) CreateCluster(ctx context.Context, name, clientID, clientSecret string, spec v1alpha1.AKSClusterSpec) (*v1alpha1.AKSClusterFuture, error) {
	nodeCount := int32(v1alpha1.DefaultNodeCount)
	if spec.NodeCount != nil {
		nodeCount = int32(*spec.NodeCount)
	}

	agentPoolProfileName := fmt.Sprintf(AgentPoolProfileNameFmt, name)
	enableRBAC := !spec.DisableRBAC

	// create (or update) - for now we care about creation only
	f, err := a.CreateOrUpdate(
		ctx,
		spec.ResourceGroupName,
		name,
		containerservice.ManagedCluster{
			Name:     &name,
			Location: util.String(spec.Location),
			ManagedClusterProperties: &containerservice.ManagedClusterProperties{
				KubernetesVersion: &spec.Version,
				DNSPrefix:         &name,
				AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{
					{
						Count:  to.Int32Ptr(nodeCount),
						Name:   to.StringPtr(agentPoolProfileName),
						VMSize: containerservice.VMSizeTypes(spec.NodeVMSize),
					},
				},
				ServicePrincipalProfile: &containerservice.ManagedClusterServicePrincipalProfile{
					ClientID: to.StringPtr(clientID),
					Secret:   to.StringPtr(clientSecret),
				},
				EnableRBAC: &enableRBAC,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	af := v1alpha1.AKSClusterFuture(f)

	return &af, nil
}

// GetCluster AKS cluster with given group and name values
func (a *AKSClient) GetCluster(ctx context.Context, group, name string) (containerservice.ManagedCluster, error) {
	return a.ManagedClustersClient.Get(context.Background(), group, name)
}

// DeleteCluster AKS cluster with given group and name values
func (a *AKSClient) DeleteCluster(ctx context.Context, group, name string) error {
	_, err := a.ManagedClustersClient.Delete(context.Background(), group, name)
	return err
}

// ListCredentials for AKS Kubernetes cluster
func (a *AKSClient) ListCredentials(group, name string) (containerservice.CredentialResult, error) {
	creds, err := a.ListClusterAdminCredentials(context.Background(), group, name)
	if err != nil {
		return containerservice.CredentialResult{}, nil
	}
	if creds.Kubeconfigs == nil || len(*creds.Kubeconfigs) == 0 {
		return containerservice.CredentialResult{}, fmt.Errorf("cluster admin credentials are not found")
	}

	return (*creds.Kubeconfigs)[0], nil
}

// WaitForCompletion waits for cluster future operation to finish
func (a *AKSClient) WaitForCompletion(ctx context.Context, future *v1alpha1.AKSClusterFuture) error {
	return future.WaitForCompletionRef(ctx, a.Client)
}

// DoneWithContext check if cluster future operation is completed
func (a *AKSClient) DoneWithContext(ctx context.Context, future *v1alpha1.AKSClusterFuture) (bool, error) {
	return future.DoneWithContext(ctx, a.Client)
}

// GetResult of the AKS Cluster operation (future)
func (a *AKSClient) GetResult(future *v1alpha1.AKSClusterFuture) (*http.Response, error) {
	sender := autorest.DecorateSender(a, autorest.DoRetryForStatusCodes(a.RetryAttempts, a.RetryDuration, autorest.StatusCodesForRetry...))
	return future.GetResult(sender)
}

//---------------------------------------------------------------------------------------------------------------------

// AKSClientsetAPI collection of three clients needed to operate on AD Application, ServicePrincipal and AKS Cluster
type AKSClientsetAPI interface {
	ApplicationAPI
	ServicePrincipalAPI
	AKSClientAPI
	Delete(context.Context, string, string, string) error
}

// AKSClientset
type AKSClientset struct {
	ApplicationAPI
	ServicePrincipalAPI
	AKSClientAPI
}

// NewAKSClientset
func NewAKSClientset(config *ClientCredentialsConfig) (AKSClientsetAPI, error) {
	appClient, err := NewApplicationClient(config)
	if err != nil {
		return nil, err
	}

	spClient, err := NewServicePrincipalClient(config)
	if err != nil {
		return nil, err
	}

	aksClient, err := NewAKSClient(config)
	if err != nil {
		return nil, err
	}

	return &AKSClientset{
		ApplicationAPI:      appClient,
		ServicePrincipalAPI: spClient,
		AKSClientAPI:        aksClient,
	}, nil
}

// Delete cluster and AD App
// Note: service principal is automatically deleted with AD App, hence no need for a dedicated delete call
func (a *AKSClientset) Delete(ctx context.Context, group, name, appID string) error {
	if err := a.DeleteCluster(ctx, group, name); err != nil && !IsErrorNotFound(err) {
		return err
	}

	if err := a.DeleteApplication(ctx, appID); err != nil && !IsErrorNotFound(err) {
		return err
	}

	return nil
}

//---------------------------------------------------------------------------------------------------------------------
// AKSClientsetFactoryAPI creates new AKSClientset instance
type AKSClientsetFactoryAPI interface {
	NewAKSClientset(*ClientCredentialsConfig) (AKSClientsetAPI, error)
}

// AKSClientsetFactory
type AKSClientsetFactory struct{}

// NewAKSClientset
func (a *AKSClientsetFactory) NewAKSClientset(config *ClientCredentialsConfig) (AKSClientsetAPI, error) {
	return NewAKSClientset(config)
}
