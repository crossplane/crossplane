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
Crossplane in that it can provision managed services in GCP. Crossplane goes a
lot further by enabling you to provision managed services from any cloud
provider and the ability to define, compose, and publish your own
infrastructure resources in a no-code way. Crossplane supports a team-centric
approach with a strong separation of concerns, that enables applications to
easily and safely consume the infrastructure they need, using any tool that
works with the Kubernetes API. GCP Config Connector is closed-source.

## AWS Service Operator

The [AWS Service Operator] is a recent project that implements a set of
Kubernetes controllers that are able to provision managed services in AWS. It
defines a set of CRDs for managed services like DynamoDB, and controllers that
can provision them via AWS CloudFormation. It is similar to Crossplane in that
it can provision managed services in AWS. Crossplane goes a lot further by
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
controllers in Crossplane.

## Terraform

[Terraform] is a popular tool for provisioning infrastructure across cloud
providers. It offers a declarative configuration language with support for
templating, composability, referential integrity and dependency management.
Terraform can declaratively manage any compatible API and perform changes when
the tool is run by a human or in a deployment pipeline. Unlike Crossplane,
Terraform does not support a team-centric approach where infrastructure
operators can define, compose, and publish new infrastructure API types in
Kubernetes, as a proxy to cloud infrastructure, so app operators can easily
consume the infrastructure they need while conforming to organizational
best-practices and security policies. Terraform takes a tools approach, and
Crossplane is at the API and control plane level. Crossplane enables you to
define control-plane APIs for your infrastructure in a no-code way, so app
teams can use them with their tool of choice, including Terraform. Terraform is
open source under a MPL license, and follows an open core business model, with
a number of its features closed source. We are evaluating whether we can use
Terraform to accelerate the development of resource controllers in Crossplane.

## Pulumi

[Pulumi] is a product that is based on Terraform and uses most of its
providers.  Instead of using a configuration language, Pulumi uses popular
programming languages like Typescript to capture the configuration. At runtime,
Pulumi generates a DAG of resources just like Terraform and applies it to cloud
providers. Unlike Crossplane, Pulumi does not support a team-centric approach
where infrastructure operators can define, compose, and publish new
infrastructure API types in Kubernetes, with active controllers that can react
to failures and continuously reconcile to achieve their desired state. Pulumi
takes a language and SDK approach and Crossplane is at the API and control
plane level. Crossplane enables you to define a control-plane for your
infrastructure in a no-code way, so app teams can use them with their tool or
SDK of choice, including Pulumi.  Pulumi is open source under a APL2 license
but a number of features require using their SaaS offering.

<!-- Named Links -->

[Open Service Broker]: https://www.openservicebrokerapi.org/
[Kubernetes Service Catalog]: https://kubernetes.io/docs/concepts/extend-kubernetes/service-catalog/
[GCP OSB]: https://cloud.google.com/kubernetes-engine/docs/concepts/google-cloud-platform-service-broker
[GCP Config Connector]: https://cloud.google.com/config-connector/docs/overview
[AWS Service Operator]: https://github.com/awslabs/aws-service-operator
[Terraform]: https://www.terraform.io/
[Pulumi]: https://www.pulumi.com/
