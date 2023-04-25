# Break Up Large Providers by Service

* Owner: Nic Cope (@negz)
* Reviewers: Hasan Türken (@turkenh), Alper Ulucinar (@ulucinar)
* Status: Draft

## Background

The most popular and largest Crossplane providers install hundreds of CRDs.
No-one actually wants to use all of them. Installing them brings a severe
performance penalty - enough to make some hosted Kubernetes services such as GKE
unresponsive.

Here are the large providers we have today:

* https://marketplace.upbound.io/providers/upbound/provider-aws/v0.30.0 - 872
* https://marketplace.upbound.io/providers/upbound/provider-azure/v0.28.0 - 692
* https://marketplace.upbound.io/providers/upbound/provider-gcp/v0.28.0 - 325
* https://marketplace.upbound.io/providers/crossplane-contrib/provider-tencentcloud/v0.6.0 - 268
* https://marketplace.upbound.io/providers/crossplane-contrib/provider-aws/v0.37.1 - 178
* https://marketplace.upbound.io/providers/frangipaneteam/provider-flexibleengine/v0.4.0 - 169
* https://marketplace.upbound.io/providers/scaleway/provider-scaleway/v0.1.0 - 73

Based on a brief survey of the community and Marketplace, no other provider
installs more than ~30 CRDs.

Quantifying the performance penalty of installing too many CRDs is hard, because
it depends on many factors. These include the size of the Kubernetes control
plane nodes and the version of Kubernetes being run. As recently as March 2023
we’ve seen that installing just `provider-aws` on a new GKE cluster can bring
the control plane offline for an hour while it scales to handle the load.

Problems tend to start happening once you’re over - very approximately - 500
CRDs. Keep in mind that many folks are installing Crossplane alongside other
tools that use CRDs. Here’s a brief survey of the number of CRDs common tools
install:

* Crossplane (without any providers) - 12
* Google Config Connector - ~200
* Azure Service Operator - ~190
* Kyverno - 12
* Istio - 14
* Argo Workflows - 8

Installing too many CRDs also affects the performance of Kubernetes clients like
kubectl, Helm, and ArgoCD. These clients are often built under the assumption
that there won’t be more than 50-100 CRDs installed on a cluster.

You can read more about the issues Kubernetes faces when too many CRDs are
present at:

* https://github.com/crossplane/crossplane/blob/v1.11.2/design/one-pager-crd-scaling.md
* https://blog.upbound.io/scaling-kubernetes-to-thousands-of-crds/

Apart from performance issues, some folks have expressed concerns about the
security and general “bloat” of having many unused CRDs installed.

Crossplane maintainers have invested in improving the performance of the
Kubernetes control plane and clients in the face of many CRDs. These
improvements have been insufficient to alleviate community pain. The assumption
that there won’t be “too many” CRDs is pervasive in the Kubernetes codebase.
There’s no one easy fix, and the conservative speed at which cloud providers and
their customers pick up new Kubernetes releases means it can take years for
fixes to be deployed.

Allowing folks to lower the ratio of installed-to-used CRDs has been proposed
for some time. The Crossplane maintainers are now aligned that it’s necessary.
We have explored the following options:

* Make it possible to filter unnecessary CRDs by configuring a provider.
* Break the biggest providers up into smaller, service-scoped providers.
* Load CRDs into the API server lazily, on-demand.

## Goals

The goal of this proposal is to eliminate _or significantly defer_ the
performance issues that arise when installing too many CRDs. By "deferring"
performance issues, I mean buying time for the Kubernetes ecosystem to improve
its performance under CRD load.

## Non-goals

The goal is not that no Crossplane deployment _ever_ crosses the ~500 CRD mark.
Instead, that should be a rare edge case until the vast majority of Kubernetes
deployments can handle it without issue.

This proposal _does not_ intend to address any security concerns related to
unused CRDs being present in a Crossplane deployment. At this time the
significance of these security concerns is not well understood. It's also
unclear how many community members are concerned, relative to the overwhelming
concern about performance.

At the present time I do not believe installing a CRD (and controller) that you
don't intend to use poses a significant risk. At least two layers of security
prevent anyone using the Crossplane Managed Resources (MRs) these CRDs define -
Kubernetes RBAC in front of the API call, and cloud IAM permissions behind it.

## Proposal

