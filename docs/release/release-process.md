# Release Process

This document is meant to be a complete end-to-end guide for how to release new
versions of software for Crossplane and its related projects.

## tl;dr Process Overview

All the details are available in the sections below, but we'll start this guide
with a very high level sequential overview for how to run the release process.

1. **feature freeze**: Merge all completed features into master branches of all
   repos to begin "feature freeze" period
1. **API docs/user guides**: Regenerate API docs and update all user guides with
   current content for scenarios included in the release
1. **release crossplane-runtime**: Tag and release a new version of
   crossplane-runtime using the GitHub UI.
1. **pin crossplane dependencies**: Update the go modules of core crossplane in
   master to depend on the newly released version of crossplane-runtime.
1. **pre-release tag crossplane**: Run tag pipeline to tag the start of
   pre-releases in master in the crossplane repo
1. **branch crossplane**: Create a new release branch using the GitHub UI for
   the crossplane repo
1. **crossplane release branch prep**: In Crossplane's release branch, update
   all examples, docs, and integration tests to update references and versions,
   including the yet to be released versions of providers and stacks.
1. **tag**: Run the tag pipeline to tag Crossplane's release branch with an
   official semver
1. **release providers**: Run the release process for each **provider** that we
   maintain
    1. **pin dependencies**: Update the go modules of each provider repo to
       depend on the new version of crossplane and crossplane-runtime.
    1. **pre-release tag**: Run tag pipeline to tag the start of pre-releases in
       **master** of each provider repo
    1. **branch**: Create a new release branch using the GitHub UI for the
       provider repo
    1. **release branch prep**: In the release branch, update all examples,
       docs, and integration tests to update references and versions
    1. **test** Test builds from the release branch, fix any critical bugs that
       are found
    1. **tag**: Run the tag pipeline to tag the release branch with an official
       semver
    1. **build/publish**: Run build pipeline to publish build with official
       semver
1. **release template stacks**: Run the release process for each **template
   stack** that we maintain. Note that the process for template stacks is
   slightly different from the stack release process.
    1. **test** Test builds from the release branch (typically `master`), fix
       any critical bugs that are found
    1. **version**: Update all version information in the docs, as appropriate
    1. **tag**: Run the tag pipeline to tag the release branch with an official
       semver
    1. **build/publish**: Run the publish pipeline to publish build with
       official semver
1. **build/publish**: Run build pipeline to publish Crossplane build from
   release branch with official semver
1. **verify**: Verify all artifacts have been published successfully, perform
   sanity testing
1. **promote**: Run promote pipelines on all repos to promote releases to
   desired channel(s)
1. **release notes**: Publish well authored and complete release notes on GitHub
1. **announce**: Announce the release on Twitter, Slack, etc.

## Detailed Process

This section will walk through the release process in more fine grained and
prescriptive detail.

### Scope

This document will cover the release process for all of the repositories that
the Crossplane team maintains and publishes regular versioned artifacts from.
This set of repositories covers both core Crossplane and the set of Providers,
Stacks, and Apps that Crossplane currently maintains:

