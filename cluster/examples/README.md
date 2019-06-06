# Crossplane Examples

In this directory you will find example workloads demonstrating the usage of various resources supported by Crossplane for each of the supported Clouds.

[Cloud provider credentials](../../docs/cloud-providers.md) will need be needed be before using any of these examples.

| Directory | Description | AWS | Azure | GCP
| ---       | ---         | --- | ---   | ---
| [cache](cache/)     | Provisions a Managed Redis service from any cloud. | Redis | Redis | CloudMemoryStore Redis |
| [compute](compute/)   | Creates a WordPress Workload that runs in a Crossplane created Kubernetes cluster using a Crossplane created managed MySQL service. ([Using Compute Workloads](https://github.com/crossplaneio/crossplane/blob/master/design/complex-workloads.md#complex-workloads-in-crossplane), [Legacy](https://github.com/crossplaneio/crossplane/issues/456)) | [EKS + RDS](../../docs/workloads/aws/wordpress-aws.md) | [AKS + Azure SQL](../../docs/workloads/azure/wordpress-azure.md) | GKE + Cloud SQL |
| [database](database/)  | Deploys a PostgreSQL database in any cloud. | RDS | Azure SQL | Cloud SQL |
| [extensions](extensions/) | Deploys the [sample-extension](https://github.com/crossplaneio/sample-extension) | n/a | n/a | n/a |
| [gitlab](gitlab/)    | Deploy GitLab in all three clouds. See the per-Cloud documentation links. | [AWS](../../docs/gitlab/gitlab-aws.md) | | [GCP](../../docs/gitlab/gitlab-gcp.md) |
| [kubernetes](kubernetes/) | Deploy a Kubernetes Cluster in any clouds. | EKS | AKS | GKE |
| [storage](storage/)   | Provisions a object storage bucket from any cloud. | S3 | Storage | GCS |
| [wordpress](wordpress/) | Provisions a MySQL service from any cloud and uses it in a WordPress deployment. | RDS | Azure SQL | Cloud SQL |
| [workloads](workloads/) | Creates a WordPress Workload that runs in a Crossplane created Kubernetes cluster using a Crossplane created managed MySQL service. ([Using Complex Workloads](../../design/complex-workloads.md#complex-workloads-in-crossplane)) |  |  | [GKE + Cloud SQL](../../docs/workloads/gcp/wordpress-gcp.md) |
