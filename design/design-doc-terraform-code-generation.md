# Code Generation Against Terraform
* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Managed resources are granular, high fidelity Crossplane representations of a
resource in an external system - i.e. resources that are managed by Crossplane.
A provider extends Crossplane by installing controllers for new kinds of managed
resources. Providers typically group conceptually related managed resources;
for example the AWS provider installs support for AWS managed resources like
RDSInstance and S3Bucket.

Crossplane community has been developing providers in Go to add support for the
services they need. However, the development process can be cumbersome for some
and sometimes just the sheer number of managed resources needed can be way too
many for one to develop one by one. In order to address this issue, we have come
up with several solutions like [crossplane-tools][crossplane-tools] for simple
generic code generation and integrating to code generator pipeline of AWS Controllers
for Kubernetes project to generate fully-fledged resources, albeit still need
manual additions to make sure corner cases are handled.

The main challenge with the code generation efforts is that the corresponding APIs
are usually not as consistent, hence need resource specific logic to be implemented
manually. However, we are not alone in this space. Terraform community has been
building declarative API for many infrastructure services handling those corner
cases. As Crossplane community, we'd like to build on top of the shoulders of the
giants and leverage that support that's been developed for years.

## Goals

We would like to develop a code generation pipeline that can be used for generating
a Crossplane provider for any Terraform provider. We'd like the code generator
pipeline to be able to:

* Offload resource-specific logic to Terraform CLI and/or Terraform Provider as
  much as possible so that we can get hands-off generation,
* Generate managed resources thst satisfy Crossplane Resource Model requirements,
* Target any Terraform provider available.

While it'd be nice to generate a whole provider from scratch, that's not quite
what we aim for. The main goal of the code generation effort is to generate
things that scale with the number of managed resources. For example, it should be
fine to manually write `ProviderConfig` CRD and its logic. 

## Options

There are different ways that we can choose to leverage Terraform providers:
* Import provider code
* Talk directly with provider server via gRPC
* Talk with Terraform CLI using HCL/JSON interface

In order to make a decision, let's inspect what makes up a managed resource support
and see how they cover those requirements.

The Go code that needs to be written to add support for a managed resource can be
classified as following:
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
| Target SDK Struct -> Spec Late Initialization | Generated `cty` Translation | Generated `cty` Translation | JSON Marshalling (multi-key) and reflection |
| API Output -> Status | Generated `cty` Translation | Generated `cty` Translation | JSON Marshalling (multi-key) |
| Spec <-> API Output Comparison for up-to-date check | Generated `cty` Translation (leaking) | Generated `cty` Translation (leaking) | `terraform plan` diff |
| Readiness Check | `create/update` completion | `create/update` completion | `terraform apply` completion |
| Calls to API | call `create/update/delete/read` funcs | call `create/update/delete/read` funcs | `terraform apply/destroy` |
| Connection Details | iterate over `create` result | iterate over `create` result | `sensitive_attributes` in tfstate |

> "leaking" in this context means that there are resource-specific details we can't
> generate.

> jsoniter library allows usage of tag keys other than "json", which would allow
> us define multiple keys for the same field to cover snake case Terraform struct
> fields and camel case CRD struct fields.

### Import Provider

The main advantage of this approach is that we're closer to the actual API calls
that are made to the external API, which would possibly make things run faster
since we eliminate HCL middleware.

On the other hand, we'd have to do some of the operations Terraform CLI does, like
diff'ing, importing, conversion of JSON/HCL to `cty` and such. Providers do give
you the customization functions whenever needed but the main pipeline of execution
lives in Terraform CLI. 

Additionally, the mechanics of providers naturally include core Terraform code
structs that we'd need to develop logic to work on and that'd increase
our bug surface since they are not intended to be called by something other than
Terraform CLI code.

### Talk to Provider

When Terraform starts executing an action, it spawns the provider gRPC server and
talks with it. Similarly, Crossplane provider process could bring up a server
and different reconciliation routines could talk with it via gRPC.

This option is not too different than importing provider since we'd still work
with `cty` wire format and implement things that Terraform CLI provides. In fact,
the earlier effort for building code generation built on top of Terraform used
this approach. It had a ton of code generator logic for handling `cty` and had to
have its own logic for merge/diff operations.

### Interact with Terraform CLI

