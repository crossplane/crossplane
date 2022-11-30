---
title: Release Process
weight: 1003
---

This document is meant to be a complete end-to-end guide for how to release new
versions of software for Crossplane and its related projects.

## tl;dr Process Overview

All the details are available in the sections below, but we'll start this guide
with a very high level sequential overview for how to run the release process.
These steps apply to all Crossplane projects, all of which utilize [Github
Actions](https://github.com/features/actions) for pipelines.

1. **feature freeze**: Merge all completed features into main development branch
   of all repos to begin "feature freeze" period.
1. **pin dependencies**: Update the go module on main development branch to
   depend on stable versions of dependencies if needed.
1. **branch repo**: Create a new release branch using the GitHub UI for the
   repo.
1. **release branch prep**: Make any release-specific updates on the release
   branch (typically documentation).
1. **tag release**: Run the `Tag` action on the _release branch_ with the
   desired version (e.g. `v0.14.0`).
1. **build/publish**: Run the `CI` and `Configurations` action on the release
   branch with the version that was just tagged.
1. **tag next pre-release**: Run the `tag` action on the main development branch
   with the `rc.0` for the next release (e.g. `v0.15.0-rc.0`).
1. **verify**: Verify all artifacts have been published successfully, perform
   sanity testing.
1. **promote**: Run the `Promote` action to promote release to desired
   channel(s).
1. **release notes**: Publish well authored and complete release notes on
   GitHub.
1. **announce**: Announce the release on Twitter, Slack, etc.

## Detailed Process

This section will walk through the release process in more fine grained and
prescriptive detail.

### Feature Freeze

Feature freeze should be performed on all repos.  In order to start the feature
freeze period, the following conditions should be met:


* All issues in the
  [milestone](https://github.com/crossplane/crossplane/milestones) should be
  closed
* Sanity testing has been performed on main development branch

### Pin Dependencies

It is a best practice to release Crossplane projects with "pinned" dependencies
to specific stable versions. For example, after crossplane-runtime has been
released, we want to update the main Crossplane repo to use that specific
released version.

To update a dependency to a specific version, simply edit the `go.mod` file to
point to the desired version, then run `go mod tidy`.

### Create Release Branch

Creating the release branch can be done within the [GitHub
UI](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/creating-and-deleting-branches-within-your-repository).
Basically, you just use the branch selector drop down and type in the name of
the new release branch, e.g. `release-0.5`. Release branch names always follow
the convention of `release-[minor-semver]`.

If this is the first ever release branch being created in a repo (uncommon), you
should also set up branch protection rules for the `release-*` pattern. You can
find existing examples in the [Crossplane repo
settings](https://github.com/crossplane/crossplane/settings/branches).

At this point, the `HEAD` commit in the release branch will be our release
candidate. The build pipeline will automatically be started due to the create
branch event, so we can start to perform testing on this build. Note that it
should be the exact same as what is currently in main development branch since
they are using the same commit and have the same tag. Also note that this is not
the official release build since we have not made the official release tag yet
(e.g. `v0.5.0`).

### Release Branch Prep

Some repos may not require any release branch prep. This is desirable as it
reduces the burden of running a new release. If this is the case for the repo
being released, you may skip this step.

In the core Crossplane repository, we need to update the release branch docs and
examples to point to the new versions that we will be releasing soon.

* Documentation, such as pinning
  [snippet](https://github.com/crossplane/crossplane/blob/release-0.14/docs/snippets)
  links to the current release branch.
  * searching for `:v` will help a lot here

#### Bug Fixes in Release Branch

During our testing of the release candidate, we may find issues or bugs that we
triage and decide we want to fix before the release goes out. In order to fix a
bug in the release branch, the following process is recommended:

1. Make the bug fix into main development branch first through the normal PR
   process
    1. If the applicable code has already been removed from the main development
       branch then simply fix the bug directly in the release branch by opening
       a PR directly against the release branch
1. Backport the fix by performing a cherry-pick of the fix's commit hash
   (**not** the merge commit) from main development branch into the release
   branch. For example, to backport a fix from the main development branch to
   `v0.5.0`, something like the following should be used:

    ```console
    git fetch --all
    git checkout -b release-0.5 upstream/release-0.5
    git cherry-pick -x <fix commit hash>
    ```

1. Open a PR with the cherry-pick commit targeting the release-branch

After all bugs have been fixed and backported to the release branch, we can move
on to tagging the final release commit.

### Tag Release

Now it's time to run the `Tag` action on the release branch.

Run the tag action by going to the repo's "Actions" tab in the Github UI. You
will be prompted for the desired branch and the version you are tagging. The
latest commit on the selected branch will be the commit that is tagged. 

### Draft Release Notes

We're getting close to starting the official release, so you should take this
opportunity to draft up the release notes. You can create a [new release draft
here](https://github.com/crossplane/crossplane/releases/new).  Make sure you
select "This is a pre-release" and hit "Save draft" when you are ready to share
and collect feedback.  Do **not** hit "Publish release" yet.

You can see and follow the template and structure from [previous
releases](https://github.com/crossplane/crossplane/releases).

### Build and Publish

Run the `CI` action on the release branch. This will build and publish the
official release with the correct version tag and all of its release artifacts
will be published.

If there are any `Configuration` packages that are built in the repo, you must
also run the `Configurations` action on the release branch. This will build,
tag, and publish the `Configuration` packages to the configured OCI image
registry.

After the pipeline runs successfully, you should verify that all artifacts have
been published to:

For all repos:
* [Docker Hub](https://hub.docker.com/repository/docker/crossplane)

For all repos with Helm charts:
* [S3 releases bucket](https://releases.crossplane.io/)
* [Helm chart repository](https://charts.crossplane.io/)

For crossplane/crossplane:
* [Docs website](https://docs.crossplane.io)
* [Configuration Packages](https://marketplace.upbound.io)

### Tag Next Pre-release

The next step is to create the pre-release tag for the `HEAD` commit in main
development branch. This tag serves as an indication of when the release was
branched from the main development branch and is also important for generating
future versions of the main development branch builds since that [versioning
process](https://github.com/upbound/build/blob/master/makelib/common.mk#L182-L196)
is based on `git describe --tags`.

> NOTE: the `build` submodule uses the latest tag by timestamp on the branch
> which the commit it is building resides on. If there were no prep commits made
> on the release branch, then its `HEAD` is even with the main development
> branch (i.e. the stable tag and the next pre-release tag will be on the same
> commit). This means that we must tag the pre-release version _after_ the
> stable version to ensure subsequent builds use the next pre-release tag as
> their base. If there are additional commits on the release branch before the
> stable tag is created, then the pre-release tag could be created first.

To accomplish this, run the `Tag` action for the repo on the main development
branch branch. You will be prompted to enter the `version` for the tag. Since
this tag will essentially be the start of pre-releases working towards the
**next** version, the `version` should be the **next** release number, plus a
trailing tag to indicate it is a pre-release.  The current convention is to use
`*-rc.0`. For example, when we are releasing the `v0.9.0` release and we are
ready for the main development branch to start working towards the **next**
release of `v0.10.0`, we would make the tag `v0.10.0-rc.0`

After the tag action has succeeded, verify in the [GitHub
UI](https://github.com/crossplane/crossplane/tags) that the tag was successfully
applied to the correct commit.

The main development branch can now be opened for new features since we have a
safe release branch to continue bug fixes and improvements for the release
itself. Essentially, the main development branch is free to now diverge from the
release branch.

### Promote

If everything looks good with the official versioned release that we just
published, we can go ahead and run the `Promote` action on the release branch.
This is a very quick pipeline that doesn't rebuild anything, it simply makes
metadata changes to the published release to also include the release in the
channel of your choice.

Run the `Promote` action on the release branch and input the version you would
like to promote (e.g. `v0.5.0`) and the channel you'd like to promote it to.

After the `Promote` actions have succeeded, verify on DockerHub and the Helm
chart repository that the release has been promoted to the right channels.

### Publish Release Notes

Now that the release has been published and verified, you can publish the
[release notes](https://github.com/crossplane/crossplane/releases) that you
drafted earlier. After incorporating all feedback, you can now click on the
"Publish release" button.

This will send an email notification with the release notes to all watchers of
the repo.

### Announce Release

We have completed the entire release, so it's now time to announce it to the
world.  Using the [@crossplane_io](https://twitter.com/crossplane_io) Twitter
account, tweet about the new release and blog.  You'll see examples from the
previous releases, such as this tweet for
[v0.4](https://twitter.com/crossplane_io/status/1189307636350705664).

Post a link to this tweet on the Slack #announcements channel, then copy a link
to that and post it in the #general channel.

### Patch Releases

We also have the ability to run patch releases to update previous releases that
have already been published.  These patch releases are always run from the last
release branch, we do **not** create a new release branch for a patch release.

The basic flow is **very** similar to a normal release, but with a few less
steps.  Please refer to details for each step in the sections above.

* Fix any bugs in the main development branch first and then `cherry-pick -x` to
  the release branch
  * If main development branch has already removed the relevant code then make
    your fix directly in the release branch
* After all testing on the release branch look good and any docs/tests have been
  updated with the new version number, run the `Tag` action on the release
  branch with the new patch version (e.g. `v0.5.1`)
* Run the normal `CI` action on the release branch to build and publish the
  release
* Publish release notes
* Run `Promote` action to promote the patch release to the appropriate channels
