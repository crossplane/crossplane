# database.aws.crossplane.io/v1beta1 API Reference

Package v1beta1 contains managed resources for AWS database services such as RDS.

This API group contains the following Crossplane resources:

* [RDSInstance](#RDSInstance)
* [RDSInstanceClass](#RDSInstanceClass)

## RDSInstance

An RDSInstance is a managed resource that represents an AWS Relational Database Service instance.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.aws.crossplane.io/v1beta1`
`kind` | string | `RDSInstance`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [RDSInstanceSpec](#RDSInstanceSpec) | An RDSInstanceSpec defines the desired state of an RDSInstance.
`status` | [RDSInstanceStatus](#RDSInstanceStatus) | An RDSInstanceStatus represents the observed state of an RDSInstance.



## RDSInstanceClass

An RDSInstanceClass is a resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.aws.crossplane.io/v1beta1`
`kind` | string | `RDSInstanceClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [RDSInstanceClassSpecTemplate](#RDSInstanceClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned RDSInstance.



## AvailabilityZone

AvailabilityZone contains Availability Zone information. This data type is used as an element in the following data type:    * OrderableDBInstanceOption Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/AvailabilityZone

Appears in:

* [SubnetInRDS](#SubnetInRDS)


Name | Type | Description
-----|------|------------
`name` | string | Name of the Availability Zone.



## CloudwatchLogsExportConfiguration

CloudwatchLogsExportConfiguration is the configuration setting for the log types to be enabled for export to CloudWatch Logs for a specific DB instance or DB cluster. The EnableLogTypes and DisableLogTypes arrays determine which logs will be exported (or not exported) to CloudWatch Logs. The values within these arrays depend on the DB engine being used. For more information, see Publishing Database Logs to Amazon CloudWatch Logs  (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_LogAccess.html#USER_LogAccess.Procedural.UploadtoCloudWatch) in the Amazon RDS User Guide. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/CloudwatchLogsExportConfiguration

Appears in:

* [RDSInstanceParameters](#RDSInstanceParameters)


Name | Type | Description
-----|------|------------
`disableLogTypes` | []string | DisableLogTypes is the list of log types to disable.
`enableLogTypes` | []string | EnableLogTypes is the list of log types to enable.



## DBInstanceStatusInfo

DBInstanceStatusInfo provides a list of status information for a DB instance. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/DBInstanceStatusInfo

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`message` | string | Message is the details of the error if there is an error for the instance. If the instance is not in an error state, this value is blank.
`normal` | bool | Normal is true if the instance is operating normally, or false if the instance is in an error state.
`status` | string | Status of the DB instance. For a StatusType of read replica, the values can be replicating, replication stop point set, replication stop point reached, error, stopped, or terminated.
`statusType` | string | StatusType is currently &#34;read replication.&#34;



## DBParameterGroupStatus

DBParameterGroupStatus is the status of the DB parameter group. This data type is used as a response element in the following actions:    * CreateDBInstance    * CreateDBInstanceReadReplica    * DeleteDBInstance    * ModifyDBInstance    * RebootDBInstance    * RestoreDBInstanceFromDBSnapshot Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/DBParameterGroupStatus

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`dbParameterGroupName` | string | DBParameterGroupName is the name of the DP parameter group.
`parameterApplyStatus` | string | ParameterApplyStatus is the status of parameter updates.



## DBSecurityGroupMembership

DBSecurityGroupMembership is used as a response element in the following actions:    * ModifyDBInstance    * RebootDBInstance    * RestoreDBInstanceFromDBSnapshot    * RestoreDBInstanceToPointInTime Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/DBSecurityGroupMembership

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`dbSecurityGroupName` | string | DBSecurityGroupName is the name of the DB security group.
`status` | string | Status is the status of the DB security group.



## DBSubnetGroupInRDS

DBSubnetGroupInRDS contains the details of an Amazon RDS DB subnet group. This data type is used as a response element in the DescribeDBSubnetGroups action. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/DBSubnetGroup

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`dbSubnetGroupArn` | string | DBSubnetGroupARN is the Amazon Resource Name (ARN) for the DB subnet group.
`dbSubnetGroupDescription` | string | DBSubnetGroupDescription provides the description of the DB subnet group.
`dbSubnetGroupName` | string | DBSubnetGroupName is the name of the DB subnet group.
`subnetGroupStatus` | string | SubnetGroupStatus provides the status of the DB subnet group.
`subnets` | [[]SubnetInRDS](#SubnetInRDS) | Subnets contains a list of Subnet elements.
`vpcId` | string | VPCID provides the VPCID of the DB subnet group.



## DBSubnetGroupNameReferencerForRDSInstance

DBSubnetGroupNameReferencerForRDSInstance is an attribute referencer that retrieves the name from a referenced DBSubnetGroup

Appears in:

* [RDSInstanceParameters](#RDSInstanceParameters)




DBSubnetGroupNameReferencerForRDSInstance supports all fields of:

* github.com/crossplaneio/stack-aws/apis/database/v1alpha3.DBSubnetGroupNameReferencer


## DomainMembership

DomainMembership is an Active Directory Domain membership record associated with the DB instance. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/DomainMembership

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`domain` | string | Domain is the identifier of the Active Directory Domain.
`fqdn` | string | FQDN us the fully qualified domain name of the Active Directory Domain.
`iamRoleName` | string | IAMRoleName is the name of the IAM role to be used when making API calls to the Directory Service.
`status` | string | Status of the DB instance&#39;s Active Directory Domain membership, such as joined, pending-join, failed etc).



## Endpoint

Endpoint is used as a response element in the following actions:    * CreateDBInstance    * DescribeDBInstances    * DeleteDBInstance Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/Endpoint

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`address` | string | Address specifies the DNS address of the DB instance.
`hostedZoneId` | string | HostedZoneID specifies the ID that Amazon Route 53 assigns when you create a hosted zone.
`port` | int | Port specifies the port that the database engine is listening on.



## IAMRoleARNReferencerForRDSInstanceMonitoringRole

IAMRoleARNReferencerForRDSInstanceMonitoringRole is an attribute referencer that retrieves an RDSInstance&#39;s MonitoringRoleARN from a referenced IAMRole.

Appears in:

* [RDSInstanceParameters](#RDSInstanceParameters)




IAMRoleARNReferencerForRDSInstanceMonitoringRole supports all fields of:

* github.com/crossplaneio/stack-aws/apis/identity/v1alpha3.IAMRoleARNReferencer


## IAMRoleNameReferencerForRDSInstanceDomainRole

IAMRoleNameReferencerForRDSInstanceDomainRole is an attribute referencer that retrieves an RDSInstance&#39;s DomainRoleName from a referenced IAMRole.

Appears in:

* [RDSInstanceParameters](#RDSInstanceParameters)




IAMRoleNameReferencerForRDSInstanceDomainRole supports all fields of:

* github.com/crossplaneio/stack-aws/apis/identity/v1alpha3.IAMRoleNameReferencer


## OptionGroupMembership

OptionGroupMembership provides information on the option groups the DB instance is a member of. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/OptionGroupMembership

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`optionGroupName` | string | OptionGroupName is the name of the option group that the instance belongs to.
`status` | string | Status is the status of the DB instance&#39;s option group membership. Valid values are: in-sync, pending-apply, pending-removal, pending-maintenance-apply, pending-maintenance-removal, applying, removing, and failed.



## PendingCloudwatchLogsExports

PendingCloudwatchLogsExports is a list of the log types whose configuration is still pending. In other words, these log types are in the process of being activated or deactivated. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/PendingCloudwatchLogsExports

Appears in:

* [PendingModifiedValues](#PendingModifiedValues)


Name | Type | Description
-----|------|------------
`logTypesToDisable` | []string | LogTypesToDisable is list of log types that are in the process of being enabled. After they are enabled, these log types are exported to CloudWatch Logs.
`logTypesToEnable` | []string | LogTypesToEnable is the log types that are in the process of being deactivated. After they are deactivated, these log types aren&#39;t exported to CloudWatch Logs.



## PendingModifiedValues

PendingModifiedValues is used as a response element in the ModifyDBInstance action. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/PendingModifiedValues

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`allocatedStorage` | int | AllocatedStorage contains the new AllocatedStorage size for the DB instance that will be applied or is currently being applied.
`backupRetentionPeriod` | int | BackupRetentionPeriod specifies the pending number of days for which automated backups are retained.
`caCertificateIdentifier` | string | CACertificateIdentifier specifies the identifier of the CA certificate for the DB instance.
`dbInstanceClass` | string | DBInstanceClass contains the new DBInstanceClass for the DB instance that will be applied or is currently being applied.
`dbSubnetGroupName` | string | DBSubnetGroupName is the new DB subnet group for the DB instance.
`engineVersion` | string | EngineVersion indicates the database engine version.
`iops` | int | IOPS specifies the new Provisioned IOPS value for the DB instance that will be applied or is currently being applied.
`licenseModel` | string | LicenseModel is the license model for the DB instance. Valid values: license-included | bring-your-own-license | general-public-license
`multiAZ` | bool | MultiAZ indicates that the Single-AZ DB instance is to change to a Multi-AZ deployment.
`pendingCloudwatchLogsExports` | [PendingCloudwatchLogsExports](#PendingCloudwatchLogsExports) | PendingCloudwatchLogsExports is a list of the log types whose configuration is still pending. In other words, these log types are in the process of being activated or deactivated.
`port` | int | Port specifies the pending port for the DB instance.
`processorFeatures` | [[]ProcessorFeature](#ProcessorFeature) | ProcessorFeatures is the number of CPU cores and the number of threads per core for the DB instance class of the DB instance.
`storageType` | string | StorageType specifies the storage type to be associated with the DB instance.



## ProcessorFeature

ProcessorFeature is a processor feature entry. For more information, see Configuring the Processor of the DB Instance Class (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.DBInstanceClass.html#USER_ConfigureProcessor) in the Amazon RDS User Guide. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/ProcessorFeature

Appears in:

* [PendingModifiedValues](#PendingModifiedValues)
* [RDSInstanceParameters](#RDSInstanceParameters)


Name | Type | Description
-----|------|------------
`name` | string | Name of the processor feature. Valid names are coreCount and threadsPerCore.
`value` | string | Value of a processor feature name.



## RDSInstanceClassSpecTemplate

An RDSInstanceClassSpecTemplate is a template for the spec of a dynamically provisioned RDSInstance.

Appears in:

* [RDSInstanceClass](#RDSInstanceClass)


Name | Type | Description
-----|------|------------
`forProvider` | [RDSInstanceParameters](#RDSInstanceParameters) | RDSInstanceParameters define the desired state of an AWS Relational Database Service instance.


RDSInstanceClassSpecTemplate supports all fields of:

* [v1alpha1.ClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#classspectemplate)


## RDSInstanceObservation

RDSInstanceObservation is the representation of the current state that is observed.

Appears in:

* [RDSInstanceStatus](#RDSInstanceStatus)


Name | Type | Description
-----|------|------------
`dbInstanceStatus` | string | DBInstanceStatus specifies the current state of this database.
`dbInstanceArn` | string | DBInstanceArn is the Amazon Resource Name (ARN) for the DB instance.
`dbParameterGroups` | [[]DBParameterGroupStatus](#DBParameterGroupStatus) | DBParameterGroups provides the list of DB parameter groups applied to this DB instance.
`dbSecurityGroups` | [[]DBSecurityGroupMembership](#DBSecurityGroupMembership) | DBSecurityGroups provides List of DB security group elements containing only DBSecurityGroup.Name and DBSecurityGroup.Status subelements.
`dbSubnetGroup` | [DBSubnetGroupInRDS](#DBSubnetGroupInRDS) | DBSubnetGroup specifies information on the subnet group associated with the DB instance, including the name, description, and subnets in the subnet group.
`dbInstancePort` | int | DBInstancePort specifies the port that the DB instance listens on. If the DB instance is part of a DB cluster, this can be a different port than the DB cluster port.
`dbResourceId` | string | DBResourceID is the AWS Region-unique, immutable identifier for the DB instance. This identifier is found in AWS CloudTrail log entries whenever the AWS KMS key for the DB instance is accessed.
`domainMemberships` | [[]DomainMembership](#DomainMembership) | DomainMemberships is the Active Directory Domain membership records associated with the DB instance.
`instanceCreateTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | InstanceCreateTime provides the date and time the DB instance was created.
`endpoint` | [Endpoint](#Endpoint) | Endpoint specifies the connection endpoint.
`enhancedMonitoringResourceArn` | string | EnhancedMonitoringResourceArn is the Amazon Resource Name (ARN) of the Amazon CloudWatch Logs log stream that receives the Enhanced Monitoring metrics data for the DB instance.
`latestRestorableTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | LatestRestorableTime specifies the latest time to which a database can be restored with point-in-time restore.
`optionGroupMemberships` | [[]OptionGroupMembership](#OptionGroupMembership) | OptionGroupMemberships provides the list of option group memberships for this DB instance.
`pendingModifiedValues` | [PendingModifiedValues](#PendingModifiedValues) | PendingModifiedValues specifies that changes to the DB instance are pending. This element is only included when changes are pending. Specific changes are identified by subelements.
`performanceInsightsEnabled` | bool | PerformanceInsightsEnabled is true if Performance Insights is enabled for the DB instance, and otherwise false.
`readReplicaDBClusterIdentifiers` | []string | ReadReplicaDBClusterIdentifiers contains one or more identifiers of Aurora DB clusters to which the RDS DB instance is replicated as a Read Replica. For example, when you create an Aurora Read Replica of an RDS MySQL DB instance, the Aurora MySQL DB cluster for the Aurora Read Replica is shown. This output does not contain information about cross region Aurora Read Replicas.
`readReplicaDBInstanceIdentifiers` | []string | ReadReplicaDBInstanceIdentifiers contains one or more identifiers of the Read Replicas associated with this DB instance.
`readReplicaSourceDBInstanceIdentifier` | string | ReadReplicaSourceDBInstanceIdentifier contains the identifier of the source DB instance if this DB instance is a Read Replica.
`secondaryAvailabilityZone` | string | SecondaryAvailabilityZone specifies the name of the secondary Availability Zone for a DB instance with multi-AZ support when it is present.
`statusInfos` | [[]DBInstanceStatusInfo](#DBInstanceStatusInfo) | StatusInfos is the status of a Read Replica. If the instance is not a Read Replica, this is blank.
`vpcSecurityGroups` | [[]VPCSecurityGroupMembership](#VPCSecurityGroupMembership) | VPCSecurityGroups provides a list of VPC security group elements that the DB instance belongs to.



## RDSInstanceParameters

RDSInstanceParameters define the desired state of an AWS Relational Database Service instance.

Appears in:

* [RDSInstanceClassSpecTemplate](#RDSInstanceClassSpecTemplate)
* [RDSInstanceSpec](#RDSInstanceSpec)


Name | Type | Description
-----|------|------------
`allocatedStorage` | Optional int | AllocatedStorage is the amount of storage (in gibibytes) to allocate for the DB instance. Type: Integer Amazon Aurora Not applicable. Aurora cluster volumes automatically grow as the amount of data in your database increases, though you are only charged for the space that you use in an Aurora cluster volume. MySQL Constraints to the amount of storage for each storage type are the following:    * General Purpose (SSD) storage (gp2): Must be an integer from 20 to 16384.    * Provisioned IOPS storage (io1): Must be an integer from 100 to 16384.    * Magnetic storage (standard): Must be an integer from 5 to 3072. MariaDB Constraints to the amount of storage for each storage type are the following:    * General Purpose (SSD) storage (gp2): Must be an integer from 20 to 16384.    * Provisioned IOPS storage (io1): Must be an integer from 100 to 16384.    * Magnetic storage (standard): Must be an integer from 5 to 3072. PostgreSQL Constraints to the amount of storage for each storage type are the following:    * General Purpose (SSD) storage (gp2): Must be an integer from 20 to 16384.    * Provisioned IOPS storage (io1): Must be an integer from 100 to 16384.    * Magnetic storage (standard): Must be an integer from 5 to 3072. Oracle Constraints to the amount of storage for each storage type are the following:    * General Purpose (SSD) storage (gp2): Must be an integer from 20 to 16384.    * Provisioned IOPS storage (io1): Must be an integer from 100 to 16384.    * Magnetic storage (standard): Must be an integer from 10 to 3072. SQL Server Constraints to the amount of storage for each storage type are the following:    * General Purpose (SSD) storage (gp2): Enterprise and Standard editions: Must be an integer from 200 to 16384. Web and Express editions: Must be an integer from 20 to 16384.    * Provisioned IOPS storage (io1): Enterprise and Standard editions: Must be an integer from 200 to 16384. Web and Express editions: Must be an integer from 100 to 16384.    * Magnetic storage (standard): Enterprise and Standard editions: Must be an integer from 200 to 1024. Web and Express editions: Must be an integer from 20 to 1024.
`autoMinorVersionUpgrade` | Optional bool | AutoMinorVersionUpgrade indicates that minor engine upgrades are applied automatically to the DB instance during the maintenance window. Default: true
`availabilityZone` | Optional string | AvailabilityZone is the EC2 Availability Zone that the DB instance is created in. For information on AWS Regions and Availability Zones, see Regions and Availability Zones (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.RegionsAndAvailabilityZones.html). Default: A random, system-chosen Availability Zone in the endpoint&#39;s AWS Region. Example: us-east-1d Constraint: The AvailabilityZone parameter can&#39;t be specified if the MultiAZ parameter is set to true. The specified Availability Zone must be in the same AWS Region as the current endpoint.
`backupRetentionPeriod` | Optional int | BackupRetentionPeriod is the number of days for which automated backups are retained. Setting this parameter to a positive number enables backups. Setting this parameter to 0 disables automated backups. Amazon Aurora Not applicable. The retention period for automated backups is managed by the DB cluster. For more information, see CreateDBCluster. Default: 1 Constraints:    * Must be a value from 0 to 35    * Cannot be set to 0 if the DB instance is a source to Read Replicas
`caCertificateIdentifier` | Optional string | CACertificateIdentifier indicates the certificate that needs to be associated with the instance.
`characterSetName` | Optional string | CharacterSetName indicates that the DB instance should be associated with the specified CharacterSet for supported engines, Amazon Aurora Not applicable. The character set is managed by the DB cluster. For more information, see CreateDBCluster.
`copyTagsToSnapshot` | Optional bool | CopyTagsToSnapshot should be true to copy all tags from the DB instance to snapshots of the DB instance, and otherwise false. The default is false.
`dbClusterIdentifier` | Optional string | DBClusterIdentifier is the identifier of the DB cluster that the instance will belong to. For information on creating a DB cluster, see CreateDBCluster. Type: String
`dbClusterParameterGroupName` | Optional string | DBClusterParameterGroupName is the name of the DB cluster parameter group to use for the DB cluster.
`dbInstanceClass` | string | DBInstanceClass is the compute and memory capacity of the DB instance, for example, db.m4.large. Not all DB instance classes are available in all AWS Regions, or for all database engines. For the full list of DB instance classes, and availability for your engine, see DB Instance Class (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.DBInstanceClass.html) in the Amazon RDS User Guide.
`dbName` | Optional string | DBName is the meaning of this parameter differs according to the database engine you use. Type: String MySQL The name of the database to create when the DB instance is created. If this parameter is not specified, no database is created in the DB instance. Constraints:    * Must contain 1 to 64 letters or numbers.    * Cannot be a word reserved by the specified database engine MariaDB The name of the database to create when the DB instance is created. If this parameter is not specified, no database is created in the DB instance. Constraints:    * Must contain 1 to 64 letters or numbers.    * Cannot be a word reserved by the specified database engine PostgreSQL The name of the database to create when the DB instance is created. If this parameter is not specified, the default &#34;postgres&#34; database is created in the DB instance. Constraints:    * Must contain 1 to 63 letters, numbers, or underscores.    * Must begin with a letter or an underscore. Subsequent characters can    be letters, underscores, or digits (0-9).    * Cannot be a word reserved by the specified database engine Oracle The Oracle System ID (SID) of the created DB instance. If you specify null, the default value ORCL is used. You can&#39;t specify the string NULL, or any other reserved word, for DBName. Default: ORCL Constraints:    * Cannot be longer than 8 characters SQL Server Not applicable. Must be null. Amazon Aurora The name of the database to create when the primary instance of the DB cluster is created. If this parameter is not specified, no database is created in the DB instance. Constraints:    * Must contain 1 to 64 letters or numbers.    * Cannot be a word reserved by the specified database engine
`dbSecurityGroups` | Optional []string | DBSecurityGroups is a list of DB security groups to associate with this DB instance. Default: The default DB security group for the database engine.
`dbSubnetGroupName` | Optional string | DBSubnetGroupName is a DB subnet group to associate with this DB instance. If there is no DB subnet group, then it is a non-VPC DB instance.
`dbSubnetGroupNameRef` | Optional [DBSubnetGroupNameReferencerForRDSInstance](#DBSubnetGroupNameReferencerForRDSInstance) | DBSubnetGroupNameRef is a reference to a DBSubnetGroup used to set DBSubnetGroupName.
`deletionProtection` | Optional bool | DeletionProtection indicates if the DB instance should have deletion protection enabled. The database can&#39;t be deleted when this value is set to true. The default is false. For more information, see  Deleting a DB Instance (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_DeleteInstance.html).
`enableCloudwatchLogsExports` | Optional []string | EnableCloudwatchLogsExports is the list of log types that need to be enabled for exporting to CloudWatch Logs. The values in the list depend on the DB engine being used. For more information, see Publishing Database Logs to Amazon CloudWatch Logs  (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_LogAccess.html#USER_LogAccess.Procedural.UploadtoCloudWatch) in the Amazon Relational Database Service User Guide.
`enableIAMDatabaseAuthentication` | Optional bool | EnableIAMDatabaseAuthentication should be true to enable mapping of AWS Identity and Access Management (IAM) accounts to database accounts, and otherwise false. You can enable IAM database authentication for the following database engines: Amazon Aurora Not applicable. Mapping AWS IAM accounts to database accounts is managed by the DB cluster. For more information, see CreateDBCluster. MySQL    * For MySQL 5.6, minor version 5.6.34 or higher    * For MySQL 5.7, minor version 5.7.16 or higher Default: false
`enablePerformanceInsights` | Optional bool | EnablePerformanceInsights should be true to enable Performance Insights for the DB instance, and otherwise false. For more information, see Using Amazon Performance Insights (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_PerfInsights.html) in the Amazon Relational Database Service User Guide.
`engine` | string | Engine is the name of the database engine to be used for this instance. Not every database engine is available for every AWS Region. Valid Values:    * aurora (for MySQL 5.6-compatible Aurora)    * aurora-mysql (for MySQL 5.7-compatible Aurora)    * aurora-postgresql    * mariadb    * mysql    * oracle-ee    * oracle-se2    * oracle-se1    * oracle-se    * postgres    * sqlserver-ee    * sqlserver-se    * sqlserver-ex    * sqlserver-web Engine is a required field
`engineVersion` | Optional string | EngineVersion is the version number of the database engine to use. For a list of valid engine versions, call DescribeDBEngineVersions. The following are the database engines and links to information about the major and minor versions that are available with Amazon RDS. Not every database engine is available for every AWS Region. Amazon Aurora Not applicable. The version number of the database engine to be used by the DB instance is managed by the DB cluster. For more information, see CreateDBCluster. MariaDB See MariaDB on Amazon RDS Versions (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_MariaDB.html#MariaDB.Concepts.VersionMgmt) in the Amazon RDS User Guide. Microsoft SQL Server See Version and Feature Support on Amazon RDS (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_SQLServer.html#SQLServer.Concepts.General.FeatureSupport) in the Amazon RDS User Guide. MySQL See MySQL on Amazon RDS Versions (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_MySQL.html#MySQL.Concepts.VersionMgmt) in the Amazon RDS User Guide. Oracle See Oracle Database Engine Release Notes (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Appendix.Oracle.PatchComposition.html) in the Amazon RDS User Guide. PostgreSQL See Supported PostgreSQL Database Versions (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_PostgreSQL.html#PostgreSQL.Concepts.General.DBVersions) in the Amazon RDS User Guide.
`iops` | Optional int | IOPS is the amount of Provisioned IOPS (input/output operations per second) to be initially allocated for the DB instance. For information about valid IOPS values, see see Amazon RDS Provisioned IOPS Storage to Improve Performance (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_Storage.html#USER_PIOPS) in the Amazon RDS User Guide. Constraints: Must be a multiple between 1 and 50 of the storage amount for the DB instance. Must also be an integer multiple of 1000. For example, if the size of your DB instance is 500 GiB, then your IOPS value can be 2000, 3000, 4000, or 5000.
`kmsKeyId` | Optional string | KMSKeyID for an encrypted DB instance. The KMS key identifier is the Amazon Resource Name (ARN) for the KMS encryption key. If you are creating a DB instance with the same AWS account that owns the KMS encryption key used to encrypt the new DB instance, then you can use the KMS key alias instead of the ARN for the KM encryption key. Amazon Aurora Not applicable. The KMS key identifier is managed by the DB cluster. For more information, see CreateDBCluster. If the StorageEncrypted parameter is true, and you do not specify a value for the KMSKeyID parameter, then Amazon RDS will use your default encryption key. AWS KMS creates the default encryption key for your AWS account. Your AWS account has a different default encryption key for each AWS Region.
`licenseModel` | Optional string | LicenseModel information for this DB instance. Valid values: license-included | bring-your-own-license | general-public-license
`masterUsername` | Optional string | MasterUsername is the name for the master user. Amazon Aurora Not applicable. The name for the master user is managed by the DB cluster. For more information, see CreateDBCluster. MariaDB Constraints:    * Required for MariaDB.    * Must be 1 to 16 letters or numbers.    * Cannot be a reserved word for the chosen database engine. Microsoft SQL Server Constraints:    * Required for SQL Server.    * Must be 1 to 128 letters or numbers.    * The first character must be a letter.    * Cannot be a reserved word for the chosen database engine. MySQL Constraints:    * Required for MySQL.    * Must be 1 to 16 letters or numbers.    * First character must be a letter.    * Cannot be a reserved word for the chosen database engine. Oracle Constraints:    * Required for Oracle.    * Must be 1 to 30 letters or numbers.    * First character must be a letter.    * Cannot be a reserved word for the chosen database engine. PostgreSQL Constraints:    * Required for PostgreSQL.    * Must be 1 to 63 letters or numbers.    * First character must be a letter.    * Cannot be a reserved word for the chosen database engine.
`monitoringInterval` | Optional int | MonitoringInterval is the interval, in seconds, between points when Enhanced Monitoring metrics are collected for the DB instance. To disable collecting Enhanced Monitoring metrics, specify 0. The default is 0. If MonitoringRoleARN is specified, then you must also set MonitoringInterval to a value other than 0. Valid Values: 0, 1, 5, 10, 15, 30, 60
`monitoringRoleArn` | Optional string | MonitoringRoleARN is the ARN for the IAM role that permits RDS to send enhanced monitoring metrics to Amazon CloudWatch Logs. For example, arn:aws:iam:123456789012:role/emaccess. For information on creating a monitoring role, go to Setting Up and Enabling Enhanced Monitoring (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Monitoring.OS.html#USER_Monitoring.OS.Enabling) in the Amazon RDS User Guide. If MonitoringInterval is set to a value other than 0, then you must supply a MonitoringRoleARN value.
`monitoringRoleArnRef` | Optional [IAMRoleARNReferencerForRDSInstanceMonitoringRole](#IAMRoleARNReferencerForRDSInstanceMonitoringRole) | MonitoringRoleARNRef is a reference to an IAMRole used to set MonitoringRoleARN.
`multiAZ` | Optional bool | MultiAZ specifies if the DB instance is a Multi-AZ deployment. You can&#39;t set the AvailabilityZone parameter if the MultiAZ parameter is set to true.
`performanceInsightsKMSKeyId` | Optional string | PerformanceInsightsKMSKeyID is the AWS KMS key identifier for encryption of Performance Insights data. The KMS key ID is the Amazon Resource Name (ARN), KMS key identifier, or the KMS key alias for the KMS encryption key.
`performanceInsightsRetentionPeriod` | Optional int | PerformanceInsightsRetentionPeriod is the amount of time, in days, to retain Performance Insights data. Valid values are 7 or 731 (2 years).
`port` | Optional int | Port number on which the database accepts connections. MySQL Default: 3306 Valid Values: 1150-65535 Type: Integer MariaDB Default: 3306 Valid Values: 1150-65535 Type: Integer PostgreSQL Default: 5432 Valid Values: 1150-65535 Type: Integer Oracle Default: 1521 Valid Values: 1150-65535 SQL Server Default: 1433 Valid Values: 1150-65535 except for 1434, 3389, 47001, 49152, and 49152 through 49156. Amazon Aurora Default: 3306 Valid Values: 1150-65535 Type: Integer
`preferredBackupWindow` | Optional string | PreferredBackupWindow is the daily time range during which automated backups are created if automated backups are enabled, using the BackupRetentionPeriod parameter. For more information, see The Backup Window (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_WorkingWithAutomatedBackups.html#USER_WorkingWithAutomatedBackups.BackupWindow) in the Amazon RDS User Guide. Amazon Aurora Not applicable. The daily time range for creating automated backups is managed by the DB cluster. For more information, see CreateDBCluster. The default is a 30-minute window selected at random from an 8-hour block of time for each AWS Region. To see the time blocks available, see  Adjusting the Preferred DB Instance Maintenance Window (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_UpgradeDBInstance.Maintenance.html#AdjustingTheMaintenanceWindow) in the Amazon RDS User Guide. Constraints:    * Must be in the format hh24:mi-hh24:mi.    * Must be in Universal Coordinated Time (UTC).    * Must not conflict with the preferred maintenance window.    * Must be at least 30 minutes.
`preferredMaintenanceWindow` | Optional string | PreferredMaintenanceWindow is the time range each week during which system maintenance can occur, in Universal Coordinated Time (UTC). For more information, see Amazon RDS Maintenance Window (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_UpgradeDBInstance.Maintenance.html#Concepts.DBMaintenance). Format: ddd:hh24:mi-ddd:hh24:mi The default is a 30-minute window selected at random from an 8-hour block of time for each AWS Region, occurring on a random day of the week. Valid Days: Mon, Tue, Wed, Thu, Fri, Sat, Sun. Constraints: Minimum 30-minute window.
`processorFeatures` | Optional [[]ProcessorFeature](#ProcessorFeature) | ProcessorFeatures is the number of CPU cores and the number of threads per core for the DB instance class of the DB instance.
`promotionTier` | Optional int | PromotionTier specifies the order in which an Aurora Replica is promoted to the primary instance after a failure of the existing primary instance. For more information, see  Fault Tolerance for an Aurora DB Cluster (http://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Managing.Backups.html#Aurora.Managing.FaultTolerance) in the Amazon Aurora User Guide. Default: 1 Valid Values: 0 - 15
`publiclyAccessible` | Optional bool | PubliclyAccessible specifies the accessibility options for the DB instance. A value of true specifies an Internet-facing instance with a publicly resolvable DNS name, which resolves to a public IP address. A value of false specifies an internal instance with a DNS name that resolves to a private IP address. Default: The default behavior varies depending on whether DBSubnetGroupName is specified. If DBSubnetGroupName is not specified, and PubliclyAccessible is not specified, the following applies:    * If the default VPC in the target region doesn’t have an Internet gateway    attached to it, the DB instance is private.    * If the default VPC in the target region has an Internet gateway attached    to it, the DB instance is public. If DBSubnetGroupName is specified, and PubliclyAccessible is not specified, the following applies:    * If the subnets are part of a VPC that doesn’t have an Internet gateway    attached to it, the DB instance is private.    * If the subnets are part of a VPC that has an Internet gateway attached    to it, the DB instance is public.
`scalingConfiguration` | Optional [ScalingConfiguration](#ScalingConfiguration) | ScalingConfiguration is the scaling properties of the DB cluster. You can only modify scaling properties for DB clusters in serverless DB engine mode.
`storageEncrypted` | Optional bool | StorageEncrypted specifies whether the DB instance is encrypted. Amazon Aurora Not applicable. The encryption for DB instances is managed by the DB cluster. For more information, see CreateDBCluster. Default: false
`storageType` | Optional string | StorageType specifies the storage type to be associated with the DB instance. Valid values: standard | gp2 | io1 If you specify io1, you must also include a value for the IOPS parameter. Default: io1 if the IOPS parameter is specified, otherwise standard
`tags` | Optional [[]Tag](#Tag) | Tags. For more information, see Tagging Amazon RDS Resources (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Tagging.html) in the Amazon RDS User Guide.
`timezone` | Optional string | Timezone of the DB instance. The time zone parameter is currently supported only by Microsoft SQL Server (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_SQLServer.html#SQLServer.Concepts.General.TimeZone).
`vpcSecurityGroupIds` | Optional []string | VPCSecurityGroupIDs is a list of EC2 VPC security groups to associate with this DB instance. Amazon Aurora Not applicable. The associated list of EC2 VPC security groups is managed by the DB cluster. For more information, see CreateDBCluster. Default: The default EC2 VPC security group for the DB subnet group&#39;s VPC.
`vpcSecurityGroupIDRefs` | Optional [[]*github.com/crossplaneio/stack-aws/apis/database/v1beta1.VPCSecurityGroupIDReferencerForRDSInstance](#*github.com/crossplaneio/stack-aws/apis/database/v1beta1.VPCSecurityGroupIDReferencerForRDSInstance) | VPCSecurityGroupIDRefs are references to VPCSecurityGroups used to set the VPCSecurityGroupIDs.
`allowMajorVersionUpgrade` | Optional bool | AllowMajorVersionUpgrade indicates that major version upgrades are allowed. Changing this parameter doesn&#39;t result in an outage and the change is asynchronously applied as soon as possible. Constraints: This parameter must be set to true when specifying a value for the EngineVersion parameter that is a different major version than the DB instance&#39;s current version.
`applyModificationsImmediately` | Optional bool | ApplyModificationsImmediately specifies whether the modifications in this request and any pending modifications are asynchronously applied as soon as possible, regardless of the PreferredMaintenanceWindow setting for the DB instance. If this parameter is set to false, changes to the DB instance are applied during the next maintenance window. Some parameter changes can cause an outage and are applied on the next call to RebootDBInstance, or the next failure reboot. Review the table of parameters in Modifying a DB Instance and Using the Apply Immediately Parameter (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Overview.DBInstance.Modifying.html) in the Amazon RDS User Guide. to see the impact that setting ApplyImmediately to true or false has for each modified parameter and to determine when the changes are applied. Default: false
`cloudwatchLogsExportConfiguration` | Optional [CloudwatchLogsExportConfiguration](#CloudwatchLogsExportConfiguration) | CloudwatchLogsExportConfiguration is the configuration setting for the log types to be enabled for export to CloudWatch Logs for a specific DB instance.
`dbParameterGroupName` | Optional string | DBParameterGroupName is the name of the DB parameter group to associate with this DB instance. If this argument is omitted, the default DBParameterGroup for the specified engine is used. Constraints:    * Must be 1 to 255 letters, numbers, or hyphens.    * First character must be a letter    * Cannot end with a hyphen or contain two consecutive hyphens
`domain` | Optional string | Domain specifies the Active Directory Domain to create the instance in.
`domainIAMRoleName` | Optional string | DomainIAMRoleName specifies the name of the IAM role to be used when making API calls to the Directory Service.
`domainIAMRoleNameRef` | Optional [IAMRoleNameReferencerForRDSInstanceDomainRole](#IAMRoleNameReferencerForRDSInstanceDomainRole) | DomainIAMRoleNameRef is a reference to an IAMRole used to set DomainIAMRoleName.
`optionGroupName` | Optional string | OptionGroupName indicates that the DB instance should be associated with the specified option group. Permanent options, such as the TDE option for Oracle Advanced Security TDE, can&#39;t be removed from an option group, and that option group can&#39;t be removed from a DB instance once it is associated with a DB instance
`useDefaultProcessorFeatures` | bool | A value that specifies that the DB instance class of the DB instance uses its default processor features.
`skipFinalSnapshotBeforeDeletion` | bool | Determines whether a final DB snapshot is created before the DB instance is deleted. If true is specified, no DBSnapshot is created. If false is specified, a DB snapshot is created before the DB instance is deleted. Note that when a DB instance is in a failure state and has a status of &#39;failed&#39;, &#39;incompatible-restore&#39;, or &#39;incompatible-network&#39;, it can only be deleted when the SkipFinalSnapshotBeforeDeletion parameter is set to &#34;true&#34;. Specify true when deleting a Read Replica. The FinalDBSnapshotIdentifier parameter must be specified if SkipFinalSnapshotBeforeDeletion is false. Default: false
`finalDBSnapshotIdentifier` | string | The DBSnapshotIdentifier of the new DBSnapshot created when SkipFinalSnapshot is set to false. Specifying this parameter and also setting the SkipFinalShapshot parameter to true results in an error. Constraints:    * Must be 1 to 255 letters or numbers.    * First character must be a letter    * Cannot end with a hyphen or contain two consecutive hyphens    * Cannot be specified when deleting a Read Replica.



## RDSInstanceSpec

An RDSInstanceSpec defines the desired state of an RDSInstance.

Appears in:

* [RDSInstance](#RDSInstance)


Name | Type | Description
-----|------|------------
`forProvider` | [RDSInstanceParameters](#RDSInstanceParameters) | RDSInstanceParameters define the desired state of an AWS Relational Database Service instance.


RDSInstanceSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)


## RDSInstanceState

RDSInstanceState represents the state of an RDS instance. Alias of string.


## RDSInstanceStatus

An RDSInstanceStatus represents the observed state of an RDSInstance.

Appears in:

* [RDSInstance](#RDSInstance)


Name | Type | Description
-----|------|------------
`atProvider` | [RDSInstanceObservation](#RDSInstanceObservation) | RDSInstanceObservation is the representation of the current state that is observed.


RDSInstanceStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## ScalingConfiguration

ScalingConfiguration contains the scaling configuration of an Aurora Serverless DB cluster. For more information, see Using Amazon Aurora Serverless (http://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless.html) in the Amazon Aurora User Guide. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/ScalingConfiguration

Appears in:

* [RDSInstanceParameters](#RDSInstanceParameters)


Name | Type | Description
-----|------|------------
`autoPause` | Optional bool | AutoPause specifies whether to allow or disallow automatic pause for an Aurora DB cluster in serverless DB engine mode. A DB cluster can be paused only when it&#39;s idle (it has no connections). If a DB cluster is paused for more than seven days, the DB cluster might be backed up with a snapshot. In this case, the DB cluster is restored when there is a request to connect to it.
`maxCapacity` | Optional int | MaxCapacity is the maximum capacity for an Aurora DB cluster in serverless DB engine mode. Valid capacity values are 2, 4, 8, 16, 32, 64, 128, and 256. The maximum capacity must be greater than or equal to the minimum capacity.
`minCapacity` | Optional int | MinCapacity is the minimum capacity for an Aurora DB cluster in serverless DB engine mode. Valid capacity values are 2, 4, 8, 16, 32, 64, 128, and 256. The minimum capacity must be less than or equal to the maximum capacity.
`secondsUntilAutoPause` | Optional int | SecondsUntilAutoPause is the time, in seconds, before an Aurora DB cluster in serverless mode is paused.



## SubnetInRDS

SubnetInRDS is used as a response element in the DescribeDBSubnetGroups action. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/Subnet

Appears in:

* [DBSubnetGroupInRDS](#DBSubnetGroupInRDS)


Name | Type | Description
-----|------|------------
`subnetAvailabilityZone` | [AvailabilityZone](#AvailabilityZone) | SubnetAvailabilityZone contains Availability Zone information. This data type is used as an element in the following data type:    * OrderableDBInstanceOption
`subnetIdentifier` | string | SubnetIdentifier specifies the identifier of the subnet.
`subnetStatus` | string | SubnetStatus specifies the status of the subnet.



## Tag

Tag is a metadata assigned to an Amazon RDS resource consisting of a key-value pair. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/Tag

Appears in:

* [RDSInstanceParameters](#RDSInstanceParameters)


Name | Type | Description
-----|------|------------
`key` | string | A key is the required name of the tag. The string value can be from 1 to 128 Unicode characters in length and can&#39;t be prefixed with &#34;aws:&#34; or &#34;rds:&#34;. The string can only contain only the set of Unicode letters, digits, white-space, &#39;_&#39;, &#39;.&#39;, &#39;/&#39;, &#39;=&#39;, &#39;&#43;&#39;, &#39;-&#39; (Java regex: &#34;^([\\p{L}\\p{Z}\\p{N}_.:/=&#43;\\-]*)$&#34;).
`value` | string | A value is the optional value of the tag. The string value can be from 1 to 256 Unicode characters in length and can&#39;t be prefixed with &#34;aws:&#34; or &#34;rds:&#34;. The string can only contain only the set of Unicode letters, digits, white-space, &#39;_&#39;, &#39;.&#39;, &#39;/&#39;, &#39;=&#39;, &#39;&#43;&#39;, &#39;-&#39; (Java regex: &#34;^([\\p{L}\\p{Z}\\p{N}_.:/=&#43;\\-]*)$&#34;).



## VPCSecurityGroupIDReferencerForRDSInstance

VPCSecurityGroupIDReferencerForRDSInstance is an attribute referencer that resolves SecurityGroupID from a referenced SecurityGroup




VPCSecurityGroupIDReferencerForRDSInstance supports all fields of:

* github.com/crossplaneio/stack-aws/apis/network/v1alpha3.SecurityGroupIDReferencer


## VPCSecurityGroupMembership

VPCSecurityGroupMembership is used as a response element for queries on VPC security group membership. Please also see https://docs.aws.amazon.com/goto/WebAPI/rds-2014-10-31/VpcSecurityGroupMembership

Appears in:

* [RDSInstanceObservation](#RDSInstanceObservation)


Name | Type | Description
-----|------|------------
`status` | string | Status is the status of the VPC security group.
`vpcSecurityGroupId` | string | VPCSecurityGroupID is the name of the VPC security group.



This API documentation was generated by `crossdocs`.