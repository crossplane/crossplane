# Resource Reclaim Policy
* Owner: Daniel Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Terminology

* **`External Resource`**: an infrastructure resource that runs outside of Crossplane (i.e. an S3 Bucket on AWS).
* **`Concrete Resource`**: a Kubernetes resource that is responsible for managing an external resource (recieves configuration details from the `ResourceClass` and `ResourceClaim`). It is of type `Resource` but will be referred to as `Concrete Resource` consistently here to avoid confusion.
* **`ResourceClass`**: a Kubernetes resource that contains implementation details specific to a certain environment or deployment, and policies related to a kind of resource.
* **`ResourceClaim`**: a Kubernetes resource that captures the desired configuration of a resource from the perspective of a workload or application.

## Background

Crossplane resource classes allow for a `reclaimPolicy` to be set on creation. Acceptable values for `reclaimPolicy` are `Delete` or `Retain`. This value informs Crossplane of how to behave when a `Concrete Resource` is deleted. If the policy is set to `Delete`, the `External Resource` will be deleted when the `Concrete Resource` is deleted, which is generally triggered by the deletetion of a `Resource Claim` for that `Concrete Resource`. If set to `Retain`, the `Concrete Resource` will be deleted, but the `External Resource` will persist.

*Note: `reclaimPolicy` is not a required field for a `ResourceClass`. If it is not supplied, the `Concrete Resource` will be created with it's default `reclaimPolicy`. Currenty, every `Concrete Resource` in Crossplane defaults to `Retain`.*

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
  classReference:
    name: standard-aws-bucket
    namespace: crossplane-system
  name: my-bucket-1234
```

3. The creation of a `ResourceClaim` triggers a `Concrete Resource` to be created using information from the claim and the referenced `ResourceClass`. If the `ResourceClass` provides a value for `reclaimPolicy` it will be set on the `Concrete Resource`. If not, the `Concrete Resource` will have its `reclaimPolicy` set to its default value.
4. The `Concrete Resource` provisions an `External Resource` (in this case an S3 bucket) and manages it.
5. A user deletes the `ResourceClaim`. Because the `ConcreteResource` has an `OwnerReference` to the `ResourceClaim`, the deletion of the `ResourceClaim` triggers the deletion of the `ConcreteResource`.
6. If the `ConcreteResource` `reclaimPolicy` is set to `Retain`, the `Concrete Resource` will be deleted, but the `External Resource` will persist (i.e. the S3 bucket will still exist in your AWS account). If the `reclaimPolicy` is set to `Delete` both the `Concrete Resource` and the `External Resource` will be deleted.

