# cache.azure.crossplane.io/v1beta1 API Reference

Package v1beta1 contains managed resources for Azure cache services such as Redis.

This API group contains the following Crossplane resources:

* [Redis](#Redis)
* [RedisClass](#RedisClass)

## Redis

A Redis is a managed resource that represents an Azure Redis cluster.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.azure.crossplane.io/v1beta1`
`kind` | string | `Redis`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [RedisSpec](#RedisSpec) | A RedisSpec defines the desired state of a Redis.
`status` | [RedisStatus](#RedisStatus) | A RedisStatus represents the observed state of a Redis.



## RedisClass

A RedisClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.azure.crossplane.io/v1beta1`
`kind` | string | `RedisClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [RedisClassSpecTemplate](#RedisClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned Redis.



## RedisClassSpecTemplate

A RedisClassSpecTemplate is a template for the spec of a dynamically provisioned Redis.

Appears in:

* [RedisClass](#RedisClass)


Name | Type | Description
-----|------|------------
`forProvider` | [RedisParameters](#RedisParameters) | RedisParameters define the desired state of an Azure Redis cluster. https://docs.microsoft.com/en-us/rest/api/redis/redis/create#redisresource


RedisClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)


## RedisObservation

RedisObservation represents the observed state of the Redis object in Azure.

Appears in:

* [RedisStatus](#RedisStatus)


Name | Type | Description
-----|------|------------
`redisVersion` | string | RedisVersion - Redis version.
`provisioningState` | string | ProvisioningState - Redis instance provisioning status. Possible values include: &#39;Creating&#39;, &#39;Deleting&#39;, &#39;Disabled&#39;, &#39;Failed&#39;, &#39;Linking&#39;, &#39;Provisioning&#39;, &#39;RecoveringScaleFailure&#39;, &#39;Scaling&#39;, &#39;Succeeded&#39;, &#39;Unlinking&#39;, &#39;Unprovisioning&#39;, &#39;Updating&#39;
`hostName` | string | HostName - Redis host name.
`port` | int | Port - Redis non-SSL port.
`sslPort` | int | SSLPort - Redis SSL port.
`linkedServers` | []string | LinkedServers - List of the linked servers associated with the cache
`id` | string | ID - Resource ID.
`name` | string | Name - Resource name.



## RedisParameters

RedisParameters define the desired state of an Azure Redis cluster. https://docs.microsoft.com/en-us/rest/api/redis/redis/create#redisresource

Appears in:

* [RedisClassSpecTemplate](#RedisClassSpecTemplate)
* [RedisSpec](#RedisSpec)


Name | Type | Description
-----|------|------------
`resourceGroupName` | string | ResourceGroupName in which to create this resource.
`resourceGroupNameRef` | [ResourceGroupNameReferencerForRedis](#ResourceGroupNameReferencerForRedis) | ResourceGroupNameRef to fetch resource group name.
`sku` | [SKU](#SKU) | Sku - The SKU of the Redis cache to deploy.
`location` | string | Location in which to create this resource.
`subnetId` | Optional string | SubnetID specifies the full resource ID of a subnet in a virtual network to deploy the Redis cache in. Example format: /subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/Microsoft.{Network|ClassicNetwork}/VirtualNetworks/vnet1/subnets/subnet1
`staticIp` | Optional string | StaticIP address. Required when deploying a Redis cache inside an existing Azure Virtual Network.
`redisConfiguration` | Optional map[string]string | RedisConfiguration - All Redis Settings. Few possible keys: rdb-backup-enabled,rdb-storage-connection-string,rdb-backup-frequency maxmemory-delta,maxmemory-policy,notify-keyspace-events,maxmemory-samples, slowlog-log-slower-than,slowlog-max-len,list-max-ziplist-entries, list-max-ziplist-value,hash-max-ziplist-entries,hash-max-ziplist-value, set-max-intset-entries,zset-max-ziplist-entries,zset-max-ziplist-value etc.
`enableNonSslPort` | Optional bool | EnableNonSSLPort specifies whether the non-ssl Redis server port (6379) is enabled.
`tenantSettings` | Optional map[string]string | TenantSettings - A dictionary of tenant settings
`shardCount` | Optional int | ShardCount specifies the number of shards to be created on a Premium Cluster Cache.
`minimumTlsVersion` | Optional string | MinimumTLSVersion - Optional: requires clients to use a specified TLS version (or higher) to connect (e,g, &#39;1.0&#39;, &#39;1.1&#39;, &#39;1.2&#39;). Possible values include: &#39;OneFullStopZero&#39;, &#39;OneFullStopOne&#39;, &#39;OneFullStopTwo&#39;
`zones` | Optional []string | Zones - A list of availability zones denoting where the resource needs to come from.
`tags` | Optional map[string]string | Tags - Resource tags.



## RedisSpec

A RedisSpec defines the desired state of a Redis.

Appears in:

* [Redis](#Redis)


Name | Type | Description
-----|------|------------
`forProvider` | [RedisParameters](#RedisParameters) | RedisParameters define the desired state of an Azure Redis cluster. https://docs.microsoft.com/en-us/rest/api/redis/redis/create#redisresource


RedisSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## RedisStatus

A RedisStatus represents the observed state of a Redis.

Appears in:

* [Redis](#Redis)


Name | Type | Description
-----|------|------------
`atProvider` | [RedisObservation](#RedisObservation) | RedisObservation represents the observed state of the Redis object in Azure.


RedisStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## ResourceGroupNameReferencerForRedis

ResourceGroupNameReferencerForRedis is an attribute referencer that resolves the name of a the ResourceGroup.

Appears in:

* [RedisParameters](#RedisParameters)




ResourceGroupNameReferencerForRedis supports all fields of:

* github.com/crossplane/stack-azure/apis/v1alpha3.ResourceGroupNameReferencer


## SKU

An SKU represents the performance and cost oriented properties of a Redis.

Appears in:

* [RedisParameters](#RedisParameters)


Name | Type | Description
-----|------|------------
`name` | string | Name specifies what type of Redis cache to deploy. Valid values: (Basic, Standard, Premium). Possible values include: &#39;Basic&#39;, &#39;Standard&#39;, &#39;Premium&#39;
`family` | string | Family specifies which family to use. Valid values: (C, P). Possible values include: &#39;C&#39;, &#39;P&#39;
`capacity` | int | Capacity specifies the size of Redis cache to deploy. Valid values: for C family (0, 1, 2, 3, 4, 5, 6), for P family (1, 2, 3, 4).



This API documentation was generated by `crossdocs`.