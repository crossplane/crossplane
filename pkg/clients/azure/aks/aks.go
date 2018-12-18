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

package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2018-03-31/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	AgentPoolProfileNameFmt = "%s-nodepool"
	MaxClusterNameLength    = 31
)

// Client interface for AKS Cluster client
type Client interface {
	Create(string, computev1alpha1.AKSClusterSpec) (containerservice.ManagedCluster, error)
	Get(string, string) (containerservice.ManagedCluster, error)
	Delete(string, string) error
	ListCredentials(string, string) (containerservice.CredentialResult, error)
}

// NewAKSClient return AKS client implementation
func NewAKSClient(config *azure.ClientCredentialsConfig) (Client, error) {
	authorizer, err := config.Authorizer()
	if err != nil {
		return nil, err
	}

	client := containerservice.NewManagedClustersClient(config.SubscriptionID)
	client.Authorizer = authorizer

	if err := client.AddToUserAgent(azure.UserAgent); err != nil {
		return nil, err
	}

	return &AKSClient{
		client:                  &client,
		ClientCredentialsConfig: config,
	}, nil
}

// AKSClient implementing Client interface
type AKSClient struct {
	client *containerservice.ManagedClustersClient
	*azure.ClientCredentialsConfig
}

// Create new AKS Cluster with given name and specs
// **NOTE**: the actual cluster name can be different from the provided name
func (a *AKSClient) Create(name string, spec computev1alpha1.AKSClusterSpec) (containerservice.ManagedCluster, error) {
	name = trimName(name)

	nodeCount := int32(computev1alpha1.DefaultNodeCount)
	if spec.NodeCount != nil {
		nodeCount = int32(*spec.NodeCount)
	}

	agentPoolProfileName := fmt.Sprintf(AgentPoolProfileNameFmt, name)
	enableRBAC := !spec.DisableRBAC

	// create (or update) - for now we care about creation only
	_, err := a.client.CreateOrUpdate(
		context.Background(),
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
					ClientID: to.StringPtr(a.ClientID),
					Secret:   to.StringPtr(a.ClientSecret),
				},
				EnableRBAC: &enableRBAC,
			},
		},
	)
	if err != nil {
		return containerservice.ManagedCluster{}, err
	}

	// retrieve newly created cluster
	return a.Get(spec.ResourceGroupName, name)
}

// Get AKS cluster with given group and name values
func (a *AKSClient) Get(group, name string) (containerservice.ManagedCluster, error) {
	return a.client.Get(context.Background(), group, name)
}

// Delete AKS clsuter with given group and name values
func (a *AKSClient) Delete(group, name string) error {
	_, err := a.client.Delete(context.Background(), group, name)
	return err
}

func (a *AKSClient) ListCredentials(group, name string) (containerservice.CredentialResult, error) {
	creds, err := a.client.ListClusterAdminCredentials(context.Background(), group, name)
	if err != nil {
		return containerservice.CredentialResult{}, nil
	}
	if creds.Kubeconfigs == nil || len(*creds.Kubeconfigs) == 0 {
		return containerservice.CredentialResult{}, fmt.Errorf("cluster admin credentials are not found")
	}

	return (*creds.Kubeconfigs)[0], nil
}

func trimName(name string) string {
	if len(name) > MaxClusterNameLength {
		name = name[:MaxClusterNameLength]
	}

	return strings.TrimSuffix(name, "-")
}
