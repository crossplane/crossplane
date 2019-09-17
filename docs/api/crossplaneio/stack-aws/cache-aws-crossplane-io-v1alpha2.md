# cache.aws.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for AWS caching services such as ElastiCache.

This API group contains the following Crossplane resources:

* [ReplicationGroup](#ReplicationGroup)
* [ReplicationGroupClass](#ReplicationGroupClass)

## ReplicationGroup

A ReplicationGroup is a managed resource that represents an AWS ElastiCache Replication Group.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.aws.crossplane.io/v1alpha2`
`kind` | string | `ReplicationGroup`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [ReplicationGroupSpec](#ReplicationGroupSpec) | A ReplicationGroupSpec defines the desired state of a ReplicationGroup.
`status` | [ReplicationGroupStatus](#ReplicationGroupStatus) | A ReplicationGroupStatus defines the observed state of a ReplicationGroup.



## ReplicationGroupClass

A ReplicationGroupClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `cache.aws.crossplane.io/v1alpha2`
`kind` | string | `ReplicationGroupClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [ReplicationGroupClassSpecTemplate](#ReplicationGroupClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned ReplicationGroup.



## MinorVersion

MinorVersion represents a supported minor version of Redis. Alias of string.


## NodeGroupConfigurationSpec

A NodeGroupConfigurationSpec specifies the desired state of a node group.

Appears in:

* [ReplicationGroupParameters](#ReplicationGroupParameters)


Name | Type | Description
-----|------|------------
`primaryAvailabilityZone` | Optional string | PrimaryAvailabilityZone specifies the Availability Zone where the primary node of this node group (shard) is launched.
`replicaAvailabilityZones` | Optional []string | ReplicaAvailabilityZones specifies a list of Availability Zones to be used for the read replicas. The number of Availability Zones in this list must match the value of ReplicaCount or ReplicasPerNodeGroup if not specified.
`replicaCount` | Optional int | ReplicaCount specifies the number of read replica nodes in this node group (shard).
`slots` | Optional string | Slots specifies the keyspace for a particular node group. Keyspaces range from 0 to 16,383. The string is in the format startkey-endkey.  Example: &#34;0-3999&#34;



## PatchVersion

PatchVersion represents a supported patch version of Redis. Alias of string.


## ReplicationGroupClassSpecTemplate

A ReplicationGroupClassSpecTemplate is a template for the spec of a dynamically provisioned ReplicationGroup.

Appears in:

* [ReplicationGroupClass](#ReplicationGroupClass)




ReplicationGroupClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [ReplicationGroupParameters](#ReplicationGroupParameters)


## ReplicationGroupParameters

ReplicationGroupParameters define the desired state of an AWS ElastiCache Replication Group. Most fields map directly to an AWS ReplicationGroup: https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_CreateReplicationGroup.html#API_CreateReplicationGroup_RequestParameters

Appears in:

* [ReplicationGroupClassSpecTemplate](#ReplicationGroupClassSpecTemplate)
* [ReplicationGroupSpec](#ReplicationGroupSpec)


Name | Type | Description
-----|------|------------
`atRestEncryptionEnabled` | Optional bool | AtRestEncryptionEnabled enables encryption at rest when set to true.  You cannot modify the value of AtRestEncryptionEnabled after the replication group is created. To enable encryption at rest on a replication group you must set AtRestEncryptionEnabled to true when you create the replication group.
`authEnabled` | Optional bool | AuthEnabled enables mandatory authentication when connecting to the managed replication group. AuthEnabled requires TransitEncryptionEnabled to be true.  While ReplicationGroupSpec mirrors the fields of the upstream replication group object as closely as possible, we expose a boolean here rather than requiring the operator pass in a string authentication token. Crossplane will generate a token automatically and expose it via a Secret.
`automaticFailoverEnabled` | Optional bool | AutomaticFailoverEnabled specifies whether a read-only replica is automatically promoted to read/write primary if the existing primary fails. If true, Multi-AZ is enabled for this replication group. If false, Multi-AZ is disabled for this replication group.  AutomaticFailoverEnabled must be enabled for Redis (cluster mode enabled) replication groups.
`cacheNodeType` | string | CacheNodeType specifies the compute and memory capacity of the nodes in the node group (shard).
`cacheParameterGroupName` | Optional string | CacheParameterGroupName specifies the name of the parameter group to associate with this replication group. If this argument is omitted, the default cache parameter group for the specified engine is used.
`cacheSecurityGroupNames` | Optional []string | CacheSecurityGroupNames specifies a list of cache security group names to associate with this replication group.
`cacheSubnetGroupName` | Optional string | CacheSubnetGroupName specifies the name of the cache subnet group to be used for the replication group. If you&#39;re going to launch your cluster in an Amazon VPC, you need to create a subnet group before you start creating a cluster.
`engineVersion` | Optional string | EngineVersion specifies the version number of the cache engine to be used for the clusters in this replication group. To view the supported cache engine versions, use the DescribeCacheEngineVersions operation.
`nodeGroupConfiguration` | Optional [[]NodeGroupConfigurationSpec](#NodeGroupConfigurationSpec) | NodeGroupConfiguration specifies a list of node group (shard) configuration options.
`notificationTopicArn` | Optional string | NotificationTopicARN specifies the Amazon Resource Name (ARN) of the Amazon Simple Notification Service (SNS) topic to which notifications are sent. The Amazon SNS topic owner must be the same as the cluster owner.
`numCacheClusters` | Optional int | NumCacheClusters specifies the number of clusters this replication group initially has. This parameter is not used if there is more than one node group (shard). You should use ReplicasPerNodeGroup instead.  If AutomaticFailoverEnabled is true, the value of this parameter must be at least 2. If AutomaticFailoverEnabled is false you can omit this parameter (it will default to 1), or you can explicitly set it to a value between 2 and 6.
`numNodeGroups` | Optional int | NumNodeGroups specifies the number of node groups (shards) for this Redis (cluster mode enabled) replication group. For Redis (cluster mode disabled) either omit this parameter or set it to 1.
`port` | Optional int | Port number on which each member of the replication group accepts connections.
`preferredCacheClusterAzs` | Optional []string | PreferredCacheClusterAZs specifies a list of EC2 Availability Zones in which the replication group&#39;s clusters are created. The order of the Availability Zones in the list is the order in which clusters are allocated. The primary cluster is created in the first AZ in the list.  This parameter is not used if there is more than one node group (shard). You should use NodeGroupConfiguration instead.  The number of Availability Zones listed must equal the value of NumCacheClusters.
`preferredMaintenanceWindow` | Optional string | PreferredMaintenanceWindow specifies the weekly time range during which maintenance on the cluster is performed. It is specified as a range in the format ddd:hh24:mi-ddd:hh24:mi (24H Clock UTC). The minimum maintenance window is a 60 minute period.  Example: sun:23:00-mon:01:30
`replicasPerNodeGroup` | Optional int | ReplicasPerNodeGroup specifies the number of replica nodes in each node group (shard). Valid values are 0 to 5.
`securityGroupIds` | Optional []string | SecurityGroupIDs specifies one or more Amazon VPC security groups associated with this replication group. Use this parameter only when you are creating a replication group in an Amazon VPC.
`snapshotArns` | Optional []string | SnapshotARNs specifies a list of Amazon Resource Names (ARN) that uniquely identify the Redis RDB snapshot files stored in Amazon S3. The snapshot files are used to populate the new replication group. The Amazon S3 object name in the ARN cannot contain any commas. The new replication group will have the number of node groups (console: shards) specified by the parameter NumNodeGroups or the number of node groups configured by NodeGroupConfiguration regardless of the number of ARNs specified here.
`snapshotName` | Optional string | SnapshotName specifies the name of a snapshot from which to restore data into the new replication group. The snapshot status changes to restoring while the new replication group is being created.
`snapshotRetentionLimit` | Optional int | SnapshotRetentionLimit specifies the number of days for which ElastiCache retains automatic snapshots before deleting them. For example, if you set SnapshotRetentionLimit to 5, a snapshot that was taken today is retained for 5 days before being deleted.
`snapshotWindow` | Optional string | SnapshotWindow specifies the daily time range (in UTC) during which ElastiCache begins taking a daily snapshot of your node group (shard).  Example: 05:00-09:00  If you do not specify this parameter, ElastiCache automatically chooses an appropriate time range.
`transitEncryptionEnabled` | Optional bool | TransitEncryptionEnabled enables in-transit encryption when set to true.  You cannot modify the value of TransitEncryptionEnabled after the cluster is created. To enable in-transit encryption on a cluster you must TransitEncryptionEnabled to true when you create a cluster.



## ReplicationGroupSpec

A ReplicationGroupSpec defines the desired state of a ReplicationGroup.

Appears in:

* [ReplicationGroup](#ReplicationGroup)




ReplicationGroupSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [ReplicationGroupParameters](#ReplicationGroupParameters)


## ReplicationGroupStatus

A ReplicationGroupStatus defines the observed state of a ReplicationGroup.

Appears in:

* [ReplicationGroup](#ReplicationGroup)


Name | Type | Description
-----|------|------------
`state` | string | State of the Replication Group.
`providerID` | string | ProviderID is the external ID to identify this resource in the cloud provider
`endpoint` | string | Endpoint of the Replication Group used in connection strings.
`port` | int | Port at which the Replication Group endpoint is listening.
`clusterEnabled` | bool | ClusterEnabled indicates whether cluster mode is enabled, i.e. whether this replication group&#39;s data can be partitioned across multiple shards.
`memberClusters` | []string | MemberClusters that are part of this replication group.
`groupName` | string | Groupname of the Replication Group.


ReplicationGroupStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


This API documentation was generated by `crossdocs`.