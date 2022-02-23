# Merging AWS, Azure and GCP Providers

* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

[Terrajet] is a code generation framework that allows you to generate CRDs that
are compatible with its own implementation of generic controller that is based
on crossplane-runtime generic managed reconciler. The schema of the CRDs are
derived from Terraform providers and the generic controller calls Terraform
under the hood. With Terrajet, we're able to generate Crossplane providers that
are on par with Terraform providers in terms of coverage in the matter of days.

Before releasing the first Terrajet-based providers, we have had a discussion
whether we'd like to have them as separate providers or just Terrajet-generated
CRDs to the existing classic providers in the provider strategy doc. The decision
for initial releases of the first three Jet-based providers (AWS, Azure and GCP)
was to have them as separate providers for the reasons stated [here](https://github.com/crossplane/crossplane/blob/master/design/design-doc-provider-strategy.md#decision-for-initial-releases).

After almost all announcement activities of Terrajet, one of the first questions
we get is which provider should be used now. We recommend that they should look at
maturity and coverage of the CRDs they need and make their own choice;
essentially treating them as two competing implementations.

We called out that we'll revisit this decision after more usage and community
feedback, which was lacking most probably because we were asking people (in Slack
and in https://github.com/crossplane/crossplane/pull/2701) for their opinions
about providers that they have not used yet. Today we have more data and more
awareness in the community, and also having more than a single implementation of
a cloud provider has been confusing for a lot of users, so a revisit is due.


## Proposal

The proposal is that we use Terrajet in provider-aws, provider-azure
and provider-gcp as another way of generating CRDs and controllers. Hence,
we'd have a single provider for each of those cloud providers, giving people
a clear guidance on which provider to use and contribute to.

The decision would cover only those provider since either choice is available
for provider builders but it would also set a precedence for others.


### Confusion of Choice and Reluctance to Adoption


### Convergence of Efforts

### Kubernetes-classic Referencing

### Ability to Convert to Handwritten Code

### Terraform Provider Dependency Lock

### Forced Reliance on Terraform Provider Implementations

## Drawbacks

### Code is Different

### Debugging Experience is Different if Error is not from Cloud

[Terrajet]: https://github.com/crossplane/terrajet
[xrm-doc]: one-pager-managed-resource-api-design.md
