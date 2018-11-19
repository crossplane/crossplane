# Roadmap

This document defines a high level roadmap for Crossplane development and upcoming releases.
The features and themes included in each milestone are optimistic in the sense that many do not have clear owners yet.
Community and contributor involvement is vital for successfully implementing all desired items for each release.
We hope that the items listed below will inspire further engagement from the community to keep Crossplane progressing and shipping exciting and valuable features.

Any dates listed below and the specific issues that will ship in a given milestone are subject to change but should give a general idea of what we are planning.
We use the [milestone](https://github.com/crossplaneio/crossplane/milestones) feature in Github so look there for the most up-to-date and issue plan.

## v0.1

* MySQL support for AWS, GCP, and Azure
  * Provider CRDs, credentials management, API/SDK consumption
  * Provider specific MySQL CRDs (Amazon RDS, Google Cloud SQL, Microsoft Azure Database for MySQL)
  * All 3 big cloud providers will be supported for all resources going forward
* PostgreSQL support for AWS, GCP, and Azure
  * same work items as MySQL support
* Controller depth and reliability
  * Full CRUD support for all resources (robust lifecycle management)
  * CRD status Conditions for status of resources
  * Event recording
  * Normalized logging using single logging solution (with configurable levels)
  * Retry/recovery from failure, idempotence, dealing with partial state
* CI builds/tests/releases
  * New isolated jenkins instance (similar to Rook's jenkins)
  * Developer unit testing with high code coverage
  * Integration testing pipeline
  * Artifact publishing (container images, crossplane helm chart, etc.)
* Documentation
  * User guides, quick-starts, walkthroughs
  * Godocs developer docs for source code/packages/libraries
* Open source project management
  * [CII best practices checklist](https://bestpractices.coreinfrastructure.org/en/projects/1599#)
  * Governance
  * Contributor License Agreement (CLA) or Developer Certificate of Origin (DCO)

## v0.2

* Support for other SockShop resources
  * MongoDB
  * Redis
  * RabbitMQ
* Support for Clusters, deploy and manage clusters lifecycle via CRDs
* Federation, deploy resources to target clusters (possibly a new project)
* Performance and Efficiency
  * 2-way reconciliation with external resources
  * Events/notifications from cloud provider on changes to external resources to trigger reconciliation
  * Parallel processing of CRD instances
