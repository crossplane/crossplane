# Composition Functions

* Owners: Nic Cope (@negz), Sergen Yalçın (@sergenyalcin)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Composition is the Crossplane feature that lets teams define their own
opinionated APIs (i.e. Kubernetes CRs). We call these APIs "Composite
Resources", or XRs for short. When a user creates an XR, Crossplane uses another
kind of resource - a `Composition` - to determine what to do. For example a
`Composition` might tell Crossplane that when a user creates an `AcmeCoDatabase`
XR it should respond by creating a GCP `CloudSQLInstance` and a
`ServiceAccount`. From our [terminology documentation][term-composition]:

> Folks accustomed to Terraform might think of a Composition as a Terraform
> module; the HCL code that describes how to take input variables and use them
> to create resources in some cloud API. Folks accustomed to Helm might think of
> a Composition as a Helm chart’s templates; the moustache templated YAML files
> that describe how to take Helm chart values and render Kubernetes resources.

A Crossplane `Composition` consists of an array of one ore more 'base'
resources. Each of these resources can be 'patched' with values derived from the
XR. The functionality enabled by a `Composition` is intentionally limited - for
example there is no support for conditionals (e.g. only create this resource if
the following conditions are met) or iteration (e.g. create N of the following
resource, where N is derived from an XR field). These limits:

* Attempt to avoid the [pitfalls][pitfalls-dsl] of configuration domain-specific
  languages.
* Allow us to express "composition logic" as a custom resource that may be
  entirely stored within and validated at create time by the Kubernetes API
  server.
* Allow the `Composition` type to be relatively simple, avoiding it becoming a
  programming language expressed as YAML.

Below is an example `Composition`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  writeConnectionSecretsToNamespace: crossplane-system
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

Limiting the functionality of our `Composition` type allows us to reduce
Crossplane's learning curve and implementation complexity. At the same time it
limits Crossplane's ability to address many use cases. It's not uncommon for
Crossplane users to resort to 'composing' Crossplane managed resources using
more featureful tools such as Helm or jsonnet when they hit the limitations
imposed by the `Composition` type, or even writing their own Kubernetes
controllers (typically in Go, using [controller-runtime]) to build bespoke
abstractions. Furthermore, some Crossplane users simply prefer to express their
intent in a language (or with a tool) they already know and feel comfortable
with, such as Helm, jsonnet, Python, HCL, Typescript, etc.

While it's true that users _could_ simply use Helm, jsonnet, etc etc to compose
Crossplane managed resources into higher level abstractions (like the
aforementioned `AcmeCoDatabase` example), we feel that these abstractions are
best exposed as _APIs_, not as client-side constructs.

Exposing abstractions as a (Kubernetes) API:

* Avoids the need to distribute (and version) client-side tooling as part of the
  collaboration process.
* Allows abstractions to be access controlled using RBAC.
* Increases tooling support - more tools have native support for calling REST
  APIs than (e.g.) building Helm charts.
* _Further_ increases tooling support by leveraging the Kubernetes ecosystem -
  things like ArgoCD and OPA.

## Goals

The proposal put forward by this document should:

* Let folks use their composition tool and/or programming language of choice.
* Support 'advanced' composition logic such as loops and conditionals.
* Balance safety (e.g. sandboxing) with speed and simplicity.
* Be possible to introduce behind a feature flag that is off by default.

While not an explicit goal, it would also be ideal if the solution put forth by
this document could serve as a test bed for new features in the contemporary
'resources array' based form of Composition.

## Proposal

### Overview

This document proposes that a new `functions` array be added to the existing
`Composition` type. This array of functions would be called either instead of or
in addition to the existing `resources` array in order to determine how an XR
should be composed.

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
in th future. The updated API would affect only the `Composition` type - no
changes would be required to the schema of `CompositeResourceDefinitions`, XRs,
etc.

Notably the functions would not be responsible for interacting with the API
server to create, update, or delete composed resources. Instead they instruct
Crossplane which resources should be created, updated, or deleted.

Under the proposed design functions could also be used for purposes besides
rendering composed resources, for example validating the results of the
`resources` array or earlier functions in the `functions` array. Furthermore, a
function could also be used to implement 'side effects' such as triggering a
replication or backup.

### Function API

This document proposes that each function uses a `ResourceList` type derived
from the [KRM function specification][krm-fn-spec] as its input and output.

```yaml
apiVersion: fn.apiextensions.crossplane.io/v1
kind: ResourceList
items:
- apiVersion: database.example.org/v1alpha1
  kind: XPostgreSQLInstance
  metadata:
    name: my-db
    annotations:
      fn.apiextensions.crossplane.io/type: "CompositeResource"
  spec:
    parameters:
      storageGB: 20
    compositionSelector:
      matchLabels:
        provider: gcp
    writeConnectionSecretToRef:
      name: db-conn
```

