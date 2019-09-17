# cache.gcp.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for GCP cache services such as CloudMemorystore.

This API group contains the following Crossplane resources:

* [CloudMemorystoreInstance](#CloudMemorystoreInstance)
* [CloudMemorystoreInstanceClass](#CloudMemorystoreInstanceClass)

## CloudMemorystoreInstance

A CloudMemorystoreInstance is a managed resource that represents a Google Cloud Memorystore instance.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.gcp.crossplane.io/v1alpha2`
`kind` | string | `CloudMemorystoreInstance`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [CloudMemorystoreInstanceSpec](#CloudMemorystoreInstanceSpec) | A CloudMemorystoreInstanceSpec defines the desired state of a CloudMemorystoreInstance.
`status` | [CloudMemorystoreInstanceStatus](#CloudMemorystoreInstanceStatus) | A CloudMemorystoreInstanceStatus represents the observed state of a CloudMemorystoreInstance.



## CloudMemorystoreInstanceClass

A CloudMemorystoreInstanceClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.gcp.crossplane.io/v1alpha2`
`kind` | string | `CloudMemorystoreInstanceClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [CloudMemorystoreInstanceClassSpecTemplate](#CloudMemorystoreInstanceClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned CloudMemorystoreInstance.



## CloudMemorystoreInstanceClassSpecTemplate

A CloudMemorystoreInstanceClassSpecTemplate is a template for the spec of a dynamically provisioned CloudMemorystoreInstance.

Appears in:

* [CloudMemorystoreInstanceClass](#CloudMemorystoreInstanceClass)




CloudMemorystoreInstanceClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [CloudMemorystoreInstanceParameters](#CloudMemorystoreInstanceParameters)


## CloudMemorystoreInstanceParameters

CloudMemorystoreInstanceParameters define the desired state of an Google Cloud Memorystore instance. Most fields map directly to an Instance: https://cloud.google.com/memorystore/docs/redis/reference/rest/v1/projects.locations.instances#Instance

Appears in:

* [CloudMemorystoreInstanceClassSpecTemplate](#CloudMemorystoreInstanceClassSpecTemplate)
* [CloudMemorystoreInstanceSpec](#CloudMemorystoreInstanceSpec)


Name | Type | Description
-----|------|------------
`region` | string | Region in which to create this Cloud Memorystore cluster.
`tier` | string | Tier specifies the replication level of the Redis cluster. BASIC provides a single Redis instance with no high availability. STANDARD_HA provides a cluster of two Redis instances in distinct availability zones. https://cloud.google.com/memorystore/docs/redis/redis-tiers
`locationId` | Optional string | LocationID specifies the zone where the instance will be provisioned. If not provided, the service will choose a zone for the instance. For STANDARD_HA tier, instances will be created across two zones for protection against zonal failures.
`alternativeLocationId` | Optional string | AlternativeLocationID is only applicable to STANDARD_HA tier, which protects the instance against zonal failures by provisioning it across two zones. If provided, it must be a different zone from the one provided in locationId.
`memorySizeGb` | int | MemorySizeGB specifies the Redis memory size in GiB.
`reservedIpRange` | Optional string | ReservedIPRange specifies the CIDR range of internal addresses that are reserved for this instance. If not provided, the service will choose an unused /29 block, for example, 10.0.0.0/29 or 192.168.0.0/29. Ranges must be unique and non-overlapping with existing subnets in an authorized network.
`authorizedNetwork` | Optional string | AuthorizedNetwork specifies the full name of the Google Compute Engine network to which the instance is connected. If left unspecified, the default network will be used.
`redisVersion` | Optional string | RedisVersion specifies the version of Redis software. If not provided, latest supported version will be used. Updating the version will perform an upgrade/downgrade to the new version. Currently, the supported values are REDIS_3_2 for Redis 3.2, and REDIS_4_0 for Redis 4.0 (the default).
`redisConfigs` | Optional map[string]string | RedisConfigs specifies Redis configuration parameters, according to http://redis.io/topics/config. Currently, the only supported parameters are: * maxmemory-policy * notify-keyspace-events



## CloudMemorystoreInstanceSpec

A CloudMemorystoreInstanceSpec defines the desired state of a CloudMemorystoreInstance.

Appears in:

* [CloudMemorystoreInstance](#CloudMemorystoreInstance)




CloudMemorystoreInstanceSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [CloudMemorystoreInstanceParameters](#CloudMemorystoreInstanceParameters)


## CloudMemorystoreInstanceStatus

A CloudMemorystoreInstanceStatus represents the observed state of a CloudMemorystoreInstance.

Appears in:

* [CloudMemorystoreInstance](#CloudMemorystoreInstance)


Name | Type | Description
-----|------|------------
`state` | string | State of this instance.
`message` | string | Additional information about the current status of this instance, if available.
`providerID` | string | ProviderID is the external ID to identify this resource in the cloud provider, e.g. &#39;projects/fooproj/locations/us-foo1/instances/foo&#39;
`currentLocationId` | string | CurrentLocationID is the current zone where the Redis endpoint is placed. For Basic Tier instances, this will always be the same as the locationId provided by the user at creation time. For Standard Tier instances, this can be either locationId or alternativeLocationId and can change after a failover event.
`endpoint` | string | Endpoint of the Cloud Memorystore instance used in connection strings.
`port` | int | Port at which the Cloud Memorystore instance endpoint is listening.


CloudMemorystoreInstanceStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


This API documentation was generated by `crossdocs`.