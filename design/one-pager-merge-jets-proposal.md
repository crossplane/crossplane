# Merging AWS, Azure and GCP Providers

* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

[Terrajet] is a code generation framework that allows you to generate CRDs
with a generic controller that is based on crossplane-runtime generic managed
reconciler. The schema of the CRDs are derived from Terraform providers and the
generic controller calls Terraform under the hood. With Terrajet, we're able
to generate Crossplane providers that are on par with Terraform providers in
terms of coverage in the matter of days. The API of the generated resources
are fully compliant with [Crossplane Resource Model (XRM)][xrm-doc].

Before releasing the first Terrajet-based providers, we have had a discussion
whether we'd like to have them as separate providers or just Terrajet-generated
CRDs to the existing classic providers in the provider strategy doc. The decision
for initial releases of the first three Jet-based providers (AWS, Azure and GCP)
was to have them as separate providers for the reasons stated [here](https://github.com/crossplane/crossplane/blob/master/design/design-doc-provider-strategy.md#decision-for-initial-releases).

We called out that we'll revisit this decision after more usage and community
feedback, which was lacking most probably because we were asking people (in Slack
and in https://github.com/crossplane/crossplane/pull/2701) for their opinions
about providers that they have not used yet. Today we have more data and more
awareness in the community, and having more than a single implementation of
a cloud provider has been confusing for a lot of users, so a revisit of the
decision regarding AWS, Azure and GCP is due.

## Goals

We have the following goals for _the big three_ providers:

* The cloud vendors should maintain their Crossplane provider.
* The users should have the ability to provision all of their infrastructure
  through Crossplane, i.e. coverage.
* The providers should be mature enough for users to depend on them in
  production, i.e. maturity.

Today, we have two Crossplane providers for each of the big three clouds. Both
variants reach those goals at different levels but none of them are maintained
by their cloud vendor today.

## Proposal

There were three options in the provider strategy doc:
A. Completely separate providers.
B. One provider but Jet-based resources in their own API group.
C. One provider and no Jet vs classic difference. Only API version would be bumped
  if the implementation changes.

We should go with the Option C, use Terrajet in provider-aws, provider-azure
and provider-gcp as another way of generating CRDs and controllers so that
they can reach the goal of coverage really fast. Then the community can
keep investing in that single provider and make each CRD mature enough to depend on in
production. In this way, we can increase the usage to attract cloud vendors to come
and join the maintainership of these providers.

There are several reasons to choose this approach instead of having providers
separated by their implementation. Let's take a look at them one by one.

### Dependence on Terraform Provider

The generic Terrajet controller calls Terraform CLI which in turn calls the Terraform
provider to operate. Each CRD has its own completely independent reconciler thread
configured to work with the schema of that resource.

This dependence causes the Crossplane provider to be exposed to the bugs of the underlying
Terraform provider. While we strive to fix the bug there first, it's not always
feasible for us to land that change in a timely manner. In addition, there are
cases where we are locked to a version with a bug where downgrading or upgrading
is not an option because of another bug or behavior change.

With the mixed provider, we can change the underlying implementation to a hand-written
controller whenever we face such a bug. With the Kubernetes API conversion tooling,
we can manage the possible API changes as well. This would also mean that over time,
we'd have less and less Terraform dependence in our ecosystem.

### Confusion for Users and Maintainers

Terraform is a great tool but in an ideal state, we'd like the cloud vendors to step
in and maintain those providers. They may choose to keep using their TF provider but
it's unlikely; AWS does not maintain their TF provider, Azure uses ARM in their TF
provider and they've shown interest in generating Crossplane based on ARM and Google
is using Magic Modules and DCL to generate TF provider, it's likely that they will
choose to do that for their Crossplane provider as well. [Provider Strategy doc][strategy-doc] goes
into more detail about these tools. In summary, Terrajet will be a
temporary solution that we put in place to get more coverage.

With that in mind, it's a hard choice to invest in Jet-based providers knowing that
they will, even though after years, be deprecated at one point and users will have
to migrate. At the same time, classic providers are very far from having the same
coverage level. This creates confusion and reluctance to use and also contribute
to one of the providers.

With a single mixed provider, the story is much clearer; we fill the gaps with Terrajet
and over time the implementation of the resources will change to be more _native_; be
it cloud vendor generation tools or handwritten API calls. As long as the CRD schema
and the API behavior is same, they can make the best choice they'd like.

### Convergence of Efforts

The classic providers have the years of usage experience and contributions from
the community that makes them mature enough to be used in production. While Jet providers
are working on top of the mature TF providers that have been maintained longer than
Crossplane's existence, they will still need contributions from the community
to get to a certain level of maturity. If there is a single provider, then the
community wouldn't have to deal with two implementations of the same API, 
trying to cover ground on both. The bugs would be fixed once and for everyone.

Another point is that, just like users, contributors would also face a decision
they may have a hard time to make - invest in classic one which will see decreased
usage or Jet one which will be deprecated at one point, even if years away.
In fact, even though it's not attacheable directly, today we are seeing decrease
in contributions to provider-aws since the announcement of Jet providers.
If there is a single mixed provider, then everyone would know the path forward and
be assured that the next step for that provider is only forward; not deprecation.

### Kubernetes-native Referencing

Crossplane has cross-resource references just like Kubernetes; you get to specify
the name of the referenced object and the controller of the referencer takes care
of the rest, which includes the type and what field to look for to get the information.

This is possible only if there is a single type to target and if the controller
knows which field should be used as the source of information. Take a look at the following
example:

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

When we have Jet and classic providers, one does not know about the other's types. Hence,
the only way to do references is to give the type of the referenced objct and
the field path of the information on that type, something roughly like the following:

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

While this referencing style is useful for many scenarios and being discussed here,
it's not the same as how Kubernetes resources reference each other and requires more
upfront information. We could possibly make it work like native with a default and
let users optionally give different type and fieldPath, there are other concerns
with the approach listed in its discussion.

In a single provider, users would keep using Kubernetes-native style referencing where
they are only concerned with the name and/or labels of the referenced object, since
there is a single type and it's defined in the same provider, hence the controller
knows these details already.

## Drawbacks

### Code Difference

Terrajet has a single controller implementation that you configure to work with different
CRDs but the folder structure is the same as classic providers, i.e. `apis` and `pkg/controller`
folders and the rest is exactly the same. The most apparent difference is that in classic
providers, you have the controller `Setup` function [together with the implementation][classic-setup]
of the controller whereas in Jet providers, there is only `Setup` [function][jet-setup] since its
implementation lives in `terrajet` repository. In either case, the controller implementation
satisfies the same [`ExternalClient`][https://github.com/crossplane/crossplane-runtime/blob/295de47/pkg/reconciler/managed/reconciler.go#L268] interface from crossplane-runtime.

The biggest difference in that regard is that the CRD structs in Jet providers need to
satisfy an additional interface called [`Terraformed`][https://github.com/crossplane/terrajet/blob/4b28784/pkg/resource/interfaces.go#L55]
in order for the generic controller to work. So, even though the structure is same, you'll
see that the generic controller makes extensive use of the functions defined in that interface
where as classic implementations work with the type directly.

### Code Debugging Experience Difference

This is probably the biggest difference between classic and Jet providers. If you run the
debugger and put breakpoints in the code, you'll realize that Jet providers run the Terraform
CLI in a temporary TF workspace folder instead of making HTTP calls to the cloud vendor. This
makes it more challenging to debug Jet providers because you need to have a rough idea of
how Terraform works, i.e. concepts like TF state and TF configuration.

One thing to keep in mind is that it's not really a common case for Crossplane users to locally
debug the code to see what's happening - usually the error in events or the status of the
resource tells you what went wrong. This is still the case for Jet providers, in fact, most
of the errors you see still look similar to classic providers since Terraform, just like Crossplane,
doesn't make much change to the errors it gets from the cloud vendor API.

[Terrajet]: https://github.com/crossplane/terrajet
[xrm-doc]: one-pager-managed-resource-api-design.md
[strategy-doc]: design-doc-provider-strategy.md
[classic-setup]: https://github.com/crossplane/provider-aws/blob/475fd24/pkg/controller/acm/controller.go#L62
[jet-setup]: https://github.com/crossplane-contrib/provider-jet-aws/blob/bbe5a3b/internal/controller/ec2/ebsvolume/zz_controller.go#L41