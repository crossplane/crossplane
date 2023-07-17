 # Composition Functions

* Owners: Nic Cope (@negz), Sergen Yalçın (@sergenyalcin)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Background

Crossplane is a framework for building cloud native control planes. These
control planes sit one level above the cloud providers and allow you to
customize the APIs they expose. Platform teams use Crossplane to offer the
developers they support simpler, safer, self-service interfaces to the cloud.

To build a control plane with Crossplane you:

1. Define the APIs you’d like your control plane to expose.
1. Extend Crossplane with support for orchestrating external resources (e.g.
   AWS).
1. Configure which external resources to orchestrate when someone calls your
   APIs.

Crossplane offers a handful of extension points that layer atop each other to
help make this possible:

* Providers extend Crossplane with Managed Resources (MRs), which are high
  fidelity, declarative representations of external APIs. Crossplane reconciles
  MRs by orchestrating an external system (e.g. AWS).
* Configurations extend Crossplane with Composite Resources (XRs), which are
  essentially arbitrary APIs, and Compositions. Crossplane reconciles XRs by
  orchestrating MRs. Compositions teach Crossplane how to do this.

The functionality enabled by XRs and Compositions is typically referred to as
simply Composition. Support for Composition was added in Crossplane [v0.10.0]
(April 2020). From our [terminology documentation][term-composition]:

> Folks accustomed to Terraform might think of a Composition as a Terraform
> module; the HCL code that describes how to take input variables and use them
> to create resources in some cloud API. Folks accustomed to Helm might think of
> a Composition as a Helm chart’s templates; the moustache templated YAML files
> that describe how to take Helm chart values and render Kubernetes resources.

A Crossplane `Composition` consists of an array of one or more 'base'
resources. Each of these resources can be 'patched' with values derived from the
XR. The functionality enabled by a `Composition` is intentionally limited - for
example there is no support for conditionals (e.g. only create this resource if
the following conditions are met) or iteration (e.g. create N of the following
resource, where N is derived from an XR field).

Below is an example `Composition`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: AcmeCoDatabase
  resources:
  - name: cloudsqlinstance
    base:
      apiVersion: database.gcp.crossplane.io/v1beta1
      kind: CloudSQLInstance
      spec:
        forProvider:
          databaseVersion: POSTGRES_9_6
          region: us-central1
          settings:
            tier: db-custom-1-3840
            dataDiskType: PD_SSD
    patches:
    - type: FromCompositeFieldPath
      fromFieldPath: spec.parameters.storageGB
      toFieldPath: spec.forProvider.settings.dataDiskSizeGb
