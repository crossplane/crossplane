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

func TestGCPNetworkSpec_IsSameAs(t *testing.T) {
	trueVal := true
	cases := map[string]struct {
		spec   GCPNetworkSpec
		status GCPNetworkStatus
		result bool
	}{
		"FullMatch": {
			spec: GCPNetworkSpec{
				Description:           "some desc",
				Name:                  "some-name",
				IPv4Range:             "10.0.0.0/9",
				AutoCreateSubnetworks: nil,
				RoutingConfig: &GCPNetworkRoutingConfig{
					RoutingMode: "REGIONAL",
				},
			},
			status: GCPNetworkStatus{
				Description:           "some desc",
				IPv4Range:             "10.0.0.0/9",
				AutoCreateSubnetworks: false,
				RoutingConfig: &GCPNetworkRoutingConfig{
					RoutingMode: "REGIONAL",
				},
			},
			result: true,
		},
		"AutoCreateSubnetworksDifferent": {
			spec: GCPNetworkSpec{
				Description:           "some desc",
				Name:                  "some-name",
				IPv4Range:             "10.0.0.0/9",
				AutoCreateSubnetworks: &trueVal,
				RoutingConfig: &GCPNetworkRoutingConfig{
					RoutingMode: "REGIONAL",
				},
			},
			status: GCPNetworkStatus{
				Description:           "some desc",
				IPv4Range:             "10.0.0.0/9",
				AutoCreateSubnetworks: false,
				RoutingConfig: &GCPNetworkRoutingConfig{
					RoutingMode: "REGIONAL",
				},
			},
			result: false,
		},
		"RoutingConfigDifferent": {
			spec: GCPNetworkSpec{
				Description:           "some desc",
				Name:                  "some-name",
				IPv4Range:             "10.0.0.0/9",
				AutoCreateSubnetworks: &trueVal,
				RoutingConfig: &GCPNetworkRoutingConfig{
					RoutingMode: "REGIONAL",
				},
			},
			status: GCPNetworkStatus{
				Description:           "some desc",
				IPv4Range:             "10.0.0.0/9",
				AutoCreateSubnetworks: true,
				RoutingConfig: &GCPNetworkRoutingConfig{
					RoutingMode: "GLOBAL",
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
