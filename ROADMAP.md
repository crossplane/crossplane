# Roadmap

This document defines a high level roadmap for Crossplane development and upcoming releases. Community and contributor involvement is vital for successfully implementing all desired items for each release. We hope that the items listed below will inspire further engagement from the community to keep Crossplane progressing and shipping exciting and valuable features.

Any dates listed below and the specific issues that will ship in a given milestone are subject to change but should give a general idea of what we are planning. We use the [milestone](https://github.com/crossplane/crossplane/milestones) feature in Github so look there for the most up-to-date and issue plan.

## Table of Contents

* [What's next](#whats-next)

  * [v1.0.0 Release Candidate (Dec 2020)](#v100-release-candidate-dec-2020)
  * [Under Consideration](#under-consideration)

* [Released](#released)
  * [v0.14.0 Hardening, robustness, v1beta1 APIs in core](#v0140---hardening-robustness-v1beta1-apis-in-core)
  * [v0.13.0 Paving the way for a v1.0 release of Crossplane](#v0130-paving-the-way-for-a-v10-release-of-crossplane)
  * [v0.12.0 Upgrade claims/classes to a more powerful composition approach](#v0120-upgrade-claimsclasses-to-a-more-powerful-composition-approach)
  * [v0.11.0 Composition v1alpha1, OAM support, backup/restore, docs overhaul](#v0110-infra-composition-v1alpha1-oam-support-backuprestore-docs-overhaul)
  * [v0.10.0 Backup/restore, resource composition, Open Application Model](#v0100-backuprestore-resource-composition-open-application-model)
  * [v0.9.0 Providers, Stacks, Apps, Addons](#v090-providers-stacks-apps-addons)
  * [v0.8.0 Stacks simplify cloud-native app and infrastructure provisioning](#v080-stacks-simplify-cloud-native-app-and-infrastructure-provisioning)
  * [v0.7.0 Deploy Workloads to any Kubernetes Cluster, including bare-metal!](#v070-deploy-workloads-to-any-kubernetes-cluster-including-bare-metal)
  * [v0.6.0 Aggregated Stack Roles, GKECluster to v1beta1, test automation](#v060-aggregated-stack-roles-gkecluster-to-v1beta1-test-automation)
  * [v0.5.0 Continuous deployment for GitLab and ArgoCD with v1beta1 APIs](#v050-continuous-deployment-for-gitlab-and-argocd-with-v1beta1-apis)
  * [v0.4.0 Initial Rook support & stable v1beta1 APIs for AWS, GCP](#v040-initial-rook-support--stable-v1beta1-apis-for-aws-gcp)
  * [v0.3.0 Enable Community to Build Providers](#v030-enable-community-to-build-providers)
  * [v0.2.0 Workload Scheduling, Expand Supported Resources](#v020-workload-scheduling-expand-supported-resources)
  * [v0.1.0 Proof of Concept](#v010-proof-of-concept)

## What's Next

### v1.0.0 Release Candidate (Dec 2020)

* Hardening and cleanup for v1.0
  * Prometheus metrics for all binaries [#314](https://github.com/crossplane/crossplane/issues/314)
  * crossplane-runtime to v1.0

* Composition
  * Claim update propagation to its underlying composite resource [#1649](https://github.com/crossplane/crossplane/issues/1649)
  * Bi-directional patching for status [#1639](https://github.com/crossplane/crossplane/issues/1639)
  * Revision support for incremental upgrades [#1481](https://github.com/crossplane/crossplane/issues/1481)
  * Support taking values from members to fill a connection secret [#1609](https://github.com/crossplane/crossplane/issues/1609)
  * Validation webhooks

* Package Manager
  * Basic dependency resolution for packages [#1842](https://github.com/crossplane/crossplane/issues/1842)
    * i.e. automatically install the providers a configuration needs.

* Providers
  * AWS Provider
    * more API types [crossplane/provider-aws#149](https://github.com/crossplane/provider-aws/issues/149)
  * Helm Provider
    * v1beta1 APIs

  * Code Generation of Providers (work-in-progress)
    * AWS ACK Code Generation of the Crossplane provider-aws
      * [initial auto generated resources](https://github.com/crossplane/provider-aws/issues/149#issuecomment-718208201)
    * Azure Code Generation of the Crossplane provider-azure
      * [initial auto generated resources](https://github.com/matthchr/k8s-infra/tree/crossplane-hacking)
    * Clouds that don't have code gen pipelines
      * Wrap stateless Terraform providers (work-in-progress) [#262](https://github.com/crossplane/crossplane/issues/262)
      * [initial auto generated resources](https://github.com/kasey/provider-terraform-aws/tree/master/generated/resources)

* Open Application Model (OAM)
  * APIs to v1beta1
  * Hardening

### Under Consideration

* General
  * First-class multi-language support for defining `Compositions` and `Configuration` packages.
  * Managed resources can accept an array of resource references for cross-resource references (CRR)
  * Per-namespace mapping of IRSA and workload identity for finer grained infra permissions in multi-tenant clusters
  * Enhanced integration testing [#1033](https://github.com/crossplane/crossplane/issues/1033)

* Composition
  * Additional conversion strategies for XRDs with multiple version of an XR defined
  * `CustomComposition` support for use with cdk8s sidecar, TYY, and others [#1678](https://github.com/crossplane/crossplane/issues/1678)

* Package Manager
  * Conversion webhooks to support installing multiple API versions at the same time

* Providers
  * Code Generation of Providers (100% coverage)
    * AWS ACK Code Generation of the Crossplane provider-aws
      * auto generate all available types in the [aws-sdk-go/models/apis](https://github.com/aws/aws-sdk-go/blob/master/models/apis)
    * Azure Code Generation of the Crossplane provider-azure
      * auto generate all available types from the Azure metadata.
    * Clouds that don't have code gen pipelines
      * Wrap stateless Terraform providers [#262](https://github.com/crossplane/crossplane/issues/262)

  * GCP Provider
    * Explore code generation of a native Crossplane provider-gcp
    * GCP: DNS, SSL, and Ingress support #1123 [#1123](https://github.com/crossplane/crossplane/issues/1123)
    * GCP storage buckets to v1beta1 [crossplane/provider-gcp#130](https://github.com/crossplane/provider-gcp/issues/130)

  * Expanded Rook support
    * Support additional Rook storage providers
    * Install & configure Rook into a target cluster

  * Additional providers being incubated in https://github.com/crossplane-contrib

* GitLab Auto DevOps Phase 2 - provision managed services from GitLab pipelines
  * Currently the auto deploy app only supports PostgreSQL DBs
  * Support additional managed services from GitLab ADO pipelines
  * Add support for MySQL, Redis, Buckets, and more.

* Ease-of-use and improved experience
  * Standalone mode allowing Crossplane to run in a single container or process [#274](https://github.com/crossplane/crossplane/issues/274)

## Released

### [v0.14.0 - Hardening, robustness, v1beta1 APIs in core](https://github.com/crossplane/crossplane/releases/tag/v0.14.0)

* Hardening and cleanup for v1.0
  * Leader election for all controllers [#5](https://github.com/crossplane/crossplane/issues/5)

* Composition
  * APIs to v1beta1
  * Surface claim binding and secret publishing errors [#1862](https://github.com/crossplane/crossplane/pull/1862)
  * XRDs can support defining multiple versions of an XR, using the `None` conversion strategy [#1871](https://github.com/crossplane/crossplane/issues/1871)
  * Hardening and robustness fixes

* Package Manager
  * APIs to v1beta1
  * `ControllerConfig` can override default values for a `Provider` [#974](https://github.com/crossplane/crossplane/issues/974)
  * Support for Crossplane version constraints in `Provider` and `Configuration` packages [#1843](https://github.com/crossplane/crossplane/issues/1843)
  * Hardening and robustness fixes

* Providers
  * AWS Provider: more API types [crossplane/provider-aws#149](https://github.com/crossplane/provider-aws/issues/149)
    * S3 Bucket Policy to v1beta1 [#391](https://github.com/crossplane/provider-aws/pull/391)
    * IAM User Access Key v1alpha1 [#403](https://github.com/crossplane/provider-aws/pull/403)

  * Helm Provider
    * Support installing a Helm `Release` from a Crossplane `Composition`
    * v1alpha1 APIs [crossplane-contrib/provider-helm#38](https://github.com/crossplane-contrib/provider-helm/pull/38)

  * Code Generation of Providers (work-in-progress)
    * AWS ACK Code Generation of the Crossplane provider-aws
      * https://github.com/jaypipes/aws-controllers-k8s/tree/crossplane
      * https://github.com/crossplane/provider-aws/issues/149#issuecomment-718208201
    * Azure Code Generation of the Crossplane provider-azure
      * https://github.com/matthchr/k8s-infra/tree/crossplane-hacking
    * Clouds that don't have code gen pipelines
      * Code gen with stateless Terraform providers [#262](https://github.com/crossplane/crossplane/issues/262)
      * https://github.com/kasey/provider-terraform-aws/tree/master/generated/resources

* Open Application Model (OAM)
  * HealthScope support for PodSpecWorkload [#243](https://github.com/crossplane/oam-kubernetes-runtime/pull/243)
  * Allow OAM controller to create events [#239](https://github.com/crossplane/oam-kubernetes-runtime/pull/239)
  * RBAC rules must use plural resource names [#236](https://github.com/crossplane/oam-kubernetes-runtime/pull/236)
  * Migrate from Jenkins pipeline to GitHub Actions [#260](https://github.com/crossplane/oam-kubernetes-runtime/pull/260)
  * CRD discovery mechanism [#261](https://github.com/crossplane/oam-kubernetes-runtime/pull/261)

* Remove deprecated `KubernetesApplication`, `KubernetesTarget`, `KubernetesCluster`
  * replaced by Composition and provider-helm

### [v0.13.0 Paving the way for a v1.0 release of Crossplane](https://github.com/crossplane/crossplane/releases/tag/v0.13.0)
* Composition

  * Final type names for XRDs and XRCs:
    [crossplane#1679](https://github.com/crossplane/crossplane/pull/1679)
    * `CompositeResourceDefinition` (XRD) replaces InfrastructureDefinition and InfrastructurePublication types.
    * `Composite Resource Claims` (XRCs) replace Requirements and they no longer require any specific kind suffix.
  * Hardening and robustness enhancements towards v1beta1 quality

* Package Manager
  * Streamlined v2 design
    [crossplane#1616](https://github.com/crossplane/crossplane/pull/1616)
    * Supports installing and managing Crossplane `Providers` and `Configurations`
  * Package Manager v2
    [crossplane#1675](https://github.com/crossplane/crossplane/pull/1675)
    * Upgrade and rollback support
    * Faster package deploys
    * Paves the way for automatic package dependency resolution

* RBAC Manager
  * Automatically manages the RBAC roles and bindings required by `Providers` and `Composite` resources
  * An optional deployment that uses RBAC privilege escalation
  * Crossplane no longer requires cluster-admin privileges.

* Providers
  * General
    * Default `ProviderConfig` supported & migration
    * Removed deprecated claims/classes - you can now create your own claim kinds with Composition

  * AWS Provider: more API types [provider-aws#149](https://github.com/crossplane/provider-aws/issues/149)
    * S3 Bucket to v1beta1 [#331](https://github.com/crossplane/provider-aws/pull/331)
    * S3 Bucket Policy support [#289](https://github.com/crossplane/provider-aws/pull/289)
    * Referencer for SubnetGroup AWS ElasticCache [#314](https://github.com/crossplane/provider-aws/pull/314)
    * Add ARN to AtProvider for SNS Topic [#348](https://github.com/crossplane/provider-aws/pull/348)
    * ECR support [#307](https://github.com/crossplane/provider-aws/issues/307)

  * Helm Provider
    * experimental support - for use in `Compositions`

  * Code generation of Crossplane providers
    * Evaluate generating native Crossplane providers with existing code gen pipelines
    * Evaluate wrapping stateless Terraform providers (work-in-progress) [#262](https://github.com/crossplane/crossplane/issues/262)

* Open Application Model (OAM)
  * Moved AppConfig controller out of core
    * Install via: `helm install crossplane` with the `--set alpha.oam.enabled=true` flag
  * Enhance health scope with informative health condition [#194](https://github.com/crossplane/oam-kubernetes-runtime#194)
  * Add component webhook to support workload definition type [#198](https://github.com/crossplane/oam-kubernetes-runtime#198)
  * Add health check support for containerized.standard.oam.dev in Health [#214](https://github.com/crossplane/oam-kubernetes-runtime#214)
  * Run with fewer privileges [#228](https://github.com/crossplane/oam-kubernetes-runtime/pull/228)
  * Hardening and robustness enhancements towards v1beta1 quality

### [v0.12.0 Upgrade claims/classes to a more powerful composition approach](https://github.com/crossplane/crossplane/releases/tag/v0.12.0)

* Composition
  * Default composition for a definition [crossplane#1471](https://github.com/crossplane/crossplane/issues/1471)
  * Enforced composition for a definition [crossplane#1470](https://github.com/crossplane/crossplane/issues/1470)
  * Enhanced testing [crossplane#1474](https://github.com/crossplane/crossplane/issues/1474)
  * Deprecate resource claims and classes [crossplane#1479](https://github.com/crossplane/crossplane/issues/1479)

* Package Manager
  * Passing non-zero fsGroup in package deployments [crossplane#1577](https://github.com/crossplane/crossplane/pull/1577)

* Providers
  * AWS Provider: additional API types [provider-aws#149](https://github.com/crossplane/provider-aws/issues/149)
    * EKSCluster to v1beta1
    * ACMPCA Certificate Authority [provider-aws#226](https://github.com/crossplane/provider-aws/pull/226)
    * IAMRolePolicyAttachment to refer IAMPolicy
    * SQS
    * Route53

  * GCP Provider
    * GKE DnsCacheConfig, GcePersistentDiskCsiDriverConfig, KalmConfig [provider-gcp#229](https://github.com/crossplane/provider-gcp/pull/229)
    * PubSub support [provider-gcp#241](https://github.com/crossplane/provider-gcp/pull/241)

* Open Application Model (OAM)
  * Design: resource dependencies in OAM [oam-kubernetes-runtime#24](https://github.com/crossplane/oam-kubernetes-runtime/pull/24)
  * Design: versioning mechanism [oam-kubernetes-runtime#29](https://github.com/crossplane/oam-kubernetes-runtime/pull/29)

### [v0.11.0 Infra composition v1alpha1, OAM support, backup/restore, docs overhaul](https://github.com/crossplane/crossplane/releases/tag/v0.11.0)

* Composition
  * enhancements for v1alpha1 quality [#1343](https://github.com/crossplane/crossplane/issues/1343)

* Providers
  * v1beta1 quality conformance doc [#933](https://github.com/crossplane/crossplane/issues/933)

  * AWS Provider
    * Networking and VPC resources to v1beta1 [crossplane/provider-aws#145](https://github.com/crossplane/provider-aws/issues/145)
    * more API types [crossplane/provider-aws#149](https://github.com/crossplane/provider-aws/issues/149)
      * DynamoDB [crossplane/provider-aws#147](https://github.com/crossplane/provider-aws/issues/147)
      * SQS [crossplane/provider-aws#170](https://github.com/crossplane/provider-aws/issues/170)
      * Cert Manager [crossplane/provider-aws#171](https://github.com/crossplane/provider-aws/issues/171)
      * DNS [crossplane/provider-aws#172](https://github.com/crossplane/provider-aws/issues/172)

  * Azure Provider
    * Firewall rules for MySQL and PostgreSQL [provider-azure#146](https://github.com/crossplane/provider-azure/pull/146)

* Open Application Model (OAM)
  * Enhanced support for [OAM](https://oam.dev/) (Open Application Model) API types

* Docs overhaul (part 3/3) - https://crossplane.io/docs
  * Backup / restore docs [crossplane#1353](https://github.com/crossplane/crossplane/issues/1353)
  * Documentation (and diagrams) about data model in Crossplane (including both application and infrastructure)
  * Updated docs sidebar

### [v0.10.0 Backup/restore, resource composition, Open Application Model](https://github.com/crossplane/crossplane/releases/tag/v0.10.0)

* Backup/restore compatibility with tools like Velero
  * Allow a KubernetesApplication to be backed up and restored [crossplane#1382](https://github.com/crossplane/crossplane/issues/1382)
  * Allow connection secrets to be backed up and restored [crossplane-runtime#140](https://github.com/crossplane/crossplane-runtime/issues/140)
  * Support backup and restore of all GCP managed resources [provider-gcp#207](https://github.com/crossplane/provider-gcp/issues/207)
  * Support backup and restore of all Azure managed resources [provider-azure#128](https://github.com/crossplane/provider-azure/issues/128)
  * Support backup and restore of all AWS managed resources [provider-aws#181](https://github.com/crossplane/provider-aws/issues/181)
  * Allow Stack, StackInstall, StackDefinition to be backed up and restored [crossplane#1389](https://github.com/crossplane/crossplane/issues/1389)

* Composition
  * Experimental MVP [#1343](https://github.com/crossplane/crossplane/issues/1343)
  * Defining your own claim kinds [#1106](https://github.com/crossplane/crossplane/issues/1106) 
  * Allowing a claim to be satisfied by multiple resources [#1105](https://github.com/crossplane/crossplane/issues/1105)

* Providers
  * Azure Provider
    * CosmosDB Account supports MongoDB and Cassandra [provider-azure#138](https://github.com/crossplane/provider-azure/pull/138)

* Open Application Model (OAM)
  * Experimental support for [OAM](https://oam.dev/) (Open Application Model) API types
  * Revised [Kubernetes-friendly OAM spec](https://github.com/oam-dev/spec/pull/304/files)
  * OAM App Config Controller support [#1268](https://github.com/crossplane/crossplane/issues/1268)
  * Enhance Crossplane to support a choice of local and remote workload scheduling
  * OAM sample app: [crossplane/app-service-tracker](https://github.com/crossplane/app-service-tracker)

* Docs overhaul (part 2/3) - https://crossplane.io/docs
  * Documentation (and diagrams) about data model in Crossplane (including both application and infrastructure)
  * Updated docs sidebar

### [v0.9.0 Providers, Stacks, Apps, Addons](https://github.com/crossplane/crossplane/releases/tag/v0.9.0)

* Rename GitHub org from [crossplaneio](https://github.com/crossplaneio) to [crossplane](https://github.com/crossplane)
* Docs overhaul (part 1/3) - https://crossplane.io/docs
* New `packageType` options in `app.yaml`, including: `Provider`, `Stack`, `Application`, and `Addon` (#1348) plus repo name updates: [#1300](https://github.com/crossplane/crossplane/issues/1300)
  * [provider-gcp](https://github.com/crossplane/provider-gcp)
  * [provider-aws](https://github.com/crossplane/provider-aws)
  * [provider-azure](https://github.com/crossplane/provider-azure)
  * [stack-gcp-sample](https://github.com/crossplane/stack-gcp-sample)
  * [stack-aws-sample](https://github.com/crossplane/stack-aws-sample)
  * [stack-azure-sample](https://github.com/crossplane/stack-azure-sample)
  * [app-wordpress](https://github.com/crossplane/app-wordpress)
  * [addon-oam-kubernetes-remote](https://github.com/crossplane/addon-oam-kubernetes-remote)
* Incorporate versioning and upgrade design feedback [#1160](https://github.com/crossplane/crossplane/issues/1160)
* Support for NoSQL database claims. Providers may now offer managed services that can be bound to this claim type. [#1356](https://github.com/crossplane/crossplane/issues/1356)
* `KubernetesApplication` now supports:
  * updates propagated to objects in a remote Kubernetes cluster. [#1341](https://github.com/crossplane/crossplane/issues/1341)
  * scheduling directly to a `KubernetesTarget` in the same namespace as a `KubernetesApplication`. [#1315](https://github.com/crossplane/crossplane/issues/1315 )
* Experimental support for [OAM](https://oam.dev/) (Open Application Model) API types:
  * Revised [Kubernetes-friendly OAM spec](https://github.com/oam-dev/spec/pull/304/files)
  * OAM App Config Controller support [#1268](https://github.com/crossplane/crossplane/issues/1268)
  * Enhance Crossplane to support a choice of local and remote workload scheduling
* Security enhanced mode with `stack manage --restrict-core-apigroups`, which restricts packages from being installed with permissions on the core API group. [#1333 ](https://github.com/crossplane/crossplane/issues/1333)
* Stacks Manager support for private repos and robot account credentials
* Release process and efficiency improvements

### [v0.8.0 Stacks simplify cloud-native app and infrastructure provisioning](https://github.com/crossplane/crossplane/releases/tag/v0.8.0)

* Stacks for ready-to-run cloud environments (GCP, AWS, Azure) [#1136](https://github.com/crossplane/crossplane/issues/1136)
  * Spin up secure cloud environments with just a few lines of yaml 
  * Single CR creates networks, subnets, secure service connectivity, k8s clusters, resource classes, etc.
* PostgreSQL 11 support on the `PostgreSQLInstance` claim 
  * thanks first-time contributor @vasartori! [#1245](https://github.com/crossplane/crossplane/pull/1245)
* Improved logging and eventing 
  * [Observability Developer Guide](https://crossplane.io/docs/v0.8/observability-developer-guide.html) for logging and eventing in Crossplane controllers
  * [crossplane/crossplane-runtime#104](https://github.com/crossplane/crossplane-runtime/issues/104) instrumentation and updated all cloud provider stacks
* Enable [provider-aws](https://github.com/crossplane/provider-aws) to authenticate to the AWS API using [IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
  * when running on EKS [provider-aws#126](https://github.com/crossplane/provider-aws/pull/126)
* Host-aware Stack Manager [#1038](https://github.com/crossplane/crossplane/issues/1038)
  * Enables deploying multiple Crossplane instances watching different Kubernetes API servers on a single Host Kubernetes cluster.
* RBAC group and role refinements
* Support default select values in the UI schema for Crossplane resources
* Template Stacks (alpha)
  * Kustomize and helm engine support for pluggable rendering
  * Ported [stack-minimal-gcp](https://github.com/crossplane/stack-minimal-gcp/pull/1) and [sample-stack-wordpress](https://github.com/crossplane/sample-stack-wordpress/pull/31) to use Template Stacks
  * Published [stack-minimal-gcp](https://hub.docker.com/r/crossplane/stack-minimal-gcp/tags) and [sample-stack-wordpress](https://hub.docker.com/r/crossplane/sample-stack-wordpress/tags) to https://hub.docker.com/u/crossplane

### [v0.7.0 Deploy Workloads to any Kubernetes Cluster, including bare-metal!](https://github.com/crossplane/crossplane/releases/tag/v0.7.0)

* KubernetesTarget kind for scheduling KubernetesApplications [#859](https://github.com/crossplane/crossplane/issues/859)
* Improved the UI schema for resources supported by Crossplane stacks [#38](https://github.com/upbound/crossplane-graphql/issues/38)
* GCP networking resources to v1beta1 [crossplane/provider-gcp#131](https://github.com/crossplane/provider-gcp/issues/131)
* GCP integration tests [crossplane/provider-gcp#87](https://github.com/crossplane/provider-gcp/issues/87)
* Template Stacks (experimental): integrate template engine controllers with stack manager [#36](https://github.com/upbound/stacks-marketplace-squad/issues/36)

### [v0.6.0 Aggregated Stack Roles, GKECluster to v1beta1, test automation](https://github.com/crossplane/crossplane/releases/tag/v0.6.0)

* The Stack Manager supports more granular management of permissions for cluster (environment) and namespace (workspace) scoped stacks.
  * Default admin, editor, and viewer roles automatically updated as Stacks are installed/uninstalled.
  * Admins can create role bindings to these roles, to simplify granting user permissions.
  * Details in the [design doc](https://github.com/crossplane/crossplane/blob/master/design/one-pager-packages-security-isolation.md).
* GKE cluster support has moved to `v1beta1` with node pool support.
  * The `v1alpha3` GKE cluster support has been left intact and can run side by side with v1beta1
* Integration test framework in the crossplane-runtime, reducing the burden to provide integration test coverage across all projects and prevent regressions.
* Helm 2 and 3 compatibility, Crossplane and all of its CRDs are supported to be installed by both Helm2 and Helm3
* Design and architecture documents:
  * ["Easy config stacks"](https://github.com/crossplane/crossplane/blob/master/design/one-pager-resource-packs.md)
  * [Consuming any Kubernetes cluster for workload scheduling](https://github.com/crossplane/crossplane/blob/master/design/one-pager-consuming-k8s-clusters.md)
  * [User experience for template stacks](https://github.com/crossplane/crossplane/blob/master/design/design-doc-template-stacks-experience.md).
* [Bug fixes and other closed issues](https://github.com/crossplane/crossplane/milestone/6?closed=1)

### [v0.5.0 Continuous deployment for GitLab and ArgoCD with v1beta1 APIs](https://github.com/crossplane/crossplane/releases/tag/v0.5.0)

* GitLab 12.5 Auto DevOps (ADO) integration phase 1 - provision managed PostgreSQL from GitLab ADO pipelines
  * Subset of the overall [GitLab Auto DevOps integration](https://gitlab.com/groups/gitlab-org/-/epics/1866#note_216080986) 
  * [Crossplane as a GitLab-managed app (phase1)](https://gitlab.com/gitlab-org/gitlab/issues/34702) - provision managed PostgreSQL from GitLab ADO pipelines

* CD integration examples ArgoCD [#631](https://github.com/crossplane/crossplane/issues/631)

* Stable v1beta1 Services APIs for managed databases and caches (Azure) [#863](https://github.com/crossplane/crossplane/issues/863)
  * Upgrade Azure stack to v1beta1: Azure Database and Azure Cache for Redis with high-def CRDs & controllers
    * crossplane/provider-azure#28 Azure SQL and Redis resources v1beta1

* Bug fixes and test automation

### [v0.4.0 Initial Rook support & stable v1beta1 APIs for AWS, GCP](https://github.com/crossplane/crossplane/releases/tag/v0.4.0)

* Claim-based provisioning of [Rook](https://rook.io/)-managed databases [#862](https://github.com/crossplane/crossplane/issues/862)
  * Support for CockroachDB and Yugabyte DB

* Stable v1beta1 Services APIs for managed databases and caches (GCP, AWS) [#863](https://github.com/crossplane/crossplane/issues/863)
  * Align on shape of APIs & best practices
    * Beta meta model w/ DB & Redis, so users can deploy to dev/test/prod
    * Naming scheme for all resources.
    * Managed resource name as external name for all resources.
  * Upgrade GCP stack to v1beta1: CloudSQL and CloudMemoryInstance with high-def CRDs & controllers
  * Upgrade AWS stack to v1beta1: RDS and ReplicationGroup with high-def CRDs & controllers

* Cross-resource referencing for networks, subnets, and other resources [#707](https://github.com/crossplane/crossplane/issues/707)
  * Support `kubectl apply -f` for a directory of resources to cleanly support GitOps for both infrastructure and apps
  * Sample infra and app repos you can `kubectl apply -f` and have a working environment quickly
    * infrastructure (networks, subnets, managed k8s cluster, resource classes for databases, etc.)
    * apps (e.g. kubernetes core resources for e.g. a Wordpress app plus the resource claims for managed service dependencies
  * Update crossplane.io services guides and stacks guides to use `kubectl apply -f` technique

* Release automation for shorter release cycles and hot fixes [#864](https://github.com/crossplane/crossplane/issues/864) 
  * Updating pipelines to include automation [#6](https://github.com/crossplane/crossplane/issues/6)
  * SonarCloud checks for cloud provider stacks [#875](https://github.com/crossplane/crossplane/issues/875)
  * crossplane-runtime build pipelines [crossplane/crossplane-runtime#14](https://github.com/crossplane/crossplane-runtime/issues/14)

* Trace utility for enhanced debugging support. [#744](https://github.com/crossplane/crossplane/issues/744)

* Simple Resource Class Selection [#952](https://github.com/crossplane/crossplane/issues/952)

* Crossplane supporting work for GitLab 12.5 Auto DevOps [#867](https://github.com/crossplane/crossplane/issues/867)

### [v0.3.0 Enable Community to Build Providers](https://github.com/crossplane/crossplane/releases/tag/v0.3.0)

* Real-world applications on-top of Crossplane
  * GitLab [#284](https://github.com/crossplane/crossplane/issues/284)
  * More applications to follow

* Resource Class enhancements: default classes, validation, annotation
  * Default resource classes - increases claim portability [#151](https://github.com/crossplane/crossplane/issues/151)
  * Resource classes can be validated and annotated [#613](https://github.com/crossplane/crossplane/issues/613)

* Infra Stacks (out-of-tree) with single-region secure connectivity between k8s and DBaaS, Redis, Buckets
  * Stacks Manager: App vs. Infra Stacks, namespace isolation, annotation support [#609](https://github.com/crossplane/crossplane/issues/609)
  * Move Infra Stacks (GCP, AWS, Azure) into separate repos & upgrade to kubebuilder2 [#612](https://github.com/crossplane/crossplane/issues/612)
  * GCP Infra Stack: single-region secure connectivity: GKE & CloudSQL, CloudMemorystore, Buckets [#615](https://github.com/crossplane/crossplane/issues/615)
  * AWS Infra Stack: single-region secure connectivity: EKS & RDS, ElastiCache, Buckets [#616](https://github.com/crossplane/crossplane/issues/616)
  * Azure Infra Stack: single-region secure connectivity: AKS & AzureSQL, AzureCache, Buckets [#617](https://github.com/crossplane/crossplane/issues/617)
  * Stacks v1 CLI / kubectl plugin: init, build, push commands [#614](https://github.com/crossplane/crossplane/issues/614)

* Docs & examples
  * Infra Stack Developer Guide [#610](https://github.com/crossplane/crossplane/issues/610)
  * Portable Wordpress App Stack (kubebuilder-based) published to registry [#572](https://github.com/crossplane/crossplane/issues/572)
  * Refresh 0.3 Docs: reflect enhancements, better on-boarding UX, easier to get started [#625](https://github.com/crossplane/crossplane/issues/625)
  * Crossplane.io reflects the updated roadmap / vision [crossplane.github.io#22](https://github.com/crossplane/crossplane.github.io/issues/22)

### [v0.2.0 Workload Scheduling, Expand Supported Resources](https://github.com/crossplane/crossplane/releases/tag/v0.2.0)

* Workload Scheduling
  * Design for smart scheduler, optimization, resource placement [#278](https://github.com/crossplane/crossplane/issues/278)
  * Basic workload scheduler with cluster selector [#309](https://github.com/crossplane/crossplane/issues/309)
  * Update workload propagation to avoid  collisions on target cluster [#308](https://github.com/crossplane/crossplane/pull/308)
  * Minimize workload deployment kubeconfig settings for AKS to be consistent with GKE, EKS [#273](https://github.com/crossplane/crossplane/issues/273)
  * Update workload deployment docs [#239](https://github.com/crossplane/crossplane/issues/239)

* New Stateful managed services across AWS, Azure, and GCP
  * Database: PostgreSQL [#54](https://github.com/crossplane/crossplane/issues/54), MySQL [#53](https://github.com/crossplane/crossplane/issues/53)
  * Cache / Redis [#137](https://github.com/crossplane/crossplane/issues/137), [#282](https://github.com/crossplane/crossplane/issues/282)
  * Buckets [#295](https://github.com/crossplane/crossplane/issues/295), [#109](https://github.com/crossplane/crossplane/issues/109)

* Performance and Efficiency
  * Reconciliation requeue pattern [#241](https://github.com/crossplane/crossplane/issues/241)

* UX Enhancements
  * Enhanced kubectl printer columns [#38](https://github.com/crossplane/crossplane/issues/38)

* Engineering
  * General resource controller used for more types [#276](https://github.com/crossplane/crossplane/issues/276)
  * Controllers use consistent logging [#7](https://github.com/crossplane/crossplane/issues/7)
  * Consistent testing paradigm [#269](https://github.com/crossplane/crossplane/issues/269)

### [v0.1.0 Proof of Concept](https://github.com/crossplane/crossplane/releases/tag/v0.1.0)

* Resource Claims, Resource Classes, and Resources
* Basic Container Workload
  * Support for Deployments / Services
  * Resource Usage and Secret management
* Cloud Providers
  * Provider CRDs, credentials management, API/SDK consumption
  * AWS, GCP, and Azure
* Managed Kubernetes Clusters
  * Support for EKS, AKS and GKE
  * Generic Kubernetes Cluster Resource Claim
  * Status and Conditions for Clusters
  * Static and Dynamic Provisioning
* MySQL Support
  * Static and Dynamic Provisioning
  * Provider specific MySQL CRDs (AWS RDS, GCP CloudSQL, Azure MySQL)
  * Connection strings and firewall support
* Resource Controller depth and reliability
  * CRUD support and robust lifecycle management
  * CRD status Conditions for status of resources
  * Event recording
  * Normalized logging using single logging solution (with configurable levels)
  * Retry/recovery from failure, idempotence, dealing with partial state
* CI builds/tests/releases
  * New jenkins instance (similar to Rook's jenkins)
  * Developer unit testing with high code coverage
  * Integration testing pipeline
  * Artifact publishing (container images, crossplane helm chart, etc.)
* Documentation
  * User guides, quick-starts, walkthroughs
  * Godocs developer docs for source code/packages/libraries
* Open source project management
  * Governance
  * Contributor License Agreement (CLA) or Developer Certificate of Origin (DCO)
