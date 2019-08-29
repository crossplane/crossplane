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
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"

	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
)

const (
	testIPv4Range         = "10.0.0.0/256"
	testDescription       = "some desc"
	testName              = "some-name"
	testRoutingMode       = "GLOBAL"
	testCreationTimestamp = "10/10/2023"
	testGatewayIPv4       = "10.0.0.0"

	testPeeringName         = "some-peering-name"
	testPeeringNetwork      = "name"
	testPeeringState        = "ACTIVE"
	testPeeringStateDetails = "more-detailed than ACTIVE"
)

func TestGCPNetworkSpec_GenerateNetwork(t *testing.T) {
	trueVal := true
	falseVal := false
	cases := map[string]struct {
		in   v1alpha1.GCPNetworkSpec
		out  *compute.Network
		fail bool
	}{
		"AutoCreateSubnetworksNil": {
			in: v1alpha1.GCPNetworkSpec{
				IPv4Range:   testIPv4Range,
				Description: testDescription,
				Name:        testName,
				RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			out: &compute.Network{
				IPv4Range:             testIPv4Range,
				Description:           testDescription,
				AutoCreateSubnetworks: false,
				Name:                  testName,
				RoutingConfig: &compute.NetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
		},
		"AutoCreateSubnetworksFalse": {
			in: v1alpha1.GCPNetworkSpec{
				IPv4Range:             testIPv4Range,
				Description:           testDescription,
				AutoCreateSubnetworks: &falseVal,
				Name:                  testName,
				RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			out: &compute.Network{
				IPv4Range:             testIPv4Range,
				Description:           testDescription,
				AutoCreateSubnetworks: false,
				Name:                  testName,
				RoutingConfig: &compute.NetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
				ForceSendFields: []string{"AutoCreateSubnetworks"},
			},
		},
		"AutoCreateSubnetworksTrue": {
			in: v1alpha1.GCPNetworkSpec{
				IPv4Range:             testIPv4Range,
				Description:           testDescription,
				AutoCreateSubnetworks: &trueVal,
				Name:                  testName,
				RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			out: &compute.Network{
				IPv4Range:             testIPv4Range,
				Description:           testDescription,
				AutoCreateSubnetworks: true,
				Name:                  testName,
				RoutingConfig: &compute.NetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
		},
		"AutoCreateSubnetworksTrueFail": {
			in: v1alpha1.GCPNetworkSpec{
				IPv4Range:             testIPv4Range,
				Description:           testDescription,
				AutoCreateSubnetworks: &trueVal,
				Name:                  testName,
				RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			out: &compute.Network{
				IPv4Range:             testIPv4Range,
				Description:           testDescription,
				AutoCreateSubnetworks: false,
				Name:                  testName,
				RoutingConfig: &compute.NetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			fail: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := GenerateNetwork(tc.in)
			if diff := cmp.Diff(r, tc.out); diff != "" && !tc.fail {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGenerateGCPNetworkStatus(t *testing.T) {
	cases := map[string]struct {
		in   compute.Network
		out  v1alpha1.GCPNetworkStatus
		fail bool
	}{
		"AllFilled": {
			in: compute.Network{
				IPv4Range:         testIPv4Range,
				Description:       testDescription,
				CreationTimestamp: testCreationTimestamp,
				GatewayIPv4:       testGatewayIPv4,
				Id:                2029819203,
				Peerings: []*compute.NetworkPeering{
					{
						AutoCreateRoutes:     true,
						ExchangeSubnetRoutes: true,
						Name:                 testPeeringName,
						Network:              testPeeringNetwork,
						State:                testPeeringState,
						StateDetails:         testPeeringStateDetails,
					},
				},
				Subnetworks: []string{
					"my-subnetwork",
				},
				Name: testName,
				RoutingConfig: &compute.NetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			out: v1alpha1.GCPNetworkStatus{
				IPv4Range:         testIPv4Range,
				Description:       testDescription,
				CreationTimestamp: testCreationTimestamp,
				GatewayIPv4:       testGatewayIPv4,
				ID:                2029819203,
				Peerings: []*v1alpha1.GCPNetworkPeering{
					{
						AutoCreateRoutes:     true,
						ExchangeSubnetRoutes: true,
						Name:                 testPeeringName,
						Network:              testPeeringNetwork,
						State:                testPeeringState,
						StateDetails:         testPeeringStateDetails,
					},
				},
				Subnetworks: []string{
					"my-subnetwork",
				},
				RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
		},
		"AllFilledFail": {
			in: compute.Network{
				IPv4Range:         testIPv4Range,
				Description:       testDescription,
				CreationTimestamp: testCreationTimestamp,
				GatewayIPv4:       testGatewayIPv4,
				Id:                2029819203,
				Peerings: []*compute.NetworkPeering{
					{
						AutoCreateRoutes:     true,
						ExchangeSubnetRoutes: true,
						Name:                 testPeeringName,
						Network:              testPeeringNetwork,
						State:                testPeeringState,
						StateDetails:         testPeeringStateDetails,
					},
				},
				Subnetworks: []string{
					"my-subnetwork",
				},
				Name: testName,
				RoutingConfig: &compute.NetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			out: v1alpha1.GCPNetworkStatus{
				IPv4Range:         testIPv4Range,
				Description:       testDescription,
				CreationTimestamp: testCreationTimestamp,
				GatewayIPv4:       testGatewayIPv4,
				ID:                2029819223103,
				Peerings: []*v1alpha1.GCPNetworkPeering{
					{
						AutoCreateRoutes:     true,
						ExchangeSubnetRoutes: true,
						Name:                 testPeeringName,
						Network:              testPeeringNetwork,
						State:                testPeeringState,
						StateDetails:         testPeeringStateDetails,
					},
				},
				Subnetworks: []string{
					"my-subnetwork",
				},
				RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
					RoutingMode: testRoutingMode,
				},
			},
			fail: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := GenerateGCPNetworkStatus(tc.in)
			if diff := cmp.Diff(r, tc.out); diff != "" && !tc.fail {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}
