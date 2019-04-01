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
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/Azure/go-autorest/autorest/to"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomDomain the custom domain assigned to this storage account.
type CustomDomain struct {
	// Name - custom domain name assigned to the storage account. Name is the CNAME source.
	Name string `json:"name,omitempty"`
	// UseSubDomainName - Indicates whether indirect CName validation is enabled.
	UseSubDomainName bool `json:"useSubDomainName,omitempty"`
}

// newCustomDomain from the storage equivalent
func newCustomDomain(d *storage.CustomDomain) *CustomDomain {
	if d == nil {
		return nil
	}
	return &CustomDomain{
		Name:             to.String(d.Name),
		UseSubDomainName: to.Bool(d.UseSubDomainName),
	}
}

// toStorageCustomDomain object format
func toStorageCustomDomain(c *CustomDomain) *storage.CustomDomain {
	if c == nil {
		return nil
	}
	return &storage.CustomDomain{
		Name:             toStringPtr(c.Name),
		UseSubDomainName: to.BoolPtr(c.UseSubDomainName),
	}
}

// EnabledEncryptionServices a list of services that support encryption.
type EnabledEncryptionServices struct {
	// Blob - The encryption function of the blob storage service.
	Blob bool `json:"blob,omitempty"`

	// File - The encryption function of the file storage service.
	File bool `json:"file,omitempty"`

	// Table - The encryption function of the table storage service.
	Table bool `json:"table,omitempty"`

	// Queue - The encryption function of the queue storage service.
	Queue bool `json:"queue,omitempty"`
}

// newEnabledEncryptionServices from the storage equivalent
func newEnabledEncryptionServices(s *storage.EncryptionServices) *EnabledEncryptionServices {
	if s == nil {
		return nil
	}

	b := func(s *storage.EncryptionService) bool {
		return s != nil && s.Enabled != nil && *s.Enabled
	}
	return &EnabledEncryptionServices{
		Blob:  b(s.Blob),
		File:  b(s.File),
		Table: b(s.Table),
		Queue: b(s.Queue),
	}
}

// toStorageEncryptedServices format
func toStorageEncryptedServices(s *EnabledEncryptionServices) *storage.EncryptionServices {
	return &storage.EncryptionServices{
		Blob:  &storage.EncryptionService{Enabled: to.BoolPtr(s.Blob)},
		File:  &storage.EncryptionService{Enabled: to.BoolPtr(s.File)},
		Table: &storage.EncryptionService{Enabled: to.BoolPtr(s.Table)},
		Queue: &storage.EncryptionService{Enabled: to.BoolPtr(s.Queue)},
	}
}

// Encryption the encryption settings on the storage account.
type Encryption struct {
	// Services - List of services which support encryption.
	Services *EnabledEncryptionServices `json:"services,omitempty"`

	// KeySource - The encryption keySource (provider).
	//
	// Possible values (case-insensitive):  Microsoft.Storage, Microsoft.Keyvault
	// +kubebuilder:validation:Enum=Microsoft.Storage,Microsoft.Keyvault
	KeySource storage.KeySource `json:"keySource,omitempty"`

	// KeyVaultProperties - Properties provided by key vault.
	KeyVaultProperties *KeyVaultProperties `json:"keyvaultproperties,omitempty"`
}

// newEncryption from the storage equivalent
func newEncryption(s *storage.Encryption) *Encryption {
	if s == nil {
		return nil
	}
	return &Encryption{
		Services:           newEnabledEncryptionServices(s.Services),
		KeySource:          s.KeySource,
		KeyVaultProperties: newKeyVaultProperties(s.KeyVaultProperties),
	}
}

// toStorageEncryption format
func toStorageEncryption(e *Encryption) *storage.Encryption {
	if e == nil {
		return nil
	}
	return &storage.Encryption{
		Services:           toStorageEncryptedServices(e.Services),
		KeySource:          e.KeySource,
		KeyVaultProperties: toStorageKeyVaultProperties(e.KeyVaultProperties),
	}
}

