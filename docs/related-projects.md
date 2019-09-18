---
title: Related Projects
toc: true
weight: 850
indent: true
---
# Related Projects

While there are many projects that address similar issues, none of them encapsulate the full use case that Crossplane addresses. This list is not exhaustive and is not meant to provide a deep analysis of the following projects, but instead to motivate why Crossplane was created.

## Open Service Broker and Service Catalog

The [Open Service Broker](https://www.openservicebrokerapi.org/) and the [Kubernetes Service Catalog](https://kubernetes.io/docs/concepts/extend-kubernetes/service-catalog/) are able to dynamically provision managed services in multiple cloud providers from Kubernetes. As a result it shares similar goals with Crossplane. However, service broker is not designed for workload portability, does not have a good separation of concern, and does not offer any integration with workload and resource scheduling. Service brokers can not span multiple cloud providers at once.

## Kubernetes Federation

The [federation-v2](https://github.com/kubernetes-sigs/federation-v2) project offers a single control plane that can span multiple Kubernetes clusters. It’s being incubated in SIG-multicluster. Crossplane shares some of the goals of managing multiple Kubernetes clusters and also the core principles of creating a higher level control plane, scheduler and controllers that span clusters. While the federation-v2 project is scoped to just Kubernetes clusters, Crossplane supports non-container workloads, and orchestrating resources that run as managed services including databases, message queues, buckets, and others. The federation effort focuses on defining Kubernetes objects that can be templatized, and propagated to other Kubernetes clusters. Crossplane focuses on defining portable workload abstractions across cloud providers and offerings. We have considered taking a dependency on the federation-v2 work within Crossplane, although it’s not clear at this point if this would accelerate the Crossplane effort.

## AWS Service Operator

The [AWS Service Operator](https://github.com/awslabs/aws-service-operator) is a recent project that implements a set of Kubernetes controllers that are able to provision managed services in AWS. It defines a set of CRDs for managed services like DynamoDB, and controllers that can provision them via AWS CloudFormation. It is similar to Crossplane in that it can provision managed services in AWS. Crossplane goes a lot further by offering workload portability across cloud multiple cloud providers, separation of concern, and a scheduler for workload and resources.

## AWS CloudFormation, GCP Deployment Manager, and Others

These products offer a declarative model for deploying and provisioning infrastructure in each of the respective cloud providers. They only work for one cloud provider and do not solve the problem of workload portability. These products are generally closed source, and offer little or no extensibility points. We have considered using some of these products as a way to implement resource controllers in Crossplane.

## Terraform

[Terraform](https://www.terraform.io/) is a popular tool for provisioning infrastructure across cloud providers. It offers a declarative configuration language with support for templating, composability, referential integrity and dependency management. Terraform can dynamically provision infrastructure and perform changes when the tool is run by a human. Unlike Crossplane, Terraform does not support workload portability across cloud providers, and does not have any active controllers that can react to failures, or make changes to running infrastructure without human intervention. Terraform attempts to solve multicloud at the tool level, while Crossplane is at the API and control plane level. Terraform is open source under a MPL license, and follows an open core business model, with a number of its features closed source. We are evaluating whether we can use Terraform to accelerate the development of resource controllers in Crossplane.

## Pulumi

[Pulumi](https://www.pulumi.com/) is a product that is based on terraform and uses most of its providers. Instead of using a configuration language, Pulumi uses popular programming languages like Typescript to capture the configuration. At runtime, Pulumi generates a DAG of resources just like terraform and applies it to cloud providers. Pulumi has an early model for workload portability that is implemented using language abstractions. Unlike Crossplane, it does not have any active controllers that can react to failures, or make changes to running infrastructure without human intervention, nor does it support workload scheduling. Pulumi attempts to solve multicloud scenarios at the language level, while Crossplane is at the API and control plane level. Pulumi is open source under a APL2 license but a number of features require using their SaaS offering.
