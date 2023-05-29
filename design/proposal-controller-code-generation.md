# Code Generation for Managed Resource Controllers
* Owner: Kasey Kirkham (@kasey)
* Reviewers: Crossplane Maintainers
* Status: Speculative, revision 0.2

# Background
Crossplane’s adoption is contingent on broad support for the Resources of the
major cloud Providers, but the toil of implementing a Managed Resource
controller is high and presents a barrier to new contributions. Developers of
new managed services generally begin by finding an exemplary Resource for the
cloud provider of their choice to duplicate and edit by hand. There are many
ways for new developers to get lost on the way to writing their first
controller. They can be overwhelmed by low-level implementation details, or
high-level concepts outside the scope of Managed Resources. Making the right
design decisions when translating a Provider Resource to Go types requires a
nuanced understanding of Provider API specifics and Crossplane patterns. The
goal of this document is to frame out a strategy for using code generation to
remove some of this friction in order to expand Crossplane.

## Code Generation Targets
A recent PR can help illustrate what is involved in implementing a Managed
Resource. These are all possible targets for code generation, with varying
degrees of complexity.

* **Resource Types**: `apis/<group>/<version>/<kind>_types.go`: Go struct
  representation of the Resource CRD, with kubebuilder annotations. This is the
  base getter/setter object which represents the Provider Managed Resource in
  the Kubernetes API.
* **Resource Type Registration**: `apis/<provider>.go` &
  `apis/<group>/<version>/register.go`: registration of Resource Types for
  controller runtime API interaction.
* **Documentation**: `apis/<group>/<version>/doc.go`: Package documentation,
  which should carry some details from the Provider about the Resources.
* **Examples**: `examples/<group>/<kind>`: valid example resource.
* **Controller Registration**: `pkg/controller/<provider>.go`:
  controller-runtime registration.
* **Controller Boilerplate**: `pkg/controller/.../managed.go`: controller
  methods, called by the `Reconciler`.
  * **`SetupX()`**: initialization function for the controller. Some parts are
    boilerplate, like `NewControllerManagedBy(mgr)`, setting up logger/recorder,
    and constructing the `Connector`.
  * **`connecter.Connect()`**: a second initialization phase, setting up any
    runtime types that require access to values from k8s api, like the Provider
    credentials.
  * **`New<API>` methods** are common in controllers, these handle constructing
    a service object using the Provider credentials stored in the
    `provider.yaml` object.
  * **`Observe()/Create()/Update()/Delete()`**: These methods would be very
    challenging to generate given the variation amongst Provider APIs -- for
    instance the major cloud Providers vary in how resources are constructed,
    related to each other, and what structural conventions are followed amongst
    various resources as the APIs evolve (eg a newer AWS API like AKS follows
    different conventions than a first generation API like S3). These methods do
    however have some boilerplate, like type checking and error handling.
  * **`isUpToDate()`**: Many controllers implement a helper method to compare
    the observed Provider state (`Observe` method) with the current state in the
    Kubernetes API. We may be able to infer which fields are mutable from
    Provider metadata.
  * **`populate/generate` methods**: merging Provider and Kubernetes
    representations of the Resource together. If we can generate `isUpToDate()`
    we can likely also generate this.
  * **Resource naming**: generating the `crossplane.io/external-name` annotation
    for a given resource (and whether we can anticipate or construct it in the
    `Observe()` step). The general problem here is differentiating between “This
    Resource does not exist in the Provider” vs “This resource was created out
    of band, but I haven’t observed it yet”.
  * **Test Boilerplate**: `pkg/controller/.../managed_test.go`: Some boilerplate
    test methods can probably be generated, like type validation, and tests for
    the Connect() method. Creating a standard structure for table-driven tests
    could be a helpful starting point. We can also try to set up guard rails for
    generated code that will be modified by developers inc. `isUpToDate` and
    `Connect()`.
* Currently generated via `make generate` (`angryjet` and `controller-tools`):
  * **`apis/<group>/<version>/zz_generated.deepcopy.go`**: deep copy method set
    for the above Resource types, from `controller-tools`.
  * **`apis/<group>/<version>/zz_generated.managed.go`**: `angryjet` generated
    getter/setter methodset, to satisfy the `resource.Managed`
    [interface][resource.Managed interface].
  * **`config/crd/<crd-name>.yaml`**: the CRD itself, which will be applied to
    the cluster where Crossplane runs.

