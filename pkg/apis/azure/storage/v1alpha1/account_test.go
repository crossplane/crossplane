/*
Copyright 2018 The Crossplane Authors.

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
	"time"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-test/deep"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_newCustomDomain(t *testing.T) {
	tests := []struct {
		name string
		args *storage.CustomDomain
		want *CustomDomain
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "value",
			args: &storage.CustomDomain{
				Name:             to.StringPtr("foo"),
				UseSubDomainName: to.BoolPtr(true),
			},
			want: &CustomDomain{
				Name:             "foo",
				UseSubDomainName: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newCustomDomain(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newCustomDomain() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageCustomDomain(t *testing.T) {
	tests := []struct {
		name string
		args *CustomDomain
		want *storage.CustomDomain
	}{
		{
			name: "test",
			args: &CustomDomain{
				Name:             "foo",
				UseSubDomainName: true,
			},
			want: &storage.CustomDomain{
				Name:             to.StringPtr("foo"),
				UseSubDomainName: to.BoolPtr(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageCustomDomain(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("CustomDomain.ToStorageCustomDomain() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newEnabledEncryptionServices(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		args *storage.EncryptionServices
		want *EnabledEncryptionServices
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &storage.EncryptionServices{
				File:  &storage.EncryptionService{Enabled: to.BoolPtr(true), LastEnabledTime: &date.Time{Time: now}},
				Table: &storage.EncryptionService{Enabled: to.BoolPtr(true), LastEnabledTime: nil},
				Queue: &storage.EncryptionService{Enabled: to.BoolPtr(false), LastEnabledTime: nil},
				Blob:  nil,
			},
			want: &EnabledEncryptionServices{
				File:  true,
				Table: true,
				Queue: false,
				Blob:  false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newEnabledEncryptionServices(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newEnabledEncryptionServices() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageEncryptedServices(t *testing.T) {
	tests := []struct {
		name string
		args *EnabledEncryptionServices
		want *storage.EncryptionServices
	}{
		{
			name: "test",
			args: &EnabledEncryptionServices{
				Blob:  true,
				File:  false,
				Table: true,
				Queue: false,
			},
			want: &storage.EncryptionServices{
				Blob:  &storage.EncryptionService{Enabled: to.BoolPtr(true)},
				File:  &storage.EncryptionService{Enabled: to.BoolPtr(false)},
				Table: &storage.EncryptionService{Enabled: to.BoolPtr(true)},
				Queue: &storage.EncryptionService{Enabled: to.BoolPtr(false)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageEncryptedServices(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("EnabledEncryptionServices.ToStorageEncryptedServices() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newEncryption(t *testing.T) {
	tests := []struct {
		name string
		args *storage.Encryption
		want *Encryption
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &storage.Encryption{},
			want: &Encryption{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newEncryption(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newEncryption() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageEncryption(t *testing.T) {
	tests := []struct {
		name string
		args *Encryption
		want *storage.Encryption
	}{
		{
			name: "test",
			args: &Encryption{
				Services:  &EnabledEncryptionServices{},
				KeySource: storage.MicrosoftKeyvault,
				KeyVaultProperties: &KeyVaultProperties{
					KeyName:     "bar",
					KeyVersion:  "1.0.0",
					KeyVaultURI: "test-uri",
				},
			},
			want: &storage.Encryption{
				Services: &storage.EncryptionServices{
					Blob:  &storage.EncryptionService{Enabled: to.BoolPtr(false)},
					File:  &storage.EncryptionService{Enabled: to.BoolPtr(false)},
					Table: &storage.EncryptionService{Enabled: to.BoolPtr(false)},
					Queue: &storage.EncryptionService{Enabled: to.BoolPtr(false)},
				},
				KeySource: storage.MicrosoftKeyvault,
				KeyVaultProperties: &storage.KeyVaultProperties{
					KeyName:     to.StringPtr("bar"),
					KeyVersion:  to.StringPtr("1.0.0"),
					KeyVaultURI: to.StringPtr("test-uri"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageEncryption(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("Encryption.ToStorageEncryption() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newEndpoints(t *testing.T) {
	tests := []struct {
		name string
		args *storage.Endpoints
		want *Endpoints
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &storage.Endpoints{
				Blob:  to.StringPtr("test-blob-ep"),
				File:  to.StringPtr("test-file-ep"),
				Table: to.StringPtr("test-table-ep"),
			},
			want: &Endpoints{
				Blob:  "test-blob-ep",
				File:  "test-file-ep",
				Table: "test-table-ep",
				Queue: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newEndpoints(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newEndpoints() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newIdentity(t *testing.T) {
	tests := []struct {
		name string
		args *storage.Identity
		want *Identity
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "value",
			args: &storage.Identity{
				PrincipalID: to.StringPtr("test-principal"),
				TenantID:    to.StringPtr(""),
			},
			want: &Identity{
				PrincipalID: "test-principal",
				TenantID:    "",
				Type:        "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newIdentity(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newIdentity() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageIdentity(t *testing.T) {
	tests := []struct {
		name string
		args *Identity
		want *storage.Identity
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &Identity{
				PrincipalID: "test-principal",
				TenantID:    "test-tenant",
				Type:        "test-type",
			},
			want: &storage.Identity{
				PrincipalID: to.StringPtr("test-principal"),
				TenantID:    to.StringPtr("test-tenant"),
				Type:        to.StringPtr("test-type"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageIdentity(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("Identity.ToStorageIdentity() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newIPRule(t *testing.T) {
	tests := []struct {
		name string
		args storage.IPRule
		want IPRule
	}{
		{name: "empty", args: storage.IPRule{}, want: IPRule{}},
		{
			name: "test",
			args: storage.IPRule{
				IPAddressOrRange: to.StringPtr("test-ip"),
			},
			want: IPRule{
				IPAddressOrRange: "test-ip",
				Action:           "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newIPRule(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newIPRule() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageIPRule(t *testing.T) {
	tests := []struct {
		name string
		args IPRule
		want storage.IPRule
	}{
		{
			name: "test",
			args: IPRule{
				IPAddressOrRange: "test-ip",
				Action:           storage.Allow,
			},
			want: storage.IPRule{
				IPAddressOrRange: to.StringPtr("test-ip"),
				Action:           storage.Allow,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageIPRule(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("IPRule.ToStroageIPRule() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newKeyVaultProperties(t *testing.T) {
	tests := []struct {
		name string
		args *storage.KeyVaultProperties
		want *KeyVaultProperties
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &storage.KeyVaultProperties{
				KeyName:     to.StringPtr("test-name"),
				KeyVersion:  to.StringPtr("test-version"),
				KeyVaultURI: nil,
			},
			want: &KeyVaultProperties{
				KeyName:     "test-name",
				KeyVersion:  "test-version",
				KeyVaultURI: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newKeyVaultProperties(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newKeyVaultProperties() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageKeyVaultProperties(t *testing.T) {
	tests := []struct {
		name string
		args *KeyVaultProperties
		want *storage.KeyVaultProperties
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &KeyVaultProperties{
				KeyName:     "test-name",
				KeyVersion:  "test-version",
				KeyVaultURI: "test-uri",
			},
			want: &storage.KeyVaultProperties{
				KeyName:     to.StringPtr("test-name"),
				KeyVersion:  to.StringPtr("test-version"),
				KeyVaultURI: to.StringPtr("test-uri"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageKeyVaultProperties(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("KeyVaultProperties.ToStorageKeyVaultProperties() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newNetworkRuleSet(t *testing.T) {
	tests := []struct {
		name string
		args *storage.NetworkRuleSet
		want *NetworkRuleSet
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &storage.NetworkRuleSet{
				Bypass: storage.AzureServices,
				IPRules: &[]storage.IPRule{
					{
						IPAddressOrRange: to.StringPtr("test-ip"),
						Action:           storage.Allow,
					},
				},
				VirtualNetworkRules: &[]storage.VirtualNetworkRule{
					{
						Action:                   storage.Allow,
						State:                    storage.StateFailed,
						VirtualNetworkResourceID: to.StringPtr("test-network-resource-id"),
					},
				},
			},
			want: &NetworkRuleSet{
				Bypass: storage.AzureServices,
				IPRules: []IPRule{
					{
						IPAddressOrRange: "test-ip",
						Action:           storage.Allow,
					},
				},
				VirtualNetworkRules: []VirtualNetworkRule{
					{
						VirtualNetworkResourceID: "test-network-resource-id",
						Action:                   storage.Allow,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newNetworkRuleSet(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newNetworkRuleSet() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageNetworkRuleSet(t *testing.T) {
	tests := []struct {
		name string
		args *NetworkRuleSet
		want *storage.NetworkRuleSet
	}{
		{
			name: "test",
			args: &NetworkRuleSet{
				IPRules: []IPRule{
					{
						IPAddressOrRange: "test-ip",
						Action:           storage.Allow,
					},
				},
				VirtualNetworkRules: []VirtualNetworkRule{
					{
						VirtualNetworkResourceID: "test-id",
						Action:                   storage.Allow,
					},
				},
			},
			want: &storage.NetworkRuleSet{
				IPRules: &[]storage.IPRule{
					{
						IPAddressOrRange: to.StringPtr("test-ip"),
						Action:           storage.Allow,
					},
				},
				VirtualNetworkRules: &[]storage.VirtualNetworkRule{
					{
						VirtualNetworkResourceID: to.StringPtr("test-id"),
						Action:                   storage.Allow,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageNetworkRuleSet(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("NetworkRuleSet.ToStorageNetworkRuleSet() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newSkuCapability(t *testing.T) {
	tests := []struct {
		name string
		args storage.SKUCapability
		want skuCapability
	}{
		{name: "empty", args: storage.SKUCapability{}, want: skuCapability{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newSkuCapability(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newSkuCapability() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageSkuCapability(t *testing.T) {
	tests := []struct {
		name string
		args skuCapability
		want storage.SKUCapability
	}{
		{
			name: "empty",
			args: skuCapability{},
			want: storage.SKUCapability{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageSkuCapability(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("skuCapability.ToStorageSkuCapability() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newSku(t *testing.T) {
	tests := []struct {
		name string
		args *storage.Sku
		want *Sku
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "values",
			args: &storage.Sku{
				Capabilities: &[]storage.SKUCapability{
					{
						Name:  to.StringPtr("test-capability-name"),
						Value: to.StringPtr("true"),
					},
				},
			},
			want: &Sku{
				Capabilities: []skuCapability{
					{
						Name:  "test-capability-name",
						Value: "true",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newSku(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newSku() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageSku(t *testing.T) {
	tests := []struct {
		name string
		args *Sku
		want *storage.Sku
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "test",
			args: &Sku{
				Capabilities: []skuCapability{
					{
						Name:  "test-capability",
						Value: "true",
					},
				},
				Kind:         storage.Storage,
				Locations:    []string{},
				Name:         storage.PremiumLRS,
				ResourceType: "test-type",
				Tier:         storage.Premium,
			},
			want: &storage.Sku{
				Capabilities: &[]storage.SKUCapability{
					{
						Name:  to.StringPtr("test-capability"),
						Value: to.StringPtr("true"),
					},
				},
				Kind:         storage.Storage,
				Name:         storage.PremiumLRS,
				ResourceType: to.StringPtr("test-type"),
				Tier:         storage.Premium,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageSku(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("Sku.ToStorageSku() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newVirtualNetworkRule(t *testing.T) {
	tests := []struct {
		name string
		args storage.VirtualNetworkRule
		want VirtualNetworkRule
	}{
		{name: "empty", args: storage.VirtualNetworkRule{}, want: VirtualNetworkRule{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newVirtualNetworkRule(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newVirtualNetworkRule() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageVirtualNetworkRule(t *testing.T) {
	tests := []struct {
		name string
		args VirtualNetworkRule
		want storage.VirtualNetworkRule
	}{
		{
			name: "test",
			args: VirtualNetworkRule{
				VirtualNetworkResourceID: "test-id",
				Action:                   storage.Allow,
			},
			want: storage.VirtualNetworkRule{
				VirtualNetworkResourceID: to.StringPtr("test-id"),
				Action:                   storage.Allow,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageVirtualNetworkRule(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("VirtualNetworkRule.ToStorageVirtualNetworkRule() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newStorageAccountSpecProperties(t *testing.T) {
	tests := []struct {
		name string
		args *storage.AccountProperties
		want *StorageAccountSpecProperties
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "values",
			args: &storage.AccountProperties{},
			want: &StorageAccountSpecProperties{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newStorageAccountSpecProperties(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newStorageAccountSpecProperties() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageAccountCreateProperties(t *testing.T) {
	tests := []struct {
		name string
		args *StorageAccountSpecProperties
		want *storage.AccountPropertiesCreateParameters
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "values",
			args: &StorageAccountSpecProperties{
				AccessTier: storage.Hot,
				CustomDomain: &CustomDomain{
					Name:             "test-domain",
					UseSubDomainName: true,
				},
				EnableHTTPSTrafficOnly: true,
				Encryption:             nil,
				NetworkRuleSet:         nil,
			},
			want: &storage.AccountPropertiesCreateParameters{
				AccessTier: storage.Hot,
				CustomDomain: &storage.CustomDomain{
					Name:             to.StringPtr("test-domain"),
					UseSubDomainName: to.BoolPtr(true),
				},
				EnableHTTPSTrafficOnly: to.BoolPtr(true),
				Encryption:             nil,
				NetworkRuleSet:         nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageAccountCreateProperties(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("StorageAccountSpecProperties.ToStorageAccountCreateProperties() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageAccountUpdateProperties(t *testing.T) {
	tests := []struct {
		name string
		args *StorageAccountSpecProperties
		want *storage.AccountPropertiesUpdateParameters
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "values",
			args: &StorageAccountSpecProperties{
				AccessTier:             storage.Cool,
				EnableHTTPSTrafficOnly: true,
			},
			want: &storage.AccountPropertiesUpdateParameters{
				AccessTier:             storage.Cool,
				EnableHTTPSTrafficOnly: to.BoolPtr(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStorageAccountUpdateProperties(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("StorageAccountSpecProperties.ToStorageAccountUpdateProperties() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_newStorageAccountStatusProperties(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		args *storage.AccountProperties
		want *StorageAccountStatusProperties
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "values",
			args: &storage.AccountProperties{
				CreationTime: &date.Time{Time: now},
			},
			want: &StorageAccountStatusProperties{
				CreationTime: &metav1.Time{Time: now},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newStorageAccountStatusProperties(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("newStorageAccountStatusProperties() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_NewStorageAccountSpec(t *testing.T) {
	tests := []struct {
		name string
		args *storage.Account
		want *StorageAccountSpec
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "values",
			args: &storage.Account{},
			want: &StorageAccountSpec{
				Tags: to.StringMap(nil),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStorageAccountSpec(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("NewStorageAccountSpec() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageAccountCreate(t *testing.T) {
	tests := []struct {
		name string
		args *StorageAccountSpec
		want storage.AccountCreateParameters
	}{
		{
			name: "empty",
			args: nil,
			want: storage.AccountCreateParameters{},
		},
		{
			name: "values",
			args: &StorageAccountSpec{
				Identity:                     &Identity{},
				Kind:                         storage.BlobStorage,
				Location:                     "us-west",
				Sku:                          &Sku{},
				Tags:                         map[string]string{"foo": "bar"},
				StorageAccountSpecProperties: &StorageAccountSpecProperties{},
			},
			want: storage.AccountCreateParameters{
				Identity: &storage.Identity{},
				Kind:     storage.BlobStorage,
				Location: to.StringPtr("us-west"),
				Sku:      &storage.Sku{},
				Tags:     map[string]*string{"foo": to.StringPtr("bar")},
				AccountPropertiesCreateParameters: &storage.AccountPropertiesCreateParameters{
					EnableHTTPSTrafficOnly: to.BoolPtr(false),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToStorageAccountCreate(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("StorageAccountSpec.toStorageAccountCreate() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStorageAccountUpdate(t *testing.T) {
	tests := []struct {
		name string
		args *StorageAccountSpec
		want storage.AccountUpdateParameters
	}{
		{
			name: "empty",
			args: nil,
			want: storage.AccountUpdateParameters{},
		},
		{
			name: "values",
			args: &StorageAccountSpec{
				Identity:                     &Identity{},
				Kind:                         storage.BlobStorage,
				Location:                     "us-west",
				Sku:                          &Sku{},
				Tags:                         map[string]string{"foo": "bar"},
				StorageAccountSpecProperties: &StorageAccountSpecProperties{},
			},
			want: storage.AccountUpdateParameters{
				Identity: &storage.Identity{},
				Sku:      &storage.Sku{},
				Tags:     map[string]*string{"foo": to.StringPtr("bar")},
				AccountPropertiesUpdateParameters: &storage.AccountPropertiesUpdateParameters{
					EnableHTTPSTrafficOnly: to.BoolPtr(false),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToStorageAccountUpdate(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("StorageAccountSpec.toStorageAccountUpdate() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_NewStorageAccountStatus(t *testing.T) {
	var tests = []struct {
		name string
		args *storage.Account
		want *StorageAccountStatus
	}{
		{name: "empty", args: nil, want: nil},
		{
			name: "values",
			args: &storage.Account{},
			want: &StorageAccountStatus{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStorageAccountStatus(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("NewStorageAccountStatus() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

const storageAccountSpecString = `{` +
	`"identity":{"principalId":"test-identity-principal-id",` +
	`"tenantId":"test-identity-tenant-id","type":"test-identity-type"},` +
	`"kind":"BlobStorage",` +
	`"location":"West US",` +
	`"sku":{"capabilities":[{"name":"test-sku-name","value":"true"}],` +
	`"kind":"BlobStorage","locations":["West US"],"name":"Standard_GRS",` +
	`"resourceType":"storageAccounts","tier":"Standard"},` +
	`"properties":{"accessTier":"Hot",` +
	`"customDomain":{"name":"test-custom-domain","useSubDomainName":true},` +
	`"supportsHttpsTrafficOnly":true,"encryption":{"services":{"blob":true},` +
	`"keySource":"Microsoft.Keyvault"}},"tags":{"application":"crossplane"}}`

var storageAccountSpec = &StorageAccountSpec{
	Identity: &Identity{
		PrincipalID: "test-identity-principal-id",
		TenantID:    "test-identity-tenant-id",
		Type:        "test-identity-type",
	},
	Kind:     storage.BlobStorage,
	Location: "West US",
	Sku: &Sku{
		Capabilities: []skuCapability{
			{
				Name:  "test-sku-name",
				Value: "true",
			},
		},
		Kind: storage.BlobStorage,
		Locations: []string{
			"West US",
		},
		Name:         storage.StandardGRS,
		ResourceType: "storageAccounts",
		Tier:         storage.Standard,
	},
	StorageAccountSpecProperties: &StorageAccountSpecProperties{
		AccessTier: storage.Hot,
		CustomDomain: &CustomDomain{
			Name:             "test-custom-domain",
			UseSubDomainName: true,
		},
		EnableHTTPSTrafficOnly: true,
		Encryption: &Encryption{
			Services: &EnabledEncryptionServices{
				Blob: true,
			},
			KeySource:          storage.MicrosoftKeyvault,
			KeyVaultProperties: nil,
		},
		NetworkRuleSet: nil,
	},
	Tags: map[string]string{
		"application": "crossplane",
	},
}

func Test_parseStorageAccountSpec(t *testing.T) {
	var tests = []struct {
		name string
		args string
		want *StorageAccountSpec
	}{
		{
			name: "parse",
			args: storageAccountSpecString,
			want: storageAccountSpec,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStorageAccountSpec(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("parseStorageAccountSpec() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_toStringPtr(t *testing.T) {
	tests := []struct {
		name string
		args string
		want *string
	}{
		{name: "empty", args: "", want: nil},
		{name: "not-empty", args: "test", want: to.StringPtr("test")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toStringPtr(tt.args); got != tt.want {
				t.Errorf("toStringPtr() = %v, want %v", got, tt.want)
			}
		})
	}
}
