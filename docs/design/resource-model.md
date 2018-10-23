# Overview

In this document we propose a model for managing cloud infrastructure across a set of disparate cloud providers and services, ranging from low level infrastructure like servers and clusters to higher level infrastructure like databases, buckets and message queues.

We use a declarative management approach in which each piece of infrastructure is modeled as a *Resource* that abstracts the underlying implementation details. We enable full lifecycle management of resources through a set of controllers that provision, manage, scale, failover, and actively respond to external changes that deviate from the desired configuration state.

Applications wanting to consume resources express their intent using *resource claims*. Claims are requests for resources and have granular requirements and limits. A *scheduler* is responsible for matching claims and resources. The scheduler can be tailored to specific workloads enabling availability, performance, cost and capacity optimizations.

Resources can be statically or dynamically provisioned. Resources and claims enable a higher degree of workload portability, and separation of concern by letting application owners express their resource requirements abstractly, and cluster owners define the implementation.

Our approach is heavily influenced by the design of the Kubernetes scheduler, and persistent volumes.

## Goals:
- A higher degree of workload portability via resource abstractions.
- A clear separation of concern between administrators and developers.
- Static and dynamic provisioning of resources.
- Late binding of resources to applications.
- Advanced scheduling techniques via resource requirements, limits and quotas.
- Lifecycle management and orchestration via controllers.
- Rich extensibility to enable wider adoption.
- Leverage as much of the Kubernetes infrastructure as possible.
- Avoid creating another plugin model like CSI or volume plugins.

## Non-Goals:
- Replace managed services in cloud providers.

# Resources, Claims and Classes

A *resource* is a piece of infrastructure. To the extent possible we model *abstract* resources that are not tied to a specific provider or implementation. Resources are created as CustomResourceDefinitions (CRDs) in Kubernetes. For a given abstract resource there are likely multiple CRDs involved, one for the abstract resource which is implementation independent, and one or more CRDs for the implementation of the resource.

For example, a `RelationalDatabase` is an abstract resource that can be used to implement a relational database like MySQL or PostgreSQL. The implementation of the database can come from a managed service without a public cloud provider or run as a set of containers in a Kubernetes cluster. Let's look at an example of a the `RelationalDatabase`:

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: RelationalDatabase
metadata:
  name: database-445
  namespace: conductor-system
spec:
  # the following configuration applies to all relational databases regardless of implementation
  engine: mysql
  version: 5.6
  highly-available: true
  capacity:
    storage: 50Gi
  reclaimPolicy: Delete
  # a template for the implementation of the relational database resource.
  # this template will be provisioned when an instance of the RelationalDatabase resource is created.
  template:
    apiVersion: database.aws.conductor.io/v1alpha1
    kind: RDSInstance
    metadata:
      label: foo
    spec:
      class: db.t2.small
      masterUsername: masteruser
status:
  phase: Created
  implementationRef: rds-instance-6763d
```

A *claim* represents a request to provision and consume a resource by an application. It also acts as the claim checks to the resource. A *scheduler* is responsible for *binding* claims to resources based on criteria and using various scheduling techniques and optimizations.

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: RelationalDatabaseClaim
metadata:
  name: wordpress-db
  namespace: wordpress
spec:
  # the following are requirements that will be matched against the resource, or used when
  # dynamically provisioning a resource
  engine: mysql
  version: ">= 5.6"
  # specifies any quantities or limit requests on resources. These are matched
  # against the capacity section of a resource and checked against quotas and limits.
  resources:
    requests:
      storage: 25Gi
  # optional resourceClass name that can select among a class of available resources
  class: performance
  # optional resource name that can be used to bind to a specific resource
  # instead of relying on the scheduler. When a claim is bound this is set by the scheduler.
  resourceName: database-445
  # optional selector to further filter the resources
  selector:
    matchExpressions:
      - {key: environment, operator: In, values: [dev]}
status:
  phase: Bound
```

A *class* represents a "profile" for a resource. Creating a class does not actually create the resource, instead when a claim is matched against a class, the resource can be dynamically created. An administrator might create different classes based on quality-of-service levels, or service plans, or to arbitrary policies determined by the administrators. A class does not have a spec or status, it's merely configuration.

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: RelationalDatabaseClass
metadata:
  name: slow
  namespace: conductor-system
reclaimPolicy: Retain
# a template for the underlying resource implementation that will be used when a resource
# is dynamically provisioned
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

# Lifecycle of a Resource

The interactions of resources, classes and claims follow this lifecycle:

## Provisioning
Resources are provisioned either statically or dynamically.

An administrator can statically provision a resource for consumption by applications. Resources carry the implementation details required to provision them. Once provisioned they are available for binding by claims.

An administrator can enable dynamic resource provisioning, where if a claim can not find a matching resource, a new resource is provisioned. The claim must specify the *resource class* to use when dynamically provisioning.

## Binding

Binding is when a claim is matched with a resource. A claim can be explicitly bound by setting the `resourceName` property, otherwise the scheduler will attempt to find a matching resource based on criteria including configuration requirements, resource classes, quotas, limits and others.

Claims can remain unbound indefinitely if a matching resource is not found, but will become bound once a resource becomes available. Depending on the resource, multiple claims can be bound to the same resource.

## Using

Applications can consume resource directly from Pods. One a claim is bound, connection information is automatically generated in the same namespace as the claim. This can include `Service`, `ConfigMap` and/or `Secret` objects. A pod 

## Reclaiming

When an application is done with their resource, the should delete the claim object. This would release the claim on the resource and based on the reclaim policy would tell the controller what to do with the resource. Resources that are dynamically provisioned inherit their `reclaimPolicy` from the resource class. We currently support the following reclaim policies:

### Retained

The `Retain` reclaim policy allows for manual reclamation of the resource. When the claim is deleted, the resource will still exist in a `Released` state. It will not be available for another claim since there might be persistent state remaining on the volume. An administrator can manually reclaim the resource volume with the following steps.
1. Delete the resource. The concrete resource implementation will not be deleted and as a result any external infrastructure will still exist.
2. Manually clean up the data on the associated storage asset accordingly.
3. Manually delete the associated external infrastructure, or if you want to reuse it, create a new resource with the storage asset definition.

### Delete

For resource that support `Delete` reclaim policy, the underlying resource will be immediately deleted when the claim is released. 

# Resource

Every resource contains a spec and status. The spec defines the declarative state of the resource, and status defines it's actual state. Because every abstract resource is implemented as separate CRD, we only show the config that is common across all of them here:

```yaml
apiVersion: [category].conductor.io/v1alpha1
kind: [abstractResourceKind]
metadata:
  name: database-445
  namespace: conductor-system
spec:
  [config for the abstract resource]
  # these are quantities and capacities
  capacity:
    storage: 50Gi
  # what happens to the resource when a claim is released
  reclaimPolicy: Retain
  # a template for the underlying resource implementation
  template:
    apiVersion: [concreteResourceGroup]
    kind: [concreteResourceKind]
    metadata:
    spec:
      [config for the concrete resource]
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
  name: slow
spec:
  # No relational database properties
  # resource wide properties
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