// Endpoints the URIs that are used to perform a retrieval of a public blob, queue, or table object.
type Endpoints struct {
	// Blob - the blob endpoint.
	Blob string `json:"blob,omitempty"`
	// Queue - the queue endpoint.
	Queue string `json:"queue,omitempty"`
	// Table - the table endpoint.
	Table string `json:"table,omitempty"`
	// File - the file endpoint.
	File string `json:"file,omitempty"`
}

// newEndpoint from the storage equivalent
func newEndpoints(ep *storage.Endpoints) *Endpoints {
	if ep == nil {
		return nil
	}
	return &Endpoints{
		Blob:  to.String(ep.Blob),
		Queue: to.String(ep.Queue),
		Table: to.String(ep.Table),
		File:  to.String(ep.File),
	}
}

// Identity identity for the resource.
type Identity struct {
	// PrincipalID - The principal ID of resource identity.
	PrincipalID string `json:"principalId,omitempty"`

	// TenantID - The tenant ID of resource.
	TenantID string `json:"tenantId,omitempty"`

	// Type - The identity type.
	Type string `json:"type,omitempty"`
}

// newIdentity from the storage equivalent
func newIdentity(s *storage.Identity) *Identity {
	if s == nil {
		return nil
	}
	return &Identity{
		PrincipalID: to.String(s.PrincipalID),
		TenantID:    to.String(s.TenantID),
		Type:        to.String(s.Type),
	}
}

// toStorageIdentity convert to storage equivalent
func toStorageIdentity(i *Identity) *storage.Identity {
	if i == nil {
		return nil
	}
	return &storage.Identity{
		PrincipalID: toStringPtr(i.PrincipalID),
		TenantID:    toStringPtr(i.TenantID),
		Type:        toStringPtr(i.Type),
	}
}

// IPRule IP rule with specific IP or IP range in CIDR format.
type IPRule struct {
	// IPAddressOrRange - Specifies the IP or IP range in CIDR format.
	// Only IPV4 address is allowed.
	IPAddressOrRange string `json:"value,omitempty"`

	// Action - The action of IP ACL rule. Possible values include: 'Allow'
	// +kubebuilder:validation:Enum=Allow
	Action storage.Action `json:"action,omitempty"`
}

// newIPRule from the storage equivalent
func newIPRule(r storage.IPRule) IPRule {
	return IPRule{
		IPAddressOrRange: to.String(r.IPAddressOrRange),
		Action:           r.Action,
	}
}

// toStorageIPRule format
func toStorageIPRule(r IPRule) storage.IPRule {
	return storage.IPRule{
		IPAddressOrRange: toStringPtr(r.IPAddressOrRange),
		Action:           r.Action,
	}
}

// KeyVaultProperties properties of key vault.
type KeyVaultProperties struct {
	// KeyName - The name of KeyVault key.
	KeyName string `json:"keyname,omitempty"`

	// KeyVersion - The version of KeyVault key.
	KeyVersion string `json:"keyversion,omitempty"`

	// KeyVaultURI - The Uri of KeyVault.
	KeyVaultURI string `json:"keyvaulturi,omitempty"`
}

// newKeyVaultProperties from the storage equivalent
func newKeyVaultProperties(p *storage.KeyVaultProperties) *KeyVaultProperties {
	if p == nil {
		return nil
	}
	return &KeyVaultProperties{
		KeyName:     to.String(p.KeyName),
		KeyVersion:  to.String(p.KeyVersion),
		KeyVaultURI: to.String(p.KeyVaultURI),
	}
}

// toStorageKeyVaultProperties format
func toStorageKeyVaultProperties(p *KeyVaultProperties) *storage.KeyVaultProperties {
	if p == nil {
		return nil
	}
	return &storage.KeyVaultProperties{
		KeyName:     toStringPtr(p.KeyName),
		KeyVersion:  toStringPtr(p.KeyVersion),
		KeyVaultURI: toStringPtr(p.KeyVaultURI),
	}
}