```

The goals of the Crossplane maintainers in designing Composition were to:

* Empower platform teams to provide a platform of useful, opinionated
  abstractions.
* Enable platform teams to define abstractions that may be portable across
  different infrastructure providers and application runtimes.
* Enable platform teams to share and reuse the abstractions they define.
* Leverage the Kubernetes Resource Model (KRM) to model applications,
  infrastructure, and the product of the two in a predictable, safe, and
  declarative fashion using low or no code.
* Avoid imposing unnecessary opinions, assumptions, or constraints around how
  applications and infrastructure should function.

The design document for Composition [captures these goals][composition-design]
using somewhat dated parlance.

Our approach to achieving our goals was heavily informed by Brian Grant’s
[Declarative application management in Kubernetes][declarative-app-management].
Brian’s document is an excellent summary of the gotchas and pitfalls faced by
those attempting to design new configuration management tools, informed by his
experiences designing Kubernetes, its precursor Borg, and many generations of
configuration languages at Google, including [BCL/GCL][bcl]. In particular, we
wanted to:

* Avoid organically growing a new DSL. These languages tend to devolve to
  incoherency as stakeholders push to “bolt on” new functionality to solve
  pressing problems at the expense of measured design. Terraform’s DSL
  unintuitively [supporting the count argument in some places but not
  others][terraform-count] is a great example of this. Inventing a new DSL also
  comes with the cost of inventing new tooling to test, lint, generate, etc,
  your DSL.
* Stick to configuration that could be modeled as a REST API. Modeling
  Composition logic as a schemafied API resource makes it possible for
  Crossplane to validate that logic and provide feedback to the platform team at
  configuration time. It also greatly increases interoperability thanks to broad
  support across tools and languages for interacting with REST APIs.

It was also important to avoid the “worst of both worlds” - i.e. growing a fully
featured ([Turing-complete][turing-complete]) DSL modeled as a REST API. To this
end we omitted common language features such as conditionals and iteration. Our
rationale being that these features were better deferred to a General Purpose
Programming Language (GPL) designed by language experts, and with extensive
existing tooling.

Since its inception the Crossplane maintainers’ vision has been that there
should essentially be two variants of Composition:

* For simple cases, use contemporary "Patch and Transform" (P&T) Composition.
* For advanced cases, bring your tool or programming language of choice.

In this context a “simple case” might involve composing fewer than ten resources
without the need for logic such as conditionals and iteration. Note that the
Composition logic, whether P&T or deferred to a tool or programming language, is
always behind the API line (behind an XR). This means the distinction is only
important to the people authoring the Compositions, never to the people
consuming them.

Offering two variants of Composition allows Crossplane users to pick the one
that is best aligned with their situation, preferences, and experience level.
For simple cases you don’t need to learn a new programming language or tool, and
there are no external dependencies - just write familiar, Kubernetes-style YAML.
For advanced cases leverage proven tools and languages with existing ecosystems
and documentation. Either way, Crossplane has no religion - if you prefer not to
“write YAML”, pick another tool and vice versa.

## Goals

The proposal put forward by this document should:

* Let folks use their composition tool and/or programming language of choice.
* Support 'advanced' composition logic such as loops and conditionals.
* Balance safety (e.g. sandboxing) with speed and simplicity.
* Be possible to introduce behind a feature flag that is off by default.

While not an explicit goal, it would also be ideal if the solution put forth by
this document could serve as a test bed for new features in the contemporary
'resources array' based form of Composition.

The user experience around authoring and maintaining Composition Functions is
out of scope for this proposal, which focuses only on adding foundational
support for the feature to Crossplane. 

## Proposal

### Overview

This document proposes that a new `functions` array be added to the existing
`Composition` type. This array of functions would be called either instead of or
in addition to the existing `resources` array in order to determine how an XR
should be composed. The array of functions acts as a pipeline; the output of
each function is passed as the input to the next. The output of the final
function tells Crossplane what must be done to reconcile the XR.

```yaml
apiVersion: apiextensions.crossplane.io/v2alpha1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance
  functions:
  - name: my-cool-function
    type: Container
    container:
      image: xkpg.io/my-cool-function:0.1.0
```

Under this proposal each function is the entrypoint of an OCI image, though the
API is designed to support different function implementations (such as webhooks)
in the future. The updated API would affect only the `Composition` type - no
changes would be required to the schema of `CompositeResourceDefinitions`, XRs,
etc.

Notably the functions would not be responsible for interacting with the API
server to create, update, or delete composed resources. Instead, they instruct
Crossplane which resources should be created, updated, or deleted.

Under the proposed design functions could also be used for purposes besides
rendering composed resources, for example validating the results of the
`resources` array or earlier functions in the `functions` array. Furthermore, a
function could also be used to implement 'side effects' such as triggering a
replication or backup.

Below is a more detailed example of an entry in the `functions` array.

```yaml
apiVersion: apiextensions.crossplane.io/v2alpha1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance
  functions:
  - name: my-cool-function
    type: Container
    # Configuration specific to `type: Container` functions.
    container:
      # The OCI image to pull and run.
      image: xkpg.io/my-cool-function:0.1.0
      # Whether to pull the function Never, Always, or IfNotPresent.
      imagePullPolicy: IfNotPresent
      # Secrets used to pull from a private registry.
      imagePullSecrets:
      - namespace: crossplane-system
        name: my-xpkg-io-creds
      # Note that only resource limits are supported - not requests.
      # The function will be run with the specified resource limits.
      resources:
        limits:
          memory: 64Mi
          cpu: 250m
      # Defaults to 'Isolated' - i.e an isolated network namespace.
      network: Accessible
      # How long the function may run before it's killed. Defaults to 10s.
      timeout: 30s
      # Containers are run by an external process listening at the supplied
      # endpoint. Specifying an endpoint is optional; the endpoint defaults to
      # the below value.
      runner:
        endpoint: unix:///@crossplane/fn/default.sock
    # An x-kubernetes-embedded-resource RawExtension (i.e. an unschemafied
    # Kubernetes resource). Passed to the function as the config block of its
    # FunctionIO.
    config:
      apiVersion: database.example.org/v1alpha1
      kind: Config
      metadata:
        name: cloudsql
      spec:
        version: POSTGRES_9_6
