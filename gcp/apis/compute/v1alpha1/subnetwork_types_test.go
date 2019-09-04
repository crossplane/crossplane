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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGCPSubnetworkSpec_IsSameAs(t *testing.T) {
	cases := map[string]struct {
		spec   GCPSubnetworkSpec
		status GCPSubnetworkStatus
		result bool
	}{
		"FullMatchSliceOrderDifferent": {
			spec: GCPSubnetworkSpec{
				Description:           "some desc",
				Name:                  "some-name",
				EnableFlowLogs:        true,
				IPCidrRange:           "10.0.0.0/9",
				Network:               "test-network",
				PrivateIPGoogleAccess: true,
				Region:                "test-region",
				SecondaryIPRanges: []*GCPSubnetworkSecondaryRange{
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.0.0.0/9",
					},
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.0.1/9",
					},
				},
			},
			status: GCPSubnetworkStatus{
				Description:           "some desc",
				Name:                  "some-name",
				EnableFlowLogs:        true,
				IPCIDRRange:           "10.0.0.0/9",
				Network:               "test-network",
				PrivateIPGoogleAccess: true,
				Region:                "test-region",
				SecondaryIPRanges: []*GCPSubnetworkSecondaryRange{
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.0.1/9",
					},
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.0.0.0/9",
					},
				},
			},
			result: true,
		},
		"BiggerStatus": {
			spec: GCPSubnetworkSpec{
				Description:           "some desc",
				Name:                  "some-name",
				EnableFlowLogs:        true,
				IPCidrRange:           "10.0.0.0/9",
				Network:               "test-network",
				PrivateIPGoogleAccess: true,
				Region:                "test-region",
				SecondaryIPRanges: []*GCPSubnetworkSecondaryRange{
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.0.0.0/9",
					},
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.0.1/9",
					},
				},
			},
			status: GCPSubnetworkStatus{
				CreationTimestamp:     "some-timestamp",
				GatewayAddress:        "123.23.12.134",
				Kind:                  "compute#resource",
				Description:           "some desc",
				Name:                  "some-name",
				EnableFlowLogs:        true,
				IPCIDRRange:           "10.0.0.0/9",
				Network:               "test-network",
				PrivateIPGoogleAccess: true,
				Region:                "test-region",
				SecondaryIPRanges: []*GCPSubnetworkSecondaryRange{
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.0.1/9",
					},
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.0.0.0/9",
					},
				},
			},
			result: true,
		},
		"DifferentEnableFlowLogs": {
			spec: GCPSubnetworkSpec{
				Description:           "some desc",
				Name:                  "some-name",
				EnableFlowLogs:        true,
				IPCidrRange:           "10.0.0.0/9",
				Network:               "test-network",
				PrivateIPGoogleAccess: true,
				Region:                "test-region",
				SecondaryIPRanges: []*GCPSubnetworkSecondaryRange{
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.0.0.0/9",
					},
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.0.1/9",
					},
				},
			},
			status: GCPSubnetworkStatus{
				CreationTimestamp:     "some-timestamp",
				GatewayAddress:        "123.23.12.134",
				Kind:                  "compute#resource",
				Description:           "some desc",
				Name:                  "some-name",
				EnableFlowLogs:        false,
				IPCIDRRange:           "10.0.0.0/9",
				Network:               "test-network",
				PrivateIPGoogleAccess: true,
				Region:                "test-region",
				SecondaryIPRanges: []*GCPSubnetworkSecondaryRange{
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.0.1/9",
					},
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.0.0.0/9",
					},
				},
			},
			result: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.result, tc.spec.IsSameAs(tc.status)); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}
