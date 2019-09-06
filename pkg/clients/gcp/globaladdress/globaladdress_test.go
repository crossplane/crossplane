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

package globaladdress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"

	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
)

func TestFromParameters(t *testing.T) {
	name := "coolName"
	address := "coolAddress"
	addressType := "coolType"
	ipVersion := "coolVersion"
	network := "coolNetwork"
	purpose := "beingCool"
	subnetwork := "coolSubnet"
	var prefixLength int64 = 3001

	cases := map[string]struct {
		p    v1alpha1.GlobalAddressParameters
		want *compute.Address
	}{
		"OptionalFieldsSet": {
			p: v1alpha1.GlobalAddressParameters{
				Address:      &address,
				AddressType:  &addressType,
				IPVersion:    &ipVersion,
				Name:         name,
				Network:      &network,
				PrefixLength: &prefixLength,
				Purpose:      &purpose,
				Subnetwork:   &subnetwork,
			},
			want: &compute.Address{
				Address:      address,
				AddressType:  addressType,
				IpVersion:    ipVersion,
				Name:         name,
				Network:      network,
				PrefixLength: prefixLength,
				Purpose:      purpose,
				Subnetwork:   subnetwork,
			},
		},
		"OptionalFieldsUnset": {
			p:    v1alpha1.GlobalAddressParameters{Name: name},
			want: &compute.Address{Name: name},
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

func TestUpdateParameters(t *testing.T) {
	name := "coolName"
	address := "coolAddress"
	addressType := "coolType"
	ipVersion := "coolVersion"
	network := "coolNetwork"
	purpose := "beingCool"
	subnetwork := "coolSubnet"
	var prefixLength int64 = 3001

	diff := "coolDiff"

	cases := map[string]struct {
		p        *v1alpha1.GlobalAddressParameters
		observed *compute.Address
		want     *v1alpha1.GlobalAddressParameters
	}{
		"OptionalFieldsSet": {
			p: &v1alpha1.GlobalAddressParameters{
				Address:      &address,
				AddressType:  &addressType,
				IPVersion:    &ipVersion,
				Name:         name,
				Network:      &network,
				PrefixLength: &prefixLength,
				Purpose:      &purpose,
				Subnetwork:   &subnetwork,
			},
			observed: &compute.Address{
				Address:      address + diff,
				AddressType:  addressType + diff,
				IpVersion:    ipVersion + diff,
				Name:         name,
				Network:      network + diff,
				PrefixLength: prefixLength + 1,
				Purpose:      purpose + diff,
				Subnetwork:   subnetwork + diff,
			},
			want: &v1alpha1.GlobalAddressParameters{
				Address:      &address,
				AddressType:  &addressType,
				IPVersion:    &ipVersion,
				Name:         name,
				Network:      &network,
				PrefixLength: &prefixLength,
				Purpose:      &purpose,
				Subnetwork:   &subnetwork,
			},
		},
		"OptionalFieldsUnset": {
			p: &v1alpha1.GlobalAddressParameters{Name: name},
			observed: &compute.Address{
				Address:      address,
				AddressType:  addressType,
				IpVersion:    ipVersion,
				Name:         name,
				Network:      network,
				PrefixLength: prefixLength,
				Purpose:      purpose,
				Subnetwork:   subnetwork,
			},
			want: &v1alpha1.GlobalAddressParameters{
				Address:      &address,
				AddressType:  &addressType,
				IPVersion:    &ipVersion,
				Name:         name,
				Network:      &network,
				PrefixLength: &prefixLength,
				Purpose:      &purpose,
				Subnetwork:   &subnetwork,
			},
		},
		"ObservedFieldsUnset": {
			p:        &v1alpha1.GlobalAddressParameters{Name: name},
			observed: &compute.Address{Name: name},
			want:     &v1alpha1.GlobalAddressParameters{Name: name},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			UpdateParameters(tc.p, tc.observed)
			if diff := cmp.Diff(tc.want, tc.p); diff != "" {
				t.Errorf("UpdateParameters(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	timestamp := "coolTime"
	link := "coolLink"
	users := []string{"coolUser", "coolerUser"}
	var id uint64 = 3001

	cases := map[string]struct {
		s        *v1alpha1.GlobalAddressStatus
		observed *compute.Address
		want     *v1alpha1.GlobalAddressStatus
	}{
		"Reserving": {
			s: &v1alpha1.GlobalAddressStatus{},
			observed: &compute.Address{
				Status:            StatusReserving,
				CreationTimestamp: timestamp,
				Id:                id,
				SelfLink:          link,
				Users:             users,
			},
			want: &v1alpha1.GlobalAddressStatus{
				ResourceStatus: runtimev1alpha1.ResourceStatus{
					ConditionedStatus: runtimev1alpha1.ConditionedStatus{
						Conditions: []runtimev1alpha1.Condition{runtimev1alpha1.Creating()},
					},
				},
				Status:            StatusReserving,
				CreationTimestamp: timestamp,
				ID:                id,
				SelfLink:          link,
				Users:             users,
			},
		},
		"Reserved": {
			s: &v1alpha1.GlobalAddressStatus{},
			observed: &compute.Address{
				Status: StatusReserved,
			},
			want: &v1alpha1.GlobalAddressStatus{
				ResourceStatus: runtimev1alpha1.ResourceStatus{
					ConditionedStatus: runtimev1alpha1.ConditionedStatus{
						Conditions: []runtimev1alpha1.Condition{runtimev1alpha1.Available()},
					},
				},
				Status: StatusReserved,
			},
		},
		"InUse": {
			s: &v1alpha1.GlobalAddressStatus{},
			observed: &compute.Address{
				Status: StatusInUse,
			},
			want: &v1alpha1.GlobalAddressStatus{
				ResourceStatus: runtimev1alpha1.ResourceStatus{
					ConditionedStatus: runtimev1alpha1.ConditionedStatus{
						Conditions: []runtimev1alpha1.Condition{runtimev1alpha1.Available()},
					},
				},
				Status: StatusInUse,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			UpdateStatus(tc.s, tc.observed)
			if diff := cmp.Diff(tc.want, tc.s, test.EquateConditions()); diff != "" {
				t.Errorf("UpdateStatus(...): -want, +got:\n%s", diff)
			}
		})
	}
}
