[![Build Status](https://jenkinsci.upbound.io/buildStatus/icon?job=crossplaneio/crossplane/build/master)](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplaneio%2Fcrossplane%2Fbuild/activity) [![GitHub release](https://img.shields.io/github/release/crossplaneio/crossplane/all.svg?style=flat-square)](https://github.com/crossplaneio/crossplane/releases) [![Docker Pulls](https://img.shields.io/docker/pulls/crossplane/crossplane.svg)](https://img.shields.io/docker/pulls/crossplane/crossplane.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/crossplaneio/crossplane)](https://goreportcard.com/report/github.com/crossplaneio/crossplane) [![Slack](https://slack.crossplane.io/badge.svg)](https://slack.crossplane.io) [![Twitter Follow](https://img.shields.io/twitter/follow/crossplane_io.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=crossplane_io&user_id=788180534543339520)

![Crossplane](docs/media/banner.png)

[Crossplane] is the open source multicloud control plane. With Crossplane you
can manage your applications and infrastructure across clouds, regions, and
clusters. Crossplane is composable and Kubernetes native. Deploy it to an
existing Kubernetes cluster to manage cloud infrastructure like databases,
buckets, and caches using `kubectl`. Deploy it standalone to manage Kubernetes
clusters, the workloads running on them, and the cloud infrastructure those
workloads depend upon.

## Architecture

![Architecture diagram](docs/media/crossplane-overview.png)

Crossplane builds on the Kubernetes control plane. It is composed of [Stacks] -
easy to install packages of Kubernetes [custom resources and controllers] that
extend Crossplane with new functionality. A stack can extend Crossplane with the
ability to manage the infrastructure of a new cloud provider, or to deploy and
manage a new cloud-native application. This enables Crossplane users to:

1. Install the **infrastructure stacks** for their desired clouds to enable
   on-demand, **cloud-portable provisioning of infrastructure** services like
   databases, caches, and Kubernetes clusters.
1. Define **portable application workloads**, including their cloud
   infrastructure dependencies, and schedule them to the most appropriate
   infrastructure across any cloud.
1. Package their workloads as an **application stack** that others may easily
   deploy.

The full vision and architecture of the Crossplane project is described in our
[architecture document].

## Get Started

Take a look at the [getting started] guide to learn how to install Crossplane,
install a stack, and provision cloud infrastructure using `kubectl`. Refer to
the Crossplane [documentation] for comprehensive guides to using Crossplane,
including deploying an application packaged as a stack.

## Get Involved

* Discuss Crossplane on [Slack] or our [developer mailing list].
* Follow us on [Twitter], or contact us via [Email].
* Join our regular community meetings.

The Crossplane community meeting takes place every other [Tuesday at 9:00am
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

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fcrossplaneio%2Fcrossplane.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fcrossplaneio%2Fcrossplane?ref=badge_large)

<!-- Named links -->

[Crossplane]: https://crossplane.io
[documentation]: https://crossplane.io/docs/latest
[Stacks]: docs/README.md#crossplane-stacks
[custom resources and controllers]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[architecture document]: https://docs.google.com/document/d/1whncqdUeU2cATGEJhHvzXWC9xdK29Er45NJeoemxebo/edit?usp=sharing
[Slack]: https://slack.crossplane.io
[developer mailing list]: https://groups.google.com/forum/#!forum/crossplane-dev
[Twitter]: https://twitter.com/crossplane_io
[Email]: mailto:info@crossplane.io
[issue against Crossplane]: https://github.com/crossplaneio/crossplane/issues
[contributing guide]: CONTRIBUTING.md
[Tuesday at 9:00am Pacific Time]: https://www.thetimezoneconverter.com/?t=9:00&tz=PT%20%28Pacific%20Time%29
[Current agenda and past meeting notes]: https://docs.google.com/document/d/1q_sp2jLQsDEOX7Yug6TPOv7Fwrys6EwcF5Itxjkno7Y/edit?usp=sharing
[Past meeting recordings]: https://www.youtube.com/playlist?list=PL510POnNVaaYYYDSICFSNWFqNbx1EMr-M
[getting started]: https://crossplane.io/docs/master/quick-start.html
