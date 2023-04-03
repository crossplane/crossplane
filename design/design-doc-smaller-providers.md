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

https://marketplace.upbound.io/providers/upbound/provider-aws/v0.30.0 - 872
https://marketplace.upbound.io/providers/upbound/provider-azure/v0.28.0 - 692
https://marketplace.upbound.io/providers/upbound/provider-gcp/v0.28.0 - 325
https://marketplace.upbound.io/providers/crossplane-contrib/provider-tencentcloud/v0.6.0 - 268
https://marketplace.upbound.io/providers/crossplane-contrib/provider-aws/v0.37.1 - 178
https://marketplace.upbound.io/providers/frangipaneteam/provider-flexibleengine/v0.4.0 - 169
https://marketplace.upbound.io/providers/scaleway/provider-scaleway/v0.1.0 - 73

Based on a brief survey of the community and Marketplace, no other provider
installs more than ~30 CRDs.

Quantifying the performance penalty of installing too many CRDs is hard, because
it depends on many factors. These include the size of the Kubernetes control
plane nodes and the version of Kubernetes being run. As recently as March 2023
we’ve seen that installing just provider-aws on a new GKE cluster can bring the
control plane offline for an hour while it scales to handle the load.

Problems tend to start happening once you’re over - very approximately - 500
CRDs. Keep in mind that many folks are installing Kubernetes alongside other
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
security and general “bloat” of having many unused CRDs installed. I believe the
security concerns to be an educational issue - I haven't yet seen a convincing
argument for removing CRDs as opposed to using RBAC.

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
  upbound/provider-aws.
* Share a `ProviderConfig` by taking a dependency on a (e.g.)
  `provider-aws-config` package.
* Continue to use contemporary “compiled-in” cross-resource-references, short to
  medium term.

Provider generation tooling like [`upjet`][upjet] would be updated to support
generating service-scoped providers.


No changes would be required to Crossplane core - i.e. to the package manager.
No changes would be required to the long tail of existing, smaller Crossplane
providers such as `provider-terraform` and `provider-kubernetes`.

The following are the high level pros and cons of the proposed approach.

Pros

* Fewer CRDs is the default user experience - no new concepts or configuration
  required.
* Requires no changes to Crossplane core - no waiting months for core changes to reach
  beta and be on by default.
* Provider work spread over many provider processes (i.e. pods) should scale
  better.
* Folks can create their own “meta-providers” (e.g. `acmeco/provider-aws`) that
  pull in just the services they want. This is just packaging - no compiling
  required.
* Granular deployments. You could upgrade the EKS provider without having to
  upgrade RDS. 

Cons

* The average deployment may require more compute resources, short term.
* At least three widely used service groups (including EC2) are still quite
  large (~100 CRDs).
* Some folks are going to feel it's not enough granularity.


### Community Impact

Based on a survey of ~40 community members, the average deployment would:

* Install ~100 provider CRDs, down from over 900 today.
* Run ~9 provider pods, up from ~2 today.

A “multi-cloud Kubernetes” scenario in which a hypothetical Crossplane user
installed support for EKS, AKS, and GKE along with the Helm and Kubernetes
providers would install 10 providers and 348 CRDs. This scenario is interesting
because it triggers the pathological case of needing to install the biggest API
group from each of the three major providers - `ec2.aws.upbound.io` (94 CRDs),
`compute.gcp.upbound.io` (88 CRDs), and `networking.azure.upbound.io` (100
CRDs). Note that this scenario is still well below the approximately 500 CRD
mark at which folks may start seeing issues.

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
provider-helm Release MR cannot today reference a provider-aws EKS cluster MR.

