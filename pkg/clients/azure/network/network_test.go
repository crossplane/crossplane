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
	"context"
	"testing"

	networkmgmt "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplaneio/crossplane/azure/apis/network/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

var (
	uid                  = types.UID("definitely-a-uuid")
	location             = "cool-location"
	enableDDOSProtection = true
	enableVMProtection   = true
	addressPrefixes      = []string{"10.0.0.0/16"}
	addressPrefix        = "10.0.0.0/16"
	serviceEndpoint      = "Microsoft.Sql"
	tags                 = map[string]string{"one": "test", "two": "test"}

	id           = "a-very-cool-id"
	etag         = "a-very-cool-etag"
	resourceType = "resource-type"
	purpose      = "cool-purpose"
	credentials  = `
		{
			"clientId": "cool-id",
			"clientSecret": "cool-secret",
			"tenantId": "cool-tenant",
			"subscriptionId": "cool-subscription",
			"activeDirectoryEndpointUrl": "cool-aad-url",
			"resourceManagerEndpointUrl": "cool-rm-url",
			"activeDirectoryGraphResourceId": "cool-graph-id"
		}
	`
)

var (
	ctx = context.Background()
)

func TestNewVirtualNetworksClient(t *testing.T) {
	cases := []struct {
		name       string
		r          []byte
		returnsErr bool
	}{
		{
			name: "Successful",
			r:    []byte(credentials),
		},
		{
			name:       "Unsuccessful",
			r:          []byte("invalid"),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewVirtualNetworksClient(ctx, tc.r)

			if tc.returnsErr != (err != nil) {
				t.Errorf("NewVirtualNetworksClient(...) error: want: %t got: %t", tc.returnsErr, err != nil)
			}

			if _, ok := got.(VirtualNetworksClient); !ok && !tc.returnsErr {
				t.Error("NewVirtualNetworksClient(...): got does not satisfy VirtualNetworksClient interface")
			}
		})
	}
}

