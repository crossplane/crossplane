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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/compute/v1"

	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
)

const (
	testName        = "some-name"
	testDescription = "some desc"
	testIPCIDRRange = "10.0.0.0/9"
	testNetwork     = "test-network"
	testRegion      = "test-region"
)

var equateSubnetworkSecondaryRange = func(i, j *compute.SubnetworkSecondaryRange) bool { return i.RangeName > j.RangeName }
var equateGCPSubnetworkSecondaryRange = func(i, j *v1alpha1.GCPSubnetworkSecondaryRange) bool { return i.RangeName > j.RangeName }

func TestGCPSubnetworkSpec_GenerateSubnetwork(t *testing.T) {
	cases := map[string]struct {
		in  v1alpha1.GCPSubnetworkSpec
		out *compute.Subnetwork
	}{
		"FilledGeneration": {
			in: v1alpha1.GCPSubnetworkSpec{
				Description:           testDescription,
				Name:                  testName,
				EnableFlowLogs:        true,
				IPCidrRange:           testIPCIDRRange,
				Network:               testNetwork,
				PrivateIPGoogleAccess: true,
				Region:                testRegion,
				SecondaryIPRanges: []*v1alpha1.GCPSubnetworkSecondaryRange{
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.1.0.0/9",
					},
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.2.1/9",
					},
				},
			},
			out: &compute.Subnetwork{
				Description:           testDescription,
				Name:                  testName,
				EnableFlowLogs:        true,
				IpCidrRange:           testIPCIDRRange,
				Network:               testNetwork,
				PrivateIpGoogleAccess: true,
				Region:                testRegion,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						RangeName:   "aazz",
						IpCidrRange: "10.0.2.1/9",
					},
					{
						RangeName:   "zzaa",
						IpCidrRange: "10.1.0.0/9",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := GenerateSubnetwork(tc.in)
			if diff := cmp.Diff(tc.out, r, cmpopts.SortSlices(equateSubnetworkSecondaryRange)); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_GenerateSubnetwork(t *testing.T) {
	cases := map[string]struct {
		in  *compute.Subnetwork
		out v1alpha1.GCPSubnetworkStatus
	}{
		"FilledGeneration": {
			out: v1alpha1.GCPSubnetworkStatus{
				Description:           testDescription,
				Name:                  testName,
				EnableFlowLogs:        true,
				IPCIDRRange:           testIPCIDRRange,
				Network:               testNetwork,
				PrivateIPGoogleAccess: true,
				Region:                testRegion,
				SecondaryIPRanges: []*v1alpha1.GCPSubnetworkSecondaryRange{
					{
						RangeName:   "zzaa",
						IPCidrRange: "10.2.0.0/9",
					},
					{
						RangeName:   "aazz",
						IPCidrRange: "10.0.1.1/9",
					},
				},
			},
			in: &compute.Subnetwork{
				Description:           testDescription,
				Name:                  testName,
				EnableFlowLogs:        true,
				IpCidrRange:           testIPCIDRRange,
				Network:               testNetwork,
				PrivateIpGoogleAccess: true,
				Region:                testRegion,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						RangeName:   "aazz",
						IpCidrRange: "10.0.1.1/9",
					},
					{
						RangeName:   "zzaa",
						IpCidrRange: "10.2.0.0/9",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := GenerateGCPSubnetworkStatus(tc.in)
			if diff := cmp.Diff(tc.out, r, cmpopts.SortSlices(equateGCPSubnetworkSecondaryRange)); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}
