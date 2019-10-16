# Stack Versioning
* Owner: Luke Weber (@lukeweber)
* Reviewers: Crossplane Maintainers
* Status: Draft, revision 1.0

## Problem

Stacks installer currently relies on installing a particular version of stack from a docker tag. The stack installer in
 the future will be able to support stack update policy, maintenance windows, warnings on major updates, and more.
 To build more complex features around stack version management we will need a common way to express versions that is
 widely understood and standards based, so we will unify on best practices and
 [semantic version 2.0](https://semver.org/spec/v2.0.0.html) as the Crossplane versioning standard.

#### Didn't docker tags already solve this?

 * I don't want my versions underlying artifacts to change:
    * Tags are a symbolic reference to an actual sha of an image and they can be updated or deleted
 * I want to have context from a version:
    * A sha is hard to compare when debugging
      * Which is newer? - cb44dd0 or d2fb617?
      * Is this alpha, beta?
    * A tag like latest is opaque to the end user and so not incredibly useful:
      * Which version am I running? you need to run special commands to figure out which sha is loaded
      * on a node and then lookup that sha to see which version it's tagged against, if it's even still tagged.
      * If the latest tag on this node the same as the latest tag on that node? When did it get restarted?

As a note, if a user wants to continue to install from an arbitrary tag, stack-installer will work, but behavior relying
 on detecting the tags underlying sha changing will not be as the Crossplane stack installer by convention will assume
 the immutability of tags.

#### Why standardize on semantic versions?

Semantic versions are a well adopted standard that give you context just from looking at the version. Based on semantic
 versions we could implement a variety of useful features, that we couldn't reasonably or reliably do if everyone is
 using a unique versioning system.

* Auto-upgrade a major, minor, patch version based on policy.
* Detect if an upgrade is available based only on the tag and the latest version in a channel, no introspecting of a
  docker package.
* Warning if the next latest version is a major upgrade.
* Compare versions running easily based on status of stack install.
* Tie human readable versions to change logs.
* I can remember a version, it's very hard to remember a sha.
* Semantic versions used by helm and widely adopted in kubernetes community.

## Design

We will rely on the established standard of [semantic version 2.0](https://semver.org/spec/v2.0.0.html) embedding these
 versions inside of the stack app.yaml metadata, and inside of the tag in a docker image.

The stack app.yaml version should always be a simple stable semantic version with no pre-release metadata, and tags
 must exist that match the semantic version in the release channel.

Creation dates will be tracked in the release channel versions for auditing purposes and will be iso-8601 format.
 This is the same format as kubernetes and is a well established standard. We will also include a sha id of the
 underlying stack container, which is immutable.

The stack install CRD will get a new optional field for channel in the spec, that will include the url to the channel.

There will be two static lists for channel information, namely the latest available in a channel, as well as the list of
 all available versions for the stack installer to manage more complicated install scenarios based on upgrade policies
 that will be defined in the future.

### Considerations

Our underlying stack storage is a docker registry, which can tag versions of stacks in any arbitrary way, but tags must
 point to a single sha of an actual docker image, so there is no historical context, i.e. previous stable versions.
 Further, if you were to use a :beta tag as a stand in for a beta channel, you would have little recourse to know how to
 downgrade to the previous beta version in the event of a problem, nor would you even know with absolute certainty which
 version you were running by looking at a spec or status in a kubernetes resource.

### Versioning

Semantic versions are widely respected as a standard to version software and are well
 understood in the software community. Helm charts rely on semantic versions.

```
Given a version number MAJOR.MINOR.PATCH, increment the:

MAJOR version when you make incompatible API changes,
MINOR version when you add functionality in a backwards compatible manner, and
PATCH version when you make backwards compatible bug fixes.
Additional labels for pre-release and build metadata are available as extensions to the MAJOR.MINOR.PATCH format.
```

Additional Info: [Semver Reading](https://semver.org/)

Semantic versions for a stack will be managed directly in the channel metadata, and will also be represented in docker
 tags that point to the same underlying images. To avoid confusion, by convention we should never replace a semantic
 version tag of software with a different underlying artifact. We should roll forward with a PATCH version. In the event
 of a security patch we should un-publish the previous stack version when necessary, ideally after publishing a new
 higher version.

### Major Versions

We can assume that major versions like MySQL 5 vs 8 series, might be better served as separate stacks, but the stack
 system will not place arbitrary restrictions on pushing a major update and assume the author of the stack understands
 the implications of a major update and can handle it in the stack upgrade logic.

Having update policies should help mitigate major version upgrade problems in the future.

### Enforcement of Version Upgrades

Versions that need to be replaced will be replaced on a roll-forward basis with a new version. We will allow
 arbitrary version updates that are less than latest to support patch versions to different minor versions of a stack
 in the event that there are multiple supported minor versions by the software vendor.

Rules for valid version:
* MAJOR.MINOR.PATCH version in the app.yaml matches the MAJOR.MINOR.PATCH part from the tag ignoring build info in tag.
* MAJOR.MINOR.PATCH tag must not already be a released stable version else promoting this version makes no sense and
 would conflict.
* By convention we can not modify an existing semantic version tag

### Modes for install (Implicit)

Pinned Install: The user will include a fully qualified package name and a specific tag to install. In this mode we will
 assume an external versioning system is in use and not attempt to check for updates.

Channel Install: If there is a channel set and there is no tag on the package, we will install the latest from the
 channel.

### Hosted artifacts

It is assumed that we will have two hosted artifacts at a channel url, i.e.

Channel url:
https://s3.amazon.com/mybucket/mychannel

Which would contain:
https://s3.amazon.com/mybucket/mychannel/latest # Latest info
https://s3.amazon.com/mybucket/mychannel/all # All version info

The stack installer would initially only use https://s3.amazon.com/mybucket/mychannel/latest, but in the future if
 the user set an upgrade policy it would likely read from all if necessary depending on rules.

### Installing from stable

The stack installer will be version aware, namely it will understand either how to upgrade to the tip of a release
 channel, or how to follow a pre-defined upgrade policy by reading the entire list of available versions.

Consider the following scenario where the installer wishes to auto-upgrade to the lastest version. If the current
 version is 1.2.3 this would be reflected in the stack install status and the package would be set to
 `crossplane/aws-stack`.  The channel would be set to the stable release channel url hosted by s3 or other means with
 the following valid payload from `/latest` from said channel.

```json
{
 "name": "stable",
 "type": "channel",
 "package": "crossplane/aws-stack",
 "latest":
  {
    "version": "1.2.3",
    "id": "461324714c7d",
    "createTime": "2019-09-12T17:39:04Z"
  }
}
```

To enact an upgrade in this scenario, you would update the payload to be a higher semantic version, i.e
 `{"latest": {"version": 1.2.4", ...} ... }`. The installer would then install a new stack image from
 `crossplane/aws-stack:shaofversion` by looking up the actual underlying artifact for that version. The stack installer
 status should reflect that is installing 1.2.4, with the particular sha.

If you update your release channel to something of a lesser value the stack installer would return an error that release
 channel is of a lesser version and not upgrade. Further if we pushed a version that contained pre-release info, this
 would not match as a stable version and would also fail to update. Ideally we would disallow pushing invalid versions
 to channels as part of our tooling.

### Arbitrary channel support

Stable is a reserved channel name to indicate clean semantic versions with no pre-release metadata. Any arbitrary
 lowercase word can be used to denote a channel name and must appear as the first item in the pre-release metadata.

Example of the beta channel:

```json
 {
  "name": "beta",
  "type": "channel",
  "package": "crossplane/aws-stack",
  "latest":
   {
     "version": "1.2.3-beta+123",
     "id": "461324714c7d",
     "createTime": "2019-09-12T17:39:04Z"
   }
 }
 ```

### Support for complex upgrades with a list of versions + update policy

Future work could further build on the latest concept in our release channels and instead define update policy.
 Instead of forcing the stack to only update to the next version, we will support maintenance and upgrade policies.

An example: Imaging the following upgrade policy - Apply patch versions immediately, minor versions in a pre-defined
 weekly maintenance window. In such a scenario, having a list of stable channel versions might be beneficial.
 Version 1.1.2 is installed, but due to a security vulnerability, the author releases a new patch version 1.1.3 and
 un-publishes 1.1.2. The stack-installer could interpret such a policy and choose to immediately install a 1.1.3 patch,
 and schedule a 1.2.3 for the maintenance window.

Example versions list:
```json
{
 "name": "stable",
 "type": "all",
 "package": "crossplane/aws-stack",
 "latest": {"version": "1.2.3", "id": "461324714c7d", "createTime": "2019-09-12T17:39:04Z"},
  "versions": [
     {"version": "1.2.3", "id": "236565319b99", "createTime": "2019-09-12T17:39:04Z"},
     {"version": "1.1.3", "id": "72f201c9f096", "createTime": "2019-09-2T4:39:04Z"},
     {"version": "1.0.3", "id": "ea2d2f10bfdb", "createTime": "2019-09-1T23:39:04Z"}
   ]}
```

### Release tooling

A release tool will need to be built to generate the static artifact that can manage versions of stacks.

Stack Publish would:
 * Optionally take input of a pre-release info `-channelname+buildmeta`
 * Read app.yaml:version and get a semantic version and concat optional pre-release info
 * Check channel to see if semantic version is set and fail if it exists
 * Check that channel name matches semantic version, if stable no metadata
 * Push the container of the stack to docker
 * Push sha + semantic version + date into channel.
 * If semantic version is greater than latest, then replace latest.
 * Add semantic version to list

Stack unpublish would:
 * Remove tag of passed semantic version
 * If latest, throw warning/confirm
 * Delete tag(sem-ver) from docker
 * Remove version from release channel
 * If latest, remove latest version and set to nil.

### Background Research on Versioning

* [Apple iOS](https://developer.apple.com/library/archive/technotes/tn2420/_index.html)
* [Android](https://developer.android.com/studio/publish/versioning)
* [Terraform](https://www.terraform.io/docs/extend/best-practices/versioning.html)
* [Helm](https://helm.sh/docs/developing_charts/#charts-and-versioning)
* [Kubernetes](https://kubernetes.io/docs/setup/release/version-skew-policy/#supported-versions)
