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

package gke

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/container/v1"

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
)

const (
	// DefaultScope used by the GKE API.
	DefaultScope = container.CloudPlatformScope

	// TODO(negz): Is this username special? I can't see any ClusterRoleBindings
	// that bind it to a role.
	adminUser = "admin"
)

// Client interface to perform cluster operations
type Client interface {
	CreateCluster(string, computev1alpha1.GKEClusterSpec) (*container.Cluster, error)
	GetCluster(zone, name string) (*container.Cluster, error)
	DeleteCluster(zone, name string) error
}

// ClusterClient implementation
type ClusterClient struct {
	creds  *google.Credentials
	client *container.Service
}

// NewClusterClient return new instance of the Client based on credentials
func NewClusterClient(creds *google.Credentials) (*ClusterClient, error) {
	client, err := container.New(oauth2.NewClient(context.Background(), creds.TokenSource))
	if err != nil {
		return nil, err
	}

	return &ClusterClient{
		creds:  creds,
		client: client,
	}, nil
}

// CreateCluster creates a new GKE cluster.
func (c *ClusterClient) CreateCluster(name string, spec computev1alpha1.GKEClusterSpec) (*container.Cluster, error) {
	cr := &container.CreateClusterRequest{
		Cluster: &container.Cluster{
			Name:                  name,
			InitialClusterVersion: spec.ClusterVersion,
			InitialNodeCount:      spec.NumNodes,
			IpAllocationPolicy: &container.IPAllocationPolicy{
				UseIpAliases:          spec.EnableIPAlias,
				CreateSubnetwork:      spec.CreateSubnetwork,
				NodeIpv4CidrBlock:     spec.NodeIPV4CIDR,
				ClusterIpv4CidrBlock:  spec.ClusterIPV4CIDR,
				ServicesIpv4CidrBlock: spec.ServiceIPV4CIDR,
			},
			NodeConfig: &container.NodeConfig{
				MachineType: spec.MachineType,
				OauthScopes: spec.Scopes,
			},
			ResourceLabels: spec.Labels,
			Zone:           spec.Zone,

			// As of Kubernetes 1.12 GKE must be asked to generate a client cert
			// that will be available via the GKE MasterAuth API. The certificate is
			// generated with CN=client - a user with no RBAC permissions. Instead
			// we user basic auth, which is still granted full admin privileges.
			MasterAuth: &container.MasterAuth{
				Username: adminUser,
				ClientCertificateConfig: &container.ClientCertificateConfig{
					IssueClientCertificate: false,
				},
			},
		},
		ProjectId: c.creds.ProjectID,
		Zone:      spec.Zone,
	}

	if _, err := c.client.Projects.Zones.Clusters.Create(cr.ProjectId, spec.Zone, cr).Do(); err != nil {
		return nil, err
	}

	return c.GetCluster(spec.Zone, name)
}

// GetCluster retrieve GKE Cluster based on provided zone and name
func (c *ClusterClient) GetCluster(zone, name string) (*container.Cluster, error) {
	return c.client.Projects.Zones.Clusters.Get(c.creds.ProjectID, zone, name).Do()
}

// DeleteCluster in the given zone with the given name
func (c *ClusterClient) DeleteCluster(zone, name string) error {
	_, err := c.client.Projects.Zones.Clusters.Delete(c.creds.ProjectID, zone, name).Do()
	if err != nil {
		if gcp.IsErrorNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// DefaultKubernetesVersion is the default Kubernetes Cluster version supported by GKE for given project/zone
func (c *ClusterClient) DefaultKubernetesVersion(zone string) (string, error) {
	sc, err := c.client.Projects.Zones.GetServerconfig(c.creds.ProjectID, zone).Fields("validMasterVersions").Do()
	if err != nil {
		return "", err
	}

	return sc.DefaultClusterVersion, nil
}
