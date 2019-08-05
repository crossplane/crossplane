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

## v0.3 - Enable Partners to Build Infra Stacks

* Support for a few real-world applications on-top of Crossplane
  * GitLab [#284](https://github.com/crossplaneio/crossplane/issues/284)
  * More applications to follow

* Resource class enhancements: default classes, validation, annotation
  * Default resource classes - increases claim portability [#151](https://github.com/crossplaneio/crossplane/issues/151)
  * Resource classes can be validated and annotated [#613](https://github.com/crossplaneio/crossplane/issues/613)

* Infra stacks (out-of-tree) with single-region secure connectivity between k8s and DBaaS, Redis, Buckets 
  * Stacks Manager: app vs. infra stacks, namespace isolation, annotation support [#609](https://github.com/crossplaneio/crossplane/issues/609)
  * Move infra stacks (GCP, AWS, Azure) into separate repos & upgrade to kubebuilder2 [#612](https://github.com/crossplaneio/crossplane/issues/612)
  * GCP infra stack: single-region secure connectivity: GKE & CloudSQL, CloudMemorystore, Buckets [#615](https://github.com/crossplaneio/crossplane/issues/615)
  * AWS infra stack: single-region secure connectivity: EKS & RDS, ElastiCache, Buckets [#616](https://github.com/crossplaneio/crossplane/issues/616)
  * Azure infra stack: single-region secure connectivity: AKS & AzureSQL, AzureCache, Buckets[#617](https://github.com/crossplaneio/crossplane/issues/617)
  * Stacks v1 CLI / kubectl plugin: init, build, push commands [#614](https://github.com/crossplaneio/crossplane/issues/614)
* Docs & examples
  * Infra stack developer guide [#610](https://github.com/crossplaneio/crossplane/issues/610)
  * Portable wordpress app stack (kubebuilder-based) published to registry [#572](https://github.com/crossplaneio/crossplane/issues/572)
  * Refresh 0.3 Docs: reflect enhancements, better on-boarding UX, easier to get started[#625](https://github.com/crossplaneio/crossplane/issues/625)
  * DevOps pipeline examples for Jenkins, GitLab, GitOps using GCP and AWS stacks [#631](https://github.com/crossplaneio/crossplane/issues/631)

## Under Consideration

* On-Prem Support: Managed Services using the Kubernetes Operator Pattern [#283](https://github.com/crossplaneio/crossplane/issues/283)
  * Early support for Rook and others
  * Seamless integration into the Resource Claim and Resource Class model
* Heterogeneous application support
  * Serverless (functions) [#285](https://github.com/crossplaneio/crossplane/issues/285)
  * Containers and other Kubernetes deployment types (e.g., Helm charts) [#158](https://github.com/crossplaneio/crossplane/issues/158)
  * Virtual Machines [#286](https://github.com/crossplaneio/crossplane/issues/286)
* Enhanced Workload Scheduling
  * Region and cloud provider aware scheduling [#279](https://github.com/crossplaneio/crossplane/issues/279)
  * Delayed binding of resources to support co-location in same region [#156](https://github.com/crossplaneio/crossplane/issues/156)
  * Workloads declare their resource usage [#115](https://github.com/crossplaneio/crossplane/issues/115)
  * Optimization for many resource attributes [#287](https://github.com/crossplaneio/crossplane/issues/287)
  * Extensibility points to allow external scheduler integration [#288](https://github.com/crossplaneio/crossplane/issues/288)
* New Stateful managed services across AWS, Azure, and GCP
  * MongoDB [#280](https://github.com/crossplaneio/crossplane/issues/280)
* Auto-scaling
  * Cluster auto-scaler [#159](https://github.com/crossplaneio/crossplane/issues/159)
  * Node pools and worker nodes [#152](https://github.com/crossplaneio/crossplane/issues/152)
* Ease-of-use and improved experience
  * Standalone mode allowing Crossplane to run in a single container or process [#274](https://github.com/crossplaneio/crossplane/issues/274)
  * Uniform resource connectivity [#149](https://github.com/crossplaneio/crossplane/issues/149)
* Engineering
  * Consistent testing paradigm [#269](https://github.com/crossplaneio/crossplane/issues/269)

## Towards v1.0 - Production Ready

* Expand coverage of support
  * Cloud providers
  * Managed services
    * Message Queues [#281](https://github.com/crossplaneio/crossplane/issues/281)
    * Memcached [#282](https://github.com/crossplaneio/crossplane/issues/282)
* [Reliability and production quality](https://github.com/crossplaneio/crossplane/labels/reliability)
  * Controllers recover failure conditions [#56](https://github.com/crossplaneio/crossplane/issues/56)
  * Controller High availability (HA) [#5](https://github.com/crossplaneio/crossplane/issues/5)
  * Core Infrastructure Initiative (CII) best practices [#58](https://github.com/crossplaneio/crossplane/issues/58)
* [Performance and Efficiency](https://github.com/crossplaneio/crossplane/labels/performance)
  * 2-way reconciliation with external resources [#290](https://github.com/crossplaneio/crossplane/issues/290)
  * Events/notifications from cloud provider on changes to external resources to trigger reconciliation [#289](https://github.com/crossplaneio/crossplane/issues/289)
