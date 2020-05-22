# Release Process and Engineering

## Background

The current lifecycle process for a new version of the Crossplane software has gotten the project
through the `v0.3` release, but also has a number of shortcomings. A summary of the current process
that is very succinct, and skips many of the details, would look like the following:

1. The maintainer team creates a [milestone](https://github.com/crossplane/crossplane/milestones)
   to represent the current/active release
1. Issues are added to the milestone and taken through their lifecycle on the [project
   board](https://github.com/crossplane/crossplane/projects)
1. The community continues to discuss the scope of the milestone and what issues should be included,
   through up front planning sessions, backlog grooming, and daily stand-ups
1. When all issues for the milestone have been completed and merged into `master`, the official
   release process begins
    1. The HEAD commit from `master` is tagged with the release version (e.g.,
       [`v0.3.0`](https://github.com/crossplane/crossplane/releases/tag/v0.3.0)) and the release
       branch is created (e.g.,
       [`release-0.3`](https://github.com/crossplane/crossplane/tree/release-0.3))
    1. Build all release artifacts of the software (e.g., binaries, container images, Helm charts,
       documentation, stack packages) for all supported architectures (`amd64`, `arm64`)
    1. Publish all artifacts to their hosted locations (GitHub, Docker Hub, crossplane.io, S3, etc.)
    1. Promote release to relevant channel (e.g. `alpha`)

### Shortcomings of Current Process

The process outlined above has a number of deficiencies that we would like to improve on:

1. Each release cycle is very long.  We did not release v0.3 for more than 5 months.
1. New features and improvements are not getting into the hands of the community quickly, preventing
   problems from being solved and feedback from being collected in a timely fashion.
1. Each long release culminates with a big deadline for a large number of features. The team feels
   immense stress and pressure because missing the current release means months until their work can
   be included in the next one.
1. The release process itself has a lot of manual steps that are time consuming, error prone, and
   insecure. The cost of performing a release is high and inhibits faster releases from happening.

## Proposed Improvements

In this section, we will cover the set of proposed changes to improve our release engineering and
process.

## Summary

For convenience, a summary of the proposal is included below:

* Increase the frequency of releases to a consistent **monthly** schedule
* Follow [semantic versioning](https://semver.org) so that each monthly release is a minor release
  (e.g. `v0.4.0`)
* Run patch releases (e.g., `v0.4.1`) as needed to address issues with quality and stability in the
  previous release
* Add pipelines to Jenkins that leverage the [build submodule](https://github.com/upbound/build/) to
  automate release tagging/branching, building, publishing, and promoting

## Release Schedule

Increasing our release cadence will have multiple benefits for both users and contributors of the
project. Users will be able to get access to new features and improvements at a faster pace with
less wait time. Contributors will have more frequent opportunities to include their work into a
release, which will decrease the stress and pressure we currently feel leading up to each large and
long awaited release.

We propose that a desirable frequency of release will be **monthly**, as that will provide a
predictable and consistent schedule that gets new features out to the community at a reasonable
pace.  We currently execute on work items in a series of sprints, each lasting 2 weeks.  To
accommodate a monthly release schedule, this sprint schedule will need to be updated.

As part of the timeline that we use to release software, there are many other events along the way
besides the release itself that facilitate a healthy software development lifecycle. A brief
description of these events and their associated goals and expectations can be found below:

* **Milestone planning**: Before a milestone begins, we need to discuss the high level priorities
  and themes, and identify the epics that will deliver on these priorities.
* **Epic discovery**: After epics are identified, we need to do some discovery and investigation
  work to appropriately scope the epics, identify the risks, and agree upon the epic's definition of
  done.
* **Weekly planning**: On a weekly basis, the team can get together for a more focused status check
  with potential course correcting.  We can identify if any priorities have changed and if any
  resources or efforts need to be reorganized. At the end of this meeting, everyone should have a
  clear understanding of what their work will focus on for the week. This is similar to the goal of
  sprint planning previously.
* **Stand-up scrums**: This is a daily event where we quickly meet to talk about our progress, if we
  are blocked on anything, and if we need to have any break-out sessions to help work through
  issues.
* **Backlog grooming and triage**: The backlog of issues and work items needs to be addressed and
  explored on a regular basis, especially in response to user demand.  In this session, we can
  triage new issues to understand their priority, we can revisit items in the backlog to see if any
  priorities have changed, and we can work on the epic discovery mentioned above if that also needs
  attention.
* **Demo party**: This is a chance to celebrate and show off the fruits of our recent labors to the
  rest of the team.  Each developer can show a demo of new things they have been working on to
  collect feedback and disseminate information about our progress and limitations to the rest of the
  team.
* **Retrospective**: After a milestone is completed, a retrospective look back over it can be very
  helpful at identifying where we were successful and what we need to improve on.  This retro should
  result in a series of action items that will help us improve our processes.  These action items
  should be captured as issues in GitHub as appropriate so they can be formally tracked and included
  in planning.

In an attempt to include all of these events in a **monthly** release timeline, we propose the below
regularly recurring schedule for each month. Given our current trajectories, the actual release
itself will occur mid-month, e.g. around the **15th of the month**.

Note that every month will have different alignment for weekdays and weekends, so this schedule is a
guide to the overall structure, not a set of specific dates that will be used every month. The team
should define the specific days/events of each monthly schedule **before** its cycle starts and
those dates should be published to a new `release-schedule.md` that is linked from the main readme
and roadmap.

| Day of release cycle | Event | Expectations |
| ------------ | ----- | ------------ |
|**Week 1**|||
|1 Mon|Milestone starts|(no actual event or meeting held for this)|
|1 Mon|Retrospective from previous release|A public formal retrospective is held for the community to participate in. The events of the previous milestone will be discussed to identify improvements. These improvements should be taken into the first planning of the milestone.|
|2 Tues|Weekly planning|Group planning session to identify priorities and focus for the week.|
|3 Wed|Backlog grooming & epic discovery|(optional) Session to triage new issues and help investigate and flesh out epics.  Early in the milestone, the emphasis is more on epic discovery since we are still working through understanding the epics.|
|**Week 2**|||
|8 Mon|Weekly planning|Group planning session to identify priorities and focus for the week.|
|10 Wed|Backlog grooming|(optional) Session to triage new issues and prioritize the backlog.|
|12 Fri|Halftime Demo party|Show off our progress in demos for the team and take in critical feedback and discuss potential course corrections to set effective strategy.|
|**Week 3**|||
|15 Mon|Weekly planning|Group planning session to identify priorities and focus for the week.|
|17 Wed|Backlog grooming|(optional) Session to triage new issues and prioritize the backlog.|
|**Week 4**|||
|22 Mon|Weekly planning|Group planning session to identify priorities and focus for the week.|
|25 Wed|Backlog grooming & Milestone planning|Session to triage new issues as we get closer to feature freeze.  Milestone planning begins for the **next** milestone as we identify themes, priorities, and high level epics that we want to address in the **next** release.|
|26 Thurs|Feature freeze|Release branch is created on GitHub from `master` and only quality/reliability improvements will be accepted into the release branch afterwards.  No new features will be accepted into the release branch.|
|27 Fri|Final Demo party|Developers give demonstrations of their accomplishments that will be shipping in the release.  Large scoped feedback should already have been collected by this point, only minor quality improvements should be accepted as feedback from the demos.
|**Week 5**|||
|30 Mon|Release published|All artifacts of the release will be published and available for the general community|

## Versioning

While the Crossplane project has already been following [semantic versioning](https://semver.org),
we will be very clear in this section about our versioning intent going forward. For the reader's
convenience, semantic versioning uses a version number format of `MAJOR.MINOR.PATCH`.

In general, before Crossplane is ready for General Availability (GA), we would like to keep the
version number below `v1.0`. This effectively reserves the first **major** release of Crossplane
(`v1.0.0`) for when we are ready to declare it stable and ready for GA. This is in alignment with
the [semver spec](https://semver.org/#spec-item-4), described as such:

> Major version zero (0.y.z) is for initial development. Anything MAY change at any time. The public
> API SHOULD NOT be considered stable.

In the meantime, every monthly release will be a **minor** release, meaning we will be incrementing
the minor version number. For example, the next 3 monthly releases of Crossplane will be `v0.4.0`,
`v0.5.0`, and `v0.6.0`.

### Patch Releases

In addition to monthly minor releases, we will also be able to perform patch releases in the event
that a severe issue is identified in the last release. Patch releases will increment the `PATCH`
version field, for example a patch for the `v0.4.0` release will have a version number of `v0.4.1`.

The exact criteria for what justifies performing a patch release is difficult to identify since
every issue is unique. However, the team will perform triage on any issues reported in the latest
release. In general, a patch release will be justified if an issue greatly affects reliability,
stability, or usability, measured both by severity as well as frequency.

The hope is that our release automation will be simple and streamlined enough that the cost of a
patch release is fairly minimal.

#### Support of Previous Versions

In terms of the scope of previous releases that we will continue to support, we propose to only
support the last/latest release. For example, when `v0.5.0` is released, we will cease to continue
providing patch releases for `v0.4.0`. Only the last release will receive patch updates.

This policy is open to change in the future as we gain more adoption and the community chooses to
stay on older releases for longer. Any decision to change this policy will take into account user
and community feedback.

#### Patch Release Process

Patch releases are always performed from the release branch of the minor release the patch is
applicable to. For example, a patch release of `v0.4.1` would be committed to and built from the
`release-0.4` branch.

1. Issue is first fixed in `master` with the normal PR workflow
    1. If the issue doesn't exist in `master`, perhaps because the offending code has been removed
       since the last release, then the fix PR should be to the release branch
1. A PR is opened in the release branch that contains a cherry-pick of the **fix commit** (not the
   merge commit)
1. The cherry-pick PR is merged into the release branch
1. The patch release pipeline automation is run with an incremented `PATCH` field from the last
   release, e.g. `v0.4.1`.

## Golden Master

To facilitate the quality of releases and the ease of performing a release, we will be adopting a
"golden master" philosophy for the `master` branch. Essentially, all changes that are accepted and
merged into `master` should be **complete** and of high quality. In addition to the functional code
changes, this also includes updating of relevant documentation and examples, test cases and
coverage, etc. In order to make releases at a regular cadence, we cannot allow incomplete,
unfinished, or low quality changes into the `master` branch.

Note that this philosophy is mainly targeted at functionality and behavior that would affect users
on mainline paths and scenarios.  It **can** be reasonable for new functionality that can only be
accessed through deliberate commands by the user to be included into master as well, even if they
are not fully fleshed out scenarios.  This requirement of deliberate action on the users part to
access new and potentially incomplete functionality can be considered a form of feature gate.  We
should consider more formal notion of feature gates in the future to be even more explicit around
new features that are not fully completed yet.

We have captured this "golden master" philosophy in more detail in our "definition of done"
statement in [#857](https://github.com/crossplane/crossplane/pull/857).

## Release Pipeline Automation

Crossplane already has [build automation pipelines](https://jenkinsci.upbound.io/blue/pipelines/)
set up in Jenkins, as well as a
[`Jenkinsfile`](https://github.com/crossplane/crossplane/blob/master/Jenkinsfile) to drive these
pipelines. However, these only perform building and testing from `master` and PRs, and publishing
from `master`. More pipelines are needed to complete the automation of our release process.

Furthermore, all official releases should be performed by the official build pipelines. We have used
a developer laptop to release Crossplane versions in the past, but that is highly undesirable since
it requires access to secret keys and accounts of the privileged build account.

### Build Submodule

The vast majority of build and release logic is contained within the reusable [build
submodule](https://github.com/upbound/build/). There is tested and vetted functionality to perform
many useful actions such as building cross platform binaries and containers, tagging releases,
creating release branches, creating Helm charts, publishing all artifacts including documentation,
promoting releases, etc. It has a ton of generally useful and helpful value for building and
releasing projects.

However, the build submodule does also have some usability issues because of its reliance on `make`
which has been captured in [#852](https://github.com/crossplane/crossplane/issues/852). Making
updates to its behavior or adding new functionality has a fairly steep learning curve and cost,
partly due to not all assumptions and behavior being thoroughly documented. We could consider
implementing this build and release functionality using another platform or framework, but that
would have a very high cost.

Any decisions on our reliance on the build submodule are out of scope for this document. We will
continue to use it with the intent of expediency and quality since it already contains the
functionality that we need and has already been built and tested.

### Pipeline Improvements

The specific pipeline automation improvements that we propose are listed below along with some
details about them.

* **build/publish**: Currently, builds from the release branch will automatically run for new
  commits as part of the
  [`crossplane/crossplane`](https://jenkinsci.upbound.io/blue/organizations/jenkins/crossplane%2Fbuild/activity)
  pipeline. However, this pipeline does not correctly identify the release version, so its
  publishing step does not result in the correct semver container image being published. It is
  believed that [#330](https://github.com/crossplane/crossplane/issues/330) may be the root cause
  for this and we should fix this issue to get automatic publishing of release builds.

* **tag**: This new pipeline would be run on the release branch when we have identified the release
  candidate commit and are ready to tag it. The underlying command run by this pipeline is capable
  of also creating the release branch but will skip that since the release branch was already
  created at the start of feature freeze. This pipeline would run the following build command:

    ```bash
    ./build/run make tag VERSION=${params.version}
    ```

* **promote**: This new pipeline would perform the promotion of a release build to a particular
  channel. We have currently been using `master` and `alpha` channels, and we can consider in the
  future changing to other channels such as `stable` and `edge`, but that is out of scope of this
  document. This pipeline would run the following build command:

    ```bash
    ./build/run make promote BRANCH_NAME=${BRANCH_NAME} VERSION=${params.version} CHANNEL=${params.channel}
    ```

## Stack Releases

While this document has so far described the release process improvements for the [main Crossplane
repository](https://github.com/crossplane/crossplane), there are also some infrastructure stacks
that the Crossplane community currently owns and is responsible for releasing, such as
[`provider-gcp`](https://github.com/crossplane/provider-gcp). Since these stack repositories currently
also take advantage of the build submodule, they will also follow a very similar process as
Crossplane and receive a similar set of improvements.

Recall that a Stack is simply just an [OCI
image](https://github.com/crossplane/crossplane/blob/master/design/design-doc-packages.md#package-format).
Therefore, the build and publishing process can follow essentially the same process that Crossplane
uses. Other functionality in the build submodule, such as creating a Helm chart, is not necessary
for Stacks, but relevant functionality should definitely be reused. For example, building and
publishing the stack package, tagging and creating a release branch, promoting the release, etc.

The GCP, AWS, and Azure stacks will all receive new `tag` and `promote` pipelines similar to
Crossplane, as well as receive a fix for
[#330](https://github.com/crossplane/crossplane/issues/330).

Note that each Stack can be released independently and on their own schedule, but for now they will
follow the same monthly release schedule as Crossplane so the community can receive frequent
updates.  Patch releases for Stacks can also be run entirely independently, so they can have a patch
release on their own schedule even when other Stacks or Crossplane itself is not being patched.

## Package Versioning

Some of the Crossplane source code repositories are expected to be imported as packages only, for
example [crossplane-runtime](https://github.com/crossplane/crossplane-runtime). This means that
they have no release artifacts that need to be published nor follow all the processes described in
this document. However, since many repositories depend on this code, it would still be wise to
specify a versioning strategy that it will follow.

As described in [crossplane-runtime
#11](https://github.com/crossplane/crossplane-runtime/issues/11), the upstream controller-runtime
versioning process is reasonable and communicates the compatibility guarantees for each tagged
version. Full details of this policy can be read in the [controller-runtime versioning
doc](https://github.com/kubernetes-sigs/controller-runtime/blob/master/VERSIONING.md), but here are
the key concepts:

* Semantic versioning should be followed
* Released (tagged) versions should be imported into other projects to ensure they receive
  compatible code
* `master` branch gets all the latest code, which may contain breaking changes
* Breaking changes are included in major releases, while other changes go into semi-immediate
  patches or minor releases

We propose that a full process should be described, agreed upon, and implemented as a follow-up to
this document.  This will be an important issue down the road, but it is not immediately pressing
before crossplane-runtime makes its first stable release.

## Issues and Unknowns

### Upgrade

While Crossplane and Stacks do not have an officially supported upgrade path, this proposal for more
frequent updates may cause some pain within the community. This isn't ideal but can be acceptable
while we are still in alpha. We should prioritize working out our upgrade story to remove this
obstacle and allow the community to take a hard dependency on running Crossplane across versions.
