# Resource Reclaim Policy
* Owner: Daniel Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Terminology

* **`External Resource`**: an infrastructure resource that runs outside of Crossplane (i.e. an S3 Bucket on AWS).
* **`Managed Resource`**: a Kubernetes resource that is responsible for managing an external resource (receives configuration details from the `ResourceClass` and `ResourceClaim`). It is of type `Resource` but will be referred to as `Managed Resource` consistently here to avoid confusion.
* **`ResourceClass`**: a Kubernetes resource that contains implementation details specific to a certain environment or deployment, and policies related to a kind of resource.
* **`ResourceClaim`**: a Kubernetes resource that captures the desired configuration of a resource from the perspective of a workload or application.

## Background

Crossplane resource classes allow for a `reclaimPolicy` to be set on creation. Acceptable values for `reclaimPolicy` are `Delete` or `Retain`. This value informs Crossplane of how to behave when a `Managed Resource` is deleted. If the policy is set to `Delete`, the `External Resource` will be deleted when the `Managed Resource` is deleted, which is generally triggered by the deletion of a `Resource Claim` for that `Managed Resource`. If set to `Retain`, the `Managed Resource` will be deleted, but the `External Resource` will persist.

## Comparison to Kubernetes Persistent Volumes

Kubernetes introduces the [`PersistentVolume`](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistent-volumes), [`PersistentVolumeClaim`](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims) and [`StorageClass`](https://kubernetes.io/docs/concepts/storage/storage-classes/) resources. These roughly map to the following Crossplane resources:

* The external storage asset --> `External Resource`
* `PersistentVolume` --> `Managed Resource`
* `StorageClass` --> `ResourceClass`
* `PersistentVolumeClaim` --> `ResourceClaim`

Like `Managed Resources` in Crossplane, `PersistentVolumes` can have their `reclaimPolicy` set directly if they are manually provisioned, or by a `StorageClass` if provisioned dynamically. If the `PersistentVolume` is provisioned manually, it will keep its `reclaimPolicy` throughout its lifecycle even if it is eventually managed by a `StorageClass` with a different `reclaimPolicy`.

Dynamic provisioning occurs when an administrator has created a `StorageClass` and a `PersistentVolumeClaim` requests storage by referencing that class or relying on a [default](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class-1) `StorageClass`. This is similar to a `ResourceClaim` referencing a `ResourceClass` in Crossplane. In both situations, the `Managed Resource` or `PersistentVolume` will inherit the `reclaimPolicy` of the `ResourceClass` or `StorageClass`. However, in Kubernetes if no `reclaimPolicy` is set on the `StorageClass` the `PersistentVolume` will default to `Delete`, while in Crossplane a `ResourceClass` without a specified `reclaimPolicy` will cause the `Managed Resource` to default to `Retain`.

The most significant difference between resources in Crossplane and persistent volumes in Kubernetes is what happens upon deletion. In Kubernetes, the `reclaimPolicy` dictates what happens to the `PersistentVolume` when a `PersistentVolumeClaim` is deleted. In Crossplane, the `reclaimPolicy` dictates what happens to the `External Resource` when a `ResourceClaim` is deleted.

For example, consider the following scenarios:

**Kubernetes:** A `PersistentVolume` exists with a `reclaimPolicy` set to `Retain`. A `PersistentVolumeClaim` that was responsible for the dynamic provisioning of that `PersistentVolume` via reference to a `StorageClass` is deleted. The `reclaimPolicy` of the `PersistentVolume` results in the Kubernetes object being retained, and thus the external storage asset being retained.

**Crossplane:** A `Managed Resource` exists with a `reclaimPolicy` set to `Retain`. A `ResourceClaim` that was responsible for the dynamic provisioning of that `Managed Resource` via reference to a `ResourceClass` is deleted. The `reclaimPolicy` of the `Managed Resource` results in the Kubernetes object being *deleted*, but the `External Resource` being retained.

In short, reclaim policies in Crossplane manage the relationship between a `ResourceClaim` and the `External Resource`, while reclaim policies in Kubernetes persistent volumes manage the relationship between the `PersistentVolumeClaim` and the `PersistentVolume` Kubernetes object.

## Workflow

Generally, resources are provisioned via the following steps:

1. A user creates a `ResourceClass`.

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: standard-aws-bucket
  namespace: crossplane-system
  annotations:
    resource: bucket.storage.crossplane.io/v1alpha1
parameters:
  versioning: "false"
  cannedACL: private
  localPermission: ReadWrite
  region: REGION
provisioner: s3bucket.storage.aws.crossplane.io/v1alpha1
providerRef:
  name: demo-aws
reclaimPolicy: Delete
```

2. A user creates a `ResourceClaim`.

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: Bucket
metadata:
  name: my-bucket
  namespace: default
spec:
  classRef:
    name: standard-aws-bucket
    namespace: crossplane-system
  name: my-bucket-1234
```

3. The creation of a `ResourceClaim` triggers a `Managed Resource` to be created using information from the claim and the referenced `ResourceClass`. If the `ResourceClass` provides a value for `reclaimPolicy` it will be set on the `Managed Resource`. If not, the `Managed Resource` will have its `reclaimPolicy` set to its default value.
4. The `Managed Resource` provisions an `External Resource` (in this case an S3 bucket) and manages it.
5. A user deletes the `ResourceClaim`. Because the `Managed Resource` has an `OwnerReference` to the `ResourceClaim`, the deletion of the `ResourceClaim` triggers the deletion of the `Managed Resource`.
6. If the `Managed Resource` `reclaimPolicy` is set to `Retain`, the `Managed Resource` will be deleted, but the `External Resource` will persist (i.e. the S3 bucket will still exist in your AWS account). If the `reclaimPolicy` is set to `Delete` both the `Managed Resource` and the `External Resource` will be deleted.

## Goals

Reclaim policies exist in Crossplane in order to provide the ability for an `External Resource` to persist outside of the lifecycle of a `ResourceClaim` to which they are bound. However, the current status of reclaim policies is flawed in that it allows for the possibility that an `External Resource` that was created by Crossplane persists after the deletion of the `Managed Resource` that represents it. This leads to an inaccurate state representation for a system that should be able to serve as the single control plane across cloud providers. Reclaim policies should more closely represent the functionality implemented in Kubernetes Persistent Volumes by tightly coupling the lifecyle of `Managed Resources` with their corresponding `External Resource`. Motivation for this change is provided by the following core concepts:

* Crossplane should always provide an accurate representation of resource state (including that of external resources)
* `External Resources` created by Crossplane should always have a `Managed Resource` representation unless the `Managed Resource` is manually removed by an administrator (i.e. no orphaned `External Resources`)
* A tightly coupled `External Resource` / `Managed Resource` pair should have the option to persist after the deletion of any `Resource Claim` with which they are bound (i.e. a `reclaimPolicy` set to `Retain`)

A move to change reclaim policies in Crossplane to match that of Kubernetes Persistent Volumes would result in the policies dictating the relationship between `ResourceClaims` and `Managed Resources` instead of the current status of `ResourceClaims` and `External Resources`. The only tradeoff in functionality with this shift is that unbound `Managed Resources` may continue to exist after deletion of a the `ResourceClaim` that they reference if their `reclaimPolicy` is set to `Retain`. This is opposed to the current system where the `Managed Resource` is always deleted upon `ResourceClaim` deletion, despite its `reclaimPolicy`. However, the persistence of the `Managed Resources` actually serves as a feature as they are a reminder that an unbound `External Resource` (which may be billing you) exists as well. To clean up the retained unbound `Managed Resource` and `External Resource`, an administrator will manually delete the `Managed Resource` from Crossplane, then take action on the `External Resource`.