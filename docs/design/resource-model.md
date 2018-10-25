# Overview

In this document we propose a model for managing cloud infrastructure across a set of disparate cloud providers and services, ranging from low level infrastructure like servers and clusters to higher level infrastructure like databases, buckets and message queues.

We use a declarative management approach in which each piece of infrastructure is modeled as a *resource* that is consumed by applications and users. We enable full lifecycle management of resources through a set of controllers that provision, manage, scale, failover, and actively respond to external changes that deviate from the desired configuration.

Every resource has an abstract definition that is not tied to a specific cloud provider or service, and represents the application's request. An abstract resource maybe be have multiple implementations across different cloud providers and services. The abstraction of resources enables a higher degree of workload portability, separation of concerns, and enables a scheduler to optimize the deployment and placement of applications and their resources.

Application and users consume abstract resources and administrators provision concrete resources. Abstract resources can be explicitly bound to concrete ones, or matched using a criteria that honors constraints and capabilities. We support both static and dynamic provisioning of concrete resources.

Abstract and concrete resources can expose a set of capabilities including capacity, region, performance, and cost that can be used by an extensible scheduler to select among alternative implementations and deployments enabled by the administrator.

Our approach is heavily influenced by the design of the Kubernetes scheduler, and persistent volumes. We use Custom Resource Definitions (CRDs) and other extension facilities in Kubernetes for our implementation. It's our goal to provide an experience that is Kubernetes native, and aligns with existing principles and practices within the ecosystem.

## Goals:
- A higher degree of workload portability via resource abstractions.
- A clear separation of concern between administrators and developers.
- Static and dynamic provisioning of resources.
- Late binding of resources to applications.
- Advanced scheduling techniques via resource requirements, limits and quotas.
- Lifecycle management of resources via controllers.
- A rich extensibility approach.
- Leverage as much of the Kubernetes infrastructure as possible.
- Avoid creating another plugin model like CSI or volume plugins.

## Non-Goals:
- Replace managed services in cloud providers.

# Resources

A *resource* is a generic term used for any object managed by Kubernetes. For example, a `Pod` and a `PersistentVolume` are resources. For Conductor, we use the same terminology for infrastructure and cloud resources and we model them using CRDs. Every resource has a `spec` that represents its desired configuration, and a `status` that represents its actual or observed state.

We differentiate between two kinds of resources:

- **Abstract** - these are resources that represent a "request" for a piece of infrastructure, and are not tied to a specific implementation, provider or service. They are "logical" definitions from the perspective of the application consuming the resource. Abstract resources are similar to a `Pod` or a `PersistentResourceClaim` in the core Kubernetes API, in that they do not identify the implementation and instead the application and user's request for consuming resources.

- **Concrete** - these represent an actual piece of infrastructure in a give cloud provider or service. Concrete resources have all the config information to provision and manage the resource. A concrete resource can implement one or more abstract resources. A `Node` and a `PersistentVolume` are examples of concrete resources in the Kubernetes API.

Application developers define the abstract resource and they are typically in the same namespace as the application. Let's look at an abstract resource for a MySQL instance:

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: MySQLInstance
metadata:
  name: wordpress-db
  namespace: wordpress
spec:
  # the following is desired configuration of the abstract resource. The configuration
  # uses fields that common across all implementations of MySQL. These will be matched
  # against the config of concrete resources.
  version: ">= 5.6"
  masterUsername: masteruser
  # the following are requirements and hints that apply to all kinds of abstract resources.
  # they are used by a scheduler that can choose among equivalent concrete resources that
  # have been made available by the administrator. They are typically represented in terms
  # of quantities, cost and other generic capabilities of all resources.
  requirements:
    capacity:
      storage: 25Gi
    region: north-america
    cost: free-tier
  # optional resource class name that is used during dynamic provisioning of an abstract
  # resource. The class identifies a "profile" of an concrete resource.
  resourceClass: performance
  # optional resource name that can identify a specific concrete resource to bind to.
  resourceName: rds-instance-445
  # optional selector to further filter the concrete resources
  resourceSelector:
    matchExpressions:
      - {key: environment, operator: In, values: [dev]}
```

The abstract resource can be implemented by multiple concrete resources. An administrator typically defines these concrete resources and they go in a different system-wide namespace. Let's look at the resource for an RDS instance in AWS:

```yaml
apiVersion: storage.aws.conductor.io/v1alpha1
kind: RDSInstance
metadata:
  name: rds-instance-445
  namespace: conductor-system