```

### Function API

This document proposes that each function uses a `FunctionIO` type as its input
and output. In the case of `Container` functions this would correspond to stdin
and stdout. Crossplane would be responsible for reading stdout from the final
function and applying its changes to the relevant XR and composed resources.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: FunctionIO
config:
  apiVersion: database.example.org/v1alpha1
  kind: Config
  metadata:
    name: cloudsql
  spec:
    version: POSTGRES_9_6
observed:
  composite:
    resource:
      apiVersion: database.example.org/v1alpha1
      kind: XPostgreSQLInstance
      metadata:
        name: my-db
      spec:
        parameters:
          storageGB: 20
        compositionSelector:
          matchLabels:
            provider: gcp
      status:
        conditions:
        - type: Ready
          status: True
    connectionDetails:
    - name: uri
      value: postgresql://db.example.org:5432
```

A `FunctionIO` resource consists of the following top-level fields:

* The `apiVersion` and `kind` (required).
* A `config` object (optional). This is a [Kubernetes resource][rawextension]
  with an arbitrary schema that may be used to provide additional configuration
  to a function. For example a `render-helm-chart` function might use its
  `config` to specify which Helm chart to render. Functions need not return
  their `config`, and any mutations will be ignored.
* An `observed` object (required). This reflects the observed state of the XR,
  any existing composed resources, and their connection details. Functions must
  return the `observed` object unmodified.
* A `desired` object (optional). This reflects the accumulated desired state of
  the XR and any composed resources. Functions may mutate the `desired` object.
* A `results` array (optional). Used to communicate information about the result
  of a function, including warnings and errors. Functions may mutate the
  `results` object.

Each function takes its `config` (if any), `observed` state, and any previously
accumulated `desired` state as input, and optionally mutates the `desired`
state. This allows the output of one function to be the input to the next.

The `observed` object consists of:

* `observed.composite.resource`. The observed XR.
* `observed.composite.connectionDetails`: The observed XR connection details.
* `observed.resources[N].name`: The name of an observed composed resource.
* `observed.resources[N].resource`: An observed composed resource.
* `observed.resources[N].connectionDetails`: An observed composed resource's
  current connection details.

If an observed composed resource appears in the Composition's `spec.resources`
array their `name` fields will match. Note that the `name` field is distinct
from a composed resource's `metadata.name` - it is used to identify the resource
within a Composition and/or its function pipeline.

The `desired` object consists of:

* `desired.composite.resource`. The desired XR.
* `desired.composite.resource.connectionDetails`. Desired XR connection details.
* `desired.resources[N].name`. The name of a desired composed resource.
* `desired.resources[N].resource`. A desired composed resource.
* `desired.resources[N].connectionDetails`. A desired composed resource's
  connection details.
* `desired.resources[N].readinessChecks`. A desired composed resource's
  readiness checks.

Note that the `desired.resources` array of the `FunctionIO` type is very
similar to the `spec.resources` array of the `Composition` type. In comparison:

* `name` works the same across both types, but is required by `FunctionIO`.
* `connectionDetails` and `readinessChecks` work the same across both types.
* `FunctionIO` does not support `base` and `patches`. Instead, a function should
  configure the `resource` field accordingly.

The `desired` state is _accumulated_ across the Composition and all of its
functions. This means the first function may be passed desired state as
specified by the `spec.resources` array of a Composite, if any, and each
function must include the accumulated desired state in its output. Desired state
is treated as an overlay on observed state, so a function pipeline need not
specify the desired state of the XR (for example) unless a function wishes to
mutate it.

A full `FunctionIO` specification will accompany the implementation. Some
example scenarios are illustrated below.

A function that wanted to create (compose) a `CloudSQLInstance` would do so by
returning the following `FunctionIO`:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: FunctionIO
observed: {}  # Omitted for brevity.
desired:
  resources:
  - name: cloudsqlinstance
    resource:
      apiVersion: database.gcp.crossplane.io/v1beta1
      kind: CloudSQLInstance
      spec:
        forProvider:
          databaseVersion: POSTGRES_9_6
          region: us-central1
          settings:
            tier: db-custom-1-3840
            dataDiskType: PD_SSD
            dataDiskSizeGb: 20
        writeConnectionSecretToRef:
          namespace: crossplane-system
          name: cloudsqlpostgresql-conn
    connectionDetails:
    - name: hostname
      fromConnectionSecretKey: hostname
    readinessChecks:
    - type: None
