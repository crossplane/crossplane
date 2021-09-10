# Terrajet: Code Generation for Terraform-based Providers
* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Approved

## Background

Managed resources are granular, high fidelity Crossplane representations of a
resource in an external system. A provider extends Crossplane by installing
controllers for new kinds of managed resources. Providers typically group
conceptually related managed resources; for example the AWS provider installs
support for AWS managed resources like RDSInstance and S3Bucket.

Crossplane community has been developing providers in Go to add support for the
services they need since the inception of the project. However, the development
process can be cumbersome for some and sometimes just the sheer number of managed
resources needed can be way too many for one to develop one by one. In order to
address this issue, we have come up with several solutions like
[crossplane-tools][crossplane-tools] for simple generic code generation and
integrating to code generator pipeline of [AWS Controllers for Kubernetes][ack-codegen]
project to generate fully-fledged resources, albeit still need manual additions
to make sure corner cases are handled.

The main challenge with the code generation efforts is that the corresponding APIs
are usually not as consistent to have a generic pipeline that works for every API.
Hence, while we can generate the parts that are generic across the board, we still
need resource specific logic to be implemented manually, just like our [provider-aws
pipeline][ack-guide].

Fortunately, we are not alone in this space. Terraform is a powerful tool that
provisions infrastructure in a declarative fashion and the challenges they had 
regarding the provider API communication are very similar to ours. As Crossplane
community, we'd like to build on top of the shoulders of the giants and leverage
the support that's been developed over the years.

## Goals

We would like to develop a code generation pipeline that can be used for generating
a Crossplane provider from any Terraform provider. We'd like that pipeline to be
able to:

* Offload resource-specific logic to Terraform CLI and/or Terraform Provider as
  much as possible so that we can get hands-off generation,
* Generate managed resources that satisfy Crossplane Resource Model requirements,
* Be able to target any Terraform provider available.

While it'd be nice to generate a whole provider from scratch, that's not quite
what we aim for. The main goal of the code generation effort is to generate
things that scale with the number of managed resources. For example, it should be
fine to manually write `ProviderConfig` CRD and its logic, but we should do our
best to offload custom diff operations to Terraform.

## Options

There are different ways that we can choose to leverage Terraform providers:
* Import provider code
* Talk directly with provider server via gRPC
* Talk with Terraform CLI using HCL/JSON interface

In order to make a decision, let's inspect what makes up a managed resource support
and see how they cover those requirements.

The Go code that needs to be written to add support for a managed resource can be
roughly classified as following:
* CRD Structs
  * Separate Spec and Status structs
* Translation Functions
  * Spec -> API Input
  * Target SDK Struct -> Spec Late Initialization
  * API Output -> Status
  * Spec <-> API Output Comparison for up-to-date check
* Controller Code
  * Readiness Check
  * Calls to API
  * Connection Details

| Functionality | Import Provider | Talk to Provider | HCL/JSON |
| --- | --- | --- | ---|
| Separate Spec and Status structs | Iterate over schema.Provider | Iterate over schema.Provider | Iterate over schema.Provider |
| Spec -> API Input | Generated `cty` Translation | Generated `cty` Translation | JSON Marshalling (multi-key) |
| Target SDK Struct -> Spec Late Initialization | Generated `cty` Translation | Generated `cty` Translation | JSON Marshalling (multi-key) and reflection/code-generation |
| API Output -> Status | Generated `cty` Translation | Generated `cty` Translation | JSON Marshalling (multi-key) |
| Spec <-> API Output Comparison for up-to-date check | Generated `cty` Translation (leaking) | Generated `cty` Translation (leaking) | `terraform plan` diff |
| Readiness Check | `create/update` completion | `create/update` completion | `terraform apply` completion |
| Calls to API | call `create/update/delete/read` funcs | call `create/update/delete/read` funcs | `terraform apply/destroy` |
| Connection Details | iterate over `create` result | iterate over `create` result | `sensitive_attributes` in tfstate |

> Note that even if we import provider code, we need to work with `cty` format
> because the input provider functions expect is of type [`schema.ResourceData`][resource-data],
> which can be generated only if you have [`terraform.InstanceState`][instance-state]
> because all of its fields are internal. While state can be constructed, due to
> its structure of [how it stores][instance-state-map] key/values, using `cty`
> empowered with the schema of the resource would be the best choice.