* [`crossplane`](https://github.com/crossplane/crossplane)
* [`provider-gcp`](https://github.com/crossplane/provider-gcp)
* [`provider-aws`](https://github.com/crossplane/provider-aws)
* [`provider-azure`](https://github.com/crossplane/provider-azure)
* [`provider-rook`](https://github.com/crossplane/provider-rook)
* [`stack-minimal-gcp`](https://github.com/crossplane/stack-minimal-gcp)
* [`stack-minimal-aws`](https://github.com/crossplane/stack-minimal-aws)
* [`stack-minimal-azure`](https://github.com/crossplane/stack-minimal-azure)
* [`app-wordpress`](https://github.com/crossplane/app-wordpress)

The release process for Providers is almost identical to that of core Crossplane
because they use the same [shared build
logic](https://github.com/upbound/build/).  The steps in this guide will apply
to all repositories listed above unless otherwise mentioned.

### Feature Freeze

Feature freeze should be performed on all repos.  In order to start the feature
freeze period, the following conditions should be met:

* All expected features should be
  ["complete"](https://github.com/crossplane/crossplane/blob/master/design/one-pager-definition-of-done.md)
  and merged into master. This includes user guides, examples, API documentation
  via [crossdocs](https://github.com/negz/crossdocs/), and test updates.
* All issues in the
  [milestone](https://github.com/crossplane/crossplane/milestones) should be
  closed
* Sanity testing has been performed on `master`

After these conditions are met, the feature freeze begins by creating the RC tag
and the release branch.

### Pin Dependencies

It is a best practice to release Crossplane projects with "pinned" dependencies
to specific versions of other upstream Crossplane projects. For example, after
crossplane-runtime has been released, we want to update the main Crossplane repo
to use that specific released version.

To update a dependency to a specific version, simply edit the `go.mod` file to
point to the desired version, then run `go mod tidy`.

### Pre-release Tag

The next step is to create the pre-release tag for the `HEAD` commit in
`master`.  This tag serves as an indication of when the release was branched
from master and is also important for generating future versions of `master`
builds since that [versioning
process](https://github.com/upbound/build/blob/master/makelib/common.mk#L182-L196)
is based on `git describe --tags`.

> **NOTE:** The `tag` pipeline does not yet support additional (pre-release)
tags in the version number, such as `v0.5.0-rc`.
[#330](https://github.com/crossplane/crossplane/issues/330) will be resolved
when this functionality is available.  In the meantime, **manually tagging and
pushing to the repo is required**.  Ignore the steps below about running the
pipeline because the pipeline won't work.

To accomplish this, run the `tag` pipeline for each repo on the `master` branch.
You will be prompted to enter the `version` for the tag and the `commit` hash to
tag. It's possible to leave the `commit` field blank to default to tagging
`HEAD`.

Since this tag will essentially be the start of pre-releases working towards the
**next** version, the `version` should be the **next** release number, plus a
trailing tag to indicate it is a pre-release.  The current convention is to use
`*-rc`.  For example, when we are releasing the `v0.9.0` release and we are
ready for master to start working towards the **next** release of `v0.10.0`, we
would make the tag `v0.10.0-rc`

After the tag pipeline has succeeded, verify in the [GitHub
UI](https://github.com/crossplane/crossplane/tags) that the tag was successfully
applied to the correct commit.

### Create Release Branch

Creating the release branch can be done within the [GitHub
UI](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/creating-and-deleting-branches-within-your-repository).
Basically, you just use the branch selector drop down and type in the name of
the new release branch, e.g. `release-0.5`. Release branch names always follow
the convention of `release-[minor-semver]`.

If this is the first ever release branch being created in a repo (uncommon), you
should also set up branch protection rules for the `release-*` pattern.  You can
find existing examples in the [Crossplane repo
settings](https://github.com/crossplane/crossplane/settings/branches).

At this point, the `HEAD` commit in the release branch will be our release
candidate.  The build pipeline will automatically be started due to the create
branch event, so we can start to perform testing on this build.  Note that it
should be the exact same as what is currently in `master` since they are using
the same commit and have the same tag.  Also note that this is not the official
release build since we have not made the official release tag yet (e.g.
`v0.5.0`).

The `master` branch can now be opened for new features since we have a safe
release branch to continue bug fixes and improvements for the release itself.
Essentially, `master` is free to now diverge from the release branch.

### Release Branch Prep

In the core Crossplane repository, we need to update the release branch docs and
examples to point to the new versions that we will be releasing soon.

* Documentation, such as [Installation
  instructions](https://github.com/crossplane/crossplane/blob/release-0.9/docs/install.md#installing-infrastructure-providers),
  and
  [Stack](https://github.com/crossplane/crossplane/blob/release-0.9/docs/stack.md)
  and
  [App](https://github.com/crossplane/crossplane/blob/release-0.9/docs/app.md)
  guides.
  * searching for `:master` will help a lot here
* Examples, such as [`StackInstall` yaml
  files](https://github.com/crossplane/crossplane/tree/release-0.9/cluster/examples/provider)
* [Helm chart
  defaults](https://github.com/crossplane/crossplane/blob/release-0.9/cluster/charts/crossplane/values.yaml.tmpl),
  ensure all `values.yaml.tmpl` files are updated.
  * provider versions
  * `templating-controller` version (if a new version is available and ready)

#### Bug Fixes in Release Branch

During our testing of the release candidate, we may find issues or bugs that we
triage and decide we want to fix before the release goes out. In order to fix a
bug in the release branch, the following process is recommended:

1. Make the bug fix into `master` first through the normal PR process
    1. If the applicable code has already been removed from `master` then simply
       fix the bug directly in the release branch by opening a PR directly
       against the release branch
1. Backport the fix by performing a cherry-pick of the fix's commit hash
   (**not** the merge commit) from `master` into the release branch.  For
   example, to backport a fix from master to `v0.5.0`, something like the
   following should be used:

    ```console
    git fetch --all
    git checkout -b release-0.5 upstream/release-0.5
    git cherry-pick -x <fix commit hash>
    ```

1. Open a PR with the cherry-pick commit targeting the release-branch

After all bugs have been fixed and backported to the release branch, we can move
on to tagging the final release commit.

### Tag Core Crossplane

Similar to running the `tag` pipelines for each stack, now it's time to run the
[`tag`
pipeline](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fcrossplane%2Fcrossplane-tag/branches)
for core Crossplane.  In fact, the [instructions](#stack-tag-pipeline) are
exactly the same:

Run the tag pipeline by clicking the Run button in the Jenkins UI in the correct
release branch's row. You will be prompted for the version you are tagging,
e.g., `v0.5.0` as well as the commit hash. The hash is optional and if you leave
it blank it will default to `HEAD` of the branch, which is what you want.

> **Note:** The first time you run a pipeline on a new branch, you won't get
> prompted for the values to input. The build will quickly fail and then you can
> run (not replay) it a second time to be prompted.  This is a Jenkins bug that
> is tracked by [#41929](https://issues.jenkins-ci.org/browse/JENKINS-41929) and
> has been open for almost 3 years, so don't hold your breath.

### Draft Release Notes

We're getting close to starting the official release, so you should take this
opportunity to draft up the release notes. You can create a [new release draft
here](https://github.com/crossplane/crossplane/releases/new).  Make sure you
select "This is a pre-release" and hit "Save draft" when you are ready to share
and collect feedback.  Do **not** hit "Publish release" yet.

You can see and follow the template and structure from [previous
releases](https://github.com/crossplane/crossplane/releases).

### Provider Release Process

This section will walk through how to release the Providers and does not
directly apply to core Crossplane.

#### Pin Provider Dependencies

Similar to core crossplane, each provider should have its crossplane related
dependencies pinned to the versions that we are releasing. In the **master**
branch of each provider repo, update the `crossplane` and `crossplane-runtime`
dependencies to the versions we are releasing.

Simply edit `go.mod` with the new versions, then run `go mod tidy`.

The providers also depend on `crossplane-tools`, but that currently does not
have official releases, so in practice should be using the latest from master.

#### Provider Pre-release tag

Follow the same steps that we did for core crossplane to tag the **master**
branch of each provider repo with a pre-release tag for the **next** version.

These steps can be found in the [pre-release tag section](#pre-release-tag).

#### Create Provider Release Branches

Now create a release branch for each of the provider repos using the GitHub UI.
The steps are the same as what we did to [create the release
branch](#create-release-branch) for core crossplane.

#### Provider Release Branch Prep

In the **release branch** for each provider, you should update the version tags
and metadata in:

* `integration_tests.sh` - `STACK_IMAGE`
* `ClusterStackInstall` sample and example yaml files
* `*.resource.yaml` - docs links in markdown
  * Not all of these `*.resource.yaml` files have links that need to be updated,
    they are infrequent and inconsistent

Searching for `:master` will be a big help here.

#### Provider Tag, Build, and Publish

Now that the Providers are all tested and their version metadata has been
updated, it's time to tag the release branch with the official version tag. You
can do this by running the `tag` pipeline on the release branch of each
Provider:

* [`provider-gcp` tag
  pipeline](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fprovider-gcp-pipelines%2Fprovider-gcp-tag/branches)
* [`provider-aws` tag
  pipeline](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fprovider-aws-pipelines%2Fprovider-aws-tag/branches/)
* [`provider-azure` tag
  pipeline](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fprovider-azure-pipelines%2Fprovider-azure-tag/branches/)
* [`provider-rook` tag
  pipeline](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fprovider-rook-pipelines%2Fprovider-rook-tag/branches/)

* Run the `tag` pipeline on the release branch
* Enter the version and commit hash (leave blank for `HEAD`)
* The first time you run on a new release branch, you won't be prompted and the
  build will fail, just run (not replay) a second time

After the tag pipeline has been run and the release branch has been tagged, you
can run the normal build pipeline on the release branch.  This will kick off the
official release build and upon success, all release artifacts will be
officially published.

After the release build succeeds, verify that the correctly versioned Provider
images have been pushed to Docker Hub.

### Template Stack Release Process

The Template Stacks we maintain are slightly different from the controller-based
stacks that we maintain. Their processes are similar but a little simpler. This
section will walk through how to release the Template Stacks themselves, and
does not directly apply to core Crossplane.

For Template Stacks, we do not use release branches unless we need to make a
patch release. In the future we may need a more robust branching strategy, but
for now we are not using branches because it is simpler.

Note that Template Stacks **do not** require any code changes to update their
version. A slight exception to this is for their `behavior.yaml` files, which
should have the `controllerImage` field updated if a new version of the
`templating-controller` is available and ready.

### Template Stack Tag And Publish Pipeline

Here is the list of all template stacks:

* [`stack-minimal-gcp`](https://github.com/crossplane/stack-minimal-gcp)
* [`stack-minimal-aws`](https://github.com/crossplane/stack-minimal-aws)
* [`stack-minimal-azure`](https://github.com/crossplane/stack-minimal-azure)
* [`app-wordpress`](https://github.com/crossplane/app-wordpress)

Each one should be released as part of a complete release, using the
instructions below. To read even more about the template stack release process,
see [the release section of this
document](https://github.com/crossplane/cicd/blob/master/docs/pipelines.md#how-do-i-cut-a-release)

Note that there's also the
[`templating-controller`](https://github.com/crossplane/templating-controller),
which supports template stacks. It is possible that it **may** need to be
released as well, but typically is released independently from Crossplane.

#### Tag the Template Stack

Once a template stack is tested and ready for cutting a semver release, we will
want to tag the repository with the new release version. In most cases, to get
the version, take a look at the most recent tag in the repo, and increment the
minor version. For example, if the most recent tag was `v0.2.0`, the new tag
should be `v0.3.0`.

Run the template stack's tag job on Jenkins, against the `master` branch. Enter
in the new tag to use. If the current release candidate is not the head of
`master`, enter in the commit to tag.

You can find the tag pipeline for the individual stack by going to the
[crossplane org in Jenkins](https://jenkinsci.upbound.io/job/crossplane/),
finding the folder with the same name as the template stack, and then going to
the `tag` job group. Then going to the `master` branch job under the group. For
example, here is [a link to the stack-minimal-gcp tag job for
master](https://jenkinsci.upbound.io/job/crossplane/job/stack-minimal-gcp/job/tag/job/master/).

> **Note:** The first time you run a pipeline on a new branch, you won't get
> prompted for the values to input and the build will fail. See details in the
> [tagging core crossplane section](#tag-core-crossplane).

#### Publish the Template Stack

After the tag pipeline has been run and the repository has been tagged, you can
run the `publish` job for the template stack. For example, here's a [link to the
stack-minimal-gcp publish
job](https://jenkinsci.upbound.io/job/crossplane/job/stack-minimal-gcp/job/publish/job/master/).
This will kick off the official release build and upon success, all release
artifacts will be officially published. This should also be run from the
`master` branch in most cases. Or, if a release branch was used, from the
release branch. The tag parameter should be used, and the tag for the current
release should be used. For example, if the new tag we created was `v0.3.0`, we
would want to provide `v0.3.0` for the `publish` job.

#### Verify the Template Stack was Published

After the publish build succeeds, verify that the correctly versioned template
stack images have been pushed to Docker Hub.

### Template Stack Patch Releases

To do a patch release with a template stack, create a release branch from the
minor version tag on the `master` branch, if a release branch doesn't already
exist. Then, the regular tagging and publishing process for template stacks can
be followed, incrementing the patch version to get the new release tag.

### Build and Release Core Crossplane

After the providers, stacks, and apps have all been released, ensure the [normal
build
pipeline](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fcrossplane%2Fbuild/branches)
is run on the release branch for core crossplane.  This will be the official
release build with an official version number and all of its release artifacts
will be published.

After the pipeline runs successfully, you should verify that all artifacts have
been published to:

* [Docker Hub](https://hub.docker.com/repository/docker/crossplane/crossplane)
* [S3 releases bucket](https://releases.crossplane.io/)
* [Helm chart repository](https://charts.crossplane.io/)
* [Docs website](https://crossplane.io/docs/latest)

### Promotion

If everything looks good with the official versioned release that we just
published, we can go ahead and run the `promote` pipeline for the core
crossplane and provider repos. This is a very quick pipeline that doesn't
rebuild anything, it simply makes metadata changes to the published release to
also include the release in the channel of your choice.

Currently, we only support the `master` and `alpha` channels.

For the core crossplane and each provider repo, run the `promote` pipeline on
the release branch and input the version you would like to promote (e.g.
`v0.5.0`) and the channel you'd like to promote it to.  The first time you run
this pipeline on a new release branch, you will not be prompted for values, so
the pipeline will fail. Just run (not replay) it a second time to be prompted.

* Run `promote` pipeline for `master` channel
* Run `promote` pipeline for `alpha` channel

After the `promote` pipelines have succeeded, verify on DockerHub and the Helm
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

* Fix any bugs in `master` first and then `cherry-pick -x` to the release branch
  * If `master` has already removed the relevant code then make your fix
    directly in the release branch
* After all testing on the release branch look good and any docs/examples/tests
  have been updated with the new version number, run the `tag` pipeline on the
  release branch with the new patch version (e.g. `v0.5.1`)
* Run the normal build pipeline on the release branch to build and publish the
  release
* Publish release notes
* Run promote pipeline to promote the patch release to the `master` and `alpha`
  channels