spec:
  # the following is desired configuration of the concrete resource. The configuration
  # will be used when provisioning the external resource in AWS.
  engine: mysql
  version: 5.9
  masterUsername: masteruser
  instance-type: db.m4.xlarge
  vpc: vpc-223-551
  securityGroups:
    - sg-2323-4445
    - sg-2323-4445
  multizone: true
  # this specifies what happens to this resource when it's no longer used by the abstract resource
  rebindPolicy: Delete
  # an optional resource class name that is used during dynamic provisioning of an abstract
  # resource. The class identifies a "profile" of an concrete resource.
  resourceClass: performance
  # the following are generic capabilities of this resource. They are not tied to the kind of
  # resource like MySQL or RDS. Instead they represent quantities, cost and other generic
  # that can be used by a scheduler.
  capabilities:
    capacity:
      storage: 50Gi
    region: north-america
    cost: free-tier
```

There can be multiple concrete resources that implement the abstract one. For example, let's look at another concrete resource for a CloudSQL instance in GCP:

```yaml
apiVersion: database.gcp.conductor.io/v1alpha1
kind: CloudSQLInstance
metadata:
  name: cloudsql-instance-787
spec:
  # these properties are specific to CloudSQLInstance
  databaseVersion: MYSQL_5_7
  tier: db-n1-standard-1
  region: us-west2
  storageType: PD_SSD
  masterUsername: masteruser
  # these properties apply to all concrete resource
  rebindPolicy: Delete
  resourceClass: performance
  capabilities:
    capacity:
      storage: 50Gi
    region: north-america
    cost: free-tier
```

To support dynamic provisioning and an optimizing scheduler, the administrator can define a class of resource instead of a concrete one. When an abstract resource is requested it can be matched against these classes. Let's look at an example of a `ResourceClass`.

```yaml
apiVersion: core.conductor.io/v1alpha1
kind: ResourceClass
metadata:
  name: rds-performance
  namespace: conductor-system
# the following are generic capabilities of this resource. They are not tied to the kind of
# resource like MySQL or RDS. Instead they represent quantities, cost and other generic
# that can be used by a scheduler.
capabilities:
  capacity:
    storage: 50Gi
  region: north-america
  cost: free-tier
supportedResources:
  - MySqlInstance.v1alpha1.storage.conductor.io
  - MySqlInstance.v1beta1.storage.conductor.io
# this specifies what happens to this resource when it's no longer used by the abstract resource
rebindPolicy: Delete
# a template for the underlying resource implementation that will be used when a resource
# is dynamically provisioned. some of these properties might get overriden by the ones in the
# the abstract resource spec.
template:
  apiVersion: database.aws.conductor.io/v1alpha1
  kind: RDSInstance
  spec:
    engine: mysql
    version: 5.9
    masterUsername: masteruser
    instance-type: db.m4.xlarge
    vpc: vpc-223-551
    securityGroups:
      - sg-2323-4445
      - sg-2323-4445
```

# Lifecycle of a Resource

Resources adhere to the following lifecycle:

## Provisioning
Concrete resources are provisioned either statically or dynamically.

An administrator can statically provision a concrete resource for consumption by applications. Concrete resources carry the all implementation details required to provision them externally in a cloud provider or service. Once provisioned they are available for binding to abstract resources.

An administrator can enable dynamic resource provisioning, where if an abstract resource can not find a matching concrete resource, a new one is provisioned.

## Binding

Binding is when an abstract resource is matched with a concrete resource. Multiple abstract resources can be bound to the same concrete resource. An abstract resource can be explicitly bound by setting the `resourceName` property on it, or it will be bound based on a criteria including its configuration requirements.

Abstract resources can remain unbound indefinitely if a matching resource is not found, but will become bound once a matching concrete resource becomes available.

## Using

Applications can consume resource directly from Pods. One an abstract resource is bound, connection information is automatically generated in the same namespace as the abstract resource. This can include `Service`, `ConfigMap` and/or `Secret` objects. A pod can connect to the resource by using environment variables or volumes based on the configmaps and secrets. Every abstract resource defines it own format for secrets and configmaps.

## Rebinding

When an application is done with the abstract resource, the should delete it. This would release the binding and based on the rebind policy would tell the controller what to do with the resource. Resources that are dynamically provisioned inherit their `rebindPolicy` from the resource class. We currently support the following rebind policies:

### Retain

The `Retain` rebind policy allows for manual reuse the resource. When the abstract resource is deleted, the resource will still exist in a `Released` state. It will not be available for another binding since there might be persistent or sensitive state remaining on it. An administrator can manually make the resources available again by following these steps:
1. Delete the concrete resource. Any external infrastructure will not be deleted.
2. Manually clean up the data on the external resource.
3. Manually delete the associated external infrastructure, or if you want to reuse it, create a new concrete resource with the storage asset definition.

### Delete

For resource that support `Delete` rebind policy, the underlying resource will be immediately deleted when the claim is released. This will also delete any external infrastructure associated with the concrete resource.

-------------

TODO:
- show examples and use cases
- show the type structures
- `status` fields
- how does a concrete resource point at an external resource in AWS, GCP, Azure. can we set that manually
