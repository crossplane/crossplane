![CI](https://github.com/crossplane/crossplane/workflows/CI/badge.svg) [![GitHub release](https://img.shields.io/github/release/crossplane/crossplane/all.svg?style=flat-square)](https://github.com/crossplane/crossplane/releases) [![Docker Pulls](https://img.shields.io/docker/pulls/crossplane/crossplane.svg)](https://img.shields.io/docker/pulls/crossplane/crossplane.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/crossplane/crossplane)](https://goreportcard.com/report/github.com/crossplane/crossplane) [![Slack](https://slack.crossplane.io/badge.svg)](https://slack.crossplane.io) [![Twitter Follow](https://img.shields.io/twitter/follow/crossplane_io.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=crossplane_io&user_id=788180534543339520)

![Crossplane](docs/media/banner.png)

Crossplane is an open source Kubernetes add-on that transforms your cluster into
a **universal control plane**. Crossplane enables platform teams to assemble
infrastructure from multiple vendors, and expose higher level self-service APIs
for application teams to consume, without having to write any code.

Crossplane extends your Kubernetes cluster to support orchestrating any
infrastructure or managed service. Compose Crossplane's granular resources into
higher level abstractions that can be versioned, managed, deployed and consumed
using your favorite tools and existing processes. [Install Crossplane][install]
into any Kubernetes cluster to get started.

Crossplane is a [Cloud Native Compute Foundation][cncf] project.

## Releases

Currently maintained releases, as well as the next upcoming release are listed
below. For more information take a look at the Crossplane [release cycle
documentation].

| Release |  Current Patch  | Release Date |      EOL      |
|:-------:|:---------------:|:------------:|:-------------:|
|   v1.1  |     [v1.1.4]    |  Mar 3, 2021 |  August 2021  |
|   v1.2  |     [v1.2.4]    | Apr 27, 2021 | October 2021  |
|   v1.3  |     [v1.3.1]    | Jun 29, 2021 | December 2021 |
|   v1.4  |     Upcoming    | Aug 31, 2021 | February 2022 |

[v1.1.4]: https://github.com/crossplane/crossplane/releases/tag/v1.1.4
[v1.2.4]: https://github.com/crossplane/crossplane/releases/tag/v1.2.4
[v1.3.1]: https://github.com/crossplane/crossplane/releases/tag/v1.3.1

## Get Involved

Crossplane is a community driven project; we welcome your contribution. To file
a bug, suggest an improvement, or request a new feature please open an [issue
against Crossplane] or the relevant provider. Refer to our [contributing guide]
for more information on how you can help.

* Discuss Crossplane on [Slack] or our [developer mailing list].
* Follow us on [Twitter], or contact us via [Email].
* Join our regular community meetings.
* Provide feedback on our [roadmap].

The Crossplane community meeting takes place every other [Thursday at 10:00am
Pacific Time][community meeting time]. Anyone who wants to discuss the direction
of the project, design and implementation reviews, or raise general questions
with the broader community is encouraged to join.

* Meeting link: <https://zoom.us/j/425148449?pwd=NEk4N0tHWGpEazhuam1yR28yWHY5QT09>
* [Current agenda and past meeting notes]
* [Past meeting recordings]

## License

Crossplane is under the Apache 2.0 license.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane?ref=badge_large)

<!-- Named links -->

[Crossplane]: https://crossplane.io
[release cycle documentation]: https://crossplane.io/docs/master/reference/release-cycle.html
[install]: https://crossplane.io/docs/latest
[Slack]: https://slack.crossplane.io
[developer mailing list]: https://groups.google.com/forum/#!forum/crossplane-dev
[Twitter]: https://twitter.com/crossplane_io
[Email]: mailto:info@crossplane.io
[issue against Crossplane]: https://github.com/crossplane/crossplane/issues
[contributing guide]: CONTRIBUTING.md
[community meeting time]: https://www.thetimezoneconverter.com/?t=10:00&tz=PT%20%28Pacific%20Time%29
[Current agenda and past meeting notes]: https://docs.google.com/document/d/1q_sp2jLQsDEOX7Yug6TPOv7Fwrys6EwcF5Itxjkno7Y/edit?usp=sharing
[Past meeting recordings]: https://www.youtube.com/playlist?list=PL510POnNVaaYYYDSICFSNWFqNbx1EMr-M
[roadmap]: https://github.com/orgs/crossplane/projects/12
[cncf]: https://www.cncf.io/