```

A function that wanted to set only an XR connection detail could return:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: FunctionIO
observed: {}  # Omitted for brevity.
desired:
  composite:
    connectionDetails:
    - type: FromValue
      name: username
      value: admin
```

A function wishing to delete a composed resource may do so by setting its
`resource` to null, for example:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: FunctionIO
observed: {}  # Omitted for brevity.
desired:
  resources:
  - name: cloudsqlinstance
    resource: null
```

A function that could not complete successfully could do so by returning the
following `FunctionIO`:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: FunctionIO
config:
  apiVersion: database.example.org/v1alpha1
  kind: Config
  metadata:
    name: cloudsql
  spec:
    version: POSTGRES_9_6
observed: {}  # Omitted for brevity.
results:
- severity: Error
  message: "Could not render Database.postgresql.crossplane.io/v1beta1`
```

### Running Container Function Pipelines

While Crossplane typically runs in a Kubernetes cluster - a cluster designed to
run containers - running an ordered _pipeline_ of short-lived containers via
Kubernetes is much less straightforward than you might expect. Refer to
[Alternatives Considered](#alternatives-considered) for details.

In order to provide flexibility and choice of tradeoffs in running containers
(e.g. speed, scalability, security) this document proposes Crossplane defer
containerized functions to an external runner. Communication with the runner
would occur via a gRPC API, with the runner expected to be listening at the
`endpoint` specified via the function's `runner` configuration block. This
endpoint would default to `unix:///@crossplane/fn/default.sock` - an abstract
[Unix domain socket][unix-domain-sockets].

Communication between Crossplane and a containerized function runner would use
the following API:

```protobuf
syntax = "proto3";

// This service defines the APIs for a containerized function runner.
service ContainerizedFunctionRunner {
    rpc RunFunction(RunFunctionRequest) returns (RunFunctionResponse) {}
}

// Corresponds to Kubernetes' image pull policy.
enum ImagePullPolicy {
  IF_NOT_PRESENT = 0;
  ALWAYS = 1;
  NEVER = 2;
}

// Corresponds to go-containerregistry's AuthConfig type.
// https://pkg.go.dev/github.com/google/go-containerregistry@v0.11.0/pkg/authn#AuthConfig
message ImagePullAuth {
  string username = 1;
  string password = 2;
  string auth = 3;
  string identity_token = 4;
  string registry_token = 5;
}

message ImagePullConfig {
  ImagePullPolicy pull_policy = 1;
  ImagePullAuth auth = 2;
}

// Containers are run without network access (in an isolated network namespace)
// by default.
enum NetworkPolicy = {
  ISOLATED = 0;
  ACCESSIBLE = 1;
}

// Only resource limits are supported. Resource requests could be added in
// future if a runner supported them (e.g. by running containers on Kubernetes).
message Resources {
  ResourceLimits limits = 1;
}

message ResourceLimits {
  string memory = 1;
  string cpu = 2;
}

message RunFunctionConfig {
  Resources resources = 1;
  NetworkPolicy network = 2;
  Duration timeout = 3;
}

// The input FunctionIO is supplied as opaque bytes.
message RunFunctionRequest {
  string image = 1;
  bytes input = 2;
  ImagePullConfig = 3;
  RunFunctionConfig = 4;
}

// The output FunctionIO is supplied as opaque bytes. Errors encountered while
// running a function (as opposed to errors returned _by_ a function) will be
// encapsulated as gRPC errors.
message RunFunctionResponse {
  bytes output = 1;
}
```

### The Default Function Runner

This document proposes that Crossplane include a default function runner. This
runner would be implemented as a sidecar to the core Crossplane container that
runs functions inside itself.

The primary advantages of this approach are speed and control. There's no need
to wait for another system (for example the Kubernetes control plane) to
schedule each container, and the runner can easily pass stdout from one
container to another's stdin. Speed of function runs is of particular importance
given that each XR typically reconciles (i.e. invokes its function pipeline)
once every 60 seconds.

