# Crossplane Versioning and Updates

* Owner: Marques Johansson (@displague)
* Reviewers: Crossplane Maintainers
* Status: Draft, revision 1.0

## Outline

* [Overview](#Overview)
* [Versioning](#Versioning)
* [Updates and Rollback](#Updates-and-Rollback)
* [Affected Components](#Affected-Components)
  * [Crossplane](#Crossplane)
    * [Crossplane Runtime](#Crossplane-Runtime)
    * [Crossplane Core](#Crossplane-Core)
    * [Classes and Claims](#Classes-and-Claims)
  * [Stacks](#Stacks)
    * [Specific Managed Resources](#Specific-Managed-Resources)
      * [GCP](#GCP)
      * [AWS](#AWS)
      * [Azure](#Azure)

## Overview

Users wanting to adopt Crossplane need assurances that their applications and managed resources can survive changes in the Crossplane ecosystem.

As the specifications and code changes, users must be able to change versions of
the Crossplane tools and resources. Version changes can be introduced through
updates (both manual and automatic), rolling back, migration of services, or
recovering from a backup or outage where there may be a partial loss of
services or state.

In the first year of Crossplane's existence, users have been given the
expectation that the "v1alpha1" and "0.x.x" version stamps indicate that no
upgrade path will be provided. As such, and understandably, there have been a
number of breaking changes introduced during this rapid development cycle.

Now that some components of Crossplane are being denoted as v1beta1 quality, it is important that an upgrade path be provided for all of the components within the Crossplane ecosystem.

Crossplane has a few independent components that must seek to make an
installation future-ready. Strategies for supporting an extended life-cycle for
each of these components must be planned, implemented, tested, and documented.
Additionally, policies should be established for defining and publishing user
migration paths. The development and operational release requirements and
processes for each of these components should also be documented.

## Versioning

[Semantic Versioning](https://semver.org/) is generally understood and accepted
within the open source community. This is also the case for the Kubernetes
community, which Crossplane commonly uses as an example when establishing practices.

Unsurprisingly, Crossplane and its related component have settled on
semantic versioning 2.0.0 for use in release labels and Git tags.

The versioning decisions and mechanics for Crossplane, Crossplane Runtime, and
Stacks follow a  process described in the [One Pager for Release Process and
Engineering](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-release-process.md).
In Crossplane tools, the build pipeline is integrated into the Git repository.
Therefore, the Git, Docker, and Helm semantic versions align, a
v0.0.4 Helm chart or Docker image is built from the v0.0.4 Git tagged code.
There is no independent package revision.

This versioning story is not complete without identifying exceptions and adding
real world context to the policy.

### Outliers

CRDs are not defined using semantic versioning, more on that in [Types of Concern](#Types-of-Concern).

The [product roadmap](https://github.com/crossplaneio/crossplane/blob/master/ROADMAP.md) for Crossplane was initially primed with versions 0.1 and
0.2. Milestones since these early markers have adopted the semantic
versioning representation of these versions, v0.3.0, v0.7.0, etc.

There are grey areas in semantic versioning about software with a [major
revision of
"0"](https://semver.org/#how-should-i-deal-with-revisions-in-the-0yz-initial-development-phase).
Crossplane has used 0.0.x as a placeholder version. CI/CD tools typically
replace these occurrences with the appropriate version during the build and
publish pipelines. Software in the Crossplane ecosystem will start with 0.1.0
releases, updating the minor version on breaking changes. Once a major version
of 1 has been reached breaking changes will result in updated major versions.

Development releases of Crossplane and Crossplane Stacks will include a release
candidate suffix `-rc`, followed by the number of commits since the `-rc` tag
was introduced, e.g. `0.7.0-rc.4`.

Furthermore, development build Helm charts for Crossplane and Stacks may also be
suffixed by the commit SHA to avoid ambiguity between branches:
`0.7.0-rc.4.gd87543b`.  Use of these Helm releases may require the `--devel`
parameter on Helm commands.

One more Crossplane and Stack version deviation to be aware of is the use of `v`
as a versioning prefix. The use of this prefix is established by convention and
use in contemporary software. In Git and Docker tags `v` is included. The prefix
is not used by in Helm charts, even though the rest of the version identifier
matches Git and Docker.

## Updates and Rollback

Crossplane maintainers have a responsibility to provide a smooth transition
between version changes. Just as software can be updated, it can be rolled back.
Without proper planning this bidirectional flow can be precarious if not
impossible.

Each component of the Crossplane ecosystem should describe how rollbacks can be
issued and what to expect. Upgrades and 

Temporarily delete stack-manager and crossplane controllers and reinstall them
when upgrading fails (without necessarily losing all data/resources)) #847

Per the Kubernetes versioning guidelines (link), Once a stack reaches v1beta1,
it is expected to offer users a conversion webhook.
https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definition-versioning/#webhook-conversion

https://book.kubebuilder.io/multiversion-tutorial/tutorial.html (default
available in 1.15+)
The process of which is
this and that.. and can be run via (how to have it in server) and (how to do it
manually:

https://github.com/kubernetes-sigs/kube-storage-version-migrator?)

## Affected Components

This one pager seeks to outline the components and versioning topics throughout the Crossplane Ecosystem.

Each component needs a separate design on how their upgrade and rollback
process works and the affects of these operations.

These designs should be reflected within this document and others that make
mention of versioning and dependency concerns, including (but not limited to):

* [Crossplane Designs](README.md)
* [Design: Stack](design-doc-stacks.md#Dependency-Resolution)
* [Design: Template Stacks Experience](design-doc-template-stacks-experience.md)
* [One Pager: Release and Engineering Process](one-pager-release-process.md)
* [One Pager: Stack UI Metadata](one-pager-stack-ui-metadata.md)
* [API Docs](../docs/api.md)
* [Crossplane Concepts](../docs/concepts.md)

### Crossplane

Claims and Classes aside, upgrading the Crossplane core has the potential to
change other aspects of the Crossplane installation, including:

* The CRDs of Stacks and StackInstalls
* The Stack-Manager controller (Roles, RoleBindings, SA, Deployment)
* The Crossplane controller (Roles, RoleBindings, SA, Deployment)

We can safely assume that the following will not be changed between versions and
these topics will not be planned for:

* The namespace for the core controllers (crossplane-system) (related)

The following topics are out of scope for this issue:

* Kubernetes version upgrades

#### Crossplane Core

When do you upgrade? (How do you know when.. what are the levels - SemVer)

Who can upgrade?

How do you upgrade? Helm2? Helm3? Kubectl -k? (need to update the installation
docs to note how to upgrade to new versions)

drop helm2 #1100?


What happens?

* what resources are affected
* role changes
* annotation changes?
* controllers restart
* resources replaces or renamed? why and how will you know?
* what can go wrong?
  * potential for data loss?
  * how to prevent problems
  * how to rollback?

#### Classes and Claims

Core Crossplane Classes and Claims will be updated in the same fashion as the
classes and claims included in Stacks, which will be described later.

##### Crossplane Core as a Stack

The managing process that handles these two sets of version changes is different
because the Crossplane Stack Manager handles updates for Stack resources while
the crossplane core image manages upgrades for core classes and claims. Both of
these managers are run from the `crossplane/crossplane` Docker image, but with
separate managers.

It is therefor advisable that the core crossplane components become bundled in a
Stack, managed by the Stack Manager (#1154). In addition to unifying the
process by which these resources are maintained, this provides other benefits of
Stack Manager management:

* Managing the RBAC roles for persona and controller access to the core types
* independent versioning and upgrading
* CRD metadata annotations for UIs
* Stacks can assert the version of core types they expect and require (#1090)

#### Types of Concern

Regardless of how these resources should be handled each of these core claims
and class types can present particular concerns.

Each of the following have both Claim and Class types associated with them.

* Databases - MySQL
* Databases - PostgreSQL
* Cache - RedisClusters
* Storage - Buckets
* Compute - KubernetesClusters
* Compute - MachineInstances
* Workload - KubernetesApplications
* Workload - KubernetesApplicationResources

As types defined by CRD, these resources are versioned as v1alpha1, v1beta1,
etc. When new Claim and Class types are introduced, they will start at v1alpha1.
Crossplane does not guarantee an upgrade path from v1alpha* releases.

This is a deviation from the deprecation policy which Crossplane otherwise
inherits from the broader Kubernetes ecosystem:
<https://kubernetes.io/docs/reference/using-api/deprecation-policy/#deprecating-parts-of-the-api>

Once a resource has been promoted to v1beta1, Crossplane maintainers will strive
to provide [webhooks](https://book.kubebuilder.io/multiversion-tutorial/conversion.html
) to migrate Kubernetes API stored values. (At time of
writing, the work backing this intention, #1152, has not begun.)

CRD conversion webhooks and services will be included in Crossplane by the v1
release.

Claim and Class versions will be matched for a particular resource, even if one
resource (BucketClass, for example) requires a change, while the compliment
(Bucket) does not.  Crossplane Core Claim and Class types will not necessarily
have matching versions, BucketClass could be at v1 while MachineInstanceClass is
at v1alpha1.

#### Crossplane Runtime

Even if the CRD has not changed, it is possible for Crossplane, Stacks, or the
underlying crossplane-runtime to handle CRD types in unexpected ways between
versions. Changes in runtime behavior, will be reflected in Docker image
versions. Semantic versioning rules apply and will inform whether breaking
changes should be expected.

There will also be cases where, without user facing changes in CRDs or the
runtime, there will need to be developer guide changes. Because this
documentation is provided in the github.com/crossplane/crossplane, developers
can expect that these changes will only be introduced as part of new version
tags.

Changes to the Crossplane (and crossplane-runtime) SDK will follow the
rules set for Go modules,
<https://github.com/golang/go/wiki/Modules#semantic-import-versioning>. For example, import paths and types will not be changed without an increment in
the semver minor version.

Additionally, changes to file (import) paths and interfaces should be outlined
in the release notes and changelog. A migration process should be defined for
developers wanting to update their Stacks. A reference PR that applies these
changes to a well supported Stack should be provided if possible.

### Stacks

Crossplane Stacks, a suite of tools to package and manage the installation of
Crossplane specific Kubernetes API extensions, present a deep need for
versioning and dependency solutions.

Stacks are designed to be dependent on the resources provided by other Stacks.
Stacks may require a specific version of a Stack or a specific version of a
sub-resource (CRD) of a Stack. In the future, Stacks may not need to specify the
name of the Stack providing the sub-resource, deferring to a CRD lookup
mechanism, nor the version of the resource, deferring to a channel subscription.
These concerns are being addressed in [#435](https://github.com/crossplaneio/crossplane/issues/435).

For now, Stacks are installed by referencing a specific image name and tag from Docker Hub (or other
registries) using a `package` and `source` parameter to a `StackInstall` or
`ClusterStackInstall` resource. The Stack Manager then unpacks that image
extracting metadata from the Stack filesystem, and creates a `Stack`
resource.  The Stack Manager also reconciles the `Stack` resource, creating RBAC
rules and a controller for this resource.  The details of
this are described in the [Crossplane Stacks design](https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md).

In the realm of Stacks, versioning rules apply to the:

* OCI Image schemaVersion
* Image name and tag
* Stack Filesystem
* CRDs included in the Stack
* Annotations applied to the CRDs
  * UI-Schema
* Dependent CRDs
* RBAC Rules Created for the controller
* Deployment controller image version

#### OCI Image schemaVersion

For the purposes of this document, the [OCI schemaVersion](https://github.com/opencontainers/image-spec/blob/master/manifest.md#image-manifest-property-descriptions) must be understood by
the host environment (Kubernetes) and the [tools that publish
stacks](https://github.com/crossplaneio/crossplane-cli) will rely on `docker`
(>= v1.3.0) or compatible OCI runtime tools to produce OCI images. This topic
will not be discussed further.

#### Image name and tag

The image name and tag of a Stack are defined by the author of the Stack and
consumed by a Stack user within the body of a `StackInstall` `package` property.
The image tag (where version is provided) is left to the Stack author and is not
interpreted by Crossplane tools. Stacks that the Crossplane team maintains are
versioned with semantic versioning as defined in the [Stack Release Process](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-release-process.md#stack-releases).
Independent stack authors are encouraged to [adopt a similar model](https://medium.com/@mccode/using-semantic-versioning-for-docker-image-tags-dfde8be06699).

#### Stack Filesystem

The Stack Filesystem, the layout and content of metadata files within the OCI
image, is currently defined by the Stacks design. A
proposal to define a [version for the Stacks
Filesystem](https://github.com/crossplaneio/crossplane/issues/900) will
establish the rules for interpreting, processing, and converting Stack format
versions.

#### CRDs included in the Stack

The [Kubernetes API versions](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definition-versioning/#version-priority) of the resources provided by Crossplane are
currently on path to becoming `v1beta1` or greater. Historically, Crossplane
APIs undergoing development have had a tendency to stay at `v1alpha1` for too
long, while undergoing many incompatible changes. This has been remedied
throughout the ecosystem, but one should be wary of any Crossplane related tools
using `v1alpha1` for this reason.
CRDs included in the Stack are versioned 

#### Annotations applied to the CRDs

##### UI Schema

While the contents within a [Stack UI
Schema](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-stack-ui-metadata.md#formal-specification)
should be versioned,
[#1192](https://github.com/crossplaneio/crossplane/pull/1192) for example,
Crossplane does not offer an official format for the schema, versioning and
migration behaviors are left to the UI schema designer and implementor.

The Stack filesystem hierarchy and CRD annotation names are ultimately
responsible for defining how UI Schemas are presented. Changes to these will be
represented in updates to the [Stacks design
version](https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md),
the Stack-Manager Docker image version, and, when necessary, the Stacks CRD
version.

#### Dependent CRDs

semver image?

Charts?

Channels / Automatic upgrades?
https://github.com/crossplaneio/crossplane/pull/937

Merging or using the best CRDs (inclusive of the most versions)?
https://github.com/crossplaneio/crossplane/issues/1013

Issues from installing multiple versions of a namespace stack?
https://github.com/crossplaneio/crossplane/issues/1014

Dependency issues from upgrading?
https://github.com/crossplaneio/crossplane/issues/434

access controls need changes between versions?
https://github.com/crossplaneio/crossplane/issues/758

Healing?
https://github.com/crossplaneio/crossplane/issues/1042

StackInstalls / ClusterStackInstalls? (v1alpha1 -> next )

Class resources
Claim resources 

Upgrading resources:
https://github.com/crossplaneio/crossplane/issues/533

#### RBAC Rules Created for the controller
defined in isolation doc, version matches stack version (what is that? where from?)

#### Deployment controller image version

the stack's deployment controller can change how the stack's CRs are processed.
this could affect the external resources, or the kubernetes resources created.
generally a substantial change in a controller will be reflected by a change in
the crd, but not necessarily. see the changelog for that stack.

the version is included in the install.yaml. 
upgrading is left to the user. the stack is intended to represent a single body
of responsible operation, the controller and crd versions supported by that
controller within that stack are intentionally grouped. updating a controller
without the matching crds is not recommended. updating the stack is recommended.

see specific managed resources for how
crossplane maintainers will keep these images compatible and what assurances
there 
link to the issue to make this configurable from stackinstall

#### Specific Managed Resources

##### GCP

v1beta1

* GKECluster - 

##### AWS

v1alpha1?

##### Azure

v1alpha1?

## Internal Links

https://github.com/upbound/upbound-tribe/issues/33