func TestNewVirtualNetworkParameters(t *testing.T) {
	cases := []struct {
		name string
		r    *v1alpha1.VirtualNetwork
		want networkmgmt.VirtualNetwork
	}{
		{
			name: "SuccessfulFull",
			r: &v1alpha1.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.VirtualNetworkSpec{
					Location: location,
					VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
						AddressSpace: v1alpha1.AddressSpace{
							AddressPrefixes: addressPrefixes,
						},
						EnableDDOSProtection: enableDDOSProtection,
						EnableVMProtection:   enableVMProtection,
					},
				},
			},
			want: networkmgmt.VirtualNetwork{
				Location: azure.ToStringPtr(location),
				Tags:     azure.ToStringPtrMap(nil),
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					EnableDdosProtection: to.BoolPtr(enableDDOSProtection),
					EnableVMProtection:   to.BoolPtr(enableVMProtection),
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
				},
			},
		},
		{
			name: "SuccessfulPartial",
			r: &v1alpha1.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.VirtualNetworkSpec{
					Location: location,
					VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
						AddressSpace: v1alpha1.AddressSpace{
							AddressPrefixes: addressPrefixes,
						},
						EnableDDOSProtection: enableDDOSProtection,
					},
				},
			},
			want: networkmgmt.VirtualNetwork{
				Location: azure.ToStringPtr(location),
				Tags:     azure.ToStringPtrMap(nil),
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					EnableDdosProtection: to.BoolPtr(enableDDOSProtection),
					EnableVMProtection:   to.BoolPtr(false),
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewVirtualNetworkParameters(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewVirtualNetworkParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestVirtualNetworkNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *v1alpha1.VirtualNetwork
		az   networkmgmt.VirtualNetwork
		want bool
	}{
		{
			name: "NeedsUpdateAddressSpace",
			kube: &v1alpha1.VirtualNetwork{
				Spec: v1alpha1.VirtualNetworkSpec{
					VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
						AddressSpace: v1alpha1.AddressSpace{
							AddressPrefixes: []string{"10.3.0.0/16"},
						},
						EnableDDOSProtection: enableDDOSProtection,
						EnableVMProtection:   enableVMProtection,
					},
					Tags: tags,
				},
			},
			az: networkmgmt.VirtualNetwork{
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
					EnableDdosProtection: to.BoolPtr(enableDDOSProtection),
					EnableVMProtection:   to.BoolPtr(enableVMProtection),
				},
				Tags: azure.ToStringPtrMap(tags),
			},
			want: true,
		},
		{
			name: "NeedsUpdateDdosProtection",
			kube: &v1alpha1.VirtualNetwork{
				Spec: v1alpha1.VirtualNetworkSpec{
					VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
						AddressSpace: v1alpha1.AddressSpace{
							AddressPrefixes: addressPrefixes,
						},
						EnableDDOSProtection: !enableDDOSProtection,
						EnableVMProtection:   enableVMProtection,
					},
					Tags: tags,
				},
			},
			az: networkmgmt.VirtualNetwork{
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
					EnableDdosProtection: to.BoolPtr(enableDDOSProtection),
					EnableVMProtection:   to.BoolPtr(enableVMProtection),
				},
				Tags: azure.ToStringPtrMap(tags),
			},
			want: true,
		},
		{
			name: "NeedsUpdateVMProtection",
			kube: &v1alpha1.VirtualNetwork{
				Spec: v1alpha1.VirtualNetworkSpec{
					VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
						AddressSpace: v1alpha1.AddressSpace{
							AddressPrefixes: addressPrefixes,
						},
						EnableDDOSProtection: enableDDOSProtection,
						EnableVMProtection:   !enableVMProtection,
					},
					Tags: tags,
				},
			},
			az: networkmgmt.VirtualNetwork{
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
					EnableDdosProtection: to.BoolPtr(enableDDOSProtection),
					EnableVMProtection:   to.BoolPtr(enableVMProtection),
				},
				Tags: azure.ToStringPtrMap(tags),
			},
			want: true,
		},
		{
			name: "NeedsUpdateTags",
			kube: &v1alpha1.VirtualNetwork{
				Spec: v1alpha1.VirtualNetworkSpec{
					VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
						AddressSpace: v1alpha1.AddressSpace{
							AddressPrefixes: addressPrefixes,
						},
						EnableDDOSProtection: enableDDOSProtection,
						EnableVMProtection:   enableVMProtection,
					},
					Tags: map[string]string{"three": "test"},
				},
			},
			az: networkmgmt.VirtualNetwork{
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
					EnableDdosProtection: to.BoolPtr(enableDDOSProtection),
					EnableVMProtection:   to.BoolPtr(enableVMProtection),
				},
				Tags: azure.ToStringPtrMap(tags),
			},
			want: true,
		},
		{
			name: "NoUpdate",
			kube: &v1alpha1.VirtualNetwork{
				Spec: v1alpha1.VirtualNetworkSpec{
					VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
						AddressSpace: v1alpha1.AddressSpace{
							AddressPrefixes: addressPrefixes,
						},
						EnableDDOSProtection: enableDDOSProtection,
						EnableVMProtection:   enableVMProtection,
					},
					Tags: tags,
				},
			},
			az: networkmgmt.VirtualNetwork{
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
					EnableDdosProtection: to.BoolPtr(enableDDOSProtection),
					EnableVMProtection:   to.BoolPtr(enableVMProtection),
				},
				Tags: azure.ToStringPtrMap(tags),
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := VirtualNetworkNeedsUpdate(tc.kube, tc.az)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("VirtualNetworkNeedsUpdate(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestVirtualNetworkStatusFromAzure(t *testing.T) {
	cases := []struct {
		name string
		r    networkmgmt.VirtualNetwork
		want v1alpha1.VirtualNetworkStatus
	}{
		{
			name: "SuccessfulFull",
			r: networkmgmt.VirtualNetwork{
				Location: azure.ToStringPtr(location),
				Etag:     azure.ToStringPtr(etag),
				ID:       azure.ToStringPtr(id),
				Type:     azure.ToStringPtr(resourceType),
				Tags:     azure.ToStringPtrMap(nil),
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					EnableDdosProtection: azure.ToBoolPtr(enableDDOSProtection),
					EnableVMProtection:   azure.ToBoolPtr(enableVMProtection),
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
					ProvisioningState: azure.ToStringPtr("Succeeded"),
					ResourceGUID:      azure.ToStringPtr(string(uid)),
				},
			},
			want: v1alpha1.VirtualNetworkStatus{
				State:        string(networkmgmt.Succeeded),
				ID:           id,
				Etag:         etag,
				Type:         resourceType,
				ResourceGUID: string(uid),
			},
		},
		{
			name: "SuccessfulPartial",
			r: networkmgmt.VirtualNetwork{
				Location: azure.ToStringPtr(location),
				Type:     azure.ToStringPtr(resourceType),
				Tags:     azure.ToStringPtrMap(nil),
				VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
					EnableDdosProtection: azure.ToBoolPtr(enableDDOSProtection),
					EnableVMProtection:   azure.ToBoolPtr(enableVMProtection),
					AddressSpace: &networkmgmt.AddressSpace{
						AddressPrefixes: &addressPrefixes,
					},
					ProvisioningState: azure.ToStringPtr("Succeeded"),
					ResourceGUID:      azure.ToStringPtr(string(uid)),
				},
			},
			want: v1alpha1.VirtualNetworkStatus{
				State:        string(networkmgmt.Succeeded),
				ResourceGUID: string(uid),
				Type:         resourceType,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := VirtualNetworkStatusFromAzure(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewVirtualNetworkParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestNewSubnetsClient(t *testing.T) {
	cases := []struct {
		name       string
		r          []byte
		returnsErr bool
	}{
		{
			name: "Successful",
			r:    []byte(credentials),
		},
		{
			name:       "Unsuccessful",
			r:          []byte("invalid"),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewSubnetsClient(ctx, tc.r)

			if tc.returnsErr != (err != nil) {
				t.Errorf("NewSubnetsClient(...) error: want: %t got: %t", tc.returnsErr, err != nil)
			}

			if _, ok := got.(SubnetsClient); !ok && !tc.returnsErr {
				t.Error("NewSubnetsClient(...): got does not satisfy SubnetsClient interface")
			}
		})
	}
}

func TestNewSubnetParameters(t *testing.T) {
	cases := []struct {
		name string
		r    *v1alpha1.Subnet
		want networkmgmt.Subnet
	}{
		{
			name: "Successful",
			r: &v1alpha1.Subnet{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.SubnetSpec{
					SubnetPropertiesFormat: v1alpha1.SubnetPropertiesFormat{
						AddressPrefix: addressPrefix,
					},
				},
			},
			want: networkmgmt.Subnet{
				SubnetPropertiesFormat: &networkmgmt.SubnetPropertiesFormat{
					AddressPrefix:    azure.ToStringPtr(addressPrefix),
					ServiceEndpoints: NewServiceEndpoints(nil),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewSubnetParameters(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewSubnetParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestNewServiceEndpoints(t *testing.T) {
	cases := []struct {
		name string
		r    []v1alpha1.ServiceEndpointPropertiesFormat
		want *[]networkmgmt.ServiceEndpointPropertiesFormat
	}{
		{
			name: "SuccessfulNotSet",
			r:    []v1alpha1.ServiceEndpointPropertiesFormat{},
			want: &[]networkmgmt.ServiceEndpointPropertiesFormat{},
		},
		{
			name: "SuccessfulSet",
			r: []v1alpha1.ServiceEndpointPropertiesFormat{
				{Service: serviceEndpoint},
			},
			want: &[]networkmgmt.ServiceEndpointPropertiesFormat{
				{Service: &serviceEndpoint},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewServiceEndpoints(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewServiceEndpoints(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestSubnetNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *v1alpha1.Subnet
		az   networkmgmt.Subnet
		want bool
	}{
		{
			name: "NeedsUpdate",
			kube: &v1alpha1.Subnet{
				Spec: v1alpha1.SubnetSpec{
					SubnetPropertiesFormat: v1alpha1.SubnetPropertiesFormat{
						AddressPrefix: "10.1.0.0/16",
					},
				},
			},
			az: networkmgmt.Subnet{
				SubnetPropertiesFormat: &networkmgmt.SubnetPropertiesFormat{
					AddressPrefix: &addressPrefix,
				},
			},
			want: true,
		},
		{
			name: "NoUpdate",
			kube: &v1alpha1.Subnet{
				Spec: v1alpha1.SubnetSpec{
					SubnetPropertiesFormat: v1alpha1.SubnetPropertiesFormat{
						AddressPrefix: addressPrefix,
					},
				},
			},
			az: networkmgmt.Subnet{
				SubnetPropertiesFormat: &networkmgmt.SubnetPropertiesFormat{
					AddressPrefix: &addressPrefix,
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SubnetNeedsUpdate(tc.kube, tc.az)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("SubnetNeedsUpdate(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestSubnetStatusFromAzure(t *testing.T) {
	cases := []struct {
		name string
		r    networkmgmt.Subnet
		want v1alpha1.SubnetStatus
	}{
		{
			name: "SuccessfulFull",
			r: networkmgmt.Subnet{
				Etag: azure.ToStringPtr(etag),
				ID:   azure.ToStringPtr(id),
				SubnetPropertiesFormat: &networkmgmt.SubnetPropertiesFormat{
					Purpose:           azure.ToStringPtr(purpose),
					ProvisioningState: azure.ToStringPtr("Succeeded"),
				},
			},
			want: v1alpha1.SubnetStatus{
				State:   string(networkmgmt.Succeeded),
				ID:      id,
				Etag:    etag,
				Purpose: purpose,
			},
		},
		{
			name: "SuccessfulPartial",
			r: networkmgmt.Subnet{
				ID: azure.ToStringPtr(id),
				SubnetPropertiesFormat: &networkmgmt.SubnetPropertiesFormat{
					ProvisioningState: azure.ToStringPtr("Succeeded"),
				},
			},
			want: v1alpha1.SubnetStatus{
				State: string(networkmgmt.Succeeded),
				ID:    id,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SubnetStatusFromAzure(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewVirtualNetworkParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}