> "leaking" in this context means that there are resource-specific details we can't
> generate.

> jsoniter library allows usage of tag keys other than "json", which would allow
> us define multiple keys for the same field to cover snake case Terraform struct
> fields and camel case CRD struct fields.

### Import Provider Go Code

The main advantage of this approach is that we're closer to the actual API calls
that are made to the external API, which would possibly make things run faster
since we eliminate HCL middleware.

On the other hand, we'd have to do some operations Terraform CLI does, like
diff'ing, importing, conversion of JSON/HCL to `cty` and such. Providers do give
you the customization functions whenever needed, but the main pipeline of execution
lives in Terraform CLI. So, we would have to reimplement those parts of Terraform
CLI.

Additionally, the mechanics of providers naturally include core Terraform code
structs that we'd need to develop logic to work on and that'd increase
our bug surface since they are not intended to be called by something other than
Terraform CLI code. This fact has some consequences around how the authors of the
providers organize their code as well. For example, Azure provider stores everything
under `internal` package which would force us to fork the provider.

### Talk to Provider Server

When Terraform starts executing an action, it spawns the provider gRPC server and
talks with it. Similarly, Crossplane provider process could bring up a server
and different reconciliation routines could talk with it via gRPC.

This option is not too different from importing provider since we'd still work
with `cty` wire format and implement things that Terraform CLI provides. In fact,
the earlier effort for building code generation built on top of Terraform used
this approach. It generates strong-typed functions for `cty` and have its own
logic for merge/diff operations. Though it doesn't support doing these operations
for nested schema structs.

### Interact with Terraform CLI

We can produce a blob of JSON and give that to Terraform CLI for processing whenever
we need to execute an operation. [As mentioned][json-hcl-compatibility] in
Terraform docs, JSON can be a full replacement for HCL and thanks to [`jsoniter`][jsoniter]
library, we'll be able to have multiple tag keys to use in marshall/unmarshall
operations depending on the context. So, we will not need to generate any conversion
functions to get JSON representation with snake case.

Additionally, this option has the advantage that we'd be interacting with
Terraform just like any user, hence all features, stability guarantees and
community support would be available to us. For example, we won't need to build
logic for importing resources or comparison tooling to see whether an update is
due; we'll call the CLI and parse its JSON output.

A disadvantage of this approach is that we'd need to work with the filesystem to
manage JSON files and `.tfstate` file that contains the state. The footprint of
multiple instances of Terraform CLI working at the same time could also bring in
resource problems, however, we have workarounds for these problems like pooling
the executions. Additionally, forking the CLI to make it work with an existing
provider server is an option we can consider as well.

## Proposal

From the three options listed, interacting with the CLI is the one that will get
us to the finish line fastest without too much compromise. Let's inspect the main
drivers of this decision.

#### Reimplementation of CLI

Terraform CLI has generic functionality for operations it performs and providers
only inject these operations with custom functions. For example, resources declare
[custom diff][custom-diff-example] functions so that Terraform CLI is aware of
resource-specific cases when it runs its `plan` logic. Another example would be
[custom import][custom-import-example] logic. As you can see, there is only a small
exception handling here so if we interact at this level we'd need to build `import`
logic and then inject this. So, while it's intriguing to see CRUD operations
exported right away, it's the other operations that need more effort and time.
Interacting with CLI means we get to use Hashicorp's implementation, which is
well-tested by the community, and not spend cycles.

#### Unfriendly Programming Interfaces

Since the providers are designed to be consumed only by Terraform CLI, they are
designed for its specific needs and whatever the CLI doesn't need is not exposed.
For example, [`ResourceDiff`][resource-diff] struct here has many fields and
functions that are not exposed. So the implementation we'd be doing if we went
with other two options will be very similar to how Terraform CLI implemented these
functions already.

Additionally, we see that some providers hide everything under `internal` package.
This makes it essential to fork the code for the importing option. In other options,
we still need to import the provider to get the schema to be used for generating
the CRD types, but that happens during development time, so we can have some
workarounds making the generator access the schema.

#### Optionality for the Future