We can produce a blob of JSON and give that to Terraform CLI for processing whenever
we need to execute an operation. [As mentioned][json-hcl-compatibility] in
Terraform docs, JSON can be a full replacement for HCL and thanks to [`jsoniter`][jsoniter]
library, we'll be able to have multiple tag keys to use in marshall/unmarshall
operations depending on the context. So, we will not need to generate any conversion
functions!

This option has the advantage that we'd be interacting with Terraform just like
any user, hence all features, stability guarantees and community support would
be available to us. For example, we won't need to build logic for importing resources
or comparison tooling to see whether an update is due; we'll call the CLI and parse
its JSON output.

A disadvantage of this approach is speed because the information format flow would
be `Go Struct -> JSON -> cty -> provider call`, whereas other options don't include
HCL in their flow. Additionally, we'd need to work with the filesystem to manage
HCL files and `.tfstate` file that contains the state. The footprint of multiple
instances of Terraform CLI working at the same time could also bring in resource
problems, however, we can fork the CLI to allow calling the CLI functions directly
in our code to avoid that.

## Proposal

From the three options listed, interacting with Terraform CLI via JSON will get us
faster to the finish line and will open up less bug surface. The main drivesr of
this decision is the simplicity of the workflow, i.e. there will be very few
places where we'd have to interact with Terraform Go structs and reimplement what's
already there. And that means we can deliver it much faster than other options.

Note that schema generation is not affected by our decision of at what layer we
will interact with Terraform at runtime; the resulting CRDs will be same with maybe
quite small differences.

### Schema Generation

Every Terraform provider exposes a [`*schema.Schema`][schema-schema] object per
resource type that has all information related to a field, such as whether it's
immutable, computed, required etc. In our pipeline, we will iterate through
each [`*schema.Schema`][schema-schema] recursively and generate spec and status
structs and fields, see Terraform AWS Provider for an example of that struct.
Then we will use standard Go `types` library to encode the information and then
print it using standard Go templating tooling. These structs will need to be
convertible to and from JSON with different keys depending on where they are used.
An example output will look like the following:

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
The source of truth will always be the one from Terraform Provider schema.

### Translation Functions

We will introduce additional tag key called `tf` in order to store the field name
used in Terraform schema so that conversions don't require strong-typed functions
to be generated. Namely, the following mechanisms will be used for each:

* Spec -> API Input
  * `jsoniter.Marshal` (using `tf` key)
* Target SDK Struct -> Spec Late Initialization
  * `jsoniter.Marshal` (using `tf` key) on empty `Parameters` object
  * Recursive iteration of fields of two `Parameters` object using `reflect` library to late-initialize. 
* API Output -> Status
  * `jsoniter.Unmarshal` (using `tf` key) using output of `terraform show --json`
* Spec <-> API Output Comparison for up-to-date check
  * Running `terraform plan --json` to see if an update is needed
  

### Controller Code

We will have a single implementation of [`ExternalClient`][external-client] that
will be used across all providers built on top of Terraform. However, since
connection details differ between provider APIs, there will be single implementation
of [`ExternalConnecter`][external-connecter] struct for every provider implemented
manually, which will instantiate a generic [`ExternalClient`][external-client]
object per CRD.

The Terraform controller will have two main parts:
* Scheduler
  * It will manage all the low-level interaction with Terraform CLI and provide
    instant, non-blocking functions to reconciler.
* Reconciler
  * It will be the struct that implements [`ExternalClient`][external-client]

#### Scheduler

Scheduler will be responsible for managing all interactions with Terraform CLI. 
We'd like it to hide the fact that Terraform operations are blocking, so it will
act like a server that the reconciler hits whenever it needs to do an operation
and get a response immediately so that reconciliations do not get stuck waiting
for the operation to complete.

A rough sketch of the scheduler interface could look like the following:

```go
type Scheduler interface {
  // Equivalent of "terraform show --json" that will be used to refresh the
  // state and fetch it.
  Show(uuid string, creds []byte, input []byte) (*ShowResult, error)
  
  // Equivalent of "terraform apply -auto-approve --json" that will be used
  // by Create and Update operations. It returns immediately; either returns
  // information about ongoing operation or starts the operation.
  Apply(uuid string, creds []byte, input []byte) (*ApplyResult, error)
  
  // Equivalent of "terraform plan --json" that will be used by Observe in order
  // to see if there is any diff that would require issuing an Update.
  Plan(uuid string, creds []byte, input []byte) (*PlanResult, error)
  
  // Equivalent of "terraform destroy --json" that will be used by Delete.
  Destroy(uuid string, creds []byte, input []byte) (*DestroyResult, error)
}

type ApplyResult struct {
  // Tells whether the apply operation is completed.
  Completed bool
  
  // Contains the current state after the operation completes.
  State []byte
}
```

