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
  # the following is the configuration of the abstract resource. The configuration
  # uses fields that are common across all implementations of MySQL, and/or represent
  # abstractions on-top of specific implementations.
  version: 5.6
  masterUsername: masteruser
  highly-available: true
  upgradePolicy: minor
  maintenanceSchedule: weekly
  encrypted: true
  # the following is a list of requests for compute, memory and storage resources. this
  # follows the Kubernetes resource model.
  resources:
    storage: 25Gi
```

The abstract resource can be implemented by multiple concrete resources. An administrator typically defines these concrete resources in a different system-wide namespace. Let's look at the resource for an RDS instance in AWS:

```yaml
apiVersion: storage.aws.conductor.io/v1alpha1
kind: RDSInstance
metadata:
  name: rds-instance-445
  namespace: conductor-system
spec:
  # the following is configuration of the concrete resource. The configuration
  # will be used when provisioning the external resource in AWS.
  engine: mysql
  version: 5.9
  masterUsername: masteruser
  instance-type: db.m4.xlarge
  preferredMaintenanceWindow: weekly
  autoMinorVersionUpgrade: true
  multizone: true
  resources:
    storage: 50Gi
  vpc: vpc-223-551
  vpcSecurityGroups:
    - sg-2323-4445
    - sg-2323-4445
```

There can be multiple concrete resources that implement the abstract one. For example, let's look at another concrete resource for a CloudSQL instance in GCP:

```yaml
apiVersion: database.gcp.conductor.io/v1alpha1
kind: CloudSQLInstance
metadata:
  name: cloudsql-instance-787
spec:
  # these properties are specific to CloudSQLInstance in GCP. The configuration
  # will be used when provisioning external resources in GCP.
  databaseVersion: MYSQL_5_7
  tier: db-n1-standard-1
  region: us-west2
  storageType: PD_SSD
  masterUsername: masteruser
  maintenanceWindow: weekly
  resources:
    storage: 50Gi
```

# Provisioning

A concrete resource can be statically or dynamically provisioned.

## Static Provisioning
An administrator can create a number of concrete resources that carry all implementation details required to provision them externally in a cloud provider or service. Once provisioned they are available for binding to abstract resources.

## Dynamic Provisioning

An administrator can enable dynamic resource provisioning, where if an abstract resource can not find a matching concrete resource, a new one is provisioned and bound. To enable dynamic provisioning the administrator needs to create one or more `ResourceClass` objects:

```yaml
apiVersion: core.conductor.io/v1alpha1
kind: ResourceClass
metadata:
  name: rds-performance
  namespace: conductor-system
provisioner: RDSInstance.database.aws.conductor.io
parameters:
  # a set of parameters that are passed to the provisioner when creating the concrete
  # resource dynamically.
  engine: mysql
  instance-type: db.m4.xlarge
  vpc: vpc-223-551
  securityGroups:
    - sg-2323-4445
    - sg-2323-4445
```

# Binding

An abstract resource can bind to a concrete resource explicitly by setting the `resourceName` configuration in it's spec:

```yaml
apiVersion: storage.conductor.io/v1alpha1
kind: MySQLInstance
metadata:
  name: wordpress-db
  namespace: wordpress
spec:
  # manually bind to a concrete resource 
  resourceName: rds-instance-445
```
Or it can be bound by matching the abstract resource config to the available concrete resource configs. A control loop will attempt this matching and bind the resource.