As explained in detail below, we'll have an interface that includes all operations
that a usual Crossplane controller needs on top of the `ExternalClient`, like
`IsUpToDate`, `GetConnectionDetails` etc. We'll hide away all the interaction
with Terraform behind that interface, and after we get the initial XRM-compliant
coverage, we can get back to the shortcomings of this approach and assess whether
it'd be worth switching to importing provider option. If we decide to change, there
will definitely be more than underlying implementation to change but the cost
will be lowered by that interface and by that time we'd have breadth of coverage
our users need already.

Overall, the CLI is not the cleanest option, but it is the one that will get us
breadth of coverage we want in a short amount of time with good stability promise
and then the chance of iterations to see what works best for Crossplane users.

## Implementation

Conceptually, there are two main parts to be implemented as part of this design.
One is the code generator, and the other one is the common tools that will be used by
the generated code.

As opposed to usual code generation tools that are fairly scoped, a code generator
for a fully-fledged controller has several moving parts, which makes configuration
more complex than others. Hence, we need a medium that supports complex statements
that is specific to provider and/or resource so that provider developers are
able to introduce unusual configurations and corner cases. In order to achieve
that, we'll have a repository with code generation tooling and every provider
will have its own pipeline generated in `cmd/generator/main.go`. For the most
parts, it will be a struct with simple inputs like Terraform Provider URL and
such but when we need complex configuration, we'll have all capabilities of Go.

All code generator utilities to be used in that Go program will live in
`crossplane/terrajet` repository together with the common tools that will be
imported by the generated provider code.

### Schema Generation

Every Terraform provider exposes a [`*schema.Schema`][schema-schema] object per
resource type that has all information related to a field, such as whether it's
immutable, computed, required etc. In our pipeline, we will iterate through
each [`*schema.Schema`][schema-schema] recursively and generate spec and status
structs and fields, see Terraform AWS Provider for [an example][schema-example]
of that struct. Then we will use standard Go `types` library to encode the
information and then print it using standard Go templating tooling. These
structs will need to be convertible to and from JSON with different keys
depending on where they are used. An example output will look like the following:

```go
// VPCParameters define the desired state of an AWS Virtual Private Cloud.
type VPCParameters struct {
	// Region is the region you'd like your VPC to be created in.
	Region *string `json:"region" tf:"region"`

	// CIDRBlock is the IPv4 network range for the VPC, in CIDR notation. For
	// example, 10.0.0.0/16.
	// +kubebuilder:validation:Required
	// +immutable
	CIDRBlock string `json:"cidrBlock" tf:"cidr_block"`

	// A boolean flag to enable/disable DNS support in the VPC
	// +optional
	EnableDNSSupport *bool `json:"enableDnsSupport,omitempty" tf:"enable_dns_support"`

	// Indicates whether the instances launched in the VPC get DNS hostnames.
	// +optional
	EnableDNSHostNames *bool `json:"enableDnsHostNames,omitempty" tf:"enable_dns_host_names"`

	// The allowed tenancy of instances launched into the VPC.
	// +optional
	InstanceTenancy *string `json:"instanceTenancy,omitempty" tf:"instance_tenancy"`
}
```

As you might have noticed, the field tags Terraform uses are snake case whereas
the JSON tags we use should be camel case and Go field name should be upper camel.
A small utility for doing back and forth with these strings will be introduced.
The source of truth will always be the snake case string from Terraform Provider
schema.

### Translation Functions

We will introduce additional tag key called `tf` in order to store the field name
used in Terraform schema so that conversions don't require strong-typed functions
to be generated. Namely, the following mechanisms will be used for each:

* Spec -> API Input
  * `jsoniter.Marshal` (using `tf` key)
* Target SDK Struct -> Spec Late Initialization
  * `jsoniter.Marshal` (using `tf` key) on empty `Parameters` object
  * Recursive iteration of fields of two `Parameters` object using `reflect` library to late-initialize. 
    * Alternatively, we can look into generating the late initialization function
      since both structs are known.
* API Output -> Status
  * `jsoniter.Unmarshal` (using `tf` key) using output of `terraform show --json`
* Spec <-> API Output Comparison for up-to-date check
  * Running `terraform plan --json` to see if an update is needed
  

### Controller Code

We will have a single implementation of [`ExternalClient`][external-client] that
will be used across all providers built on top of Terraform. However, since
connection details differ between provider APIs, there will be single implementation
of [`ExternalConnecter`][external-connecter] struct for every provider implemented
manually, which will instantiate the generic [`ExternalClient`][external-client]
implementation per CRD.

