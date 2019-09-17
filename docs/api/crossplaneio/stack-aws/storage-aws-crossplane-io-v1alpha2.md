# storage.aws.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for AWS storage services such as S3.

This API group contains the following Crossplane resources:

* [DBSubnetGroup](#DBSubnetGroup)
* [S3Bucket](#S3Bucket)
* [S3BucketClass](#S3BucketClass)

## DBSubnetGroup

A DBSubnetGroup is a managed resource that represents an AWS VPC Database Subnet Group.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.aws.crossplane.io/v1alpha2`
`kind` | string | `DBSubnetGroup`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [DBSubnetGroupSpec](#DBSubnetGroupSpec) | A DBSubnetGroupSpec defines the desired state of a DBSubnetGroup.
`status` | [DBSubnetGroupStatus](#DBSubnetGroupStatus) | A DBSubnetGroupStatus represents the observed state of a DBSubnetGroup.



## S3Bucket

An S3Bucket is a managed resource that represents an AWS S3 Bucket.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.aws.crossplane.io/v1alpha2`
`kind` | string | `S3Bucket`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [S3BucketSpec](#S3BucketSpec) | S3BucketSpec defines the desired state of S3Bucket
`status` | [S3BucketStatus](#S3BucketStatus) | S3BucketStatus defines the observed state of S3Bucket



## S3BucketClass

An S3BucketClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.aws.crossplane.io/v1alpha2`
`kind` | string | `S3BucketClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [S3BucketClassSpecTemplate](#S3BucketClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned S3Bucket.



## DBSubnetGroupExternalStatus

DBSubnetGroupExternalStatus keeps the state for the external resource

Appears in:

* [DBSubnetGroupStatus](#DBSubnetGroupStatus)


Name | Type | Description
-----|------|------------
`groupArn` | string | The Amazon Resource Name (ARN) for the DB subnet group.
`groupStatus` | string | Provides the status of the DB subnet group.
`subnets` | [[]Subnet](#Subnet) | Contains a list of Subnet elements.
`vpcId` | string | Provides the VpcId of the DB subnet group.



## DBSubnetGroupParameters

DBSubnetGroupParameters define the desired state of an AWS VPC Database Subnet Group.

Appears in:

* [DBSubnetGroupSpec](#DBSubnetGroupSpec)


Name | Type | Description
-----|------|------------
`description` | string | The description for the DB subnet group.
`groupName` | string | The name for the DB subnet group. This value is stored as a lowercase string.
`subnetIds` | []string | The EC2 Subnet IDs for the DB subnet group.
`tags` | [[]Tag](#Tag) | A list of tags. For more information, see Tagging Amazon RDS Resources (http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Tagging.html) in the Amazon RDS User Guide.



## DBSubnetGroupSpec

A DBSubnetGroupSpec defines the desired state of a DBSubnetGroup.

Appears in:

* [DBSubnetGroup](#DBSubnetGroup)




DBSubnetGroupSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [DBSubnetGroupParameters](#DBSubnetGroupParameters)


## DBSubnetGroupStatus

A DBSubnetGroupStatus represents the observed state of a DBSubnetGroup.

Appears in:

* [DBSubnetGroup](#DBSubnetGroup)




DBSubnetGroupStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)
* [DBSubnetGroupExternalStatus](#DBSubnetGroupExternalStatus)


## S3BucketClassSpecTemplate

An S3BucketClassSpecTemplate is a template for the spec of a dynamically provisioned S3Bucket.

Appears in:

* [S3BucketClass](#S3BucketClass)




S3BucketClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [S3BucketParameters](#S3BucketParameters)


## S3BucketParameters

S3BucketParameters define the desired state of an AWS S3 Bucket.

Appears in:

* [S3BucketClassSpecTemplate](#S3BucketClassSpecTemplate)
* [S3BucketSpec](#S3BucketSpec)


Name | Type | Description
-----|------|------------
`nameFormat` | Optional string | NameFormat specifies the name of the external S3Bucket instance. The first instance of the string &#39;%s&#39; will be replaced with the Kubernetes UID of this S3Bucket. Omit this field to use the UID alone as the name.
`region` | string | Region of the bucket.
`cannedACL` | Optional [s3.BucketCannedACL](https://godoc.org/github.com/aws/aws-sdk-go-v2/service/s3#BucketCannedACL) | CannedACL applies a standard AWS built-in ACL for common bucket use cases.
`versioning` | Optional bool | Versioning enables versioning of objects stored in this bucket.
`localPermission` | [storage/v1alpha1.LocalPermissionType](../crossplane/storage-crossplane-io-v1alpha1.md#localpermissiontype) | LocalPermission is the permissions granted on the bucket for the provider specific bucket service account that is available in a secret after provisioning.



## S3BucketSpec

S3BucketSpec defines the desired state of S3Bucket

Appears in:

* [S3Bucket](#S3Bucket)




S3BucketSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [S3BucketParameters](#S3BucketParameters)


## S3BucketStatus

S3BucketStatus defines the observed state of S3Bucket

Appears in:

* [S3Bucket](#S3Bucket)


Name | Type | Description
-----|------|------------
`providerID` | string | ProviderID is the AWS identifier for this bucket.
`iamUsername` | string | IAMUsername is the name of an IAM user that is automatically created and granted access to this bucket by Crossplane at bucket creation time.
`lastUserPolicyVersion` | int | LastUserPolicyVersion is the most recent version of the policy associated with this bucket&#39;s IAMUser.
`lastLocalPermission` | [storage/v1alpha1.LocalPermissionType](../crossplane/storage-crossplane-io-v1alpha1.md#localpermissiontype) | LastLocalPermission is the most recent local permission that was set for this bucket.


S3BucketStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## Subnet

Subnet represents a aws subnet

Appears in:

* [DBSubnetGroupExternalStatus](#DBSubnetGroupExternalStatus)


Name | Type | Description
-----|------|------------
`subnetID` | string | Specifies the identifier of the subnet.
`subnetStatus` | string | Specifies the status of the subnet.



## Tag

Tag defines a tag

Appears in:

* [DBSubnetGroupParameters](#DBSubnetGroupParameters)


Name | Type | Description
-----|------|------------
`key` | string | Key is the name of the tag.
`value` | string | Value is the value of the tag.



This API documentation was generated by `crossdocs`.