// NetworkRuleSet network rule set
type NetworkRuleSet struct {
	// Bypass - Specifies whether traffic is bypassed for Logging/Metrics/AzureServices.
	// Possible values are any combination of Logging|Metrics|AzureServices
	// (For example, "Logging, Metrics"), or None to bypass none of those traffics.
	// Possible values include: 'None', 'Logging', 'Metrics', 'AzureServices'
	Bypass storage.Bypass `json:"bypass,omitempty"`

	// VirtualNetworkRules - Sets the virtual network rules
	VirtualNetworkRules []VirtualNetworkRule `json:"virtualNetworkRules,omitempty"`

	// IPRules - Sets the IP ACL rules
	IPRules []IPRule `json:"ipRules,omitempty"`

	// DefaultAction - Specifies the default action of allow or deny when no other rules match.
	//
	// Possible values include: 'Allow', 'Deny'
	// +kubebuilder:validation:Enum=Allow,Deny
	DefaultAction storage.DefaultAction `json:"defaultAction,omitempty"`
}

// newNetworkRuleSet from the storage equivalent
func newNetworkRuleSet(s *storage.NetworkRuleSet) *NetworkRuleSet {
	if s == nil {
		return nil
	}

	var networkRules []VirtualNetworkRule
	if s.VirtualNetworkRules != nil {
		networkRules = make([]VirtualNetworkRule, len(*s.VirtualNetworkRules))
		for i, v := range *s.VirtualNetworkRules {
			networkRules[i] = newVirtualNetworkRule(v)
		}
	}

	var ipRules []IPRule
	if s.IPRules != nil {
		ipRules = make([]IPRule, len(*s.IPRules))
		for i, v := range *s.IPRules {
			ipRules[i] = newIPRule(v)
		}
	}

	return &NetworkRuleSet{
		Bypass:              s.Bypass,
		VirtualNetworkRules: networkRules,
		IPRules:             ipRules,
		DefaultAction:       s.DefaultAction,
	}
}

// toStorageNetworkRuleSet format
func toStorageNetworkRuleSet(n *NetworkRuleSet) *storage.NetworkRuleSet {
	if n == nil {
		return nil
	}

	var networkRules *[]storage.VirtualNetworkRule
	if l := len(n.VirtualNetworkRules); l > 0 {
		nr := make([]storage.VirtualNetworkRule, l)
		for i, v := range n.VirtualNetworkRules {
			nr[i] = toStorageVirtualNetworkRule(v)
		}
		networkRules = &nr
	}

	var ipRules *[]storage.IPRule
	if l := len(n.IPRules); l > 0 {
		ir := make([]storage.IPRule, len(n.IPRules))
		for i, v := range n.IPRules {
			ir[i] = toStorageIPRule(v)
		}
		ipRules = &ir
	}

	return &storage.NetworkRuleSet{
		Bypass:              n.Bypass,
		DefaultAction:       n.DefaultAction,
		IPRules:             ipRules,
		VirtualNetworkRules: networkRules,
	}
}

// skuCapability the capability information in the specified sku, including file
// encryption, network acls, change notification, etc.
type skuCapability struct {
	// Name - The name of capability, The capability information in the specified sku,
	// including file encryption, network acls, change notification, etc.
	Name string `json:"name,omitempty"`

	// Value - A string value to indicate states of given capability.
	// Possibly 'true' or 'false'.
	// +kubebuilder:validation:Enum=true,false
	Value string `json:"value,omitempty"`
}

// newSkuCapability from the storage equivalent
func newSkuCapability(s storage.SKUCapability) skuCapability {
	return skuCapability{
		Name:  to.String(s.Name),
		Value: to.String(s.Value),
	}
}

