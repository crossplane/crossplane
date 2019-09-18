---
title: Developer Guide
toc: true
weight: 710
---
# Developer Guide

Welcome to the Crossplane Developer Guide!

## Overview

Infra Stacks like [stack-gcp][stack-gcp], [stack-aws][stack-aws], and
[stack-azure][stack-azure] extend Crossplane to support managed service
provisioning (databases, caches, buckets), secure connectivity (VPCs, subnets,
peering, ACLs, secrets), and provisioning managed Kubernetes clusters on demand
to further isolate the blast radius of applications.

Infra Stacks are typically pre-built and published to the [Stacks
registry][stack-registry], where they can be installed by a cluster
administrator using a [`ClusterStackInstall`][stack-install-docs] kind via the
Kubernetes API or with the [`stack install`][crossplane-cli-usage] command.

App Stacks depend on Infra Stacks like [stack-gcp][stack-gcp],
[stack-aws][stack-aws], or [stack-azure][stack-azure] to provide the managed
services they depend on via the Kubernetes API.

App Stacks may also be pre-built and published to the [Stacks
registry][stack-registry] where they can be deployed by application teams using
a [`StackInstall`][crossplane-cli-usage] kind via the Kubernetes API or with
the [`stack install`][crossplane-cli-usage] command.

## Infra Stacks

### Using Infra Stacks

The [Crossplane Services Guide][services-user-guide] shows how to use existing
 Infra Stacks to deploy a Wordpress `Deployment` that securely consumes a MySQL
 instance from GCP, AWS, or Azure all from `kubectl`.

### Building Infra Stacks

Infra Stacks are out-of-tree Crossplane extensions that can be built and
published on their own schedule separate from the core Crossplane repos.

Crossplane enables the community to build a modular, open cloud control plane
where any cloud service or capability can be added using the [Stack
Manager][stack-manager], an extension manager for the Kubernetes API. Crossplane
Stacks simplify the work required to build, publish, install and manage control
plane extensions with a powerful RBAC permission model, integrated dependency
management, and more.

The [Services Developer Guide][services-developer-guide] shows how to:

* Extend existing Infra Stacks ([stack-gcp][stack-gcp], [stack-aws][stack-aws],
  [stack-azure][stack-azure]) to add more cloud services.
* Build a new Infra Stack to add more cloud providers.
* Make independent cloud offerings available via the Kubernetes API, so
  application teams can use them just like standard Kubernetes resources.

## App Stacks

### Using App Stacks

The [Crossplane Stacks Guide][stack-user-guide] guide shows how to use a
[portable App Stack][stack-wordpress-registry] that can deploy with any Infra
Stack including: [stack-gcp][stack-gcp], [stack-aws][stack-aws], or
[stack-azure][stack-azure].

### Building App Stacks

To learn how to build a "Hello World" Stack see the [Stacks Quick Start][stack-quick-start].

For a complete App Stack, see the [portable Wordpress App
Stack][stack-wordpress] with a kubebuilder-based app
[`Controller`][kubernetes-controller] that owns a `WordressInstance` CRD, builds
a complete `KubernetesApplication`, and automates much of what's covered in the
[Crossplane Services Guide][services-user-guide] plus dynamic cluster
provisioning, so you can provision a complete Wordpress app instance from
`kubectl` using a single Kubernetes object.

## Learn More

If you have any questions, please drop us a note on [Crossplane
Slack][join-crossplane-slack] or [contact us][contact-us]!

To [learn more][learn-more] checkout these [useful links][learn-more].

<!-- Named links -->
[services-user-guide]: services-guide.md
[stack-user-guide]: stacks-guide.md
[stack-registry]: https://hub.docker.com/search?q=crossplane&type=image
[crossplane-cli-usage]: https://github.com/crossplaneio/crossplane-cli#usage
[stack-install-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#installation-flow
[stack-gcp]: https://github.com/crossplaneio/stack-gcp
[stack-aws]: https://github.com/crossplaneio/stack-aws
[stack-azure]: https://github.com/crossplaneio/stack-azure
[stack-wordpress]: https://github.com/crossplaneio/sample-stack-wordpress
[stack-wordpress-registry]: https://hub.docker.com/r/crossplane/sample-stack-wordpress
[stack-manager]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#terminology
[services-developer-guide]: services-developer-guide.md
[stack-quick-start]: https://github.com/crossplaneio/crossplane-cli#quick-start-stacks
[kubernetes-controller]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers
[join-crossplane-slack]: https://slack.crossplane.io
[contact-us]: https://github.com/crossplaneio/crossplane#contact
[learn-more]: learn-more.md
