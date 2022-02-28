# Merging AWS, Azure and GCP Providers

* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

[Terrajet] is a code generation framework that allows you to generate CRDs with
a generic controller that is based on crossplane-runtime generic managed
reconciler. The schema of the CRDs are derived from Terraform providers and the
generic controller calls Terraform under the hood. With Terrajet, we're able to
generate Crossplane providers that are on par with Terraform providers in terms
of coverage in the matter of days. The API of the generated resources are fully
compliant with [Crossplane Resource Model (XRM)][xrm-doc]. You can read more
about its internals in [the deep dive blog post series][deep-dive-blogs].

Before releasing the first Terrajet-based providers, we have had a discussion
whether we'd like to have them as separate providers or have the
Terrajet-generated CRDs added to the existing classic providers in the [provider
strategy doc][strategy-doc]. The decision for initial releases of the first
three Jet-based providers ([AWS][jet-aws], [Azure][jet-azure] and
[GCP][jet-gcp]) was to have them as separate providers for the reasons stated
[here](https://github.com/crossplane/crossplane/blob/master/design/design-doc-provider-strategy.md#decision-for-initial-releases).

We called out that we'll revisit this decision after more usage and community
feedback, which was lacking most probably because we were asking people (in
Slack and in https://github.com/crossplane/crossplane/pull/2701) for their
opinions about providers that they have not used yet. Today we have more data
and more awareness in the community, and having two different implementations
has been confusing for a lot of users, so a revisit of the decision regarding
AWS, Azure and GCP is due.

## Goals

We have the following goals for _the big three_ providers:

* The cloud vendors should maintain their Crossplane provider.
* The users should have the ability to provision all of their infrastructure
  through Crossplane, i.e. coverage.
* The providers should be mature enough for users to depend on them in
  production, i.e. maturity.

Today, we have two Crossplane providers for each of the big three clouds.
* AWS
  * Classic with [124 CRDs][provider-aws-crds] with 37 beta CRDs.
  * Jet with [763 CRDs][jet-aws-crds], no beta CRDs.
* Azure
  * Classic with [21 CRDs][provider-azure-crds] with 7 beta CRDs.
  * Jet with [647 CRDs][jet-azure-crds], no beta CRDs.
* Google Cloud
  * Classic with [27 CRDs][provider-gcp-crds] with 11 beta CRDs.
  * Jet with [438 CRDs][jet-gcp-crds], no beta CRDs.

Both variants of each reach those goals at different levels but none of them are
maintained by their cloud vendor. The aim of this document is to capture a
decision about the future of these provider implementations with the goals
stated above in mind.

## Proposal

There were three options in the provider strategy doc:
* Option A: Completely separate providers.
* Option B: One provider but Jet-based resources in their own API group.
* Option C: One provider and no Jet vs classic difference. Only API version
  would be bumped if the implementation changes.

We should go with the Option C, i.e. use Terrajet in provider-aws,
provider-azure and provider-gcp as another way of generating CRDs and
controllers so that they can reach the goal of coverage really fast. Then the
community can keep investing in that single provider and make each CRD mature
enough to depend on in production. In this way, we can increase the usage by
increasing both the coverage and the maturity of the CRDs in a consolidated way.
This increase in usage can attract cloud vendors to come and join the
maintainership of these providers. Once they do, they can either progressively
convert the implementation of the controllers to their choice of technology or
start with new providers which we can suggest people to use and we can provide
migration tooling from the community-maintained one. In either case, the users
would see a clear path forward and we'd not waste any efforts by the community.

Note that we are making two decisions here:
* There will be a single provider.
* The single provider will be the classic one and we'll add Terrajet CRDs on
  top.

There are several reasons to choose this approach instead of having providers
separated by whether they call APIs directly or use Terrajet. Let's take a look
at them one by one.

### Dependence on Terraform Provider

The generic Terrajet controller calls Terraform CLI which in turn calls its
Terraform provider to operate. Each CRD has its own completely independent
reconciler thread configured to work with the schema of that resource.

This dependence causes the Crossplane provider to be exposed to the bugs of the
underlying Terraform provider. While we strive to fix the bug there first, it's
not always feasible for us to land that change in a timely manner. In addition,
there are cases where we are locked to a version with a bug where downgrading or
upgrading is not an option because of another bug or behavior change.

With the mixed provider, we can change the underlying implementation to a
hand-written controller whenever we face such a bug. With the Kubernetes API
conversion tooling, we can manage the possible API changes as well. This would
also mean that over time, we'd have less and less Terraform dependence in our
ecosystem.

### Confusion for Users and Maintainers

Terraform is a great tool but in an ideal state, we'd like the cloud vendors to
step in and maintain those providers. They may choose to keep using their TF
provider but it's unlikely; AWS does not maintain their TF provider, Azure uses
ARM in their TF provider and they've shown interest in generating Crossplane
based on ARM and Google is using Magic Modules and DCL to generate TF provider,
so it's likely that they will choose to do that for their Crossplane provider as
well. [Provider Strategy doc][strategy-doc] goes into more detail about these
tools. In summary, Terrajet will be a temporary solution that we put in place to
get more coverage and at some point we'll need to discuss how to change the
implementation to those tools.

With that in mind, it's a hard choice to invest in separate Jet-based providers
knowing that they will, even though after years, be deprecated at one point and
users will have to migrate. At the same time, classic providers are very far
from having the same coverage level. This creates confusion and reluctance to
use either of the providers, which also results in decreased pace of
contributions because of the divergence of efforts.

With a single mixed provider, the story is much clearer; we fill the gaps with
Terrajet and over time the implementation of the resources will change to be
more _native_; be it cloud vendors using their own approaches or handwritten API
calls community writes that may be necessary to overcome a bug in TF provider.
As long as the CRD schema and the API behavior is same, the maintainers can make
the best choice depending on the situation, communicate any breaking changes and
provide a smooth experience to the users. In case cloud vendor steps in and
decides to start its own provider, then the story is still clear; there is a new
provider maintained by its vendor and you should migrate to that one once you
see fit as opposed to Jet vs classic choice where you need to think about
different trade-offs with different and uncertain timelines.

### Ownership by The Cloud Vendors

As stated above, we hope that cloud vendors will come in and maintain their
Crossplane providers. It seems like Terraform has achieved this goal only for
GCP; Azure and AWS Terraform providers are not maintained by the vendors. In our
case, Crossplane is a CNCF project with an open governance model, so we can't
really compare it with Terraform, a property of HashiCorp, in that respect but
it still provides a data point.

The strategy we choose should take the following possibilities into account:
* They may choose to bootstrap their own Crossplane provider, maybe even not
under Crossplane organization.
* They might decide to convert the underlying implementation to their own
technology, similar to GCP Terraform provider that is [undergoing a
process][gcp-tf-contributing] where they change implementation from handwritten
API calls to Magic Modules and DCL.
* They would provide the tools but not take part in maintenance of the codebase.

In the first case, regardless of whether we go with separate or mixed providers,
we will likely suggest users to migrate to the vendor-maintained provider so
that they can get commercial grade support for the bugs in the provider. One
could argue that if users are on a single provider once that happens, it's
probably easier to handle the migration. In addition, it's likely the main
reason they'd step in would be community-maintained provider having a large user base,
so it's not very likely that they will start a new provider and handle migration
from a provider with such a large user base.

In the second case, mixed provider wouldn't require any manual migration. The
conversion webhooks that Kubernetes API provides will take care of the API
changes as described in [provider strategy doc][strategy-doc] in detail. In the
case of separate providers though, users of one of the provider would have to
migrate.

In the third case, if we go with single mixed provider, we can choose to use the
cloud vendor tooling for the resources we decide to convert. If we go with
separate providers, we could keep working with those tools, like AWS CC, in the
existing classic provider or bootstrap a new one but either case would either be
similar or worse than the single mixed provider in terms of choices users would
have to consider. Overall, the decision of how to go from Terrajet to
cloud-vendor tooling is an independent decision from how we manage Terrajet and
classic provider mix since it's the step after we decide that, if ever, the
provided tooling is better than we have. For example, today, calling AWS API
directly is definitely not better than Terrajet.

In all cases, our best bet on getting them to maintain their provider is to
increase usage so that their customers demand it and the single mixed provider
makes people more comfortable by reducing the decision overhead and providing
confidence to users about what they can expect to happen to the tool they build
their infrastructure on top of.

### Convergence of Efforts

The classic providers have the years of usage experience and contributions from
the community that makes them mature enough to be used in production. While Jet
providers are working on top of the mature TF providers that have been
maintained longer than Crossplane's existence in some cases, they will still
need contributions from the community to get to a certain level of maturity. If
there is a single provider, then the community wouldn't have to deal with two
implementations of the same API, trying to cover ground on both. The bugs would
be fixed once and for everyone.

Another point is that, just like users, contributors would also face a decision
they may have a hard time to make - invest in classic one which will see
decreased usage or Jet one which will be deprecated at one point, even if years
away. If there is a single mixed provider, then everyone would know the path
forward and be assured that the next step for that provider is only additive
towards the end goals; not deprecation or removal of their contributions in
favor of a temporary approach.

### Cross-resource References

Crossplane has cross-resource references just like Kubernetes; you get to
specify the name of the referenced object or labels to select it and the
controller of the referencer takes care of the rest, which includes the type and
what field to look for to get the information.

This is possible only if there is a single type to target and if the controller
knows which field should be used as the source of information. Take a look at
the following example:

```yaml
# User can give this information manually but if vpcIdRef is used, controller knows
# which field of VPC object to copy here.
vpcId: vpc-2314334
# User can give only the name of the VPC object and then vpcId will be populated.
vpcIdRef:
  name: my-vpc
# User can give only a selector and then the vpcIdRef will be populated.
vpcIdSelector:
  matchLabels:
    app: ola
```

When we have Jet and classic providers, one does not know about the other's
types. Hence, the only way to do references is to give the type of the
referenced objct and the field path of the information on that type, something
roughly like the following:

```yaml
vpcId: vpc-2314334
vpcIdRef:
  apiVersion: ec2.aws.crossplane.io/v1beta1
  kind: VPC
  fieldPath: metadata.annotations[crossplane.io/external-name]
vpcIdSelector:
  apiVersion: ec2.aws.crossplane.io/v1beta1
  kind: VPC
  fieldPath: metadata.annotations[crossplane.io/external-name]
  matchLabels:
    app: ola
```

While this referencing style is useful for many scenarios and being discussed
[here][generic-references], it's not the same as how Kubernetes resources
reference each other and requires more upfront information. We could possibly
make it work like native with a default and let users optionally give different
`kind`, `apiVersion` and `fieldPath`, there are other concerns with the approach
listed in its discussion.

In a single provider, users would keep using Kubernetes-native style referencing
where they are only concerned with the name and/or labels of the referenced
object, since there is a single type and it's defined in the same provider, the
controller knows these details already.

## Drawbacks

### Code Difference

Terrajet has a single controller implementation that you configure to work with
different CRDs but the folder structure is the same as classic providers, i.e.
`apis` and `pkg/controller` folders and the rest is exactly the same. The most
apparent difference is that in classic providers, you have the controller
`Setup` function [together with the implementation][classic-setup] of the
controller whereas in Jet providers, there is only `Setup` [function][jet-setup]
since its implementation lives in `terrajet` repository. In either case, the
controller implementation satisfies the same
[`ExternalClient`][externalclient-interface] interface from crossplane-runtime.

The biggest difference in that regard is that the CRD structs in Jet providers
need to satisfy an additional interface called
[`Terraformed`][terraformed-interface] in order for the generic controller to
work. So, even though the structure is same, you'll see that [the generic
controller][generic-controller] makes extensive use of the functions defined in
that interface where as classic implementations work with the type directly.

### Maintenance Challenges

This is probably the biggest difference between classic and Jet providers. If
you run the debugger and put breakpoints in the code, you'll realize that Jet
providers run the Terraform CLI in a temporary TF workspace folder instead of
making HTTP calls to the cloud vendor. This makes it more challenging to debug
Jet providers because you need to have a rough idea of how Terraform works, i.e.
concepts like TF state and TF configuration. In either case though, if you use a
Jet-generated CRD in Jet provider or mixed provider, you'll need to bear, this
cost. Mixing the provider doesn't really make that harder or easier but what to
expect when you start debugging could be different, i.e. not every CRD is
debugged in the same way.

One thing to keep in mind is that it's not really a common case for Crossplane
users to locally debug the code to see what's happening - usually the error in
events or the status of the resource tells you what went wrong. This is still
the case for Jet providers, in fact, most of the errors you see still look
similar to classic providers since Terraform, just like Crossplane, doesn't make
much change to the errors it gets from the cloud vendor API.

## What Others Did

The concept of providers are common with projects like Terraform and Pulumi.
These projects have had similar challenges as well regarding the technology
choices that they made to build their providers.

In Terraform, it seems that the maintainers (not affiliated with AWS) has
decided to bootstrap a new provider when they decided to move to AWS Cloud
Control whereas GCP Provider (maintained by GCP) had [decided to
mix][gcp-tf-contributing] the manually written TF code with their Magic Modules
and DCL. Azure seems to have a single provider that is built with ARM. The
common theme of all these TF providers is that the users have a linear choice to
make; it's either a single provider or they know which provider is going to be
maintained in the foreseeable future. In our case, we have separate Jet and
classic providers, but it's hard to convince people that Terrajet is a temporary
solution but also Jet providers will be maintained long enough to outlive the
project they are currently building, hence the confusion.

For Pulumi, the story is a bit different. They had started with providers
generated from TF equivalents, a similar approach to Terrajet. But then they
[bootstrapped][pulumi-blog] new providers that use cloud vendor tooling
directly, without Terraform. At all points, their users had a clear idea of
what's next and the choice is usually clear enough. In our case, we started with
cloud vendor tooling and then introduced the temporary solution and that make
people hesitant to choose either.

Even though there are similarities, our trajectory of starting with cloud vendor
tooling and introducing a provider with more coverage using a middle-layer, i.e.
Terrajet, is unique and makes a big difference in how people should think about
the choice they make. So, we have one of the three in Terraform going with mixed
approach and none in Pulumi, but the trajectories don't really map enough to
point to either of them as similar cases.

[Terrajet]: https://github.com/crossplane/terrajet
[deep-dive-blogs]: https://blog.crossplane.io/deep-dive-terrajet-part-i/
[xrm-doc]: one-pager-managed-resource-api-design.md
[strategy-doc]: design-doc-provider-strategy.md
[jet-aws]: https://github.com/crossplane-contrib/provider-jet-aws
[jet-gcp]: https://github.com/crossplane-contrib/provider-jet-gcp/
[jet-azure]: https://github.com/crossplane-contrib/provider-jet-azure/
[gcp-tf-contributing]:
    https://github.com/hashicorp/terraform-provider-google/blob/610f62b/.github/CONTRIBUTING.md#generated-resources
[classic-setup]:
    https://github.com/crossplane/provider-aws/blob/475fd24/pkg/controller/acm/controller.go#L62
[jet-setup]:
    https://github.com/crossplane-contrib/provider-jet-aws/blob/bbe5a3b/internal/controller/ec2/ebsvolume/zz_controller.go#L41
[generic-references]: https://github.com/crossplane/crossplane/pull/2385
[terraformed-interface]:
    https://github.com/crossplane/terrajet/blob/4b28784/pkg/resource/interfaces.go#L55
[externalclient-interface]:
    https://github.com/crossplane/crossplane-runtime/blob/295de47/pkg/reconciler/managed/reconciler.go#L268
[generic-controller]:
    https://github.com/crossplane/terrajet/blob/ca43613/pkg/controller/external.go#L108
[provider-aws-crds]:
    https://doc.crds.dev/github.com/crossplane/provider-aws@v0.24.1
[jet-aws-crds]:
    https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-aws@v0.4.0-preview
[provider-azure-crds]:
    https://doc.crds.dev/github.com/crossplane/provider-azure@v0.18.1
[jet-azure-crds]:
    https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-azure@v0.7.1-preview
[provider-gcp-crds]:
    https://doc.crds.dev/github.com/crossplane/provider-gcp@v0.20.0
[jet-gcp-crds]:
    https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-gcp@v0.2.0-preview
[pulumi-blog]: https://www.pulumi.com/blog/announcing-aws-native/