// toStorageSkuCapability format
func toStorageSkuCapability(s skuCapability) storage.SKUCapability {
	return storage.SKUCapability{
		Name:  toStringPtr(s.Name),
		Value: toStringPtr(s.Value),
	}
}

// Sku the Sku of the storage account.
type Sku struct {
	// Capabilities - The capability information in the specified sku, including
	// file encryption, network acls, change notification, etc.
	Capabilities []skuCapability `json:"capabilities,omitempty"`

	// Kind - Indicates the type of storage account.
	//
	// Possible values include: 'Storage', 'BlobStorage'
	// +kubebuilder:validation:Enum=Storage,BlobStorage
	Kind storage.Kind `json:"kind,omitempty"`

	// Locations - The set of locations that the Sku is available.
	// This will be supported and registered Azure Geo Regions (e.g. West US, East US, Southeast Asia, etc.).
	Locations []string `json:"locations,omitempty"`

	// Name - Gets or sets the sku name. Required for account creation; optional for update.
	// Note that in older versions, sku name was called accountType.
	//
	// Possible values include: 'Standard_LRS', 'Standard_GRS', 'Standard_RAGRS', 'Standard_ZRS', 'Premium_LRS'
	// +kubebuilder:validation:Enum=Standard_LRS,Standard_GRS,Standard_RAGRS,Standard_ZRS,Premium_LRS
	Name storage.SkuName `json:"name"`

	// ResourceType - The type of the resource, usually it is 'storageAccounts'.
	ResourceType string `json:"resourceType,omitempty"`

	// Tier - Gets the sku tier. This is based on the Sku name.
	//
	// Possible values include: 'Standard', 'Premium'
	// +kubebuilder:validation:Enum=Standard,Premium
	Tier storage.SkuTier `json:"tier,omitempty"`
}

// newSku from the storage equivalent
func newSku(s *storage.Sku) *Sku {
	if s == nil {
		return nil
	}

	var capabilities []skuCapability
	if s.Capabilities != nil {
		capabilities = make([]skuCapability, len(*s.Capabilities))
		for i, v := range *s.Capabilities {
			capabilities[i] = newSkuCapability(v)
		}
	}

	return &Sku{
		Capabilities: capabilities,
		Kind:         s.Kind,
		Locations:    to.StringSlice(s.Locations),
		Name:         s.Name,
		ResourceType: to.String(s.ResourceType),
		Tier:         s.Tier,
	}
}

// toStorageSku format
func toStorageSku(s *Sku) *storage.Sku {
	if s == nil {
		return nil
	}

	var capabilities *[]storage.SKUCapability
	if len(s.Capabilities) > 0 {
		cbp := make([]storage.SKUCapability, len(s.Capabilities))
		for i, v := range s.Capabilities {
			cbp[i] = toStorageSkuCapability(v)
		}
		capabilities = &cbp
	}

	var locations *[]string
	if len(s.Locations) > 0 {
		locations = to.StringSlicePtr(s.Locations)
	}

	return &storage.Sku{
		Capabilities: capabilities,
		Kind:         s.Kind,
		Locations:    locations,
		Name:         s.Name,
		ResourceType: toStringPtr(s.ResourceType),
		Tier:         s.Tier,
	}
}

// VirtualNetworkRule virtual Network rule.
type VirtualNetworkRule struct {
	// VirtualNetworkResourceID - Resource ID of a subnet,
	// for example: /subscriptions/{subscriptionId}/resourceGroups/{groupName}/providers/Microsoft.Network/virtualNetworks/{vnetName}/subnets/{subnetName}.
	VirtualNetworkResourceID string `json:"id,omitempty"`

	// Action - The action of virtual network rule. Possible values include: 'Allow'
	// +kubebuilder:validation:Enum=Allow
	Action storage.Action `json:"action,omitempty"`
}

