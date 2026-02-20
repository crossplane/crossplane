[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/3260/badge)](https://www.bestpractices.dev/projects/3260) ![CI](https://github.com/crossplane/crossplane/workflows/CI/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/crossplane/crossplane)](https://goreportcard.com/report/github.com/crossplane/crossplane)

![Crossplane](banner.png)

Crossplane is a framework for building cloud native control planes without
needing to write code. It has a highly extensible backend that enables you to
build a control plane that can orchestrate applications and infrastructure no
matter where they run, and a highly configurable frontend that puts you in
control of the schema of the declarative API it offers.

Crossplane is a [Cloud Native Computing Foundation][cncf] project.

## Get Started

Crossplane's [Get Started Docs] covers install and resource quickstarts.

## Releases

[![GitHub release](https://img.shields.io/github/release/crossplane/crossplane/all.svg)](https://github.com/crossplane/crossplane/releases) [![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/crossplane)](https://artifacthub.io/packages/helm/crossplane/crossplane)

Currently maintained releases, as well as the next few upcoming releases are
listed below. For more information take a look at the Crossplane [release cycle
documentation].

| Release | Release Date  |   EOL    |
|:-------:|:-------------:|:--------:|
|  v1.20  | May 21, 2025  | TBD      |
|  v2.0   |  Aug 8, 2025  | May 2026 |
|  v2.1   |  Nov 5, 2025  | Aug 2026 |
|  v2.2   | Early Feb '26 | Nov 2026 |
|  v2.3   | Early May '26 | Feb 2027 |
|  v2.4   | Early Aug '26 | May 2027 |

You can subscribe to the [community calendar] to track all release dates, and
find the most recent releases on the [releases] page.

The release process is fully documented in the [`crossplane/release`] repo.

### Extended v1.20 Support

Crossplane v1.20 is receiving extended support as the final minor release of the
v1 major version. Its EOL date is yet to be determined. Until that time, it will
continue to receive critical fixes only.

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

[![Slack](https://img.shields.io/badge/slack-crossplane-red?logo=slack)](https://slack.crossplane.io) [![Bluesky Follow](https://img.shields.io/badge/bluesky-Follow-blue?logo=bluesky)](https://bsky.app/profile/crossplane.io) [![Twitter Follow](https://img.shields.io/twitter/follow/crossplane_io?logo=X&label=Follow&style=flat)](https://twitter.com/intent/follow?screen_name=crossplane_io&user_id=788180534543339520) [![YouTube Channel Subscribers](https://img.shields.io/youtube/channel/subscribers/UC19FgzMBMqBro361HbE46Fw)](https://www.youtube.com/@Crossplane)

Crossplane is a community driven project; we welcome your contribution. To file
a bug, suggest an improvement, or request a new feature please open an [issue
against Crossplane] or the relevant provider. Refer to our [contributing guide]
for more information on how you can help.

* Discuss Crossplane on [Slack].
* Follow us on [Bluesky], [Twitter], or [LinkedIn].
* Contact us via [Email].
* Join our regular community meetings.
* Provide feedback on our [roadmap and releases board].

The Crossplane community meeting takes place every 4 weeks on [Thursday at
10:00am Pacific Time][community meeting time]. You can find the up to date
meeting schedule on the [Community Calendar][community calendar].

Anyone who wants to discuss the direction of the project, design and
implementation reviews, or raise general questions with the broader community is
encouraged to join.

* Meeting link: <https://zoom-lfx.platform.linuxfoundation.org/meeting/98901510164?password=c60c41ae-1e1e-42d0-9a74-16de2fbb66f9>
* [Current agenda and past meeting notes]
* [Past meeting recordings]
* [Community Calendar][community calendar]

### Special Interest Groups (SIG)

The Crossplane project supports SIGs as discussion groups that bring together
community members with shared interests. SIGs have no decision making authority
or ownership responsibilities. They serve purely as collaborative forums for
community discussion.

If you're interested in any of the areas below, consider joining the discussion
in their Slack channels. To propose a new SIG that isn't represented, reach out
through any of the contact methods in the [get involved] section.

Each SIG collaborates primarily in Slack, and some groups hold regular meetings
that you can find in the [Community Calendar][community calendar].

- [#sig-cli][sig-cli]
- [#sig-composition-environments][sig-composition-environments-slack]
- [#sig-composition-functions][sig-composition-functions-slack]
- [#sig-deletion-ordering][sig-deletion-ordering-slack]
- [#sig-devex][sig-devex-slack]
- [#sig-docs][sig-docs-slack]
- [#sig-e2e-testing][sig-e2e-testing-slack]
- [#sig-observability][sig-observability-slack]
- [#sig-observe-only][sig-observe-only-slack]
- [#sig-prod-readiness][sig-prod-readiness-slack]
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
[Bluesky]: https://bsky.app/profile/crossplane.io
[Twitter]: https://twitter.com/crossplane_io
[LinkedIn]: https://www.linkedin.com/company/crossplane/
[Email]: mailto:crossplane-info@lists.cncf.io
[issue against Crossplane]: https://github.com/crossplane/crossplane/issues
[contributing guide]: contributing/README.md
[community meeting time]: https://www.thetimezoneconverter.com/?t=10:00&tz=PT%20%28Pacific%20Time%29
[Current agenda and past meeting notes]: https://docs.google.com/document/d/1q_sp2jLQsDEOX7Yug6TPOv7Fwrys6EwcF5Itxjkno7Y/edit?usp=sharing
[Past meeting recordings]: https://www.youtube.com/playlist?list=PL510POnNVaaYYYDSICFSNWFqNbx1EMr-M
[roadmap and releases board]: https://github.com/orgs/crossplane/projects/20/views/9?pane=info
[cncf]: https://www.cncf.io/
[Get Started Docs]: https://docs.crossplane.io/latest/get-started/get-started-with-composition
[community calendar]: https://zoom-lfx.platform.linuxfoundation.org/meetings/crossplane?view=month
[releases]: https://github.com/crossplane/crossplane/releases
[`crossplane/release`]: https://github.com/crossplane/release
[ADOPTERS.md]: ADOPTERS.md
[regular community meetings]: https://github.com/crossplane/crossplane/blob/main/README.md#get-involved
[Crossplane Roadmap]: https://github.com/orgs/crossplane/projects/20/views/9?pane=info
[get involved]: https://github.com/crossplane/crossplane/blob/main/README.md#get-involved
[sig-cli]: https://crossplane.slack.com/archives/C08V9PMLRQA
[sig-composition-environments-slack]: https://crossplane.slack.com/archives/C05BP6QFLUW
[sig-composition-functions-slack]: https://crossplane.slack.com/archives/C031Y29CSAE
[sig-deletion-ordering-slack]: https://crossplane.slack.com/archives/C05BP8W5ALW
[sig-devex-slack]: https://crossplane.slack.com/archives/C05U1LLM3B2
[sig-docs-slack]: https://crossplane.slack.com/archives/C02CAQ52DPU
[sig-e2e-testing-slack]: https://crossplane.slack.com/archives/C05C8CCTVNV
[sig-observability-slack]: https://crossplane.slack.com/archives/C061GNH3LA0
[sig-observe-only-slack]: https://crossplane.slack.com/archives/C04D5988QEA
[sig-prod-readiness-slack]: https://crossplane.slack.com/archives/C0AEYUFP9GQ
[sig-provider-families-slack]: https://crossplane.slack.com/archives/C056YAQRV16
[sig-secret-stores-slack]: https://crossplane.slack.com/archives/C05BY7DKFV2
[sig-upjet-slack]: https://crossplane.slack.com/archives/C05T19TB729