In the medium-to-long term I believe contemporary cross-resource references
should be deprecated in favor of a generic alternative that supports references
across providers. This is tracked in Crossplane issue [#1770][issue-1770].

In the short term, breaking up the biggest providers would cause many existing
cross-resource references to be from one provider to another. For example today
an EKS-cluster-to-VPC reference is “internal” to a single provider
(provider-aws). If providers were broken up by API group this reference would
cross provider boundaries; from provider-aws-eks to provider-aws-ec2.

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

### Compute Resource Impact

I expect 'native' providers (i.e. providers that aren't generated with `upjet`)
to use less CPU and memory under this proposal. This is because while adding
extra pods means a little extra overhead, most folks will have a lot fewer
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
overall. Today provider-aws, at idle, must start almost 900 controllers. One for
each supported kind of managed resource. Each controller spins up a few
goroutines and subscribes to API server updates for the type of MR it
reconciles. By breaking up providers significantly fewer controllers would be
running, using resources at idle.

We expect significantly more compute resources to be consumed by Terraform
provider processes because there will need to be more of them. It’s safe to
assume each deployment of provider-aws becomes ~7 smaller deployments on
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

## Alternatives Considered

I considered the following alternatives before making this proposal.

### CRD Filtering

The gist of this proposal is to update the existing Provider.pkg.crossplane.io
API to include a configurable allow list of enabled CRDs. As far as I can tell
this is the only feasible alternative to breaking up the largest Providers.

If we took this approach I would propose that:

* We teach the Package Manager that only some Providers support filtering. Such
  Providers would have something like `pkg.crossplane.io/filtering: enabled` in
  their `crossplane.yaml`. This is necessary to avoid filtering being silently
  ineffectual when not supported by a Provider.
* We add an optional filter list to the Provider.pkg type. This would be
  replicated to the ProviderRevision.pkg type (that actually installs/owns a
  Provider’s CRDs).
* If you installed a filtering-enabled Provider into an empty control plane and
  didn’t specify what types to enable, no types would be enabled.
* If you installed a filtering-enabled Provider into a control plane the Package
  Manager would automatically detect what types were in use and enable them
  automatically.
* If you installed a filtering-enabled Provider as a dependency of a
  Configuration the Package Manager would scan its Compositions for types and
  enable them automatically.

This allows us to have an “opt-in” to the types you want approach, without
uninstalling the types you’re already using when you upgrade from a Provider
that doesn’t support filtering to one that does. This is preferable to an
"opt-out" approach, which I believe to be untenable. The default UX for a large
provider cannot continue to be installing all of its CRDs: this would result in
a bad user experience (i.e. broken clusters, slow tools) for anyone who did not
know they needed to filter.

Pros
* Maximum granularity - you could filter down to only most specific set of CRDs
  you need.
* No more pods, and thus no more provider compute resources, than what is needed
  today.

Cons
* A new concept for everyone to learn about - filtering.
* Varying user experiences: filtered providers install no types by default,
  unfiltered providers install all types by default.
* Longer release cycle. Would not be enabled by default for several months.

There would be quite a lot of work required to Crossplane core, and some to
providers to implement this feature. The core work would be subject to the
typical alpha, beta, GA release cycle. Assuming we could design and implement
this in time for Crossplane v1.13 (due in July) this means we wouldn’t be able
to recommend folks run the feature in production until at least Crossplane v1.14
(due in October).

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
the API server directly. This may be feasible for API servers we control (e.g.
MCPs), but is probably not feasible in the wild.

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
means that even if the ~900 provider-aws CRDs aren’t actually loaded, kubectl
will try to make ~900 API requests for the types it sees in the discovery API
when someone runs kubectl get managed.

There would be quite a lot of work required to both Crossplane core and
providers to implement this feature. The core work would be subject to the
typical alpha, beta, GA release cycle. Assuming we could design and implement
this in time for Crossplane v1.12 (due in April) this means we wouldn’t be able
to recommend folks run the feature in production until at least Crossplane v1.13
(due in July).

[upjet]: https://github.com/upbound/upjet
[issue-1770]: https://github.com/crossplane/crossplane/issues/1770
[issue-3754]: https://github.com/crossplane/crossplane/issues/3754
[kep-aggregated-discovery]: https://github.com/kubernetes/enhancements/issues/3352