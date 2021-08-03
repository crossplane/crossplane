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
* Generate managed resources taht satisfy Crossplane Resource Model requirements,
* Target any Terraform provider available.

While it'd be nice to generate a whole provider from scratch, that's not our main
goal. For example, it should be fine to manually write `ProviderConfig` CRD and
its logic. The key point is to automate things that have to be implemented for
every resource differently.

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

| Functionality | Import Provider | Talk to Provider | HCL |
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
diff operations. Additionally, the mechanics of providers naturally include core
Terraform code structs that we'd need to develop logic to work on and that'd increase
our bug surface since they are not intended to be called by something other than
Terraform CLI code.

### Talk to Provider

When Terraform starts executing an action, it spawns the provider gRPC server and
talks with it. Similarly, Crossplane provider process could bring up a server
and different reconciliation routines could talk with it via gRPC.

This option is not too different than importing provider since we'd still work
with `cty` wire format and implement things that Terraform CLI provides. In fact,
the earlier effort for building code generation built on top of Terraform used
this approach.

### Interact with Terraform CLI

We can produce a blob of JSON and give that to Terraform CLI for processing whenever
we need to execute an operation. [As mentioned][json-hcl-compatibility] in
Terraform docs, JSON can be a full replacement for HCL and thanks to [`jsoniter`][jsoniter]
library, we'll be able to have multiple tag keys to use in marshall/unmarshall
operations depending on the context.


This option has the advantage that we'd be interacting
with Terraform just like any user, hence all features, stability guarantees and
community support would be available to us. Additionally, doing conversion from
and to JSON wouldn't require generated code as HCL library provides powerful tools
to do that.

A disadvantage of this approach is speed because the information format flow would
be `Go Struct -> JSON -> cty -> provider call`, whereas other options don't include
HCL in their flow. Additionally, we'd need to work with the filesystem to manage
HCL files and `.tfstate` file that contains the state. The footprint of multiple
instances of Terraform CLI working at the same time could also bring in resource
problems, however, we can fork the CLI to allow calling the CLI functions directly
in our code to avoid that.

## Proposal

From the three options listed, interacting with Terraform CLI via HCL will get us
faster to the finish line and will open up less bug surface. The main driver of
this decision is the simplicity of the workflow, i.e. there will be very few
places where we'd have to interact with Terraform Go structs and reimplement what's
already there.

Note that schema generation is not affected by our decision of at what layer we
will interact with Terraform at runtime; the resulting CRDs will be same with maybe
quite small differences.

### Schema Generation

Every Terraform provider exposes a [`*schema.Schema`][schema-schema] object per
resource type that has all information related to a field, such as whether it's
immutable, computed, required etc. In our pipeline, we will iterate through
each [`*schema.Schema`][schema-schema] and generate spec and status structs and
fields, see Terraform AWS Provider for an example of that struct. Then ee will use
standard Go `types` library to encode the information and then print it using
standard Go templating tooling. These structs will need to be convertible to and
from JSON with different keys depending on where they are used. An example output
will look like the following:

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
  * It will manage all the low-level interaction with Terraform CLI.
* Reconciler
  * It will be the struct that implements [`ExternalClient`][external-client]

#### Scheduler

Scheduler will be responsible for managing all interactions with Terraform CLI.
The separation of concerns between the scheduler, and the reconciler will be handled
by an interface so that if we decide

We'd like it to hide the fact that Terraform operations are
blocking, so it will act like a server that the reconciler hits whenever it needs
to do an operation and get a response immediately so that reconciliations do not
get stuck waiting for the operation to complete.

A rough sketch of the scheduler interface could look like the following:

```go
type Scheduler interface {
	// Equivalent of "terraform show --json" that will be used to refresh the
	// state and fetch it.
    Show(uuid string, input []byte) (*ShowResult, error)
    
    // Equivalent of "terraform apply -auto-approve --json" that will be used
    // by Create and Update operations. It returns immediately; either returns
    // information about ongoing operation or starts the operation.
	Apply(uuid string, input []byte) (*ApplyResult, error)
    
    // Equivalent of "terraform plan --json" that will be used by Observe in order
    // to see if there is any diff that would require issuing an Update.
    Plan(uuid string, input []byte) (*PlanResult, error)
    
    // Equivalent of "terraform destroy --json" that will be used by Delete.
    Destroy(uuid string, input []byte) (*DestroyResult, error)
}
```

As you might have noticed, the interface is rather tightly coupled with Terraform
CLI interface. While it's intriguing to have a more generic interface and have the
flexibility of changing the option we chose only at that level, that's usually
not the case and much more than what interface exposes would get changed. So, we
propose not to optimize for flexibility at this level. From provider maintainers'
perspective, since the controller code won't be generated but imported commonly,
change cost will be low already.

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

## Implementation

Conceptually, there are two main parts to be implemented as part of this design.
One is the code generator and the other one is the tools that will be used by
the generated code.

### Code Generator






[jsoniter]: https://github.com/scoutapp/jsoniter-go
[json-multipletagkey]: https://github.com/scoutapp/jsoniter-go/blob/ca39e5a/config.go#L22
[schema-schema]: https://github.com/hashicorp/terraform-plugin-sdk/blob/9321fe1/helper/schema/schema.go#L37
[external-client]: https://github.com/crossplane/crossplane-runtime/blob/5193d24/pkg/reconciler/managed/reconciler.go#L187
[external-connecter]: https://github.com/crossplane/crossplane-runtime/blob/5193d24/pkg/reconciler/managed/reconciler.go#L166
[id-setid]: https://github.com/hashicorp/terraform-plugin-sdk/blob/aabfaf5/helper/schema/resource_data.go#L246
[tf-id-example]: https://github.com/hashicorp/terraform-provider-aws/blob/2692792/aws/resource_aws_rds_cluster.go#L933
[managed-resource-patches]: https://github.com/crossplane/crossplane/issues/1770