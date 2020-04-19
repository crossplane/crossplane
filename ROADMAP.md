# Roadmap

This document defines a high level roadmap for Crossplane development and upcoming releases. Community and contributor involvement is vital for successfully implementing all desired items for each release. We hope that the items listed below will inspire further engagement from the community to keep Crossplane progressing and shipping exciting and valuable features.

Any dates listed below and the specific issues that will ship in a given milestone are subject to change but should give a general idea of what we are planning. We use the [milestone](https://github.com/crossplane/crossplane/milestones) feature in Github so look there for the most up-to-date and issue plan.

## [v0.1 - Proof of Concept](https://github.com/crossplane/crossplane/releases/tag/v0.1.0)

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

## [v0.2 - Workload Scheduling, Expand Supported Resources](https://github.com/crossplane/crossplane/releases/tag/v0.2.0)

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

## [v0.3 - Enable Community to Build Infra Stacks](https://github.com/crossplane/crossplane/releases/tag/v0.3.0)

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

## [v0.4.0 Initial Rook support & stable v1beta1 APIs for AWS, GCP](https://github.com/crossplane/crossplane/releases/tag/v0.4.0)
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

## [v0.5.0 Continous deployment for GitLab and ArgoCD with v1beta1 APIs](https://github.com/crossplane/crossplane/releases/tag/v0.5.0)
* GitLab 12.5 Auto DevOps (ADO) integration phase 1 - provision managed PostgreSQL from GitLab ADO pipelines
  * Subset of the overall [GitLab Auto DevOps integration](https://gitlab.com/groups/gitlab-org/-/epics/1866#note_216080986) 
  * [Crossplane as a GitLab-managed app (phase1)](https://gitlab.com/gitlab-org/gitlab/issues/34702) - provision managed PostgreSQL from GitLab ADO pipelines

 * CD integration examples ArgoCD [#631](https://github.com/crossplane/crossplane/issues/631)

* Stable v1beta1 Services APIs for managed databases and caches (Azure) [#863](https://github.com/crossplane/crossplane/issues/863)
  * Upgrade Azure stack to v1beta1: Azure Database and Azure Cache for Redis with high-def CRDs & controllers
    * crossplane/provider-azure#28 Azure SQL and Redis resources v1beta1

* Bug fixes and test automation

## [v0.6.0 Aggregated Stack Roles, GKECluster to v1beta1, test automation](https://github.com/crossplane/crossplane/releases/tag/v0.6.0)
* The Stack Manager supports more granular management of permissions for cluster (environment) and namespace (workspace) scoped stacks.
  * Default admin, editor, and viewer roles automatically updated as Stacks are installed/uninstalled.
  * Admins can create role bindings to these roles, to simplify granting user permissions.
  * Details in the [design doc](https://github.com/crossplane/crossplane/blob/master/design/one-pager-stacks-security-isolation.md).
* GKE cluster support has moved to `v1beta1` with node pool support.
  * The `v1alpha3` GKE cluster support has been left intact and can run side by side with v1beta1 
* Integration test framework in the crossplane-runtime, reducing the burden to provide integration test coverage across all projects and prevent regressions.
* Helm 2 and 3 compatibility, Crossplane and all of its CRDs are supported to be installed by both Helm2 and Helm3
* Design and architecture documents:
  * ["Easy config stacks"](https://github.com/crossplane/crossplane/blob/master/design/one-pager-resource-packs.md)
  * [Consuming any Kubernetes cluster for workload scheduling](https://github.com/crossplane/crossplane/blob/master/design/one-pager-consuming-k8s-clusters.md)
  * [User experience for template stacks](https://github.com/crossplane/crossplane/blob/master/design/design-doc-template-stacks-experience.md).
* [Bug fixes and other closed issues](https://github.com/crossplane/crossplane/milestone/6?closed=1)

## [v0.7.0 Deploy Workloads to any Kubernetes Cluster, including bare-metal!](https://github.com/crossplane/crossplane/releases/tag/v0.7.0)
* KubernetesTarget kind for scheduling KubernetesApplications [#859](https://github.com/crossplane/crossplane/issues/859)
* Improved the UI schema for resources supported by Crossplane stacks [#38](https://github.com/upbound/crossplane-graphql/issues/38)
* GCP networking resources to v1beta1 [crossplane/provider-gcp#131](https://github.com/crossplane/provider-gcp/issues/131)
* GCP integration tests [crossplane/provider-gcp#87](https://github.com/crossplane/provider-gcp/issues/87)
* Template Stacks (experimental): integrate template engine controllers with stack manager [#36](https://github.com/upbound/stacks-marketplace-squad/issues/36)

## [v0.8.0 Stacks simplify cloud-native app and infrastructure provisioning](https://github.com/crossplane/crossplane/releases/tag/v0.8.0)
- Stacks for ready-to-run cloud environments (GCP, AWS, Azure) [#1136](https://github.com/crossplane/crossplane/issues/1136)
  - Spin up secure cloud environments with just a few lines of yaml 
  - Single CR creates networks, subnets, secure service connectivity, k8s clusters, resource classes, etc.
- PostgreSQL 11 support on the `PostgreSQLInstance` claim 
  - thanks first-time contributor @vasartori! [#1245](https://github.com/crossplane/crossplane/pull/1245)
- Improved logging and eventing 
  - [Observability Developer Guide](https://crossplane.io/docs/v0.8/observability-developer-guide.html) for logging and eventing in Crossplane controllers
  - [crossplane/crossplane-runtime#104](https://github.com/crossplane/crossplane-runtime/issues/104) instrumentation and updated all cloud provider stacks
- Enable [provider-aws](https://github.com/crossplane/provider-aws) to authenticate to the AWS API using [IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
  - when running on EKS [provider-aws#126](https://github.com/crossplane/provider-aws/pull/126)
- Host-aware Stack Manager [#1038](https://github.com/crossplane/crossplane/issues/1038)
  - Enables deploying multiple Crossplane instances watching different Kubernetes API servers on a single Host Kubernetes cluster.
- RBAC group and role refinements
- Support default select values in the UI schema for Crossplane resources
- Template Stacks (alpha)
  - Kustomize and helm engine support for pluggable rendering
  - Ported [stack-minimal-gcp](https://github.com/crossplane/stack-minimal-gcp/pull/1) and [sample-stack-wordpress](https://github.com/crossplane/sample-stack-wordpress/pull/31) to use Template Stacks
  - Published [stack-minimal-gcp](https://hub.docker.com/r/crossplane/stack-minimal-gcp/tags) and [sample-stack-wordpress](https://hub.docker.com/r/crossplane/sample-stack-wordpress/tags) to https://hub.docker.com/u/crossplane

## [v0.9.0 Providers, Stacks, Apps, Addons](https://github.com/crossplane/crossplane/releases/tag/v0.9.0)
* Rename GitHub org from [crossplaneio](https://github.com/crossplaneio) to [crossplane](https://github.com/crossplane)
* Docs overhaul (part 1/2) - https://crossplane.io/docs
* New `packageType` options in `app.yaml`, including: `Provider`, `Stack`, `Application`, and `Addon` (#1348) plus repo name updates: [#1300](https://github.com/crossplane/crossplane/issues/1300)
  - [provider-gcp](https://github.com/crossplane/provider-gcp)
  - [provider-aws](https://github.com/crossplane/provider-aws)
  - [provider-azure](https://github.com/crossplane/provider-azure)
  - [stack-gcp-sample](https://github.com/crossplane/stack-gcp-sample)
  - [stack-aws-sample](https://github.com/crossplane/stack-aws-sample)
  - [stack-azure-sample](https://github.com/crossplane/stack-azure-sample)
  - [app-wordpress](https://github.com/crossplane/app-wordpress)
  - [addon-oam-kubernetes-remote](https://github.com/crossplane/addon-oam-kubernetes-remote)
* Incorporate versioning and upgrade design feedback [#1160](https://github.com/crossplane/crossplane/issues/1160)
* Support for NoSQL database claims. Providers may now offer managed services that can be bound to this claim type. [#1356](https://github.com/crossplane/crossplane/issues/1356)
* `KubernetesApplication` now supports:
   - updates propagated to objects in a remote Kubernetes cluster. [#1341 ](https://github.com/crossplane/crossplane/issues/1341)
   - scheduling directly to a `KubernetesTarget` in the same namespace as a `KubernetesApplication`. [#1315 ](https://github.com/crossplane/crossplane/issues/1315 )
* Experimental support for [OAM](https://oam.dev/) (Open Application Model) API types:
  * Revised [Kubernetes-friendly OAM spec](https://github.com/oam-dev/spec/pull/304/files)
  * OAM App Config Controller support [#1268](https://github.com/crossplane/crossplane/issues/1268)
  * Enhance Crossplane to support a choice of local and remote workload scheduling
* Security enhanced mode with `stack manage --restrict-core-apigroups`, which restricts packages from being installed with permissions on the core API group. [#1333 ](https://github.com/crossplane/crossplane/issues/1333)
* Stacks Manager support for private repos and robot account credentials
* Release process and efficiency improvements


## v0.10.0
* Backup/restore support - e.g. with Velero
  - Allow a KubernetesApplication to be backed up and restored [crossplane#1382](https://crossplane/crossplane/issues/#1382)
  - Allow connection secrets to be backed up and restored [crossplane-runtime#140](https://crossplane/crossplane-runtime/issues/#140)
  - Support backup and restore of all GCP managed resources [provider-gcp#207](https://crossplane/provider-gcp/issues/#207)
  - Support backup and restore of all Azure managed resources [provider-azure#128](https://crossplane/provider-azure/issues/#128)
  - Support backup and restore of all AWS managed resources [provider-aws#181](https://crossplane/provider-gcp/issues/#181)
  - Allow Stack, StackInstall, StackDefinition to be backed up and restored [crossplane#1389](https://crossplane/crossplane/issues/#1389)
  - Backup and Restore doc [crossplane#1353](https://crossplane/crossplane/issues/#1353)

* v1beta1 quality conformance doc [#933](https://github.com/crossplane/crossplane/issues/933)
* v1beta1 quality for AWS API types  
  - Networking and VPC [crossplane/provider-aws#145](https://github.com/crossplane/provider-aws/issues/145)

* AWS Provider: additional API types [crossplane/provider-aws#149](https://github.com/crossplane/provider-aws/issues/149)
  - DynamoDB [crossplane/provider-aws#147](https://github.com/crossplane/provider-aws/issues/147)
  - SQS [crossplane/provider-aws#170](https://github.com/crossplane/provider-aws/issues/170)
  - Cert Manager [crossplane/provider-aws#171](https://github.com/crossplane/provider-aws/issues/171)
  - DNS [crossplane/provider-aws#172](https://github.com/crossplane/provider-aws/issues/172)

* Basic versioning and upgrade support [#1334](https://github.com/crossplane/crossplane/issues/1334)

* Resource composition - experimental MVP [#1343](https://github.com/crossplane/crossplane/issues/1343)

* Experimental support for [OAM](https://oam.dev/) (Open Application Model) API types
  * Revised [Kubernetes-friendly OAM spec](https://github.com/oam-dev/spec/pull/304/files)
  * OAM App Config Controller support [#1268](https://github.com/crossplane/crossplane/issues/1268)
  * Enhance Crossplane to support a choice of local and remote workload scheduling
  * OAM sample app: [crossplane/app-service-tracker](https://github.com/crossplane/app-service-tracker)

* Docs overhaul (part 2/2) - https://crossplane.io/docs
  * Documentation (and diagrams) about data model in Crossplane (including both application and infrastructure)
  * Updated docs sidebar

## Roadmap
* Versioning and upgrade support [#879](https://github.com/crossplane/crossplane/issues/879) 

* Integration testing
  * Integration testing support [#1033](https://github.com/crossplane/crossplane/issues/1033)
  * AWS Stack integration tests 
  * Azure Stack integration tests 

* Designs for:
   * Defining your own claim kinds [#1106](https://github.com/crossplane/crossplane/issues/1106) 
   * Allowing a claim to be satisfied by multiple resources [#1105](https://github.com/crossplane/crossplane/issues/1105)
   * Versioning and upgrade support [#879](https://github.com/crossplane/crossplane/issues/879), [#435](https://github.com/crossplane/crossplane/issues/435)

* GCP: DNS, SSL, and Ingress support #1123 [#1123](https://github.com/crossplane/crossplane/issues/1123)

* More real-world Stacks into multiple clouds
  * Refresh existing GitLab Stack to use latest Crossplane [#866](https://github.com/crossplane/crossplane/issues/866)
  * Additional real-world apps and scenarios [#868](https://github.com/crossplane/crossplane/issues/868)
  * Stacks Manager support for private repos and robot account credentials

* UX enhancements for debuggability and observability
  * Visible error messages for all error cases surfaced in claims and/or eventing
  * Static provisioning examples to highlight simplicity. 

 * v1beta1 Services APIs
   * Incorporate beta1 feedback
   * Upgrade other supported services to v1beta1 (e.g. Buckets, etc.)
   * Code generation of API types, controller scaffolding to further streamline additional services
   * GCP storage buckets to v1beta1 [crossplane/provider-gcp#130](https://github.com/crossplane/provider-gcp/issues/130)
   * AWS S3 buckets [crossplane/provider-aws#99](https://github.com/crossplane/provider-aws/issues/99)

* Expanded Rook support
  * Support additional Rook storage providers
  * Install & configure Rook into a target cluster

 * GitLab Auto DevOps integration phase 2 - provision managed services from GitLab pipelines
   * Currently the auto deploy app only supports PostgreSQL DBs
   * Support additional managed services from GitLab ADO pipelines
   * Add support for MySQL, Redis, Buckets, and more. (GitLab 12.6)

* Policy-based secure connectivity & environment configuration
  * Additional secure connectivity strategies for GCP, AWS, Azure
  * Reuse of resource classes across environments

* Enhanced Workload Scheduling
  * Region and cloud provider aware scheduling [#279](https://github.com/crossplane/crossplane/issues/279)
  * Delayed binding of resources to support co-location in same region [#156](https://github.com/crossplane/crossplane/issues/156)
  * Workloads declare their resource usage [#115](https://github.com/crossplane/crossplane/issues/115)
  * Optimization for many resource attributes [#287](https://github.com/crossplane/crossplane/issues/287)
  * Extensibility points to allow external scheduler integration [#288](https://github.com/crossplane/crossplane/issues/288)

* Heterogeneous application support
  * Serverless (functions) [#285](https://github.com/crossplane/crossplane/issues/285)
  * Containers and other Kubernetes deployment types (e.g., Helm charts) [#158](https://github.com/crossplane/crossplane/issues/158)
  * Virtual Machines [#286](https://github.com/crossplane/crossplane/issues/286)

* New Stateful managed services across AWS, Azure, and GCP
  * MongoDB [#280](https://github.com/crossplane/crossplane/issues/280)
  * Message Queues [#281](https://github.com/crossplane/crossplane/issues/281)

* Auto-scaling
  * Cluster auto-scaler [#159](https://github.com/crossplane/crossplane/issues/159)
  * Node pools and worker nodes [#152](https://github.com/crossplane/crossplane/issues/152)

* Ease-of-use and improved experience
  * Standalone mode allowing Crossplane to run in a single container or process [#274](https://github.com/crossplane/crossplane/issues/274)

* [Reliability and production quality](https://github.com/crossplane/crossplane/labels/reliability)
  * Controllers recover failure conditions [#56](https://github.com/crossplane/crossplane/issues/56)
  * Controller High availability (HA) [#5](https://github.com/crossplane/crossplane/issues/5)
  * Core Infrastructure Initiative (CII) best practices [#58](https://github.com/crossplane/crossplane/issues/58)
* [Performance and Efficiency](https://github.com/crossplane/crossplane/labels/performance)
  * 2-way reconciliation with external resources [#290](https://github.com/crossplane/crossplane/issues/290)
  * Events/notifications from cloud provider on changes to external resources to trigger reconciliation [#289](https://github.com/crossplane/crossplane/issues/289)
