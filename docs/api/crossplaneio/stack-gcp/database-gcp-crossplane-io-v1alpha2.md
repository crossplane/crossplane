# database.gcp.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for GCP database services such as CloudSQL.

This API group contains the following Crossplane resources:

* [CloudsqlInstance](#CloudsqlInstance)
* [CloudsqlInstanceClass](#CloudsqlInstanceClass)

## CloudsqlInstance

A CloudsqlInstance is a managed resource that represents a Google CloudSQL instance.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.gcp.crossplane.io/v1alpha2`
`kind` | string | `CloudsqlInstance`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [CloudsqlInstanceSpec](#CloudsqlInstanceSpec) | A CloudsqlInstanceSpec defines the desired state of a CloudsqlInstance.
`status` | [CloudsqlInstanceStatus](#CloudsqlInstanceStatus) | A CloudsqlInstanceStatus represents the observed state of a CloudsqlInstance.



## CloudsqlInstanceClass

A CloudsqlInstanceClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `database.gcp.crossplane.io/v1alpha2`
`kind` | string | `CloudsqlInstanceClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [CloudsqlInstanceClassSpecTemplate](#CloudsqlInstanceClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned CloudsqlInstance.



## CloudsqlInstanceClassSpecTemplate

A CloudsqlInstanceClassSpecTemplate is a template for the spec of a dynamically provisioned CloudsqlInstance.

Appears in:

* [CloudsqlInstanceClass](#CloudsqlInstanceClass)




CloudsqlInstanceClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [CloudsqlInstanceParameters](#CloudsqlInstanceParameters)


## CloudsqlInstanceParameters

CloudsqlInstanceParameters define the desired state of a Google CloudSQL instance.

Appears in:

* [CloudsqlInstanceClassSpecTemplate](#CloudsqlInstanceClassSpecTemplate)
* [CloudsqlInstanceSpec](#CloudsqlInstanceSpec)


Name | Type | Description
-----|------|------------
`authorizedNetworks` | Optional []string | AuthorizedNetworks is the list of external networks that are allowed to connect to the instance using the IP. In CIDR notation, also known as &#39;slash&#39; notation (e.g. 192.168.100.0/24).
`privateNetwork` | Optional string | PrivateNetwork is the resource link for the VPC network from which the Cloud SQL instance is accessible for private IP. For example, /projects/myProject/global/networks/default. This setting can be updated, but it cannot be removed after it is set.
`ipv4Enabled` | Optional bool | Ipv4Enabled specifies whether the instance should be assigned an IP address or not.
`databaseVersion` | string | DatabaseVersion specifies he database engine type and version. MySQL Second Generation instances use MYSQL_5_7 (default) or MYSQL_5_6. MySQL First Generation instances use MYSQL_5_6 (default) or MYSQL_5_5 PostgreSQL instances uses POSTGRES_9_6 (default) or POSTGRES_11.
`labels` | Optional map[string]string | Labels to apply to this CloudSQL instance.
`region` | string | Region specifies the geographical region of this CloudSQL instance.
`storageType` | string | StorageType specifies the type of the data disk, either PD_SSD or PD_HDD.
`storageGB` | int64 | StorageGB specifies the size of the data disk. The minimum is 10GB.
`tier` | string | Tier (or machine type) for this instance, for example db-n1-standard-1 (MySQL instances) or db-custom-1-3840 (PostgreSQL instances). For MySQL instances, this property determines whether the instance is First or Second Generation. For more information, see https://cloud.google.com/sql/docs/mysql/instance-settings
`nameFormat` | string | NameFormat specifies the name of the extenral CloudSQL instance. The first instance of the string &#39;%s&#39; will be replaced with the Kubernetes UID of this CloudsqlInstance.



## CloudsqlInstanceSpec

A CloudsqlInstanceSpec defines the desired state of a CloudsqlInstance.

Appears in:

* [CloudsqlInstance](#CloudsqlInstance)




CloudsqlInstanceSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [CloudsqlInstanceParameters](#CloudsqlInstanceParameters)


## CloudsqlInstanceStatus

A CloudsqlInstanceStatus represents the observed state of a CloudsqlInstance.

Appears in:

* [CloudsqlInstance](#CloudsqlInstance)


Name | Type | Description
-----|------|------------
`state` | string | State of this CloudsqlInstance.
`publicIp` | string | PublicIP is used to connect to this resource from other authorized networks.
`privateIp` | string | PrivateIP is used to connect to this instance from the same Network.


CloudsqlInstanceStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


This API documentation was generated by `crossdocs`.