# database.gcp.crossplane.io/v1beta1 API Reference

Package v1beta1 contains managed resources for GCP database services such as CloudSQL.

This API group contains the following Crossplane resources:

* [CloudSQLInstance](#CloudSQLInstance)
* [CloudSQLInstanceClass](#CloudSQLInstanceClass)

## CloudSQLInstance

A CloudSQLInstance is a managed resource that represents a Google CloudSQL instance.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.gcp.crossplane.io/v1beta1`
`kind` | string | `CloudSQLInstance`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [CloudSQLInstanceSpec](#CloudSQLInstanceSpec) | A CloudSQLInstanceSpec defines the desired state of a CloudSQLInstance.
`status` | [CloudSQLInstanceStatus](#CloudSQLInstanceStatus) | A CloudSQLInstanceStatus represents the observed state of a CloudSQLInstance.



## CloudSQLInstanceClass

A CloudSQLInstanceClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.gcp.crossplane.io/v1beta1`
`kind` | string | `CloudSQLInstanceClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [CloudSQLInstanceClassSpecTemplate](#CloudSQLInstanceClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned CloudSQLInstance.



## ACLEntry

ACLEntry is an entry for an Access Control list.


Name | Type | Description
-----|------|------------
`expirationTime` | Optional string | ExpirationTime: The time when this access control entry expires in RFC 3339 format, for example 2012-11-15T16:19:00.094Z.
`name` | Optional string | Name: An optional label to identify this entry.
`value` | Optional string | Value: The whitelisted value for the access control list.



## BackupConfiguration

BackupConfiguration is database instance backup configuration.

Appears in:

* [Settings](#Settings)


Name | Type | Description
-----|------|------------
`binaryLogEnabled` | Optional bool | BinaryLogEnabled: Whether binary log is enabled. If backup configuration is disabled, binary log must be disabled as well.
`enabled` | Optional bool | Enabled: Whether this configuration is enabled.
`location` | Optional string | Location: The location of the backup.
`replicationLogArchivingEnabled` | Optional bool | ReplicationLogArchivingEnabled: Reserved for future use.
`startTime` | Optional string | StartTime: Start time for the daily backup configuration in UTC timezone in the 24 hour format - HH:MM.



## CloudSQLInstanceClassSpecTemplate

A CloudSQLInstanceClassSpecTemplate is a template for the spec of a dynamically provisioned CloudSQLInstance.

Appears in:

* [CloudSQLInstanceClass](#CloudSQLInstanceClass)


Name | Type | Description
-----|------|------------
`forProvider` | [CloudSQLInstanceParameters](#CloudSQLInstanceParameters) | CloudSQLInstanceParameters define the desired state of a Google CloudSQL instance. Most of its fields are direct mirror of GCP DatabaseInstance object. See https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances#DatabaseInstance


CloudSQLInstanceClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)


## CloudSQLInstanceObservation

CloudSQLInstanceObservation is used to show the observed state of the Cloud SQL resource on GCP.

Appears in:

* [CloudSQLInstanceStatus](#CloudSQLInstanceStatus)


Name | Type | Description
-----|------|------------
`backendType` | string | BackendType: FIRST_GEN: First Generation instance. MySQL only. SECOND_GEN: Second Generation instance or PostgreSQL instance. EXTERNAL: A database server that is not managed by Google. This property is read-only; use the tier property in the settings object to determine the database type and Second or First Generation.
`currentDiskSize` | int64 | CurrentDiskSize: The current disk usage of the instance in bytes. This property has been deprecated. Users should use the &#34;cloudsql.googleapis.com/database/disk/bytes_used&#34; metric in Cloud Monitoring API instead. Please see this announcement for details.
`connectionName` | string | ConnectionName: Connection name of the Cloud SQL instance used in connection strings.
`diskEncryptionStatus` | [DiskEncryptionStatus](#DiskEncryptionStatus) | DiskEncryptionStatus: Disk encryption status specific to an instance. Applies only to Second Generation instances.
`failoverReplica` | [DatabaseInstanceFailoverReplicaStatus](#DatabaseInstanceFailoverReplicaStatus) | FailoverReplica: The name and status of the failover replica. This property is applicable only to Second Generation instances.
`gceZone` | string | GceZone: The Compute Engine zone that the instance is currently serving from. This value could be different from the zone that was specified when the instance was created if the instance has failed over to its secondary zone.
`ipAddresses` | [[]*github.com/crossplaneio/stack-gcp/apis/database/v1beta1.IPMapping](#*github.com/crossplaneio/stack-gcp/apis/database/v1beta1.IPMapping) | IPAddresses: The assigned IP addresses for the instance.
`ipv6Address` | string | IPv6Address: The IPv6 address assigned to the instance. This property is applicable only to First Generation instances.
`project` | string | Project: The project ID of the project containing the Cloud SQL instance. The Google apps domain is prefixed if applicable.
`selfLink` | string | SelfLink: The URI of this resource.
`serviceAccountEmailAddress` | string | ServiceAccountEmailAddress: The service account email address assigned to the instance. This property is applicable only to Second Generation instances.
`state` | string | State: The current serving state of the Cloud SQL instance. This can be one of the following. RUNNABLE: The instance is running, or is ready to run when accessed. SUSPENDED: The instance is not available, for example due to problems with billing. PENDING_CREATE: The instance is being created. MAINTENANCE: The instance is down for maintenance. FAILED: The instance creation failed. UNKNOWN_STATE: The state of the instance is unknown.
`settingsVersion` | int64 | SettingsVersion: The version of instance settings. This is a required field for update method to make sure concurrent updates are handled properly. During update, use the most recent settingsVersion value for this instance and do not try to update this value.



## CloudSQLInstanceParameters

CloudSQLInstanceParameters define the desired state of a Google CloudSQL instance. Most of its fields are direct mirror of GCP DatabaseInstance object. See https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances#DatabaseInstance

Appears in:

* [CloudSQLInstanceClassSpecTemplate](#CloudSQLInstanceClassSpecTemplate)
* [CloudSQLInstanceSpec](#CloudSQLInstanceSpec)


Name | Type | Description
-----|------|------------
`region` | string | Region: The geographical region. Can be us-central (FIRST_GEN instances only), us-central1 (SECOND_GEN instances only), asia-east1 or europe-west1. Defaults to us-central or us-central1 depending on the instance type (First Generation or Second Generation). The region can not be changed after instance creation.
`settings` | [Settings](#Settings) | Settings: The user settings.
`databaseVersion` | Optional string | DatabaseVersion: The database engine type and version. The databaseVersion field can not be changed after instance creation. MySQL Second Generation instances: MYSQL_5_7 (default) or MYSQL_5_6. PostgreSQL instances: POSTGRES_9_6 (default) or POSTGRES_11 Beta. MySQL First Generation instances: MYSQL_5_6 (default) or MYSQL_5_5
`masterInstanceName` | Optional string | MasterInstanceName: The name of the instance which will act as master in the replication setup.
`diskEncryptionConfiguration` | Optional [DiskEncryptionConfiguration](#DiskEncryptionConfiguration) | DiskEncryptionConfiguration: Disk encryption configuration specific to an instance. Applies only to Second Generation instances.
`failoverReplica` | Optional [DatabaseInstanceFailoverReplicaSpec](#DatabaseInstanceFailoverReplicaSpec) | FailoverReplica: The name and status of the failover replica. This property is applicable only to Second Generation instances.
`gceZone` | Optional string | GceZone: The Compute Engine zone that the instance is currently serving from. This value could be different from the zone that was specified when the instance was created if the instance has failed over to its secondary zone.
`instanceType` | Optional string | InstanceType: The instance type. This can be one of the following. CLOUD_SQL_INSTANCE: A Cloud SQL instance that is not replicating from a master. ON_PREMISES_INSTANCE: An instance running on the customer&#39;s premises. READ_REPLICA_INSTANCE: A Cloud SQL instance configured as a read-replica.
`maxDiskSize` | Optional int64 | MaxDiskSize: The maximum disk size of the instance in bytes.
`onPremisesConfiguration` | Optional [OnPremisesConfiguration](#OnPremisesConfiguration) | OnPremisesConfiguration: Configuration specific to on-premises instances.
`replicaNames` | Optional []string | ReplicaNames: The replicas of the instance.
`suspensionReason` | Optional []string | SuspensionReason: If the instance state is SUSPENDED, the reason for the suspension.



## CloudSQLInstanceSpec

A CloudSQLInstanceSpec defines the desired state of a CloudSQLInstance.

Appears in:

* [CloudSQLInstance](#CloudSQLInstance)


Name | Type | Description
-----|------|------------
`forProvider` | [CloudSQLInstanceParameters](#CloudSQLInstanceParameters) | CloudSQLInstanceParameters define the desired state of a Google CloudSQL instance. Most of its fields are direct mirror of GCP DatabaseInstance object. See https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances#DatabaseInstance


CloudSQLInstanceSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## CloudSQLInstanceStatus

A CloudSQLInstanceStatus represents the observed state of a CloudSQLInstance.

Appears in:

* [CloudSQLInstance](#CloudSQLInstance)


Name | Type | Description
-----|------|------------
`atProvider` | [CloudSQLInstanceObservation](#CloudSQLInstanceObservation) | CloudSQLInstanceObservation is used to show the observed state of the Cloud SQL resource on GCP.


CloudSQLInstanceStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## DatabaseFlags

DatabaseFlags are database flags for Cloud SQL instances.


Name | Type | Description
-----|------|------------
`name` | string | Name: The name of the flag. These flags are passed at instance startup, so include both server options and system variables for MySQL. Flags should be specified with underscores, not hyphens. For more information, see Configuring Database Flags in the Cloud SQL documentation.
`value` | string | Value: The value of the flag. Booleans should be set to on for true and off for false. This field must be omitted if the flag doesn&#39;t take a value.



## DatabaseInstanceFailoverReplicaSpec

DatabaseInstanceFailoverReplicaSpec is where you can specify a name for the failover replica.

Appears in:

* [CloudSQLInstanceParameters](#CloudSQLInstanceParameters)


Name | Type | Description
-----|------|------------
`name` | string | Name: The name of the failover replica. If specified at instance creation, a failover replica is created for the instance. The name doesn&#39;t include the project ID. This property is applicable only to Second Generation instances.



## DatabaseInstanceFailoverReplicaStatus

DatabaseInstanceFailoverReplicaStatus is status of the failover replica.

Appears in:

* [CloudSQLInstanceObservation](#CloudSQLInstanceObservation)


Name | Type | Description
-----|------|------------
`available` | bool | Available: The availability status of the failover replica. A false status indicates that the failover replica is out of sync. The master can only failover to the failover replica when the status is true.



## DiskEncryptionConfiguration

DiskEncryptionConfiguration is disk encryption configuration.

Appears in:

* [CloudSQLInstanceParameters](#CloudSQLInstanceParameters)


Name | Type | Description
-----|------|------------
`kmsKeyName` | string | KmsKeyName: KMS key resource name



## DiskEncryptionStatus

DiskEncryptionStatus is disk encryption status.

Appears in:

* [CloudSQLInstanceObservation](#CloudSQLInstanceObservation)


Name | Type | Description
-----|------|------------
`kmsKeyVersionName` | string | KmsKeyVersionName: KMS key version used to encrypt the Cloud SQL instance disk



## IPConfiguration

IPConfiguration is the IP Management configuration.

Appears in:

* [Settings](#Settings)


Name | Type | Description
-----|------|------------
`authorizedNetworks` | Optional [[]*github.com/crossplaneio/stack-gcp/apis/database/v1beta1.ACLEntry](#*github.com/crossplaneio/stack-gcp/apis/database/v1beta1.ACLEntry) | AuthorizedNetworks: The list of external networks that are allowed to connect to the instance using the IP. In CIDR notation, also known as &#39;slash&#39; notation (e.g. 192.168.100.0/24).
`ipv4Enabled` | Optional bool | Ipv4Enabled: Whether the instance should be assigned an IP address or not.
`privateNetwork` | Optional string | PrivateNetwork: The resource link for the VPC network from which the Cloud SQL instance is accessible for private IP. For example, /projects/myProject/global/networks/default. This setting can be updated, but it cannot be removed after it is set.
`privateNetworkRef` | [NetworkURIReferencerForCloudSQLInstance](#NetworkURIReferencerForCloudSQLInstance) | PrivateNetworkRef sets the PrivateNetwork field by resolving the resource link of the referenced Crossplane Network managed resource. The Network must have an active Service Networking connection peering before resolution will proceed. https://cloud.google.com/vpc/docs/configure-private-services-access
`requireSsl` | Optional bool | RequireSsl: Whether SSL connections over IP should be enforced or not.



## IPMapping

IPMapping is database instance IP Mapping.


Name | Type | Description
-----|------|------------
`ipAddress` | string | IPAddress: The IP address assigned.
`timeToRetire` | string | TimeToRetire: The due time for this IP to be retired in RFC 3339 format, for example 2012-11-15T16:19:00.094Z. This field is only available when the IP is scheduled to be retired.
`type` | string | Type: The type of this IP address. A PRIMARY address is a public address that can accept incoming connections. A PRIVATE address is a private address that can accept incoming connections. An OUTGOING address is the source address of connections originating from the instance, if supported.



## LocationPreference

LocationPreference is preferred location. This specifies where a Cloud SQL instance should preferably be located, either in a specific Compute Engine zone, or co-located with an App Engine application. Note that if the preferred location is not available, the instance will be located as close as possible within the region. Only one location may be specified.

Appears in:

* [Settings](#Settings)


Name | Type | Description
-----|------|------------
`followGaeApplication` | Optional string | FollowGaeApplication: The AppEngine application to follow, it must be in the same region as the Cloud SQL instance.
`zone` | Optional string | Zone: The preferred Compute Engine zone (e.g. us-central1-a, us-central1-b, etc.).



## MaintenanceWindow

MaintenanceWindow specifies when a v2 Cloud SQL instance should preferably be restarted for system maintenance purposes.

Appears in:

* [Settings](#Settings)


Name | Type | Description
-----|------|------------
`day` | Optional int64 | Day: day of week (1-7), starting on Monday.
`hour` | Optional int64 | Hour: hour of day - 0 to 23.
`updateTrack` | Optional string | UpdateTrack: Maintenance timing setting: canary (Earlier) or stable (Later).



## NetworkURIReferencerForCloudSQLInstance

NetworkURIReferencerForCloudSQLInstance resolves references from a CloudSQLInstance to a Network by returning the referenced Network&#39;s resource link, e.g. /projects/example/global/networks/example.

Appears in:

* [IPConfiguration](#IPConfiguration)




NetworkURIReferencerForCloudSQLInstance supports all fields of:

* [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core)


## OnPremisesConfiguration

OnPremisesConfiguration is on-premises instance configuration.

Appears in:

* [CloudSQLInstanceParameters](#CloudSQLInstanceParameters)


Name | Type | Description
-----|------|------------
`hostPort` | string | HostPort: The host and port of the on-premises instance in host:port format



## Settings

Settings is Cloud SQL database instance settings.

Appears in:

* [CloudSQLInstanceParameters](#CloudSQLInstanceParameters)


Name | Type | Description
-----|------|------------
`tier` | string | Tier: The tier (or machine type) for this instance, for example db-n1-standard-1 (MySQL instances) or db-custom-1-3840 (PostgreSQL instances). For MySQL instances, this property determines whether the instance is First or Second Generation. For more information, see Instance Settings.
`activationPolicy` | Optional string | ActivationPolicy: The activation policy specifies when the instance is activated; it is applicable only when the instance state is RUNNABLE. Valid values: ALWAYS: The instance is on, and remains so even in the absence of connection requests. NEVER: The instance is off; it is not activated, even if a connection request arrives. ON_DEMAND: First Generation instances only. The instance responds to incoming requests, and turns itself off when not in use. Instances with PER_USE pricing turn off after 15 minutes of inactivity. Instances with PER_PACKAGE pricing turn off after 12 hours of inactivity.
`authorizedGaeApplications` | Optional []string | AuthorizedGaeApplications: The App Engine app IDs that can access this instance. First Generation instances only.
`availabilityType` | Optional string | AvailabilityType: Availability type (PostgreSQL instances only). Potential values: ZONAL: The instance serves data from only one zone. Outages in that zone affect data accessibility. REGIONAL: The instance can serve data from more than one zone in a region (it is highly available). For more information, see Overview of the High Availability Configuration.
`crashSafeReplicationEnabled` | Optional bool | CrashSafeReplicationEnabled: Configuration specific to read replica instances. Indicates whether database flags for crash-safe replication are enabled. This property is only applicable to First Generation instances.
`storageAutoResize` | Optional bool | StorageAutoResize: Configuration to increase storage size automatically. The default value is true. Not used for First Generation instances.
`dataDiskType` | Optional string | DataDiskType: The type of data disk: PD_SSD (default) or PD_HDD. Not used for First Generation instances.
`pricingPlan` | Optional string | PricingPlan: The pricing plan for this instance. This can be either PER_USE or PACKAGE. Only PER_USE is supported for Second Generation instances.
`replicationType` | Optional string | ReplicationType: The type of replication this instance uses. This can be either ASYNCHRONOUS or SYNCHRONOUS. This property is only applicable to First Generation instances.
`userLabels` | Optional map[string]string | UserLabels: User-provided labels, represented as a dictionary where each label is a single key value pair.
`databaseFlags` | Optional [[]*github.com/crossplaneio/stack-gcp/apis/database/v1beta1.DatabaseFlags](#*github.com/crossplaneio/stack-gcp/apis/database/v1beta1.DatabaseFlags) | DatabaseFlags is the array of database flags passed to the instance at startup.
`backupConfiguration` | Optional [BackupConfiguration](#BackupConfiguration) | BackupConfiguration is the daily backup configuration for the instance.
`ipConfiguration` | Optional [IPConfiguration](#IPConfiguration) | IPConfiguration: The settings for IP Management. This allows to enable or disable the instance IP and manage which external networks can connect to the instance. The IPv4 address cannot be disabled for Second Generation instances.
`locationPreference` | Optional [LocationPreference](#LocationPreference) | LocationPreference is the location preference settings. This allows the instance to be located as near as possible to either an App Engine app or Compute Engine zone for better performance. App Engine co-location is only applicable to First Generation instances.
`maintenanceWindow` | Optional [MaintenanceWindow](#MaintenanceWindow) | MaintenanceWindow: The maintenance window for this instance. This specifies when the instance can be restarted for maintenance purposes. Not used for First Generation instances.
`dataDiskSizeGb` | Optional int64 | DataDiskSizeGb: The size of data disk, in GB. The data disk size minimum is 10GB. Not used for First Generation instances.
`databaseReplicationEnabled` | Optional bool | DatabaseReplicationEnabled: Configuration specific to read replica instances. Indicates whether replication is enabled or not.
`storageAutoResizeLimit` | Optional int64 | StorageAutoResizeLimit: The maximum size to which storage capacity can be automatically increased. The default value is 0, which specifies that there is no limit. Not used for First Generation instances.



This API documentation was generated by `crossdocs`.