# Proposal
Because Provider APIs are filled with inconsistencies, and because we can
observe the amount of hand-engineered code present in competing products, we
know that building Managed Resource Controllers in an automated fashion will
likely be a long journey. But looking at the Code Generation Targets above, we
can clearly capture some value early on by **generating boilerplate code** and
building a **code generation toolchain with built-in points for extensibility**
to tackle the problem in phases, expanding the coverage of generated code as we
learn more about the problem.

## Pipeline Architecture
In order to decouple interpretation of a Provider API description from the code
generation process, it seems useful to borrow the concept of frontend/backend
layering from compilers. In a traditional compiler, the frontend is responsible
for parsing source code and generating an intermediate format, often an abstract
syntax tree, which the backend parses in order to perform platform specific code
generation. This separation of concerns allows a compiler toolchain to interpret
different languages on the frontend and target different architectures on the
backend.

In our case, the backend would be responsible for generating Controller
boilerplate independent of the Provider. This allows different frontend
implementations to be written for different Providers, or possibly using
different API metadata sources for the same Provider. For instance, a frontend
could analyze an OpenAPI document, cloud provider specific discovery documents,
or directly parse cloud provider SDKs. In between backend and frontend we need
to define an intermediate format (like an AST in a compiler).

## MRS

Since the CRD is the lingua franca of Kubernetes API objects, it makes intuitive
sense to use a json-schema representation CRDs, with a few changes. First, we
may want to simplify the schema, trimming away aspects that exist to support the
Kubernetes runtime environment, like schema versioning and subresources. Second,
there are additional metadata we expect to infer from the Provider, or to be
overlayed into the intermediate format by developers, which we want to maintain
push downstream, like documentation, or metadata indicating that a field is
optional, mutable, read-only, or useful for `controller-tools` annotations.
Using a tree-like data structure makes for a simpler and less heuristic parser
compared to scanning through a Go source tree, and should be easier to work with
in tests.  Using a json-schema representation allows us to leverage json-schema
validation and libraries.  Managed Resource Schema (MRS) is the working title
for this format.

## Open Questions

### Provider API metadata sources
More work is needed to determine which source to use for generating the MRS for
each Provider API. Some possibilities:

**OpenAPI**: Potentially allows us to write the simplest frontend parser; major
cloud providers either provide OpenAPI documents (Alibaba, Azure) or well-known
discovery formats for which mechanical translators are available (Google), or
bundle OpenAPI documents in their SDK repos for runtime support or to drive code
generation (AWS).
* Azure: [directly supports OpenAPI] 
* Alibaba: [Alibaba API Explorer] 
* GCP: the documents in the [OpenAPI Directory] are created by munging the GCP
  Discovery API Service with an open source API description format conversion
  tool
* AWS: the documents in the [OpenAPI Directory] are pulled from the
  https://github.com/aws/aws-sdk-js/ repository; several official AWS SDKs
  duplicate these same documents

**Terraform**: Has support for [AWS][terraform-aws], [GCP][terraform-gcp],
[Azure][terraform-azure], [Alibaba Cloud][terraform-alicloud], and
[more][terraform-providers]. Terraform Providers do follow some internal
conventions, and Hashicorp provide libraries like [terraform-config-inspect], so
it may be easier to parse Terraform code rather than write a different parser
for each Provider.  Terraform resources also solve the problem of merging
verb-specific resources (create/read/update/delete) into a single declarative
object.

**Provider SDK**: This is like the Terraform option; maybe better, maybe worse.
Providers might have cleaner structure than Terraform because their SDKs are
designed to be used and read by developers (vs Terraform where provider code is
“internal”, and where additional complexity exists to support Terraform internal
requirements). This is probably high effort because we would need to do it over
again for each Provider, however it could be easier for developers to fill in
CRUD controller methods using well-documented and Provider-supported SDK code.

**Provider-specific Infrastructure DSL**: rather than trying to represent the
direct resource-level APIs, we could build on top of cloud provider "stack
management" DSLs (AWS: [CloudFormation][aws-cloudformation]; GCP: [Config
Connector][gcp-config-connector]; Azure: [Resource
Manager][azure-resource-manager], Alibaba: [ROS][alibaba-ros])). In the case of
Azure this appears to be the same as OpenAPI because Resource Manager is the way
Azure presents a consistent OpenAPI schema for everything.  Config Connector’s
install package has CRDs which represent the resources supported by CC. We could
mechanically transform these CRDs into MRSs. Note: Deployment Manager Resources
reflect the properties of the GCP resources they mirror, but are generic
structures from the SDK point of view, so they don’t give us any additional
source of structured data to scrape. Alibaba ROS documentation includes a
json-based description of properties for every supported resource, but it's
unclear whether these are available in a machine-readable format (other than
scraping markdown documentation, which is a possibility).

