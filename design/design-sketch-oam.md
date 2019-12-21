# Kubernetes Native OAM and Crossplane

* Owners: Nic Cope (@negz), Bassam Tabbara (@bassam), Phil Prasek (@prasek)
* Reviewers: [Crossplane maintainers], [OAM maintainers]
* Status: Draft

## Background

The [Open Application Model] (OAM) is a specification for building cloud native
applications. OAM aims to enable the separation of concerns between application
developers, application operators, and infrastructure operators. Crossplane is
an open source multicloud control plane that shares many of OAM's goals around
modelling cloud native applications and separation of concerns.

The OAM specification defines four core configuration documents:

* A `ComponentSchematic` describes a component. It is a schema - something that
  may be composed to run an application, but does not represent a running
  application in and of itself. A component is of a particular workload type.
  The OAM defines core types, all of which are shapes of containerized workload.
  OAM runtimes may define "extended workloads", which define their own schema.
* An `ApplicationScope` groups components, for example by placing them in a
  particular VPC network or applying resource usage constraints on them.
* A `Trait` augments a `ComponentSchematic` with additional functionality, for
  example by adding load balancing or rate limiting to the component.
* An `ApplicationConfiguration` instantiates a group of components by specifying
  and providing inputs to their schematics, applying any traits associated with
  each schematic, and grouping the component instances into one or more scopes.

Crossplane models an application as a _workload_, of which the only extant kind
is `KubernetesApplication`. A `KubernetesApplication` is an array of templates
for Kubernetes resources of any kind that are co-scheduled and submitted to a
Kubernetes control plane (distinct from the Kubernetes control plane upon which
Crossplane runs). Crossplane _may_ support other workload kinds, for example
`ServerlessApplication`, in future.

Crossplane also models and manages the cloud infrastructure that an application
may depend upon in the form of _managed resources_, and _resource claims_. A
resource claim models the need for a service from an application's perspective,
i.e. "a MySQL instance" (`kind: MySQLInstance`). These needs are resolved by
binding the claim to a managed resource, which represents a concrete, specific,
instance of cloud infrastructure (for example `kind: MySQLServer` in Azure, or
`kind: RDSInstance` in AWS).

The Crossplane and OAM projects have similar goals. Both projects recognise the
need to model cloud native applications, and to enable the separation of
concerns between the personas who typically build and run such applications.
Both projects draw inspiration from Kubernetes, but each have embraced it to a
different extent. The configuration documents outlined by the OAM specification
are a subset of the Kubernetes [API conventions], and Rudr (the only extant open
source OAM runtime) is built on Kubernetes, but the OAM specification does not
require that runtimes take a dependency on Kubernetes. Crossplane's ambitions
extend far beyond orchestrating Kubernetes clusters - it intends to model
workloads and infrastructure that have nothing to do with Kubernetes - but it
_does_ take a dependency and build on the Kubernetes control plane. All
Crossplane configuration documents _must_ be defined as Kubernetes [Custom
Resource Definitions] (CRDs), and _must_ be reconciled by Kubernetes
[controllers].

## Goals

The goal of this proposal is to align OAM and Crossplane configuration concepts,
enabling Crossplane to act as an OAM runtime - a project capable of enacting
applications defined via OAM.

## Proposal

This document proposes that OAM take a dependency on the Kubernetes API server.
OAM configuration schema would be defined as a series of CRDs. We do not believe
this to be incompatible with OAM’s goal of supporting multiple orchestrators and
runtimes. Our experience with the Crossplane project has demonstrated that the
API server and controller combination provides a strong framework upon which to
build orchestration software, even when Kubernetes is not the desired platform
upon which to _run_ the orchestrated software and infrastructure. We believe
taking this dependency will simplify the OAM specification and its runtime
implementations. Taking this dependency would result in a stronger alignment
with the Cloud-Native community, enabling powerful scenarios around multicloud
applications. Embracing Kubernetes API conventions would also serve to make the
OAM specification itself more succinct by removing the need to re-specify common
requirements such as type and object metadata.

Specifically this document proposes:

* Each type of OAM workload - be it core or extended - be represented by a
  distinct kind of custom resource. Such resources would represent an _instance_
  of configuration to be reconciled by a runtime (i.e. controller), not a
  _schema_. The schema would be defined by a CRD. For example the core workload
  types may be recast as `kind: ContainerizedWorkload`. The existing, unmodified
  Crossplane resource claim kinds (for example `kind: RedisCluster`) could
  effectively function as an extended workload type.
