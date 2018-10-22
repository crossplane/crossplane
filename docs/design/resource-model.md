# Overview

In this document we propose a model for managing cloud infrastructure across a set of disparate cloud providers and services, ranging from low level infrastructure like servers and clusters to higher level infrastructure like databases and message queues.

We use a declarative management approach in which each piece of infrastructure is modeled as a *Resource* that abstracts the underlying implementation details. We enable full lifecycle management of resources through a set of controllers that provision, manage, scale, failover, and actively respond to external changes that deviate from the desired state.

Applications wanting to consume resources express their intent using *resource claims*. Claims are requests for resources and have granular requirements and limits. A *scheduler* is responsible for finding the best matching *resource* to satisfy the claim. The scheduler is extensible and can be tailored to specific workloads enabling availability, performance, cost and capacity optimizations. Claims also enable a higher degree of workload portability, enabling applications to express their resource requirements abstractly.

Resources can be statically or dynamically provisioned on consumption. Resources can be provisioned by administrators and consumed by applications without explicitly binding them, enabling a clean separation of concerns among developers, and administrators.

Our approach is heavily influenced by the design principles behind the pod and node scheduler and persistent volumes in Kubernetes.

## Goals:
- A higher degree of workload portability via resource abstractions.
- A clear separation of concern between administrators and developers.
- Static and dynamic provisioning of resources.
- Late binding of resources to applications.
- Advanced scheduling techniques via resource requirements, limits and quotas.
- Lifecycle management and orchestration via controllers.
- A rich extensibility to enable wider adoption.
- Leverage as much of the Kubernetes infrastructure as possible.

## Non-Goals:
- Replace managed services in cloud providers.

# Resources and Claims

A *resource* is a piece of infrastructure. To the extent possible we model *abstract* resources that are not tied to a specific provider or implementation.

For example, a `RelationalDatabase` is a resource that can be used to implement a relational database like MySQL or PostgreSQL. The implementation of the database can come from a managed service in a cloud provider or run as a set of containers in a Kubernetes cluster.

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: RelationalDatabase
metadata:
  name: my-database
spec:
  engine: mysql
  capabilities:
    highly-available: true
  capacity:
    storage: 50Gi
  reclaimPolicy: Retain
  ...
```

A `Claim` represents a request to provision and consume a resource by an application. It also acts as the claim checks to the resource. A *scheduler* is responsible for *binding* claims to resources based on criteria and using various scheduling techniques and optimizations.

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: RelationalDatabaseClaim
metadata:
  name: my-database
spec:
  engine: mysql
  resources:
    requests:
      storage: 25Gi
  ...
```

Note that we model resources and claims as specific types of resource like `RelationalDatabase` and not as generic meta-types like `Resource`. This enables stronger typing and also is closer in spirit to what an application wants to consume.

# Lifecycle of a Resource

The interactions of resources and claims follow this lifecycle:

## Provisioning
Resources are provisioned either statically or dynamically.

An administrator can statically provision for consumption by applications. Resources carry the implementation details required to provision them. Once provisioned they are available for consumption by claims.

If no resources are find that satisfy a claim, an administrator can enable dynamic resource provisioning in which a resource is created as needed. Dynamic provisioning requires the use of *resource classes* which are described below.

## Binding

When an application creates a claim, the scheduler will attempt to find a matching resource, and binds them together. When using dynamic provisioning, a resource can be created on the fly and bound to the claim. Claims can remain unbound indefinitely if a matching resource is not found, but will become bound one a resource becomes available. Depending on the resource, multiple claims can be bound to the same resource.

## Using

Applications can consume resource directly from Pods. One a claim is bound, the Pod could consume it's connection details via a configMap or secret object.

# Resource

Every resource contains a spec and status. The spec defines the declarative state of the resource, and status defines it's actual state. Instead of showing every resource type, we show the general pattern we use when defining resource types:

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: <ResourceName>
metadata:
  name: my-database
spec:
  capabilities:
    # general capabilities of the resource. these are used 
    # by the scheduler when binding claims
  capacity:
    # capacity and limits of the of the resource. these are used 
    # by the scheduler when binding claims
  reclaimPolicy: Retain
  # a template for the underlying resource implementation
  template:
    apiVersion: database.aws.conductor.io/v1alpha1
    kind: RDSInstance
    metadata:
      label: foo
    spec:
      class: db.t2.small
      engine: mysql
      masterUsername: masteruser
```

TODO: can the implementation spec reference something from the containing resource spec? Like size. what about other capabilities? Should we do this via a scoped template language?

# Claim

A claim is a request for a resource and acts as a claim check for it. Claims are defined by applications when they want to consume resources. Instead of showing every claim type, we show a general

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: <ClaimName>
metadata:
  name: my-database
spec:
  resources:
    # specifies any quantities or limit requests on resources. These are matched
    # against the capacity section of a resource
  requests:
    # requests for functionality that are matched against the capabilities section
    # of a resource
  # a claim can request a particular class by specifying a name.
  class: slow
  # claims can specify a selector to further filter the resources
  selector:
    matchLabels:
      release: "stable"
    matchExpressions:
      - {key: environment, operator: In, values: [dev]}
```

# Resource Class

Resource claims are designed for portability, and as such they can not specify provider or implementation specific requirements. Instead we use the concept of a *resource class* to enable varying implementations or "classes" of resources. Different classes might map to quality-of-service levels, or service plans, or to arbitrary policies determined by the administrators. An administrator can define a resource class as follows:

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: <ClassName>
metadata:
  name: my-database-class
spec:
  reclaimPolicy: Retain
  # a template for the underlying resource implementation
  template:
    apiVersion: database.aws.conductor.io/v1alpha1
    kind: RDSInstance
    metadata:
      label: foo
    spec:
      class: db.t2.small
      engine: mysql
      masterUsername: masteruser
```

