---
name: Release
about: Cut a Crossplane release
labels: release
---

<!--
Issue title should be in the following format:

    Cut vX.Y.Z Release
-->

<!--
Please assign the release manager to the issue.
-->

### Release Information

**Type:**
<!-- Select One -->
- [ ] Minor
- [ ] Patch

**Version:** <!-- e.g. v1.3.0 -->

**Release Date:** <!-- e.g. Jun 29, 2021 -->

### Steps

<!-- Please complete the following steps in order. Links should be populated at the bottom of this section. -->

I have:
<!-- Please uncomment the following section only if cutting a minor release. These should be completed at the beginning of Code Freeze: https://crossplane.io/docs/v1.2/reference/release-cycle.html#code-freeze -->
<!--
- [ ] Created the [release branch][release-branch].
- [ ] Created and merged an empty commit to the `master` branch
  ([PR][empty-commit-pr]).
- [ ] Run the [Tag workflow][rc-tag-workflow] on the `master` branch with the
  next release candidate tag ([Tag][rc-tag]).
-->
- [ ] Updated all version information in the documentation on the relevant
  release branch ([PR][docs-update-pr]).
- [ ] Run the [Tag workflow][tag-workflow] on the relevant release branch with
  the proper release version ([Tag][tag]).
- [ ] Run the [CI workflow][ci-workflow] on the release branch and verified that
  the tagged build version exists on the [releases.crossplane.io] `build`
  channel. ([build][release-build]).
- [ ] Run the [Configurations workflow][configurations-workflow] on the release
  branch and verified  that version exists on [registry.upbound.io] for all
  getting started packages.
    - [ ] [xp/getting-started-with-aws]
    - [ ] [xp/getting-started-with-aws-with-vpc]
    - [ ] [xp/getting-started-with-azure]
    - [ ] [xp/getting-started-with-gcp]
- [ ] Run the [Promote workflow][promote-workflow] with channel `stable` on the
  release branch and verified that the tagged build version exists on the
  [releases.crossplane.io] `stable` channel ([build][stable-build]).
- [ ] Published a [new release] for the tagged version, with the same name as
  the version and descriptive release notes ([release][release]).
- [ ] Updated the current release version in the [Crossplane docs website repo]
  ([PR][website-pr]).
- [ ] Updated the [releases table] in the `README.md` on `master`
  ([PR][release-table-pr]).
- [ ] Ensured that users have been notified of the release on all communication
  channels:
    - [ ] [Slack][slack]
    - [ ] [Twitter][twitter]

<!-- Links Populated by Release Manager -->

<!-- Only relevant for minor releases -->
[release-branch]: <!-- Link to Release Branch, e.g. https://github.com/crossplane/crossplane/tree/release-1.3 -->
[empty-commit-pr]: <!-- Link to PR, e.g. https://github.com/crossplane/crossplane/pull/2395 -->
[rc-tag-workflow]: <!-- Link to workflow for tagging the next RC version on master -->
[rc-tag]: <!-- Link to RC tag on master, e.g. v1.4.0-rc.0 -->

<!-- Relevant for all releases -->
[docs-update-pr]: <!-- Link to merged PR, e.g. https://github.com/crossplane/crossplane/pull/2386 -->
[tag-workflow]: <!-- Link to workflow for tagging version on release branch -->
[tag]: <!-- Link to tag on release branch, e.g. v1.3.0 -->
[ci-workflow]: <!-- Link to CI workflow on release branch -->
[release-build]: <!-- Link to build, e.g. https://releases.crossplane.io/build/release-1.3/v1.3.0 -->
[configurations-workflow]: <!-- Link to Configurations workflow run on release branch -->
[xp/getting-started-with-aws]: <!-- Link to package version, e.g. https://cloud.upbound.io/registry/xp/getting-started-with-aws/v1.3.0 -->
[xp/getting-started-with-aws-with-vpc]: <!-- Link to package version, e.g. https://cloud.upbound.io/registry/xp/getting-started-with-aws-with-vpc/v1.3.0 -->
[xp/getting-started-with-azure]: <!-- Link to package version, e.g. https://cloud.upbound.io/registry/xp/getting-started-with-azure/v1.3.0 -->
[xp/getting-started-with-gcp]: <!-- Link to package version, e.g. https://cloud.upbound.io/registry/xp/getting-started-with-gcp/v1.3.0 -->
[promote-workflow]: <!-- Link to Promote workflow -->
[stable-build]: <!-- Link to build, e.g. https://releases.crossplane.io/stable/v1.3.0 -->
[release]: <!-- Link to release notes -->
[website-pr]: <!-- Link to merged PR, e.g. https://github.com/crossplane/crossplane.github.io/pull/112 -->
[slack]: <!-- Link to message in Crossplane slack -->
[twitter]: <!-- Link to tweet -->
[release-table-pr]: <!-- Link to merged PR, e.g. https://github.com/crossplane/crossplane/pull/2388 -->

### Notes

<!-- This section is reserved for any relevant notes or links for this release. -->


<!-- Named Links -->
[releases.crossplane.io]: https://releases.crossplane.io
[registry.upbound.io]: https://cloud.upbound.io/browse
[new release]: https://github.com/crossplane/crossplane/releases/new
[releases table]: https://github.com/crossplane/crossplane#releases
[Crossplane docs website repo]: https://github.com/crossplane/crossplane.github.io
