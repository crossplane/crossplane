# storage.gcp.crossplane.io/v1alpha2 API Reference

Package v1alpha2 contains managed resources for GCP storage services such as GCS buckets.

This API group contains the following Crossplane resources:

* [Bucket](#Bucket)
* [BucketClass](#BucketClass)

## Bucket

A Bucket is a managed resource that represents a Google Cloud Storage bucket.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.gcp.crossplane.io/v1alpha2`
`kind` | string | `Bucket`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`spec` | [BucketSpec](#BucketSpec) | A BucketSpec defines the desired state of a Bucket.
`status` | [BucketStatus](#BucketStatus) | A BucketStatus represents the observed state of a Bucket.



## BucketClass

A BucketClass is a non-portable resource class. It defines the desired spec of resource claims that use it to dynamically provision a managed resource.


Name | Type | Description
-----|------|------------
`apiVersion` | string | `storage.gcp.crossplane.io/v1alpha2`
`kind` | string | `BucketClass`
`metadata` | [meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) | Kubernetes object metadata.
`specTemplate` | [BucketClassSpecTemplate](#BucketClassSpecTemplate) | SpecTemplate is a template for the spec of a dynamically provisioned Bucket.



## ACLRule

ACLRule represents a grant for a role to an entity (user, group or team) for a Google Cloud Storage object or bucket.

Appears in:

* [BucketSpecAttrs](#BucketSpecAttrs)


Name | Type | Description
-----|------|------------
`entity` | string | 
`entityId` | string | 
`role` | string | 
`domain` | string | 
`email` | string | 
`projectTeam` | [ProjectTeam](#ProjectTeam) | 



## BucketClassSpecTemplate

A BucketClassSpecTemplate is a template for the spec of a dynamically provisioned Bucket.

Appears in:

* [BucketClass](#BucketClass)




BucketClassSpecTemplate supports all fields of:

* [v1alpha1.NonPortableClassSpecTemplate](../crossplane-runtime/core-crossplane-io-v1alpha1.md#nonportableclassspectemplate)
* [BucketParameters](#BucketParameters)


## BucketEncryption

BucketEncryption is a bucket&#39;s encryption configuration.

Appears in:

* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


Name | Type | Description
-----|------|------------
`defaultKmsKeyName` | string | A Cloud KMS key name, in the form projects/P/locations/L/keyRings/R/cryptoKeys/K, that will be used to encrypt objects inserted into this bucket, if no encryption method is specified. The key&#39;s location must be the same as the bucket&#39;s.



## BucketLogging

BucketLogging holds the bucket&#39;s logging configuration, which defines the destination bucket and optional name prefix for the current bucket&#39;s logs.

Appears in:

* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


Name | Type | Description
-----|------|------------
`logBucket` | string | The destination bucket where the current bucket&#39;s logs should be placed.
`logObjectPrefix` | string | A prefix for log object names.



## BucketOutputAttrs

BucketOutputAttrs represent the subset of metadata for a Google Cloud Storage bucket limited to output (read-only) fields.

Appears in:

* [BucketStatus](#BucketStatus)


Name | Type | Description
-----|------|------------
`bucketPolicyOnly` | [BucketPolicyOnly](#BucketPolicyOnly) | BucketPolicyOnly configures access checks to use only bucket-level IAM policies.
`created` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | Created is the creation time of the bucket.
`name` | string | Name is the name of the bucket.
`retentionPolicy` | [RetentionPolicyStatus](#RetentionPolicyStatus) | Retention policy enforces a minimum retention time for all objects contained in the bucket. A RetentionPolicy of nil implies the bucket has no minimum data retention.  This feature is in private alpha release. It is not currently available to most customers. It might be changed in backwards-incompatible ways and is not subject to any SLA or deprecation policy.



## BucketParameters

BucketParameters define the desired state of a Google Cloud Storage Bucket. Most fields map directly to a bucket resource: https://cloud.google.com/storage/docs/json_api/v1/buckets#resource

Appears in:

* [BucketClassSpecTemplate](#BucketClassSpecTemplate)
* [BucketSpec](#BucketSpec)


Name | Type | Description
-----|------|------------
`nameFormat` | string | NameFormat specifies the name of the external Bucket. The first instance of the string &#39;%s&#39; will be replaced with the Kubernetes UID of this Bucket.
`serviceAccountSecretRef` | [core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core) | ServiceAccountSecretRef contains GCP ServiceAccount secret that will be used for bucket connection secret credentials


BucketParameters supports all fields of:

* [BucketSpecAttrs](#BucketSpecAttrs)


## BucketPolicyOnly

BucketPolicyOnly configures access checks to use only bucket-level IAM policies.

Appears in:

* [BucketOutputAttrs](#BucketOutputAttrs)
* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


Name | Type | Description
-----|------|------------
`enabled` | bool | Enabled specifies whether access checks use only bucket-level IAM policies. Enabled may be disabled until the locked time.
`lockedTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | LockedTime specifies the deadline for changing Enabled from true to false.



## BucketSpec

A BucketSpec defines the desired state of a Bucket.

Appears in:

* [Bucket](#Bucket)




BucketSpec supports all fields of:

* [v1alpha1.ResourceSpec](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcespec)
* [BucketParameters](#BucketParameters)


## BucketSpecAttrs

BucketSpecAttrs represents the full set of metadata for a Google Cloud Storage bucket limited to all input attributes

Appears in:

* [BucketParameters](#BucketParameters)


Name | Type | Description
-----|------|------------
`acl` | [[]ACLRule](#ACLRule) | ACL is the list of access control rules on the bucket.
`defaultObjectAcl` | [[]ACLRule](#ACLRule) | DefaultObjectACL is the list of access controls to apply to new objects when no object ACL is provided.
`location` | string | Location is the location of the bucket. It defaults to &#34;US&#34;.
`storageClass` | string | StorageClass is the default storage class of the bucket. This defines how objects in the bucket are stored and determines the SLA and the cost of storage. Typical values are &#34;MULTI_REGIONAL&#34;, &#34;REGIONAL&#34;, &#34;NEARLINE&#34;, &#34;COLDLINE&#34;, &#34;STANDARD&#34; and &#34;DURABLE_REDUCED_AVAILABILITY&#34;. Defaults to &#34;STANDARD&#34;, which is equivalent to &#34;MULTI_REGIONAL&#34; or &#34;REGIONAL&#34; depending on the bucket&#39;s location settings.


BucketSpecAttrs supports all fields of:

* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


## BucketStatus

A BucketStatus represents the observed state of a Bucket.

Appears in:

* [Bucket](#Bucket)


Name | Type | Description
-----|------|------------
`attributes` | [BucketOutputAttrs](#BucketOutputAttrs) | BucketOutputAttrs represent the subset of metadata for a Google Cloud Storage bucket limited to output (read-only) fields.


BucketStatus supports all fields of:

* [v1alpha1.ResourceStatus](../crossplane-runtime/core-crossplane-io-v1alpha1.md#resourcestatus)


## BucketUpdatableAttrs

BucketUpdatableAttrs represents the subset of parameters of a Google Cloud Storage bucket that may be updated.

Appears in:

* [BucketSpecAttrs](#BucketSpecAttrs)


Name | Type | Description
-----|------|------------
`bucketPolicyOnly` | [BucketPolicyOnly](#BucketPolicyOnly) | BucketPolicyOnly configures access checks to use only bucket-level IAM policies.
`cors` | [[]CORS](#CORS) | The bucket&#39;s Cross-Origin Resource Sharing (CORS) configuration.
`defaultEventBasedHold` | bool | DefaultEventBasedHold is the default value for event-based hold on newly created objects in this bucket. It defaults to false.
`encryption` | [BucketEncryption](#BucketEncryption) | The encryption configuration used by default for newly inserted objects.
`labels` | map[string]string | Labels are the bucket&#39;s labels.
`lifecycle` | [Lifecycle](#Lifecycle) | Lifecycle is the lifecycle configuration for objects in the bucket.
`logging` | [BucketLogging](#BucketLogging) | The logging configuration.
`predefinedAcl` | string | If not empty, applies a predefined set of access controls. It should be set only when creating a bucket. It is always empty for BucketAttrs returned from the service. See https://cloud.google.com/storage/docs/json_api/v1/buckets/insert for valid values.
`predefinedCefaultObjectAcl` | string | If not empty, applies a predefined set of default object access controls. It should be set only when creating a bucket. It is always empty for BucketAttrs returned from the service. See https://cloud.google.com/storage/docs/json_api/v1/buckets/insert for valid values.
`requesterPays` | bool | RequesterPays reports whether the bucket is a Requester Pays bucket. Clients performing operations on Requester Pays buckets must provide a user project (see BucketHandle.UserProject), which will be billed for the operations.
`retentionPolicy` | [RetentionPolicy](#RetentionPolicy) | Retention policy enforces a minimum retention time for all objects contained in the bucket. A RetentionPolicy of nil implies the bucket has no minimum data retention.  This feature is in private alpha release. It is not currently available to most customers. It might be changed in backwards-incompatible ways and is not subject to any SLA or deprecation policy.
`versioningEnabled` | bool | VersioningEnabled reports whether this bucket has versioning enabled.
`website` | [BucketWebsite](#BucketWebsite) | The website configuration.



## BucketWebsite

BucketWebsite holds the bucket&#39;s website configuration, controlling how the service behaves when accessing bucket contents as a web site. See https://cloud.google.com/storage/docs/static-website for more information.

Appears in:

* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


Name | Type | Description
-----|------|------------
`mainPageSuffix` | string | If the requested object path is missing, the service will ensure the path has a trailing &#39;/&#39;, append this suffix, and attempt to retrieve the resulting object. This allows the creation of index.html objects to represent directory pages.
`notFundPage` | string | If the requested object path is missing, and any mainPageSuffix object is missing, if applicable, the service will return the named object from this bucket as the content for a 404 Not Found result.



## CORS

CORS is the bucket&#39;s Cross-Origin Resource Sharing (CORS) configuration.

Appears in:

* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


Name | Type | Description
-----|------|------------
`maxAge` | [meta/v1.Duration](https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration) | MaxAge is the value to return in the Access-Control-Max-Age header used in preflight responses.
`methods` | []string | Methods is the list of HTTP methods on which to include CORS response headers, (GET, OPTIONS, POST, etc) Note: &#34;*&#34; is permitted in the list of methods, and means &#34;any method&#34;.
`origins` | []string | Origins is the list of Origins eligible to receive CORS response headers. Note: &#34;*&#34; is permitted in the list of origins, and means &#34;any Origin&#34;.
`responseHeaders` | []string | ResponseHeaders is the list of HTTP headers other than the simple response headers to give permission for the user-agent to share across domains.



## Lifecycle

Lifecycle is the lifecycle configuration for objects in the bucket.

Appears in:

* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


Name | Type | Description
-----|------|------------
`rules` | [[]LifecycleRule](#LifecycleRule) | 



## LifecycleAction

LifecycleAction is a lifecycle configuration action.

Appears in:

* [LifecycleRule](#LifecycleRule)


Name | Type | Description
-----|------|------------
`storageClass` | string | StorageClass is the storage class to set on matching objects if the Action is &#34;SetStorageClass&#34;.
`type` | string | Type is the type of action to take on matching objects.  Acceptable values are &#34;Delete&#34; to delete matching objects and &#34;SetStorageClass&#34; to set the storage class defined in StorageClass on matching objects.



## LifecycleCondition

LifecycleCondition is a set of conditions used to match objects and take an action automatically. All configured conditions must be met for the associated action to be taken.

Appears in:

* [LifecycleRule](#LifecycleRule)


Name | Type | Description
-----|------|------------
`ageInDays` | int64 | AgeInDays is the age of the object in days.
`createdBefore` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | CreatedBefore is the time the object was created.  This condition is satisfied when an object is created before midnight of the specified date in UTC.
`liveness` | [storage.Liveness](https://godoc.org/cloud.google.com/go/storage#Liveness) | Liveness specifies the object&#39;s liveness. Relevant only for versioned objects
`matchesStorageClasses` | []string | MatchesStorageClasses is the condition matching the object&#39;s storage class.  Values include &#34;MULTI_REGIONAL&#34;, &#34;REGIONAL&#34;, &#34;NEARLINE&#34;, &#34;COLDLINE&#34;, &#34;STANDARD&#34;, and &#34;DURABLE_REDUCED_AVAILABILITY&#34;.
`numNewerVersions` | int64 | NumNewerVersions is the condition matching objects with a number of newer versions.  If the value is N, this condition is satisfied when there are at least N versions (including the live version) newer than this version of the object.



## LifecycleRule

LifecycleRule is a lifecycle configuration rule.  When all the configured conditions are met by an object in the bucket, the configured action will automatically be taken on that object.

Appears in:

* [Lifecycle](#Lifecycle)


Name | Type | Description
-----|------|------------
`action` | [LifecycleAction](#LifecycleAction) | Action is the action to take when all of the associated conditions are met.
`condition` | [LifecycleCondition](#LifecycleCondition) | Condition is the set of conditions that must be met for the associated action to be taken.



## ProjectTeam

ProjectTeam is the project team associated with the entity, if any.

Appears in:

* [ACLRule](#ACLRule)


Name | Type | Description
-----|------|------------
`projectNumber` | string | 
`team` | string | 



## RetentionPolicy

RetentionPolicy enforces a minimum retention time for all objects contained in the bucket.  Any attempt to overwrite or delete objects younger than the retention period will result in an error. An unlocked retention policy can be modified or removed from the bucket via the Update method. A locked retention policy cannot be removed or shortened in duration for the lifetime of the bucket.  This feature is in private alpha release. It is not currently available to most customers. It might be changed in backwards-incompatible ways and is not subject to any SLA or deprecation policy.

Appears in:

* [BucketUpdatableAttrs](#BucketUpdatableAttrs)


Name | Type | Description
-----|------|------------
`retentionPeriodSeconds` | int | RetentionPeriod specifies the duration value in seconds that objects need to be retained. Retention duration must be greater than zero and less than 100 years. Note that enforcement of retention periods less than a day is not guaranteed. Such periods should only be used for testing purposes.



## RetentionPolicyStatus

RetentionPolicyStatus output component of storage.RetentionPolicy

Appears in:

* [BucketOutputAttrs](#BucketOutputAttrs)


Name | Type | Description
-----|------|------------
`effectiveTime` | [meta/v1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta) | EffectiveTime is the time from which the policy was enforced and effective.
`isLocked` | bool | IsLocked describes whether the bucket is locked. Once locked, an object retention policy cannot be modified.



This API documentation was generated by `crossdocs`.