**Provider-specific discovery documents**: This mostly applies to [Google
Discovery Documents]; Azure and Alibaba both directly support OpenAPI. If AWS
has a service discovery format I haven’t been able to find it yet, but the other
option here would be to use the [CloudFormation Resource Specification],
particularly if we use CloudFormation for CRUD under the hood rather than
individual AWS resource APIs.

### Merge Patching / Injection / Overlays
The generation of CRD types and reconciliation methods are challenging when the
cloud provider interactions don’t map 1:1 with the CRD types. Instead of the
kubebuilder paradigm of generating source code files that users modify, it would
be interesting to explore injection points where some of the components outlined
in “Output Targets” could be replaced or augmented, possibly in a middleware
model. The code generation process should be modular and pluggable, allowing for
customizations to the rendered code. Knowing exactly what form these overlays
will take is going to require first prototyping the MRS->Go tools, but here are
a few thoughts on what this could look like.

It’s possible that we’ll wind up with injection points in the Frontend and
Backend of the generation pipeline; manipulating CRD structure in the Frontend,
and customizing code generation in the Backend. For instance in the Frontend,
the json-schema object structure can be directly manipulated as in-memory go
structs or json. Driving code generation by manipulating the MRS structure could
be less fragile than directly working with the code generation routines in the
backend. For a problem like customizing Reference resolution, we probably also
need backend hooks where user-supplied code can be directly invoked.

The generated code could also follow a plugin registration pattern (keyed to a
particular provider+resource+component) where developers can
override/augment/append generated code without touching the generic code
generation functions. Given that triggering code generation could have
unintended side effects when provider metadata descriptions are revised, this
could give developers more control to modify controllers without worrying about
broader versioning concerns.

### Unstructured data
Provider types can have cross-type relationships, for instance a subnetwork in
GCP belongs to a VPC. The reference in the APIs can be represented as a simple
string, with no “strongly typed” relationship in the generated code. As we
assess Provider API discovery documents, we should observe whether these
relationships are represented in a structured way that could drive code
generation to build [Reference Resolvers]. It’s possible that the resolution
logic is complex enough to move into Overlays, and all we need to solve at this
stage is making sure we preserve metadata from the provider in the MRS.

### Deprecate Kubebuilder
Most of the interesting code generation we get from Kubebuilder actually seems
to come from [controller-tools]. Kubebuilder is more geared towards generating
boilerplate, which will be superseded by the boilerplate generated by this
project. We will likely want to remove `kubebuilder` from our workflow and
directly work with [controller-tools]/[controller-runtime], building from the
annotated CRD structure instead of special comments in go code.

## Project Phases

**Phase 1 (1-2 sprints)**: prototype. Design the MRS schema and write a library
for interfacing with it. Write a prototype translation between a GCP resource
(CloudSQL?) and the MRS format. Implement translation from the MRS format to a
xx_types.go generated source file and structural boilerplate (eg controller
scaffolding but not provider API interaction). Hand code a controller using
these generated types, both to get a sense for whether this phase is releasable
on its own as well as to stress test the MRS structure and ensure GCP metadata
supports the full translation process. Outcome of this phase will include a
detailed design doc which can be shared with development partners for feedback,
and firmer timelines for the next phases.

**Phase 2**: Pick equivalently deep Alibaba, Azure and AWS resources as CloudSQL
to go through the same translation exercise, from provider metadata ->
xx_types.go. To run the gamut of AWS API generations, we may want to work
through an older API like S3, as well as something newer. The outcome of this
phase should be a releasable library for interacting with the MRS format, and a
complete implementation of the MRS->xx_types.go generation pipeline for a few
resources. By the end of this phase we should also have clarity on the major
Open Questions listed above.

If the OpenAPI approach turns out to make sense, we also should run the MRS
generation tool against the broad surface of Provider APIs and build out a
backlog of special cases and/or open issues against the core translation tool to
address general issues / patterns that weren’t obvious when analyzing a
particular API. For other approaches I expect there to be more edge cases and
patterns to tease out which will probably spill over into phase 3.

**Phase 3**: At this point we should refactor `angryjet` into the generation
tool so it can operate on the MRS structure rather than process .go file outputs
downstream, also deprecating `kubebuilder` in favor of directly working with the
underlying `controller-tools` library. Documentation and build pipelines should
be updated to reflect the new tooling. Exit from this phase will also be gated
by resolving all the P0 issues with provider->MRS->boilerplate pipeline; we
should have a working provider->MRS pipeline for AWS, Azure and GCP. Engage
development partners to kick the tires on generation for particular providers.