Each function in the array would be executed as a pipeline, with each function:

1. Taking the XR and zero or more composed resources as input.
1. Producing the optionally mutated XR, optionally mutated composed resources,
   and newly rendered composed resources as output.

This allows the output of one function to be the input to the next. The first
function in the pipeline would be supplied with either:

1. Only the XR. This would be the case for newly created XRs whose Composition
   did not include a `resources` array.
1. The XR and one or more resources. This would be the case if either:
   1. The Composition included both a `resources` array and a `functions` array.
      In this case the `resources` array is computed first then passed to
      `functions`.
   1. This was not a newly created XR, but instead an XR that had already
      created its composed resources and was now being updated.

In the case of `Container` functions this input and output would correspond to
stdin and stdout. This may differ for future function implementations; for
example a hypothetical webhook function implementation might take the input as
an HTTP PUT request body and return the output as a response body. Crossplane
would be responsible for reading stdout from the final function and applying its
changes to the relevant XR and subsequent composed resources.

An example function that responded by asking Crossplane to create (compose) a
`CloudSQLInstance` would do so by returning the following `ResourceList`:

```yaml
apiVersion: fn.apiextensions.crossplane.io/v1alpha1
kind: ResourceList
items:
- apiVersion: database.example.org/v1alpha1
  kind: XPostgreSQLInstance
  metadata:
    name: my-db
    annotations:
      fn.apiextensions.crossplane.io/type: "CompositeResource"
  spec:
    parameters:
      storageGB: 20
    compositionSelector:
      matchLabels:
        provider: gcp
    writeConnectionSecretToRef:
      name: db-conn
- apiVersion: database.gcp.crossplane.io/v1beta1
  kind: CloudSQLInstance
  metadata:
    name: cloudsqlpostgresql
    annotations:
      fn.apiextensions.crossplane.io/type: "ComposedResource"
      fn.apiextensions.crossplane.io/name: cloudsqlinstance
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
results:
- severity: Error
  message: "Could not render Database.postgresql.crossplane.io/v1alpha1`
```

While KRM-function-like this API is not KRM function compatible. See the
[Alternatives Considered](#alternatives-considered) section for details on why.

### Running Container Function Pipelines

Crossplane is almost always deployed as a container in a Kubernetes Pod. This
makes running a pipeline of containers challenging. There are several options,
all of which boil down to one of:

1. Run the container pipeline inside Crossplane's container.
1. Ask another system to run the container pipeline.

This document proposes that Crossplane run the container pipeline inside its
container. The primary advantages of doing so are speed and control. There's no
need to wait for another system (for example the Kubernetes control plane) to
schedule each container, and Crossplane can easily pass stdout from one
container to another's stdin. Speed of function runs is of particular importance
to this design given that each XR typically reconciles (i.e. invokes its
function pipeline) once every 60 seconds.

The disadvantages of running the pipeline inside the Crossplane container are
scale and reinvention of the wheel. The resources available to the Crossplane
container will bound how many functions it can run at any one time, and
Crossplane will need to handle features that the Kubelet already offers such as
pull secrets, caching etc. In future Crossplane may support other ways to invoke
the container pipeline - refer to the [Alternatives
Considered](#alternatives-considered) section for more information.

[Rootless containers][rootless] appear to be the most promising way to run
functions as containers inside the Crossplane container:

> Rootless containers uses `user_namespaces(7)` (UserNS) for emulating fake
> privileges that are enough to create containers. The pseudo-root user gains
> capabilities such as `CAP_SYS_ADMIN` and `CAP_NET_ADMIN` inside UserNS to
> perform fake-privileged operations such as creating mount namespaces, network
> namespaces, and creating TAP devices.

Using user namespaces allows us to use the other kinds of namespaces listed
above to ensure an extra layer of isolation for the functions we run. For
example a network namespace could be configured to prevent a function having
network access.

User namespaces are well supported by modern Linux Kernels, having been
introduced in Linux 3.8. Many OCI runtimes (including `runc`, `crun`, and
`runsc`) support rootless mode. `crun` appears to be the most promising choice
because:

* It is more self-contained than `runc` (the reference and most commonly used
  OCI runtime), which relies on setuid binaries to setup user namespaces.
* `runsc` (aka gVisor) uses extra defense in depth features which are not
  allowed by most container seccomp policies.

Of course, "a container" is in fact many things and some parts of rootless
containers are less well supported; for example cgroups v2 is required in order
to limit resources like CPU and memory available to a particular function.
cgroups v2 has been available in Linux since 4.15, but was not enabled by many
distributions until 2021. In practice this means Crossplane users must use a
[sufficiently modern][cgroups-v2-distros] distribution on their Kubernetes nodes
in order to constrain the resources of a Composition function.

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

```
Usage: crun [OPTION...] COMMAND [OPTION...]

