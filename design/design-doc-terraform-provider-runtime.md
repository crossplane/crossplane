# Generating providers using terraform-provider-runtime

* Owner: Kasey Kirkham (@kasey)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background and Purpose

In order to accelerate the expansion of Crossplane provider coverage, and to
automate the creation of Crossplane providers for smaller services which have an
existing Terraform provider implementation, we are working on a project to use
code generation to automatically create Crossplane Reconcilers and related
resources such as CRDs and their `resource.Managed` implementation from
Terraform schema metadata. The final implementation uses Terraform plugin
binaries, via Terraform’s grpc api, to handle executing the provider-side CRUD
interactions.

In order to build code generated software, it helps to first build a prototype
of the final product of code generation. The purpose of this document is to
review the design of that prototype and the supporting
`terraform-provider-runtime` library, before describing the steps needed to move
past bootstrapping, outlining subsequent phases needed to bring the project to a
usable alpha.


## Overview of existing prototype design


### Repos

note: these projects are under my account during R&D, but they all use
`github.com/crossplane` in their import paths. They are in various states of
refactoring, `terraform-provider-gen` being the most messed up as it bears the
vestigial cmds and pkgs of the earlier prototype.



*   [https://github.com/kasey/provider-terraform-gcp](https://github.com/kasey/provider-terraform-gcp)
    *   example of what a generated provider would look like, using the GCP IAM
        resource type.
*   [https://github.com/kasey/terraform-provider-runtime](https://github.com/kasey/terraform-provider-runtime)
    *   common library and runtime support for generated providers
*   [https://github.com/kasey/terraform-provider-gen](https://github.com/kasey/terraform-provider-gen)
    *   where code generation toolchain will live, currently just holds some
        scraps from the earlier prototype post-refactoring that will be helpful
        when we build the code generator


### Structure


#### terraform-provider-runtime

Instead of generating a separate controller type for each resource type, the
prototype uses a common `ExternalClient` runtime which dispatches calls to
resource-specific plugin code, wired in through dependency injection. The
Runtime library consists of:


*   `pkg/client/`: encapsulates the details of managing a pool of Terraform
    provider plugin subprocesses. These processes are shared across resource
    types, and accessed via the Terraform plugin grpc protocol.
*   `pkg/plugin/`: Types and interfaces needed to register, and access, the
    dependency injected, resource-specific code.
*   `pkg/api/`: Uses the plugin package to offer a CRUD API that operates on
    `resource.Managed` values, and abstracts away the logic for translating a
    `Reconciler` CRUD operation on a `resource.Managed` to the equivalent set of
    operations against the Terraform grpc api, with serialization and diffing
    handled by the injected plugin code.
*   `pkg/controller`: Connect implementation to borrow (and lazy init) client
    connections from the pool. Uses the api package to fulfill the
    ExternalClient interface.


#### `client` package

The Client library manages a pool of Terraform connections (and processes). A
connection is borrowed from the pool in the
[Connect](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/controller/terraform/connector.go)
method of the reconciliation loop. The `Connect` method also spawns a goroutine
which cleans up the connection and returns it back to the pool once the context
passed in from `Reconcile()` is canceled (ie once the Reconciler loop
completes). So the lease on a connection spans the entire `Reconcile` pass. The
code assumes that the Terraform provider binary is present in a path with no
other instances of the provider, which can be guaranteed by the container build
process. The plugin will live in a fixed canonical location for images generated
by the build pipeline but a flag exists to set a different path for development
purposes.


##### Blocking behavior and timeouts

Terraform API calls are blocking; a long blocking operation will take a
connection out of the pool until a response is received from the cloud. Since
connections will be shared by reconcile loops for multiple Kinds, a slow
resource request could exhaust the pool and delay queue processing. It would be
helpful to provide some visibility into the pool status to help observe the
internal state and rule out other issues when the queue backs up.


#### `plugin` package and generated code

The plugin package supports the separation of concerns between generic
reconciliation logic and the resource-specific methods for serialization and
initialization. It also supports the goal of supporting multiple layers of
implementation, with user-defined implementations overwriting generated code at
different levels; either an entire resource, or individual fields in the
`plugin.Implementation`. The plugin package consists of the following
components:


*   `ProviderInit`: (probably a bad name) exists primarily because the
    Crossplane provider configuration and credentials will need to be translated
    for use by Terraform in a resource-specific way. `ProviderInit.Initializer`
    is expected to internally manage looking up the CR for itself, perform
    Terraform-specific config translation, and set up a Terraform plugin
    subprocess/connection. It is invoked by the connection pool in a lazy
    initialization style when a resource’s Connect method is called.
*   **Interfaces**: the resource-specific and provider-specific code has been
    broken down into a set of single-method interfaces. These interfaces are
    described in the plugin package:
    *   **provider**
        [interface](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/plugin/provider.go),
        [Initializer](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/client/provider.go#L112),
        [implementation](https://github.com/kasey/provider-terraform-gcp/blob/master/generated/provider/v1alpha1/index.go):
        `ProviderInit` is a struct mapping a runtime metadata (GVK and Scheme)
        with the initialization function needed to register the provider CRD and
        pass through all the values necessary for the connection pool to work
        around the existing ExternalConnector interface. Initializer should
        possibly move from the client package to plugin.
    *   **compare**
        [interface](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/plugin/compare.go),
        [implementation](https://github.com/kasey/provider-terraform-gcp/blob/master/generated/iam/v1alpha1/compare.go):
        `ResourceMerger` encapsulates the logic for merging the cluster and
        provider representations of a managed resource. The return value is a
        `MergeDescription`, a bit mask -ish type, describing what kinds of
        mutations have occurred so the controller can decide if it needs to
        update the `Spec` field, or `Annotations` (or do other things for eg
        observability purposes).
    *   **configure**
        [interface](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/plugin/configure.go),
        [implementation](https://github.com/kasey/provider-terraform-gcp/blob/master/generated/iam/v1alpha1/configure.go):
        the `ReconcilerConfigurer` interface described here is used to do the
        initialization and registration of the `Reconciler`. For generated code
        this would typically be quite boilerplate. **This is the field that
        alternate implementations would use to overwrite the generated
        implementation**.
    *   **representations**
        [interface](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/plugin/representations.go),
        [implementation](https://github.com/kasey/provider-terraform-gcp/blob/master/generated/iam/v1alpha1/representations.go):
        interfaces describing methods for translating between `resource.Managed`
        and `cty.Value` (Terraform's native serialization format).
    *   `plugin.Implementation`
        [struct](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/plugin/implementation.go#L71),
        [implementation](https://github.com/kasey/provider-terraform-gcp/blob/master/generated/iam/v1alpha1/index.go):
        a struct holding the set of interfaces described above.

        *   **plugin.ImplementationMerger**: An ImplementationMerger collects a
            sequence of Implementations through its Overlay method. The Merge()
            method can then generate a single Implementation which is the result
            of merging all the layers into a single Implementation. It does this
            by picking a non-nil value for each field from the highest possible
            layer.
        *   **plugin.Indexer**: ImplementationMergers are specific to a single
            resource. Indexer manages creating an ImplementationMerger for each
            GVK and delegates to its Overlay method when an Implementation is
            overlaid.
        *   **plugin.Index**: generated by plugin.Indexer.BuildIndex(). This is
            the flattened representation of all the plugin.Implementations
            collected up to the point where BuildIndex is called. It is used to
            create new Invokers, which is how the merged Implementations are
            ultimately used.
        *   **plugin.Invoker**: thin wrapper around the flattened
            plugin.Implementation from plugin.Index, for syntactic sugar.
            Obtained by calling InvokerForGVK() on plugin.Index.


#### `api` package

The API methods work directly on `resource.Managed` types, allowing the
controller code to treat the resources as generic interface types and pushing
the resource-specific methods into dependency-injected callbacks. The signatures
of the functions all roughly look like:

`func Create(p *client.Provider, inv *plugin.Invoker, res resource.Managed)
(resource.Managed, error)`

[Create()
implementation](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/api/create.go)

Where `*client.Provider` is a Terraform provider subprocess connection wrapper,
and `*plugin.Invoker` provides the ability to invoke dependency injected
(usually generated) functions.

The general flow of the API CRUD methods is:


*   Get Schema from Terraform provider (over grpc)
*   Use `CtyEncoder` to get the Terraform native representation of the resource
*   Use the cty encoded value and Terraform resource name to construct a grpc
    request
*   Make grpc call to Terraform provider, handle errors
*   Use `CtyDecoder` to translate the Terraform response value back to a
    resource.Managed
*   The ExternalClient implementation takes the resulting `resource.Managed` and
    sometimes uses `ResourceMerger` to compare/update the local resource. I am
    considering a slight refactoring of this code to make the api crud methods
    an instantiated type that is used in place of the invoker, meaning the
    `Invoker` type would be hidden from the `ExternalClient`. In this model the
    `MergeDescription` that results from using ResourceMerger would probably be
    integrated into the return values, or wrapped, with the `resource.Managed`,
    in a new return type.


#### `controller` package

The
[controller](https://github.com/kasey/terraform-provider-runtime/blob/master/pkg/controller/external.go)
uses the CRUD functionality exposed by the API package and the `ResourceMerger`
for the given resource to fulfill the `ExternalClient` contract. The controller
has a different initialization scheme from other Crossplane providers. Each
generated resource package, or user-developed overlay, must have an `Index()`
method that returns the `plugin.Implementation` mapping for that resource.
Multiple layers of Implementation for a given GVK are merged via the
`*plugin.ImplementationMerger` described elsewhere.


#### main.go

The cmd for a generated provider (eg [google’s
main.go](https://github.com/kasey/provider-terraform-gcp/blob/master/main.go))
is also intended to be code generated. Most of the initialization logic has been
pushed into the controller package, so main is short and sweet:


```
providerInit := generated.ProviderInit()
idxr := plugin.NewIndexer()
generated.Index(idxr)
idx, err := idxr.BuildIndex()
kingpin.FatalIfError(err, "Failed to index provider plugin")

opts := ctrl.Options{SyncPeriod: syncPeriod}
ropts := client.NewRuntimeOptions().
    WithPluginDirectory(*pluginDirectory).
    WithPoolSize(5)
log.Debug("Starting", "sync-period", syncPeriod.String())
err = controller.StartTerraformManager(idx, providerInit, opts, ropts, log)
```

## Code Generator

The output targets for the code generator would be split between provider
outputs and resource outputs. For the provider, we would generate the main.go
entrypoint, as well as the Index() method to use the provided plugin.Indexer to
register all the injected plugin implementations layers, and the PluginInit
initialization function.

### Overlays

In order to override generated resource code with custom user code, such as
existing managed resource controllers, we need a metadata data format to
describe an alternate import path where types relating to a particular CRD can
be sourced. The `plugin.ImplementationMerger` gives us an open ended design
where we can experiment with the right level of granularity for overrides. In
both of the following cases, the parts of the processing chain that your code
will replace depends on which fields have non-nil values in the Implementation
returned by the `Index()` method found in the specified package.

*   registration level: A fully-qualified package import path would be specified
    in a configuration, and the plugin.Implementation returned by that package’s
    Index() function would take the place of generated code for the entire
    resource by specifying a `ReconcilerConfigurer`. This allows us to mix and
    match generated code with hand-written managed resources.
*   Any other member of the `plugin.Implementation`, for instance modifying the
    `ResourceMerger`, or perhaps adding additional fields to the Implementation
    type to add new capabilities, like something to handle Secrets. We’re still
    experimenting with this additional flexibility to understand if it makes
    sense.

Both of these cases could be specified in a configuration like this:

```
resource_overlays:
- terraform_name: google_service_account
  full_package_name: "github.com/crossplane/provider-gcp/apis/iam/v1alpha1/"
```

### Metadata

There are some pieces of metadata we can’t derive from the Terraform schema:
*   api group
*   kubernetes name (we can do a decent job of this automatically, but may want
    to override)
*   fields to display in kubectl lists

These need to be specified by configuration somehow. Here is a sketch of what
the configuration yaml could look like:

```
resource_metadata:
- terraform_name: google_service_account
  api_group: iam.gcp.terraform-plugin.crossplane.io
  crd_name: ServiceAccount
  kubectl_printcolumns:
  - type: string
    json_path: .spec.forProvider.displayName
    name: DISPLAYNAME
```

### Conditions

There are two conceptual levels of Status conditions in crossplane resource
code; existence and readiness. Some resources are considered ready when the
cloud provider has indicated they have been created, for instance an IAM user is
immediately available after the creation request completes. Resources like RDS
databases and Kubernetes clusters have a more fine-grained state model than
boolean existence. An RDS database has states `Creating`, `Deleting` and
`Unavailable` in addition to `Available`. The generated types will only set the
former, generic Ready condition indicating whether the resource exists in the
provider.

## Follow-up work

The following areas need additional design work. Secrets and References could
overlap somewhat in their solution but Conditions call for a more generic
solution to executing arbitrary user-specified code as part of a reconcile loop.

### Conditions

In order to replicate this state model we will need to allow custom code to
specify mappings between different response field values and our state model.
I’m also open to ideas for representing these conditions in a more declarative
fashion if anyone else has already given this some thought.

### Secrets

Crossplane in some cases generates passwords for the cloud APIs that accept
user-specified passwords. Terraform does not give any special treatment to
passwords or secrets. Passwords are specified as strings and stored in plaintext
in Terraform state files. We have a few options for how to deal with this:

1. Do the same thing as Terraform. Treat the password the same as any other
   string field in the CRD.
2. Allow references to k8s secrets to be connected to password fields in
   resource CRs
3. Add something to the semantics of the CRD that indicates a password should be
   generated for a given field (similar to our current password generation
   behavior in some cases)

I think that for most users being able to specify their own passwords will be
seen as a valuable feature. It also removes the responsibility of password
generation from our system. I suggest we start with 1 and work on a spec to
discuss how 2 or 3 would function.

The Reconciler/External contract for connection publication works by returning a
ConnectionDetails value from the Observe, Create or Update methods, so this
scheme could be specific to Terraform reconcilers.

### References

For a given field in a resource, some users may want to obtain a value by
reference at runtime, while others may want to set a value for the field
directly, or rely on composition to fill in a value. The existing resource
reference design requires 2 accessory fields for every field which needs to
support references, ie `FieldName` must also include a `FieldNameRef` with type
of `*xpv1.Reference`. This poses special challenges for code
generation since Terraform does not provide any additional metadata indicating
which fields need reference support at code generation time. We need to choose
between:

1. specifying structured metadata describing references to the code generator
2. add reference handling callbacks to the `plugin.Implementation` scheme. These
   could plug into the `Reconciler` via the
   [ReferenceResolver](https://github.com/crossplane/crossplane-runtime/blob/master/pkg/reconciler/managed/reconciler.go#L143)
   interface.
3. building a reference implementation that does not need to know how resources
   can refer to each other

Option 1 takes a lot of human toil, or plugging in to other metadata
descriptions and translating to a common format, which we need to write code
generators for etc.

Option 2 is the most similar to the implementation that we have today and would
likely be the lowest effort option at the beginning, but would create a backlog
of future work to build reference resolver implementations for every resource
where we need to support references.

Option 3 is my preference and is similar to how Terraform handles the situation,
with the question of references pushed into a user-space syntax for referring to
nested values. One possible design would be to add a Reference block to the Spec
with similar semantics to composition, but for runtime resolution. This would
have the biggest potential impact on Crossplane, but each option requires some
additional design and discussion.

## Work Phases

### Prototype

August: Get prototype working 100% against the IAM resource to determine what
code we need to generate.

### Code Generation

August/Sept:  Build code generation tooling to reproduce functionality of the
prototype resource for all Google and AWS. Adapt to resources that may vary from
what we’ve looked at already. Coverage over all resources, without references,
secrets or deep readiness. Build extension point mechanisms. Refactor code based
on core team feedback.


### References, Secrets

September/October: Write design docs and work on prototypes of these designs in
September. Begin implementation after code generation is stabilized (no major
changes based on testing of more complex resources), early October.


### Operability

October: When we get a baseline of resource support, coordinate with alpha users
to work through a dogfood MVP. Get feedback on DX from this engagement, with a
particular focus on observability and debugging.


### Conditions

Design deeper condition checks in September, begin implementation when we’re
happy with the state of References and Secrets.


### Documentation

August-October?: Investigate how we can generate documentation from Terraform
documentation. We may be able to treat this as an independent project,
particularly at the r&d phase where we look at Terraform’s doc repos and assess
how we would integrate them. This may get blocked on getting the code generator
far enough along that we can at least generate CRD go type sources for
consumption by the doc.crds.dev tools.

### Integration Testing

October+: Investigate generating integration tests for all resources. We should
ensure that we have demonstrated resources can successfully
observe/create/update/delete before releasing them, potentially releasing each
provider in chunks as we get through testing/QC on the individual resources.