The Terraform controller will have two main parts:
* Scheduler
  * It will manage all the low-level interaction with Terraform CLI and provide
    instant, non-blocking functions to reconciler.
* Reconciler
  * It will be the struct that implements [`ExternalClient`][external-client] and
    talks with the scheduler.

#### Scheduler

Scheduler will be responsible for managing all interactions related to Terraform
in a non-blocking fashion. While it's known that the underlying implementations
will be interacting with Terraform, we'd like to hide the fact that it's talking
with CLI so that we have optionality in the future to replace that with another
mechanism. So, the interface will have functions that represent granular Crossplane
functions instead of CLI calls.

A rough sketch of the that interface could look like the following:
```go
type Scheduler interface {
	Exists(ctx context.Context, mg resource.Managed) (bool, error)
	UpdateStatus(ctx context.Context, mg resource.Managed) error
	LateInitialize(ctx context.Context, mg resource.Managed) (bool, error)
	IsReady(ctx context.Context, mg resource.Managed) (bool, error)
	IsUpToDate(ctx context.Context, mg resource.Managed) (bool, error)
	GetConnectionDetails(ctx context.Context, mg resource.Managed) (resource.ConnectionDetails, error)
	Apply(ctx context.Context, mg resource.Managed) (*ApplyResult, error)
	Delete(ctx context.Context, mg resource.Managed) (*DeletionResult, error)
}

type ApplyResult struct {
  // Tells whether the apply operation is completed.
  Completed bool
  
  // Sensitive information that is available during creation/update.
  ConnectionDetails resource.ConnectionDetails
}
```

While the interface is generic, if we decide to change underlying implementation,
it's likely that some things above that interface would have to change as well.
But investing in this abstraction is worth it now because the cost is low given
the fact that the same implementation will live in reconciler anyway if not put
under this interface.

#### Reconciler

We call it reconciler for the lack of a better word, but in reality it's just an
implementation of the existing [`ExternalClient`][external-client], hence the actual
reconciler is the generic managed reconciler that is used by all managed resource
controllers.

The main responsibility of the reconciler is to utilize the functions of the
scheduler to implement Crossplane functionality. When we look at what a fully-fledged
[`ExternalClient`][external-client] implementation does together with functionality
in managed reconciler, the following list is what needs to be handled by the
generated provider.

* Setup Function
  * Wiring up the connector
    * Code will be generated.
* Connect
  * Fetch the credentials
    * Manually written per provider, used by all CRDs.
  * Set up the client
    * Runtime includes helpers but mostly manually written per provider.
* Observe
  * Status update
    * Unmarshal result of `Show` into status.
  * Late initialization
    * Unmarshal result of `Show` into spec and late-init with existing spec using reflection.
  * Readiness check
    * If the last `Apply` operation is completed. 
  * Check if update is needed
    * Result of `Plan` function.
  * Connection details
    * `sensitive_attributes` in `Show` result will be published.
* Create
  * Create call
    * Call `Apply`
  * Connection details
    * `sensitive_attributes` in `Show` result will be published.
* Update
  * Update call(s)
    * Call `Apply`
  * Connection details
    * `sensitive_attributes` in `Show` result will be published.
* Delete
  * Delete call
    * Call `Destroy`
* Referencer resolution
  * [Managed resource patches][managed-resource-patches] will be used initially
    if available.
  * Since the transforms can be complicated to define in YAML, a helper in Go to
    inject referencer fields will be implemented. It will be used in the generator
    code that lives in provider repository. 
* Initializers
  * External name annotation
    * `Id()` and `SetId()` [functions][id-setid] of `ResourceData` [can be used][tf-id-example]
      with the fallback of accepting a fieldpath in the configuration object of
      the code generator.
  * Default tags
    * Most Terraform schemas include `tags` field. A generic utility to check whether
      the field exists, and if so, add the default tags will cover this functionality.

#### Terraform State

Since the CRDs are generated using the schema of Terraform resource, copying the
related portions of `tfstate` to spec and status will let us store the state in
etcd. In each reconciliation, we'll construct `tfstate` from the fields of custom
resource instance.

