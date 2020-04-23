[![Build Status](https://jenkinsci.upbound.io/buildStatus/icon?job=crossplane/crossplane/build/master)](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fcrossplane%2Fbuild/activity) [![GitHub release](https://img.shields.io/github/release/crossplane/crossplane/all.svg?style=flat-square)](https://github.com/crossplane/crossplane/releases) [![Docker Pulls](https://img.shields.io/docker/pulls/crossplane/crossplane.svg)](https://img.shields.io/docker/pulls/crossplane/crossplane.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/crossplane/crossplane)](https://goreportcard.com/report/github.com/crossplane/crossplane) [![Slack](https://slack.crossplane.io/badge.svg)](https://slack.crossplane.io) [![Twitter Follow](https://img.shields.io/twitter/follow/crossplane_io.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=crossplane_io&user_id=788180534543339520)

![Crossplane](docs/media/banner.png)

Crossplane is an open source control plane that allows you to manage
applications and infrastructure the Kubernetes way. It provides the following
features:

- Deployment and management of cloud provider managed services using the
  Kubernetes API.
- Management and scheduling of configuration data across multiple Kubernetes
  clusters.
- Separation of concern between infrastructure owners, application owners, and
  developers.
- Infrastructure agnostic packaging of applications and their dependencies.
- Scheduling applications into different clusters, zones, and regions.

Crossplane does not:

- Require that you run your workloads on Kubernetes.
- Manage the data plane across Kubernetes clusters.
- Manage or provision non-hosted Kubernetes clusters.

Crossplane can be [installed](docs/getting-started/install.md) into any Kubernetes cluster, and
is compatible with any Kubernetes-native project. It manages external services
by installing [Custom Resource
Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
(CRDs) and
[reconciling](https://kubernetes.io/docs/concepts/architecture/controller/)
instances of those Custom Resources. Crossplane is built to be extensible,
meaning that anyone can add functionality for an new or existing cloud provider.

Crossplane is comprised of four main components:

1. **Core Crossplane**: the set of Kubernetes CRDs and controllers that manage
   installation of `providers`, `stacks`, and `applications`, as well as the
   scheduling of configuration data to remote Kubernetes clusters.
2. **Providers**: the set of Kubernetes CRDs and controllers that provision and
   manage services on cloud providers. A cloud provider is any service that
   exposes infrastructure via an API.
    - Examples: [Google Cloud
      Platform](https://github.com/crossplane/provider-gcp), [Amazon Web
      Services](https://github.com/crossplane/provider-aws),
      [Azure](https://github.com/crossplane/provider-azure),
      [Alibaba](https://github.com/crossplane/provider-alibaba),
      [Github](https://github.com/crossplane/provider-github)
3. **Stacks**: a bundled set of custom resources that together represent an
   environment on a cloud provider. The bundle of instances can be created by a
   single custom resource.
   - Examples: [GCP Sample
     Stack](https://github.com/crossplane/stack-gcp-sample), [AWS Sample
     Stack](https://github.com/crossplane/stack-aws-sample), [Azure Sample
     Stack](https://github.com/crossplane/stack-azure-sample)
4. **Applications**: a deployable unit of code and configuration, which, when
   created, may involve provisioning new services which are managed by a
   `provider`, or consuming services created by a `stack`.
    - Examples: [Wordpress](https://github.com/crossplane/app-wordpress)

The full vision and architecture of the Crossplane project is described in our
[architecture document].

For more information, take a look at the official Crossplane [documentation].

## Get Involved

* Discuss Crossplane on [Slack] or our [developer mailing list].
* Follow us on [Twitter], or contact us via [Email].
* Join our regular community meetings.

The Crossplane community meeting takes place every other [Monday at 10:00am
Pacific Time]. Anyone who wants to discuss the direction of the project, design
and implementation reviews, or raise general questions with the broader
community is encouraged to join.

* Meeting link: https://zoom.us/j/425148449
* [Current agenda and past meeting notes]
* [Past meeting recordings]

Crossplane is a community driven project; we welcome your contribution. To file
a bug, suggest an improvement, or request a new feature please open an [issue
against Crossplane] or the relevant stack. Refer to our [contributing guide] for
more information on how you can help.

## License

Crossplane is under the Apache 2.0 license.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane?ref=badge_large)

<!-- Named links -->

[Crossplane]: https://crossplane.io
[documentation]: https://crossplane.io/docs/latest
[architecture document]: https://docs.google.com/document/d/1whncqdUeU2cATGEJhHvzXWC9xdK29Er45NJeoemxebo/edit?usp=sharing
[Slack]: https://slack.crossplane.io
[developer mailing list]: https://groups.google.com/forum/#!forum/crossplane-dev
[Twitter]: https://twitter.com/crossplane_io
[Email]: mailto:info@crossplane.io
[issue against Crossplane]: https://github.com/crossplane/crossplane/issues
[contributing guide]: CONTRIBUTING.md
[Monday at 10:00am Pacific Time]: https://www.thetimezoneconverter.com/?t=10:00&tz=PT%20%28Pacific%20Time%29
[Current agenda and past meeting notes]: https://docs.google.com/document/d/1q_sp2jLQsDEOX7Yug6TPOv7Fwrys6EwcF5Itxjkno7Y/edit?usp=sharing
[Past meeting recordings]: https://www.youtube.com/playlist?list=PL510POnNVaaYYYDSICFSNWFqNbx1EMr-M
