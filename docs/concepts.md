---
title: Concepts
toc: true
weight: 310
---
# Table of Contents
1. [Concepts](#concepts)
2. [Feature Areas](#feature-areas)
3. [Glossary](#glossary)

# Concepts
## Control Plane
Crossplane is an open source multicloud control plane that consists of smart controllers that can work across clouds to enable workload portability, provisioning and full-lifecycle management of infrastructure across a wide range of providers, vendors, regions, and offerings.
The control plane presents a declarative management style API that covers a wide range of portable abstractions that facilitate these goals across disparate environments, clusters, regions, and clouds.
Crossplane can be thought of as a higher-order orchestrator across cloud providers.
For convenience, Crossplane can run directly on-top of an existing Kubernetes cluster without requiring any changes, even though Crossplane does not necessarily schedule or run any containers on the host cluster.

## Resources and Workloads
In Crossplane, a *resource* represents an external piece of infrastructure ranging from low level services like clusters and servers, to higher level infrastructure like databases, message queues, buckets, and more.
Resources are represented as persistent object within the crossplane, and they typically manage one or more pieces of external infrastructure within a cloud provider or cloud offering.
Resources can also represent local or in-cluster services.

We model *workloads* as schedulable units of work that the user intends to run on a cloud provider.
Crossplane will support multiple types of workloads including container and serverless.
You can think of workloads as units that run **your** code and applications.
Every type of workload has a different kind of payload.
For example, a container workload can include a set of objects that will be deployed on a managed Kubernetes cluster, or a reference to helm chart, etc.
A serverless workload could include a function that will run on a serverless managed service.
Workloads can contain requirements for where and how the workload can run, including regions, providers, affinity, cost, and others that the scheduler can use when assigning the workload.

## Resource Claims and Resource Classes
To support workload portability we expose the concept of a resource claim and a resource class.
A resource claim is a persistent object that captures the desired configuration of a resource from the perspective of a workload or application.
Its configuration is cloud-provider and cloud-offering independent and it’s free of implementation and/or environmental details.
A resource claim can be thought of as a request for an actual resource and is typically created by a developer or application owner.

A resource class is configuration that contains implementation details specific to a certain environment or deployment, and policies related to a kind of resource.
A ResourceClass acts as a template with implementation details and policy for resources that will be dynamically provisioned by the workload at deployment time.
A resource class is typically created by an admin or infrastructure owner.

## Dynamic and Static Provisioning

A resource can be statically or dynamically provisioned.
Static provisioning is when an administrator creates the resource manually.
They set the configuration required to provision and manage the corresponding external resource within a cloud provider or cloud offering.
Once provisioned, resources are available to be bound to resource claims.

Dynamic provisioning is when an resource claim does not find a matching resource and provisions a new one instead.
The newly provisioned resource is automatically bound to the resource claim.
To enable dynamic provisioning the administrator needs to create one or more resource class objects.

## Connection Secrets
Workloads reference all the resources they consume in their `resources` section.
This helps Crossplane setup connectivity between the workload and resource, and create objects that hold connection information.
For example, for a database provisioned and managed by Crossplane, a secret will be created that contains a connection string, user and password.
This secret will be propagated to the target cluster so that it can be used by the workload.

## Secure Connectivity
To provide secure network connectivity between application deployments in a target cluster
and the managed services they are using, Crossplane supports
provisioning and life-cycle management of networks, subnets, peering, and firewall rules to
provide secure connectivity.

## Stacks
Stacks extend Crossplane with new functionality. Crossplane provides Stacks for GCP, AWS,
and Azure that are installed with a Stack Manager that can download packages,
resolve dependencies, and execute controllers.
Stacks are designed for simplified RBAC configuration and namespace
isolation for improved security in multi-team environments. Stacks are published to a registry
where they can be downloaded, explored, and organized.

Stacks enable the community to add support for more clouds providers and and managed services.  Stacks support
out-of-tree extensibility so they can be released on their own schedule. A CLI can init,
build, publish, install, and uninstall Stacks from developer laptops or
in continuous delivery pipelines.

Stacks for GCP, AWS, and Azure support provisioning managed services (database, cache, buckets),
managed clusters (GKE, EKS, AKS), and secure connectivity (networks, subnets, firewall rules).
Stacks for independent cloud offerings can be installed alongside the Stacks for GCP, AWS, and Azure
to customize Crossplane with the right mix of managed services for your organization.

# Feature Areas
Crossplane has four main feature areas: Services, Stacks, Clusters and Workloads.

## Crossplane Services
Crossplane supports provisioning managed services using `kubectl`.  It applies
the Kubernetes pattern for Persistent Volume (PV)
claims and classes to managed service provisioning with support for a strong
separation of concern between app teams and cluster administrators.

App teams can choose between cloud-specific and portable services including
managed databases, message queues, buckets, data pipelines, and more to define
complete applications, build once, and deploy into multiple clouds using
continuous delivery pipelines or GitOps flows.

Cluster administrators can define self-service policies and best-practice
configurations to accelerate app delivery and improve security, so app teams can
focus on delivering their app instead of cloud-specific infrastructure details.

Secure connectivity between managed services and managed Kubernetes clusters is also supported
in Crossplane such that private networking can be established declaratively using
`kubectl`.

Crossplane is designed to support the following types of managed services.

### Managed Kubernetes Services
Managed Kubernetes currently supported for GKE, EKS, AKS.

Kubernetes clusters are another type of resource that can be dynamically provisioned using a
generic resource claim by the application developer and an environment specific resource
class by the cluster administrator.

Future support for additional managed services.

### Database Services
Support for PostgreSQL, MySQL, and Redis.

Database managed services can be statically or dynamically provisioned by Crossplane in AWS, GCP, and Azure.
An application developer simply has to specify their general need for a database such as MySQL,
without any specific knowledge of what environment that database will run in or even what
specific type of database it will be at runtime.

The cluster administrator specifies a resource class that acts as a template with the
implementation details and policy specific to the environment that the generic MySQL resource is being deployed to.
This enables the database to be dynamically provisioned at deployment time without the
application developer needing to know any of the details, which promotes portability and reusability.

Future support for additional managed services.

### Storage Services
Support for S3, Buckets, and Azure Blob storage.

Future support for additional managed services.

### Networking Services
Support for networks, subnets, and firewall rules.

Future support for additional managed services.

### Load Balancing Services
Future support.

### Cloud DNS Services
Future support.

### Advanced Networking Connectivity Services
Future support.

### Big Data Services
Future support.

### Machine Learning Services
Future support.

## Crossplane Stacks
Stacks extend Crossplane with new functionality.

See [Stacks](#stacks).

## Crossplane Workloads
Crossplane includes an extensible workload scheduler that observes application
policies to select a suitable target cluster from a pool of available clusters.
The workload scheduler can be customized to consider a number of criteria including
capabilities, availability, reliability, cost, regions, and performance while
deploying workloads and their resources. Complex workloads can be modeled as a `KubernetesApplication`.

## Crossplane Clusters
Crossplane supports dynamic provisioning of managed
Kubernetes clusters from a single control plane with consistent multi-cluster
best-practice configuration and secure connectivity between target Kubernetes
clusters and the managed services provisioned for applications. Managed
Kubernetes clusters can be dynamically provisioned with a `KubernetesCluster`.

# Glossary

## Kubernetes
Crossplane is built on the Kubernetes API machinery as a platform for declarative management.
We rely on common terminology from the [Kubernetes Glossary][kubernetes-glossary] where possible,
and we don't seek to reproduce that glossary here.

[kubernetes-glossary]: https://kubernetes.io/docs/reference/glossary/?all=true
However we'll summarize some commonly used concepts for convenience.

### CRD
A standard Kubernetes Custom Resource Definition (CRD), which defines a new type of resource that can be managed declaratively.
This serves as the unit of management in Crossplane.
The CRD is composed of spec and status sections and supports API level versioning (e.g., v1alpha1)

### Controller
A standard Kubernetes Custom Controller, providing active control loops that own one or more CRDs.
Can be implemented in different ways, such as
golang code (controller-runtime), templates, functions/hooks, templates, a new DSL, etc.
The implementation itself is versioned using semantic versioning (e.g., v1.0.4)

### Namespace
Allows logical grouping of resources in Kubernetes that can be secured with RBAC rules.

## Crossplane

### Stack
The unit of extending Crossplane with new functionality. A stack is a Controller that
owns one or more CRDs and depends on zero or more CRDs.

See [Stacks](#stacks).

### Stack Registry
A registry where Stacks can be published, downloaded, explored, and categorized.
The registry understands a Stack’s custom controller and its CRDs and indexes by both -- you could lookup a custom controller by the CRD name and vice versa.

### Stack Package Format
The package format for Stacks that contains the Stack definition, metadata, icons, CRDs, and other Stack specific files.

### Stack Manager
The component that is responsible for installing a Stack’s custom controllers and resources in Crossplane.
It can download packages, resolve dependencies, install resources and execute controllers.
This component is also responsible for managing the complete life-cycle of Stacks, including upgrading them as new versions become available.

### Application Stack
App Stacks simplify operations for an app by moving app lifecycle management into a Kubernetes controller
that owns an app CRD with a handful of settings required to deploy a new app instance,
complete with the managed services it depends on.

Application Stacks depend on Infrastructure Stacks like stack-gcp, stack-aws,
and stack-azure to provide managed services via the Kubernetes API.

### Infrastructure Stack
Infrastructure Stacks like stack-gcp, stack-aws, and stack-azure extend Crossplane
to support managed service provisioning (DBaaS, cache, buckets), secure connectivity
(VPCs, subnets, peering, ACLs, secrets), and provisioning managed Kubernetes clusters
on demand to further isolate the blast radius of applications.

### Cloud Provider Stack
See [infrastructure-stack](#infrastructure-stack).

### Cluster
A Kubernetes cluster.

### Managed Cluster
A Managed Kubernetes cluster from a service provider such as GKE, EKS, or AKS.

### Target Cluster
A Kubernetes cluster where application deployments and pods are scheduled to run.

### Control Cluster
See [Dedicated Crossplane Instance](#dedicated-crossplane-instance).

### Crossplane Instance
A Kubernetes cluster with:
* Crossplane installed
* One or more worker nodes where Crossplane controllers can run
* Zero or more Crossplane Stacks installed

### Dedicated Crossplane Instance
Crossplane instance running on a dedicated Kubernetes cluster 
separate from the target Kubernetes cluster(s) where
application deployments and pods are scheduled to run.

### Embedded Crossplane Instance
Crossplane instance running on a Kubernetes target cluster where app deployments and pods will run.  

### Cloud Provider
Cloud provider such as GCP, AWS, Azure offering IaaS, cloud networking, and managed services.

### Managed Service Provider
Managed service provider such as Elastic Cloud, MLab, PKS that run on cloud provider IaaS.

### Provider
A Crossplane kind that connects Crossplane to a cloud provider or managed service provider.

### Infrastructure
Infrastructure ranging from low level services like clusters and servers,
to higher level infrastructure like databases, message queues, buckets,
secure connectivity, managed Kubernetes, and more

### Infrastructure Namespace
Crossplane supports connecting multiple cloud provider accounts from
a single control plane, so different environments (dev, staging, prod) can
use separate accounts, projects, and/or credentials.

The provider and resource classes for these environments can be kept separate
using an infrastructure namespace (gcp-infra-dev, aws-infra-dev, or azure-infra-dev)
for each environment. You can create as many as you like using whatever naming works best for your organization.

### Project Namespace
When running a shared control plane or cluster it's a common practice to
create separate project namespaces (app-project1-dev) for each app project or team so their resource
are kept separate and secure. Crossplane supports this model.

### App Project Namespace
See [project-namespace](#project-namespace)

### Dynamic Provisioning
Dynamic provisioning is when an resource claim does not find a matching resource and provisions
a new one instead. The newly provisioned resource is automatically bound to the resource claim.
To enable dynamic provisioning the administrator needs to create one or more resource class objects.

### Static Provisioning
Static provisioning is when an administrator creates the resource manually. They set the configuration required to
provision and manage the corresponding external resource within a cloud provider or cloud offering.
Once provisioned, resources are available to be bound to resource claims.

### Resource
A resource represents an external piece of infrastructure ranging from low level services like clusters and
servers, to higher level infrastructure like databases, message queues, buckets, and more

### External Resource
An actual resource that exists outside Kubernetes, typically in the cloud.
AWS RDS and GCP Cloud Memorystore instances are external resources.

### Managed Resource
The Crossplane representation of an external resource.
The `RDSInstance` and `CloudMemorystoreInstance` Kubernetes kinds are managed
resources. A managed resource models the satisfaction of a need; i.e. the need
for a Redis Cluster is satisfied by the allocation (aka binding) of a
`CloudMemoryStoreInstance`.

### Resource Claim
The Crossplane representation of a request for the
allocation of a managed resource. Resource claims typically represent the need
for a managed resource that implements a particular protocol. `MySQLInstance`
and `RedisCluster` are examples of resource claims.

### Resource Class
The Crossplane representation of the desired configuration
of a managed resource. Resource claims reference a resource class in order to
specify how they should be satisfied by a managed resource.

### Cloud-Specific Resource Class
Cloud-specific Resource Classes capture reusable, best-practice configurations for a specific managed service.

For example, Wordpress requires a MySQL database which can be satisfied by CloudSQL, RDS, or Azure DB, so
cloud-specific resource classes would be created for CloudSQL, RDS, and Azure DB.

### Non-Portable Resource Class
Another term for [cloud-specific resource class](#cloud-specific-resource-class).

### Portable Resource Class
Portable Resource Classes define a named class of service that can be used by portable `Resource Claims`
in the same namespace. When used in a project namespace, this enables the
project to provision portable managed services using `kubectl`.

### Connection Secret
A Kubernetes `Secret` encoding all data required to
connect to (or consume) an external resource.

### Claimant
The Kubernetes representation of a process wishing
to connect to a managed resource, typically a `Pod` or some abstraction
thereupon such as a `Deployment` or `KubernetesApplication`.

### Consumer
See [claimant](#claimant).

### Workload
We model workloads as schedulable units of work that the user intends to run on a cloud provider.
Crossplane will support multiple types of workloads including container and serverless.
You can think of workloads as units that run your code and applications.
Every type of workload has a different kind of payload.

### Kubernetes Application
A `KubernetesApplication` is a type of workload, with a `KubernetesCluster` label selector
used for scheduling, and a series of resource templates representing resources
to be deployed to the scheduled cluster, and managed resources are provisioned
and securely connected to the application.

### In-Tree
In-tree means its source code lives in a core Crossplane git repository.

### Out-of-Tree
Out-of-tree means its source code lives outside of a core Crossplane git repository.

Often used to refer to Crossplane extensions, controllers or Stacks.

Out-of-tree extensibility enables to the community to build, release, publish,
and install Crossplane extensions separately from the core Crossplane repos.
