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

package connection

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"
	servicenetworking "google.golang.org/api/servicenetworking/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"

	"github.com/crossplaneio/crossplane/gcp/apis/servicenetworking/v1alpha1"
)

func TestFromParameters(t *testing.T) {
	network := "coolnetwork"
	ranges := []string{"coolRange", "coolerRange"}

	cases := map[string]struct {
		p    v1alpha1.ConnectionParameters
		want *servicenetworking.Connection
	}{
		"Simple": {
			p: v1alpha1.ConnectionParameters{
				Network:               network,
				ReservedPeeringRanges: ranges,
			},
			want: &servicenetworking.Connection{
				Network:               network,
				ReservedPeeringRanges: ranges,
				ForceSendFields:       []string{"ReservedPeeringRanges"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := FromParameters(tc.p)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("FromParameters(...): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestUpToDate(t *testing.T) {
	cases := map[string]struct {
		p        v1alpha1.ConnectionParameters
		observed *servicenetworking.Connection
		want     bool
	}{
		"UpToDate": {
			p:        v1alpha1.ConnectionParameters{ReservedPeeringRanges: []string{"a", "b"}},
			observed: &servicenetworking.Connection{ReservedPeeringRanges: []string{"b", "a"}},
			want:     true,
		},
		"NotUpToDate": {
			p:        v1alpha1.ConnectionParameters{ReservedPeeringRanges: []string{"a", "c"}},
			observed: &servicenetworking.Connection{ReservedPeeringRanges: []string{"b", "a"}},
			want:     false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := UpToDate(tc.p, tc.observed)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("UpToDate(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	peering := "coolPeering"
	service := "coolService"

	cases := map[string]struct {
		s    *v1alpha1.ConnectionStatus
		o    Observation
		want *v1alpha1.ConnectionStatus
	}{
		"PeeringActive": {
			s: &v1alpha1.ConnectionStatus{},
			o: Observation{
				Connection: &servicenetworking.Connection{
					Peering: peering,
					Service: service,
				},
				Network: &compute.Network{
					Peerings: []*compute.NetworkPeering{{
						Name:  peering,
						State: PeeringStateActive,
					}},
				},
			},
			want: &v1alpha1.ConnectionStatus{
				ResourceStatus: runtimev1alpha1.ResourceStatus{
					ConditionedStatus: runtimev1alpha1.ConditionedStatus{
						Conditions: []runtimev1alpha1.Condition{runtimev1alpha1.Available()},
					},
				},
				Peering: peering,
				Service: service,
			},
		},
		"PeeringInactive": {
			s: &v1alpha1.ConnectionStatus{},
			o: Observation{
				Connection: &servicenetworking.Connection{
					Peering: peering,
					Service: service,
				},
				Network: &compute.Network{
					Peerings: []*compute.NetworkPeering{{
						Name:  peering,
						State: PeeringStateInactive,
					}},
				},
			},
			want: &v1alpha1.ConnectionStatus{
				ResourceStatus: runtimev1alpha1.ResourceStatus{
					ConditionedStatus: runtimev1alpha1.ConditionedStatus{
						Conditions: []runtimev1alpha1.Condition{runtimev1alpha1.Unavailable()},
					},
				},
				Peering: peering,
				Service: service,
			},
		},
		"PeeringDoesNotExist": {
			s: &v1alpha1.ConnectionStatus{},
			o: Observation{
				Connection: &servicenetworking.Connection{
					Peering: peering,
					Service: service,
				},
				Network: &compute.Network{},
			},
			want: &v1alpha1.ConnectionStatus{
				ResourceStatus: runtimev1alpha1.ResourceStatus{
					ConditionedStatus: runtimev1alpha1.ConditionedStatus{
						Conditions: []runtimev1alpha1.Condition{runtimev1alpha1.Unavailable()},
					},
				},
				Peering: peering,
				Service: service,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			UpdateStatus(tc.s, tc.o)
			if diff := cmp.Diff(tc.want, tc.s, test.EquateConditions()); diff != "" {
				t.Errorf("UpdateStatus(...): -want, +got:\n%s", diff)
			}
		})
	}
}
