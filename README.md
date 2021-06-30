![CI](https://github.com/crossplane/crossplane/workflows/CI/badge.svg) [![GitHub release](https://img.shields.io/github/release/crossplane/crossplane/all.svg?style=flat-square)](https://github.com/crossplane/crossplane/releases) [![Docker Pulls](https://img.shields.io/docker/pulls/crossplane/crossplane.svg)](https://img.shields.io/docker/pulls/crossplane/crossplane.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/crossplane/crossplane)](https://goreportcard.com/report/github.com/crossplane/crossplane) [![Slack](https://slack.crossplane.io/badge.svg)](https://slack.crossplane.io) [![Twitter Follow](https://img.shields.io/twitter/follow/crossplane_io.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=crossplane_io&user_id=788180534543339520)

![Crossplane](docs/media/banner.png)

Crossplane is an open source Kubernetes add-on that enables platform teams to
assemble infrastructure from multiple vendors, and expose higher level
self-service APIs for application teams to consume. Crossplane effectively
enables platform teams to quickly put together their own opinionated platform
declaratively without having to write any code, and offer it to their
application teams as a self-service Kubernetes-style declarative API.

Both the higher level abstractions as well as the granular resources they are
composed of are represented simply as objects in the Kubernetes API, meaning
they can all be provisioned and managed by kubectl, GitOps, or any tools that
can talk with the Kubernetes API. To facilitate reuse and sharing of these APIs,
Crossplane supports packaging them in a standard OCI image and distributing via
any compliant registry.

Platform engineers are able to define organizational policies and guardrails
behind these self-service API abstractions. The developer is presented with the
limited set of configuration that they need to tune for their use-case and is
not exposed to any of the complexities of the low-level infrastructure below the
API. Access to these APIs is managed with Kubernetes-native RBAC, thus enabling
the level of permissioning to be at the level of abstraction.

While extending the Kubernetes control plane with a diverse set of vendors,
resources, and abstractions, Crossplane recognized the need for a single
consistent API across all of them. To this end, we have created the Crossplane
Resource Model (XRM). XRM extends the Kubernetes Resource Model (KRM) in an
opinionated way, resulting in a universal experience for managing resources,
regardless of where they reside. When interacting with the XRM, things like
credentials, workload identity, connection secrets, status conditions, deletion
policies, and references to other resources work the same no matter what
provider or level of abstraction they are a part of.

The functionality and value of the Crossplane project can be summarized at a
very high level by these two main areas:

1. Enabling infrastructure owners to build custom platform abstractions (APIs)
   composed of granular resources that allow developer self-service and service
   catalog use cases
2. Providing a universal experience for managing infrastructure, resources, and
   abstractions consistently across all vendors and environments in a uniform
   way, called the Crossplane Resource Model (XRM)

## Releases

Currently maintained releases, as well as the next upcoming release are listed
below. For more information take a look at the Crossplane [release cycle
documentation].

| Release |  Current Patch  | Release Date |      EOL      |
|:-------:|:---------------:|:------------:|:-------------:|
|   v1.1  |     [v1.1.3]    |  Mar 3, 2021 |  August 2021  |
|   v1.2  |     [v1.2.3]    | Apr 27, 2021 | October 2021  |
|   v1.3  |     [v1.3.0]    | Jun 29, 2021 | December 2021 |
|   v1.4  |     Upcoming    | Aug 31, 2021 | February 2022 |

[v1.1.3]: https://github.com/crossplane/crossplane/releases/tag/v1.1.3
[v1.2.3]: https://github.com/crossplane/crossplane/releases/tag/v1.2.3
[v1.3.0]: https://github.com/crossplane/crossplane/releases/tag/v1.3.0

## Getting Started

Take a look at the [documentation] to get started.

## Get Involved

* Discuss Crossplane on [Slack] or our [developer mailing list].
* Follow us on [Twitter], or contact us via [Email].
* Join our regular community meetings.
* Provide feedback on our [roadmap](ROADMAP.md).

The Crossplane community meeting takes place every other [Thursday at 10:00am
Pacific Time][community meeting time]. Anyone who wants to discuss the direction
of the project, design and implementation reviews, or raise general questions
with the broader community is encouraged to join.

* Meeting link: <https://zoom.us/j/425148449?pwd=NEk4N0tHWGpEazhuam1yR28yWHY5QT09>
* [Current agenda and past meeting notes]
* [Past meeting recordings]

Crossplane is a community driven project; we welcome your contribution. To file
a bug, suggest an improvement, or request a new feature please open an [issue
against Crossplane] or the relevant provider. Refer to our [contributing guide]
for more information on how you can help.

## License

Crossplane is under the Apache 2.0 license.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane?ref=badge_large)

<!-- Named links -->

[Crossplane]: https://crossplane.io
[release cycle documentation]: https://crossplane.io/docs/master/reference/release-cycle.html
[documentation]: https://crossplane.io/docs/latest
[Slack]: https://slack.crossplane.io
[developer mailing list]: https://groups.google.com/forum/#!forum/crossplane-dev
[Twitter]: https://twitter.com/crossplane_io
[Email]: mailto:info@crossplane.io
[issue against Crossplane]: https://github.com/crossplane/crossplane/issues
[contributing guide]: CONTRIBUTING.md
[community meeting time]: https://www.thetimezoneconverter.com/?t=10:00&tz=PT%20%28Pacific%20Time%29
[Current agenda and past meeting notes]: https://docs.google.com/document/d/1q_sp2jLQsDEOX7Yug6TPOv7Fwrys6EwcF5Itxjkno7Y/edit?usp=sharing
[Past meeting recordings]: https://www.youtube.com/playlist?list=PL510POnNVaaYYYDSICFSNWFqNbx1EMr-M