The disadvantages of running the pipeline inside a sidecar container are scale
and reinvention of the wheel. The resources available to the sidecar container
will bound how many functions it can run at any one time, and it will need to
handle features that the Kubelet already offers such as pull secrets, caching
etc.

[Rootless containers][rootless] appear to be the most promising way to run
functions as containers inside a container:

> Rootless containers uses `user_namespaces(7)` (UserNS) for emulating fake
> privileges that are enough to create containers. The pseudo-root user gains
> capabilities such as `CAP_SYS_ADMIN` and `CAP_NET_ADMIN` inside UserNS to
> perform fake-privileged operations such as creating mount namespaces, network
> namespaces, and creating TAP devices.

Using user namespaces allows the runner to use the other kinds of namespaces
listed above to ensure an extra layer of isolation for the functions it runs.
For example a network namespace could be configured to prevent a function having
network access.

User namespaces are well supported by modern Linux Kernels, having been
introduced in Linux 3.8. Many OCI runtimes (including `runc`, `crun`, and
`runsc`) support rootless mode. `crun` appears to be the most promising choice
because:

* It is more self-contained than `runc` (the reference and most commonly used
  OCI runtime), which relies on setuid binaries to setup user namespaces.
* `runsc` (aka gVisor) uses extra defense in depth features which are not
  allowed inside most containers due to their seccomp policies.

Of course, "a container" is in fact many technologies working together and some
parts of rootless containers are less well supported than others; for example
cgroups v2 is required in order to limit resources like CPU and memory available
to a particular function. cgroups v2 has been available in Linux since 4.15, but
was not enabled by many distributions until 2021. In practice this means
Crossplane users must use a [sufficiently modern][cgroups-v2-distros]
distribution on their Kubernetes nodes in order to constrain the resources of a
Composition function.

Similarly, [overlayfs] was not allowed inside user namespaces until Linux 5.11.
Overlayfs is typically used to create a root filesystem for a container that is
backed by a read-write 'upper' directory overlaid on a read-only 'lower'
directory. This allows the root OCI image filesystem to persist as a cache of
sorts, while changes made during the lifetime of a container can be easily
discarded. It's possible to replicate these benefits (at the expense of disk
usage and start-up time) by falling back to making a throwaway copy of the root
filesystem for each container run where overlayfs is not available.

Under the approach proposed by this document each function run would involve the
following steps:

1. Use [go-containerregistry] to pull the function's OCI image.
1. Extract (untar) the OCI image's flattened filesystem to disk.
1. Create a filesystem for the container - either an overlay or a copy of the
   filesystem extracted in step 2.
1. Derive an [OCI runtime configuration][oci-rt-cfg] from the
   [OCI image configuration][oci-img-cfg] supplied by go-containerregistry.
1. Execute `crun run` to invoke the function in a rootless container.

Executing `crun` directly as opposed to using a higher level tool like `docker`
or `podman` allows the default function runner to avoid new dependencies apart
from a single static binary (i.e. `crun`). It keeps most functionality (pulling
images etc) inside the runner's codebase, delegating only container creation to
an external tool. Composition Functions are always short-lived and should always
have their stdin and stdout attached to the runner, so wrappers like
`containerd-shim` or `conmon` should not be required. The short-lived, "one
shot" nature of Composition Functions means it should also be acceptable to
`crun run` the container rather than using `crun create`, `crun start`, etc.

At the time of writing rootless containers appear to be supported by Kubernetes,
including Amazon's Elastic Kubernetes Service (EKS) and Google Kubernetes Engine
(GKE).

Testing using GKE 1.21.10-gke.2000 with Container Optimized OS (with containerd)
cos-89-16108-604-19 nodes (Kernel COS-5.4.170) found that it was possible to run
`unshare -rUm` (i.e. to create a new user and mount namespace) inside an Alpine
Linux container as long as AppArmor was disabled by applying the annotation
`container.apparmor.security.beta.kubernetes.io/${CONTAINER_NAME}=unconfined`.
It's possible to create user namespaces with AppArmor enabled, but not to create
mount namespaces with different mount propagation from their parent.

It is not possible to use rootless containers with gVisor enabled, as gVisor
does not yet [support mount namespaces][gvisor-mountns]. This means that it is
not possible to use rootless containers with GKE Autopilot, which requires that
gVisor be used.

