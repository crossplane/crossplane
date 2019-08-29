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

package subnetwork

import (
	"google.golang.org/api/compute/v1"

	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
)

// GenerateSubnetwork creates a *googlecompute.Subnetwork object using GCPSubnetworkSpec.
func GenerateSubnetwork(s v1alpha1.GCPSubnetworkSpec) *compute.Subnetwork {
	sn := &compute.Subnetwork{}
	sn.Name = s.Name
	sn.Description = s.Description
	sn.EnableFlowLogs = s.EnableFlowLogs
	sn.IpCidrRange = s.IPCidrRange
	sn.Network = s.Network
	sn.PrivateIpGoogleAccess = s.PrivateIPGoogleAccess
	sn.Region = s.Region
	for _, val := range s.SecondaryIPRanges {
		obj := &compute.SubnetworkSecondaryRange{
			IpCidrRange: val.IPCidrRange,
			RangeName:   val.RangeName,
		}
		sn.SecondaryIpRanges = append(sn.SecondaryIpRanges, obj)
	}
	return sn
}

// GenerateGCPSubnetworkStatus creates a GCPSubnetworkStatus object using *googlecompute.Subnetwork.
func GenerateGCPSubnetworkStatus(in *compute.Subnetwork) v1alpha1.GCPSubnetworkStatus {
	s := v1alpha1.GCPSubnetworkStatus{}
	s.Name = in.Name
	s.Description = in.Description
	s.EnableFlowLogs = in.EnableFlowLogs
	s.Fingerprint = in.Fingerprint
	s.IPCIDRRange = in.IpCidrRange
	s.Network = in.Network
	s.PrivateIPGoogleAccess = in.PrivateIpGoogleAccess
	s.Region = in.Region
	for _, val := range in.SecondaryIpRanges {
		obj := &v1alpha1.GCPSubnetworkSecondaryRange{
			IPCidrRange: val.IpCidrRange,
			RangeName:   val.RangeName,
		}
		s.SecondaryIPRanges = append(s.SecondaryIPRanges, obj)
	}
	s.CreationTimestamp = in.CreationTimestamp
	s.GatewayAddress = in.GatewayAddress
	s.ID = in.Id
	s.Kind = in.Kind
	s.SelfLink = in.SelfLink
	return s
}
