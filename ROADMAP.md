# Roadmap

This document defines a high level roadmap for Crossplane development and upcoming releases. Community and contributor involvement is vital for successfully implementing all desired items for each release. We hope that the items listed below will inspire further engagement from the community to keep Crossplane progressing and shipping exciting and valuable features.

Any dates listed below and the specific issues that will ship in a given milestone are subject to change but should give a general idea of what we are planning. We use the [milestone](https://github.com/crossplaneio/crossplane/milestones) feature in Github so look there for the most up-to-date and issue plan.

## [v0.1 - Proof of Concept](https://github.com/crossplaneio/crossplane/releases/tag/v0.1.0)

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

## [v0.2 - Workload Scheduling, Expand Supported Resources](https://github.com/crossplaneio/crossplane/releases/tag/v0.2.0)

* Workload Scheduling
  * Design for smart scheduler, optimization, resource placement [#278](https://github.com/crossplaneio/crossplane/issues/278)
  * Basic workload scheduler with cluster selector [#309](https://github.com/crossplaneio/crossplane/issues/309)
  * Update workload propagation to avoid  collisions on target cluster [#308](https://github.com/crossplaneio/crossplane/pull/308)
  * Minimize workload deployment kubeconfig settings for AKS to be consistent with GKE, EKS [#273](https://github.com/crossplaneio/crossplane/issues/273)
  * Update workload deployment docs [#239](https://github.com/crossplaneio/crossplane/issues/239)

* New Stateful managed services across AWS, Azure, and GCP
  * Database: PostgreSQL [#54](https://github.com/crossplaneio/crossplane/issues/54), MySQL [#53](https://github.com/crossplaneio/crossplane/issues/53)
  * Cache / Redis [#137](https://github.com/crossplaneio/crossplane/issues/137), [#282](https://github.com/crossplaneio/crossplane/issues/282)
  * Buckets [#295](https://github.com/crossplaneio/crossplane/issues/295), [#109](https://github.com/crossplaneio/crossplane/issues/109)

* Performance and Efficiency
  * Reconciliation requeue pattern [#241](https://github.com/crossplaneio/crossplane/issues/241)

* UX Enhancements
  * Enhanced kubectl printer columns [#38](https://github.com/crossplaneio/crossplane/issues/38)

* Engineering
  * General resource controller used for more types [#276](https://github.com/crossplaneio/crossplane/issues/276)
  * Controllers use consistent logging [#7](https://github.com/crossplaneio/crossplane/issues/7)
  * Consistent testing paradigm [#269](https://github.com/crossplaneio/crossplane/issues/269)

## [v0.3 - Enable Community to Build Infra Stacks](https://github.com/crossplaneio/crossplane/releases/tag/v0.3.0)

* Real-world applications on-top of Crossplane
  * GitLab [#284](https://github.com/crossplaneio/crossplane/issues/284)
  * More applications to follow

* Resource Class enhancements: default classes, validation, annotation
  * Default resource classes - increases claim portability [#151](https://github.com/crossplaneio/crossplane/issues/151)
  * Resource classes can be validated and annotated [#613](https://github.com/crossplaneio/crossplane/issues/613)

* Infra Stacks (out-of-tree) with single-region secure connectivity between k8s and DBaaS, Redis, Buckets
  * Stacks Manager: App vs. Infra Stacks, namespace isolation, annotation support [#609](https://github.com/crossplaneio/crossplane/issues/609)
  * Move Infra Stacks (GCP, AWS, Azure) into separate repos & upgrade to kubebuilder2 [#612](https://github.com/crossplaneio/crossplane/issues/612)
  * GCP Infra Stack: single-region secure connectivity: GKE & CloudSQL, CloudMemorystore, Buckets [#615](https://github.com/crossplaneio/crossplane/issues/615)
  * AWS Infra Stack: single-region secure connectivity: EKS & RDS, ElastiCache, Buckets [#616](https://github.com/crossplaneio/crossplane/issues/616)
  * Azure Infra Stack: single-region secure connectivity: AKS & AzureSQL, AzureCache, Buckets [#617](https://github.com/crossplaneio/crossplane/issues/617)
  * Stacks v1 CLI / kubectl plugin: init, build, push commands [#614](https://github.com/crossplaneio/crossplane/issues/614)

* Docs & examples
  * Infra Stack Developer Guide [#610](https://github.com/crossplaneio/crossplane/issues/610)
  * Portable Wordpress App Stack (kubebuilder-based) published to registry [#572](https://github.com/crossplaneio/crossplane/issues/572)
  * Refresh 0.3 Docs: reflect enhancements, better on-boarding UX, easier to get started [#625](https://github.com/crossplaneio/crossplane/issues/625)
  * Crossplane.io reflects the updated roadmap / vision [crossplaneio.github.io#22](https://github.com/crossplaneio/crossplaneio.github.io/issues/22)

## [v0.4.0 Initial Rook support & stable v1beta1 APIs for AWS, GCP](https://github.com/crossplaneio/crossplane/releases/tag/v0.4.0)
* Claim-based provisioning of [Rook](https://rook.io/)-managed databases [#862](https://github.com/crossplaneio/crossplane/issues/862)
  * Support for CockroachDB and Yugabyte DB

* Stable v1beta1 Services APIs for managed databases and caches (GCP, AWS) [#863](https://github.com/crossplaneio/crossplane/issues/863)
  * Align on shape of APIs & best practices
    * Beta meta model w/ DB & Redis, so users can deploy to dev/test/prod
    * Naming scheme for all resources.
    * Managed resource name as external name for all resources.
  * Upgrade GCP stack to v1beta1: CloudSQL and CloudMemoryInstance with high-def CRDs & controllers
  * Upgrade AWS stack to v1beta1: RDS and ReplicationGroup with high-def CRDs & controllers

* Cross-resource referencing for networks, subnets, and other resources [#707](https://github.com/crossplaneio/crossplane/issues/707) 
  * Support `kubectl apply -f` for a directory of resources to cleanly support GitOps for both infrastructure and apps
  * Sample infra and app repos you can `kubectl apply -f` and have a working environment quickly
    * infrastructure (networks, subnets, managed k8s cluster, resource classes for databases, etc.)
    * apps (e.g. kubernetes core resources for e.g. a Wordpress app plus the resource claims for managed service dependencies
  * Update crossplane.io services guides and stacks guides to use `kubectl apply -f` technique

 * Release automation for shorter release cycles and hot fixes [#864](https://github.com/crossplaneio/crossplane/issues/864) 
   * Updating pipelines to include automation [#6](https://github.com/crossplaneio/crossplane/issues/6)
   * SonarCloud checks for cloud provider stacks [#875](https://github.com/crossplaneio/crossplane/issues/875)
   * crossplane-runtime build pipelines [crossplaneio/crossplane-runtime#14](https://github.com/crossplaneio/crossplane-runtime/issues/14)

 * Trace utility for enhanced debugging support. [#744](https://github.com/crossplaneio/crossplane/issues/744)

 * Simple Resource Class Selection [#952](https://github.com/crossplaneio/crossplane/issues/952)

 * Crossplane supporting work for GitLab 12.5 Auto DevOps [#867](https://github.com/crossplaneio/crossplane/issues/867)

## [v0.5.0 Continous deployment for GitLab and ArgoCD with v1beta1 APIs](https://github.com/crossplaneio/crossplane/releases/tag/v0.5.0)
* GitLab 12.5 Auto DevOps (ADO) integration phase 1 - provision managed PostgreSQL from GitLab ADO pipelines
  * Subset of the overall [GitLab Auto DevOps integration](https://gitlab.com/groups/gitlab-org/-/epics/1866#note_216080986) 
  * [Crossplane as a GitLab-managed app (phase1)](https://gitlab.com/gitlab-org/gitlab/issues/34702) - provision managed PostgreSQL from GitLab ADO pipelines

 * CD integration examples ArgoCD [#631](https://github.com/crossplaneio/crossplane/issues/631)

* Stable v1beta1 Services APIs for managed databases and caches (Azure) [#863](https://github.com/crossplaneio/crossplane/issues/863)
  * Upgrade Azure stack to v1beta1: Azure Database and Azure Cache for Redis with high-def CRDs & controllers
    * crossplaneio/stack-azure#28 Azure SQL and Redis resources v1beta1

* Bug fixes and test automation

## [v0.6.0 Aggregated Stack Roles, GKECluster to v1beta1, test automation](https://github.com/crossplaneio/crossplane/releases/tag/v0.6.0)
* The Stack Manager supports more granular management of permissions for cluster (environment) and namespace (workspace) scoped stacks.
  * Default admin, editor, and viewer roles automatically updated as Stacks are installed/uninstalled.
  * Admins can create role bindings to these roles, to simplify granting user permissions.
  * Details in the [design doc](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-stacks-security-isolation.md).
* GKE cluster support has moved to `v1beta1` with node pool support.
  * The `v1alpha3` GKE cluster support has been left intact and can run side by side with v1beta1 
* Integration test framework in the crossplane-runtime, reducing the burden to provide integration test coverage across all projects and prevent regressions.
* Helm 2 and 3 compatibility, Crossplane and all of its CRDs are supported to be installed by both Helm2 and Helm3
* Design and architecture documents:
  * ["Easy config stacks"](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-packs.md)
  * [Consuming any Kubernetes cluster for workload scheduling](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-consuming-k8s-clusters.md)
  * [User experience for template stacks](https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-template-stacks-experience.md).
* [Bug fixes and other closed issues](https://github.com/crossplaneio/crossplane/milestone/6?closed=1)

## v0.7.0
* KubernetesTarget kind for scheduling KubernetesApplications [#859](https://github.com/crossplaneio/crossplane/issues/859)
* Versioning and upgrade support [#879](https://github.com/crossplaneio/crossplane/issues/879)
  * Design one-pager [#435](https://github.com/crossplaneio/crossplane/issues/435)
* Template Stacks - easier to build App & Config Stacks (Preview) [#853](https://github.com/crossplaneio/crossplane/issues/853) 
* GCP storage buckets to v1beta1 [crossplaneio/stack-gcp#130](https://github.com/crossplaneio/stack-gcp/issues/130)
* GCP networking resources to v1beta1 [crossplaneio/stack-gcp#131](https://github.com/crossplaneio/stack-gcp/issues/131)
* Improved logging and eventing [crossplaneio/crossplane-runtime#104](https://github.com/crossplaneio/crossplane-runtime/issues/104)
* Integration testing
  * Integration testing support [#1033](https://github.com/crossplaneio/crossplane/issues/1033)
  * GCP integration tests [crossplaneio/stack-gcp#87](https://github.com/crossplaneio/stack-gcp/issues/87)

## Roadmap
* Stacks Manager
  * Versioning and upgrade [#879](https://github.com/crossplaneio/crossplane/issues/879) 

* GCP: DNS, SSL, and Ingress support #1123 [#1123](https://github.com/crossplaneio/crossplane/issues/1123)

* More real-world Stacks into multiple clouds
  * Refresh existing GitLab Stack to use latest Crossplane [#866](https://github.com/crossplaneio/crossplane/issues/866)
  * Additional real-world apps and scenarios [#868](https://github.com/crossplaneio/crossplane/issues/868)
  * Stacks Manager support for private repos and robot account credentials

* UX enhancements for debuggability and observability
  * Visible error messages for all error cases surfaced in claims and/or eventing
  * Static provisioning examples to highlight simplicity. 

 * v1beta1 Services APIs
   * Incorporate beta1 feedback
   * Upgrade other supported services to v1beta1 (e.g. Buckets, etc.)
   * Code generation of API types, controller scaffolding to further streamline additional services

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
  * Region and cloud provider aware scheduling [#279](https://github.com/crossplaneio/crossplane/issues/279)
  * Delayed binding of resources to support co-location in same region [#156](https://github.com/crossplaneio/crossplane/issues/156)
  * Workloads declare their resource usage [#115](https://github.com/crossplaneio/crossplane/issues/115)
  * Optimization for many resource attributes [#287](https://github.com/crossplaneio/crossplane/issues/287)
  * Extensibility points to allow external scheduler integration [#288](https://github.com/crossplaneio/crossplane/issues/288)

* Heterogeneous application support
  * Serverless (functions) [#285](https://github.com/crossplaneio/crossplane/issues/285)
  * Containers and other Kubernetes deployment types (e.g., Helm charts) [#158](https://github.com/crossplaneio/crossplane/issues/158)
  * Virtual Machines [#286](https://github.com/crossplaneio/crossplane/issues/286)

* New Stateful managed services across AWS, Azure, and GCP
  * MongoDB [#280](https://github.com/crossplaneio/crossplane/issues/280)
  * Message Queues [#281](https://github.com/crossplaneio/crossplane/issues/281)

* Auto-scaling
  * Cluster auto-scaler [#159](https://github.com/crossplaneio/crossplane/issues/159)
  * Node pools and worker nodes [#152](https://github.com/crossplaneio/crossplane/issues/152)

* Ease-of-use and improved experience
  * Standalone mode allowing Crossplane to run in a single container or process [#274](https://github.com/crossplaneio/crossplane/issues/274)

* [Reliability and production quality](https://github.com/crossplaneio/crossplane/labels/reliability)
  * Controllers recover failure conditions [#56](https://github.com/crossplaneio/crossplane/issues/56)
  * Controller High availability (HA) [#5](https://github.com/crossplaneio/crossplane/issues/5)
  * Core Infrastructure Initiative (CII) best practices [#58](https://github.com/crossplaneio/crossplane/issues/58)
* [Performance and Efficiency](https://github.com/crossplaneio/crossplane/labels/performance)
  * 2-way reconciliation with external resources [#290](https://github.com/crossplaneio/crossplane/issues/290)
  * Events/notifications from cloud provider on changes to external resources to trigger reconciliation [#289](https://github.com/crossplaneio/crossplane/issues/289)
