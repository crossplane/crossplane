# cache.gcp.crossplane.io/v1beta1 API Reference

Package v1beta1 contains managed resources for GCP cache services such as CloudMemorystore.

This API group contains the following Crossplane resources:

* [CloudMemorystoreInstance](#CloudMemorystoreInstance)
* [CloudMemorystoreInstanceClass](#CloudMemorystoreInstanceClass)

## CloudMemorystoreInstance

A CloudMemorystoreInstance is a managed resource that represents a Google Cloud Memorystore instance.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.gcp.crossplane.io/v1beta1`
`kind` | string | `CloudMemorystoreInstance`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [CloudMemorystoreInstanceSpec](#CloudMemorystoreInstanceSpec) | A CloudMemorystoreInstanceSpec defines the desired state of a CloudMemorystoreInstance.
`status` | [CloudMemorystoreInstanceStatus](#CloudMemorystoreInstanceStatus) | A CloudMemorystoreInstanceStatus represents the observed state of a CloudMemorystoreInstance.



## CloudMemorystoreInstanceClass

A CloudMemorystoreInstanceClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.gcp.crossplane.io/v1beta1`
`kind` | string | `CloudMemorystoreInstanceClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [CloudMemorystoreInstanceClassSpecTemplate](#CloudMemorystoreInstanceClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned CloudMemorystoreInstance.



## CloudMemorystoreInstanceClassSpecTemplate

A CloudMemorystoreInstanceClassSpecTemplate is a template for the spec of a dynamically provisioned CloudMemorystoreInstance.

Appears in:

* [CloudMemorystoreInstanceClass](#CloudMemorystoreInstanceClass)


Name | Type | Description
-----|------|------------
`forProvider` | [CloudMemorystoreInstanceParameters](#CloudMemorystoreInstanceParameters) | CloudMemorystoreInstanceParameters define the desired state of an Google Cloud Memorystore instance. Most fields map directly to an Instance: https://cloud.google.com/memorystore/docs/redis/reference/rest/v1/projects.locations.instances#Instance


CloudMemorystoreInstanceClassSpecTemplate supports all fields of:

* github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1.ClassSpecTemplate


## CloudMemorystoreInstanceObservation

CloudMemorystoreInstanceObservation is used to show the observed state of the CloudMemorystore resource on GCP.

Appears in:

* [CloudMemorystoreInstanceStatus](#CloudMemorystoreInstanceStatus)


Name | Type | Description
-----|------|------------
`name` | string | Unique name of the resource in this scope including project and location using the form:     `projects/{project_id}/locations/{location_id}/instances/{instance_id}`  Note: Redis instances are managed and addressed at regional level so location_id here refers to a GCP region; however, users may choose which specific zone (or collection of zones for cross-zone instances) an instance should be provisioned in. Refer to [location_id] and [alternative_location_id] fields for more details.
`host` | string | Hostname or IP address of the exposed Redis endpoint used by clients to connect to the service.
`port` | int32 | The port number of the exposed Redis endpoint.
`currentLocationId` | string | The current zone where the Redis endpoint is placed. For Basic Tier instances, this will always be the same as the [location_id] provided by the user at creation time. For Standard Tier instances, this can be either [location_id] or [alternative_location_id] and can change after a failover event.
`createTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | The time the instance was created.
`state` | string | The current state of this instance.
`statusMessage` | string | Additional information about the current status of this instance, if available.
`persistenceIamIdentity` | string | Cloud IAM identity used by import / export operations to transfer data to/from Cloud Storage. Format is &#34;serviceAccount:&lt;service_account_email&gt;&#34;. The value may change over time for a given instance so should be checked before each import/export operation.



## CloudMemorystoreInstanceParameters

CloudMemorystoreInstanceParameters define the desired state of an Google Cloud Memorystore instance. Most fields map directly to an Instance: https://cloud.google.com/memorystore/docs/redis/reference/rest/v1/projects.locations.instances#Instance

Appears in:

* [CloudMemorystoreInstanceClassSpecTemplate](#CloudMemorystoreInstanceClassSpecTemplate)
* [CloudMemorystoreInstanceSpec](#CloudMemorystoreInstanceSpec)


Name | Type | Description
-----|------|------------
`region` | string | Region in which to create this Cloud Memorystore cluster.
`tier` | string | Tier specifies the replication level of the Redis cluster. BASIC provides a single Redis instance with no high availability. STANDARD_HA provides a cluster of two Redis instances in distinct availability zones. https://cloud.google.com/memorystore/docs/redis/redis-tiers
`memorySizeGb` | int32 | Redis memory size in GiB.
`displayName` | Optional string | An arbitrary and optional user-provided name for the instance.
`labels` | Optional map[string]string | Resource labels to represent user provided metadata
`locationId` | Optional string | The zone where the instance will be provisioned. If not provided, the service will choose a zone for the instance. For STANDARD_HA tier, instances will be created across two zones for protection against zonal failures. If [alternative_location_id] is also provided, it must be different from [location_id].
`alternativeLocationId` | Optional string | Only applicable to STANDARD_HA tier which protects the instance against zonal failures by provisioning it across two zones. If provided, it must be a different zone from the one provided in [location_id].
`redisVersion` | Optional string | The version of Redis software. If not provided, latest supported version will be used. Updating the version will perform an upgrade/downgrade to the new version. Currently, the supported values are:   *   `REDIS_4_0` for Redis 4.0 compatibility (default)  *   `REDIS_3_2` for Redis 3.2 compatibility
`reservedIpRange` | Optional string | The CIDR range of internal addresses that are reserved for this instance. If not provided, the service will choose an unused /29 block, for example, 10.0.0.0/29 or 192.168.0.0/29. Ranges must be unique and non-overlapping with existing subnets in an authorized network.
`redisConfigs` | Optional map[string]string | Redis configuration parameters, according to http://redis.io/topics/config. Currently, the only supported parameters are:   Redis 3.2 and above:   *   maxmemory-policy  *   notify-keyspace-events   Redis 4.0 and above:   *   activedefrag  *   lfu-log-factor  *   lfu-decay-time
`authorizedNetwork` | Optional string | The full name of the Google Compute Engine [network](/compute/docs/networks-and-firewalls#networks) to which the instance is connected. If left unspecified, the `default` network will be used.



## CloudMemorystoreInstanceSpec

A CloudMemorystoreInstanceSpec defines the desired state of a CloudMemorystoreInstance.

Appears in:

* [CloudMemorystoreInstance](#CloudMemorystoreInstance)


Name | Type | Description
-----|------|------------
`forProvider` | [CloudMemorystoreInstanceParameters](#CloudMemorystoreInstanceParameters) | CloudMemorystoreInstanceParameters define the desired state of an Google Cloud Memorystore instance. Most fields map directly to an Instance: https://cloud.google.com/memorystore/docs/redis/reference/rest/v1/projects.locations.instances#Instance


CloudMemorystoreInstanceSpec supports all fields of:

* github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1.ResourceSpec


## CloudMemorystoreInstanceStatus

A CloudMemorystoreInstanceStatus represents the observed state of a CloudMemorystoreInstance.

Appears in:

* [CloudMemorystoreInstance](#CloudMemorystoreInstance)


Name | Type | Description
-----|------|------------
`atProvider` | [CloudMemorystoreInstanceObservation](#CloudMemorystoreInstanceObservation) | CloudMemorystoreInstanceObservation is used to show the observed state of the CloudMemorystore resource on GCP.


CloudMemorystoreInstanceStatus supports all fields of:

* github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1.ResourceStatus


This API documentation was generated by `crossdocs`.