There are two exceptions though: resource id and sensitive attributes. We'll store
the resource id as the external name of the resource, i.e. `metadata.annotations[crossplane.io/external-name]`
and the sensitive attributes will be stored in the connection detail secret of the
resource. For sensitive inputs, we'll have a mechanism to add a [`SecretKeySelector`][secret-key-selector]
for that field and use it to get the input from a `Secret` user pointed to.

#### API Representation

Crossplane tries to match the corresponding provider API as closely as possible
with a clear separation, i.e. dedicated `spec.forProvider`. While Terraform also
does that for most resources, there are occasionally fields in the Terraform schema
of the resource that configures the behavior of Terraform execution rather than
only the request made to the provider API. For example, `force_destroy` is a field
in S3 Bucket Terraform schema but there is no corresponding API call - Terraform
just deletes all objects in the bucket before calling deletion.

For such cases, we'll expose configuration points for provider authors to provide
per-resource exception information. They will be able to remove/add specific fields,
manipulate their JSON tags or code comments. Since authors will call the pipeline
in a Go context, they will be able to reuse same exception information for many
resources if it's generic enough.

#### References

> It is still to be decided whether we'll keep current cross-resource references.

In order to address an information that exist on another resource, we need the
following information in addition to the name of the target resource:
* Kind
* Field path(s)
* String manipulations
* Target field path

With our current cross-resource referencing mechanism, developers are able to
give this [information][resolve-references-example] in development time except
the name of the target resource. For example, in order to reference a VPC, a
Crossplane user only needs to give the name of the VPC like following:
```yaml
spec:
  forProvider:
    vpcIdRef:
      name: my-little-vpc
```

In order to achieve the same, simple API, we need to be able to gather the
identification information given above. Unfortunately, Terraform's referencing
mechanism is very generic in the sense that a resource implementation doesn't
know what it can reference to or whether it's referenced from another object.

> Terraform Provider authors need to define a separate [schema][data-source-example]
> that includes values that might be needed by others, which is called `data_source`
> instead of `resource`. Additionally, HCL as a language provides primitives for
> string manipulation that make it easier to write ad-hoc manipulation functions
> compared to YAML.

So this information will be something that we need human input as a customization
to the code generation pipeline. In the code generator implementation of providers,
i.e. in `cmd/generator/main.go`, developers will specify what field of a specific
resource references to what and how the transform should be handled in Go. Then
the code generator will add the necessary reference & selector fields and generate
the [`ResolveReferences`][resolve-references] function of that resource. Since
the medium is Go, developers will have the full flexibility for specifying this
information.

## Shortcomings

There are a few risks associated with interacting with Terraform CLI. Firstly,
execution of Terraform CLI requires provider binary to exist in its workspace
folder. Additionally, each execution brings up a separate provider server to talk
to. We'll investigate how to make all Terraform invocations use the same provider.
But in the worst case, we'd need to manage a pool of workspace folders and put
the `main.tf` and `terraform.tfstate` files of the managed resource when a
reconciliation request comes in.

## Future Considerations

### Generating CRDs for Terraform Modules

Conceptually, Terraform modules correspond to composition in Crossplane. However,
migrating a Terraform module to composition means a whole rewrite. We can possibly
generate a CRD in runtime with given Terraform module schema and reconcile it using
Terraform CLI. Hence, users would be able to bring their modules as is but expose
them as managed resource that they can use directly or in a composition.

### Use Declarative Libraries of Providers

Some providers have declarative versions of their SDKs that their Terraform
provider is built on top of. For example, Google has [declarative client library][dcl]
with strong-typed structs. Such libraries handle all the necessary
customizations and exceptions, so we can generate code to use those libraries
instead of depending on Terraform in the future. However, not all providers have
declarative libraries, AWS being one such example.

## Alternatives Considered

### Why not build on top of existing effort?

The earlier effort about building code generator for Terraform providers had made
the choice of interacting with provider servers via gRPC. So, the logic that existed
in Terraform CLI had to be implemented and in some cases, like `cty` conversion,
it used code generation instead of generic marshaling like Terraform CLI.
Additionally, due to the fact that that effort wasn't finished, it didn't cover
all Terraform logic, hence there are places where we need to keep implementing
functionality of CLI.

Overall, the cost of writing the pipeline from scratch targeting the CLI is lower
than to understand the existing code, spot the broken parts, adopt it and make
it work today.

### Why Terraform?