Testing using EKS v1.21.5-eks-9017834 with Amazon Linux 2 nodes (Kernel
5.4.188-104.359.amzn2.x86_64) found that it was possible to run `unshare -rUm`
inside an Alpine Linux container 'out of the box'.

The `unshare` syscall used to create containers is rejected by the default
Docker and containerd seccomp profiles. seccomp is disabled ("Unconstrained") by
default in Kubernetes, but that will soon change per [this KEP][kep-seccomp]
which proposes that Kubernetes use the seccomp profiles of its container engine
(i.e. containerd) by default. Once this happens Crossplane will either need to
run with the "Unconstrained" seccomp profile, or a variant of the default
containerd seccomp profile that allows a few extra syscalls (i.e. at least
`unshare` and `mount`). This can be done by setting a Pod's
`spec.securityContext.seccompProfile.type` field to `Unconstrained`.

### Packaging Containerized Functions

This document proposes that containerized functions support Crossplane [package
metadata][package-meta] in the form of a `package.yaml` file at the root of the
flattened filesystem and/or the OCI layer annotated as `io.crossplane.xpkg:
base` per the [xpkg spec][xpkg-spec]. This `package.yaml` file would contain a
custom-resource-like YAML document of type `Function.meta.pkg.crossplane.io`.

Unlike `Configuration` and `Provider` packages, `Function` packages would not
actually be processed by the Crossplane package manager but rather by the
Composition (`apiextensions`) machinery. In practice Crossplane would be
ignorant of the `package.yaml` file; it would exist purely as a way to attach
"package-like" metadata to containerized Crossplane functions. Therefore, unlike
the existing package types the `package.yaml` would contain no `spec` section.

An example `package.yaml` might look like:

```yaml
# Required. Must be as below.
apiVersion: meta.pkg.crossplane.io/v1alpha1
# Required. Must be as below.
kind: Function
# Required.
metadata:
  # Required. Must comply with Kubernetes API conventions.
  name: function-example
  # Optional. Must comply with Kubernetes API conventions.
  annotations:
    meta.crossplane.io/source: https://github.com/negz/example-fn
    meta.crossplane.io/description: An example function
```

## Alternatives Considered

Most of the alternatives considered in this design could also be thought of as
future considerations. In most cases these alternatives don't make sense at the
present time but likely will in the future.

### Using Webhooks to Run Functions

Crossplane could invoke functions by calling a webhook rather than running an
OCI container. In this model function input and output would still take the form
of a `FunctionIO`, but would be HTTP request and response bodies rather than a
container's stdin and stdout.

The primary detractor of this approach is the burden it puts on function authors
and Crossplane operators. Rather than simply publishing an OCI image the author
and/or Crossplane operator must deploy and operate a web server, ensuring secure
communication between Crossplane and the webhook endpoint.

Support for `type: Webhook` functions will likely be added shortly after initial
support for `type: Container` functions is released.

### Using chroots to Run Functions

Crossplane could invoke functions packaged as OCI images by unarchiving them and
then running them inside a simple `chroot`. This offers more compatibility than
rootless containers at the expense of isolation - it's not possible to constrain
a chrooted function's compute resources, network access, etc. `type: Chroot`
functions would use the same artifacts as `type: Container` functions but invoke
them differently.

Support for `type: Chroot` functions could be added shortly after initial
support for `type: Container` functions are released if `type: Container` proves
to be insufficiently compatible (e.g. for clusters running gVisor, or that
require seccomp be enabled).

### Using Kubernetes to Run Containerized Functions

Asking Kubernetes to run a container pipeline is less straightforward than you
might think. Crossplane could schedule a `Pod` for each XR reconcile, or create
a `CronJob` to do so regularly. Another option could be to connect directly to a
Kubelet. This approach would enjoy all the advantages of the existing Kubelet
machinery (pull secrets, caching, etc) but incurs overhead in other areas, for
example:

* Every reconcile requires a pod to be scheduled, which may potentially block on
  node scale-up, etc.
* stdin and stdout must be streamed via the API server, for example by using the
  [`/attach` subresource][attach].
