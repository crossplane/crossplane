# Project Conductor (codename)

## What is Conductor?

Conductor is an open source **external-resource-definition** for Kubernetes , providing the platform, framework, and support for a diverse set of the managed resources offered by major cloud providers (Currently focused on AWS and GCP)

Conductor turns storage software into self-managing, self-scaling, and self-healing of managed cloud resources. Conductor extends the facilities provided by Kubernetes such container management, scheduling and orchestration to the external resources.

Conductor integrates deeply into cloud native environments leveraging extension points and providing a seamless experience for scheduling, lifecycle management, resource management, security, monitoring, and user experience.

For more details about the cloud providers and resources currently supported by Conductor, please refer to the [project status section](#project-status) below.
We plan to continue adding support for other cloud providers and resource based on community demand and engagement in future releases. See our [roadmap](ROADMAP.md) for more details.

## Contributing

We welcome contributions. See [Contributing](CONTRIBUTING.md) to get started.

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please open an [issue](https://github.com/upbound/conductor/issues).

## Project Status

The status of each storage provider supported by Conductor can be found in the table below.
Each API group is assigned its own individual status to reflect their varying maturity and stability.
More details about API versioning and status in Kubernetes can be found on the Kubernetes [API versioning page](https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning), but the key difference between the statuses are summarized below:

* **Alpha:** The API may change in incompatible ways in a later software release without notice, recommended for use only in short-lived testing clusters, due to increased risk of bugs and lack of long-term support.
* **Beta:** Support for the overall features will not be dropped, though details may change. Support for upgrading or migrating between versions will be provided, either through automation or manual steps.
* **Stable:** Features will appear in released software for many subsequent versions and support for upgrading between versions will be provided with software automation in the vast majority of scenarios.


Name | Details | API Group | Status
-----|---------|-----------|-------

### Official Releases

Official releases of Conductor can be found on the [releases page](https://github.com/upbound/conductor/releases).
Please note that it is **strongly recommended** that you use [official releases](https://github.com/upbound/concuctor/releases) of Conductor, as unreleased versions from the master branch are subject to changes and incompatibilities that will not be supported in the official releases.
Builds from the master branch can have functionality changed and even removed at any time without compatibility support and without prior notice.

## Licensing

Conductor is under the Apache 2.0 license.
