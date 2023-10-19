---
name: Release
about: Cut a Crossplane release
labels: release
---

<!--
Issue title should be in the following format:

    Cut vX.Y.0 Release on DATE

For example:

    Cut v1.3.0 on June 29, 2021.

Please assign the release manager to the issue.
-->

This issue can be closed when we have completed the following steps (in order).
Please ensure all artifacts (PRs, workflow runs, Tweets, etc) are linked from
this issue for posterity. Refer to this [prior release issue][release-1.11.0] for
examples of each step, assuming release vX.Y.0 is being cut.

- [ ] **[In Crossplane Runtime]**: Prepared the release branch `release-X.Y` at the beginning of [Code Freeze]:
  - [ ] Created the release branch using the [GitHub UI][create-branch].
  - [ ] Created and merged an empty commit to the `master` branch, if required to have it at least one commit ahead of the release branch.
  - [ ] Run the [Tag workflow][tag-workflow] on the `master` branch with the release candidate tag for the next release `vX.Y+1.0-rc.0`.
- [ ] **[In Core Crossplane]:** Prepared the release branch `release-X.Y` at the beginning of [Code Freeze]:
  - [ ] Created the release branch using the [GitHub UI][create-branch].
  - [ ] Created and merged an empty commit to the `master` branch, if required to have it at least one commit ahead of the release branch.
  - [ ] Run the [Tag workflow][tag-workflow] on the `master` branch with the release candidate tag for the next release `vX.Y+1.0-rc.0`.
- [ ] Opened a [docs release issue].
- [ ] Checked that the [GitHub milestone] for this release only contains closed issues.
- [ ] Cut a Crossplane Runtime version and consume it from Crossplane.
  - [ ] **[In Crossplane Runtime]**: Run the [Tag workflow][tag-workflow] on the `release-X.Y` branch with the proper release version, `vX.Y.0`. Message suggested, but not required: `Release vX.Y.0`.
  - [ ] **[In Core Crossplane]:** Update the Crossplane Runtime dependency on master and backport it to `release-X.Y` branch.
- [ ] Run the [Tag workflow][tag-workflow] on the `release-X.Y` branch with the proper release version, `vX.Y.0`. Message suggested, but not required: `Release vX.Y.0`.
- [ ] Run the [CI workflow][ci-workflow] on the release branch and verified that the tagged build version exists on the [releases.crossplane.io] `build` channel, e.g. `build/release-X.Y/vX.Y.0/...` should contain all the relevant binaries.
- [ ] Run the [Promote workflow][promote-workflow] with channel `stable` on the `release-X.Y` branch and verified that the tagged build version exists on the [releases.crossplane.io] `stable` channel at `stable/vX.Y.0/...`.
- [ ] Published a [new release] for the tagged version, with the same name as the version and descriptive release notes, taking care of generating the changes list selecting as "Previous tag" `vX.<Y-1>.0`, so the first of the releases for the previous minor. Before publishing the release notes, set them as Draft and ask the rest of the team to double check them.
- [ ] Checked that the [docs release issue] created previously has been completed.
- [ ] Updated, in a single PR, the following on `master`:
  - [ ] The [releases table] in the `README.md`, removing the now old unsupported release and adding the new one.
  - [ ] The `baseBranches` list in `.github/renovate.json5`, removing the now old unsupported release and adding the new one.
- [ ] Closed the GitHub milestone for this release.
- [ ] Publish a blog post about the release to the [crossplane blog]
- [ ] Ensured that users have been notified of the release on all communication channels:
  - [ ] Slack: `#announcements` channel on Crossplane's Slack workspace.
  - [ ] Twitter: reach out to a Crossplane maintainer or steering committee member, see [OWNERS.md][owners].
  - [ ] LinkedIn: same as Twitter
- [ ] Request @jbw976 to remove all old docs versions from Google Search
- [ ] Remove any extra permissions given to release team members for this release


<!-- Named Links -->
[Code Freeze]: https://docs.crossplane.io/knowledge-base/guides/release-cycle/#code-freeze
[ci-workflow]: https://github.com/crossplane/crossplane/actions/workflows/ci.yml
[configurations-workflow]: https://github.com/crossplane/crossplane/actions/workflows/configurations.yml
[create-branch]: https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/creating-and-deleting-branches-within-your-repository
[docs release issue]: https://github.com/crossplane/docs/issues/new?assignees=&labels=release&template=new_release.md&title=Release+Crossplane+version...+
[new release]: https://github.com/crossplane/crossplane/releases/new
[owners]: https://github.com/crossplane/crossplane/blob/master/OWNERS.md
[promote-workflow]: https://github.com/crossplane/crossplane/actions/workflows/promote.yml
[release-1.11.0]: https://github.com/crossplane/crossplane/issues/3600
[releases table]: https://github.com/crossplane/crossplane#releases
[releases.crossplane.io]: https://releases.crossplane.io
[tag-workflow]: https://github.com/crossplane/crossplane/actions/workflows/tag.yml
[GitHub milestone]: https://github.com/crossplane/crossplane/milestones
[crossplane blog]: https://blog.crossplane.io
