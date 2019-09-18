---
title: API Reference
toc: true
weight: 610
---
# API Reference

Crossplane is configured using a variety of Kubernetes resources. These
resources are separated into API groups. Core API groups are provided by
Crossplane, while cloud provider specific API groups are provided by each cloud
provider's stack.

Each API Group supported by Crossplane is assigned its own individual status to
reflect the varying maturity and stability. More details about API versioning
and status in Kubernetes can be found on the Kubernetes [API versioning] page,
but the key difference between the statuses are summarized below:

* **Alpha:** The API may change in incompatible ways in a later software release
  without notice, recommended for use only in short-lived testing clusters, due
  to increased risk of bugs and lack of long-term support.
* **Beta:** Support for the overall features will not be dropped, though details
  may change. Support for upgrading or migrating between versions will be
  provided, either through automation or manual steps.
* **Stable:** Features will appear in released software for many subsequent
  versions and support for upgrading between versions will be provided with
  software automation in the vast majority of scenarios.

## Crossplane

Crossplane is responsible for all portable resources, including resource claims
(for example `MySQLInstance`) and portable resource classes (for example
`MySQLInstanceClass). Crossplane currently supports the following API groups:

API Group | Status | Details
--------  | ------ | -------
[`cache.crossplane.io/v1alpha1`] | Alpha | Cache service claims and classes.
[`compute.crossplane.io/v1alpha1`] | Alpha | Compute service resource claims and classes.
[`database.crossplane.io/v1alpha1`] | Alpha | Database service resource claims and classes.
[`storage.crossplane.io/v1alpha1`] | Alpha | Blob storage service resource claims and classes.
[`stacks.crossplane.io/v1alpha1`] | Alpha | Resources for managing Crossplane Stacks.
[`workload.crossplane.io/v1alpha1`] | Alpha | Resources for managing Crossplane Workloads.

[`cache.crossplane.io/v1alpha1`]: api/crossplaneio/crossplane/cache-crossplane-io-v1alpha1.md
[`compute.crossplane.io/v1alpha1`]: api/crossplaneio/crossplane/compute-crossplane-io-v1alpha1.md
[`database.crossplane.io/v1alpha1`]: api/crossplaneio/crossplane/database-crossplane-io-v1alpha1.md
[`storage.crossplane.io/v1alpha1`]: api/crossplaneio/crossplane/storage-crossplane-io-v1alpha1.md
[`stacks.crossplane.io/v1alpha1`]: api/crossplaneio/crossplane/stacks-crossplane-io-v1alpha1.md
[`workload.crossplane.io/v1alpha1`]: api/crossplaneio/crossplane/workload-crossplane-io-v1alpha1.md

## AWS Stack

The AWS Stack is responsible for all AWS specific resources, including managed
resources (for example `EKSCluster`), and cloud specific resource classes (for
example `EKSClusterClass`) The AWS Stack currently supports the following API
groups:

API Group | Status | Details
--------  | ------ | -------
[`aws.crossplane.io/v1alpha2`] | Alpha | Core AWS resources.
[`cache.aws.crossplane.io/v1alpha2`] | Alpha | ElastiCache managed resources and classes.
[`compute.aws.crossplane.io/v1alpha2`] | Alpha | EKS managed resources and classes.
[`database.aws.crossplane.io/v1alpha2`] | Alpha | RDS managed resources and classes.
[`identity.aws.crossplane.io/v1alpha2`] | Alpha | IAM managed resources and classes.
[`network.aws.crossplane.io/v1alpha2`] | Alpha | VPC managed resources and classes.
[`storage.aws.crossplane.io/v1alpha2`] | Alpha | S3 managed resources and classes.

[`aws.crossplane.io/v1alpha2`]: api/crossplaneio/stack-aws/aws-crossplane-io-v1alpha2.md
[`cache.aws.crossplane.io/v1alpha2`]: api/crossplaneio/stack-aws/cache-aws-crossplane-io-v1alpha2.md
[`compute.aws.crossplane.io/v1alpha2`]: api/crossplaneio/stack-aws/compute-aws-crossplane-io-v1alpha2.md
[`database.aws.crossplane.io/v1alpha2`]: api/crossplaneio/stack-aws/database-aws-crossplane-io-v1alpha2.md
[`identity.aws.crossplane.io/v1alpha2`]: api/crossplaneio/stack-aws/identity-aws-crossplane-io-v1alpha2.md
[`network.aws.crossplane.io/v1alpha2`]: api/crossplaneio/stack-aws/network-aws-crossplane-io-v1alpha2.md
[`storage.aws.crossplane.io/v1alpha2`]: api/crossplaneio/stack-aws/storage-aws-crossplane-io-v1alpha2.md

## Azure Stack

The Azure Stack is responsible for all Azure specific resources, including
managed resources (for example `AKSCluster`), and cloud specific resource
classes (for example `AKSClusterClass`) The Azure Stack currently supports the
following API groups:

API Group | Status | Details
--------  | ------ | -------
[`azure.crossplane.io/v1alpha2`] | Alpha | Core Azure resources.
[`cache.azure.crossplane.io/v1alpha2`] | Alpha | Azure Redis managed resources and classes.
[`compute.azure.crossplane.io/v1alpha2`] | Alpha | AKS managed resources and classes.
[`database.azure.crossplane.io/v1alpha2`] | Alpha | Azure Database managed resources and classes.
[`network.azure.crossplane.io/v1alpha2`] | Alpha | Virtual network managed resources and classes.
[`storage.azure.crossplane.io/v1alpha2`] | Alpha | Azure Blob Storage managed resources and classes.

[`azure.crossplane.io/v1alpha2`]: api/crossplaneio/stack-azure/azure-crossplane-io-v1alpha2.md
[`cache.azure.crossplane.io/v1alpha2`]: api/crossplaneio/stack-azure/cache-azure-crossplane-io-v1alpha2.md
[`compute.azure.crossplane.io/v1alpha2`]: api/crossplaneio/stack-azure/compute-azure-crossplane-io-v1alpha2.md
[`database.azure.crossplane.io/v1alpha2`]: api/crossplaneio/stack-azure/database-azure-crossplane-io-v1alpha2.md
[`network.azure.crossplane.io/v1alpha2`]: api/crossplaneio/stack-azure/network-azure-crossplane-io-v1alpha2.md
[`storage.azure.crossplane.io/v1alpha2`]: api/crossplaneio/stack-azure/storage-azure-crossplane-io-v1alpha2.md

## GCP Stack

The GCP Stack is responsible for all GCP specific resources, including managed
resources (for example `GKECluster`), and cloud specific resource classes (for
example `GKEClusterClass`) The GCP Stack currently supports the following API
groups:

API Group | Status | Details
--------  | ------ | -------
[`gcp.crossplane.io/v1alpha2`] | Alpha | Core GCP resources.
[`cache.gcp.crossplane.io/v1alpha2`] | Alpha | CloudMemorystore managed resources and classes.
[`compute.gcp.crossplane.io/v1alpha2`] | Alpha | Compute and Kubernetes Engine managed resources and classes.
[`database.gcp.crossplane.io/v1alpha2`] | Alpha | CloudSQL managed resources and classes.
[`servicenetworking.gcp.crossplane.io/v1alpha2`] | Alpha | Service Networking managed resources and classes.
[`storage.gcp.crossplane.io/v1alpha2`] | Alpha | Cloud Storage managed resources and classes.

[`gcp.crossplane.io/v1alpha2`]: api/crossplaneio/stack-gcp/gcp-crossplane-io-v1alpha2.md
[`cache.gcp.crossplane.io/v1alpha2`]: api/crossplaneio/stack-gcp/cache-gcp-crossplane-io-v1alpha2.md
[`compute.gcp.crossplane.io/v1alpha2`]: api/crossplaneio/stack-gcp/compute-gcp-crossplane-io-v1alpha2.md
[`database.gcp.crossplane.io/v1alpha2`]: api/crossplaneio/stack-gcp/database-gcp-crossplane-io-v1alpha2.md
[`servicenetworking.gcp.crossplane.io/v1alpha2`]: api/crossplaneio/stack-gcp/servicenetworking-gcp-crossplane-io-v1alpha2.md
[`storage.gcp.crossplane.io/v1alpha2`]: api/crossplaneio/stack-gcp/storage-gcp-crossplane-io-v1alpha2.md

[API Versioning]: https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning
