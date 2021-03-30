---
title: Related Projects
toc: true
weight: 1201
indent: true
---

# Related Projects

While there are many projects that address similar issues, none of them
encapsulate the full use case that Crossplane addresses. This list is not
exhaustive and is not meant to provide a deep analysis of the following
projects, but instead to motivate why Crossplane was created.

## Open Service Broker and Service Catalog

The [Open Service Broker] and the [Kubernetes Service Catalog] are able to
dynamically provision cloud services from Kubernetes. As a result it shares
similar goals with Crossplane. However, service broker does not have the
ability to define, compose, and publish your own infrastructure resources to
the Kubernetes API in a no-code way. Crossplane goes further by enabling
infrastructure operators to hide infrastructure complexity and include policy
guardrails, with a team-centric approach and a strong separation of concerns,
so applications can easily and safely consume the infrastructure they need,
using any tool that works with the Kubernetes API. Solutions like the [GCP
implementation of Open Service Broker][GCP OSB] have been deprecated in favor
of a more Kubernetes-native solution, but one that is Google-specific and
closed source.

## GCP Config Connector

The [GCP Config Connector] is the GCP replacement for Open Service Broker, and
implements a set of Kubernetes controllers that are able to provision managed
services in GCP. It defines a set of CRDs for managed services like CloudSQL,
and controllers that can provision them via their cloud APIs. It is similar to
Crossplane in that it can provision managed services in GCP. Crossplane goes 
further by enabling you to provision managed services from any cloud
provider and the ability to define, compose, and publish your own
infrastructure resources in a no-code way. Crossplane supports a team-centric
approach with a strong separation of concerns, that enables applications to
easily and safely consume the infrastructure they need, using any tool that
works with the Kubernetes API. GCP Config Connector is closed-source.

## AWS Controllers for Kubernetes

The [AWS Controllers for Kubernetes] is a recent project that implements a set of
Kubernetes controllers that are able to provision managed services in AWS. It
defines a set of CRDs for managed services like DynamoDB, and controllers that
can provision them. It is similar to Crossplane in that
it can provision managed services in AWS. Crossplane goes further by
enabling you to provision managed services from any cloud provider and the
ability to define, compose, and publish your own infrastructure API types in
Kubernetes in a no-code way. Crossplane supports a team-centric approach with a
strong separation of concerns, that enables applications to easily and safely
consume the infrastructure they need, using any tool that works with the
Kubernetes API.

## AWS CloudFormation, GCP Deployment Manager, and Others

These products offer a declarative model for deploying and provisioning
infrastructure in each of the respective cloud providers. They only work for
one cloud provider, are generally closed source, and offer little or no
extensibility points, let alone being able to extend the Kubernetes API to
provide your own infrastructure abstractions in a no-code way. We have
considered using some of these products as a way to implement resource
controllers in Crossplane. These projects use an Infrastructure as Code
approach to management, while Crossplane offers an API-driven control plane.

## Terraform and Pulumi

[Terraform] and [Pulumi] are tools for provisioning infrastructure across cloud
providers that offer a declarative configuration language with support for
templating, composability, referential integrity and dependency management.
Terraform can declaratively manage any compatible API and perform changes when
the tool is run by a human or in a deployment pipeline. Terraform is an
Infrastructure as Code tool, while Crossplane offers an API-driven control
plane.

<!-- Named Links -->

[Open Service Broker]: https://www.openservicebrokerapi.org/
[Kubernetes Service Catalog]: https://kubernetes.io/docs/concepts/extend-kubernetes/service-catalog/
[GCP OSB]: https://cloud.google.com/kubernetes-engine/docs/concepts/google-cloud-platform-service-broker
[GCP Config Connector]: https://cloud.google.com/config-connector/docs/overview
[AWS Controllers for Kubernetes]: https://github.com/aws-controllers-k8s/community
[Terraform]: https://www.terraform.io/
[Pulumi]: https://www.pulumi.com/
