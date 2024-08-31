[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/3260/badge)](https://www.bestpractices.dev/projects/3260) ![CI](https://github.com/crossplane/crossplane/workflows/CI/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/crossplane/crossplane)](https://goreportcard.com/report/github.com/crossplane/crossplane)

![Crossplane](banner.png)

Crossplane is a framework for building cloud native control planes without
needing to write code. It has a highly extensible backend that enables you to
build a control plane that can orchestrate applications and infrastructure no
matter where they run, and a highly configurable frontend that puts you in
control of the schema of the declarative API it offers.

Crossplane is a [Cloud Native Computing Foundation][cncf] project.

## Get Started

Crossplane's [Get Started Docs] cover install and cloud provider quickstarts.

## Releases

[![GitHub release](https://img.shields.io/github/release/crossplane/crossplane/all.svg)](https://github.com/crossplane/crossplane/releases) [![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/crossplane)](https://artifacthub.io/packages/helm/crossplane/crossplane)

Currently maintained releases, as well as the next few upcoming releases are
listed below. For more information take a look at the Crossplane [release cycle
documentation].

| Release | Release Date  |   EOL    |
|:-------:|:-------------:|:--------:|
|  v1.15  | Feb 15, 2024  | Nov 2024 |
|  v1.16  | May 15, 2024  | Feb 2025 |
|  v1.17  | Aug 29, 2024  | May 2025 |
|  v1.18  | Early Nov '24 | Aug 2025 |
|  v1.19  | Early Feb '25 | Nov 2025 |
|  v1.20  | Early May '25 | Feb 2026 |

You can subscribe to the [community calendar] to track all release dates, and
find the most recent releases on the [releases] page.

## Roadmap

The public roadmap for Crossplane is published as a GitHub project board. Issues
added to the roadmap have been triaged and identified as valuable to the
community, and therefore a priority for the project that we expect to invest in.

The maintainer team regularly triages requests from the community to identify
features and issues of suitable scope and impact to include in this roadmap. The
community is encouraged to show their support for potential roadmap issues by
adding a :+1: reaction, leaving descriptive comments, and attending the
[regular community meetings] to discuss their requirements and use cases.

The maintainer team updates the roadmap on an as needed basis, in response to
demand, priority, and available resources. The public roadmap can be updated at
any time.

Milestones assigned to any issues in the roadmap are intended to give a sense of
overall priority and the expected order of delivery. They should be considered
approximate estimations and are **not** a strict commitment to a specific
delivery timeline.

[Crossplane Roadmap]

## Get Involved

[![Slack](https://img.shields.io/badge/slack-crossplane-red?logo=slack)](https://slack.crossplane.io) [![Twitter Follow](https://img.shields.io/twitter/follow/crossplane_io?logo=X&label=Follow&style=flat)](https://twitter.com/intent/follow?screen_name=crossplane_io&user_id=788180534543339520) [![YouTube Channel Subscribers](https://img.shields.io/youtube/channel/subscribers/UC19FgzMBMqBro361HbE46Fw)](https://www.youtube.com/@Crossplane)

Crossplane is a community driven project; we welcome your contribution. To file
a bug, suggest an improvement, or request a new feature please open an [issue
against Crossplane] or the relevant provider. Refer to our [contributing guide]
for more information on how you can help.

* Discuss Crossplane on [Slack] or our [developer mailing list].
* Follow us on [Twitter] or [LinkedIn], or subscribe to our [newsletter].
* Contact us via [Email].
* Join our regular community meetings.
* Provide feedback on our [roadmap and releases board].

The Crossplane community meeting takes place every 4 weeks on [Thursday at
10:00am Pacific Time][community meeting time]. You can find the up to date
meeting schedule on the [Community Calendar][community calendar].

Anyone who wants to discuss the direction of the project, design and
implementation reviews, or raise general questions with the broader community is
encouraged to join.

* Meeting link: <https://zoom.us/j/425148449?pwd=NEk4N0tHWGpEazhuam1yR28yWHY5QT09>
* [Current agenda and past meeting notes]
* [Past meeting recordings]
* [Community Calendar][community calendar]

### Special Interest Groups (SIG)

Each SIG collaborates in Slack and some groups have regular meetings, you can
find the meetings in the [Community Calendar][community calendar].
- [#sig-composition-environments][sig-composition-environments-slack]
- [#sig-composition-functions][sig-composition-functions-slack]
- [#sig-deletion-ordering][sig-deletion-ordering-slack]
- [#sig-devex][sig-devex-slack]
- [#sig-docs][sig-docs-slack]
- [#sig-e2e-testing][sig-e2e-testing-slack]
- [#sig-observability][sig-observability-slack]
- [#sig-observe-only][sig-observe-only-slack]
- [#sig-provider-families][sig-provider-families-slack]
- [#sig-secret-stores][sig-secret-stores-slack]
- [#sig-upjet][sig-upjet-slack]

## Adopters

A list of publicly known users of the Crossplane project can be found in [ADOPTERS.md].  We
encourage all users of Crossplane to add themselves to this list - we want to see the community's
growing success!

## License

Crossplane is under the Apache 2.0 license.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fcrossplane%2Fcrossplane?ref=badge_large)

<!-- Named links -->

[Crossplane]: https://crossplane.io
[release cycle documentation]: https://docs.crossplane.io/knowledge-base/guides/release-cycle
[install]: https://crossplane.io/docs/latest
[Slack]: https://slack.crossplane.io
[developer mailing list]: https://groups.google.com/forum/#!forum/crossplane-dev
[Twitter]: https://twitter.com/crossplane_io
[LinkedIn]: https://www.linkedin.com/company/crossplane/
[newsletter]: https://eepurl.com/ivy4v-/
[Email]: mailto:info@crossplane.io
[issue against Crossplane]: https://github.com/crossplane/crossplane/issues
[contributing guide]: contributing/README.md
[community meeting time]: https://www.thetimezoneconverter.com/?t=10:00&tz=PT%20%28Pacific%20Time%29
[Current agenda and past meeting notes]: https://docs.google.com/document/d/1q_sp2jLQsDEOX7Yug6TPOv7Fwrys6EwcF5Itxjkno7Y/edit?usp=sharing
[Past meeting recordings]: https://www.youtube.com/playlist?list=PL510POnNVaaYYYDSICFSNWFqNbx1EMr-M
[roadmap and releases board]: https://github.com/orgs/crossplane/projects/20/views/9?pane=info
[cncf]: https://www.cncf.io/
[Get Started Docs]: https://docs.crossplane.io/latest/getting-started/
[community calendar]: https://calendar.google.com/calendar/embed?src=c_2cdn0hs9e2m05rrv1233cjoj1k%40group.calendar.google.com
[releases]: https://github.com/crossplane/crossplane/releases
[ADOPTERS.md]: ADOPTERS.md
[regular community meetings]: https://github.com/crossplane/crossplane/blob/master/README.md#get-involved
[Crossplane Roadmap]: https://github.com/orgs/crossplane/projects/20/views/9?pane=info
[sig-composition-environments-slack]: https://crossplane.slack.com/archives/C05BP6QFLUW
[sig-composition-functions-slack]: https://crossplane.slack.com/archives/C031Y29CSAE
[sig-deletion-ordering-slack]: https://crossplane.slack.com/archives/C05BP8W5ALW
[sig-devex-slack]: https://crossplane.slack.com/archives/C05U1LLM3B2
[sig-docs-slack]: https://crossplane.slack.com/archives/C02CAQ52DPU
[sig-e2e-testing-slack]: https://crossplane.slack.com/archives/C05C8CCTVNV
[sig-observability-slack]: https://crossplane.slack.com/archives/C061GNH3LA0
[sig-observe-only-slack]: https://crossplane.slack.com/archives/C04D5988QEA
[sig-provider-families-slack]: https://crossplane.slack.com/archives/C056YAQRV16
[sig-secret-stores-slack]: https://crossplane.slack.com/archives/C05BY7DKFV2
[sig-upjet-slack]: https://crossplane.slack.com/archives/C05T19TB729
