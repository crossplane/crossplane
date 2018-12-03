# Roadmap

This document defines a high level roadmap for Crossplane development and upcoming releases. Community and contributor involvement is vital for successfully implementing all desired items for each release. We hope that the items listed below will inspire further engagement from the community to keep Crossplane progressing and shipping exciting and valuable features.

Any dates listed below and the specific issues that will ship in a given milestone are subject to change but should give a general idea of what we are planning. We use the [milestone](https://github.com/crossplaneio/crossplane/milestones) feature in Github so look there for the most up-to-date and issue plan.

## v0.1 - Proof of Concept

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

## v0.2 - Run Real-world Applications

* Support for a few realworld applications on-top of Crossplane
  * GitLab and others coming
* Workload Scheduling
  * Region and cloud provider aware scheduling
  * Delayed binding of resources to support co-location in same region
* Resource Pool and Auto-Scalers
  * Support for automatically creating Kubernetes Clusters
* New Stateful managed services across AWS, Azure, and GCP 
  * PostgresSQL
  * MongoDB
  * Redis
  * ActiveMQ
  * Memcached
* Managed Services using the Kubernetes Operator Pattern
  * Support for running Managed Services based on the Operator pattern
  * Early support for Rook and others
  * Seamless integration into the Resource Claim and Resource Class model
* Performance and Efficiency
  * 2-way reconciliation with external resources
  * Events/notifications from cloud provider on changes to external resources to trigger reconciliation
  * Parallel processing of CRD instances