I propose that provider maintainers break the largest providers into smaller,
service-scoped providers. Each service-scoped provider would correspond to what
is currently an API group within a provider. For example,
`upbound/provider-aws-rds` would extend Crossplane with support for the 22
managed resources (and thus 22 CRDs) in the `rds.aws.upbound.io` API group.

### Overview

I proposed we only break up the largest providers. For example:

* `upbound/provider-aws` - Becomes ~150 smaller providers.
* `upbound/provider-azure` - Becomes ~100 smaller providers.
* `upbound/provider-gcp` - Becomes ~75 smaller providers.
* `crossplane-contrib/provider-tencentcloud` - Becomes ~50 smaller providers.

It's important to note that we don't expect anyone to be installing and
operating all of these providers. Per [Community Impact](#community-impact), we
expect folks to install ~10 providers on average.

These providers would:

* Be built from “monorepos” - the same repositories they are today, like
  `upbound/provider-aws`.
* Share a `ProviderConfig` by taking a dependency on a (e.g.)
  `provider-aws-config` package.
* Continue to use contemporary “compiled-in” cross-resource-references, short to
  medium term.

Provider generation tooling like [`upjet`][upjet] would be updated to support
generating service-scoped providers.

Minimal changes would be required to Crossplane core. No changes would be
required to the long tail of existing, smaller Crossplane providers such as
`provider-terraform` and `provider-kubernetes`.

The following are the high level pros and cons of the proposed approach.

Pros

* No new concepts or configuration required.
* Fewer CRDs is the default user experience
* Simple, non-disruptive migration path from existing providers.
* Folks can create their own “meta-providers” (e.g. `acmeco/provider-aws`) that
  pull in just the services they want. This is just packaging - no compiling
  required.
* Provider work spread over many provider processes (i.e. pods) should scale
  better.
* Granular deployments. You could upgrade the EKS provider without having to
  upgrade RDS. 
* No major changes to Crossplane core, which means the rollout is uncoupled from
  Crossplane's quarterly release cycle.

Cons

* Granular deployments. More providers to upgrade, monitor, etc.
* Does not achieve a perfect 1:1 ratio of installed-to-used CRDs.
* The average deployment may require more compute resources, short term.

I'd like to expand on the first pro a little. No new concepts or configuration
required. To me, this is the key benefit of the proposed approach, and the
reason I believe it's the best option for our project and community, long term.

Imagine you're learning how Crossplane works for the first time. The first two
steps on your getting started checklist are probably:

1. Install Crossplane.
2. Install the Provider(s) for the things you want to manage.

Keep in mind today that the things you might want to manage might be a cloud
provider like AWS, but it might also be Helm charts, SQL databases, etc. Most
folks today are running more than one provider.

Under this proposal, this does not change. You'd explain how to get started with
Crossplane in exactly the same way. The only difference is that in some cases
you'd think about "the things you want to manage" in a more granular way. You'd
think "I want RDS, ElastiCache, CloudSQL, GKE, and Helm" not "I want AWS, GCP,
and Helm".

I would argue that most folks already think this way. Folks don't think "I want
AWS support" without having some idea which AWS _services_ they want to use.
Confirming "Does it support AWS?" is a tiny speed bump before you confirm "Does
it support RDS?" (or whatever services you need).

### Community Impact

Based on a survey of ~40 community members, the average deployment would:

* Install ~176 provider CRDs, down from over 900 today.
* Run ~9 provider pods, up from ~2 today.

Keep in mind this represents where folks are at with Crossplane today. Over time
we expect these numbers to grow as production deployments mature.

### Cross-Resource References

Some MRs can reference other MRs; for example an EKS cluster MR can reference a
VPC MR to indicate that it should be connected to that VPC. We call these
cross-resource references.

Today cross-resource references are implemented at the code level. That is, the
EKS cluster MR controller needs to be compiled with knowledge of the VPC MR type
and how to reference it.

Even outside of this proposal this is limiting; it’s not possible today to make
a reference from one provider to another. Doing so would require all providers
to be compiled against all other providers. This means for example that a
provider-helm Release MR cannot today reference a `provider-aws` EKS cluster MR.

In the medium-to-long term I believe contemporary cross-resource references
should be deprecated in favor of a generic alternative that supports references
across providers. This is tracked in Crossplane issue [#1770][issue-1770].

In the short term, breaking up the biggest providers would cause many existing
cross-resource references to be from one provider to another. For example today
an EKS-cluster-to-VPC reference is “internal” to a single provider
(`provider-aws`). If providers were broken up by API group this reference would
cross provider boundaries; from `provider-aws-eks` to `provider-aws-ec2-core`.

I’m not concerned about this as a short term problem. As long as all AWS
providers (for example) are built from a monorepo it should be possible to
compile them with support for the types they reference. This does in theory
create a coupling between AWS providers, but I don’t expect this to be an issue.

Consider for example an EKS cluster that references a VPC by its external name
annotation. Then assume we discarded external names as a concept. Both the EKS
and EC2 providers would need to be upgraded to support this change - there’s a
coupling. In practice cross-resource reference implementations rarely change
once implemented, so I think this problem is largely theoretical.

### Provider Configs

All Crossplane providers install CRDs for a few special custom resources (CRs):

* ProviderConfig - Configures the provider. Typically with authentication
  credentials.
* ProviderConfigUsage - Tracks usage of a ProviderConfig. Prevents it being
  deleted.
* StoreConfig - Configures how to access a Secret Store.

In the [upstream Crossplane issue][issue-3754] that tracks this proposal a key
theme was that community members don’t want to manage more ProviderConfigs than
they do today. A community member running 10 smaller AWS providers, each with
MRs spanning 10 AWS accounts would go from 10 ProviderConfigs to 10 x 10 = 100
ProviderConfigs.

ProviderConfigs are typically created by humans, or as part of a Composition. MR
controllers create a ProviderConfigUsage for each MR. A ProviderConfig
controller places a finalizer on each ProviderConfig to prevent it from being
deleted while ProviderConfigUsages reference it.

I propose that as part of breaking up the biggest providers, each provider
"family" include a special “config” provider - e.g. `provider-aws-config`, or
`provider-azure-config`. This config provider would:

* Install the ProviderConfig, ProviderConfigUsage, and StoreConfig CRDs.
* Start a controller to prevent the ProviderConfig being deleted while
  ProviderConfigUsages referenced it.

All other providers in the provider family would take a package manager
dependency on the relevant config provider. This:

* Ensures the config provider is installed automatically.
* Reduces coupling - dependent providers can depend on a semantic version range.

Consider the following scenarios:

* A ProviderConfig is updated with a new field.
* A ProviderConfig enum field is updated with a new value

The first scenario is well suited to semantic version dependencies. The new
field would need to be optional to avoid a breaking schema change to the
ProviderConfig type (this is true today). Providers depending on “at least
version X” of the config provider would be unaffected by updates; they would
simply ignore the new optional field. Providers that wished to leverage the new
field would update their dependency to require at least the version in which the
field was introduced.

The second scenario is a little trickier. If the config provider was updated and
existing ProviderConfigs were updated in place to use the new enum value (e.g.
`spec.credentials.source: Vault`) all dependent providers would need to be
updated. Any that weren’t would treat the new enum value as unknown. In this
scenario the best practice would be to either update all providers with support
for the new value, or update only some providers and have them use a new
ProviderConfig using the new value, leaving the existing ProviderConfigs
untouched.

### Inter-Provider Dependencies

As established in the [Provider Configs](#provider-configs) section, all
providers within a provider family will depend on a single config provider. For
example all `provider-aws-*` providers will have a package dependency on
`provider-aws-config`.

A common misconception is that providers will need to depend on the providers
they may (cross-resource) reference. This is not the case. I expect that the
_only_ dependency providers within a family will have is on their config
provider.

Each cross resource reference consists of three fields:

1. An underlying field that refers to some other resource. For example
   `spec.forProvider.vpcID` may refer to a VPC by its ID (e.g. `vpc-deadbeef`).
2. A reference field that references a Crossplane VPC MR by its `metadata.name`.
3. A selector field that selects a resource to be referenced.

These are resolved in selector -> reference -> underlying field order.

Note that it's not actually _required_ to specify a reference or selector. Even
if `vpcID` is (in practice, if not schema) a required field you could just set
the `vpcID` field directly. In this case there is no Crossplane VPC MR to
reference; you don't need `provider-aws-ec2-core` to be installed at all.

If you _did_ want to reference a VPC MR you'd need to have one first. Therefore
you install `provider-aws-ec2-core` to _create a VPC MR_, not because you want
to _reference a VPC MR_.

Following this logic, there's no need for a provider to take a package
dependency on `provider-aws-ec2-core`. If you know you'll want to manage (and
reference) VPCs you can install `provider-aws-ec2-core` directly. If you don't,
don't.

### Compute Resource Impact

I expect 'native' providers (i.e. providers that aren't generated with `upjet`)
to use less CPU and memory under this proposal. This is because each extra
provider pod incurs a _little_ extra overhead, folks will have a lot fewer
Crossplane controllers running. Each controller, even at idle, uses at least
some memory to keep caches and goroutines around.

For upjet-based providers it's a different story. Currently upjet-based
providers use a lot of CPU and memory. There’s an ongoing effort to address
this, but we suspect we can get providers down to _at least_:

* ~600MB memory for the Crossplane provider process.
* ~50MB per Terraform CLI invocation (we run 2-3 of these per reconcile).
* ~300MB per Terraform provider process.

The current thinking is that we need to run one Terraform provider process for
each ProviderConfig that is used by at least one MR. So for example one upjet
provider:

* Doing nothing would use ~600MB memory.
* Simultaneously reconciling 10 MRs all sharing one ProviderConfig would use ~1.4GB memory.
* Simultaneously reconciling 10 MRs split across two ProviderConfigs would use ~1.7GB memory.

We’re also currently seeing heavy CPU usage - roughly one CPU core to reconcile
10 MRs simultaneously. One theory is that the Terraform CLI is using a lot of
CPU, but we need to profile to confirm this.

By breaking up big upjet-based providers we expect to see:

* No change to the compute resources consumed by Terraform CLI invocations.
* Fewer compute resources consumed by Crossplane provider processes.
* Significantly more compute resources consumed by Terraform provider processes.

We expect CPU and memory used by Terraform CLI invocations to remain the same
for the same number of MRs. This is because the upjet providers invoke the
Terraform CLI 2-3 times per reconcile. The number of reconciles won’t change -
they’ll just be spread across multiple Provider pods.

We expect fewer compute resources (in particular less memory) will be consumed
by Crossplane provider processes because we’ll be running fewer controllers
overall. Today `provider-aws`, at idle, must start almost 900 controllers. One
for each supported kind of managed resource. Each controller spins up a few
goroutines and subscribes to API server updates for the type of MR it
reconciles. By breaking up providers significantly fewer controllers would be
running, using resources at idle.

We expect significantly more compute resources to be consumed by Terraform
provider processes because there will need to be more of them. It’s safe to
assume each deployment of `provider-aws` becomes ~7 smaller deployments on
average. For a single ProviderConfig, each of these smaller deployments would
incur the ~300MB of memory required to run a Terraform provider process. This
doubles as we increase to two ProviderConfigs, etc.

The increased compute resources consumed by needing to run Terraform providers
is a serious concern. I’m not considering it a blocker for this proposal because
we feel there is a lot of room for improvement in the medium to long term. In
addition to the current steps being investigated to improve performance, we
could consider:

* Replacing the most widely-used MR controllers with native implementations that
  use dramatically fewer resources.
* Cutting out Terraform CLI invocations by talking directly to the Terraform
  provider process.
* Cutting out both Terraform CLI invocations and Terraform provider processes by
  importing the Terraform provider code directly. Terraform makes this hard to
  do, but it’s possible. It appears to be what Config Connector does.

### Rollout and Migration

As with any major Crossplane change, it's important to allow the community to
experiment and provide feedback before we commit. Given that this change is
largely decoupled from Crossplane core and its release cycle, I propose that we:

* Continue to publish the existing, too-large, providers for now.
* Begin publishing smaller, service scoped providers in parallel.

These smaller providers would be marked as 'release candidates' to begin with. I
prefer this to 'alpha' in this context as it better communicates that the
_packaging_ is what's changing. The code itself does not become any less mature
(it's today's existing providers, repackaged).

Once we have confidence in the service scoped provider pattern we would mark the
too-large providers as deprecated and set a date at which they would no longer
be built and published.

Migration from (for example) `upbound/provider-aws` to `upbound/provider-aws-*`
should not be significantly more complicated or risky than migrating from one
version of `upbound/provider-aws` to the next. This is because there would be no
schema changes to the MRs within the provider.

To upgrade a control plane running (for example) `upbound/provider-aws` with
several existing MRs in-place, the process would be:

1. Install the relevant new `upbound/provider-aws-*` provider packages, with
   `revisionActivationPolicy: Manual`. This policy tells the providers not to
   start their controllers until they are activated.
2. Uninstall the `upbound/provider-aws` provider package.
3. Set the `revisionActivationPolicy` of the new `upbound/provider-aws-*`
   provider packages to `Automatic`, letting them start their controllers.

Behind the scenes the Crossplane package manager:

1. Adds owner references to the CRDs the new provider packages intend to own.
2. Removes owner references from the CRDs the old provider package owned.

As the old provider is uninstalled, the CRDs it owned will be garbage collected
(i.e. deleted) if no new provider added an owner reference. If a new provider
_did_ add an owner reference it will become the only owner of the CRD and mark
itself as the controller reference. Each CRD can have only one controller
reference, which represents the provider that is responsible for reconciling it.

### RBAC Manager Updates

The only change required to Crossplane core to implement this proposal is to the
RBAC manager. Currently the RBAC manager grants a provider (i.e. a provider
pod's service account) permission to access it needs to reconcile its MRs and to
read its ProviderConfig.

Under this proposal many providers would need to access a ProviderConfig that
was installed by another provider (within its family). At first, providers would
also need access to any MRs from other providers in their family that they might
reference.

To support this, the RBAC manager would need to grant a provider permission to
all of the CRDs within its family. This would be done by adding a new
`pkg.crossplane.io/provider-family` label to the package metadata in
`crossplane.yaml`. This label will be propagated to the relevant
`ProviderRevision`, allowing the RBAC manager to easily discover all revisions
within a family and thus all CRDs owned by revisions within a family.

It's worth noting that these updates are not strictly _necessary_ but do bring a
large improvement in user experience. Today it's possible for a user to avoid
running the RBAC manager and curate their own RBAC rules. In theory community
members could run Crossplane without these RBAC manager changes, but would be
faced with the onerous task of manually granting access to all CRDs within a
provider family.

## Future Improvements

The following improvements are out of scope for this initial design, but could
be made in future if necessary.

### Large Services

One service API group from each of the three major providers is quite large:

* `ec2.aws.upbound.io` (94 CRDs)
* `compute.gcp.upbound.io` (88 CRDs)
* `networking.azure.upbound.io` (100 CRDs)

`datafactory.azure.upbound.io` is also quite large at 45 CRDs. All other
services currently have 25 or fewer CRDs.

These large services contain types that are very commonly used. For example
`ec2.aws.upbound.io` contains the `VPC` and `SecurityGroup` types, which are
dependencies of many other services. To deploy an EKS cluster you may want to
create and reference a `VPC` and a `SecurityGroup` (or three). To do so, you
would need to bring in `provider-aws-ec2` and thus install ~90 superfluous CRDs.

In these cases where a service:

* Is large, containing 25 or more CRDs
* Is commonly used as a dependency of other services
* Contains a few 'core' types alongside many less commonly used types

We could break the service up into two providers; "core" and "everything else".
For example in the case of `ec2.aws.upbound.io` we would create:

* `upbound/provider-aws-ec2-core` - Contains the ~10-20 most commonly used types
  (e.g. `VPC`, `SecurityGroup`, etc.
* `upbound/provider-aws-ec2` - Contains the remaining types.

The `ec2.aws.upbound.io` API group would be split across these two providers.
The core provider would be a dependency of the full provider, such that:

* Installing `upbound/provider-aws-ec2` installed support for all of EC2,
  including the things in core (as a dependency).
* Installing `upbound/provider-aws-ec2-core` installed support for just the
  core types.

We would like to avoid making this change until there is signal that it's
necessary (i.e. folks are starting to bump up on CRD limits again). We believe
that breaking up providers by cloud service (where possible) is easier to reason
about than more arbitrary distinctions like 'core' and 'everything else'.

## Alternatives Considered

I considered the following alternatives before making this proposal.

### CRD Filtering

The gist of this proposal is to update the existing Provider.pkg.crossplane.io
API to include a configurable allow list of enabled CRDs. As far as I can tell
this is the only feasible alternative to breaking up the largest Providers.

#### Overview

Two proof-of-concept implementations for this proposal have been developed:

* https://github.com/crossplane/crossplane/pull/2646
* https://github.com/crossplane/crossplane/pull/3987

In each case the proposed API looks something like:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-aws
spec:
  package: upbound/provider-aws:v0.40.0
  # A new, optional field enables matching APIs (i.e. CRDs). When this field is
  # not specified all APIs are enabled.
  enabledAPIs:
  - "vpcs.ec2.aws.upbound.io"
  - "*.rds.aws.upbound.io"
```

Each time the `enabledAPIs` field was updated a new `ProviderRevision` would be
created, with an immutable set of `enabledAPIs`. This would cause the Provider
to be restarted to reconfigure itself with support for the updated set of
enabled APIs.

Many community members find this alternative appealing. On the surface it
appears to be a much smaller change relative to breaking up providers. This is
especially true for folks who are already using Crossplane today, and who have
already deployed the contemporary, too-large providers. It also makes it
possible to achieve a perfect 1:1 installed-to-used ratio of CRDs. You can
optimize your control plane to never install any (Crossplane) CRD that you don't
intend to use.

I do not recommend we pursue this alternative because I believe it in fact
increases the complexity of Crossplane - making it harder to reason about. In
some ways this is obvious. Instead of:

1. Install Crossplane.
2. Install the Providers you need.

The flow becomes:

1. Install Crossplane.
2. Install the Providers you need.
3. Configure which parts of the Providers you want to use.

This alone may not seem so bad, but there are further constraints to consider.
I'll touch on some of these below.

#### Default Behavior

The first constraint is that we cannot make a breaking change to the v1
`Provider` API. This means the new `enabledAPIs` field must be optional, and
thus there must be a safe default. Installing all CRDs by default is not safe -
we know installing a single large provider can break a cluster. Therefore this
cannot continue to be the default behavior.

Instead we could install _no_ MR CRDs by default, but this is a breaking (and
potentially surprising) behavior change. Updating to a version of Crossplane
that supported filtering would uninstall all MR CRDs. It may also be challenging
for a new Crossplane user to even discover what APIs (CRDs) they can enable.

The safest and most compatible option therefore seems to be to install no MR
CRDs by default, but to have the package manager automatically detect and
implicitly enable any APIs that were already in use.

#### Adding Support to Providers

Support for filtering would need to be built into the Crossplane package
manager, but also built into every Crossplane provider. It's the package manager
that extracts CRDs from a provider package and installs them, but currently
providers assume they should start a controller for every CRD they own. If any
of those CRDs aren't enabled, the relevant controllers will fail to start.

This raises two questions:

1. Do we need to add support to all providers?
2. How do we handle providers that don't support filtering?

Note that we need to answer the second question regardless of the answer to the
first. Even if we required all providers to support filtering there would be a
lengthy transitional period while we waited for every provider to be updated.

Adding filtering support to all providers seems a little pointless. The majority
of the long tail of providers deliver so few CRDs that there's little reason to
filter them. Many, like `provider-helm`, `provider-kubernetes`, and
`provider-terraform` have only a single MR CRD. There would be no practical
reason to install the provider but disable its only CRD.

Presuming we didn't require all providers to support filtering, we would need to
figure out what happens when someone tries to specify `enabledAPIs` for a
provider that didn't support it. I believe ideally this would involve adding
some kind of meta-data to providers that _did_ support filtering so that we
could return an informative error for those that did not. Otherwise, depending
on how the package manager told the provider what controllers to enable, the
error might be something unintuitive like "unknown CLI flag --enable-APIs".

#### Package Dependencies

In Crossplane a `Configuration` package can depend on a `Provider`. For example
https://github.com/upbound/platform-ref-aws has the following dependencies:

```yaml
apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: platform-ref-aws
spec:
  dependsOn:
  - provider: xpkg.upbound.io/upbound/provider-aws
    version: ">=v0.15.0"
  - provider: xpkg.upbound.io/crossplane-contrib/provider-helm
    version: ">=v0.12.0"
```

In practice `platform-ref-aws` depends on `provider-aws` because it contains
Compositions that use several AWS MRs, including RDS Instances, EC2 VPCs, and
EKS Clusters.

If we were to implement CRD filtering at the `Provider` level we wouldn't be
able to satisfy these dependencies; they don't contain enough information. A
`Configuration` could depend on a `Provider` because it wanted to use RDS
instances, and the dependency would be satisfied by the `Provider` simply being
installed, regardless of whether the desired RDS APIs were actually enabled.

A solution here might be to expand `dependsOn` to include a list of the actual
resources being depended upon. Each time a `Configuration` was installed the
relevant provider would need to be reconfigured and restarted to enable any
additional APIs. The set of enabled APIs would be those explicitly specified,
plus those implicitly enabled by any `Configuration` dependencies.

This option has its own challenges; again we could not make this new field
required - that would be a breaking API change. We would need to figure out what
to do if it was omitted. One option might be to scan the contents of the
`Configuration` package (e.g. its Compositions) to determine what MRs it uses.
If Composition Functions were in use we'd also need to decorate them with
information about what MRs they might create, since that information is not part
of the Composition where the functions are called.

#### Conclusion

I do believe that it would be possible build support for CRD filtering. It is a
solvable problem.

It would be a significant amount of work, and that work would be subject to the
typical alpha, beta, GA release cycle due to the extent of the changes to
Crossplane. This means it would take longer to get into people's hands, but that
alone is not a good reason to discount the approach.

The main reason I suggest we do not pursue this alternative is that I believe it
adds a lot of cognitive complexity to Crossplane. In addition to needing to
understand the concept of filtering itself as a baseline, folks need to
understand:

* What happens when you update the filter?
* What happens when you don't specify a filter?
* What happens when you update from a version of a provider that does not
  support filtering to one that does?
* What happens when your configuration depends on a provider that supports
  filtering?
* What happens when you specify a filter but the provider does not support it?

This is a lot for Crossplane users to reason about, and in most cases the answer
to the question is implicit and "magical" behavior.

### Lazy-load CRDs

The gist of this proposal is that we find a way to lazily load CRDs into the API
server - i.e. install them on-demand as a client like kubectl tries to use the
types they define. We’ve explored three possible ways to do this:

* By running a proxy in front of the API server.
* By delegating part of the API server’s API surface to an aggregation API
  server.
* By using a mutating or admission webhook.

Of these options the aggregation API server seems to be the most feasible. We
found that an API request to create a non-existent type was rejected before a
webhook gets a chance to load the CRD for the type on-demand. The proxy option
would require all Kubernetes clients to talk to the proxy rather than talking to
the API server directly. This is probably not feasible in the wild.

I believe all forms of lazy-loading to be ultimately infeasible due to the
discovery API (see the links in [Background](#background) if you’re unfamiliar
with discovery and its issues). In order for clients like kubectl to try to use
(e.g. list or create) a particular kind of MR it must exist in the discovery
API. This means all supported types would need to exist in the discovery API
even if their CRDs weren’t yet loaded. It would not be possible to trigger
lazy-loading of a CRD on its first use if clients never tried to use it because
they didn’t think it existed.

Currently most Kubernetes clients have a discovery API rate limiter that allows
bursts of up to 300 requests per second. With the three big Official Providers
“installed” (whether there CRDs were actually installed or setup to be
lazily-loaded)  there would be over 325 discovery API endpoints (one per API
group). This means clients would make over 300 requests and rate-limit
themselves (and log about it).

A [much more efficient discovery API][kep-aggregated-discovery] is being
introduced as beta with Kubernetes v1.27 in April. We expect this API to
eliminate most discovery issues, but based on historical release timeframes we
don’t expect it to be widely deployed until 2024. It’s not likely to be
available at all on EKS until late September 2023.

One issue that won’t be eliminated by the more efficient discovery API is CRD
category queries like `kubectl get managed` and `kubectl get crossplane`. These
queries are inherently inefficient. Clients must first discover all supported
types (i.e. walk the discovery API) to determine which types are in a particular
category, then make at least one API request per type to list instances. This
means that even if the ~900 `provider-aws` CRDs aren’t actually loaded, kubectl
will try to make ~900 API requests for the types it sees in the discovery API
when someone runs kubectl get managed.

There would be quite a lot of work required to both Crossplane core and
providers to implement this feature. The core work would be subject to the
typical alpha, beta, GA release cycle.

[upjet]: https://github.com/upbound/upjet
[issue-1770]: https://github.com/crossplane/crossplane/issues/1770
[issue-3754]: https://github.com/crossplane/crossplane/issues/3754
[kep-aggregated-discovery]: https://github.com/kubernetes/enhancements/issues/3352