// newVirtualNetworkRule from the storage equivalent
func newVirtualNetworkRule(s storage.VirtualNetworkRule) VirtualNetworkRule {
	return VirtualNetworkRule{
		VirtualNetworkResourceID: to.String(s.VirtualNetworkResourceID),
		Action:                   s.Action,
	}
}

// toStorageVirtualNetworkRule format
func toStorageVirtualNetworkRule(v VirtualNetworkRule) storage.VirtualNetworkRule {
	return storage.VirtualNetworkRule{
		VirtualNetworkResourceID: toStringPtr(v.VirtualNetworkResourceID),
		Action:                   v.Action,
	}
}

// StorageAccountSpecProperties the parameters used to create the storage account.
type StorageAccountSpecProperties struct {
	// AccessTier - Required for storage accounts where kind = BlobStorage.
	// The access tier used for billing.
	// Possible values include: 'Hot', 'Cool'
	// +kubebuilder:validation:Enum=Hot,Cool
	AccessTier storage.AccessTier `json:"accessTier,omitempty"`

	// CustomDomain - User domain assigned to the storage account.
	// Name is the CNAME source. Only one custom domain is supported per storage account at this time.
	// to clear the existing custom domain, use an empty string for the custom domain name property.
	CustomDomain *CustomDomain `json:"customDomain,omitempty"`

	// EnableHTTPSTrafficOnly - Allows https traffic only to storage service if sets to true.
	EnableHTTPSTrafficOnly bool `json:"supportsHttpsTrafficOnly,omitempty"`

	// Encryption - Provides the encryption settings on the account.
	// If left unspecified the account encryption settings will remain the same.
	// The default setting is unencrypted.
	Encryption *Encryption `json:"encryption,omitempty"`

	// NetworkRuleSet - Network rule set
	NetworkRuleSet *NetworkRuleSet `json:"networkAcls,omitempty"`
}

// newStorageAccountSpecProperties from the storage equivalent
func newStorageAccountSpecProperties(p *storage.AccountProperties) *StorageAccountSpecProperties {
	if p == nil {
		return nil
	}
	return &StorageAccountSpecProperties{
		AccessTier:             p.AccessTier,
		CustomDomain:           newCustomDomain(p.CustomDomain),
		EnableHTTPSTrafficOnly: to.Bool(p.EnableHTTPSTrafficOnly),
		Encryption:             newEncryption(p.Encryption),
		NetworkRuleSet:         newNetworkRuleSet(p.NetworkRuleSet),
	}
}

// toStorageAccountCreateProperties from storage spec
func toStorageAccountCreateProperties(s *StorageAccountSpecProperties) *storage.AccountPropertiesCreateParameters {
	if s == nil {
		return nil
	}
	return &storage.AccountPropertiesCreateParameters{
		AccessTier:             s.AccessTier,
		CustomDomain:           toStorageCustomDomain(s.CustomDomain),
		EnableHTTPSTrafficOnly: to.BoolPtr(s.EnableHTTPSTrafficOnly),
		Encryption:             toStorageEncryption(s.Encryption),
		NetworkRuleSet:         toStorageNetworkRuleSet(s.NetworkRuleSet),
	}
}

// toStorageAccountUpdateProperties from storage spec
func toStorageAccountUpdateProperties(s *StorageAccountSpecProperties) *storage.AccountPropertiesUpdateParameters {
	if s == nil {
		return nil
	}
	return &storage.AccountPropertiesUpdateParameters{
		AccessTier:             s.AccessTier,
		CustomDomain:           toStorageCustomDomain(s.CustomDomain),
		EnableHTTPSTrafficOnly: to.BoolPtr(s.EnableHTTPSTrafficOnly),
		Encryption:             toStorageEncryption(s.Encryption),
		NetworkRuleSet:         toStorageNetworkRuleSet(s.NetworkRuleSet),
	}
}

