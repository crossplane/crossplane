# storage.azure.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for Azure storage services such as containers and accounts.

This API group contains the following Crossplane resources:

* [Account](#Account)
* [AccountClass](#AccountClass)
* [Container](#Container)
* [ContainerClass](#ContainerClass)

## Account

An Account is a managed resource that represents an Azure Blob Service Account.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.azure.crossplane.io/v1alpha2`
`kind` | string | `Account`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [AccountSpec](#AccountSpec) | An AccountSpec defines the desired state of an Account.
`status` | [AccountStatus](#AccountStatus) | An AccountStatus represents the observed state of an Account.



## AccountClass

An AccountClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.azure.crossplane.io/v1alpha2`
`kind` | string | `AccountClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [AccountClassSpecTemplate](#AccountClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned Account.



## Container

A Container is a managed resource that represents an Azure Blob Storage Container.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.azure.crossplane.io/v1alpha2`
`kind` | string | `Container`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [ContainerSpec](#ContainerSpec) | A ContainerSpec defines the desired state of a Container.
`status` | [ContainerStatus](#ContainerStatus) | A ContainerStatus represents the observed status of a Container.



## ContainerClass

A ContainerClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.azure.crossplane.io/v1alpha2`
`kind` | string | `ContainerClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [ContainerClassSpecTemplate](#ContainerClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned Container.



## AccountClassSpecTemplate

An AccountClassSpecTemplate is a template for the spec of a dynamically provisioned Account.

Appears in:

* [AccountClass](#AccountClass)




AccountClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [AccountParameters](#AccountParameters)


## AccountParameters

AccountParameters define the desired state of an Azure Blob Storage Account.

Appears in:

* [AccountClassSpecTemplate](#AccountClassSpecTemplate)
* [AccountSpec](#AccountSpec)


Name | Type | Description
-----|------|------------
`resourceGroupName` | string | ResourceGroupName specifies the resource group for this Account.
`storageAccountName` | string | StorageAccountName specifies the name for this Account.
`storageAccountSpec` | [StorageAccountSpec](#StorageAccountSpec) | StorageAccountSpec specifies the desired state of this Account.



## AccountSpec

An AccountSpec defines the desired state of an Account.

Appears in:

* [Account](#Account)




AccountSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [AccountParameters](#AccountParameters)


## AccountStatus

An AccountStatus represents the observed state of an Account.

Appears in:

* [Account](#Account)




AccountStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [StorageAccountStatus](#StorageAccountStatus)


## ContainerClassSpecTemplate

A ContainerClassSpecTemplate is a template for the spec of a dynamically provisioned Container.

Appears in:

* [ContainerClass](#ContainerClass)




ContainerClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [ContainerParameters](#ContainerParameters)


## ContainerParameters

ContainerParameters define the desired state of an Azure Blob Storage Container.

Appears in:

* [ContainerClassSpecTemplate](#ContainerClassSpecTemplate)
* [ContainerSpec](#ContainerSpec)


Name | Type | Description
-----|------|------------
`nameFormat` | string | NameFormat specifies the name of the external Container. The first instance of the string &#39;%s&#39; will be replaced with the Kubernetes UID of this Container.
`metadata` | Optional [azblob.Metadata](https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob#Metadata) | Metadata for this Container.
`publicAccessType` | Optional [azblob.PublicAccessType](https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob#PublicAccessType) | PublicAccessType for this container; either &#34;blob&#34; or &#34;container&#34;.
`accountReference` | [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core) | AccountReference to the Azure Blob Storage Account this Container will reside within.



## ContainerSpec

A ContainerSpec defines the desired state of a Container.

Appears in:

* [Container](#Container)


Name | Type | Description
-----|------|------------
`writeConnectionSecretToRef` | Optional [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core) | WriteConnectionSecretToReference specifies the name of a Secret, in the same namespace as this managed resource, to which any connection details for this managed resource should be written. Connection details frequently include the endpoint, username, and password required to connect to the managed resource.
`claimRef` | Optional [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | ClaimReference specifies the resource claim to which this managed resource will be bound. ClaimReference is set automatically during dynamic provisioning. Crossplane does not currently support setting this field manually, per https://github.com/crossplaneio/crossplane-runtime/issues/19
`classRef` | Optional [core/v1.ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core) | NonPortableClassReference specifies the non-portable resource class that was used to dynamically provision this managed resource, if any. Crossplane does not currently support setting this field manually, per https://github.com/crossplaneio/crossplane-runtime/issues/20
`reclaimPolicy` | Optional [v1alpha1.ReclaimPolicy](../crossplane-runtime/core-crossplane-io-v1alpha1.md#reclaimpolicy) | ReclaimPolicy specifies what will happen to the external resource this managed resource manages when the managed resource is deleted. &#34;Delete&#34; deletes the external resource, while &#34;Retain&#34; (the default) does not. Note this behaviour is subtly different from other uses of the ReclaimPolicy concept within the Kubernetes ecosystem per https://github.com/crossplaneio/crossplane-runtime/issues/21


ContainerSpec supports all fields of:

* [ContainerParameters](#ContainerParameters)


## ContainerStatus

A ContainerStatus represents the observed status of a Container.

Appears in:

* [Container](#Container)


Name | Type | Description
-----|------|------------
`name` | string | Name of this Container.


ContainerStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## CustomDomain

CustomDomain specifies the custom domain assigned to this storage account.

Appears in:

* [StorageAccountSpecProperties](#StorageAccountSpecProperties)


Name | Type | Description
-----|------|------------
`name` | Optional string | Name - custom domain name assigned to the storage account. Name is the CNAME source.
`useSubDomainName` | Optional bool | UseSubDomainName - Indicates whether indirect CNAME validation is enabled.



## EnabledEncryptionServices

EnabledEncryptionServices a list of services that support encryption.

Appears in:

* [Encryption](#Encryption)


Name | Type | Description
-----|------|------------
`blob` | bool | Blob - The encryption function of the blob storage service.
`file` | bool | File - The encryption function of the file storage service.
`table` | bool | Table - The encryption function of the table storage service.
`queue` | bool | Queue - The encryption function of the queue storage service.



## Encryption

Encryption the encryption settings on the storage account.

Appears in:

* [StorageAccountSpecProperties](#StorageAccountSpecProperties)


Name | Type | Description
-----|------|------------
`services` | [EnabledEncryptionServices](#EnabledEncryptionServices) | Services - List of services which support encryption.
`keySource` | [storage.KeySource](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#KeySource) | KeySource - The encryption keySource (provider).  Possible values (case-insensitive):  Microsoft.Storage, Microsoft.Keyvault
`keyvaultproperties` | [KeyVaultProperties](#KeyVaultProperties) | KeyVaultProperties - Properties provided by key vault.



## Endpoints

Endpoints the URIs that are used to perform a retrieval of a public blob, queue, or table object.

Appears in:

* [StorageAccountStatusProperties](#StorageAccountStatusProperties)


Name | Type | Description
-----|------|------------
`blob` | string | Blob - the blob endpoint.
`queue` | string | Queue - the queue endpoint.
`table` | string | Table - the table endpoint.
`file` | string | File - the file endpoint.



## IPRule

IPRule IP rule with specific IP or IP range in CIDR format.

Appears in:

* [NetworkRuleSet](#NetworkRuleSet)


Name | Type | Description
-----|------|------------
`value` | string | IPAddressOrRange - Specifies the IP or IP range in CIDR format. Only IPV4 address is allowed.
`action` | [storage.Action](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#Action) | Action - The action of IP ACL rule. Possible values include: &#39;Allow&#39;



## Identity

Identity identity for the resource.

Appears in:

* [StorageAccountSpec](#StorageAccountSpec)


Name | Type | Description
-----|------|------------
`principalId` | string | PrincipalID - The principal ID of resource identity.
`tenantId` | string | TenantID - The tenant ID of resource.
`type` | string | Type - The identity type.



## KeyVaultProperties

KeyVaultProperties properties of key vault.

Appears in:

* [Encryption](#Encryption)


Name | Type | Description
-----|------|------------
`keyname` | string | KeyName - The name of KeyVault key.
`keyversion` | string | KeyVersion - The version of KeyVault key.
`keyvaulturi` | string | KeyVaultURI - The Uri of KeyVault.



## NetworkRuleSet

NetworkRuleSet network rule set

Appears in:

* [StorageAccountSpecProperties](#StorageAccountSpecProperties)


Name | Type | Description
-----|------|------------
`bypass` | [storage.Bypass](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#Bypass) | Bypass - Specifies whether traffic is bypassed for Logging/Metrics/AzureServices. Possible values are any combination of Logging|Metrics|AzureServices (For example, &#34;Logging, Metrics&#34;), or None to bypass none of those traffics. Possible values include: &#39;None&#39;, &#39;Logging&#39;, &#39;Metrics&#39;, &#39;AzureServices&#39;
`virtualNetworkRules` | [[]VirtualNetworkRule](#VirtualNetworkRule) | VirtualNetworkRules - Sets the virtual network rules
`ipRules` | [[]IPRule](#IPRule) | IPRules - Sets the IP ACL rules
`defaultAction` | [storage.DefaultAction](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#DefaultAction) | DefaultAction - Specifies the default action of allow or deny when no other rules match.  Possible values include: &#39;Allow&#39;, &#39;Deny&#39;



## Sku

Sku of an Azure Blob Storage Account.

Appears in:

* [StorageAccountSpec](#StorageAccountSpec)


Name | Type | Description
-----|------|------------
`capabilities` | [[]skuCapability](#skuCapability) | Capabilities - The capability information in the specified sku, including file encryption, network acls, change notification, etc.
`kind` | [storage.Kind](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#Kind) | Kind - Indicates the type of storage account.  Possible values include: &#39;Storage&#39;, &#39;BlobStorage&#39;
`locations` | []string | Locations - The set of locations that the Sku is available. This will be supported and registered Azure Geo Regions (e.g. West US, East US, Southeast Asia, etc.).
`name` | [storage.SkuName](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#SkuName) | Name - Gets or sets the sku name. Required for account creation; optional for update. Note that in older versions, sku name was called accountType.  Possible values include: &#39;Standard_LRS&#39;, &#39;Standard_GRS&#39;, &#39;Standard_RAGRS&#39;, &#39;Standard_ZRS&#39;, &#39;Premium_LRS&#39;
`resourceType` | string | ResourceType - The type of the resource, usually it is &#39;storageAccounts&#39;.
`tier` | [storage.SkuTier](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#SkuTier) | Tier - Gets the sku tier. This is based on the Sku name.  Possible values include: &#39;Standard&#39;, &#39;Premium&#39;



## StorageAccountSpec

A StorageAccountSpec defines the desired state of an Azure Blob Storage account.

Appears in:

* [AccountParameters](#AccountParameters)


Name | Type | Description
-----|------|------------
`identity` | Optional [Identity](#Identity) | Identity - The identity of the resource.
`kind` | [storage.Kind](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#Kind) | Kind - Indicates the type of storage account. Possible values include: &#39;Storage&#39;, &#39;BlobStorage&#39;
`location` | string | Location - The location of the resource. This will be one of the supported and registered Azure Geo Regions (e.g. West US, East US, Southeast Asia, etc.).
`sku` | [Sku](#Sku) | Sku of the storage account.
`properties` | Optional [StorageAccountSpecProperties](#StorageAccountSpecProperties) | StorageAccountSpecProperties - The parameters used to create the storage account.
`tags` | Optional map[string]string | Tags - A list of key value pairs that describe the resource. These tags can be used for viewing and grouping this resource (across resource groups). A maximum of 15 tags can be provided for a resource. Each tag must have a key with a length no greater than 128 characters and a value with a length no greater than 256 characters.



## StorageAccountSpecProperties

StorageAccountSpecProperties the parameters used to create the storage account.

Appears in:

* [StorageAccountSpec](#StorageAccountSpec)


Name | Type | Description
-----|------|------------
`accessTier` | [storage.AccessTier](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#AccessTier) | AccessTier - Required for storage accounts where kind = BlobStorage. The access tier used for billing. Possible values include: &#39;Hot&#39;, &#39;Cool&#39;
`customDomain` | [CustomDomain](#CustomDomain) | CustomDomain - User domain assigned to the storage account. Name is the CNAME source. Only one custom domain is supported per storage account at this time. to clear the existing custom domain, use an empty string for the custom domain name property.
`supportsHttpsTrafficOnly` | bool | EnableHTTPSTrafficOnly - Allows https traffic only to storage service if sets to true.
`encryption` | [Encryption](#Encryption) | Encryption - Provides the encryption settings on the account. If left unspecified the account encryption settings will remain the same. The default setting is unencrypted.
`networkAcls` | [NetworkRuleSet](#NetworkRuleSet) | NetworkRuleSet - Network rule set



## StorageAccountStatus

A StorageAccountStatus represents the observed status of an Account.

Appears in:

* [AccountStatus](#AccountStatus)


Name | Type | Description
-----|------|------------
`id` | string | ID of this Account.
`name` | string | Name of this Account.
`type` | string | Type of this Account.
`properties` | [StorageAccountStatusProperties](#StorageAccountStatusProperties) | Properties of this Account.



## StorageAccountStatusProperties

StorageAccountStatusProperties represent the observed state of an Account.

Appears in:

* [StorageAccountStatus](#StorageAccountStatus)


Name | Type | Description
-----|------|------------
`creationTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | CreationTime - the creation date and time of the storage account in UTC.
`lastGeoFailoverTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | LastGeoFailoverTime - the timestamp of the most recent instance of a failover to the secondary location. Only the most recent timestamp is retained. This element is not returned if there has never been a failover instance. Only available if the accountType is Standard_GRS or Standard_RAGRS.
`primaryEndpoints` | [Endpoints](#Endpoints) | PrimaryEndpoints - the URLs that are used to perform a retrieval of a public blob, queue, or table object. Note that Standard_ZRS and Premium_LRS accounts only return the blob endpoint.
`primaryLocation` | string | PrimaryLocation - the location of the primary data center for the storage account.
`provisioningState` | [storage.ProvisioningState](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#ProvisioningState) | ProvisioningState - the status of the storage account at the time the operation was called. Possible values include: &#39;Creating&#39;, &#39;ResolvingDNS&#39;, &#39;Succeeded&#39;
`secondaryEndpoints` | [Endpoints](#Endpoints) | SecondaryEndpoints - the URLs that are used to perform a retrieval of a public blob, queue, or table object from the secondary location of the storage account. Only available if the Sku name is Standard_RAGRS.
`secondaryLocation` | string | SecondaryLocation - the location of the geo-replicated secondary for the storage account. Only available if the accountType is Standard_GRS or Standard_RAGRS.
`statusOfPrimary` | [storage.AccountStatus](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#AccountStatus) | StatusOfPrimary - the status indicating whether the primary location of the storage account is available or unavailable. Possible values include: &#39;Available&#39;, &#39;Unavailable&#39;
`statusOfSecondary` | [storage.AccountStatus](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#AccountStatus) | StatusOfSecondary - the status indicating whether the secondary location of the storage account is available or unavailable. Only available if the Sku name is Standard_GRS or Standard_RAGRS. Possible values include: &#39;Available&#39;, &#39;Unavailable&#39;



## VirtualNetworkRule

VirtualNetworkRule virtual Network rule.

Appears in:

* [NetworkRuleSet](#NetworkRuleSet)


Name | Type | Description
-----|------|------------
`id` | string | VirtualNetworkResourceID - Resource ID of a subnet, for example: /subscriptions/{subscriptionId}/resourceGroups/{groupName}/providers/Microsoft.Network/virtualNetworks/{vnetName}/subnets/{subnetName}.
`action` | [storage.Action](https://godoc.org/github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage#Action) | Action - The action of virtual network rule. Possible values include: &#39;Allow&#39;



This API documentation was generated by `crossdocs`.