---
title: Concepts
toc: true
weight: 410
---
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

We model *workloads* as a schedulable units of work that the user intends to run on a cloud provider.
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

Workloads reference all the resources the consume in their `resources` section.
This helps Crossplane setup connectivity between the workload and resource, and create objects that hold connection information.
For example, for a database provisioned and managed by Crossplane, a secret will be created that contains a connection string, user and password.
This secret will be propagated to the target cluster so that it can be used by the workload.