// StorageAccountStatusProperties - account status properties of the storage account.
type StorageAccountStatusProperties struct {

	// CreationTime - the creation date and time of the storage account in UTC.
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// LastGeoFailoverTime - the timestamp of the most recent instance of a
	// failover to the secondary location. Only the most recent timestamp is retained.
	// This element is not returned if there has never been a failover instance.
	// Only available if the accountType is Standard_GRS or Standard_RAGRS.
	LastGeoFailoverTime *metav1.Time `json:"lastGeoFailoverTime,omitempty"`

	// PrimaryEndpoints - the URLs that are used to perform a retrieval of a public blob, queue, or table object.
	// Note that Standard_ZRS and Premium_LRS accounts only return the blob endpoint.
	PrimaryEndpoints *Endpoints `json:"primaryEndpoints,omitempty"`

	// PrimaryLocation - the location of the primary data center for the storage account.
	PrimaryLocation string `json:"primaryLocation,omitempty"`

	// ProvisioningState - the status of the storage account at the time the operation was called.
	// Possible values include: 'Creating', 'ResolvingDNS', 'Succeeded'
	// +kubebuilder:validation:Enum=Creating,ResolvingDNS,Succeeded
	ProvisioningState storage.ProvisioningState `json:"provisioningState,omitempty"`

	// SecondaryEndpoints - the URLs that are used to perform a retrieval of a
	// public blob, queue, or table object from the secondary location of the
	// storage account. Only available if the Sku name is Standard_RAGRS.
	SecondaryEndpoints *Endpoints `json:"secondaryEndpoints,omitempty"`

	// SecondaryLocation - the location of the geo-replicated secondary for the
	// storage account. Only available if the accountType is Standard_GRS or Standard_RAGRS.
	SecondaryLocation string `json:"secondaryLocation,omitempty"`

	// StatusOfPrimary - the status indicating whether the primary location
	// of the storage account is available or unavailable.
	// Possible values include: 'Available', 'Unavailable'
	StatusOfPrimary storage.AccountStatus `json:"statusOfPrimary,omitempty"`

	// StatusOfSecondary - the status indicating whether the secondary location
	// of the storage account is available or unavailable.
	// Only available if the Sku name is Standard_GRS or Standard_RAGRS.
	// Possible values include: 'Available', 'Unavailable'
	// +kubebuilder:validation:Enum=Available,Unavailable
	StatusOfSecondary storage.AccountStatus `json:"statusOfSecondary,omitempty"`
}

// newStorageAccountStatusProperties from the storage equivalent
func newStorageAccountStatusProperties(s *storage.AccountProperties) *StorageAccountStatusProperties {
	if s == nil {
		return nil
	}
	tf := func(dt *date.Time) *metav1.Time {
		if dt == nil {
			return nil
		}
		return &metav1.Time{Time: dt.Time}
	}

	return &StorageAccountStatusProperties{
		CreationTime:        tf(s.CreationTime),
		LastGeoFailoverTime: tf(s.LastGeoFailoverTime),
		PrimaryEndpoints:    newEndpoints(s.PrimaryEndpoints),
		PrimaryLocation:     to.String(s.PrimaryLocation),
		ProvisioningState:   s.ProvisioningState,
		SecondaryEndpoints:  newEndpoints(s.SecondaryEndpoints),
		SecondaryLocation:   to.String(s.SecondaryLocation),
		StatusOfPrimary:     s.StatusOfPrimary,
		StatusOfSecondary:   s.StatusOfSecondary,
	}
}

