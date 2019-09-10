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
	compute "google.golang.org/api/compute/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"

	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
)

// Known Address statuses.
const (
	StatusInUse     = "IN_USE"
	StatusReserved  = "RESERVED"
	StatusReserving = "RESERVING"
)

// FromParameters converts the supplied GlobalAddressParameters into an
// Address suitable for use with the Google Compute API.
func FromParameters(p v1alpha1.GlobalAddressParameters) *compute.Address {
	// Kubernetes API conventions dictate that optional, unspecified fields must
	// be nil. GCP API clients omit any field set to its zero value, using
	// NullFields and ForceSendFields to handle edge cases around unsetting
	// previously set values, or forcing zero values to be set. The Address API
	// does not support updates, so we can safely convert any nil pointer to
	// string or int64 to their zero values.
	return &compute.Address{
		Address:      gcp.StringValue(p.Address),
		AddressType:  gcp.StringValue(p.AddressType),
		IpVersion:    gcp.StringValue(p.IPVersion),
		Name:         p.Name,
		Network:      gcp.StringValue(p.Network),
		PrefixLength: gcp.Int64Value(p.PrefixLength),
		Purpose:      gcp.StringValue(p.Purpose),
		Subnetwork:   gcp.StringValue(p.Subnetwork),
	}
}

// UpdateParameters updates any unset (i.e. nil) optional fields of the supplied
// GlobalAddressParameters that are set (i.e. non-zero) on the supplied Address.
func UpdateParameters(p *v1alpha1.GlobalAddressParameters, observed *compute.Address) {
	p.Address = gcp.LateInitializeString(p.Address, observed.Address)
	p.AddressType = gcp.LateInitializeString(p.AddressType, observed.AddressType)
	p.IPVersion = gcp.LateInitializeString(p.IPVersion, observed.IpVersion)
	p.Network = gcp.LateInitializeString(p.Network, observed.Network)
	p.PrefixLength = gcp.LateInitializeInt64(p.PrefixLength, observed.PrefixLength)
	p.Purpose = gcp.LateInitializeString(p.Purpose, observed.Purpose)
	p.Subnetwork = gcp.LateInitializeString(p.Subnetwork, observed.Subnetwork)
}

// UpdateStatus updates any fields of the supplied GlobalAddressStatus to
// reflect the state of the supplied Address.
func UpdateStatus(s *v1alpha1.GlobalAddressStatus, observed *compute.Address) {
	switch observed.Status {
	case StatusReserving:
		s.SetConditions(runtimev1alpha1.Creating())
	case StatusInUse, StatusReserved:
		s.SetConditions(runtimev1alpha1.Available())
	}

	s.CreationTimestamp = observed.CreationTimestamp
	s.ID = observed.Id
	s.SelfLink = observed.SelfLink
	s.Status = observed.Status
	s.Users = observed.Users
}
