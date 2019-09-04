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

package network

import (
	googlecompute "google.golang.org/api/compute/v1"

	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
)

// GenerateNetwork takes a *GCPNetworkSpec and returns *googlecompute.Network.
// It assigns only the fields that are writable, i.e. not labelled as [Output Only]
// in Google's reference.
func GenerateNetwork(in v1alpha1.GCPNetworkSpec) *googlecompute.Network {
	n := &googlecompute.Network{}
	n.IPv4Range = in.IPv4Range
	if in.AutoCreateSubnetworks != nil {
		n.AutoCreateSubnetworks = *in.AutoCreateSubnetworks
		if !n.AutoCreateSubnetworks {
			n.ForceSendFields = []string{"AutoCreateSubnetworks"}
		}
	}
	n.Description = in.Description
	n.Name = in.Name
	if in.RoutingConfig != nil {
		n.RoutingConfig = &googlecompute.NetworkRoutingConfig{
			RoutingMode: in.RoutingConfig.RoutingMode,
		}
	}
	return n
}

// GenerateGCPNetworkStatus takes a googlecompute.Network and returns *GCPNetworkStatus
// It assings all the fields.
func GenerateGCPNetworkStatus(in googlecompute.Network) v1alpha1.GCPNetworkStatus {
	gn := v1alpha1.GCPNetworkStatus{
		IPv4Range:             in.IPv4Range,
		AutoCreateSubnetworks: in.AutoCreateSubnetworks,
		CreationTimestamp:     in.CreationTimestamp,
		Description:           in.Description,
		GatewayIPv4:           in.GatewayIPv4,
		ID:                    in.Id,
		SelfLink:              in.SelfLink,
		Subnetworks:           in.Subnetworks,
	}
	if in.RoutingConfig != nil {
		gn.RoutingConfig = &v1alpha1.GCPNetworkRoutingConfig{
			RoutingMode: in.RoutingConfig.RoutingMode,
		}
	}
	for _, p := range in.Peerings {
		gp := &v1alpha1.GCPNetworkPeering{
			Name:                 p.Name,
			Network:              p.Network,
			State:                p.State,
			AutoCreateRoutes:     p.AutoCreateRoutes,
			ExchangeSubnetRoutes: p.ExchangeSubnetRoutes,
			StateDetails:         p.StateDetails,
		}
		gn.Peerings = append(gn.Peerings, gp)
	}
	return gn
}