// StorageAccountSpec the parameters used when creating or updating a storage account.
type StorageAccountSpec struct {
	// Identity - The identity of the resource.
	Identity *Identity `json:"identity,omitempty"`

	// Kind - Required. Indicates the type of storage account.
	// Possible values include: 'Storage', 'BlobStorage'
	// +kubebuilder:validation:Enum=Storage,BlobStorage
	Kind storage.Kind `json:"kind,omitempty"`

	// Location - Required. Gets or sets the location of the resource.
	// This will be one of the supported and registered Azure Geo Regions (e.g. West US, East US, Southeast Asia, etc.).
	// The geo region of a resource cannot be changed once it is created,
	// but if an identical geo region is specified on update, the request will succeed.
	// NOTE: not updatable
	Location string `json:"location,omitempty"`

	// Sku - Required. Gets or sets the sku name.
	Sku *Sku `json:"sku,omitempty"`

	// StorageAccountSpecProperties - The parameters used to create the storage account.
	*StorageAccountSpecProperties `json:"properties,omitempty"`

	// Tags - Gets or sets a list of key value pairs that describe the resource.
	// These tags can be used for viewing and grouping this resource (across resource groups).
	// A maximum of 15 tags can be provided for a resource.
	// Each tag must have a key with a length no greater than 128 characters and
	// a value with a length no greater than 256 characters.
	Tags map[string]string `json:"tags,omitempty"`
}

// NewStorageAccountSpec from the storage Account
func NewStorageAccountSpec(a *storage.Account) *StorageAccountSpec {
	if a == nil {
		return nil
	}
	return &StorageAccountSpec{
		Identity:                     newIdentity(a.Identity),
		Kind:                         a.Kind,
		Location:                     to.String(a.Location),
		Sku:                          newSku(a.Sku),
		StorageAccountSpecProperties: newStorageAccountSpecProperties(a.AccountProperties),
		Tags:                         to.StringMap(a.Tags),
	}
}

// parseStorageAccountSpec from json encoded string
func parseStorageAccountSpec(s string) *StorageAccountSpec {
	sas := &StorageAccountSpec{}
	_ = json.Unmarshal([]byte(s), sas)
	return sas
}

// ToStorageAccountCreate from StorageAccountSpec
func ToStorageAccountCreate(s *StorageAccountSpec) storage.AccountCreateParameters {
	if s == nil {
		return storage.AccountCreateParameters{}
	}

	acp := storage.AccountCreateParameters{
		Kind:     s.Kind,
		Location: toStringPtr(s.Location),
		Sku:      toStorageSku(s.Sku),
		Tags:     *to.StringMapPtr(s.Tags),
	}

	if v := s.StorageAccountSpecProperties; v != nil {
		acp.AccountPropertiesCreateParameters = toStorageAccountCreateProperties(v)
	}
	if v := s.Identity; v != nil {
		acp.Identity = toStorageIdentity(v)
	}

	return acp
}

// ToStorageAccountUpdate from StorageAccountSpec
func ToStorageAccountUpdate(s *StorageAccountSpec) storage.AccountUpdateParameters {
	if s == nil {
		return storage.AccountUpdateParameters{}
	}

	return storage.AccountUpdateParameters{
		AccountPropertiesUpdateParameters: toStorageAccountUpdateProperties(s.StorageAccountSpecProperties),
		Identity:                          toStorageIdentity(s.Identity),
		Sku:                               toStorageSku(s.Sku),
		Tags:                              *to.StringMapPtr(s.Tags),
	}
}

// StorageAccountStatus the storage account.
type StorageAccountStatus struct {
	// ID - Resource Id
	ID string `json:"id,omitempty"`

	// Name - Resource name
	Name string `json:"name,omitempty"`

	// Type - Resource type
	Type string `json:"type,omitempty"`

	*StorageAccountStatusProperties `json:"properties,omitempty"`
}

// NewStorageAccountStatus from the storage Account
func NewStorageAccountStatus(a *storage.Account) *StorageAccountStatus {
	if a == nil {
		return nil
	}
	return &StorageAccountStatus{
		ID:                             to.String(a.ID),
		Name:                           to.String(a.Name),
		Type:                           to.String(a.Type),
		StorageAccountStatusProperties: newStorageAccountStatusProperties(a.AccountProperties),
	}
}

func toStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
