---
title: Services Guide
toc: true
weight: 410
---

# Services Guide 
This guide is an overview of enabling cloud service provisioning on an existing
Kubernetes target cluster, including how to integrate Crossplane with existing
cloud networking configurations to provide secure managed service connectivity.
Step-by-step instructions are provided for [GCP][gcp-services-guide],
[AWS][aws-services-guide], and [Azure][azure-services-guide].

To dynamically provision a new Kubernetes target cluster see the Stacks Guides
for [GCP][stack-guide-gcp], [AWS][stack-guide-aws], and
[Azure][stack-guide-azure].

## Table of Contents
1. [Introduction](#introduction)
1. [Secure network connectivity for cloud
   services](#secure-network-connectivity-for-cloud-services)
1. [Dynamic provisioning with claims and
   classes](#dynamic-provisioning-with-claims-and-classes)
1. [Connection secrets for pods in a
   deployment](#connection-secrets-for-pods-in-a-deployment)
1. [Next Steps](#next-steps)
1. [Learn More](#learn-more)

## Introduction 
Cloud service provisioning can be added to existing clusters by
installing Crossplane directly onto the target cluster. Crossplane is designed
to integrate with existing cloud networking and security resources, so managed
services like RDS, CloudSQL, and Azure DB can be provisioned using Kubernetes
objects and securely consumed by pods in a cluster. 

Crossplane achieves this by:
1. establishing secure network connectivity between the worker nodes in a
   cluster and cloud services
1. populating Kuberentes `Secrets` that pods in a `Deployment` can use to
   securely access the managed service

## Secure network connectivity for cloud services 
Crossplane currently supports private IP secure connectivity for AWS, GCP, and
Azure Stacks. Managed services instances are made available on the cluster's
prviate network(s) so pods can access them.  Crossplane also supports
configuring ingress/egress rules to further restrict allowed network traffic.

While each cloud provider uses different resources for establishing secure
connectivity between a Kubernetes cluster (EKS, GKE, AKS) and managed services
(RDS, CloudSQL, and Azure DB), the basic pattern is the same:
1. Configure cluster networking
   * network(s) and subnet(s) - L3 networking for the worker nodes
1. Enable managed service access: 
   * private service connection / endpoint - make services available via
     peering or other
   * private IP range(s) or subnet group - the private IPs a managed service
     will get
   * security groups or network rules - to restrict network traffic
1. Provision a managed service instance
   * creates an instance e.g. MySQL from RDS, CloudSQL, or Azure DB 
   * assigns a private IP from the private IP range above
1. Securely use the managed service with secrets
   * pods on a cluster node can access the managed service via private IP
   * pods use credentials to securely connect to a managed service

Crossplane provides Kubernetes resources for all of the above, so you can define
a secure connectivity model for the managed services you want to make available
for self-service provisioning in the cluster using claims and classes.

## Dynamic provisioning with claims and classes 
Crossplane employs a layered architecture consisting of managed resources that
represent a cloud service, and resource claims and classes that enable dynamic
provisioning of those services.

Managed resources are high fidelity representations of the API resources that
make up a cloud service. They're not portable across clouds.  A
`CloudSQLInstance` is an example of a managed resource - it's relevant only to
the Google Cloud Platform (GCP) and exposes all of the nitty gritty
configuration details of a CloudSQL instance. The networking and security
Kubernetes resources mentioned above fall into this category.

Resource claims and classes are the next layer up. Resource claims like
`MySQLInstance` enable dynamic provisioning of managed resources by matching a
claim to a class like a `CloudSQLInstanceClass` that provides the detailed
configuration template to provision a new cloud service instance. Resource
classes can reference secure connectivity resources (networks), such that new
instances of that class can be made available on the cluster's private network.
Resource classes, cluster networking, and secure connectivity resources are
designed to work together to enable self-service provisioning of securely
connected cloud services in a Kubernetes cluster.

Resource claims can be matched to a class in several ways: 
1. rely on a class marked  `resourceclass.crossplane.io/is-default-class:
"true"`
1. match on class labels using a `claim.spec.classSelector` 
1. use a `claim.spec.classRef` to a specific class

The first two methods rely on a default class of service or use a
`classSelector` that matches any suitable resource class available in the
target cluster.  As such, the first two methods are considered portable
resource claims that can be used in any cluster that provides the desired class
of service. You may have one cluster using GCP and another cluster using AWS,
and the same claim can be used in either cluster so long as the claim can be
matched to a suitable class of cloud service.

The third method uses an explicit `classRef` to a specific resource class like
a `CloudSQLInstanceClass` which means the claim may only be used with that
class.  Since resource classes are specific to a single cloud, claims that use
a `classRef` are not portable across different cloud providers.

## Connection secrets for pods in a deployment 
Resource claims automatically write a connection secret that pods in a
deployment can use to securely access the underlying cloud service. The claim's
`writeConnectionSecretToRef` field is used to specify the name of the secret
that should be created, which can then be used in the deployment's
configuration. Since the claim is created in Kubernetes, and the secret is
automatically populated by Crossplane, all cloud service secrets are managed
automatically without leaving Kubernetes.

## Next steps 
Step-by-step instructions for enabling cloud service provisioning on an
existing cluster are provided in the service guides for:
* [GCP][gcp-services-guide]
* [AWS][aws-services-guide]
* [Azure][azure-services-guide]

### Learn More
* [Join Crossplane Slack][join-crossplane-slack]
* [Contact Us][contact-us]
* [Learn More][learn-more]

<!-- Named links --> 
[gcp-services-guide]: services/gcp-services-guide.md
[aws-services-guide]: services/aws-services-guide.md
[azure-services-guide]: services/azure-services-guide.md

[stack-guide-gcp]: stacks-guide-gcp.md
[stack-guide-aws]: stacks-guide-aws.md
[stack-guide-azure]: stacks-guide-azure.md

[contact-us]: https://github.com/crossplane/crossplane#contact
[join-crossplane-slack]: https://slack.crossplane.io
[learn-more]: learn-more.md
