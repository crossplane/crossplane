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
this issue for posterity. Refer to this [prior release issue][release-1.7] for
examples of each step.

- [ ] Updated all version information in the documentation on the relevant release branch.
- [ ] Run the [Tag workflow][tag-workflow] on the relevant release branch with the proper release version.
- [ ] Run the [CI workflow][ci-workflow] on the release branch and verified that the tagged build version exists on the [releases.crossplane.io] `build` channel..
- [ ] Run the [Configurations workflow][configurations-workflow] on the release branch and verified  that version exists on [xpkg.upbound.io] for all getting started packages.
  - [ ] `xp/getting-started-with-aws`
  - [ ] `xp/getting-started-with-with-with-vpc`
  - [ ] `xp/getting-started-with-azure`
  - [ ] `xp/getting-started-with-gcp`
- [ ] Run the [Promote workflow][promote-workflow] with channel `stable` on the release branch and verified that the tagged build version exists on the [releases.crossplane.io] `stable` channel.
- [ ] Published a [new release] for the tagged version, with the same name as the version and descriptive release notes.
- [ ] Update the [`crossplane/test` repo test workflows][crossplane-test-workflows] to ensure the checkout release branch and helm install version(s) point at the new release
- [ ] Ensured that users have been notified of the release on all communication channels:
  - [ ] Slack
  - [ ] Twitter

<!-- Named Links -->
[releases.crossplane.io]: https://releases.crossplane.io
[xpkg.upbound.io]: https://cloud.upbound.io/browse
[new release]: https://github.com/crossplane/crossplane/releases/new
[tag-workflow]: https://github.com/crossplane/crossplane/actions/workflows/tag.yml
[ci-workflow]: https://github.com/crossplane/crossplane/actions/workflows/ci.yml
[configurations-workflow]: https://github.com/crossplane/crossplane/actions/workflows/configurations.yml
[promote-workflow]: https://github.com/crossplane/crossplane/actions/workflows/promote.yml
[crossplane-test-workflows]: https://github.com/crossplane/test/tree/master/.github/workflows
[release-1.7]: https://github.com/crossplane/crossplane/issues/2977