* Similarly, each type of application scope and trait would be a distinct kind
  of Kubernetes resource, defined by its own CRD, for example
  `kind: NetworkScope`, or `kind: ManualScalerTrait`.
* Each `kind: ApplicationConfiguration` resource would instantiate application
  configurations by specifying the configuration for the application's
  components and traits inline. The creation of an `ApplicationConfiguration`
  would result in the creation of these 'templated' resources, each of which
  could be reconciled by its own Kubernetes controller. This is similar to how
  Crossplane's `KubernetesApplication` functions. Scopes, which may apply to
  many applications, would be configured in advance and referenced by an
  `ApplicationConfiguration`.

The following is an example of the proposed `ApplicationConfiguration` (drawing
from the examples included in the contemporary OAM specification) that includes
a Crossplane `RedisCluster` resource claim in the place of an extended workload
type, and uses a `SchedulingScope` to determine where Crossplane should deploy
the application.

```yaml
---
apiVersion: oam.crossplane.io/v1alpha1
kind: SchedulingScope
metadata:
  namespace: default
  name: production-europe
  annotations:
    description: >
      Ensures applications are scheduled to production infrastructure in Europe.
spec:
  # This label selector is used to limit where this application's components
  # may run. What exactly it selects (a Kubernetes cluster, an instance
  # group, a serverless platform, etc) is up to the runtime.
  selector:
    matchLabels:
      environment: production
      region: europe
---
kind: ApplicationConfiguration
metadata:
  namespace: default
  name: coolest-app
  annotations:
    description: A very cool cloud-native application.
spec:
  scopes:
  - scopeRef:
      apiVersion: core.oam.dev/v1alpha1
      kind: SchedulingScope
      name: production-europe
  components:
  - component:
      apiVersion: core.oam.dev/v1alpha1
      kind: ContainerizedWorkload
      metadata:
        annotations:
          version: v1.0.0
      spec:
        # Under this proposal the workloadType would be limited to the core OAM
        # types, i.e. it would define the function of the resulting workload -
        # Server, Singleton, Worker, etc. Extended workloads are a distinct kind
        # and thus kind: ContainerizedWorkload has no workloadSettings field.
        workloadType: core.oam.dev/v1alpha1.Server
        osType: linux
        containers:
        - name: my-twitter-bot-frontend
          image:
            name: example/my-twitter-bot-frontend:v1.0.0
          resources:
            cpu:
              required: 1.0
            memory:
              required: 100MB
          ports:
          - name: http
            value: 8080
          env:
          - name: USERNAME
            value: cooladmin
          - name: PASSWORD
            valueFrom:
              secretKeyRef:
                name: cool-frontend
                key: password
          - name: BACKEND_ADDRESS
            value: https://example.org
          livenessProbe:
            httpGet:
              port: 8080
              path: /healthz
          readinessProbe:
            httpGet:
              port: 8080
              path: /healthz
    traits:
    # Contemporary traits model the workload kinds to which they may apply. This
    # could be modelled as an annotation on the CRD of each Trait kind.
    - apiVersion: core.oam.dev/v1alpha1
      kind: ManualScalerTrait
      spec:
        replicaCount: 3
  - component:
      apiVersion: cache.crossplane.io/v1alpha1
      kind: RedisCluster
      spec:
        classSelector:
          matchLabels:
            environment: production
            region: europe
        writeConnectionSecretToRef:
          name: coolest-app-redis
        engineVersion: "3.2"
```

Submitting the above `ApplicationController` would cause an instance of each of
the scopes, components, and traits it templates to be created verbatim (modulo
some generated object metadata) similar to how the `template` of a Kubernetes
`ReplicaSet` becomes a set of `Pod` resources. For example:

