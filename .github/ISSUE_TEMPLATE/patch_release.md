---
name: Patch Release
about: Cut a Crossplane patch release
labels: release
---

<!--
Issue title should be in the following format:

    Cut vX.Y.Z Release on DATE

For example:

    Cut v1.3.1 on June 29, 2021.

Please assign the release manager to the issue.
-->

This issue can be closed when we have completed the following steps (in order).
Please ensure all artifacts (PRs, workflow runs, Tweets, etc) are linked from
this issue for posterity. Refer to this [prior release issue][release-1.11.1] for
examples of each step, assuming vX.Y.Z is being cut.

- [ ] Run the [Tag workflow][tag-workflow] on the `release-X.Y`branch with the proper release version, `vX.Y.Z`. Message suggested, but not required: `Release vX.Y.Z`.
- [ ] Run the [CI workflow][ci-workflow] on the release branch and verified that the tagged build version exists on the [releases.crossplane.io] `build` channel, e.g. `build/release-X.Y/vX.Y.Z/...` should contain all the relevant binaries.
- [ ] Run the [Configurations workflow][configurations-workflow] on the release branch and verified  that version exists on [xpkg.upbound.io] for all getting started packages.
  - [ ] `xp/getting-started-with-aws`
  - [ ] `xp/getting-started-with-aws-with-vpc`
  - [ ] `xp/getting-started-with-azure`
  - [ ] `xp/getting-started-with-gcp`
- [ ] Run the [Promote workflow][promote-workflow] with channel `stable` on the `release-X.Y` branch and verified that the tagged build version exists on the [releases.crossplane.io] `stable` channel at `stable/vX.Y.Z/...`.
- [ ] Published a [new release] for the tagged version, with the same name as the version and descriptive release notes, taking care of generating the changes list selecting as "Previous tag" `vX.Y.<Z-1>`, so the previous patch release for the same minor.
- [ ] Ensured that users have been notified of the release on all communication channels:
  - [ ] Slack: `#announcements` channel on Crossplane's Slack workspace.
  - [ ] Twitter: reach out to a Crossplane maintainer or steering committee member, see [OWNERS.md][owners].

<!-- Named Links -->
[ci-workflow]: https://github.com/crossplane/crossplane/actions/workflows/ci.yml
[configurations-workflow]: https://github.com/crossplane/crossplane/actions/workflows/configurations.yml
[new release]: https://github.com/crossplane/crossplane/releases/new
[owners]: https://github.com/crossplane/crossplane/blob/master/OWNERS.md
[promote-workflow]: https://github.com/crossplane/crossplane/actions/workflows/promote.yml
[release-1.11.1]: https://github.com/crossplane/crossplane/issues/3796
[releases table]: https://github.com/crossplane/crossplane#releases
[releases.crossplane.io]: https://releases.crossplane.io
[tag-workflow]: https://github.com/crossplane/crossplane/actions/workflows/tag.yml
[xpkg.upbound.io]: https://marketplace.upbound.io/configurations?query=getting-started