As you might have noticed, the interface is rather tightly coupled with Terraform
CLI interface. While it's intriguing to have a more generic interface and have the
flexibility of changing the option we chose only at that level, that's usually
not the case and much more than what interface exposes would get changed. So, we
propose not to optimize for flexibility at this level. The main reason to have
this interface is to hide blocking operations.

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
  * [Managed resource patches][managed-resource-patches] will be used initially.
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
      the field exists, and if so, add the default tags.

## Shortcomings

There are a few risks associated with interacting with Terraform CLI. Firstly, it
might be slower than usual. The Terraform CLI brings up the provider server and
then talks to it, which takes time. Also, footprint of the multiple executions in
memory could affect resource usage. In general, managing the lifecycle of a binary
brings its own consequences.

However, most of these problems could be solved to some extent by importing
Terraform CLI and interacting with it in Go. The reason we don't do that initially
is that none of the packages are exported, i.e. they all live in `internal` package.
At some point, we can fork the Terraform CLI and expose them, if need be.

## Implementation

Conceptually, there are two main parts to be implemented as part of this design.
One is the code generator and the other one is the tools that will be used by
the generated code.

Every provider will have another executable that is its own code generator and it
will define its own pipeline in Go. As opposed to usual code generation tools that
are fairly scoped, a code generator for a fully-fledged controller has several
moving parts, which makes configuration more complex than others. Hence, having
the pipeline be constructed in Go will give more capabilities.

Code generator utilities to be used in that Go program will live in `crossplane/terrajet`
repository together with the common tools such as scheduler and reconciler implementation.

## Future Considerations

### Generating CRDs for Terraform Modules

We can possibly generate CRDs in runtime with given Terraform module schema and
reconcile it. Hence, users would be able to bring their modules and expose managed
resource API for them.

### Use Declarative Libraries of Providers

Google has DCL and Azure has ARM projects with strong-typed structs that are
used by Terraform as well. These libraries handle all the necessary customizations
and exceptions, so we can generate code to use those libraries instead of
depending on Terraform in the future. However, not all providers have declarative
libraries, AWS being one such example.

## Alternatives Considered

### Why not build on top of existing effort?

The earlier effort about building code generator for Terraform providers had made
the choice of interacting with provider servers via gRPC and unfortunately that
made things very complicated and since it didn't cover all Terraform logic, there
are places where we need to do resource-specific customization and that's against
our goal of having hands-off generation for things that scale with the number of
managed resources.

But some parts that are not directly related to gRPC communication will be
reused. For example, schema generation is common in all approaches, so we might be
able to reuse some parts and make it work with our new JSON tags.

### Why Terraform?

The main reason we don't target provider SDKs directly is the whole suite of
exceptional cases that need to be handled and customizations that should be
implemented per resource. Terraform community has done that for years with
hand-crafted code and no other tool has as much coverage as Terraform.

[jsoniter]: https://github.com/scoutapp/jsoniter-go
[json-multipletagkey]: https://github.com/scoutapp/jsoniter-go/blob/ca39e5a/config.go#L22
[schema-schema]: https://github.com/hashicorp/terraform-plugin-sdk/blob/9321fe1/helper/schema/schema.go#L37
[external-client]: https://github.com/crossplane/crossplane-runtime/blob/5193d24/pkg/reconciler/managed/reconciler.go#L187
[external-connecter]: https://github.com/crossplane/crossplane-runtime/blob/5193d24/pkg/reconciler/managed/reconciler.go#L166
[id-setid]: https://github.com/hashicorp/terraform-plugin-sdk/blob/aabfaf5/helper/schema/resource_data.go#L246
[tf-id-example]: https://github.com/hashicorp/terraform-provider-aws/blob/2692792/aws/resource_aws_rds_cluster.go#L933
[managed-resource-patches]: https://github.com/crossplane/crossplane/issues/1770