```yaml
---
apiVersion: core.oam.dev/v1alpha1
kind: ContainerizedWorkload
metadata:
  # namespace and name are derived from the parent ApplicationConfiguration.
  # An owner reference (not modelled here) represents that this workload is
  # owned and controlled by its parent ApplicationConfiguration.
  namespace: default
  name: coolest-app-f92dm
  annotations:
    version: v1.0.0
spec:
  # Under this proposal the workloadType would be limited to the core OAM
  # types, i.e. it would define the function of the resulting workload -
  # Server, Singleton, Worker, etc. Extended workloads are a distinct kind
  # and thus kind: ContainerizedWorkload has no workloadSettings field.
  workloadType: core.oam.dev/v1alpha1.Server
  osType: linux
  containers:
  - name: my-twitter-bot-frontend
    image:
      name: example/my-twitter-bot-frontend:v1.0.0
    resources:
      cpu:
        required: 1.0
      memory:
        required: 100MB
    ports:
    - name: http
      value: 8080
    env:
    - name: USERNAME
      value: cooladmin
    - name: PASSWORD
      valueFrom:
        secretKeyRef:
          name: cool-frontend
          key: password
    - name: BACKEND_ADDRESS
      value: https://example.org
    livenessProbe:
      httpGet:
        port: 8080
        path: /healthz
    readinessProbe:
      httpGet:
        port: 8080
        path: /healthz
---
apiVersion: core.oam.dev/v1alpha1
kind: ManualScalerTrait
metadata:
  namespace: default
  name: coolest-app-n3v3y
spec:
  replicaCount: 3
---
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  namespace: default
  name: coolest-app-jnkf9
spec:
  classSelector:
    matchLabels:
      environment: production
      region: europe
  writeConnectionSecretToRef:
    name: coolest-app-redis
  engineVersion: "3.2
```

### Crossplane as an OAM Runtime

This document proposes Crossplane initially support two kinds of component:
`ContainerizedWorkload`, and Crossplane resource claims. Each containerized
workload would be rendered to a series of Kubernetes resources (e.g. `Service`,
`Deployment`, `Ingress`, etc). These resources would be created either in the
Kubernetes control plane upon which Crossplane is running (similar to Rudr), or
scheduled to an external Kubernetes cluster of which Crossplane is aware. The
proposed `SchedulingScope` would determine where the resources were scheduled.
Each resources that should be created on an external Kubernetes cluster would be
represented in the Crossplane API server as a `KubernetesApplicationResource` as
it is today. Note that these `KubernetesApplicationResource` resources would be
owned (in the controller reference sense) by the `ContainerizedWorkload`; there
would be no intermediate `KubernetesApplication`. `KubernetesApplication` would
continue to exist as a distinct, lower level concept in Crossplane, as it is
useful for scheduling and running arbitrary Kubernetes resources on an external
cluster without the separation of concerns focused abstraction of an
`ApplicationConfiguration`.

## Open Questions

The following questions are unresolved by this proposal:

* Should Crossplane resource claims natively support the concept of OAM traits?
  This proposal implies a distinct Kubernetes controller would reconcile any
  `ContainerizedWorkload` resources, which would be required to maintain object
  references to any configured traits. The proposal also implies that resource
  claims _become_ a kind of extended workload (rather than being produced by a
  kind of extended workload).
* Should Crossplane resource claims natively support the concept of OAM scopes?
  Similar to the above concerns, if an `ApplicationConfiguration` templates and
  produces resource claims verbatim (as opposed to producing a resource claim
  modified based on the supplied scopes), the controllers for said claims would
  need to be aware of scopes in order to consider them when acting on the
  resource claim.
* What is the best approach to validate the array of components and traits under
  the `ApplicationConfiguration` `components` field? One option may be to use a
  validating webhook that finds the CRD that defines the templated kind and uses
  its OpenAPI schema to validate it.
* How could we maintain the existing user experience around easily discovering
  components and traits when a trait may be of different kinds? One approach may
  be to use [CRD categories] to enable `kubectl get traits` to return all traits
  regardless of their specific kind (e.g. `SchedulingTrait`, or
  `ManualScalerTrait`).

[Crossplane maintainers]: ../OWNERS.md
[OAM maintainers]: https://github.com/oam-dev/spec/blob/49a3c62/OWNERS.md
[Open Application Model]: https://oam.dev/
[Crossplane]: https://crossplane.io/
[API Conventions]: https://github.com/kubernetes/community/blob/3879cf71c/contributors/devel/sig-architecture/api-conventions.md
[Custom Resource Definitions]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[controllers]: https://kubernetes.io/docs/concepts/architecture/controller/
[CRD categories]: https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#categories