COMMANDS:
        create      - create a container
        delete      - remove definition for a container
        exec        - exec a command in a running container
        list        - list known containers
        kill        - send a signal to the container init process
        ps          - show the processes in the container
        run         - run a container
        spec        - generate a configuration file
        start       - start a container
        state       - output the state of a container
        pause       - pause all the processes in the container
        resume      - unpause the processes in the container
        update      - update container resource constraints
```

Executing `crun` directly as opposed to using a higher level tool like `docker`
or `podman` allows Crossplane to avoid new dependencies apart from a single
static binary (i.e. `crun`). It keeps most functionality (pulling images etc)
inside Crossplane's codebase, delegating only container creation to an external
tool. Crossplane functions are always short-lived and should always have their
stdin and stdout attached to Crossplane, so wrappers like `containerd-shim` or
`conmon` should not be required. The short-lived, "one shot" nature of
Crossplane functions means it should also be acceptable to `crun run` the
container rather than using `crun create`, `crun start`, etc.

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

## Alternatives Considered

Most of the alternatives considered in this design could also be thought of as
future considerations. In most cases these alternatives don't make sense at the
present time but likely will in future.

### Using Webhooks to Run Functions

Crossplane could invoke functions by calling a webhook rather than running an
OCI container. In this model function input and output would still take the form
of a `ResourceList`, but would be HTTP request and response bodies rather than a
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

### Using the Kubelet to Run Containzerized Functions

Asking another system to run the container pipeline has a different set of
challenges. Crossplane could schedule a `Pod` for each XR reconcile, or create a
`CronJob` to do so regularly. Another option could be to connect directly to a
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

### Using KRM Function Spec Compliant Functions

While the design proposed by this document is heavily inspired by KRM Functions,
the [KRM function specification][krm-fn-spec] as it currently exists is not an
ideal fit. This is because:

1. It is built around the needs of CLI tooling - including several references to
   (client-side) 'files' that don't exist in the Crossplane context. 
1. Crossplane needs additional metadata to distinguish which resource in the
   `ResourceList` is the composite resource and which are the composed
   resources.

[Work is ongoing][krm-fn-runtimes] within the `kpt` project as well as
Kubernetes sig-cli to make the KRM function specification 'server-side'
compatible; it may be possible to achieve compatibility in future. Compatibility
with the existing [catalog][krm-fn-catalog] of KRM functions would be nice to
have for functions such as `set-annotations`, `gatekeeper`, and
`render-helm-chart`.

### gVisor

[gVisor][gvisor] supports rootless mode, but requires too many privileges to run
in a container. A proof-of-concept [exists][gvisor-unpriv] to add an
`--unprivileged` flag to gVisor, allowing it to run inside a container. It's
unlikely that gVisor will work in all situations in the near future - for
example gVisor cannot currently run inside gVisor and support for anything other
than x86 architectures is experimental.

[term-composition]: https://crossplane.io/docs/v1.6/concepts/terminology.html#composition
[pitfalls-dsl]: https://github.com/kubernetes/community/blob/8956bcd54dc6f99bcb681c79a7e5399289e15630/contributors/design-proposals/architecture/declarative-application-management.md#pitfalls-of-configuration-domain-specific-languages-dsls
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[krm-fn-spec]: https://github.com/kubernetes-sigs/kustomize/blob/9d5491/cmd/config/docs/api-conventions/functions-spec.md
[argo-execs]: https://argoproj.github.io/argo-workflows/workflow-executors/
[rootless]: https://rootlesscontaine.rs/how-it-works/userns/
[cgroups-v2-distros]: https://rootlesscontaine.rs/getting-started/common/cgroup2/#checking-whether-cgroup-v2-is-already-enabled
[overlayfs]: https://www.kernel.org/doc/html/latest/filesystems/overlayfs.html
[go-containerregistry]: https://github.com/google/go-containerregistry
[oci-rt-cfg]: https://github.com/opencontainers/runtime-spec/blob/v1.0.2/config.md
[oci-img-cfg]: https://github.com/opencontainers/image-spec/blob/v1.0.2/config.md
[gvisor-mountns]: https://github.com/google/gvisor/issues/221
[kep-seccomp]: https://github.com/kubernetes/enhancements/issues/2413
[attach]: https://github.com/kubernetes/kubectl/blob/18a531/pkg/cmd/attach/attach.go
[emissary]: https://github.com/argoproj/argo-workflows/blob/702b293/workflow/executor/emissary/emissary.go#L25
[krm-fn-spec]: https://github.com/kubernetes-sigs/kustomize/blob/9d5491/cmd/config/docs/api-conventions/functions-spec.md
[krm-fn-runtimes]: https://github.com/GoogleContainerTools/kpt/issues/2567
[krm-fn-catalog]: https://catalog.kpt.dev
[gvisor]: https://gvisor.dev
[gvisor-unpriv]: https://github.com/google/gvisor/issues/4371#issuecomment-700917549