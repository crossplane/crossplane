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
	"encoding/json"
	"reflect"

	networkmgmt "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network/networkapi"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane/azure/apis/network/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

// A VirtualNetworksClient handles CRUD operations for Azure Virtual Networks.
type VirtualNetworksClient networkapi.VirtualNetworksClientAPI

// NewVirtualNetworksClient returns a new Azure Virtual Networks client. Credentials must be
// passed as JSON encoded data.
func NewVirtualNetworksClient(ctx context.Context, credentials []byte) (VirtualNetworksClient, error) {
	c := azure.Credentials{}
	if err := json.Unmarshal(credentials, &c); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}

	client := networkmgmt.NewVirtualNetworksClient(c.SubscriptionID)

	cfg := auth.ClientCredentialsConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		TenantID:     c.TenantID,
		AADEndpoint:  c.ActiveDirectoryEndpointURL,
		Resource:     c.ResourceManagerEndpointURL,
	}
	a, err := cfg.Authorizer()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create Azure authorizer from credentials config")
	}
	client.Authorizer = a
	if err := client.AddToUserAgent(azure.UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}

	return client, nil
}

// NewVirtualNetworkParameters returns an Azure VirtualNetwork object from a virtual network spec
func NewVirtualNetworkParameters(v *v1alpha1.VirtualNetwork) networkmgmt.VirtualNetwork {
	return networkmgmt.VirtualNetwork{
		Location: azure.ToStringPtr(v.Spec.Location),
		Tags:     azure.ToStringPtrMap(v.Spec.Tags),
		VirtualNetworkPropertiesFormat: &networkmgmt.VirtualNetworkPropertiesFormat{
			EnableDdosProtection: azure.ToBoolPtr(v.Spec.VirtualNetworkPropertiesFormat.EnableDDOSProtection, azure.FieldRequired),
			EnableVMProtection:   azure.ToBoolPtr(v.Spec.VirtualNetworkPropertiesFormat.EnableVMProtection, azure.FieldRequired),
			AddressSpace: &networkmgmt.AddressSpace{
				AddressPrefixes: &v.Spec.VirtualNetworkPropertiesFormat.AddressSpace.AddressPrefixes,
			},
		},
	}
}

// VirtualNetworkNeedsUpdate determines if a virtual network need to be updated
func VirtualNetworkNeedsUpdate(kube *v1alpha1.VirtualNetwork, az networkmgmt.VirtualNetwork) bool {
	up := NewVirtualNetworkParameters(kube)

	switch {
	case !reflect.DeepEqual(up.VirtualNetworkPropertiesFormat.AddressSpace, az.VirtualNetworkPropertiesFormat.AddressSpace):
		return true
	case !reflect.DeepEqual(up.VirtualNetworkPropertiesFormat.EnableDdosProtection, az.VirtualNetworkPropertiesFormat.EnableDdosProtection):
		return true
	case !reflect.DeepEqual(up.VirtualNetworkPropertiesFormat.EnableVMProtection, az.VirtualNetworkPropertiesFormat.EnableVMProtection):
		return true
	case !reflect.DeepEqual(up.Tags, az.Tags):
		return true
	}

	return false
}

// VirtualNetworkStatusFromAzure converts an Azure virtual network to
// a VirtualNetworkStatus
func VirtualNetworkStatusFromAzure(az networkmgmt.VirtualNetwork) v1alpha1.VirtualNetworkStatus {
	return v1alpha1.VirtualNetworkStatus{
		State:        azure.ToString(az.ProvisioningState),
		ID:           azure.ToString(az.ID),
		Etag:         azure.ToString(az.Etag),
		ResourceGUID: azure.ToString(az.ResourceGUID),
		Type:         azure.ToString(az.Type),
	}
}

// A SubnetsClient handles CRUD operations for Azure Virtual Networks.
type SubnetsClient networkapi.SubnetsClientAPI

// NewSubnetsClient returns a new Azure Virtual Networks client. Credentials must be
// passed as JSON encoded data.
func NewSubnetsClient(ctx context.Context, credentials []byte) (SubnetsClient, error) {
	c := azure.Credentials{}
	if err := json.Unmarshal(credentials, &c); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}

	client := networkmgmt.NewSubnetsClient(c.SubscriptionID)

	cfg := auth.ClientCredentialsConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		TenantID:     c.TenantID,
		AADEndpoint:  c.ActiveDirectoryEndpointURL,
		Resource:     c.ResourceManagerEndpointURL,
	}
	a, err := cfg.Authorizer()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create Azure authorizer from credentials config")
	}
	client.Authorizer = a
	if err := client.AddToUserAgent(azure.UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}

	return client, nil
}

// NewSubnetParameters returns an Azure Subnet object from a subnet spec
func NewSubnetParameters(s *v1alpha1.Subnet) networkmgmt.Subnet {
	return networkmgmt.Subnet{
		SubnetPropertiesFormat: &networkmgmt.SubnetPropertiesFormat{
			AddressPrefix:    azure.ToStringPtr(s.Spec.SubnetPropertiesFormat.AddressPrefix),
			ServiceEndpoints: NewServiceEndpoints(s.Spec.SubnetPropertiesFormat.ServiceEndpoints),
		},
	}
}

// NewServiceEndpoints converts to Azure ServiceEndpointPropertiesFormat
func NewServiceEndpoints(e []v1alpha1.ServiceEndpointPropertiesFormat) *[]networkmgmt.ServiceEndpointPropertiesFormat {
	endpoints := make([]networkmgmt.ServiceEndpointPropertiesFormat, len(e))

	for i, end := range e {
		endpoints[i] = networkmgmt.ServiceEndpointPropertiesFormat{
			Service: azure.ToStringPtr(end.Service),
		}
	}

	return &endpoints
}

// SubnetNeedsUpdate determines if a virtual network need to be updated
func SubnetNeedsUpdate(kube *v1alpha1.Subnet, az networkmgmt.Subnet) bool {
	up := NewSubnetParameters(kube)

	return !reflect.DeepEqual(up.SubnetPropertiesFormat.AddressPrefix, az.SubnetPropertiesFormat.AddressPrefix)
}

// SubnetStatusFromAzure converts an Azure subnet to a SubnetStatus
func SubnetStatusFromAzure(az networkmgmt.Subnet) v1alpha1.SubnetStatus {
	return v1alpha1.SubnetStatus{
		State:   azure.ToString(az.ProvisioningState),
		Etag:    azure.ToString(az.Etag),
		ID:      azure.ToString(az.ID),
		Purpose: azure.ToString(az.Purpose),
	}
}