The main reason we don't target provider SDKs directly is the whole suite of
exceptional cases that need to be handled and customizations that should be
implemented per resource. Terraform community has done that for years with
hand-crafted code and no other tool has as much coverage as Terraform.

## Prior Art

There are many projects in infrastructure space that builds on top of Terraform.
Each of the projects have their own limitations, additional features and different
license restrictions.

* [Crossplane: Terraform Provider Runtime](https://github.com/crossplane/crossplane/blob/e2d7278/design/design-doc-terraform-provider-runtime.md)
* [Crossplane: provider-terraform](https://github.com/crossplane-contrib/provider-terraform)
* [Hashicorp Terraform Cloud Operator](https://github.com/hashicorp/terraform-k8s)
* [Rancher Terraform Controller](https://github.com/rancher/terraform-controller)
* [OAM Terraform Controller](https://github.com/oam-dev/terraform-controller)
* [Kubeform](https://github.com/kubeform/kubeform)
* [Terraform Operator](https://github.com/isaaguilar/terraform-operator)

[jsoniter]: https://github.com/scoutapp/jsoniter-go
[json-multipletagkey]: https://github.com/scoutapp/jsoniter-go/blob/ca39e5a/config.go#L22
[schema-schema]: https://github.com/hashicorp/terraform-plugin-sdk/blob/9321fe1/helper/schema/schema.go#L37
[external-client]: https://github.com/crossplane/crossplane-runtime/blob/5193d24/pkg/reconciler/managed/reconciler.go#L187
[external-connecter]: https://github.com/crossplane/crossplane-runtime/blob/5193d24/pkg/reconciler/managed/reconciler.go#L166
[id-setid]: https://github.com/hashicorp/terraform-plugin-sdk/blob/aabfaf5/helper/schema/resource_data.go#L246
[tf-id-example]: https://github.com/hashicorp/terraform-provider-aws/blob/2692792/aws/resource_aws_rds_cluster.go#L933
[managed-resource-patches]: https://github.com/crossplane/crossplane/issues/1770
[schema-example]: https://github.com/hashicorp/terraform-provider-aws/blob/aa9937d/aws/resource_aws_db_instance.go#L46
[custom-diff-example]: https://github.com/hashicorp/terraform-provider-aws/blob/73484854207e4459a25a9b2a0da92d5963ceed59/aws/resource_aws_vpc.go#L35
[custom-import-example]: https://github.com/hashicorp/terraform-provider-aws/blob/73484854207e4459a25a9b2a0da92d5963ceed59/aws/resource_aws_vpc.go#L801
[resource-diff]: https://github.com/hashicorp/terraform-provider-aws/blob/73484854207e4459a25a9b2a0da92d5963ceed59/awsproviderlint/vendor/github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema/resource_diff.go#L111
[data-source-example]: https://github.com/hashicorp/terraform-provider-aws/blob/aa9937d9ba4c8f6e65b39cd19def87edcb7b2ff2/aws/data_source_aws_db_instance.go#L17
[resolve-references-example]: https://github.com/crossplane/provider-aws/blob/c269977/apis/apigatewayv2/v1alpha1/referencers.go#L30
[resolve-references]: https://github.com/crossplane/crossplane-runtime/blob/f2440d9/pkg/reference/reference.go#L105
[dcl]: https://github.com/GoogleCloudPlatform/declarative-resource-client-library/blob/338dce1/services/google/compute/firewall_policy_rule.go#L321
[ack-codegen]: https://github.com/crossplane/provider-aws/blob/master/CODE_GENERATION.md
[crossplane-tools]: https://github.com/crossplane/crossplane-tools/
[ack-guide]: https://github.com/crossplane/provider-aws/blob/master/CODE_GENERATION.md
[secret-key-selector]: https://github.com/crossplane/crossplane-runtime/blob/36fc69eff96ecb5856f156fec077ed3f3c3b30b1/apis/common/v1/resource.go#L72
[instance-state]: https://github.com/hashicorp/terraform-plugin-sdk/blob/0e34772/helper/schema/resource.go#L859
[resource-data]: https://github.com/hashicorp/terraform-plugin-sdk/blob/0e34772dad547d6b69148f57d95b324af9929542/helper/schema/resource_data.go#L22
[instance-state-map]: https://github.com/hashicorp/terraform-plugin-sdk/blob/0e34772dad547d6b69148f57d95b324af9929542/terraform/state.go#L1330