* Running containers in order requires either (ab)using init containers or
  injecting a middleware binary that blocks container starts to ensure they run
  in order (similar to Argo Workflow's '[emissary]' executor):

> The emissary works by replacing the container's command with its own command.
> This allows that command to capture stdout, the exit code, and easily
> terminate your process. The emissary can also delay the start of your process.

You can see some of the many options Argo Workflows explored to address these
issues before landing on `emissary` in their list of
[deprecated executors][argo-deprecated-executors].

### Using KRM Function Spec Compliant Functions

While the design proposed by this document is heavily inspired by KRM Functions,
the [KRM function specification][krm-fn-spec] as it currently exists is not an
ideal fit. This is because:

1. It is built around the needs of CLI tooling - including several references to
   (client-side) 'files' that don't exist in the Crossplane context. 
1. Crossplane needs additional metadata to distinguish which resource in the
   `ResourceList` is the composite resource and which are the composed
   resources.

### gVisor

[gVisor][gvisor] supports rootless mode, but requires too many privileges to run
in a container. A proof-of-concept [exists][gvisor-unpriv] to add an
`--unprivileged` flag to gVisor, allowing it to run inside a container. It's
unlikely that gVisor will work in all situations in the near future - for
example gVisor cannot currently run inside gVisor and support for anything other
than x86 architectures is experimental.

[term-composition]: https://crossplane.io/docs/v1.9/concepts/terminology.html#composition
[v0.10.0]: https://github.com/crossplane/crossplane/releases/tag/v0.10.0
[composition-design]: https://github.com/crossplane/crossplane/blob/e02c7a3/design/design-doc-composition.md#goals
[declarative-app-management]: https://docs.google.com/document/d/1cLPGweVEYrVqQvBLJg6sxV-TrE5Rm2MNOBA_cxZP2WU/edit
[bcl]: https://twitter.com/bgrant0607/status/1123620689930358786?lang=en
[terraform-count]: https://www.terraform.io/language/meta-arguments/count
[turing-complete]: https://en.wikipedia.org/wiki/Turing_completeness#Unintentional_Turing_completeness  
[pitfalls-dsl]: https://github.com/kubernetes/community/blob/8956bcd54dc6f99bcb681c79a7e5399289e15630/contributors/design-proposals/architecture/declarative-application-management.md#pitfalls-of-configuration-domain-specific-languages-dsls
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[krm-fn-spec]: https://github.com/kubernetes-sigs/kustomize/blob/9d5491/cmd/config/docs/api-conventions/functions-spec.md
[rawextension]: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#rawextension
[unix-domain-sockets]: https://man7.org/linux/man-pages/man7/unix.7.html
[rootless]: https://rootlesscontaine.rs/how-it-works/userns/
[cgroups-v2-distros]: https://rootlesscontaine.rs/getting-started/common/cgroup2/#checking-whether-cgroup-v2-is-already-enabled
[overlayfs]: https://www.kernel.org/doc/html/latest/filesystems/overlayfs.html
[go-containerregistry]: https://github.com/google/go-containerregistry
[oci-rt-cfg]: https://github.com/opencontainers/runtime-spec/blob/v1.0.2/config.md
[oci-img-cfg]: https://github.com/opencontainers/image-spec/blob/v1.0.2/config.md
[gvisor-mountns]: https://github.com/google/gvisor/issues/221
[kep-seccomp]: https://github.com/kubernetes/enhancements/issues/2413
[package-meta]: https://github.com/crossplane/crossplane/blob/035e77b/design/one-pager-package-format-v2.md
[xpkg-spec]: https://github.com/crossplane/crossplane/blob/035e77b/docs/reference/xpkg.md
[attach]: https://github.com/kubernetes/kubectl/blob/18a531/pkg/cmd/attach/attach.go
[emissary]: https://github.com/argoproj/argo-workflows/blob/702b293/workflow/executor/emissary/emissary.go#L25
[argo-deprecated-executors]: https://github.com/argoproj/argo-workflows/blob/v3.4.1/docs/workflow-executors.md
[krm-fn-spec]: https://github.com/kubernetes-sigs/kustomize/blob/9d5491/cmd/config/docs/api-conventions/functions-spec.md
[krm-fn-runtimes]: https://github.com/GoogleContainerTools/kpt/issues/2567
[krm-fn-catalog]: https://catalog.kpt.dev
[gvisor]: https://gvisor.dev
[gvisor-unpriv]: https://github.com/google/gvisor/issues/4371#issuecomment-700917549