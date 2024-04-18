# Provider Strategy in Crossplane

* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

Crossplane providers have been developed since the inception of the project
using the published APIs of the cloud providers, git servers and any software
that exposes an API. However, the coverage of the providers hasn't been at a
level that we'd like it to be. In a lot of cases, users opt for adding support
for the resources they need, depending on how automated the process is. But
there are many users that don't have the means to do that contribution and they
decide to check if their needs are met at a later date.

In order to lower the cost of adding a new resource and reach a critical mass
that will increase the pace of the contributions to keep up with users, we have
decided to invest in a project called
[Terrajet](https://github.com/crossplane-contrib/terrajet) that will let
provider developers generate the CRDs and use a common runtime that wraps
Terraform CLI for its operations. This way we're able to add support for a
resource in a matter of minutes.

With great power comes responsibility. Now that we are able to generate Terrajet
based providers, what to do with the existing providers is a question that we
need to answer sooner than later.

This document will try to summarize the current summary of tools that we can
imagine used in the providers in the long term and then proposes a strategy that
will inform our next steps with Terrajet providers as well as these new tools
and where they sit in the big picture of Crossplane ecosystem.

It's important to note that we have not made any decisions about using the
tools here, we're analysing them as possible candidates to see what it'd take
to use them as part of a native code generation pipeline and migrate to that.

## Summary of Tools

In this section, we will examine each of the candidate tools that cloud
providers maintain in addition to their Terraform providers. Each of the tooling
section includes a list of metadata requirements that we have for full
automation of the code generation. Note that the requirements are not blockers
for building code generator; it only reflects the effort needed to build it and
the custom code that may be needed per-resource. The more requirements are met,
the easier code generation implementation will be.

### AWS Cloud Control API

> Note that we already have native code generation pipeline built with Amazon
> Controllers for Kubernetes [(ACK)](https://github.com/crossplane/provider-aws/blob/master/CODE_GENERATION.md).

AWS Cloud Control API is a new managed service
[announced](https://aws.amazon.com/blogs/aws/announcing-aws-cloud-control-api/)
by AWS. In its essence, it's what powers
[`CloudFormation`](https://aws.amazon.com/cloudformation/), which is a
declarative API of AWS for managing cloud resources. It has many similarities to
how users interact with `CloudFormation` but it's not as heavyweight. The main
difference between the two is that `CloudFormation` allows you to manage
multiple resources in a single object called `Stack` whereas Cloud Control API
supports a single resource and the tracking is done via request tokens.

You can give it a try by using the script [here](assets/design-doc-provider-strategy/cloudcontrol/aws.sh).

Here is some high level notes about how Cloud Control behaves:

* Support in CloudFormation registry does not mean supported in Cloud Control,
  [authoritative
  list](https://docs.aws.amazon.com/cloudcontrolapi/latest/userguide/supported-resources.html).
  * 376 resources out of 627 entries in CloudFormation Registry are supported.
* DBInstance, EKS Cluster and other resources that has sensitive inputs/outputs
  are not supported yet.
  * Unclear how the API will handle one-time outputs such as secret key of IAM
    User or Kubeconfig of EKS Cluster.
* No metadata about sensitive fields.
* Metadata about create-only and read-only fields, others can be updated.
* Completely async, tracking done by request token IDs.
* Impersonation by giving another IAM principal to use as the acting entity of
  underlying API calls.


Metadata requirements:

* [x] Identifier field.
  * [x] Consistency between taking it as input and returning as output, via
    `primaryIdentifier`.
* [ ] Sensitive field list, input/output.
* [ ] References to other resources.
* [x] Required fields.
* [x] Spec and Status separation, via `readOnlyProperties`
* [x] Immutability, via `createOnlyProperties`.
* [x] List of API calls being done.
* [x] Tracking of unique identifier of Create call, via request tokens.
* [ ] Enum validation in schema.


#### Schema

The schema of the resources used in Cloud Control is exactly same as
CloudFormation.

The following is the list of validated example YAML of the same resource in
different kinds of providers:

```yaml
# Native provider-aws
apiVersion: ecr.aws.crossplane.io/v1alpha1
kind: Repository
metadata:
  name: example
  labels:
    region: us-east-1
spec:
  forProvider:
    region: us-east-1
    imageScanningConfiguration:
      scanOnPush: true
    imageTagMutability: IMMUTABLE
    tags:
     - key: key1
       value: value1

# Terrajet provider-jet-aws
apiVersion: ecr.aws.jet.crossplane.io/v1alpha1
kind: Repository
metadata:
  name: sample-repository
spec:
  forProvider:
    region: us-east-1
    imageScanningConfiguration:
      - scanOnPush: true
    imageTagMutability: IMMUTABLE
    tags:
      key1: value1

# Cloud Control API input
ImageScanningConfiguration:
  ScanOnPush: true
ImageTagMutability: IMMUTABLE
RepositoryName: sample-repository # field is marked as identifier in the schema.

# Possible CloudControl Implementation using provided schema
apiVersion: ecr.aws.crossplane.io/v1alpha1
kind: Repository
metadata:
  name: sample-repository
spec:
  forProvider:
    region: us-east-1
    imageScanningConfiguration:
      - scanOnPush: true
    imageTagMutability: IMMUTABLE
    tags:
     - key: key1
       value: value1
```

As you might have noticed, the schemas are very similar except the `tags` field
because Terraform maintainers apparently made a deliberate decision there to
improve UX while Cloud Control stuck with how AWS SDK represents them similar to
our native provider.

A more complex resource would be an RDS Instance. The example YAML isn't
validated with Cloud Control API since there is no support for that resource yet
but it works with CloudFormation. See the comparison snippet
[here](assets/design-doc-provider-strategy/cloudcontrol/comparison-dbinstance.yaml). Two things to note there is
that we have `autogeneratePassword` field that we implemented in native provider
to improve UX and there is `skipFinalSnapshot` parameter that is not included in
Cloud Control schema since it's a parameter for deletion. Other than these two,
native and Cloud Control are very similar and Terraform one has different names
for a subset of the fields.

### Google DCL

Google DCL is a declarative Go library that declarative infra tools such as
Terraform, Ansible and Pulumi uses instead of using the low level SDK. While
Cloud Control and Azure Resource Manager is also available to call using
provider SDKs, this library can be considered as a separate SDK with all the
functionality and [strong-typed
structs](https://github.com/GoogleCloudPlatform/declarative-resource-client-library/blob/4c08aed/services/google/compute/network.go#L27)
it provides instead of JSON blob as the medium of configuration as opposed to
the other two.

By looking at the
[codebase](https://github.com/GoogleCloudPlatform/declarative-resource-client-library),
it's clear that the first user of this library was Terraform since a lot of
config options and the way the functions are supposed to called are optimized
for how Terraform works. One notable example is that all calls are synchronous
and blocking just like Terraform.

You can give it a try by using the Go program [here](assets/design-doc-provider-strategy/dcl/dcl.go).

Here is some high level notes about how Google DCL works:

* Terraform GCP Provider [is being
  built](https://github.com/hashicorp/terraform-provider-google/blob/fcea0cb/go.mod#L4)
  on this SDK.
  * There are exceptions to this. Some notable examples:
    * [Cloud
      SQL](https://github.com/hashicorp/terraform-provider-google/blob/c96c998d0f6d134e1959f6314e319c9fd1258d39/google/resource_sql_database_instance.go#L18)
      group.
    * Most `compute` APIs are on DCL,
      [Instance](https://github.com/hashicorp/terraform-provider-google/blob/c96c998d0f6d134e1959f6314e319c9fd1258d39/google/resource_compute_instance.go#L21)
      is not.
    * `container` APIs [are
      not](https://github.com/hashicorp/terraform-provider-google/blob/c96c998d0f6d134e1959f6314e319c9fd1258d39/google/resource_container_cluster.go#L18)
      on DCL at all, [no Go
      files](https://github.com/GoogleCloudPlatform/declarative-resource-client-library/tree/main/services/google/container)
      in DCL for that group.
* It seems like progress is being made to move all to DCL but it's an ongoing
  process. In many cases, like
  [`Instance`](https://github.com/GoogleCloudPlatform/declarative-resource-client-library/blob/4c08aed/services/google/compute/instance.yaml)
  there is the schema file but no Go files.
* DCL schema is used in Terraform but there are some fields added to improve the
  UX, like `delete_default_routes_on_create` in `Network`.

Metadata requirements:

* [x] Identifier field, via `ID()` function similar to TF.
  * [ ] Consistency between taking it as input and returning as output.
* [ ] Sensitive field list, input/output.
* [x] References to other resources, via `x-dcl-references`.
* [x] Required fields, via `required`
* [x] Spec and Status separation, via `readOnly`.
* [x] Immutability, via `immutable`.
* [ ] List of API calls being done.
* [x] Tracking of unique identifier of Create call, tracking is done with
  user-defined name in most cases.
* [x] Enum validation in schema, `enum`.

#### Schema

The schema Terraform Google Provider uses is exactly the same schema as DCL,
with some minor exceptions. The following is a list of example YAMLs in
different contexts:

```yaml
# Crossplane Native GCP Provider
apiVersion: compute.gcp.crossplane.io/v1beta1
kind: Network
metadata:
  name: example
spec:
  forProvider:
    autoCreateSubnetworks: false
    routingConfig:
      routingMode: REGIONAL

# Possible provider-jet-gcp, written by looking at TF schema
apiVersion: compute.gcp.crossplane.io/v1beta1
kind: Network
metadata:
  name: example
spec:
  forProvider:
    autoCreateSubnetworks: false
    routingConfig:
      routingMode: REGIONAL
    deleteDefaultRoutesOnCreate: false # This field is added by Terraform maintainers.

# Possible Crossplane Provider built with DCL
apiVersion: compute.gcp.crossplane.io/v1beta1
kind: Network
metadata:
  name: example
spec:
  forProvider:
    autoCreateSubnetworks: false
    routingConfig:
      routingMode: REGIONAL
```

For a more complex resource such as `GKE Cluster`, see [this YAML
file](assets/design-doc-provider-strategy/dcl/comparison-cluster.yaml).

### Azure Resource Manager

[Azure Resource
Manager](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/overview)
is the name of the control plane used for all Azure resource provisioning and
management tasks. You can use service-specific API endpoints to manage resources
and the Azure SDK has many packages with strong-typed structs that target those
APIs. It's closer to GCP API where all operations are resource-based as opposed
to AWS API where most of the endpoints are verb-based.

[Azure Resource Manager
Template](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/overview#terminology)
is a special endpoint that allows management of multiple resources in a dynamic
way; similar to AWS CloudFormation. ARM Template API is what powers [Azure
Service Operation (ASO)
v2](https://github.com/Azure/azure-service-operator/tree/cc47e45/v2).

A few high level notes about how it works:

* The API is closer to CloudFormation than Cloud Control.
  * It accepts an
    [array](https://docs.microsoft.com/en-us/azure/azure-resource-manager/templates/template-tutorial-add-resource?tabs=azure-powershell#add-resource)
    of resource JSON blobs.
* Schema is published in [Open API
  format](https://github.com/Azure/azure-rest-api-specs/blob/350d1c0a395d133a9e02cf00afdaf8bdb336ce56/specification/network/resource-manager/Microsoft.Network/stable/2021-03-01/network.json)
  instead of a custom format.
* Azure Service Operator (ASO) is [getting
  built](https://github.com/Azure/azure-service-operator/blob/c33aeb3/v2/internal/controller/armclient/raw_client.go#L116)
  on top of ARM.
* It is already resource-based and both SDK and ARM use the same schema.
  * This is different than CloudFormation and AWS SDK where CloudFormation can
    consolidate many verb calls into one resource.


Metadata requirements:
* [x] Identifier field.
  * [x] Consistency between taking it as input and returning as output, similar
    to GCP but need more investigation.
* [x] Sensitive input, via `x-ms-secret`.
  * [ ] Output fields such as `kubeconfig` are not marked as secret
* [ ] References to other resources.
* [x] Required fields, via `required`
* [x] Spec and Status separation, via `readOnly`
* [ ] Immutability.
* [ ] List of API calls being done.
* [x] Tracking of unique identifier of Create call, via constructed resource
  path.
* [x] Enum validation in schema.

#### Schema

Since all tooling uses the same spec as source of truth for their code
generation, the schemas are same with minor differences.

The following examples are for Postgre SQL Server.

```yaml
# Crossplane Native Provider
apiVersion: database.azure.crossplane.io/v1beta1
kind: PostgreSQLServer
metadata:
  name: example-psql
spec:
  forProvider:
    administratorLogin: myadmin
    resourceGroupName: example-rg
    location: West US 2
    minimalTlsVersion: TLS12
    sslEnforcement: Disabled
    version: "11"
    sku:
      tier: GeneralPurpose
      capacity: 2
      family: Gen5
    storageProfile:
      storageMB: 20480

# Terrajet provider-jet-azure
apiVersion: postgresql.azure.jet.crossplane.io/v1alpha1
kind: PostgresqlServer
metadata:
  name: example
spec:
  forProvider:
    name: example-psqlserver
    resourceGroupName: example
    location: "East US"
    administratorLogin: "psqladminun"
    skuName: "GP_Gen5_4" # different than native where 3 fields are used.
    version: "11"
    storageMb: 640000 # different than native where this is given under storageProfile
    publicNetworkAccessEnabled: true # schema same, but not supported in our native provider yet.
    sslEnforcementEnabled: true # in native, enum Disabled/Enabled instead of boolean
    sslMinimalTlsVersionEnforced: "TLS1_2" # named differently as minimalTlsVersion

# Possible ARM implementation example,
# written by looking at https://docs.microsoft.com/en-us/azure/templates/microsoft.dbforpostgresql/servers?tabs=json
# Exactly same as native implementation.
apiVersion: database.azure.crossplane.io/v1alpha1
kind: PostgreSQLServer
metadata:
  name: example-psql
spec:
  forProvider:
    administratorLogin: myadmin
    resourceGroupName: example-rg
    location: West US 2
    minimalTlsVersion: TLS12
    sslEnforcement: Disabled
    version: "11"
    sku:
      tier: GeneralPurpose
      capacity: 2
      family: Gen5
    storageProfile:
      storageMB: 20480
```

### Terraform Providers

Terraform providers of the big three clouds differ in their heterogeneity of the
APIs used:
* AWS TF uses solely AWS SDK and they created a new provider to work with Cloud
  Control API.
  * CC supports [376
    resources](https://docs.aws.amazon.com/cloudcontrolapi/latest/userguide/supported-resources.html)
    (does not include main resources like EKS Cluster, RDS or Buckets) whereas
    AWS TF has ~760.
* Google TF uses DCL wherever possible.
  * Some main resources are still not supported in DCL like GKE, CloudSQL,
    Compute Instance etc.
* Azure TF uses the Azure endpoints directly.
  * I couldn't find a resource that uses the generic ARM Template endpoint, they
    all used SDK.
  * However, ARM Template endpoint does have full coverage as opposed to DCL and
    Cloud Control. Though it's questionable whether comparing it with CC instead
    of CloudFormation.

In terms of coverage, the best bet is still Terraform Providers except for Azure
where ARM Template has full coverage. At the same time, AWS CloudFormation has
the same coverage level, too, if we were to include it. So, while it depends on
how you look at it Terraform providers have all of them unified and filled the
gaps in the schemas whenever needed.

In terms of the shape of the APIs we build, you can see from the examples that
the closer to the lower level API, the similar field names and structures we
get. But it seems like this is mostly due to the different versioning practices
followed. For example, we see that Azure TF has different names and formats for
some of the fields since it didn't change the schema when they updated the SDK
they used, instead implemented manual pairings to not break users because TF
provides a single version HCL schema to users whereas Azure versions their
schemas by date. Overall, the differences are mostly due to TF not wanting to
break users with the new schemas published by the clouds.

Another aspect of the API shape discussion is the additional properties that TF
providers decide to add. For example, GCP `Network` has
`delete_default_routes_on_create` property in TF provider that doesn't exist in
GCP API. Even though it's very rare, Crossplane native provider has similar
additions as well. For example, in order to not have a racing condition we
removed the nodepool section from GKE cluster to have people create `NodePool`s
separately and that made us remove the default node pool at every creation,
which is something users toggle in TF via `remove_default_node_pool` property.

Metadata requirements:
* [x] Identifier field.
  * [ ] Consistency between taking it as input and returning as output.
* [x] Sensitive input/output, via `Sensitive`.
* [ ] References to other resources.
* [x] Required fields, via `Required`
* [x] Spec and Status separation, via `Computed`
* [ ] Immutability.
* [ ] List of API calls being done.
* [x] Tracking of unique identifier of Create call, CLI handle.
* [ ] Enum validation in schema.

### Resource Definitions

One of the most important aspects of the schema discussions is about what is
considered as `resource` or `property` by each tool. Because while we can always
handle the property name, value and structural differences with Kubernetes API
versioning tools, it's more challenging to handle the case where a single
resource is defined as two or more separate resources in the new version or vice
versa.

The most notable examples of this is `Bucket` resource in AWS. From SDK
perspective, `Bucket` has very limited set of properties and you configure other
aspects such as `CORSRule` with different calls. However, both
[CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-bucket.html)
and [AWS
TF](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/s3_bucket#argument-reference)
provider make deliberate decision about including those calls in their `Bucket`
definition and there is no clear metadata in the SDK that make this decision for
you.

From resource definition perspective, each cloud deserves its own summary:
* Google TF uses DCL whenever possible already and the GCP API is already
  resource-based.
  * There isn't much discrepancy here.
* AWS TF seems to have generally been following CloudFormation which is powered
  by Cloud Control.
  * There are exceptions though, so one needs to check Cloud Control Registry.
* Azure TF uses Azure SDK mostly and Azure API is already resource-based.
  * ARM Template endpoint uses the same schema as the low level API, so it
    follows the same decisions as well.

### Provider Strategy of Terraform

Currently, there is a single stable and recommended TF provider for each of the
big three clouds:
* AWS TF (only native): https://github.com/hashicorp/terraform-provider-aws
* AWS TF CC (only CC): https://github.com/hashicorp/terraform-provider-awscc
* Azure TF (only ARM, no ARM Template):
  https://github.com/hashicorp/terraform-provider-azurerm
* Google (mix of DCL and native):
  https://github.com/hashicorp/terraform-provider-google
  * Separate Google Provider for Beta APIs, similar to us:
    https://github.com/hashicorp/terraform-provider-google-beta

With the analysis we have done, it seems that creation of a new provider for
Cloud Control was more of an exception rather than a strategy that Terraform
follows when it comes what tooling the provider is using. For example, Google
provider uses DCL and SDK at the same time.

## Options

### A: Provider per SDK/API

We can have the following set of providers to cover three big clouds:
* AWS
  * Terrajet AWS: https://github.com/crossplane-contrib/provider-jet-aws
  * Cloud Control AWS: https://github.com/crossplane-contrib/provider-awscc
  * Native AWS: https://github.com/crossplane/provider-aws
* GCP
  * Terrajet GCP: https://github.com/crossplane-contrib/provider-jet-gcp
  * DCL GCP: https://github.com/crossplane-contrib/provider-gcpdcl
  * Native GCP: https://github.com/crossplane/provider-gcp
* Azure
  * Terrajet Azure: https://github.com/crossplane-contrib/provider-jet-azure
  * ARM Templates: https://github.com/crossplane-contrib/provider-azurerm
  * Native Azure: https://github.com/crossplane/provider-azure

The main advantages of this approach are:
* For users
  * All errors of resources in a single provider are in the same format.
  * Expectation about the duration of operations are at the same level with all
    resources.
  * Single way of debugging the problem when they dive into the code.
* For maintainers
  * Single way of debugging
  * Separation on repository level would allow cloud providers to come in for
    maintaining the provider more easily.

The main disadvantages:
* For users
  * No cross-resource references between providers.
    * You have to use generic patching in composition for value transport.
  * Might have to install N providers to cover all their CRD requirements.
  * Confusing to choose which provider would work best for them.
  * Reluctance to use any non-native ones because of the worries about us
    supporting others in the long term.
  * No automated API-level migration is possible from one CRD to another if they
    decide to use another provider.
    * Either through scripts or manual actions.
* For maintainers
  * More repositories to maintain, including issue triaging, reviewing PRs and
    providing support.
  * Have to think about migration scenarios and might have to provide tooling
    for migration outside of Kubernetes API.

### Single Provider with Multiple APIs

We cannot use different tools for the same CRD at the same `apiVersion`, which
includes group and version. So, we need a separation level on either group or
version level. The most important thing to look for is the migration path for
implementation changes. For example, there can be a bug in Azure TF provider
that we can't solve and we might want to switch to native implementation. The
choice we make here should lower the cost of this switch as much as possible
with the best user experience.

#### B: Separation on Group Level

We can separate the CRDs by the tool their managed reconciler uses on group
level like:
* `s3.aws.jet.crossplane.io/v1alpha1` for Terrajet S3
* `s3.aws.cc.crossplane.io/v1alpha1` for Cloud Control S3
* `s3.aws.crossplane.io/v1alpha1` for native S3

Since the separation is on group level, it may seem like we can have a different
`Bucket` CRD in each group, that's not really feasible today since the
cross-resource referencing works only with single target type, i.e.
`spec.s3Import.bucketNameRef` can only target a single type because if it
targets multiple types then there could be more than a single candidate for
resolution with the same name.

The main advantages:
* For users
  * Single provider that they can trust since it will be the one maintained
    long-term.
  * Full coverage with a single provider.
  * Cross-resource references between all CRDs of a single cloud.
* For maintainers
  * Single repository to maintain for a single cloud provider.

The main disadvantages:
* For users
  * If implementation of a resource changes, they have to do manual migration.
    * No API-level automatic migration since they are completely separate CRDs.
  * 
* For maintainers
  * For every implementation change, have to provide manual
    instructions/scripts.
    * No two CRDs can exist at the same, so, the instructions won't be as simple
      as field key/value changes.
  * Cloud provider teams may not be OK with maintaining Terrajet-based code when
    they want to take over the ownership of their Crossplane provider.


#### C: Separation on Version Level

We can separate the CRDs by the tool their managed reconciler uses on version
level like:
* `s3.aws.crossplane.io/v1alpha1` for the initial introduction of the S3
  resource.
* `s3.aws.crossplane.io/v1alpha2` when we decide to switch to another tool,
  either Cloud Control or native SDK
* `s3.aws.crossplane.io/v1beta1` for when we feel it's mature enough to support
  on beta level.

The main advantages:
* For users
  * Single provider that they can trust since it will be the one maintained
    long-term.
  * Full coverage with a single provider.
  * Cross-resource references between all CRDs of a single cloud.
  * Completely automated migration from one implementation to another without
    having to worry what to use.
* For maintainers
  * If there is a bug in Terraform provider, we can migrate that specific resource
    to native implementation with minimal cost instead of having to fix Terraform
    provider or getting locked into a state where updating its version isn't
    possible.
  * Single repository to maintain for a single cloud provider.
  * Use Kubernetes API versioning utilities to handle migration, such as
    conversion webhooks.
    * This won't work if [resource definitions](#resource-definitions) are
      different, so each resource introduction needs a bit more work to make
      sure no obscure definition is used.

The main disadvantages:
* For users
  * They have to look at the label/annotation of CRD to know the underlying
    implementation.
  * Debugging experience might be different in cases where the error comes from
    the tool instead of provider API.
* For maintainers
  * Debugging will require knowledge of the tooling used for that CRD.
  * Cloud provider teams may not be OK with maintaining Terrajet-based code when
    they want to take over the ownership of their Crossplane provider.

## Proposal

For each of the big three cloud providers, we have the long-term plan of using
their tools to the extent possible in the long term, e.g. Cloud Control for AWS,
DCL for GCP and possibly ARM Templates for Azure. However, their Terraform
providers are our best short to medium bet due to the vast coverage and maturity
it provides. That was one of our main motivations behind Terrajet.

Given that we know the underlying tool will change at some point in the future,
we need to think about how to manage that change with the least amount of burden
on users to do the migration and maintainers to provide the tools for that and
maintain the whole process. In addition, the migration process is important to
be light-weight for the cases where we'd like to switch to another
implementation because of a bug in Terraform provider.

In that regard, Option C where we separate on version level sounds like the best
trade-off due to the following reasons:
* Migration of custom resources will be automated at the Kubernetes API server
  level.
  * Maintainers will need to provide Go functions schema changes but the base
    tooling for conversion webhooks is available and standard.
  * Users won't need to take any manual action until the transition period of
    given version ends and still get the latest tooling available, just like
    standard Kubernetes resources.
* Maintaining a separate repository for each tooling does not scale in even
  short-term due to uptick in Crossplane users.
* Users won't have to think about what tool the controller of the CRD uses, it
  becomes just an implementation detail rather than a choice you need to make at
  provider level.
* In practice, roughly 70% of the CRDs are small resources, hence less surface
  for having bugs, meaning I expect them to stay on the same implementation even
  in the long term.

The course of action would look roughly like the following:
1. Generate all missing CRDs in native providers using Terrajet.
  * This will get us to full coverage in matter of a few weeks.
2. Develop the conversion webhook interfaces in crossplane-runtime.
  * We already need this for all kinds of version updates from `v1beta1` and
    upwards.
3. Once we decide to build on cloud provider tools, or see a bug in TF provider
   that cannot be solved in the short term we can switch to those resources.
   We'll need to add migration function to convert from one schema to the other.
  * In addition, we can decide to have main resources like Kubernetes clusters,
    databases, VM instances to be native but all other smaller ones TF-based.

With this strategy, we'll be able to have full coverage in a very short time and
continue improving those resources in a single provider.

### Drawbacks and Risks

The main risk of this approach is that what happens if we end up in a state that
we just cannot have automated migration to the new CRD. In such cases, we can
provide the manual instructions/scripts that we'd have to provide for every
migration in other two options. For example, if the resource definition changes
with the tooling change or even change in the source API, the migration effort
will be similar to other two options.

Another thing to keep in mind as drawback is that debugging will be harder for
users since error messages could look different if they are returned before
making it to the API, e.g. Terraform errors. Additionally, when they dive into
the code, they will see different logic depending on the tool used in the
controller. It's not as bad as looking at Terraform code vs native SDK since
whatever tool we use, we build it on top of the same managed reconciler that
powers all Crossplane managed resource controllers. However, it's still
something to keep in mind.

## Decision for Initial Releases

We have decided on taking the Option A where Terrajet-based providers reside in
their own repositories, completely separate from native providers.

The main drivers of this decision are:
* Possible reluctance from cloud providers to own a provider repository with
  non-native API calls.
  * We cannot really remove but only replace a CRD with native implementation if
    that's a blocker for them.
* Concerns around the migration path from Terrajet to native implementation.
  * While we do not see immediate concern in regard to Kubernetes API-level
    versioning and the webhooks for doing the schema migration, we do not have
    data about the Terrajet-specific problems that may arise, hence it feels
    like too much magic to folks.
  * Separate providers allow having two implementations of the same external
    service in different providers which won't make it a necessity to migrate.
* It's much easier to merge two providers later if the migration path is that
  clean but the cost of a separation is way higher.
* We will commit to maintain Terrajet-based for providers for a long time, on
  the scale of years.

The costs that we will need to account for by this decision are roughly:
* Commit to maintain Terrajet-based providers for years along with native ones.
  * This is the highest cost by far.
* Confusion for users about which provider to use.
* Reluctance to adopt Terrajet-based providers since there is a **native**
  counterpart.
  * We'll be able to tell them that Terrajet-based providers will be maintained
    for a long time but not everyone asks.
* If there is a bug in Terraform provider, we can't switch to native implementation
  of that resource, at least not commit to develop tools to do this, which makes
  us more vulnerable to Terraform Provider bugs that we can't control.
* Less than ideal user experience if we enable usage of two flavors at the same
  time.
  * It's not really feasible to have Kubernetes-style strong-typed cross
    resource references. We'd have to implement a generic way of information
    transportation between the managed resources, which has its own quirks and
    concerns.
* Migration from one to the other is completely manual and involves some YAML
  surgery.
* The separation will signal that the Terraform is not just an implementation
  detail, but in reality, it actually is.
  * The CRDs and controllers we generate are completely XRM-compliant and work
    just like other resources from the end user perspective.
