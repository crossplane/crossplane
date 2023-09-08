# Release Process

## New Patch Release (vX.Y.Z)

In order to cut a new patch release from an existing release branch `release-X.Y`, follow these steps:

- Run the [Tag workflow][tag-workflow] on the `release-X.Y` branch with the proper release version, `vX.Y.Z`. Message suggested, but not required: `Release vX.Y.Z`.
- Draft the [new release notes], and share them with the rest of the team to ensure that all the required information is included.
- Publish the above release notes.

## New Minor Release (vX.Y.0)

In order to cut a new minor release, follow these steps:

- Create a new release branch `release-X.Y` from `master`, using the [GitHub UI][create-branch].
- Create and merge an empty commit to the `master` branch, if required to have it at least one commit ahead of the release branch.
- Run the [Tag workflow][tag-workflow] on the `master` branch with the release candidate tag for the next release, so `vX.<Y+1>.0-rc.0`.
- Run the [Tag workflow][tag-workflow] on the `release-X.Y` branch with the proper release version, `vX.Y.0`. Message suggested, but not required: `Release vX.Y.0`.
- Draft the [new release notes], and share them with the rest of the team to ensure that all the required information is included.
- Publish the above release notes.

<!-- Named Links -->
[create-branch]: https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/creating-and-deleting-branches-within-your-repository
[new release notes]: https://github.com/crossplane/crossplane-runtime/releases/new
[tag-workflow]: https://github.com/crossplane/crossplane-runtime/actions/workflows/tag.yml
