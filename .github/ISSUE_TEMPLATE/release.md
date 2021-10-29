---
name: Release
about: Cut a Crossplane release
labels: release
---

<!--
Issue title should be in the following format:

    Cut vX.Y.Z Release on DATE

For example:

    Cut v1.3.0 on June 29, 2021.

Please assign the release manager to the issue.
-->

### Steps
<!--
Please complete the following steps in order. Links should be populated at the
bottom of this section.
-->

This issue can be closed when we have:

<!--
Please uncomment the following block only if cutting a minor release. Most of
these should be completed at the beginning of Code Freeze:
https://crossplane.io/docs/v1.2/reference/release-cycle.html#code-freeze

The exception is the Crossplane docs website repo update. You can open a PR at
code freeze time, but it should not be merged until the release is complete.
-->

<!--
- [ ] Created the [release branch][release-branch].
- [ ] Created and merged an empty commit to the `master` branch ([PR][empty-commit-pr]).
- [ ] Run the [Tag workflow][rc-tag-workflow] on the `master` branch with the next release candidate tag ([Tag][rc-tag]).
- [ ] Updated the current release version in the [Crossplane docs website repo] ([link][website-pr]).
- [ ] Updated the release branch reaching EOL with docs removal directive ([link][eol-pr]).
-->
- [ ] Updated all version information in the documentation on the relevant release branch ([link][docs-update-pr]).
- [ ] Run the [Tag workflow][tag-workflow] on the relevant release branch with the proper release version ([link][tag]).
- [ ] Run the [CI workflow][ci-workflow] on the release branch and verified that the tagged build version exists on the [releases.crossplane.io] `build` channel. ([link][release-build]).
- [ ] Run the [Configurations workflow][configurations-workflow] on the release branch and verified  that version exists on [registry.upbound.io] for all getting started packages.
    - [ ] `xp/getting-started-with-aws` ([link][xp/getting-started-with-aws])
    - [ ] `xp/getting-started-with-with-with-vpc` ([link][xp/getting-started-with-aws-with-vpc])
    - [ ] `xp/getting-started-with-azure` ([link][xp/getting-started-with-azure])
    - [ ] `xp/getting-started-with-gcp` ([link][xp/getting-started-with-gcp])
- [ ] Run the [Promote workflow][promote-workflow] with channel `stable` on the release branch and verified that the tagged build version exists on the [releases.crossplane.io] `stable` channel ([link][stable-build]).
- [ ] Published a [new release] for the tagged version, with the same name as the version and descriptive release notes ([link][release]).
- [ ] Updated the [releases table] in the `README.md` on `master` ([link][release-table-pr]).
- [ ] Ensured that users have been notified of the release on all communication channels:
    - [ ] Slack ([link][slack])
    - [ ] Twitter ([link][twitter])


<!-- Links Populated by Release Manager -->

<!--
Only relevant for minor releases. This should look something like the below
example from the v1.3.0 release.

[release-branch]: https://github.com/crossplane/crossplane/tree/release-1.3
[empty-commit-pr]: https://github.com/crossplane/crossplane/pull/2395
[rc-tag-workflow]: https://github.com/crossplane/crossplane/runs/2880453549?check_suite_focus=true
[rc-tag]: https://github.com/crossplane/crossplane/releases/tag/v1.4.0-rc.0
[website-pr]: https://github.com/crossplane/crossplane.github.io/pull/112
[eol-pr]: https://github.com/crossplane/crossplane/pull/2679
-->
[release-branch]: #
[empty-commit-pr]: #
[rc-tag-workflow]: #
[rc-tag]: #
[website-pr]: #
[eol-pr]: #

<!--
Relevant for all releases. This should look something like the below example
from the v1.3.0 release.

[docs-update-pr]: https://github.com/crossplane/crossplane/pull/2412
[tag-workflow]: https://github.com/crossplane/crossplane/runs/2945452331?check_suite_focus=true
[tag]: https://github.com/crossplane/crossplane/releases/tag/v1.3.0
[ci-workflow]: https://github.com/crossplane/crossplane/actions/runs/983799776
[release-build]: https://releases.crossplane.io/build/release-1.3/
[configurations-workflow]: https://github.com/crossplane/crossplane/runs/2945538373
[xp/getting-started-with-aws]: https://cloud.upbound.io/registry/xp/getting-started-with-aws/v1.3.0
[xp/getting-started-with-aws-with-vpc]: https://cloud.upbound.io/registry/xp/getting-started-with-aws-with-vpc/v1.3.0
[xp/getting-started-with-azure]: https://cloud.upbound.io/registry/xp/getting-started-with-azure/v1.3.0
[xp/getting-started-with-gcp]: https://cloud.upbound.io/registry/xp/getting-started-with-gcp/v1.3.0
[promote-workflow]: https://github.com/crossplane/crossplane/actions/runs/983871530
[stable-build]: https://releases.crossplane.io/stable/v1.3.0/
[release]: https://github.com/crossplane/crossplane/releases/tag/v1.3.0
[slack]: https://crossplane.slack.com/archives/CEFQCGW1H/p1625001259051300
[twitter]: https://twitter.com/crossplane_io/status/1409986997687627778?s=20
-->
[docs-update-pr]: #
[tag-workflow]: #
[tag]: #
[ci-workflow]: #
[release-build]: #
[configurations-workflow]: #
[xp/getting-started-with-aws]: #
[xp/getting-started-with-aws-with-vpc]: #
[xp/getting-started-with-azure]: #
[xp/getting-started-with-gcp]: #
[promote-workflow]: #
[stable-build]: #
[release]: #
[slack]: #
[twitter]: #
[release-table-pr]: #

### Notes

<!-- This section is reserved for any relevant notes or links for this release. -->

<!-- Named Links -->
[releases.crossplane.io]: https://releases.crossplane.io
[registry.upbound.io]: https://cloud.upbound.io/browse
[new release]: https://github.com/crossplane/crossplane/releases/new
[releases table]: https://github.com/crossplane/crossplane#releases
[Crossplane docs website repo]: https://github.com/crossplane/crossplane.github.io