**Phase 4**: Clean up phase. At this point we should have a mature tool chain
and process for building controller boilerplate that we can document and demo.
Guide new developers on the major cloud providers to use this tool. If we feel
that it is worth attempting to generate deeper CRUD automation beyond just
scaffolding and boilerplate, we should at this point generate a new proposal for
what that will look like in the next major version of this tool.

## Related Works

### Azure AutoRest (and other OpenAPI client generator tools)
[AutoRest] is a tool built by the Azure team to generate client bindings for an
OpenAPI endpoint, including client bindings underlying the Azure SDK, which is
used by the Terraform Azure provider. If the tool turns out to be sufficiently
generic we could explore it for generating client bindings based on OpenAPI
documents for other providers, although other OpenAPI client generation tools
exist like [openapi-generator]. We’ll want to take a closer look at OpenAPI code
generation tools if we pursue that direction for other providers.

### AWS Service Operator (ASO)
The [pipeline architecture][aso-v2-architecture] of the next iteration of
[Amazon Service Operator][amazon-aso] appears to be very similar to what we are
considering, leapfrogging kubebuilder and working directly with
`controller-tools` to generate controller code. The existing ASO uses
CloudFormation for Provider API metadata discovery; it’s unclear from the
architecture doc if they plan to continue to use CloudFormation or another API
description: “The first code generation phase consumes model information from a
canonical source of truth about an AWS service”.

### Magic Modules
[Magic Modules] is a Google project, written in Ruby, that does low-level code
generation, translating Google’s discovery format to bindings for Terraform and
Ansible.

## Kubeform
[KubeForm] generates CRDs using Hashicorp’s [terraform-config-inspect] library
to extract types from Terraform providers, rendering them to CRDs and
boilerplate Go types via Go templates. Under the hood the controller (KFC) uses
the generated types to for k8s API interactions and templates out Terraform
manifests for resources requiring reconciliation, [directly exec()’ing
`terraform apply`][kubeform-terraform-apply]. This project does not appear to be
actively maintained.

### Pulumi
[Pulumi] is internally implemented as a wrapper on top of Terraform providers;
it is effectively a Terraform SDK. Pulumi not only uses Terraform to map out
provider types and methods, but also links in Terraform types and interacts with
Terraform's state file data structure through an adapter layer. A lot of the
machinery in pulumi exists to present a consistent interface to Terraform across
other supported languages inc javascript, python and .NET.


[Alibaba API Explorer]: https://api.aliyun.com/#/ 
[AutoRest]: https://github.com/Azure/autorest 
[CloudFormation Resource Specification]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-resource-specification.html
[Google Discovery Documents]: https://developers.google.com/discovery/v1/reference/apis
[KubeForm]: https://github.com/kubeform/kubeform 
[Magic Moduels]: https://github.com/GoogleCloudPlatform/magic-modules 
[OpenAPI Directory]: https://github.com/APIs-guru/openapi-directory
[Pulumi]: https://github.com/pulumi/pulumi 
[Reference Resolvers]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
[alibaba-ros]: https://www.alibabacloud.com/product/ros 
[amazon-aso]: https://github.com/aws/aws-service-operator-k8s 
[aso-v2-architecture]: https://github.com/jaypipes/aws-service-operator-k8s/blob/91e63414efb00564662adf6eaafc20e124a3b2d3/docs/code-generation.md
[aws-cloudformation]: https://aws.amazon.com/cloudformation/ 
[azure-resource-manager]: https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/overview
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[controller-tools]: https://github.com/kubernetes-sigs/controller-tools
[directly supports OpenAPI]: https://github.com/Azure/azure-rest-api-specs 
[gcp-config-connector]: https://cloud.google.com/config-connector/docs/overview
[kubeform-terraform-apply]: https://github.com/kubeform/kfc/blob/master/pkg/controllers/terraform.go
[openapi-generator]: https://github.com/OpenAPITools/openapi-generator 
[resource.Managed interface]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/resource#Managed
[terraform-alicloud]: https://github.com/terraform-providers/terraform-provider-alicloud
[terraform-aws]: https://github.com/terraform-providers/terraform-provider-aws
[terraform-azure]: https://github.com/terraform-providers/terraform-provider-azurerm
[terraform-config-inspect]: https://github.com/hashicorp/terraform-config-inspect/ 
[terraform-gcp]: https://github.com/terraform-providers/terraform-provider-google
